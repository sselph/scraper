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
	rom.RegisterFormat(".fig", decodeSNES)
	rom.RegisterFormat(".swc", decodeSNES)
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
