package main

import (
	"archive/tar"
	"browseimage/layerreader"
	"browseimage/targzi"
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyEntryWhiteouts(t *testing.T) {
	ctx := context.Background()
	fileMap := map[string]*layerreader.EntryWithLayer{}
	apply := func(layer string, names ...string) {
		for _, name := range names {
			applyEntry(ctx, fileMap, targzi.Entry{Hdr: tar.Header{Name: name}}, layer)
		}
	}

	apply("lower",
		"etc/", "etc/foo/", "etc/foo/a.conf", "etc/foobar", "etc/passwd",
		"opt/", "opt/x", "opt/sub/", "opt/sub/y",
	)
	apply("upper",
		"etc/.wh.foo",    // directory whiteout: removes etc/foo/ and its contents only
		"etc/.wh.passwd", // file whiteout
		"opt/-early",     // sorts before the opaque marker in the same layer
		"opt/.wh..wh..opq",
		"opt/late",
	)

	got := make([]string, 0, len(fileMap))
	for name := range fileMap {
		got = append(got, name)
	}
	sort.Strings(got)

	assert.Equal(t, []string{"etc/", "etc/foobar", "opt/", "opt/-early", "opt/late"}, got)
}
