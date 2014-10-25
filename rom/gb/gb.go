// Package gb decodes gb files.

package gb

import (
	"github.com/sselph/scraper/rom"
)

func init() {
	rom.RegisterFormat(".gb", rom.Noop)
	rom.RegisterFormat(".gbc", rom.Noop)
	rom.RegisterFormat(".gba", rom.Noop)
}
