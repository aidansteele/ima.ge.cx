package layerreader

import (
	"archive/tar"
	"browseimage/bitypes"
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/klauspost/compress/gzip"
	"io"
)

type Local struct{}

func (l Local) ReadLayer(ctx context.Context, key *bitypes.ImageInfoKey, layer v1.Layer) ([]MyTarHeader, error) {
	rawr, err := layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("getting layer reader: %w", err)
	}

	return l.ReadUncompressed(ctx, key, layer, rawr)
}

func (l Local) ReadUncompressed(ctx context.Context, key *bitypes.ImageInfoKey, layer v1.Layer, uncompressed io.Reader) ([]MyTarHeader, error) {
	r, err := gzip.NewReader(uncompressed)
	if err != nil {
		return nil, fmt.Errorf("initializing gzip reader: %w", err)
	}

	headers := []MyTarHeader{}

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterating gar: %w", err)
		}

		headers = append(headers, convertTarHeader(hdr))
	}

	sortHeaders(headers)

	return headers, nil
}
