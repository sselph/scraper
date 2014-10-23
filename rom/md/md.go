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

// Package md decodes md and smd files.

package md

import (
	"bytes"
	"github.com/sselph/scraper/rom"
	"io"
	"io/ioutil"
	"os"
)

func init() {
	rom.RegisterFormat(".smd", decodeSMD)
	rom.RegisterFormat(".md", decodeMD)
	rom.RegisterFormat(".gen", rom.Noop)
}

func DeInterleave(p []byte) []byte {
	l := len(p)
	m := l / 2
	b := make([]byte, l)
	for i, x := range p {
		if i < m {
			b[i*2] = x
		} else {
			b[i*2-l+1] = x
		}
	}
	return b
}

type SMDReader struct {
	f      *os.File
	b      []byte
	r      *int
}

func (r SMDReader) Read(p []byte) (int, error) {
	ll := len(p)
	rl := ll - *r.r
	l := rl + 16 - 1 - (rl - 1) % 16;
	copy(p, r.b[:*r.r])
	if rl <= 0 {
		*r.r = *r.r - ll
		copy(r.b, r.b[ll:])
		return ll, nil
	}
	n := *r.r
	for i := 0; i < l / 16; i++ {
		b := make([]byte, 16)
		x, err := io.ReadFull(r.f, b)
		if x == 0 || err != nil {
			return n, err
		}
		b = DeInterleave(b)
		if ll < n+x {
			copy(p[n:ll], b)
			copy(r.b, b[ll-n:])
			*r.r = n+x - ll
			return ll, nil
		} else {
			copy(p[n:n+16], b)
		}
		n += x
	}
	return ll, nil
}

func (r SMDReader) Close() error {
	return r.f.Close()
}

func decodeSMD(f *os.File) (io.ReadCloser, error) {
	f.Seek(512, 0)
	i := 0
	return SMDReader{f, make([]byte, 16), &i}, nil
}

type MDReader struct {
	r      io.Reader
}

func (r MDReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	return n, err
}

func (r MDReader) Close() error {
	return nil
}

func decodeMD(f *os.File) (io.ReadCloser, error) {
	b, err := ioutil.ReadAll(f)
	b = DeInterleave(b)
	if err != nil {
		return nil, err
	}
	return MDReader{bytes.NewReader(b)}, nil
}
