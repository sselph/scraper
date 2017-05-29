package hash

import (
	"archive/zip"
	"fmt"
	"io"
	"path/filepath"
)

type zipReader struct {
	f   *zip.ReadCloser
	rom io.ReadCloser
}

func (r zipReader) Read(p []byte) (int, error) {
	n, err := r.rom.Read(p)
	return n, err
}

func (r zipReader) Close() error {
	r.rom.Close()
	return r.f.Close()
}

func decodeZip(f string) (io.ReadCloser, error) {
	r, err := zip.OpenReader(f)
	if err != nil {
		return nil, err
	}
	var zr zipReader
	for _, zf := range r.File {
		ext := filepath.Ext(zf.FileHeader.Name)
		if decoder, ok := getDecoder(ext); ok {
			rf, err := zf.Open()
			if err != nil {
				continue
			}
			rs := zf.FileHeader.UncompressedSize64
			rom, err := decoder(rf, int64(rs))
			if err != nil {
				continue
			}
			zr = zipReader{r, rom}
			return zr, nil
		}
	}
	return nil, fmt.Errorf("No valid roms found in zip.")
}
