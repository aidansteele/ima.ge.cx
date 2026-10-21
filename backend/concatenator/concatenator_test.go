package main

import (
	"archive/tar"
	"browseimage/bitypes"
	"browseimage/httpclient"
	"browseimage/layerreader"
	"browseimage/targzi"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCombinedIndexes(t *testing.T) {
	input := &concatenatorInput{Key: &bitypes.ImageInfoKey{Repo: "repo"}}
	objects := map[string][]byte{}
	for i, names := range [][]string{{"live", "gone", "history", "removed/deep/file"}, {"live", "history"}, {".wh.gone", ".wh.history", ".wh.removed"}, {"tail"}, {"last", ".up/examples.yaml", ".up/nested/other.yaml"}} {
		hash := v1.Hash{Algorithm: "sha256", Hex: fmt.Sprint(i)}
		input.Layers = append(input.Layers, hash)
		key := fmt.Sprintf("exact/%d.json.gz", i)
		input.Indexes = append(input.Indexes, layerreader.RemoteOutput{Tar: layerreader.Put{Key: key}, Gzi: layerreader.Put{Key: fmt.Sprintf("exact/%d.gzi", i)}})
		if i == 0 {
			input.Indexes[i].Tar.VersionId = "version-one"
		}
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		for _, name := range names {
			parent := filepath.Dir(name)
			if parent == "." {
				parent = "/"
			} else {
				parent += "/"
			}
			require.NoError(t, json.NewEncoder(gz).Encode(targzi.Entry{
				Hdr: tar.Header{Name: name, Typeflag: tar.TypeReg, Size: int64(i + 1)}, Parent: parent,
				Offset: 512, Spans: []int{1},
			}))
		}
		require.NoError(t, gz.Close())
		objects["/bucket/"+key] = buf.Bytes()
	}
	later := make(chan struct{})
	firstReleased := make(chan struct{})
	var published []string
	views := map[string]map[string]*layerreader.EntryWithLayer{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			body, ok := objects[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			first := strings.Contains(r.URL.Path, "/4.json.gz")
			second := strings.Contains(r.URL.Path, "/3.json.gz")
			fifth := strings.Contains(r.URL.Path, "/0.json.gz")
			if fifth {
				assert.Equal(t, "version-one", r.URL.Query().Get("versionId"))
				select {
				case <-firstReleased:
				default:
					t.Error("download window advanced past a blocked first layer")
				}
			}
			if first {
				select {
				case <-later:
				case <-time.After(5 * time.Second):
					t.Error("downloads were not parallel")
				}
				time.Sleep(30 * time.Millisecond)
				close(firstReleased)
			}
			w.Header().Set("Content-Length", fmt.Sprint(len(body)))
			_, _ = w.Write(body)
			if second {
				close(later)
			}
			return
		}
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method %s", r.Method)
			w.WriteHeader(400)
			return
		}
		gz, err := gzip.NewReader(r.Body)
		if !assert.NoError(t, err) {
			w.WriteHeader(400)
			return
		}
		defer gz.Close()
		view := map[string]*layerreader.EntryWithLayer{}
		dec := json.NewDecoder(gz)
		var names []string
		for {
			var e layerreader.EntryWithLayer
			err := dec.Decode(&e)
			if err == io.EOF {
				break
			}
			if !assert.NoError(t, err) {
				return
			}
			view[e.Hdr.Name] = &e
			names = append(names, e.Hdr.Name)
		}
		assert.True(t, sort.StringsAreSorted(names))
		name := filepath.Base(r.URL.Path)
		published = append(published, name)
		views[name] = view
		w.Header().Set("x-amz-version-id", "published")
	}))
	defer server.Close()
	api := s3.NewFromConfig(aws.Config{Region: "us-east-1", HTTPClient: httpclient.DefaultClient, Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""), EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(string, string, ...interface{}) (aws.Endpoint, error) { return aws.Endpoint{URL: server.URL}, nil })}, func(o *s3.Options) { o.UsePathStyle = true })
	c := &concatenator{bucket: "bucket", downloader: manager.NewDownloader(api), uploader: manager.NewUploader(api)}
	out, err := c.handle(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "published", out.VersionId)
	assert.Equal(t, []string{"hidden-index.json.gz", "index.json.gz"}, published)
	final, hidden := views["index.json.gz"], views["hidden-index.json.gz"]
	require.Len(t, final, 7)
	require.Len(t, hidden, 5)
	// Crossplane's .up/examples.yaml has no directory header. Published listings
	// must expose every ancestor, without resurrecting whiteouted directories.
	for _, view := range []map[string]*layerreader.EntryWithLayer{final, hidden} {
		for name, entry := range view {
			if entry.Parent == "/" {
				continue
			}
			require.Contains(t, view, entry.Parent, name)
			assert.Equal(t, byte(tar.TypeDir), view[entry.Parent].Hdr.Typeflag)
		}
	}
	assert.NotContains(t, final, "removed/")
	assert.NotContains(t, hidden, ".up/")
	assert.Equal(t, int64(10), final[".up/"].DirSize)
	assert.Equal(t, int64(2), final[".up/"].DirFiles)
	assert.Equal(t, int64(5), final[".up/nested/"].DirSize)
	assert.Empty(t, final[".up/"].Layer, "synthetic directories have no stored layer")
	assert.Equal(t, 512, final[".up/examples.yaml"].Offset)
	assert.Equal(t, []int{1}, final[".up/examples.yaml"].Spans)
	for name, want := range map[string]int{"live": 1, "gone": 0, "history": 1, "tail": 3, "last": 4} {
		e := final[name]
		if e == nil {
			e = hidden[name]
		}
		require.NotNil(t, e, name)
		assert.Equal(t, input.Layers[want].String(), e.Layer)
		assert.Equal(t, int64(want+1), e.Hdr.Size)
		assert.Equal(t, input.Indexes[want].Gzi.Key, e.GzipIndex)
	}
}

func TestHandleRejectsMismatchedIndexes(t *testing.T) {
	c := &concatenator{}
	for _, indexes := range [][]layerreader.RemoteOutput{nil, {}} {
		_, err := c.handle(context.Background(), &concatenatorInput{Layers: []v1.Hash{{}}, Indexes: indexes})
		require.ErrorContains(t, err, "does not match")
	}
}

func TestMergeLayerWhiteouts(t *testing.T) {
	merged := newMergedIndex()
	fileMap := merged.final
	apply := func(layer string, names ...string) {
		var entries []targzi.Entry
		for _, name := range names {
			entries = append(entries, targzi.Entry{Hdr: tar.Header{Name: name}})
		}
		merged.addLayer(entries, layer, "")
	}

	apply("upper",
		"etc/.wh.foo",    // directory whiteout: removes etc/foo/ and its contents only
		"etc/.wh.passwd", // file whiteout
		"opt/-early",     // sorts before the opaque marker in the same layer
		"opt/.wh..wh..opq",
		"opt/late",
	)
	apply("lower",
		"etc/", "etc/foo/", "etc/foo/a.conf", "etc/foobar", "etc/passwd",
		"opt/", "opt/x", "opt/sub/", "opt/sub/y",
	)

	got := make([]string, 0, len(fileMap))
	for name := range fileMap {
		got = append(got, name)
	}
	sort.Strings(got)

	assert.Equal(t, []string{"etc/", "etc/foobar", "opt/", "opt/-early", "opt/late"}, got)
}

func TestAddDirectorySizes(t *testing.T) {
	merged := newMergedIndex()
	fileMap := merged.final
	apply := func(layer string, entries ...tar.Header) {
		var batch []targzi.Entry
		for _, h := range entries {
			batch = append(batch, targzi.Entry{Hdr: h})
		}
		merged.addLayer(batch, layer, "")
	}
	dir := func(name string) tar.Header { return tar.Header{Name: name, Typeflag: tar.TypeDir} }
	file := func(name string, size int64) tar.Header {
		return tar.Header{Name: name, Typeflag: tar.TypeReg, Size: size}
	}

	apply("upper", file("usr/.wh.README", 0), file("usr/lib/c.so", 5))
	apply("lower",
		dir("usr/"), dir("usr/lib/"), file("usr/lib/a.so", 1000), file("usr/lib/b.so", 200),
		file("usr/README", 30), tar.Header{Name: "usr/link", Typeflag: tar.TypeSymlink, Linkname: "README"},
		dir("usr/libexec/"), file("usr/libexec/x", 4),
	)
	addDirectorySizes(fileMap)

	got := func(name string) [2]int64 { return [2]int64{fileMap[name].DirSize, fileMap[name].DirFiles} }
	// The deleted README and the symlink don't count, and usr/lib/ doesn't include usr/libexec/.
	assert.Equal(t, [2]int64{1209, 4}, got("usr/"))
	assert.Equal(t, [2]int64{1205, 3}, got("usr/lib/"))
	assert.Equal(t, [2]int64{4, 1}, got("usr/libexec/"))
	assert.Equal(t, [2]int64{0, 0}, got("usr/lib/a.so"))
}

func TestHiddenEntries(t *testing.T) {
	merged := newMergedIndex()
	final, stored := merged.final, merged.stored
	layers := map[string][]targzi.Entry{}
	add := func(layer, name string, size int64, kind byte) {
		parent := filepath.Dir(strings.TrimSuffix(name, "/"))
		if parent == "." {
			parent = "/"
		} else {
			parent += "/"
		}
		layers[layer] = append(layers[layer], targzi.Entry{
			Hdr:    tar.Header{Name: name, Size: size, Typeflag: kind},
			Offset: 123, Spans: []int{2, 3}, Parent: parent,
		})
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
	for _, layer := range []string{"upper", "middle", "lower"} {
		merged.addLayer(layers[layer], layer, "")
	}
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
