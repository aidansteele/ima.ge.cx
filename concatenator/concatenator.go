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
	Key    *bitypes.ImageInfoKey
	Layers []v1.Hash
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
	ctx = logging.WithRequestPayload(ctx, input)
	slog.InfoContext(ctx, "handling concatenator request")

	fileMap := map[string]*layerreader.EntryWithLayer{}
	stored := map[string]*layerreader.EntryWithLayer{}

	for _, hash := range input.Layers {
		w := manager.NewWriteAtBuffer(nil)
		_, err := ll.downloader.Download(ctx, w, &s3.GetObjectInput{
			Bucket: &ll.bucket,
			Key:    aws.String(fmt.Sprintf("layers/%s/files.json.gz", hash)),
		})
		if err != nil {
			return nil, fmt.Errorf("downloading layer files index: %w", err)
		}

		gzr, err := gzip.NewReader(bytes.NewReader(w.Bytes()))
		if err != nil {
			return nil, fmt.Errorf("creating gzip reader: %w", err)
		}

		dec := json.NewDecoder(gzr)
		for {
			e := targzi.Entry{}
			err = dec.Decode(&e)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("unmarshalling entry: %w", err)
			}

			mergeEntry(ctx, fileMap, stored, e, hash.String())
		}
	}

	addDirectorySizes(fileMap)
	// Publish hidden data first so a newly completed index always has both views.
	if _, err := ll.uploadIndex(ctx, input, "hidden-index.json.gz", hiddenEntries(fileMap, stored)); err != nil {
		return nil, err
	}
	upload, err := ll.uploadIndex(ctx, input, "index.json.gz", fileMap)
	if err != nil {
		return nil, err
	}
	return &concatenatorOutput{Key: aws.ToString(upload.Key), VersionId: aws.ToString(upload.VersionID)}, nil
}

func (ll *concatenator) uploadIndex(ctx context.Context, input *concatenatorInput, filename string, fileMap map[string]*layerreader.EntryWithLayer) (*manager.UploadOutput, error) {
	arr := make([]*layerreader.EntryWithLayer, 0, len(fileMap))
	for _, e := range fileMap {
		arr = append(arr, e)
	}

	sort.Slice(arr, func(i, j int) bool {
		return arr[i].Hdr.Name < arr[j].Hdr.Name
	})

	buf := &bytes.Buffer{}
	gzw := gzip.NewWriter(buf)

	// json.Encoder terminates each value with a newline, producing JSON Lines.
	enc := json.NewEncoder(gzw)
	for _, e := range arr {
		err := enc.Encode(e)
		if err != nil {
			return nil, fmt.Errorf("writing combined index entry: %w", err)
		}
	}

	err := gzw.Close()
	if err != nil {
		return nil, fmt.Errorf("closing gzip writer: %w", err)
	}

	upload, err := ll.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: &ll.bucket,
		Key:    aws.String(fmt.Sprintf("images/%s/%s/%s", input.Key.Repo, input.Key.Digest, filename)),
		Body:   buf,
	})
	if err != nil {
		return nil, fmt.Errorf("uploading combined index: %w", err)
	}

	return upload, nil
}

// Keep only the latest stored header for each path, independently of whiteouts.
func mergeEntry(ctx context.Context, final, stored map[string]*layerreader.EntryWithLayer, e targzi.Entry, layer string) {
	if !strings.HasPrefix(filepath.Base(e.Hdr.Name), layerreader.WhiteoutPrefix) {
		stored[strings.TrimSuffix(e.Hdr.Name, "/")] = &layerreader.EntryWithLayer{Entry: e, Layer: layer}
	}
	applyEntry(ctx, final, e, layer)
}

func hiddenEntries(final, stored map[string]*layerreader.EntryWithLayer) map[string]*layerreader.EntryWithLayer {
	present := map[string]bool{}
	for name := range final {
		name = strings.TrimSuffix(name, "/")
		present[name] = true
		for dir := filepath.Dir(name); dir != "." && dir != "/"; dir = filepath.Dir(dir) {
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

// applyEntry merges one entry from layer into fileMap, which holds the
// entries of the layers below it keyed by path. Whiteouts only remove entries
// from lower layers, never entries from the layer that contains them.
func applyEntry(ctx context.Context, fileMap map[string]*layerreader.EntryWithLayer, e targzi.Entry, layer string) {
	base := filepath.Base(e.Hdr.Name)

	switch {
	case base == layerreader.WhiteoutOpaqueDir:
		// An opaque whiteout hides the directory's existing contents but keeps the directory.
		dir := strings.TrimSuffix(e.Hdr.Name, layerreader.WhiteoutOpaqueDir)
		slog.DebugContext(ctx, "processing opaque whiteout directory", "dir", dir)
		for key, entry := range fileMap {
			if key != dir && strings.HasPrefix(key, dir) && entry.Layer != layer {
				delete(fileMap, key)
				slog.DebugContext(ctx, "deleting recursively", "path", key)
			}
		}
	case strings.HasPrefix(base, layerreader.WhiteoutPrefix):
		// A whiteout removes a file, or a directory (keyed with a trailing
		// slash) and everything beneath it.
		target := filepath.Join(filepath.Dir(e.Hdr.Name), strings.TrimPrefix(base, layerreader.WhiteoutPrefix))
		for key, entry := range fileMap {
			if (key == target || strings.HasPrefix(key, target+"/")) && entry.Layer != layer {
				delete(fileMap, key)
				slog.DebugContext(ctx, "deleting whiteout path", "path", key)
			}
		}
	default:
		fileMap[e.Hdr.Name] = &layerreader.EntryWithLayer{Entry: e, Layer: layer}
	}
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
