package layerreader

import (
	"archive/tar"
	"browseimage/bitypes"
	"browseimage/targzi"
	"context"
	"github.com/google/go-containerregistry/pkg/v1"
	"sort"
	"strings"
	"time"
)

const WhiteoutPrefix = ".wh."
const WhiteoutOpaqueDir = ".wh..wh..opq"

type EntryWithLayer struct {
	targzi.Entry
	Layer string
	// Immutable gzip index paired with the spans in this entry.
	GzipIndex string `json:",omitempty"`
	// For directories: the total size and number of the regular files
	// beneath them in the final filesystem.
	DirSize  int64 `json:",omitempty"`
	DirFiles int64 `json:",omitempty"`
}

type LayerReader interface {
	ReadLayer(ctx context.Context, key *bitypes.ImageInfoKey, layer v1.Layer) ([]MyTarHeader, error)
}

func sortHeaders(headers []MyTarHeader) {
	sort.Slice(headers, func(i, j int) bool {
		iname := headers[i].Name
		jname := headers[j].Name

		if strings.HasSuffix(iname, WhiteoutOpaqueDir) && !strings.HasSuffix(jname, WhiteoutOpaqueDir) {
			return true
		} else if !strings.HasSuffix(iname, WhiteoutOpaqueDir) && strings.HasSuffix(jname, WhiteoutOpaqueDir) {
			return false
		}

		return iname < jname
	})
}

type MyTarHeader struct {
	Name     string
	Type     byte
	Linkname string `json:",omitempty"`
	Size     int64
	Mode     int64
	Uid      int
	Gid      int
	Uname    string `json:",omitempty"`
	Gname    string `json:",omitempty"`
	ModTime  time.Time
	PAX      map[string]string `json:",omitempty"`
}

func convertTarHeader(in *tar.Header) MyTarHeader {
	return MyTarHeader{
		Name:     in.Name,
		Type:     in.Typeflag,
		Linkname: in.Linkname,
		Size:     in.Size,
		Mode:     in.Mode,
		Uid:      in.Uid,
		Gid:      in.Gid,
		Uname:    in.Uname,
		Gname:    in.Gname,
		ModTime:  in.ModTime,
		PAX:      in.PAXRecords,
	}
}
