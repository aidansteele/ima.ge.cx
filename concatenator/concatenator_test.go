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

func TestAddDirectorySizes(t *testing.T) {
	ctx := context.Background()
	fileMap := map[string]*layerreader.EntryWithLayer{}
	apply := func(layer string, entries ...tar.Header) {
		for _, h := range entries {
			applyEntry(ctx, fileMap, targzi.Entry{Hdr: h}, layer)
		}
	}
	dir := func(name string) tar.Header { return tar.Header{Name: name, Typeflag: tar.TypeDir} }
	file := func(name string, size int64) tar.Header {
		return tar.Header{Name: name, Typeflag: tar.TypeReg, Size: size}
	}

	apply("lower",
		dir("usr/"), dir("usr/lib/"), file("usr/lib/a.so", 1000), file("usr/lib/b.so", 200),
		file("usr/README", 30), tar.Header{Name: "usr/link", Typeflag: tar.TypeSymlink, Linkname: "README"},
		dir("usr/libexec/"), file("usr/libexec/x", 4),
	)
	apply("upper", file("usr/.wh.README", 0), file("usr/lib/c.so", 5))
	addDirectorySizes(fileMap)

	got := func(name string) [2]int64 { return [2]int64{fileMap[name].DirSize, fileMap[name].DirFiles} }
	// The deleted README and the symlink don't count, and usr/lib/ doesn't include usr/libexec/.
	assert.Equal(t, [2]int64{1209, 4}, got("usr/"))
	assert.Equal(t, [2]int64{1205, 3}, got("usr/lib/"))
	assert.Equal(t, [2]int64{4, 1}, got("usr/libexec/"))
	assert.Equal(t, [2]int64{0, 0}, got("usr/lib/a.so"))
}
