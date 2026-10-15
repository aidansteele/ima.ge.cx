package targzi

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestIndexRangeRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("gztool"); err != nil {
		t.Skip("requires gztool on PATH (run with the built Linux layer)")
	}
	large := make([]byte, 24<<20)
	rand.New(rand.NewSource(42)).Read(large)
	for i := range large {
		large[i] &= 31 // Dynamic Huffman blocks, not only incompressible stored blocks.
	}
	files := []struct {
		name string
		body []byte
	}{{"large.bin", large}, {"pax/" + strings.Repeat("long", 40), []byte("after multiple checkpoints")}, {"empty", nil}}
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: f.name, Size: int64(len(f.body)), Mode: 0644}))
		_, err := tw.Write(f.body)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	_, err := gz.Write(make([]byte, 512<<10)) // Padding after the tar EOF must be drained.
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	var count int64
	// Hide seekability to exercise the HTTP/stdin path.
	idx, err := BuildIndex(context.Background(), t.TempDir(), struct{ io.Reader }{bytes.NewReader(compressed.Bytes())}, &count)
	require.NoError(t, err)
	require.Equal(t, int64(len(files)), count)
	spans, err := idx.Spans()
	require.NoError(t, err)
	require.Greater(t, len(spans), 1)
	for _, entry := range idx.Entries {
		var expected []byte
		for _, f := range files {
			if f.name == entry.Hdr.Name {
				expected = f.body
			}
		}
		require.Equal(t, int64(len(expected)), entry.Hdr.Size)
		if entry.Hdr.Size == 0 {
			continue
		}
		start, end, err := CompressedRange(spans, entry.Spans)
		require.NoError(t, err)
		if end == 0 || end >= compressed.Len() {
			end = compressed.Len() - 1
		}
		got, err := idx.Extract(context.Background(), bytes.NewReader(compressed.Bytes()[start-1:end+1]), start, entry.Offset, int(entry.Hdr.Size))
		require.NoError(t, err)
		require.True(t, bytes.Equal(expected, got), "range extraction differs for %s", entry.Hdr.Name)
	}
}

func TestFileContents(t *testing.T) {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile("ak2-dev"))
	require.NoError(t, err)

	api := s3.NewFromConfig(cfg)
	te := &TarExplorer{
		s3:         api,
		downloader: manager.NewDownloader(api),
		bucket:     "browseimage-bucket-1oax5dhlqcoxp",
	}

	path := "etc/apt/apt.conf.d/docker-autoremove-suggests"
	//path := "usr/share/common-licenses/LGPL-2.1"
	contents, err := te.FileContents(ctx, "layers/sha256:a603fa5e3b4127f210503aaa6189abf6286ee5a73deeaab460f8f33ebc6b64e2", path)
	require.NoError(t, err)

	fmt.Println(string(contents))
}

func TestSelect(t *testing.T) {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile("ak2-dev"))
	require.NoError(t, err)

	dl := &TarExplorer{
		s3:     s3.NewFromConfig(cfg),
		bucket: "browseimage-bucket-1oax5dhlqcoxp",
	}

	entries, err := dl.ListDirectory(ctx, "test", "usr/bin")
	require.NoError(t, err)

	for _, entry := range entries {
		spew.Dump(entry.Hdr.Name)
	}
}

func TestDockerHub(t *testing.T) {
	refstr := "jetbrains/teamcity-server@sha256:f0f0b6cdae5755ca117c8d0c1f1405d9a2207ecddbb7f91864b4d11764fb5cd8"
	ref, err := name.ParseReference(refstr)
	require.NoError(t, err)

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithTransport(&transport{RoundTripper: http.DefaultTransport}))
	require.NoError(t, err)

	layers, err := img.Layers()
	require.NoError(t, err)

	for _, layer := range layers {
		digest, err := layer.Digest()
		require.NoError(t, err)

		fmt.Println(digest)
	}
}

//func TestRemote(t *testing.T) {
//	ctx := context.Background()
//
//	cfg, err := config.LoadDefaultConfig(ctx,
//		config.WithSharedConfigProfile("ak2-dev"),
//		//config.WithClientLogMode(aws.LogRequest|aws.LogResponse),
//	)
//	require.NoError(t, err)
//
//	hashes := []string{
//		"a603fa5e3b4127f210503aaa6189abf6286ee5a73deeaab460f8f33ebc6b64e2",
//		"478909de3dddf82d41bf49336ca75e99dda4b994ff95f8b3c7f9929eccf5bd9c",
//		"c6ca03fe204049fa50d3c113d614872b39a55329ae8bd6014f62c180a42d7bed",
//		"d954040416f1deb0cff7c8e72b661d12c8f48b505cf3a5ed6cd1a1d83c88db52",
//		"27c35e965fb9d5f4d256f54dd88d77fbff7e44918e99578781807ecd5678678f",
//		"a00ba28ea36df33d54581bfc3079bd73bd4c828db5dc23247be44835f76532eb",
//		"3657b76c00c1e959659153ac3fc1d1aff047136c77249d0642fba3f23675bdc9",
//		"7f00a466485c6c84f1709352511a22a32f9dfd6e1d80611d415fb3f2a6ba9de4",
//	}
//
//	api := s3.NewFromConfig(cfg)
//
//	fileMap := map[string]*layerreader.EntryWithLayer{}
//
//	for _, hash := range hashes {
//		get, err := api.GetObject(ctx, &s3.GetObjectInput{
//			Bucket: aws.String("browseimage-bucket-1oax5dhlqcoxp"),
//			Key:    aws.String(fmt.Sprintf("sha256:%s/files.json.gz", hash)),
//		})
//		require.NoError(t, err)
//
//		gzr, err := gzip.NewReader(get.Body)
//		require.NoError(t, err)
//
//		scan := bufio.NewScanner(gzr)
//		for scan.Scan() {
//			e := Entry{}
//			err = json.Unmarshal(scan.Bytes(), &e)
//			require.NoError(t, err)
//
//			base := filepath.Base(e.Hdr.Name)
//			if base == layerreader.WhiteoutOpaqueDir {
//				deletes := []string{}
//				deletedDir := strings.TrimSuffix(e.Hdr.Name, layerreader.WhiteoutOpaqueDir)
//				fmt.Printf("need to delete dir %s\n", deletedDir)
//				for key := range fileMap {
//					if strings.HasPrefix(key, deletedDir) {
//						deletes = append(deletes, key)
//					}
//				}
//				for _, del := range deletes {
//					delete(fileMap, del)
//					fmt.Printf("deleting (recursively) %s\n", del)
//				}
//			} else if strings.HasPrefix(base, layerreader.WhiteoutPrefix) {
//				name := strings.TrimPrefix(base, layerreader.WhiteoutPrefix)
//				delete(fileMap, name)
//				fmt.Printf("deleting %s\n", name)
//			} else {
//				fileMap[e.Hdr.Name] = &layerreader.EntryWithLayer{Entry: e, Layer: hash}
//			}
//		}
//	}
//
//	arr := make([]*layerreader.EntryWithLayer, 0, len(fileMap))
//	for _, e := range fileMap {
//		e := e
//		arr = append(arr, e)
//	}
//
//	sort.Slice(arr, func(i, j int) bool {
//		return arr[i].Hdr.Name < arr[j].Hdr.Name
//	})
//
//	f, err := os.Create("all.json")
//	for _, e := range arr {
//		j, _ := json.Marshal(e)
//		f.Write(j)
//		f.Write([]byte{'\n'})
//	}
//
//	f.Close()
//
//	//api := lambda.NewFromConfig(cfg)
//	//g, ctx := errgroup.WithContext(ctx)
//	//for _, hash := range hashes {
//	//	hash := hash
//	//	g.Go(func() error {
//	//		j, _ := json.Marshal(layerreader.RemoteInput{
//	//			Key: &bitypes.ImageInfoKey{
//	//				Repo:   "mcr.microsoft.com/dotnet/sdk",
//	//				Tag:    "6.0",
//	//				Digest: "sha256:3dfedfc30f95c93c3e1d41a2d376f4d3d6fef665888859b616c3b46dde695b73",
//	//			},
//	//			Layer: v1.Hash{
//	//				Algorithm: "sha256",
//	//				Hex:       hash,
//	//			},
//	//		})
//	//
//	//		invoke, err := api.Invoke(ctx, &lambda.InvokeInput{
//	//			FunctionName: aws.String("browseimage-LayerReader-FDQiSG33u9E5:live"),
//	//			LogType:      types.LogTypeTail,
//	//			Payload:      j,
//	//		})
//	//		if err != nil {
//	//			return fmt.Errorf(": %w", err)
//	//		}
//	//
//	//		fmt.Println(string(invoke.Payload))
//	//		return nil
//	//	})
//	//}
//	//
//	//err = g.Wait()
//	//require.NoError(t, err)
//}

func TestBuildIndex(t *testing.T) {
	newDigest, err := name.NewDigest("mcr.microsoft.com/dotnet/sdk:6.0@sha256:a603fa5e3b4127f210503aaa6189abf6286ee5a73deeaab460f8f33ebc6b64e2")
	require.NoError(t, err)

	layer, err := remote.Layer(newDigest, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	require.NoError(t, err)

	in, err := layer.Compressed()
	require.NoError(t, err)

	wd, _ := os.Getwd()
	var fileCount int64
	index, err := BuildIndex(context.Background(), wd, in, &fileCount)
	require.NoError(t, err)
	require.NotNil(t, index)

	fmt.Println(index.GzIndexPath)
	fmt.Println(index.FileIndexPath)
}

func TestExtractUsingIndex(t *testing.T) {
	idx, err := ReadIndex("/Users/aidan/dev/ge/browseimage/targzi/targzi2915824909")
	require.NoError(t, err)

	entry := Entry{}
	for _, e := range idx.Entries {
		if e.Hdr.Name == "usr/share/common-licenses/LGPL-2.1" {
			entry = *e
			break
		}
	}

	c := &http.Client{
		Transport: &transport{RoundTripper: http.DefaultTransport},
	}
	req, _ := http.NewRequest("GET", "https://mcr.microsoft.com/v2/dotnet/sdk/blobs/sha256:a603fa5e3b4127f210503aaa6189abf6286ee5a73deeaab460f8f33ebc6b64e2", nil)

	spans, err := idx.Spans()
	require.NoError(t, err)
	span := IndexSpan{}
	for _, s := range spans {
		if s.Number == entry.Spans[0] {
			span = s
			break
		}
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", span.Compressed-1))

	resp, err := c.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	//body, err := idx.Extract(resp.Body, span.Compressed, entry.Offset, int(entry.Hdr.Size))
	//require.NoError(t, err)
	//
	//fmt.Println(string(body))
}
