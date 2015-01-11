package lnx

import (
	"bytes"
	"fmt"
	"github.com/sselph/scraper/rom"
	"io"
)

func init() {
	rom.RegisterFormat(".lnx", decodeLNX)
	rom.RegisterFormat(".lyx", rom.Noop)
}

func decodeLNX(f io.ReadCloser, s int64) (io.ReadCloser, error) {
	if s < 4 {
		return nil, fmt.Errorf("invalid ROM")
	}
	tmp := make([]byte, 64)
	_, err := io.ReadFull(f, tmp)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(tmp[:4], []byte("LYNX")) {
		return nil, fmt.Errorf("lnx file missing magic LYNX header, maybe it is a lyx")
	}
	return f, nil
}
