package rom

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
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
	ext := strings.ToLower(path.Ext(gzr.Header.Name))
	if KnownExt(ext) {
		d, err := ioutil.ReadAll(gzr)
		if err != nil {
			return nil, err
		}
		r := bytes.NewReader(d)
		decoder := formats[ext]
		return decoder(ioutil.NopCloser(r), int64(r.Len()))
	}
	return nil, fmt.Errorf("No valid roms found in gzip.")
}
