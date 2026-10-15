package main

import (
	"browseimage/bitypes"
	"browseimage/layerreader"
	"browseimage/logging"
	"browseimage/targzi"
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func main() {
	logging.Init()

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(fmt.Sprintf("err %+v", err))
	}

	s3api := s3.NewFromConfig(cfg)
	d := downloader{
		s3:       s3api,
		uploader: manager.NewUploader(s3api),
		bucket:   os.Getenv("BUCKET"),
		dynamodb: dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
			o.Retryer = retry.NewStandard(func(o *retry.StandardOptions) {
				o.MaxAttempts = 10
			})
		}),
		table: os.Getenv("TABLE"),
		cache: os.Getenv("LAYER_CACHE") != "false",
	}

	lambda.Start(logging.Middleware(d.handle))
}

type downloader struct {
	s3       *s3.Client
	uploader *manager.Uploader
	bucket   string
	dynamodb *dynamodb.Client
	table    string
	cache    bool
}

func (d *downloader) handle(ctx context.Context, input *layerreader.RemoteInput) (*layerreader.RemoteOutput, error) {
	ctx = logging.WithRequestPayload(ctx, input)
	slog.InfoContext(ctx, "handling remote layer download request")
	started := time.Now()

	ref, err := name.ParseReference(fmt.Sprintf("%s@%s", input.Key.Repo, input.Key.Digest))
	if err != nil {
		return nil, fmt.Errorf("parsing ref: %w", err)
	}

	img, err := remote.Image(ref, remote.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("getting image for ref: %w", err)
	}

	layer, err := img.LayerByDigest(input.Layer)
	if err != nil {
		return nil, fmt.Errorf("getting remote layer: %w", err)
	}

	totalSize, err := layer.Size()
	if err != nil {
		return nil, fmt.Errorf("calculating layer total size: %w", err)
	}

	key := bitypes.LayerProgressKey{
		Repo:        input.Key.Repo,
		ImageDigest: input.Key.Digest,
		LayerDigest: input.Layer.String(),
	}

	cacheKey := fmt.Sprintf("layers/index-v2/%s/complete.json", input.Layer)
	if d.cache {
		cached, err := d.cachedIndex(ctx, cacheKey)
		if err != nil {
			return nil, err
		}
		if cached != nil && cached.TotalBytes == totalSize {
			_, err := d.dynamodb.PutItem(ctx, &dynamodb.PutItemInput{
				TableName: &d.table,
				Item: (&bitypes.LayerProgress{LayerProgressKey: key, TotalBytes: totalSize,
					CompletedBytes: totalSize, TotalFiles: cached.Files, CompletedFiles: cached.Files}).Marshal(),
			})
			slog.InfoContext(ctx, "reused layer index", "duration", time.Since(started), "bytes", totalSize)
			return &cached.RemoteOutput, err
		}
	}

	_, err = d.dynamodb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &d.table,
		Item: (&bitypes.LayerProgress{
			LayerProgressKey: key,
			TotalBytes:       totalSize,
		}).Marshal(),
	})
	if err != nil {
		return nil, fmt.Errorf("putting initial layer progress metrics: %w", err)
	}

	raw, err := layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("getting layer reader: %w", err)
	}

	defer raw.Close()

	checksum, err := v1.Hasher(input.Layer.Algorithm)
	if err != nil {
		return nil, err
	}
	counter := &CountReader{Reader: io.TeeReader(raw, checksum)}
	var fileCount int64 = 0

	countctx, cancel := context.WithCancel(ctx)
	progressDone := make(chan struct{})
	defer func() { cancel(); <-progressDone }()
	go func() {
		defer close(progressDone)
		d.countProgress(countctx, &counter.count, &fileCount, key)
	}()

	// Lambda reuses /tmp across invocations, so remove the index files once uploaded.
	dir, err := os.MkdirTemp("", "targzi*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	indexStart := time.Now()
	index, err := targzi.BuildIndex(ctx, dir, counter, &fileCount)
	if err != nil {
		return nil, fmt.Errorf("building index: %w", err)
	}
	if counter.count != totalSize || input.Layer.Hex != fmt.Sprintf("%x", checksum.Sum(nil)) {
		return nil, fmt.Errorf("layer size or digest mismatch")
	}
	slog.InfoContext(ctx, "indexed layer", "duration", time.Since(indexStart), "read_duration", counter.readDuration, "bytes", counter.count, "files", fileCount)

	// Flush the final progress to DynamoDB before continuing.
	cancel()
	<-progressDone

	// Each build writes an immutable pair. A single completion object publishes
	// the pair atomically; concurrent builds cannot mix offsets and dictionaries.
	var buildID [16]byte
	if _, err := cryptorand.Read(buildID[:]); err != nil {
		return nil, fmt.Errorf("creating index build ID: %w", err)
	}
	prefix := fmt.Sprintf("layers/index-v2/%s/%x/", input.Layer, buildID)

	gzf, err := os.Open(index.GzIndexPath)
	if err != nil {
		return nil, fmt.Errorf("opening gzip index file: %w", err)
	}
	defer gzf.Close()
	gzfstat, err := gzf.Stat()
	if err != nil {
		return nil, fmt.Errorf("getting gzip index file stats: %w", err)
	}

	gziPut, err := d.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: &d.bucket,
		Key:    aws.String(prefix + filepath.Base(index.GzIndexPath)),
		Body:   gzf,
	})
	if err != nil {
		return nil, fmt.Errorf("uploading gzip index to S3: %w", err)
	}

	ff, err := os.Open(index.FileIndexPath)
	if err != nil {
		return nil, fmt.Errorf("opening file index: %w", err)
	}
	defer ff.Close()
	ffstat, err := ff.Stat()
	if err != nil {
		return nil, fmt.Errorf("getting file index stats: %w", err)
	}

	tarPut, err := d.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: &d.bucket,
		Key:    aws.String(prefix + filepath.Base(index.FileIndexPath)),
		Body:   ff,
	})
	if err != nil {
		return nil, fmt.Errorf("uploading file index to S3: %w", err)
	}

	output := &layerreader.RemoteOutput{
		Gzi: layerreader.Put{
			Key:       aws.ToString(gziPut.Key),
			VersionId: aws.ToString(gziPut.VersionID),
			Size:      gzfstat.Size(),
		},
		Tar: layerreader.Put{
			Key:       aws.ToString(tarPut.Key),
			VersionId: aws.ToString(tarPut.VersionID),
			Size:      ffstat.Size(),
		},
	}
	if d.cache {
		body, err := json.Marshal(layerCache{RemoteOutput: *output, TotalBytes: totalSize, Files: fileCount})
		if err != nil {
			return nil, err
		}
		if _, err := d.s3.PutObject(ctx, &s3.PutObjectInput{Bucket: &d.bucket, Key: &cacheKey, Body: bytes.NewReader(body)}); err != nil {
			return nil, fmt.Errorf("publishing layer cache: %w", err)
		}
	}
	slog.InfoContext(ctx, "completed layer", "duration", time.Since(started))
	return output, nil
}

type layerCache struct {
	layerreader.RemoteOutput
	TotalBytes int64
	Files      int64
}

func (d *downloader) cachedIndex(ctx context.Context, key string) (*layerCache, error) {
	get, err := d.s3.GetObject(ctx, &s3.GetObjectInput{Bucket: &d.bucket, Key: &key})
	var missing *s3types.NoSuchKey
	if errors.As(err, &missing) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading layer cache: %w", err)
	}
	defer get.Body.Close()
	var cached layerCache
	if err := json.NewDecoder(get.Body).Decode(&cached); err != nil {
		return nil, fmt.Errorf("decoding layer cache: %w", err)
	}
	return &cached, nil
}

// countProgress writes the byte and file counters to DynamoDB every second
// until ctx is cancelled, then writes them one final time.
func (d *downloader) countProgress(ctx context.Context, byteCounter, fileCounter *int64, key bitypes.LayerProgressKey) {
	update := func(ctx context.Context) {
		byteCount := atomic.LoadInt64(byteCounter)
		fileCount := atomic.LoadInt64(fileCounter)

		_, err := d.dynamodb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName:        &d.table,
			Key:              key.Key(),
			UpdateExpression: aws.String("SET CompletedBytes = :CompletedBytes, CompletedFiles = :CompletedFiles"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":CompletedBytes": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", byteCount)},
				":CompletedFiles": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", fileCount)},
			},
		})
		if err != nil {
			// Progress is informational; a failed update must not fail the layer.
			slog.WarnContext(ctx, "updating layer progress", "error", err)
		}
	}

	// added some jitter to avoid request storms to ddb
	select {
	case <-ctx.Done():
	case <-time.After(time.Duration(rand.Int63n(int64(time.Second)))):
	}

	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			// ctx is already cancelled here, so the final write needs a context that isn't.
			update(context.WithoutCancel(ctx))
			return
		case <-tick.C:
			update(ctx)
		}
	}
}

type CountReader struct {
	io.Reader
	count        int64
	readDuration time.Duration
}

func (c *CountReader) Read(p []byte) (n int, err error) {
	started := time.Now()
	n, err = c.Reader.Read(p)
	c.readDuration += time.Since(started)
	atomic.AddInt64(&c.count, int64(n))
	return
}
