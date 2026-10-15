package main

import (
	"archive/tar"
	"browseimage/bitypes"
	"browseimage/layerreader"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/require"
)

func TestLayerIndexCache(t *testing.T) {
	if _, err := exec.LookPath("gztool"); err != nil {
		t.Skip("requires gztool on PATH (run with the built Linux layer)")
	}
	var blob bytes.Buffer
	gz := gzip.NewWriter(&blob)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "hello", Mode: 0644, Size: 5}))
	_, err := tw.Write([]byte("hello"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	layer, err := v1.NewHash(fmt.Sprintf("sha256:%x", sha256.Sum256(blob.Bytes())))
	require.NoError(t, err)
	manifest := []byte(fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"sha256:%064d","size":2},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":%q,"size":%d}]}`, 0, layer.String(), blob.Len()))
	var mu sync.Mutex
	objects := map[string][]byte{}
	denyBlob, denyManifest, failFiles := false, false, false
	var progress map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/v2/":
			w.Write([]byte("{}"))
		case strings.Contains(r.URL.Path, "/manifests/"):
			if denyManifest {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			w.Write(manifest)
		case strings.Contains(r.URL.Path, "/blobs/"):
			if denyBlob {
				w.WriteHeader(http.StatusForbidden)
			} else {
				w.Write(blob.Bytes())
			}
		case r.Header.Get("X-Amz-Target") != "":
			var payload map[string]any
			json.NewDecoder(r.Body).Decode(&payload)
			if item, ok := payload["Item"].(map[string]any); ok {
				progress = item
			}
			w.Header().Set("Content-Type", "application/x-amz-json-1.0")
			w.Write([]byte("{}"))
		case r.Method == http.MethodPut:
			if failFiles && strings.HasSuffix(r.URL.Path, "/files.json.gz") {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			objects[r.URL.Path], _ = io.ReadAll(r.Body)
			w.Header().Set("x-amz-version-id", "version")
		case r.Method == http.MethodGet:
			if body, ok := objects[r.URL.Path]; ok {
				w.Write(body)
			} else {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("<Error><Code>NoSuchKey</Code></Error>"))
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	cfg := aws.Config{Region: "us-west-2", Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""), EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(string, string, ...interface{}) (aws.Endpoint, error) { return aws.Endpoint{URL: server.URL}, nil })}
	api := s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true })
	d := downloader{s3: api, uploader: manager.NewUploader(api), dynamodb: dynamodb.NewFromConfig(cfg), bucket: "bucket", table: "table", cache: true}
	input := &layerreader.RemoteInput{Key: &bitypes.ImageInfoKey{Repo: strings.TrimPrefix(server.URL, "http://") + "/image", Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(manifest))}, Layer: layer}
	first, err := d.handle(context.Background(), input)
	require.NoError(t, err)
	mu.Lock()
	firstGzi := append([]byte(nil), objects["/bucket/"+first.Gzi.Key]...)
	firstTar := append([]byte(nil), objects["/bucket/"+first.Tar.Key]...)
	mu.Unlock()
	d.cache = false
	second, err := d.handle(context.Background(), input)
	require.NoError(t, err)
	require.NotEqual(t, first.Gzi.Key, second.Gzi.Key)
	mu.Lock()
	require.Equal(t, firstGzi, objects["/bucket/"+first.Gzi.Key])
	require.Equal(t, firstTar, objects["/bucket/"+first.Tar.Key])
	denyBlob = true
	mu.Unlock()
	d.cache = true
	cached, err := d.handle(context.Background(), input)
	require.NoError(t, err, "a cache hit must not download the blob")
	require.Equal(t, first, cached)
	mu.Lock()
	require.Equal(t, map[string]any{"N": fmt.Sprint(blob.Len())}, progress["CompletedBytes"])
	require.Equal(t, map[string]any{"N": "1"}, progress["CompletedFiles"])
	denyManifest = true
	mu.Unlock()
	_, err = d.handle(context.Background(), input)
	require.Error(t, err, "a cached index must not bypass source manifest access")
	mu.Lock()
	denyManifest = false
	marker := "/bucket/layers/index-v2/" + layer.String() + "/complete.json"
	delete(objects, marker)
	denyBlob, failFiles = false, true
	mu.Unlock()
	_, err = d.handle(context.Background(), input)
	require.Error(t, err)
	mu.Lock()
	defer mu.Unlock()
	require.NotContains(t, objects, marker, "a partial pair must never be reusable")
}
