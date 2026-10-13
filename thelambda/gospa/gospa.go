package gospa

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
)

func Handler(fsys fs.FS, indexPath string) (http.Handler, error) {
	body, err := fs.ReadFile(fsys, indexPath)
	if err != nil {
		return nil, fmt.Errorf("reading index: %w", err)
	}

	stat, err := fs.Stat(fsys, indexPath)
	if err != nil {
		return nil, fmt.Errorf("getting index stats: %w", err)
	}

	return intercept404(http.FileServer(http.FS(fsys)), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		// ServeContent seeks the reader, so each request needs its own.
		http.ServeContent(w, r, indexPath, stat.ModTime(), bytes.NewReader(body))
	})), nil
}

type hookedResponseWriter struct {
	http.ResponseWriter
	got404 bool
}

func (hrw *hookedResponseWriter) WriteHeader(status int) {
	if status == http.StatusNotFound {
		// Don't actually write the 404 header, just set a flag.
		hrw.got404 = true
	} else {
		hrw.ResponseWriter.WriteHeader(status)
	}
}

func (hrw *hookedResponseWriter) Write(p []byte) (int, error) {
	if hrw.got404 {
		// No-op, but pretend that we wrote len(p) bytes to the writer.
		return len(p), nil
	}

	return hrw.ResponseWriter.Write(p)
}

func intercept404(handler, on404 http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hookedWriter := &hookedResponseWriter{ResponseWriter: w}
		handler.ServeHTTP(hookedWriter, r)

		if hookedWriter.got404 {
			on404.ServeHTTP(w, r)
		}
	})
}
