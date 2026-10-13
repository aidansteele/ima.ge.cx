package main

import (
	"browseimage/bitypes"
	"browseimage/layerreader"
	"browseimage/logging"
	"browseimage/targzi"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/http/httputil"
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
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func main() {
	logging.Init()

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(fmt.Sprintf("err %+v", err))
	}

	d := downloader{
		uploader: manager.NewUploader(s3.NewFromConfig(cfg)),
		bucket:   os.Getenv("BUCKET"),
		dynamodb: dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
			o.Retryer = retry.NewStandard(func(o *retry.StandardOptions) {
				o.MaxAttempts = 10
			})
		}),
		table: os.Getenv("TABLE"),
	}

	lambda.Start(logging.Middleware(d.handle))
}

type downloader struct {
	uploader *manager.Uploader
	bucket   string
	dynamodb *dynamodb.Client
	table    string
}

type transport struct{}

func (t transport) RoundTrip(request *http.Request) (*http.Response, error) {
	ctx := request.Context()
	dump, _ := httputil.DumpRequestOut(request, false)
	slog.DebugContext(ctx, "outgoing HTTP request", "dump", string(dump))

	resp, err := http.DefaultTransport.RoundTrip(request)
	if resp != nil {
		dump, _ = httputil.DumpResponse(resp, false)
		slog.DebugContext(ctx, "incoming HTTP response", "dump", string(dump))
	}

	return resp, err
}

func (d *downloader) handle(ctx context.Context, input *layerreader.RemoteInput) (*layerreader.RemoteOutput, error) {
	ctx = logging.WithRequestPayload(ctx, input)
	slog.InfoContext(ctx, "handling remote layer download request")

	ref, err := name.ParseReference(fmt.Sprintf("%s@%s", input.Key.Repo, input.Key.Digest))
	if err != nil {
		return nil, fmt.Errorf("parsing ref: %w", err)
	}

	img, err := remote.Image(ref, remote.WithTransport(transport{}), remote.WithContext(ctx))
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

	counter := &CountReader{Reader: raw}
	var fileCount int64 = 0

	countctx, cancel := context.WithCancel(ctx)
	defer cancel()
	progressDone := make(chan struct{})
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

	index, err := targzi.BuildIndex(dir, counter, &fileCount)
	if err != nil {
		return nil, fmt.Errorf("building index: %w", err)
	}

	// Flush the final progress to DynamoDB before continuing.
	cancel()
	<-progressDone

	prefix := fmt.Sprintf("layers/%s/", input.Layer)

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

	return &layerreader.RemoteOutput{
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
	}, nil
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
	count int64
}

func (c *CountReader) Read(p []byte) (n int, err error) {
	n, err = c.Reader.Read(p)
	atomic.AddInt64(&c.count, int64(n))
	return
}
