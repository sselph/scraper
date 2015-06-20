// Package bin decodes bin files.

package bin

import (
	"github.com/sselph/scraper/rom"
)

func init() {
	rom.RegisterFormat(".bin", rom.Noop)
	rom.RegisterFormat(".a26", rom.Noop)
	rom.RegisterFormat(".rom", rom.Noop)
	rom.RegisterFormat(".cue", rom.Noop)
	rom.RegisterFormat(".gdi", rom.Noop)
}
