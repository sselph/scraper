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
	rom.RegisterFormat(".mgd", decodeMGD)
	rom.RegisterFormat(".gen", rom.Noop)
	rom.RegisterFormat(".md", rom.Noop)
}

func DeInterleave(p []byte) []byte {
	l := len(p)
	m := l / 2
	b := make([]byte, l)
	for i, x := range p {
		if i < m {
			b[i*2+1] = x
		} else {
			b[i*2-l] = x
		}
	}
	return b
}

type SMDReader struct {
	f *os.File
	b []byte
	r *int
}

func (r SMDReader) Read(p []byte) (int, error) {
	ll := len(p)
	rl := ll - *r.r
	l := rl + 16384 - 1 - (rl-1)%16384
	copy(p, r.b[:*r.r])
	if rl <= 0 {
		*r.r = *r.r - ll
		copy(r.b, r.b[ll:])
		return ll, nil
	}
	n := *r.r
	for i := 0; i < l/16384; i++ {
		b := make([]byte, 16384)
		x, err := io.ReadFull(r.f, b)
		if x == 0 || err != nil {
			return n, err
		}
		b = DeInterleave(b)
		if ll < n+x {
			copy(p[n:ll], b)
			copy(r.b, b[ll-n:])
			*r.r = n + x - ll
			return ll, nil
		} else {
			copy(p[n:n+16384], b)
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
	return SMDReader{f, make([]byte, 16384), &i}, nil
}

type MGDReader struct {
	r io.Reader
}

func (r MGDReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	return n, err
}

func (r MGDReader) Close() error {
	return nil
}

func decodeMGD(f *os.File) (io.ReadCloser, error) {
	b, err := ioutil.ReadAll(f)
	f.Close()
	b = DeInterleave(b)
	if err != nil {
		return nil, err
	}
	return MGDReader{bytes.NewReader(b)}, nil
}
