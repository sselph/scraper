// Package nes decodes .nes files
package nes

import (
	"fmt"
	"github.com/sselph/scraper/rom"
	"io"
	"os"
)

func init() {
	rom.RegisterFormat(".nes", decodeNES)
}

type NESReader struct {
	f      *os.File
	offset int64
	size   int64
	start  int64
	end    int64
}

func (r *NESReader) Read(p []byte) (int, error) {
	n, err := r.f.Read(p)
	if err != nil {
		return n, err
	}
	if r.offset+int64(n) > r.end {
		n = int(r.end - r.offset)
	}
	r.offset += int64(n)
	return n, err
}

func (r *NESReader) Close() error {
	return r.f.Close()
}

func decodeNES(f *os.File) (io.ReadCloser, error) {
	header := make([]byte, 16)
	n, err := f.Read(header)
	if err != nil {
		return nil, err
	}
	if n < 16 {
		return nil, fmt.Errorf("invalid header")
	}
	prgSize := int64(header[4])
	chrSize := int64(header[5])
	if header[7]&12 == 8 {
		romSize := int64(header[9])
		chrSize = romSize&0x0F<<8 + chrSize
		prgSize = romSize&0xF0<<4 + prgSize
	}
	var prg, chr int64
	prg = 16 * 1024 * prgSize
	chr = 8 * 1024 * chrSize
	hasTrainer := header[6]&4 == 4
	var offset int64
	offset = 16
	if hasTrainer {
		offset, err = f.Seek(512, 1)
		if err != nil {
			return nil, err
		}
	}
	return &NESReader{f, offset, prg + chr, offset, offset + prg + chr}, nil
}
