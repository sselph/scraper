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

// Package snes decodes .sms and .sfc files.
package snes

import (
	"io"
	"os"
	"github.com/sselph/scraper/rom"
)

func init() {
	rom.RegisterFormat(".smc", decodeSNES)
	rom.RegisterFormat(".sfc", decodeSNES)
}

func decodeSNES(f *os.File) (io.ReadCloser, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.Size()%1024 == 512 {
		f.Seek(512, 0)
	}
	return f, nil
}
