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

// Package rom has helper functions for extracting rom data. Currently it is only used to hash them.
package rom

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
)

var formats = make(map[string]func(*os.File) (io.ReadCloser, error))

// RegisterFormat registers a format with the rom package.
func RegisterFormat(ext string, decode func(*os.File) (io.ReadCloser, error)) {
	formats[ext] = decode
}

// Decode takes a path and returns a reader for the inner rom data.
func Decode(p string) (io.ReadCloser, error) {
	ext := path.Ext(p)
	decode, ok := formats[ext]
	if !ok {
		return nil, fmt.Errorf("no registered decoder for extention %s", ext)
	}
	r, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	ret, err := decode(r)
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
	_, ok := formats[e]
	return ok
}
