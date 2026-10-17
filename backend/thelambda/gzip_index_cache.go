package main

import (
	"container/list"
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const gzipIndexCacheBytes = 512 << 20

// gzipIndexCache owns its directory. Entries remain pinned until the caller
// releases them, since gztool opens the pathname more than once during a read.
type gzipIndexCache struct {
	mu      sync.Mutex
	dir     string
	limit   int64
	used    int64
	entries map[string]*list.Element
	recency list.List
}

type cachedGzipIndex struct {
	key, path string
	size      int64
	users     int
}

func newGzipIndexCache(dir string, limit int64) (*gzipIndexCache, error) {
	// Lambda can preserve /tmp across a runtime reset. Discard files whose
	// accounting was lost with the previous process, including partial downloads.
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &gzipIndexCache{dir: dir, limit: limit, entries: make(map[string]*list.Element)}, nil
}

// acquire returns a complete local index and a release function. Downloads are
// serialized, but callers can use pinned files concurrently (e.g. local HTTP).
func (c *gzipIndexCache) acquire(ctx context.Context, client *s3.Client, bucket, key string) (string, func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cacheKey := bucket + "\x00" + key
	element := c.entries[cacheKey]
	if element != nil {
		return c.pin(element)
	}
	get, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		return "", nil, fmt.Errorf("getting gzip index: %w", err)
	}
	defer get.Body.Close()

	size := get.ContentLength
	if size < 0 || size > c.limit {
		return "", nil, fmt.Errorf("gzip index size %d exceeds cache capacity %d", size, c.limit)
	}
	// Reserve space before writing, not after a download has filled /tmp.
	for c.used+size > c.limit {
		victim := c.recency.Back()
		for victim != nil && victim.Value.(*cachedGzipIndex).users != 0 {
			victim = victim.Prev()
		}
		if victim == nil {
			return "", nil, fmt.Errorf("gzip index cache is full with files in use")
		}
		entry := victim.Value.(*cachedGzipIndex)
		if err := os.Remove(entry.path); err != nil {
			return "", nil, fmt.Errorf("evicting gzip index: %w", err)
		}
		c.used -= entry.size
		delete(c.entries, entry.key)
		c.recency.Remove(victim)
	}

	f, err := os.CreateTemp(c.dir, "gzi-")
	if err != nil {
		return "", nil, fmt.Errorf("creating gzip index file: %w", err)
	}
	// Do not publish a partial file, or write more than the reserved size.
	_, err = io.CopyN(f, get.Body, size)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("writing gzip index file: %w", err)
	}
	entry := &cachedGzipIndex{key: cacheKey, path: f.Name(), size: size}
	element = c.recency.PushFront(entry)
	c.entries[cacheKey] = element
	c.used += size
	return c.pin(element)
}

// pin is called with mu held.
func (c *gzipIndexCache) pin(element *list.Element) (string, func(), error) {
	entry := element.Value.(*cachedGzipIndex)
	entry.users++
	c.recency.MoveToFront(element)
	return entry.path, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		entry.users--
	}, nil
}
