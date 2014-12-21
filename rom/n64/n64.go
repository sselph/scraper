package n64

import (
	"fmt"
	"github.com/sselph/scraper/rom"
	"io"
)

func init() {
	rom.RegisterFormat(".n64", decodeN64)
	rom.RegisterFormat(".v64", decodeN64)
	rom.RegisterFormat(".z64", decodeN64)
}

func decodeN64(f io.ReadCloser, s int64) (io.ReadCloser, error) {
	i := 0
	r := SwapReader{f, make([]byte, 4), &i, noSwap}
	if s < 4 {
		return nil, fmt.Errorf("invalid ROM")
	}
	_, err := io.ReadFull(r.f, r.b)
	*r.r = 4
	if err != nil {
		return nil, err
	}
	switch {
	case r.b[0] == 0x80:
		r.s = zSwap
	case r.b[3] == 0x80:
		r.s = nSwap
	}
	return r, nil
}

func noSwap(b []byte) {}

func zSwap(b []byte) {
	l := len(b)
	for i := 0; i < l; i += 4 {
		b[i+1], b[i] = b[i], b[i+1]
		b[i+3], b[i+2] = b[i+2], b[i+3]
	}
}

func nSwap(b []byte) {
	l := len(b)
	for i := 0; i < l; i += 4 {
		b[i+2], b[i] = b[i], b[i+2]
		b[i+3], b[i+1] = b[i+1], b[i+3]
	}
}

type SwapReader struct {
	f io.ReadCloser
	b []byte
	r *int
	s func([]byte)
}

func (r SwapReader) Read(p []byte) (int, error) {
	ll := len(p)
	rl := ll - *r.r
	l := rl + 4 - 1 - (rl-1)%4
	copy(p, r.b[:*r.r])
	if rl <= 0 {
		*r.r = *r.r - ll
		copy(r.b, r.b[ll:])
		return ll, nil
	}
	n := *r.r
	b := make([]byte, l)
	x, err := io.ReadFull(r.f, b)
	if x == 0 || err != nil {
		return n, err
	}
	copy(p[n:ll], b[:x])
	n += x
	if ll <= n {
		r.s(p)
		copy(r.b, b[x+ll-n:x])
		*r.r = n - ll
		return ll, nil
	} else {
		r.s(p[:n])
		*r.r = 0
		return n, nil
	}
}

func (r SwapReader) Close() error {
	return r.f.Close()
}
