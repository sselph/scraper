package rom

import (
	"archive/zip"
	"fmt"
	"io"
	"path"
	"strings"
)

type ZipReader struct {
	f   *zip.ReadCloser
	rom io.ReadCloser
}

func (r ZipReader) Read(p []byte) (int, error) {
	n, err := r.rom.Read(p)
	return n, err
}

func (r ZipReader) Close() error {
	r.rom.Close()
	return r.f.Close()
}

func decodeZip(f string) (io.ReadCloser, error) {
	r, err := zip.OpenReader(f)
	if err != nil {
		return nil, err
	}
	var zr ZipReader
	for _, zf := range r.File {
		ext := strings.ToLower(path.Ext(zf.FileHeader.Name))
		if KnownExt(ext) {
			rf, err := zf.Open()
			if err != nil {
				continue
			}
			rs := zf.FileHeader.UncompressedSize64
			decoder := formats[ext]
			rom, err := decoder(rf, int64(rs))
			if err != nil {
				continue
			}
			zr = ZipReader{r, rom}
			return zr, nil
		}
	}
	return nil, fmt.Errorf("No valid roms found in zip.")
}
