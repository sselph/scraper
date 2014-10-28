// Package snes decodes .sms and .sfc files.
package snes

import (
	"github.com/sselph/scraper/rom"
	"io"
)

func init() {
	rom.RegisterFormat(".smc", decodeSNES)
	rom.RegisterFormat(".sfc", decodeSNES)
	rom.RegisterFormat(".fig", decodeSNES)
	rom.RegisterFormat(".swc", decodeSNES)
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
