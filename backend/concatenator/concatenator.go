package main

import (
	"archive/tar"
	"browseimage/bitypes"
	"browseimage/layerreader"
	"browseimage/logging"
	"browseimage/targzi"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/klauspost/compress/gzip"
)

func main() {
	logging.Init()

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(fmt.Sprintf("%+v", err))
	}

	api := s3.NewFromConfig(cfg)

	c := &concatenator{
		uploader:   manager.NewUploader(api),
		downloader: manager.NewDownloader(api),
		bucket:     os.Getenv("BUCKET"),
	}
	lambda.Start(logging.Middleware(c.handle))
}

type concatenatorInput struct {
	Key     *bitypes.ImageInfoKey
	Layers  []v1.Hash
	Indexes []layerreader.RemoteOutput
}

type concatenatorOutput struct {
	Key       string
	VersionId string
}

type concatenator struct {
	uploader   *manager.Uploader
	downloader *manager.Downloader
	bucket     string
}

func (ll *concatenator) handle(ctx context.Context, input *concatenatorInput) (*concatenatorOutput, error) {
	started := time.Now()
	if input.Indexes != nil && len(input.Indexes) != len(input.Layers) {
		return nil, fmt.Errorf("indexes length %d does not match layers length %d", len(input.Indexes), len(input.Layers))
	}
	ctx = logging.WithAttrs(ctx, "repo", input.Key.Repo, "digest", input.Key.Digest)
	slog.InfoContext(ctx, "handling concatenator request")
	ctx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()

	merged := newMergedIndex()
	fileMap := merged.final

	// A sliding window, rather than an unrestricted worker queue: even when
	// the first layer is slow, at most four decoded layers are resident ahead.
	type result struct {
		entries []targzi.Entry
		err     error
	}
	const window = 4
	var pending [window]chan result
	start := func(i int) {
		ch := make(chan result, 1)
		pending[i%window] = ch
		workers.Add(1)
		go func() {
			defer workers.Done()
			entries, err := ll.downloadIndex(ctx, input, len(input.Layers)-1-i)
			ch <- result{entries, err}
		}()
	}
	for i := 0; i < window && i < len(input.Layers); i++ {
		start(i)
	}
	var mergeDuration time.Duration
	for i := range input.Layers {
		r := <-pending[i%window]
		if r.err != nil {
			return nil, r.err
		}
		mergeStart := time.Now()
		layer := len(input.Layers) - 1 - i
		gzi := ""
		if input.Indexes != nil {
			gzi = input.Indexes[layer].Gzi.Key
		}
		merged.addLayer(r.entries, input.Layers[layer].String(), gzi)
		mergeDuration += time.Since(mergeStart)
		r.entries = nil
		if i+window < len(input.Layers) {
			start(i + window)
		}
	}
	slog.InfoContext(ctx, "combined layer indexes", "duration", time.Since(started), "merge_duration", mergeDuration, "layers", len(input.Layers))

	sizeStart := time.Now()
	addDirectorySizes(fileMap)
	slog.InfoContext(ctx, "sized final directories", "duration", time.Since(sizeStart))
	hiddenStart := time.Now()
	hidden := hiddenEntries(fileMap, merged.stored)
	slog.InfoContext(ctx, "built hidden index", "duration", time.Since(hiddenStart), "entries", len(hidden))
	// Publish hidden data first so a newly completed index always has both views.
	if _, err := ll.uploadIndex(ctx, input, "hidden-index.json.gz", hidden); err != nil {
		return nil, err
	}
	upload, err := ll.uploadIndex(ctx, input, "index.json.gz", fileMap)
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "completed concatenation", "duration", time.Since(started))
	return &concatenatorOutput{Key: aws.ToString(upload.Key), VersionId: aws.ToString(upload.VersionID)}, nil
}

func (ll *concatenator) downloadIndex(ctx context.Context, input *concatenatorInput, i int) ([]targzi.Entry, error) {
	started := time.Now()
	key := fmt.Sprintf("layers/%s/files.json.gz", input.Layers[i])
	var version *string
	if input.Indexes != nil {
		key = input.Indexes[i].Tar.Key
		if input.Indexes[i].Tar.VersionId != "" {
			version = aws.String(input.Indexes[i].Tar.VersionId)
		}
	}
	w := manager.NewWriteAtBuffer(nil)
	if _, err := ll.downloader.Download(ctx, w, &s3.GetObjectInput{Bucket: &ll.bucket, Key: &key, VersionId: version}); err != nil {
		return nil, fmt.Errorf("downloading layer %d files index: %w", i, err)
	}
	downloadDuration := time.Since(started)
	gzr, err := gzip.NewReader(bytes.NewReader(w.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gzr.Close()
	decodeStart := time.Now()
	var entries []targzi.Entry
	dec := json.NewDecoder(gzr)
	for {
		var e targzi.Entry
		if err := dec.Decode(&e); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("unmarshalling layer %d entry: %w", i, err)
		}
		entries = append(entries, e)
	}
	slog.InfoContext(ctx, "read layer index", "layer", i, "download_duration", downloadDuration, "decode_duration", time.Since(decodeStart), "entries", len(entries))
	return entries, nil
}

func (ll *concatenator) uploadIndex(ctx context.Context, input *concatenatorInput, filename string, fileMap map[string]*layerreader.EntryWithLayer) (*manager.UploadOutput, error) {
	started := time.Now()
	arr := make([]*layerreader.EntryWithLayer, 0, len(fileMap))
	for _, e := range fileMap {
		arr = append(arr, e)
	}

	sort.Slice(arr, func(i, j int) bool {
		return arr[i].Hdr.Name < arr[j].Hdr.Name
	})

	buf := &bytes.Buffer{}
	gzw, err := gzip.NewWriterLevel(buf, gzip.BestSpeed)
	if err != nil {
		return nil, fmt.Errorf("creating gzip writer: %w", err)
	}

	// json.Encoder terminates each value with a newline, producing JSON Lines.
	enc := json.NewEncoder(gzw)
	for _, e := range arr {
		err := enc.Encode(e)
		if err != nil {
			return nil, fmt.Errorf("writing combined index entry: %w", err)
		}
	}

	err = gzw.Close()
	if err != nil {
		return nil, fmt.Errorf("closing gzip writer: %w", err)
	}

	encodeDuration := time.Since(started)
	uploadStart := time.Now()
	upload, err := ll.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: &ll.bucket,
		Key:    aws.String(fmt.Sprintf("images/%s/%s/%s", input.Key.Repo, input.Key.Digest, filename)),
		Body:   buf,
	})
	if err != nil {
		return nil, fmt.Errorf("uploading combined index: %w", err)
	}
	slog.InfoContext(ctx, "published combined index", "filename", filename, "encode_duration", encodeDuration, "upload_duration", time.Since(uploadStart), "entries", len(arr))

	return upload, nil
}

type mergedIndex struct {
	final, stored   map[string]*layerreader.EntryWithLayer
	targets, opaque map[string]bool
}

func newMergedIndex() *mergedIndex {
	return &mergedIndex{
		final: map[string]*layerreader.EntryWithLayer{}, stored: map[string]*layerreader.EntryWithLayer{},
		targets: map[string]bool{}, opaque: map[string]bool{},
	}
}

// Visit layers newest-first: each path is considered once per stored header,
// rather than rescanning the accumulated filesystem for every whiteout.
func (m *mergedIndex) addLayer(entries []targzi.Entry, layer, gzipIndex string) {
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		path := e.Hdr.Name
		if strings.HasPrefix(filepath.Base(path), layerreader.WhiteoutPrefix) {
			continue
		}
		// Hidden mode needs the latest stored header even if a whiteout hides it.
		storedPath := strings.TrimSuffix(path, "/")
		if m.stored[storedPath] == nil {
			m.stored[storedPath] = &layerreader.EntryWithLayer{Entry: e, Layer: layer, GzipIndex: gzipIndex}
		}
		if m.final[path] != nil {
			continue
		}
		removed := m.targets[path] || (m.opaque[""] && path != "")
		for j := 0; !removed && j < len(path); j++ {
			if path[j] == '/' {
				removed = m.targets[path[:j]] || (m.opaque[path[:j+1]] && j+1 < len(path))
			}
		}
		if !removed {
			m.final[path] = &layerreader.EntryWithLayer{Entry: e, Layer: layer, GzipIndex: gzipIndex}
		}
	}
	// Whiteouts affect only lower layers, never their own layer's entries.
	for _, e := range entries {
		base := filepath.Base(e.Hdr.Name)
		if base == layerreader.WhiteoutOpaqueDir {
			m.opaque[strings.TrimSuffix(e.Hdr.Name, layerreader.WhiteoutOpaqueDir)] = true
		} else if strings.HasPrefix(base, layerreader.WhiteoutPrefix) {
			m.targets[filepath.Join(filepath.Dir(e.Hdr.Name), strings.TrimPrefix(base, layerreader.WhiteoutPrefix))] = true
		}
	}
}

func hiddenEntries(final, stored map[string]*layerreader.EntryWithLayer) map[string]*layerreader.EntryWithLayer {
	present := map[string]bool{}
	for name := range final {
		name = strings.TrimSuffix(name, "/")
		present[name] = true
		for dir := filepath.Dir(name); dir != "." && dir != "/"; dir = filepath.Dir(dir) {
			if present[dir] {
				break // Its ancestors were already marked by an earlier path.
			}
			present[dir] = true
		}
	}
	hidden := map[string]*layerreader.EntryWithLayer{}
	for path, entry := range stored {
		// Directories are included only as navigation to hidden non-directory entries.
		if present[path] || entry.Hdr.Typeflag == tar.TypeDir {
			continue
		}
		copy := *entry
		hidden[entry.Hdr.Name] = &copy
		for dir := filepath.Dir(path); dir != "." && dir != "/"; dir = filepath.Dir(dir) {
			name := dir + "/"
			if _, ok := hidden[name]; ok {
				continue
			}
			parent := filepath.Dir(dir)
			if parent == "." {
				parent = "/"
			} else if parent != "/" {
				parent += "/"
			}
			hidden[name] = &layerreader.EntryWithLayer{Entry: targzi.Entry{
				Hdr: tar.Header{Name: name, Typeflag: tar.TypeDir}, Parent: parent,
			}}
		}
	}
	addDirectorySizes(hidden)
	return hidden
}

// addDirectorySizes sets each directory's DirSize and DirFiles to the totals
// of the regular files beneath it. Directory names end with "/", and fileMap
// already reflects whiteouts, so deleted files aren't counted.
func addDirectorySizes(fileMap map[string]*layerreader.EntryWithLayer) {
	type total struct{ size, files int64 }
	totals := map[string]*total{}

	for name, e := range fileMap {
		if e.Hdr.Typeflag != tar.TypeReg && e.Hdr.Typeflag != tar.TypeRegA {
			continue
		}
		for i := strings.LastIndex(name, "/"); i >= 0; i = strings.LastIndex(name[:i], "/") {
			dir := name[:i+1]
			t := totals[dir]
			if t == nil {
				t = &total{}
				totals[dir] = t
			}
			t.size += e.Hdr.Size
			t.files++
		}
	}

	for dir, t := range totals {
		if e, ok := fileMap[dir]; ok {
			e.DirSize, e.DirFiles = t.size, t.files
		}
	}
}
