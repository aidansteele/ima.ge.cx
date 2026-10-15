package main

import (
	"archive/tar"
	"browseimage/layerreader"
	"browseimage/targzi"
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestHiddenEntries(t *testing.T) {
	final := map[string]*layerreader.EntryWithLayer{}
	stored := map[string]*layerreader.EntryWithLayer{}
	add := func(layer, name string, size int64, kind byte) {
		parent := filepath.Dir(strings.TrimSuffix(name, "/"))
		if parent == "." {
			parent = "/"
		} else {
			parent += "/"
		}
		mergeEntry(context.Background(), final, stored, targzi.Entry{
			Hdr:    tar.Header{Name: name, Size: size, Typeflag: kind},
			Offset: 123, Spans: []int{2, 3}, Parent: parent,
		}, layer)
	}
	for _, name := range []string{"gone/deep/file", "gone/link", "gone2/live", "opaque/old", "opaque2/live", "recreated", "overwritten", "same", "latest"} {
		add("lower", name, 7, tar.TypeReg)
	}
	add("lower", "empty/", 0, tar.TypeDir)
	add("middle", "latest", 11, tar.TypeReg)
	add("middle", "gone/link", 0, tar.TypeSymlink)
	add("upper", "same", 9, tar.TypeReg)
	add("upper", "opaque/early", 90, tar.TypeReg)
	for _, name := range []string{".wh.gone", "opaque/.wh..wh..opq", ".wh.recreated", ".wh.same", ".wh.latest", ".wh.empty"} {
		add("upper", name, 0, tar.TypeReg)
	}
	add("upper", "opaque/new", 100, tar.TypeReg)
	add("upper", "recreated", 20, tar.TypeReg)
	add("upper", "overwritten", 30, tar.TypeReg)
	hidden := hiddenEntries(final, stored)
	require.Len(t, hidden, 7)
	for _, name := range []string{"gone/deep/file", "gone/link", "opaque/old", "latest", "gone/", "gone/deep/", "opaque/"} {
		require.Contains(t, hidden, name)
	}
	assert.Equal(t, int64(7), hidden["gone/"].DirSize)
	assert.Equal(t, int64(1), hidden["gone/"].DirFiles)
	assert.Equal(t, int64(7), hidden["gone/deep/"].DirSize)
	assert.Equal(t, "/", hidden["gone/"].Parent)
	assert.Equal(t, "gone/", hidden["gone/deep/"].Parent)
	assert.Equal(t, "gone/deep/", hidden["gone/deep/file"].Parent)
	assert.Equal(t, int64(7), hidden["opaque/"].DirSize) // excludes live files
	assert.Equal(t, "middle", hidden["latest"].Layer)
	assert.Equal(t, int64(11), hidden["latest"].Hdr.Size)
	assert.Equal(t, 123, hidden["latest"].Offset)
	assert.Equal(t, []int{2, 3}, hidden["latest"].Spans)
	assert.Contains(t, final, "gone2/live")
	assert.Contains(t, final, "opaque/early")
	assert.Contains(t, final, "opaque2/live")
	assert.Contains(t, final, "same")
}
