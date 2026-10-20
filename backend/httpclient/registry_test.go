package httpclient_test

import (
	"browseimage/httpclient"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestRegistryRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		name       string
		after      []string
		method     string
		body       string
		status     int
		attempts   int32
		wantStatus int
		minWait    time.Duration
		deadline   bool
	}{
		{name: "seconds", after: []string{"1"}, attempts: 2, wantStatus: 206, minWait: time.Second},
		{name: "date", after: []string{"future"}, attempts: 2, wantStatus: 206, minWait: time.Second},
		{name: "past date", after: []string{time.Unix(0, 0).UTC().Format(http.TimeFormat)}, attempts: 2, wantStatus: 206},
		{name: "missing", after: []string{""}, attempts: 2, wantStatus: 206, minWait: time.Second},
		{name: "malformed", after: []string{"invalid"}, attempts: 2, wantStatus: 206, minWait: time.Second},
		{name: "negative", after: []string{"-1"}, attempts: 2, wantStatus: 206, minWait: time.Second},
		{name: "exhausted", after: []string{"0", "0", "0"}, attempts: 3, wantStatus: 429},
		{name: "cumulative budget", after: []string{"3", "3"}, attempts: 2, wantStatus: 429, minWait: 3 * time.Second},
		{name: "exact budget", after: []string{"5", "0"}, attempts: 3, wantStatus: 206, minWait: 5 * time.Second},
		{name: "too long", after: []string{"6"}, attempts: 1, wantStatus: 429},
		{name: "distant date", after: []string{"Wed, 01 Jan 2100 00:00:00 GMT"}, attempts: 1, wantStatus: 429},
		{name: "duration overflow", after: []string{"18446744073709551615"}, attempts: 1, wantStatus: 429},
		{name: "integer overflow", after: []string{"18446744073709551616"}, attempts: 1, wantStatus: 429},
		{name: "deadline", after: []string{"1"}, attempts: 1, wantStatus: 429, deadline: true},
		{name: "head", after: []string{"0"}, method: "HEAD", attempts: 2, wantStatus: 206},
		{name: "post", after: []string{"0"}, method: "POST", attempts: 1, wantStatus: 429},
		{name: "get with body", after: []string{"0"}, body: "request", attempts: 1, wantStatus: 429},
		{name: "forbidden", after: []string{"0"}, status: 403, attempts: 1, wantStatus: 403},
		{name: "unavailable", after: []string{"0"}, status: 503, attempts: 1, wantStatus: 503},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Range") != "bytes=17-29" || r.Header.Get("Authorization") != "Bearer test" {
					t.Error("retry lost range or authorization")
				}
				i := int(attempts.Add(1)) - 1
				if i < len(tc.after) {
					after := tc.after[i]
					if after == "future" {
						after = time.Now().Add(2 * time.Second).UTC().Format(http.TimeFormat)
					}
					w.Header().Set("Retry-After", after)
					status := tc.status
					if status == 0 {
						status = http.StatusTooManyRequests
					}
					w.WriteHeader(status)
					fmt.Fprint(w, "throttled")
					return
				}
				w.WriteHeader(http.StatusPartialContent)
				fmt.Fprint(w, "range content")
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if tc.deadline {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
				defer cancel()
			}
			method := tc.method
			if method == "" {
				method = http.MethodGet
			}
			req, err := http.NewRequestWithContext(ctx, method, server.URL, strings.NewReader(tc.body))
			require.NoError(t, err)
			req.Header.Set("Range", "bytes=17-29")
			req.Header.Set("Authorization", "Bearer test")
			started := time.Now()
			resp, err := (&http.Client{Transport: httpclient.RegistryTransport}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.GreaterOrEqual(t, time.Since(started), tc.minWait)
			require.Equal(t, tc.attempts, attempts.Load())
			require.Equal(t, tc.wantStatus, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			if method != http.MethodHead {
				want := "throttled"
				if tc.wantStatus == http.StatusPartialContent {
					want = "range content"
				}
				require.Equal(t, want, string(body), "final response body remains readable")
			}
		})
	}
}

func TestRegistryRetryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var attempts atomic.Int32
	closed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
		w.(http.Flusher).Flush()
		// The retry must close this unfinished body without trying to drain it.
		<-r.Context().Done()
		close(closed)
	}))
	defer server.Close()
	go func() {
		select {
		case <-closed:
		case <-time.After(time.Second):
		}
		cancel()
	}()
	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	_, err = (&http.Client{Transport: httpclient.RegistryTransport}).Do(req)
	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(started), time.Second)
	require.EqualValues(t, 1, attempts.Load())
}

func TestRegistryLibraryRetries(t *testing.T) {
	for _, library := range []string{"go-containerregistry", "oras"} {
		for _, throttles := range []int32{1, 3} {
			t.Run(fmt.Sprintf("%s/throttles=%d", library, throttles), func(t *testing.T) {
				var attempts atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/v2/" {
						fmt.Fprint(w, "{}")
						return
					}
					if attempts.Add(1) <= throttles {
						w.Header().Set("Retry-After", "0")
						w.WriteHeader(http.StatusTooManyRequests)
						fmt.Fprint(w, `{"errors":[{"code":"TOOMANYREQUESTS","message":"slow down"}]}`)
						return
					}
					fmt.Fprint(w, `{"name":"image","tags":["latest"]}`)
				}))
				defer server.Close()
				if library == "go-containerregistry" {
					repo, err := name.NewRepository(strings.TrimPrefix(server.URL, "http://") + "/image")
					require.NoError(t, err)
					tags, err := remote.List(repo, remote.WithTransport(httpclient.RegistryTransport))
					if throttles == 1 {
						require.NoError(t, err)
						require.Equal(t, []string{"latest"}, tags)
					} else {
						require.Error(t, err)
					}
				} else {
					client := &auth.Client{Client: &http.Client{Transport: httpclient.RegistryTransport}, Cache: auth.NewCache()}
					req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/image/tags/list", nil)
					require.NoError(t, err)
					resp, err := client.Do(req)
					require.NoError(t, err)
					defer resp.Body.Close()
					if throttles == 1 {
						require.Equal(t, http.StatusOK, resp.StatusCode)
					} else {
						require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
					}
				}
				want := int32(2)
				if throttles == 3 {
					want = 3
				}
				require.Equal(t, want, attempts.Load(), "library retries must not multiply 429 attempts")
			})
		}
	}
}
