package targzi

import (
	"archive/tar"
	"fmt"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"os/exec"
	"regexp"
	"testing"
)

func TestGztool(t *testing.T) {
	in, err := os.Open("ak2.tar.gz")
	require.NoError(t, err)

	cmd := exec.Command("./gztool/gztool", "-I", "index", "-b", "0")
	cmd.Stdin = in
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)

	err = cmd.Start()
	require.NoError(t, err)

	//tidx := &bytes.Buffer{}
	//r, err := asm.NewInputTarStream(stdout, storage.NewJSONPacker(tidx), storage.NewDiscardFilePutter())
	//require.NoError(t, err)
	//
	//_, err = io.Copy(io.Discard, r)
	//require.NoError(t, err)
	//
	//fmt.Println(tidx.String())

	off := &offsetReporter{Reader: stdout}
	tr := tar.NewReader(off)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		require.NoError(t, err)

		fmt.Printf("%d %d %s\n", off.offset, hdr.Size, hdr.Name)
		//spew.Dump(Hdr.Name)
		//fmt.Printf("Contents of %s:\n", Hdr.Name)
		//if _, err := io.Copy(os.Stdout, tr); err != nil {
		//	log.Fatal(err)
		//}
		//fmt.Println()
	}
}

func TestExtractFile(t *testing.T) {
	in, err := os.Open("ak2.tar.gz")
	require.NoError(t, err)

	_, err = in.Seek(3110140-1, io.SeekStart)
	require.NoError(t, err)

	cmd := exec.Command("./gztool/gztool", "-I", "index", "-n", "3110140", "-b", "15697408", "-r", "1314")
	cmd.Stdin = in
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)

	err = cmd.Start()
	require.NoError(t, err)

	body, err := io.ReadAll(stdout)
	require.NoError(t, err)

	err = cmd.Wait()
	require.NoError(t, err)

	fmt.Println(string(body))
}

/*
15697408 1314 ak2/underquoting-go/go.mod
15699456 5147 ak2/underquoting-go/go.sum
15705600 0 ak2/underquoting-go/generate_first_url/
15706112 0 ak2/underquoting-go/result_page_processor/
15706624 263 ak2/underquoting-go/deploy.sh
15707648 2260 ak2/underquoting-go/cfn.yml
*/

func TestReadIndexInfo(t *testing.T) {
	output := `Checking index file 'index' ...
	Size of index file (v1)  : 273.25 kiB (279805 Bytes)
	Number of index points   : 22
	Size of uncompressed file: 22.91 MiB (24020480 Bytes)
	Number of lines          : 349337 (349.34 k)
	Compression factor       : 67.94%
	List of points:
	#: @ compressed/uncompressed byte L#line_number (window data size in Bytes @window's beginning at index file), ...
#1: @ 10 / 0 L1 ( 0 @60 ), #2: @ 406901 / 1136572 L23463 ( 9135 @92 ), #3: @ 535299 / 2227934 L51865 ( 11883 @9259 ), #4: @ 877282 / 3281796 L75136 ( 13840 @21174 ), #5: @ 1076407 / 4611692 L102889 ( 2128 @35046 ), 
#6: @ 1479496 / 5683118 L124229 ( 8115 @37206 ), #7: @ 1606250 / 6749007 L151893 ( 14261 @45353 ), #8: @ 1999550 / 7858755 L173000 ( 12731 @59646 ), #9: @ 2162596 / 8923614 L200968 ( 32146 @72409 ), #10: @ 2889585 / 9995092 L213721 ( 16862 @104587 ), 
#11: @ 3286509 / 11587636 L248873 ( 2040 @121481 ), #12: @ 3669653 / 12647004 L275217 ( 17743 @123553 ), #13: @ 4138888 / 13732691 L290903 ( 13230 @141328 ), #14: @ 4438930 / 14846134 L297520 ( 13044 @154590 ), #15: @ 4667586 / 15929989 L310023 ( 17151 @167666 ), 
#16: @ 5119103 / 17012620 L311816 ( 16889 @184849 ), #17: @ 5567031 / 18088862 L313699 ( 15260 @201770 ), #18: @ 5957133 / 19147180 L316246 ( 11745 @217062 ), #19: @ 6263982 / 20258369 L318065 ( 9635 @228839 ), #20: @ 6666848 / 21313733 L321076 ( 13806 @238506 ), 
#21: @ 7070006 / 22388130 L331654 ( 17108 @252344 ), #22: @ 7533911 / 23500440 L344366 ( 10297 @269484 ), 
`

	re := regexp.MustCompile(`#(\d+): @ (\d+) / (\d+)`)
	matches := re.FindAllStringSubmatch(output, -1)
	for _, m := range matches {
		span, compressed, uncompressed := m[1], m[2], m[3]
		fmt.Printf("#%s: %s / %s\n", span, compressed, uncompressed)
	}
}

/*
ACTION: Check & list info in index file

Checking index file 'index' ...
	Size of index file (v1)  : 24.44 kiB (25022 Bytes)
	Number of index points   : 3
	Size of uncompressed file: 22.91 MiB (24020480 Bytes)
	Number of lines          : 349337 (349.34 k)
	Compression factor       : 68.71%
	List of points:

#: @ compressed/uncompressed byte L#line_number (window data size in Bytes @window's beginning at index file), ...

#1: @ 10      / 0        L1      ( 0 @60 ),
#2: @ 3110140 / 10485897 L228126 ( 14042 @92 ),
#3: @ 6568507 / 20989348 L319826 ( 10832 @14166 ),
*/
