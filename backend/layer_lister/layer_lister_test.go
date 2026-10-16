package main

import (
	"browseimage/httpclient"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/require"
)

func TestLayerSizesFromManifest(t *testing.T) {
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	large, small := fmt.Sprintf("sha256:%064d", 1), fmt.Sprintf("sha256:%064d", 2)
	configDigest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("{}")))
	manifest := fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":%q,"size":2},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":%q,"size":67108864},{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":%q,"size":67108863}]}`, configDigest, large, small)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/":
			fmt.Fprint(w, "{}")
		case strings.Contains(r.URL.Path, "/manifests/"):
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			fmt.Fprint(w, manifest)
		case strings.HasSuffix(r.URL.Path, "/blobs/"+configDigest):
			fmt.Fprint(w, "{}")
		case r.Header.Get("X-Amz-Target") != "":
			w.Header().Set("Content-Type", "application/x-amz-json-1.0")
			fmt.Fprint(w, "{}")
		default:
			// Layer sizes must be available even when blob downloads are forbidden.
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer server.Close()
	cfg := aws.Config{
		Region:      "us-west-2",
		HTTPClient:  httpclient.DefaultClient,
		Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""),
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(string, string, ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: server.URL}, nil
		}),
	}
	lister := layerLister{dynamodb: dynamodb.NewFromConfig(cfg), table: "test"}
	result, err := lister.handle(context.Background(), &layerListerInput{
		Repo:   strings.TrimPrefix(server.URL, "http://") + "/image",
		Digest: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(manifest))),
	})
	require.NoError(t, err)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.JSONEq(t, fmt.Sprintf(`{"Layers":[%q,%q],"LayerSizes":{%q:67108864,%q:67108863}}`, large, small, large, small), string(encoded))
}
