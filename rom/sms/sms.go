package sms

import (
	"github.com/sselph/scraper/rom"
)

func init() {
	rom.RegisterFormat(".sms", rom.Noop)
}
