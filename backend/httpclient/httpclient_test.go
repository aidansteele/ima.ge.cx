package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
)

func testClient(t *testing.T, h2 bool, handler http.HandlerFunc, headers, idle time.Duration) (*http.Client, string) {
	t.Helper()
	s := httptest.NewUnstartedServer(handler)
	s.EnableHTTP2 = h2
	s.StartTLS()
	t.Cleanup(s.Close)
	transport := newTransport(headers, idle).(*transport)
	transport.base.TLSClientConfig = s.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	return client, s.URL
}

func TestStalledTransfers(t *testing.T) {
	for _, h2 := range []bool{false, true} {
		for _, stage := range []string{"headers", "download", "upload"} {
			t.Run(fmt.Sprintf("http2=%t/%s", h2, stage), func(t *testing.T) {
				release := make(chan struct{})
				headers, idle := time.Second, 100*time.Millisecond
				if stage == "headers" {
					headers, idle = 100*time.Millisecond, time.Second
				}
				client, url := testClient(t, h2, func(w http.ResponseWriter, r *http.Request) {
					if stage == "download" {
						io.WriteString(w, "partial")
						w.(http.Flusher).Flush()
					}
					<-release
				}, headers, idle)
				t.Cleanup(func() { close(release) })
				var upload io.Reader
				if stage == "upload" {
					// Larger than socket and HTTP/2 flow-control buffers. The
					// server never reads it, so sending must eventually stall.
					upload = io.LimitReader(zeros{}, 1<<30)
				}
				req, err := http.NewRequest(http.MethodPost, url, upload)
				if err != nil {
					t.Fatal(err)
				}
				started := time.Now()
				resp, err := client.Do(req)
				if err == nil {
					defer resp.Body.Close()
					_, err = io.ReadAll(resp.Body)
				}
				var timeout net.Error
				if !errors.As(err, &timeout) || !timeout.Timeout() {
					t.Fatalf("expected a retryable timeout, got %v", err)
				}
				if !retry.NewStandard().IsErrorRetryable(err) {
					t.Fatalf("AWS SDK does not recognize timeout as retryable: %v", err)
				}
				if stage != "headers" && !errors.Is(err, os.ErrDeadlineExceeded) {
					t.Fatalf("expected transfer idle timeout, got %v", err)
				}
				if elapsed := time.Since(started); elapsed >= time.Second {
					t.Fatalf("timeout took %s, expected well below 1s", elapsed)
				}
			})
		}
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestHealthyStreaming(t *testing.T) {
	for _, h2 := range []bool{false, true} {
		t.Run(fmt.Sprintf("http2=%t", h2), func(t *testing.T) {
			client, url := testClient(t, h2, func(w http.ResponseWriter, r *http.Request) {
				data, err := io.ReadAll(r.Body)
				if err != nil || string(data) != strings.Repeat("upload", 8) {
					http.Error(w, "incomplete upload", http.StatusBadRequest)
					return
				}
				for i := 0; i < 8; i++ {
					io.WriteString(w, "download")
					w.(http.Flusher).Flush()
					time.Sleep(50 * time.Millisecond)
				}
			}, time.Second, 200*time.Millisecond)
			reader, writer := io.Pipe()
			defer reader.Close()
			go func() {
				defer writer.Close()
				for i := 0; i < 8; i++ {
					if _, err := io.WriteString(writer, "upload"); err != nil {
						return
					}
					time.Sleep(50 * time.Millisecond)
				}
			}()
			resp, err := client.Post(url, "application/octet-stream", reader)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil || string(data) != strings.Repeat("download", 8) {
				t.Fatalf("incomplete response: %q, %v", data, err)
			}
			if (resp.ProtoMajor == 2) != h2 {
				t.Fatalf("unexpected protocol: %s", resp.Proto)
			}
		})
	}
}

func TestHTTP2TimeoutIsPerStream(t *testing.T) {
	client, url := testClient(t, true, func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		if r.URL.Path == "/stalled" {
			<-r.Context().Done()
			return
		}
		for i := 0; i < 12; i++ {
			io.WriteString(w, "ok")
			w.(http.Flusher).Flush()
			time.Sleep(50 * time.Millisecond)
		}
	}, time.Second, 200*time.Millisecond)
	stalled, err := client.Get(url + "/stalled")
	if err != nil {
		t.Fatal(err)
	}
	defer stalled.Body.Close()
	healthy, err := client.Get(url + "/healthy")
	if err != nil {
		t.Fatal(err)
	}
	defer healthy.Body.Close()
	done := make(chan error, 1)
	go func() {
		_, err := io.ReadAll(stalled.Body)
		done <- err
	}()
	data, err := io.ReadAll(healthy.Body)
	if err != nil || string(data) != strings.Repeat("ok", 12) {
		t.Fatalf("timeout interrupted healthy stream: %q, %v", data, err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("expected stalled stream to time out: %v", err)
		}
	default:
		t.Fatal("traffic on healthy stream kept stalled stream alive")
	}
}

func TestCallerCancellation(t *testing.T) {
	client, url := testClient(t, true, func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}, time.Second, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	cancel()
	_, err = io.ReadAll(resp.Body)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("caller cancellation changed to %v", err)
	}
}
