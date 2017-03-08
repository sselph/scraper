package hash

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

var extra map[string]bool

func init() {
	extra = make(map[string]bool)
}

func AddExtra(e ...string) {
	for _, x := range e {
		extra[strings.ToLower(x)] = true
	}
}

func DelExtra(e ...string) {
	for _, x := range e {
		delete(extra, strings.ToLower(x))
	}
}

func HasExtra(e string) bool {
	return extra[e]
}

func ClearExtra() {
	extra = make(map[string]bool)
}

type decoder func(io.ReadCloser, int64) (io.ReadCloser, error)

// Noop does nothong but return the passed in file.
func noop(f io.ReadCloser, s int64) (io.ReadCloser, error) {
	return f, nil
}

func decodeLNX(f io.ReadCloser, s int64) (io.ReadCloser, error) {
	if s < 4 {
		return nil, fmt.Errorf("invalid ROM")
	}
	tmp := make([]byte, 64)
	_, err := io.ReadFull(f, tmp)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	if err == io.ErrUnexpectedEOF || !bytes.Equal(tmp[:4], []byte("LYNX")) {
		return newMultiReader(tmp, f), nil
	}
	return f, nil
}

type multireader struct {
	r  io.ReadCloser
	mr io.Reader
}

func (mr *multireader) Read(b []byte) (int, error) {
	return mr.mr.Read(b)
}

func (mr *multireader) Close() error {
	return mr.r.Close()
}

func newMultiReader(b []byte, r io.ReadCloser) io.ReadCloser {
	return &multireader{r, io.MultiReader(bytes.NewReader(b), r)}
}

func decodeA78(f io.ReadCloser, s int64) (io.ReadCloser, error) {
	tmp := make([]byte, 128)
	_, err := io.ReadFull(f, tmp)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	if err == io.ErrUnexpectedEOF || !bytes.Equal(tmp[1:10], []byte("ATARI7800")) {
		return newMultiReader(tmp, f), nil
	}
	return f, nil
}

func deinterleave(p []byte) []byte {
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

func decodeSMD(f io.ReadCloser, s int64) (io.ReadCloser, error) {
	return decodeMD(f, s, ".smd")
}

func decodeMGD(f io.ReadCloser, s int64) (io.ReadCloser, error) {
	return decodeMD(f, s, ".mgd")
}

func decodeGEN(f io.ReadCloser, s int64) (io.ReadCloser, error) {
	return decodeMD(f, s, ".gen")
}

func decodeMD(f io.ReadCloser, s int64, e string) (io.ReadCloser, error) {
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
		return ioutil.NopCloser(bytes.NewReader(b)), nil
	}
	if bytes.Equal(b[8320:8328], []byte("SG EEI  ")) || bytes.Equal(b[8320:8328], []byte("SG EADIE")) {
		for i := 0; int64(i) < (s / int64(16384)); i++ {
			x := i * 16384
			copy(b[x:x+16384], deinterleave(b[x:x+16384]))
		}
		return ioutil.NopCloser(bytes.NewReader(b)), nil
	}
	if bytes.Equal(b[128:135], []byte("EAGNSS ")) || bytes.Equal(b[128:135], []byte("EAMG RV")) {
		b = deinterleave(b)
		return ioutil.NopCloser(bytes.NewReader(b)), nil
	}
	switch e {
	case ".smd":
		for i := 0; int64(i) < (s / int64(16384)); i++ {
			x := i * 16384
			copy(b[x:x+16384], deinterleave(b[x:x+16384]))
		}
		return ioutil.NopCloser(bytes.NewReader(b)), nil
	case ".mgd":
		b = deinterleave(b)
		return ioutil.NopCloser(bytes.NewReader(b)), nil
	case ".gen":
		return ioutil.NopCloser(bytes.NewReader(b)), nil
	}
	return nil, fmt.Errorf("Unknown MD Error")
}

func decodeN64(f io.ReadCloser, s int64) (io.ReadCloser, error) {
	i := 0
	r := swapReader{f, make([]byte, 4), &i, noSwap}
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
		if l-i < 4 {
			continue
		}
		b[i+1], b[i] = b[i], b[i+1]
		b[i+3], b[i+2] = b[i+2], b[i+3]
	}
}

func nSwap(b []byte) {
	l := len(b)
	for i := 0; i < l; i += 4 {
		if l-i < 4 {
			continue
		}
		b[i+2], b[i] = b[i], b[i+2]
		b[i+3], b[i+1] = b[i+1], b[i+3]
	}
}

type swapReader struct {
	f io.ReadCloser
	b []byte
	r *int
	s func([]byte)
}

func (r swapReader) Read(p []byte) (int, error) {
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
	if x == 0 || err != nil && err != io.ErrUnexpectedEOF {
		return n, err
	}
	copy(p[n:ll], b[:x])
	n += x
	if ll <= n {
		r.s(p)
		copy(r.b, b[x+ll-n:x])
		*r.r = n - ll
		return ll, nil
	}
	r.s(p[:n])
	*r.r = 0
	return n, nil
}

func (r swapReader) Close() error {
	return r.f.Close()
}

type nesReader struct {
	f      io.ReadCloser
	offset int64
	size   int64
	start  int64
	end    int64
}

func (r *nesReader) Read(p []byte) (int, error) {
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

func (r *nesReader) Close() error {
	return r.f.Close()
}

func decodeNES(f io.ReadCloser, s int64) (io.ReadCloser, error) {
	header := make([]byte, 16)
	n, err := io.ReadFull(f, header)
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
		tmp := make([]byte, 512)
		n, err := io.ReadFull(f, tmp)
		if err != nil {
			return nil, err
		}
		offset += int64(n)
	}
	return &nesReader{f, offset, prg + chr, offset, offset + prg + chr}, nil
}

func decodeSNES(f io.ReadCloser, s int64) (io.ReadCloser, error) {
	if s%1024 == 512 {
		tmp := make([]byte, 512)
		_, err := io.ReadFull(f, tmp)
		if err != nil {
			return nil, err
		}
	}
	return f, nil
}

func getDecoder(ext string) (decoder, bool) {
	ext = strings.ToLower(ext)
	switch ext {
	case ".bin", ".a26", ".a52", ".rom", ".cue", ".gdi", ".gb", ".gba", ".gbc", ".32x", ".gg",
		".pce", ".sms", ".col", ".ngp", ".ngc", ".sg", ".int", ".vb", ".vec", ".gam", ".j64",
		".jag", ".mgw", ".nds", ".fds":
		return noop, true
	case ".a78":
		return decodeA78, true
	case ".lnx", ".lyx":
		return decodeLNX, true
	case ".smd":
		return decodeSMD, true
	case ".mgd":
		return decodeMGD, true
	case ".gen", ".md":
		return decodeGEN, true
	case ".n64", ".v64", ".z64":
		return decodeN64, true
	case ".nes":
		return decodeNES, true
	case ".smc", ".sfc", ".fig", ".swc":
		return decodeSNES, true
	default:
		return noop, extra[ext]
	}
}

// KnownExt returns true if the ext is recognized.
func KnownExt(ext string) bool {
	ext = strings.ToLower(ext)
	if ext == ".zip" {
		return true
	}
	if ext == ".7z" {
		return true
	}
	if ext == ".gz" {
		return true
	}
	_, ok := getDecoder(ext)
	return ok
}

// decode takes a path and returns a reader for the inner rom data.
func decode(p string) (io.ReadCloser, error) {
	ext := strings.ToLower(path.Ext(p))
	if ext == ".zip" {
		r, err := decodeZip(p)
		return r, err
	}
	if ext == ".7z" {
		r, err := decode7Zip(p)
		return r, err
	}
	if ext == ".gz" {
		r, err := decodeGZip(p)
		return r, err
	}
	decode, _ := getDecoder(ext)
	r, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	fi, err := r.Stat()
	if err != nil {
		return nil, err
	}
	ret, err := decode(r, fi.Size())
	return ret, err
}

// Hash returns the hash of a rom given a path to the file and hash function.
func Hash(p string, h hash.Hash, buf []byte) (string, error) {
	r, err := decode(p)
	if err != nil {
		return "", err
	}
	defer r.Close()
	for {
		n, err := r.Read(buf)
		if err != nil && err != io.EOF {
			return "", err
		}
		if n == 0 {
			break
		}
		h.Write(buf[:n])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
