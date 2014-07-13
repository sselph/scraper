/*
Copyright (c) 2014 sselph

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

// Package nes decodes .nes files
package nes

import (
	"fmt"
	"io"
	"os"
	"github.com/sselph/scraper/rom"
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
