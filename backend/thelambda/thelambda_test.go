package main

import (
	"browseimage/bitypes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImageQuery(t *testing.T) {
	const digest = "sha256:030eaf505bc7d39bfe1c485762603a95b5d43277f98cd3334d85e84547073abb"

	// repo is used verbatim in DynamoDB and S3 keys, so only the tag may be removed.
	for image, wantRepo := range map[string]string{
		"ubuntu":                           "ubuntu",
		"golang:1.19":                      "golang",
		"mcr.microsoft.com/dotnet/sdk:6.0": "mcr.microsoft.com/dotnet/sdk",
		"localhost:5000/team/app":          "localhost:5000/team/app",
		"localhost:5000/team/app:v1":       "localhost:5000/team/app",
	} {
		repo, ref, err := parseImageQuery(url.Values{"image": {image}, "digest": {digest}})
		require.NoError(t, err, image)
		assert.Equal(t, wantRepo, repo, image)
		assert.Equal(t, digest, ref.DigestStr(), image)
	}

	for _, q := range []url.Values{
		{"image": {"ubuntu"}},
		{"image": {"ubuntu"}, "digest": {"sha256:030e"}}, // a prefix would match other digests in DynamoDB
		{"digest": {digest}},
	} {
		_, _, err := parseImageQuery(q)
		var se *statusError
		require.True(t, errors.As(err, &se), q.Encode())
		assert.Equal(t, http.StatusBadRequest, se.status, q.Encode())
	}
}

func TestImageMetadata(t *testing.T) {
	const digest = "sha256:030eaf505bc7d39bfe1c485762603a95b5d43277f98cd3334d85e84547073abb"
	config := []byte(`{"os":"linux","architecture":"arm64","variant":"v8"}`)
	manifest := []byte(`{"layers":[{"size":1234},{"size":5678}]}`)

	for _, indexed := range []bool{false, true} {
		name := "not indexed"
		if indexed {
			name = "indexed"
		}
		t.Run(name, func(t *testing.T) {
			// A local AWS HTTP stub makes any attempt to write an index fail.
			db := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Amz-Target") != "DynamoDB_20120810.Query" {
					t.Error("metadata request attempted a database write")
					w.WriteHeader(http.StatusForbidden)
					return
				}
				items := []any{}
				if indexed {
					items = append(items, map[string]any{
						"pk":          map[string]string{"S": "image#example/app"},
						"sk":          map[string]string{"S": "digest#" + digest},
						"Tags":        map[string]any{"L": []any{}},
						"RawConfig":   map[string]any{"B": config},
						"Manifest":    map[string]any{"B": manifest},
						"ExecutionId": map[string]string{"S": "test"},
						"Status":      map[string]string{"S": "SUCCEEDED"},
						"TotalSize":   map[string]string{"N": "6912"},
						"Duration":    map[string]string{"N": "1000000000"},
						"Retrieved":   map[string]string{"S": "2026-10-14T00:00:00Z"},
					})
				}
				w.Header().Set("Content-Type", "application/x-amz-json-1.0")
				json.NewEncoder(w).Encode(map[string]any{"Items": items})
			}))
			defer db.Close()
			h := &handler{table: "test", dynamodb: dynamodb.New(dynamodb.Options{
				Region: "us-east-1", Credentials: aws.AnonymousCredentials{},
				EndpointResolver: dynamodb.EndpointResolverFromURL(db.URL),
			})}
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/api/metadata?"+url.Values{
				"image": {"example/app:v1"}, "digest": {digest},
			}.Encode(), nil)
			handleErrors(h.handleMetadata)(w, r)

			if !indexed {
				assert.Equal(t, http.StatusNotFound, w.Code)
				assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
				assert.Contains(t, w.Body.String(), "image has not been indexed")
				return
			}
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			var output HandleImageOutput
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &output))
			assert.Equal(t, bitypes.ImageInfoStatusSucceeded, output.Status)
			assert.Equal(t, "example/app", output.Repo)
			assert.Equal(t, digest, output.Digest)
			assert.Equal(t, int64(6912), output.TotalSize)
			assert.JSONEq(t, string(config), string(output.Config))
			assert.JSONEq(t, string(manifest), string(output.Manifest))
		})
	}
}

func TestHiddenIndexAPI(t *testing.T) {
	const digest = "sha256:030eaf505bc7d39bfe1c485762603a95b5d43277f98cd3334d85e84547073abb"
	for _, endpoint := range []string{"dir", "file"} {
		for _, hidden := range []string{"", "false", "true"} {
			t.Run(endpoint+"/"+hidden, func(t *testing.T) {
				filename := "index.json.gz"
				if hidden == "true" {
					filename = "hidden-index.json.gz"
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/bucket/images/example/app/"+digest+"/"+filename {
						w.WriteHeader(http.StatusForbidden)
						return
					}
					w.Header().Set("Content-Type", "application/xml")
					w.WriteHeader(http.StatusNotFound)
					w.Write([]byte(`<Error><Code>NoSuchKey</Code><Message>missing</Message></Error>`))
				}))
				defer server.Close()
				h := &handler{bucket: "bucket", s3: s3.New(s3.Options{
					Region: "us-east-1", Credentials: aws.AnonymousCredentials{},
					EndpointResolver: s3.EndpointResolverFromURL(server.URL), UsePathStyle: true,
				})}
				q := url.Values{"image": {"example/app"}, "digest": {digest}, "hidden": {hidden}, "path": {"gone"}}
				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/api/"+endpoint+"?"+q.Encode(), nil)
				fn := h.handleListDirectory
				if endpoint == "file" {
					fn = h.handleFileContents
				}
				handleErrors(fn)(w, r)
				assert.Equal(t, http.StatusNotFound, w.Code)
				assert.Contains(t, w.Body.String(), "image has not been indexed")
			})
		}
	}
}

func TestGzipIndexCacheLRU(t *testing.T) {
	var downloads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads.Add(1)
		body := map[string]string{"a": "aaa", "b": "bbbbb", "c": "cccc", "full": "12345678"}[filepath.Base(r.URL.Path)]
		fmt.Fprint(w, body)
	}))
	defer server.Close()
	client := s3.New(s3.Options{
		Region: "us-east-1", Credentials: aws.AnonymousCredentials{},
		EndpointResolver: s3.EndpointResolverFromURL(server.URL), UsePathStyle: true,
	})
	dir := filepath.Join(t.TempDir(), "cache")
	cache, err := newGzipIndexCache(dir, 8)
	require.NoError(t, err)
	acquire := func(key, want string) (string, func()) {
		path, release, err := cache.acquire(context.Background(), client, "bucket", "layers/index-v2/"+key)
		require.NoError(t, err)
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, want, string(body))
		return path, release
	}
	usage := func() int64 {
		files, err := os.ReadDir(dir)
		require.NoError(t, err)
		var size int64
		for _, file := range files {
			info, err := file.Info()
			require.NoError(t, err)
			size += info.Size()
		}
		return size
	}

	a, release := acquire("a", "aaa")
	release()
	b, release := acquire("b", "bbbbb")
	release()
	require.Equal(t, int64(8), usage()) // exactly at capacity is allowed

	// Concurrent hits share a complete file, and refresh recency without S3.
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path, done, err := cache.acquire(context.Background(), client, "bucket", "layers/index-v2/a")
			if assert.NoError(t, err) {
				defer done()
				assert.Equal(t, a, path)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(2), downloads.Load())
	_, release = acquire("c", "cccc")
	release()
	require.NoFileExists(t, b) // b, not the oldest-created a, must be evicted
	require.FileExists(t, a)
	require.Equal(t, int64(7), usage())

	_, release = acquire("a", "aaa") // keep a pinned while making room
	_, _, err = cache.acquire(context.Background(), client, "bucket", "layers/index-v2/full")
	require.ErrorContains(t, err, "in use")
	require.FileExists(t, a)
	require.LessOrEqual(t, usage(), int64(8))
	release()
	_, release = acquire("full", "12345678")
	release()
	require.NoFileExists(t, a)
	require.Equal(t, int64(8), usage())

	// A runtime reset must not leave unaccounted files behind in persistent /tmp.
	_, err = newGzipIndexCache(dir, 8)
	require.NoError(t, err)
	require.Zero(t, usage())
}

func TestGzipIndexCacheFailedDownloads(t *testing.T) {
	var broken atomic.Bool
	broken.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if filepath.Base(r.URL.Path) == "oversized" {
			fmt.Fprint(w, "123456789")
			return
		}
		w.Header().Set("Content-Length", "8")
		if broken.Load() {
			fmt.Fprint(w, "123")
			return
		}
		fmt.Fprint(w, "12345678")
	}))
	defer server.Close()
	client := s3.New(s3.Options{
		Region: "us-east-1", Credentials: aws.AnonymousCredentials{},
		EndpointResolver: s3.EndpointResolverFromURL(server.URL), UsePathStyle: true,
	})
	dir := filepath.Join(t.TempDir(), "cache")
	cache, err := newGzipIndexCache(dir, 8)
	require.NoError(t, err)
	for _, key := range []string{"oversized", "truncated"} {
		_, _, err = cache.acquire(context.Background(), client, "bucket", "layers/index-v2/"+key)
		require.Error(t, err)
		files, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, files)
	}
	broken.Store(false)
	path, release, err := cache.acquire(context.Background(), client, "bucket", "layers/index-v2/truncated")
	require.NoError(t, err)
	defer release()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "12345678", string(body))
}
