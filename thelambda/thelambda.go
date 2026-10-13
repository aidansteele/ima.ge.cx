package main

import (
	"browseimage/bitypes"
	"browseimage/handlehttp"
	"browseimage/layerreader"
	"browseimage/logging"
	"browseimage/s3select"
	"browseimage/targzi"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"mime"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-xray-sdk-go/instrumentation/awsv2"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/aws/smithy-go"
	"github.com/glassechidna/go-emf/emf"
	"github.com/glassechidna/go-emf/emf/unit"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/gorilla/mux"
	"github.com/oklog/ulid/v2"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func main() {
	logging.Init()

	ctx := context.Background()

	emf.Namespace = "browseimage"

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(fmt.Sprintf("%+v", err))
	}

	awsv2.AWSV2Instrumentor(&cfg.APIOptions)

	h := &handler{
		s3:     s3.NewFromConfig(cfg),
		bucket: os.Getenv("BUCKET"),
		http: &auth.Client{
			//Client: xray.Client(&http.Client{Transport: &transport{RoundTripper: http.DefaultTransport}}),
			Client: xray.Client(http.DefaultClient),
			Cache:  auth.DefaultCache,
		},
		transport: xray.RoundTripper(http.DefaultTransport),
		dynamodb:  dynamodb.NewFromConfig(cfg),
		table:     os.Getenv("TABLE"),
		sfn:       sfn.NewFromConfig(cfg),
		machine:   os.Getenv("MACHINE"),
		entropy:   ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0),
	}

	r := mux.NewRouter()
	r.HandleFunc("/api/dir", handleErrors(h.handleListDirectory))
	r.HandleFunc("/api/file", handleErrors(h.handleFileContents))
	r.HandleFunc("/api/info", handleErrors(h.handleInfo))
	r.HandleFunc("/api/lookup", handleErrors(h.handleLookup))
	r.HandleFunc("/api/tags", handleErrors(h.handleTags))

	r.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			version := os.Getenv("AWS_LAMBDA_FUNCTION_VERSION")

			query := map[string]string{}
			for k, vs := range r.URL.Query() {
				query[k] = vs[0]
			}

			ctx := logging.WithAttrs(r.Context(),
				"path", r.URL.Path,
				"method", r.Method,
				"query", query,
			)
			r = r.WithContext(ctx)

			slog.InfoContext(ctx, "handling HTTP request")

			defer func() {
				rerr := recover()

				duration := time.Since(start)
				msi := emf.MSI{
					"FunctionVersion": version,
					"Path":            r.URL.Path,
					"Query":           query,
					"TraceId":         os.Getenv("_X_AMZN_TRACE_ID"),
					"Milliseconds":    emf.Metric(float64(duration.Milliseconds()), unit.Milliseconds),
				}
				defer emf.Emit(msi)

				if rerr != nil {
					// Re-panicking would make the Lambda runtime exit the process
					// and the client would get a 502, so respond with a 500 instead.
					msi["Error"] = fmt.Sprintf("%+v", rerr)
					slog.ErrorContext(ctx, "request panicked", "error", rerr, "stack", string(debug.Stack()))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()

			w.Header().Set("Function-Version", version)
			w.Header().Set("Function-Region", os.Getenv("AWS_REGION"))
			h.ServeHTTP(w, r)
		})
	})

	if _, ok := os.LookupEnv("_HANDLER"); ok {
		lambda.Start(handlehttp.WrapHandler(r))
	} else {
		err = http.ListenAndServe(":8080", r)
		panic(err)
	}
}

type handler struct {
	s3        *s3.Client
	bucket    string
	http      *auth.Client
	transport http.RoundTripper
	dynamodb  *dynamodb.Client
	table     string
	sfn       *sfn.Client
	machine   string
	entropy   io.Reader
}

// statusError is an error that should be reported to the client with a specific HTTP status.
type statusError struct {
	status int
	err    error
}

func (e *statusError) Error() string { return e.err.Error() }
func (e *statusError) Unwrap() error { return e.err }

func withStatus(status int, err error) error {
	return &statusError{status: status, err: err}
}

// handleErrors adapts a handler that returns an error into an http.HandlerFunc.
// Errors become 500 responses unless they carry a status via withStatus.
func handleErrors(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := fn(w, r)
		if err == nil {
			return
		}

		status := http.StatusInternalServerError
		msg := http.StatusText(status)
		var se *statusError
		if errors.As(err, &se) {
			status = se.status
			msg = se.Error()
		}

		level := slog.LevelError
		if status < http.StatusInternalServerError {
			level = slog.LevelWarn
		}
		slog.Log(r.Context(), level, "handling HTTP request", "error", err, "status", status)
		http.Error(w, msg, status)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	j, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshalling response: %w", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(j)
	return nil
}

func isNoSuchKey(err error) bool {
	var ae smithy.APIError
	return errors.As(err, &ae) && ae.ErrorCode() == "NoSuchKey"
}

func isConditionalCheckFailed(err error) bool {
	var ccf *types.ConditionalCheckFailedException
	return errors.As(err, &ccf)
}

// parseImageQuery reads the image and digest query parameters. repo is the
// image name with any tag removed, as used in DynamoDB and S3 keys.
func parseImageQuery(q url.Values) (repo string, ref name.Digest, err error) {
	image := q.Get("image")
	repo = image
	// A colon after the last slash starts the tag; an earlier one belongs to a registry port.
	if i := strings.LastIndex(image, ":"); i > strings.LastIndex(image, "/") {
		repo = image[:i]
	}

	ref, err = name.NewDigest(repo + "@" + q.Get("digest"))
	if err != nil {
		return "", name.Digest{}, withStatus(http.StatusBadRequest, fmt.Errorf("invalid image or digest: %w", err))
	}

	return repo, ref, nil
}

type lookupOutput struct {
	Error   string        `json:",omitempty"`
	Options []imageOption `json:",omitempty"`
}

type imageOption struct {
	Digest   v1.Hash
	Platform *v1.Platform
}

func (h *handler) remoteOptions(ctx context.Context) []remote.Option {
	return []remote.Option{
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithTransport(h.transport),
		remote.WithUserAgent("site/ima.ge.cx author/aidan@awsteele.com"),
	}
}

func (h *handler) handleLookup(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	w.Header().Set("Cache-Control", "max-age=60")

	opts, err := h.lookupOptions(ctx, r.URL.Query().Get("image"))
	if err != nil {
		return writeJSON(w, http.StatusOK, lookupOutput{Error: err.Error()})
	}

	return writeJSON(w, http.StatusOK, lookupOutput{Options: opts})
}

// lookupOptions returns the platform variants of an image reference.
func (h *handler) lookupOptions(ctx context.Context, imageQuery string) ([]imageOption, error) {
	ref, err := name.ParseReference(imageQuery)
	if err != nil {
		return nil, err
	}

	desc, err := remote.Get(ref, h.remoteOptions(ctx)...)
	if err != nil {
		return nil, err
	}

	if desc.MediaType.IsIndex() {
		index, err := desc.ImageIndex()
		if err != nil {
			return nil, err
		}

		im, err := index.IndexManifest()
		if err != nil {
			return nil, err
		}

		opts := []imageOption{}
		for _, manifest := range im.Manifests {
			// Skip entries that aren't runnable images, such as the build
			// attestation manifests (platform unknown/unknown) that BuildKit adds.
			if manifest.Platform == nil || manifest.Annotations["vnd.docker.reference.type"] == "attestation-manifest" {
				continue
			}

			opts = append(opts, imageOption{
				Digest:   manifest.Digest,
				Platform: manifest.Platform,
			})
		}

		return opts, nil
	}

	// Single image manifest (v2 or schema1)
	image, err := desc.Image()
	if err != nil {
		return nil, err
	}

	digest, err := image.Digest()
	if err != nil {
		return nil, err
	}

	cf, err := image.ConfigFile()
	if err != nil {
		return nil, err
	}

	return []imageOption{{
		Digest: digest,
		Platform: &v1.Platform{
			Architecture: cf.Architecture,
			OS:           cf.OS,
			OSVersion:    cf.OSVersion,
			Variant:      cf.Variant,
		},
	}}, nil
}

type tagsOutput struct {
	Error string   `json:",omitempty"`
	Tags  []string `json:",omitempty"`
}

// handleTags lists the tags in an image's repository. Like handleLookup,
// registry errors are reported in the body so the UI can show them.
func (h *handler) handleTags(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	w.Header().Set("Cache-Control", "max-age=300")

	ref, err := name.ParseReference(r.URL.Query().Get("image"))
	if err != nil {
		return writeJSON(w, http.StatusOK, tagsOutput{Error: err.Error()})
	}

	tags, err := remote.List(ref.Context(), h.remoteOptions(ctx)...)
	if err != nil {
		return writeJSON(w, http.StatusOK, tagsOutput{Error: err.Error()})
	}

	return writeJSON(w, http.StatusOK, tagsOutput{Tags: tags})
}

type LayerProgress struct {
	Layer          string
	TotalBytes     int64
	CompletedBytes int64
	TotalFiles     int64
	CompletedFiles int64
}

type HandleImageOutput struct {
	Status          bitypes.ImageInfoStatus
	Repo            string          `json:",omitempty"`
	Digest          string          `json:",omitempty"`
	ExecutionId     string          `json:",omitempty"`
	Progresses      []LayerProgress `json:",omitempty"`
	TotalSize       int64           `json:",omitempty"`
	CompletedSize   int64           `json:",omitempty"`
	EstimateSeconds int64           `json:",omitempty"`
	DurationSeconds float64         `json:",omitempty"`
	Retrieved       time.Time       `json:",omitempty"`
	Config          json.RawMessage `json:",omitempty"`
	Manifest        json.RawMessage `json:",omitempty"`
}

func estimateSeconds(totalSize int64) int64 {
	return 2 + (totalSize / 25e6)
}

func (h *handler) handleStartExecution(key *bitypes.ImageInfoKey, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	w.Header().Set("Cache-Control", "no-store")

	executionId := fmt.Sprintf("BI%s10", ulid.MustNew(ulid.Timestamp(time.Now()), h.entropy))

	item := &bitypes.ImageInfoItem{
		ImageInfoKey: *key,
		ExecutionId:  executionId,
		Status:       bitypes.ImageInfoStatusPending,
		Retrieved:    time.Now(),
	}

	_, err := h.dynamodb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &h.table,
		Item:                item.DynamoItem(),
		ConditionExpression: aws.String("attribute_not_exists(pk)"),
	})
	if isConditionalCheckFailed(err) {
		// A concurrent request started indexing this image first. The client
		// polls while the status is pending, so it will see that execution.
		return writeJSON(w, http.StatusOK, HandleImageOutput{Status: bitypes.ImageInfoStatusPending})
	}
	if err != nil {
		return fmt.Errorf("putting image info item: %w", err)
	}

	sfnInput, err := json.Marshal(key)
	if err != nil {
		return fmt.Errorf("marshalling execution input: %w", err)
	}

	_, err = h.sfn.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: &h.machine,
		Name:            &executionId,
		TraceHeader:     aws.String(os.Getenv("_X_AMZN_TRACE_ID")),
		Input:           aws.String(string(sfnInput)),
	})
	if err != nil {
		// Without this, the image would stay pending until the item expires.
		_, delErr := h.dynamodb.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName:                 &h.table,
			Key:                       item.Key(),
			ConditionExpression:       aws.String("ExecutionId = :executionId"),
			ExpressionAttributeValues: map[string]types.AttributeValue{":executionId": &types.AttributeValueMemberS{Value: executionId}},
		})
		return errors.Join(fmt.Errorf("starting execution: %w", err), delErr)
	}

	_, err = h.dynamodb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                &h.table,
		Key:                      item.Key(),
		UpdateExpression:         aws.String("SET #status = :running"),
		ConditionExpression:      aws.String("ExecutionId = :executionId AND #status = :pending"),
		ExpressionAttributeNames: map[string]string{"#status": "Status"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":executionId": &types.AttributeValueMemberS{Value: executionId},
			":pending":     &types.AttributeValueMemberS{Value: string(bitypes.ImageInfoStatusPending)},
			":running":     &types.AttributeValueMemberS{Value: string(bitypes.ImageInfoStatusRunning)},
		},
	})
	// If the condition fails, the execution has already finished and the
	// finalizer's status must not be overwritten.
	if err != nil && !isConditionalCheckFailed(err) {
		return fmt.Errorf("marking image info running: %w", err)
	}

	return writeJSON(w, http.StatusCreated, item)
}

func (h *handler) handleInfo(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	repo, ref, err := parseImageQuery(r.URL.Query())
	if err != nil {
		return err
	}
	digest := ref.DigestStr()

	p := dynamodb.NewQueryPaginator(h.dynamodb, &dynamodb.QueryInput{
		TableName:              &h.table,
		ConsistentRead:         aws.Bool(true),
		KeyConditionExpression: aws.String("pk = :pk and begins_with(sk, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("image#%s", repo)},
			":sk": &types.AttributeValueMemberS{Value: fmt.Sprintf("digest#%s", digest)},
		},
	})

	imageInfo := bitypes.ImageInfoItem{}
	progresses := []LayerProgress{}
	var completedSize int64 = 0

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("querying image info: %w", err)
		}

		for _, item := range page.Items {
			sk := strings.Split(item["sk"].(*types.AttributeValueMemberS).Value, "#")
			switch len(sk) {
			case 2:
				err = attributevalue.UnmarshalMap(item, &imageInfo)
				if err != nil {
					return fmt.Errorf("unmarshalling image info: %w", err)
				}
			case 4:
				lp := bitypes.LayerProgress{}
				err = attributevalue.UnmarshalMap(item, &lp)
				if err != nil {
					return fmt.Errorf("unmarshalling layer progress: %w", err)
				}
				progresses = append(progresses, LayerProgress{
					Layer:          lp.LayerDigest,
					TotalBytes:     lp.TotalBytes,
					CompletedBytes: lp.CompletedBytes,
					TotalFiles:     lp.TotalFiles,
					CompletedFiles: lp.CompletedFiles,
				})

				completedSize += lp.CompletedBytes
			default:
				return fmt.Errorf("unexpected dynamo item: %+v", item)
			}
		}
	}

	// image was not in dynamodb
	if imageInfo.Digest == "" {
		return h.handleStartExecution(&bitypes.ImageInfoKey{Repo: repo, Digest: digest}, w, r)
	}

	maxAge := time.Second
	if imageInfo.Status == bitypes.ImageInfoStatusSucceeded {
		maxAge = time.Hour * 24
	}

	output := &HandleImageOutput{
		Status:          imageInfo.Status,
		Repo:            imageInfo.Repo,
		Digest:          imageInfo.Digest,
		ExecutionId:     imageInfo.ExecutionId,
		TotalSize:       imageInfo.TotalSize,
		CompletedSize:   completedSize,
		Progresses:      progresses,
		EstimateSeconds: estimateSeconds(imageInfo.TotalSize),
		DurationSeconds: imageInfo.Duration.Seconds(),
		Retrieved:       imageInfo.Retrieved,
		Config:          imageInfo.RawConfig,
		Manifest:        imageInfo.Manifest,
	}

	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", maxAge/time.Second))
	return writeJSON(w, http.StatusOK, output)
}

func indexKey(repo string, ref name.Digest) string {
	return fmt.Sprintf("images/%s/%s/index.json.gz", repo, ref.DigestStr())
}

func (h *handler) handleListDirectory(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	q := r.URL.Query()
	repo, ref, err := parseImageQuery(q)
	if err != nil {
		return err
	}

	path := q.Get("path")
	if path == "" {
		path = "/"
	}

	msi := emf.MSI{
		"Image":      repo,
		"Digest":     ref.DigestStr(),
		"Path":       path,
		"StatusCode": emf.Dimension("200"),
	}
	defer emf.Emit(msi)

	query := "SELECT * FROM s3object s WHERE s.Parent = " + s3select.Quote(path)
	entries, err := s3select.Select[layerreader.EntryWithLayer](ctx, h.s3, h.bucket, indexKey(repo, ref), query)
	if isNoSuchKey(err) {
		msi["StatusCode"] = emf.Dimension("404")
		return withStatus(http.StatusNotFound, errors.New("image has not been indexed"))
	}
	if err != nil {
		msi["StatusCode"] = emf.Dimension("500")
		return fmt.Errorf("listing directory: %w", err)
	}

	return writeJSON(w, http.StatusOK, entries)
}

func (h *handler) handleFileContents(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	q := r.URL.Query()
	repo, ref, err := parseImageQuery(q)
	if err != nil {
		return err
	}

	path := q.Get("path")

	query := "SELECT * FROM s3object s WHERE s.Hdr.Name = " + s3select.Quote(path)
	entries, err := s3select.Select[layerreader.EntryWithLayer](ctx, h.s3, h.bucket, indexKey(repo, ref), query)
	if isNoSuchKey(err) {
		return withStatus(http.StatusNotFound, errors.New("image has not been indexed"))
	}
	if err != nil {
		return fmt.Errorf("querying file index: %w", err)
	}

	if len(entries) == 0 {
		return withStatus(http.StatusNotFound, errors.New("file not found"))
	}
	if len(entries) != 1 {
		return fmt.Errorf("unexpected number of entries: %d", len(entries))
	}
	entry := entries[0]

	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Empty files, directories and links have no content to extract, and gztool rejects a zero length.
	if entry.Hdr.Size == 0 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		return nil
	}

	indexPath, err := h.downloadGzipIndex(ctx, entry.Layer)
	if err != nil {
		return err
	}
	defer os.Remove(indexPath)

	spans, err := targzi.Spans(indexPath)
	if err != nil {
		return fmt.Errorf("reading spans: %w", err)
	}

	start, end, err := targzi.CompressedRange(spans, entry.Spans)
	if err != nil {
		return fmt.Errorf("finding compressed range: %w", err)
	}

	repository := ref.Context()
	ctx = auth.WithScopes(ctx, ref.Scope("pull"))

	blobURL := fmt.Sprintf("https://%s/v2/%s/blobs/%s", repository.RegistryStr(), repository.RepositoryStr(), entry.Layer)
	req, err := http.NewRequestWithContext(ctx, "GET", blobURL, nil)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}
	req.Header.Set("Range", targzi.RangeHeader(start, end))

	resp, err := h.http.Do(req)
	if err != nil {
		return fmt.Errorf("fetching layer blob: %w", err)
	}
	defer resp.Body.Close()

	// gztool needs the body to begin at the requested offset; any other
	// response (an error or the whole blob) would produce garbage.
	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("fetching layer blob: unexpected status %s", resp.Status)
	}

	extracted, err := targzi.Extract(ctx, resp.Body, indexPath, start, entry.Offset, int(entry.Hdr.Size))
	if err != nil {
		return fmt.Errorf("extracting file: %w", err)
	}

	// Never let browsers render image content (e.g. HTML) as active content on this origin.
	contentType := "application/octet-stream"
	mediaType, params, _ := mime.ParseMediaType(http.DetectContentType(extracted))
	if strings.HasPrefix(mediaType, "text/") {
		contentType = mime.FormatMediaType("text/plain", params)
	}
	w.Header().Set("Content-Type", contentType)
	w.Write(extracted)
	return nil
}

// downloadGzipIndex downloads a layer's gztool index to a temporary file and
// returns its path. The caller must remove the file.
func (h *handler) downloadGzipIndex(ctx context.Context, layer string) (string, error) {
	get, err := h.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &h.bucket,
		Key:    aws.String(fmt.Sprintf("layers/%s/index.gzi", layer)),
	})
	if err != nil {
		return "", fmt.Errorf("getting gzip index: %w", err)
	}
	defer get.Body.Close()

	indexFile, err := os.CreateTemp("", "gzi*")
	if err != nil {
		return "", fmt.Errorf("creating gzip index file: %w", err)
	}

	_, err = io.Copy(indexFile, get.Body)
	if closeErr := indexFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(indexFile.Name())
		return "", fmt.Errorf("writing gzip index file: %w", err)
	}

	return indexFile.Name(), nil
}
