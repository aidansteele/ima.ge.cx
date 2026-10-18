package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// serveWeb implements the native HTTP lifecycle after index.js replaces Node.
// The readiness API is an internal contract, not an official Go runtime API.
func serveWeb(ctx context.Context, handler http.Handler, endpoint, runtimeAPI string) error {
	listener, err := net.Listen("tcp", endpoint)
	if err != nil {
		return err
	}
	defer listener.Close()

	// execve removed the RIC, so we must signal readiness ourselves. Bound this
	// call so a broken runtime API cannot leave a permanently half-started server.
	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(readyCtx, http.MethodPost,
		"http://"+runtimeAPI+"/2025-09-01/ready", strings.NewReader("{}"))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("announcing readiness: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("announcing readiness: %s", resp.Status)
	}

	server := &http.Server{Handler: webHandler(handler), ReadHeaderTimeout: 5 * time.Second}
	// Wait for Shutdown, not just Serve: Serve returns as soon as the listener
	// closes, before in-flight requests have finished draining.
	done := make(chan struct{})
	stopCtx, stop := context.WithCancel(ctx)
	defer stop()
	go func() {
		defer close(done)
		<-stopCtx.Done()
		drainCtx, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
		defer cancel()
		if err := server.Shutdown(drainCtx); err != nil {
			server.Close()
		}
	}()
	slog.Info("web server ready", "address", listener.Addr().String())
	err = server.Serve(listener)
	stop()
	<-done
	slog.Info("web server stopped")
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func webHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Function URLs supplied CORS before; native HTTP must do it itself.
		// Preserve the public API's allow-any-origin, credentialed policy.
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", r.Header.Get("Access-Control-Request-Method"))
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		// Native HTTP lacks the Event Function invocation deadline. Propagate a
		// deadline to AWS/registry requests and extraction subprocesses ourselves.
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
