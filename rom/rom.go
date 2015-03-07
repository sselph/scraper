// Package rom has helper functions for extracting rom data. Currently it is only used to hash them.
package rom

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

type Decoder func(io.ReadCloser, int64) (io.ReadCloser, error)

var formats = make(map[string]Decoder)

// RegisterFormat registers a format with the rom package.
func RegisterFormat(ext string, decode Decoder) {
	formats[ext] = decode
}

// Decode takes a path and returns a reader for the inner rom data.
func Decode(p string) (io.ReadCloser, error) {
	ext := strings.ToLower(path.Ext(p))
	if ext == ".zip" {
		r, err := decodeZip(p)
		return r, err
	}
	if ext == ".gz" {
		r, err := decodeGZip(p)
		return r, err
	}
	decode, ok := formats[ext]
	if !ok {
		return nil, fmt.Errorf("no registered decoder for extention %s", ext)
	}
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

// SHA1 takes a pathand returns the SHA1 hash of the inner rom.
func SHA1(p string) (string, error) {
	r, err := Decode(p)
	if err != nil {
		return "", err
	}
	defer r.Close()
	buf := make([]byte, 4*1024*1024)
	h := sha1.New()
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

// KnownExt returns True if the extention is registered.
func KnownExt(e string) bool {
	if strings.ToLower(e) == ".zip" {
		return true
	}
	if strings.ToLower(e) == ".gz" {
		return true
	}
	_, ok := formats[strings.ToLower(e)]
	return ok
}

// Noop does nothong but return the passed in file.
func Noop(f io.ReadCloser, s int64) (io.ReadCloser, error) {
	return f, nil
}
