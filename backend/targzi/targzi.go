package targzi

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/klauspost/compress/gzip"
)

type Entry struct {
	Offset int
	Spans  []int
	Hdr    tar.Header
	Parent string
}

type Index struct {
	Entries       []*Entry
	GzIndexPath   string
	FileIndexPath string
}

type IndexSpan struct {
	Number       int
	Uncompressed int
	Compressed   int
}

func (i *Index) Spans() ([]IndexSpan, error) {
	return Spans(i.GzIndexPath)
}

func ReadIndex(dir string) (*Index, error) {
	gzIndexPath := fmt.Sprintf("%s/index.gzi", dir)
	_, err := Spans(gzIndexPath)
	if err != nil {
		return nil, fmt.Errorf("reading index spans: %w", err)
	}

	fileIndexPath := fmt.Sprintf("%s/files.json.gz", dir)
	findex, err := os.Open(fileIndexPath)
	if err != nil {
		return nil, fmt.Errorf("opening file index: %w", err)
	}
	defer findex.Close()

	gzr, err := gzip.NewReader(findex)
	if err != nil {
		return nil, fmt.Errorf("gunzipping file index: %w", err)
	}
	defer gzr.Close()

	entries := []*Entry{}

	dec := json.NewDecoder(gzr)
	for {
		e := Entry{}
		err = dec.Decode(&e)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("unmarshalling entry: %w", err)
		}

		entries = append(entries, &e)
	}

	return &Index{
		Entries:       entries,
		GzIndexPath:   gzIndexPath,
		FileIndexPath: fileIndexPath,
	}, nil
}

func (i *Index) Extract(ctx context.Context, gz io.Reader, skipped, uncompressedOffset, length int) ([]byte, error) {
	return Extract(ctx, gz, i.GzIndexPath, skipped, uncompressedOffset, length)
}

func Extract(ctx context.Context, gz io.Reader, gzIndexPath string, skipped, uncompressedOffset, length int) ([]byte, error) {
	_, seg := xray.BeginSubsegment(ctx, "gztool")
	defer seg.Close(nil)

	cmd := exec.CommandContext(ctx,
		"gztool",
		"-I", gzIndexPath,
		"-n", fmt.Sprintf("%d", skipped),
		"-b", fmt.Sprintf("%d", uncompressedOffset),
		"-r", fmt.Sprintf("%d", length),
	)

	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = gz

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("running gztool: %w", err)
	}

	return stdout.Bytes(), nil
}

func BuildIndex(ctx context.Context, root string, gz io.Reader, fileCounter *int64) (*Index, error) {
	return BuildIndexWithWorkers(ctx, root, gz, fileCounter, 0)
}

func BuildIndexWithWorkers(ctx context.Context, root string, gz io.Reader, fileCounter *int64, workers int) (*Index, error) {
	started := time.Now()
	dir, err := os.MkdirTemp(root, "targzi*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	fileIndexPath := fmt.Sprintf("%s/files.json.gz", dir)
	findex, err := os.Create(fileIndexPath)
	if err != nil {
		return nil, fmt.Errorf("creating file index: %w", err)
	}
	defer findex.Close()

	gzIndexPath := fmt.Sprintf("%s/index.gzi", dir)

	// Tar offsets are byte-based; counting newlines wastes a scan of every byte.
	cmd := exec.CommandContext(ctx, "gztool", "-z", "-I", gzIndexPath, "-b", "0")
	if workers > 0 {
		cmd = exec.CommandContext(ctx, "rapidgzip", "-d", "-c", "-P", strconv.Itoa(workers), "--export-index", gzIndexPath, "--index-format", "gztool")
	}
	cmd.WaitDelay = time.Second
	cmd.Stdin = gz
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("starting gztool: %w", err)
	}
	waited := false
	defer func() {
		stdout.Close()
		if !waited {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()

	entries := []*Entry{}

	// Avoid a pipe syscall for every 512-byte tar header.
	off := &offsetReporter{Reader: bufio.NewReaderSize(stdout, 256<<10)}
	tr := tar.NewReader(off)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return nil, fmt.Errorf("iterating tar file: %w", err)
		}

		hdr.Name = filepath.Clean(hdr.Name)
		if hdr.Name == "." {
			hdr.Name = "/"
		}

		parent := filepath.Dir(hdr.Name)
		if parent == "." {
			parent = "/"
		} else if parent != "/" {
			parent += "/"
		}

		if hdr.Name != "/" && hdr.FileInfo().IsDir() {
			hdr.Name += "/"
		}

		entries = append(entries, &Entry{
			Hdr:    *hdr,
			Offset: off.offset,
			Parent: parent,
		})

		atomic.AddInt64(fileCounter, 1)
	}
	slog.InfoContext(ctx, "read tar headers", "duration", time.Since(started), "entries", len(entries))

	sortStart := time.Now()
	sort.Slice(entries, func(i, j int) bool {
		iname := strings.TrimSuffix(entries[i].Hdr.Name, "/")
		idepth := strings.Count(iname, "/")

		jname := strings.TrimSuffix(entries[j].Hdr.Name, "/")
		jdepth := strings.Count(jname, "/")

		if idepth == jdepth {
			return iname < jname
		}
		return idepth < jdepth
	})
	sortDuration := time.Since(sortStart)

	// The tar reader stops at the end-of-archive marker, but gztool keeps
	// writing any trailing padding. Drain it so gztool can exit; Wait must
	// not be called before all reads from the pipe have completed.
	_, err = io.Copy(io.Discard, off)
	if err != nil {
		return nil, fmt.Errorf("draining gztool output: %w", err)
	}

	err = cmd.Wait()
	waited = true
	if err != nil {
		return nil, fmt.Errorf("gztool exit: %w", err)
	}
	slog.InfoContext(ctx, "finished decoder", "duration", time.Since(started), "sort_duration", sortDuration, "workers", workers)

	metadataStart := time.Now()
	spans, err := Spans(gzIndexPath)
	if err != nil {
		return nil, fmt.Errorf("reading index spans: %w", err)
	}

	sort.Slice(spans, func(i, j int) bool {
		return spans[i].Uncompressed > spans[j].Uncompressed
	})

	for _, entry := range entries {
		firstSpan := sort.Search(len(spans), func(i int) bool {
			return entry.Offset >= spans[i].Uncompressed
		})

		lastSpan := sort.Search(len(spans), func(i int) bool {
			return entry.Offset+int(entry.Hdr.Size) >= spans[i].Uncompressed
		})

		for idx := lastSpan; idx <= firstSpan; idx++ {
			entry.Spans = append(entry.Spans, spans[idx].Number)
		}

		sort.Ints(entry.Spans)
	}

	gzw, err := gzip.NewWriterLevel(findex, gzip.BestSpeed)
	if err != nil {
		return nil, fmt.Errorf("creating file index gzip stream: %w", err)
	}

	// json.Encoder terminates each value with a newline, producing JSON Lines.
	enc := json.NewEncoder(gzw)
	for _, entry := range entries {
		err = enc.Encode(entry)
		if err != nil {
			return nil, fmt.Errorf("writing file index entry: %w", err)
		}
	}

	err = gzw.Close()
	if err != nil {
		return nil, fmt.Errorf("closing file index gzip stream: %w", err)
	}

	err = findex.Close()
	if err != nil {
		return nil, fmt.Errorf("closing file index: %w", err)
	}
	slog.InfoContext(ctx, "encoded layer metadata", "duration", time.Since(metadataStart), "spans", len(spans))

	return &Index{
		Entries:       entries,
		GzIndexPath:   gzIndexPath,
		FileIndexPath: fileIndexPath,
	}, nil
}

// CompressedRange returns the compressed byte offsets needed to extract an
// entry that covers the given index span numbers. start is the compressed
// offset of the first span; end is the compressed offset of the span after
// the last one, or 0 if the entry runs to the end of the file.
func CompressedRange(spans []IndexSpan, entrySpans []int) (start, end int, err error) {
	if len(entrySpans) == 0 {
		return 0, 0, fmt.Errorf("entry has no spans")
	}

	firstSpan := entrySpans[0]
	lastSpan := entrySpans[len(entrySpans)-1]
	foundFirst := false
	for idx, s := range spans {
		if s.Number == firstSpan {
			start = s.Compressed
			foundFirst = true
		}
		if s.Number == lastSpan && idx+1 < len(spans) {
			end = spans[idx+1].Compressed
		}
	}

	if !foundFirst {
		return 0, 0, fmt.Errorf("span %d not found in index", firstSpan)
	}

	return start, end, nil
}

// RangeHeader returns the HTTP Range header value for the compressed bytes
// between start and end, as returned by CompressedRange. gztool numbers
// compressed bytes from 1, so it needs the byte before start too.
func RangeHeader(start, end int) string {
	rangeHdr := fmt.Sprintf("bytes=%d-", start-1)
	if end > 0 {
		rangeHdr += fmt.Sprintf("%d", end)
	}
	return rangeHdr
}

func Spans(path string) ([]IndexSpan, error) {
	stdout := &bytes.Buffer{}

	cmd := exec.Command("gztool", "-ll", path)
	cmd.Stdout = stdout
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("running gztool introspection: %w", err)
	}

	spans := []IndexSpan{}

	re := regexp.MustCompile(`#(\d+): @ (\d+) / (\d+)`)
	matches := re.FindAllStringSubmatch(stdout.String(), -1)
	for _, m := range matches {
		number, _ := strconv.Atoi(m[1])
		compressed, _ := strconv.Atoi(m[2])
		uncompressed, _ := strconv.Atoi(m[3])

		spans = append(spans, IndexSpan{
			Number:       number,
			Uncompressed: uncompressed,
			Compressed:   compressed,
		})
	}

	return spans, nil
}

type offsetReporter struct {
	io.Reader
	offset int
}

func (o *offsetReporter) Read(p []byte) (n int, err error) {
	n, err = o.Reader.Read(p)
	o.offset += n
	return
}
