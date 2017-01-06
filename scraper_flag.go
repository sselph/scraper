package main

import (
	"fmt"
	"strconv"
)

type optBool struct {
	v   bool
	set bool
}

func (b *optBool) String() string {
	return fmt.Sprintf("%t", b.v)
}

func (b *optBool) Set(s string) error {
	b.set = true
	v, err := strconv.ParseBool(s)
	b.v = v
	return err
}

func (b *optBool) IsBoolFlag() bool { return true }
