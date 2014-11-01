package pce

import (
	"github.com/sselph/scraper/rom"
)

func init() {
	rom.RegisterFormat(".pce", rom.Noop)
}
