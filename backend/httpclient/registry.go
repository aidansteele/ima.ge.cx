package httpclient

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RegistryTransport shares DefaultClient's connection pool. Each RoundTrip adds
// at most two retries for HTTP 429, with five seconds of cumulative waiting.
// Retry-After accepts seconds or an HTTP date; missing/invalid values fall back
// to one then two seconds. AWS clients retain their own retry policy.
// Do not mutate it.
var RegistryTransport http.RoundTripper = &registryTransport{base: DefaultClient.Transport}

type registryTransport struct {
	base http.RoundTripper
}

func (t *registryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Do not replay writes (including token POSTs) or requests with bodies.
	if (req.Method != http.MethodGet && req.Method != http.MethodHead) || (req.Body != nil && req.Body != http.NoBody) {
		return t.base.RoundTrip(req)
	}
	budget := 5 * time.Second
	for attempt := 0; ; attempt++ {
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		resp, err := t.base.RoundTrip(req)
		// go-containerregistry v0.10.0 already retries transport errors, but
		// neither it nor our pinned ORAS client retries HTTP 429 responses.
		if err != nil || resp.StatusCode != http.StatusTooManyRequests || attempt == 2 {
			return resp, err
		}
		delay := time.Duration(attempt+1) * time.Second
		value := strings.TrimSpace(resp.Header.Get("Retry-After"))
		if seconds, err := strconv.ParseUint(value, 10, 64); err == nil {
			// Compare before conversion to avoid overflowing time.Duration.
			if seconds > uint64(budget/time.Second) {
				return resp, nil
			}
			delay = time.Duration(seconds) * time.Second
		} else if errors.Is(err, strconv.ErrRange) {
			return resp, nil
		} else if date, err := http.ParseTime(value); err == nil {
			delay = time.Until(date)
			if delay < 0 {
				delay = 0
			}
		}
		// Never shorten a registry's requested delay to fit our budget.
		if delay > budget {
			return resp, nil
		}
		if deadline, ok := req.Context().Deadline(); ok && delay >= time.Until(deadline) {
			return resp, nil
		}
		// Close rather than drain an untrusted error body: draining could add
		// an unbounded transfer to a small retry budget.
		resp.Body.Close()
		timer := time.NewTimer(delay)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		case <-timer.C:
		}
		budget -= delay
	}
}
