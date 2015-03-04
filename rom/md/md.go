// Package md decodes md and smd files.

package md

import (
	"bytes"
	"fmt"
	"github.com/sselph/scraper/rom"
	"io"
	"io/ioutil"
)

func init() {
	rom.RegisterFormat(".smd", decodeMD)
	rom.RegisterFormat(".mgd", decodeMD)
	rom.RegisterFormat(".gen", decodeMD)
	rom.RegisterFormat(".md", decodeMD)
	rom.RegisterFormat(".32x", rom.Noop)
	rom.RegisterFormat(".gg", rom.Noop)
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

func decodeMD(f io.ReadCloser, s int64) (io.ReadCloser, error) {
	if s%16384 == 512 {
		tmp := make([]byte, 512)
		_, err := io.ReadFull(f, tmp)
		if err != nil {
			return nil, err
		}
		s -= 512
	}
	if s%16384 != 0 {
		return nil, fmt.Errorf("Invalid MD size")
	}
	b, err := ioutil.ReadAll(f)
	f.Close()
	if err != nil {
		return nil, err
	}
	if bytes.Equal(b[256:260], []byte("SEGA")) {
		return MDReader{bytes.NewReader(b)}, nil
	}
	x := DeInterleave(b[0:16384])
	if bytes.Equal(x[256:260], []byte("SEGA")) {
		for i := 0; int64(i) < (s / int64(16384)); i++ {
			x := i * 16384
			copy(b[x:x+16384], DeInterleave(b[x:x+16384]))
		}
		return MDReader{bytes.NewReader(b)}, nil
	}
	b = DeInterleave(b)
	return MDReader{bytes.NewReader(b)}, nil
}

type MDReader struct {
	r io.Reader
}

func (r MDReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	return n, err
}

func (r MDReader) Close() error {
	return nil
}
