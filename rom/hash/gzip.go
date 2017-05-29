package hash

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

func decodeGZip(f string) (io.ReadCloser, error) {
	file, err := os.Open(f)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()
	ext := filepath.Ext(gzr.Header.Name)
	if decoder, ok := getDecoder(ext); ok {
		d, err := ioutil.ReadAll(gzr)
		if err != nil {
			return nil, err
		}
		r := bytes.NewReader(d)
		return decoder(ioutil.NopCloser(r), int64(r.Len()))
	}
	return nil, fmt.Errorf("No valid roms found in gzip.")
}
