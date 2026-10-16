// Package httpclient bounds stalled requests without limiting the total duration
// of healthy registry and S3 transfers.
package httpclient

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// DefaultClient is shared to retain connection pooling. Do not mutate it.
var DefaultClient = &http.Client{Transport: newTransport(10*time.Second, 30*time.Second)}

func newTransport(headers, idle time.Duration) http.RoundTripper {
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.DialContext = (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	base.TLSHandshakeTimeout = 10 * time.Second
	base.ResponseHeaderTimeout = headers
	return &transport{base: base, idle: idle}
}

type transport struct {
	base *http.Transport
	idle time.Duration
}

func (t *transport) CloseIdleConnections() {
	t.base.CloseIdleConnections()
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancelCause(req.Context())
	// Implement net.Error directly: url.Error.Timeout does not unwrap errors.
	timeout := &net.OpError{Op: "HTTP transfer idle for " + t.idle.String(), Net: "http", Err: os.ErrDeadlineExceeded}
	var mu sync.Mutex
	deadline := time.Now().Add(t.idle)
	stopped := false
	var timer *time.Timer
	// A callback may already be waiting for the mutex when a read resets the
	// timer. Check the latest deadline rather than cancelling an active transfer.
	mu.Lock()
	timer = time.AfterFunc(t.idle, func() {
		mu.Lock()
		defer mu.Unlock()
		if stopped {
			return
		}
		if remaining := time.Until(deadline); remaining > 0 {
			timer.Reset(remaining)
			return
		}
		stopped = true
		cancel(timeout)
	})
	mu.Unlock()
	progress := func() {
		mu.Lock()
		defer mu.Unlock()
		if !stopped {
			deadline = time.Now().Add(t.idle)
			timer.Reset(t.idle)
		}
	}
	finish := func() {
		mu.Lock()
		stopped = true
		timer.Stop()
		mu.Unlock()
		cancel(nil)
	}
	req = req.Clone(ctx)
	if req.Body != nil && req.Body != http.NoBody {
		req.Body = &body{ReadCloser: req.Body, progress: progress, ctx: ctx, timeout: timeout}
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		cause := context.Cause(ctx)
		finish()
		if cause == timeout {
			return nil, timeout
		}
		return nil, err
	}
	progress()
	resp.Body = &body{ReadCloser: resp.Body, progress: progress, finish: finish, ctx: ctx, timeout: timeout}
	return resp, nil
}

type body struct {
	io.ReadCloser
	progress func()
	finish   func()
	ctx      context.Context
	timeout  error
}

func (b *body) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.progress()
	}
	if context.Cause(b.ctx) == b.timeout {
		err = b.timeout
	}
	if err != nil && b.finish != nil {
		b.finish()
	}
	return n, err
}

func (b *body) Close() error {
	if b.finish != nil {
		b.finish()
	}
	return b.ReadCloser.Close()
}
