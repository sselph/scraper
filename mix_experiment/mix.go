package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"

	"github.com/nfnt/resize"
)

var mode = flag.String("mode", "std4", "The `mode` to use, std3 or std4.")
var screenPath = flag.String("s", "", "The `path` to the screen image.")
var boxPath = flag.String("b", "", "The `path` to the 3D boxart image.")
var cartPath = flag.String("c", "", "The `path` to the 3D cartridge image.")
var wheelPath = flag.String("w", "", "The `path` to the wheel iamge.")
var outputPath = flag.String("o", "comp_test.png", "The `path` to write the output.")

// PreDefValue are the predefined values for Values.
type PreDefValue int

const (
	Undefined PreDefValue = iota
	Center
	Left
	Right
	Up
	Down
)

// Value is a value which can be absolute, relative, or a predefined short hand.
type Value struct {
	Rel    float64
	Abs    int
	PreDef PreDefValue
}

// V returns the value based on the overall image dimension and the dimension of the
// overlayed image.
func (v Value) V(total, img int) int {
	if v.PreDef != Undefined {
		switch v.PreDef {
		case Center:
			return (total / 2) - (img / 2)
		case Left, Up:
			return 0
		case Right, Down:
			return total - img
		}
	}
	if v.Rel != 0 {
		return int(math.Floor(float64(total) * v.Rel))
	}
	return v.Abs
}

// Element represents an element in the overall image.
type Element struct {
	Image    string
	Width    Value
	Height   Value
	TopLeftX Value
	TopLeftY Value
}

// Def represents the overall mix definition.
type Def struct {
	Width    int
	Height   int
	Elements []Element
}

// StandardThree creates a Def for the Standard 3 image mix.
func StandardThree(screen, box, wheel string) *Def {
	def := &Def{
		Width:  600,
		Height: 400,
	}
	def.Elements = append(def.Elements, Element{
		Image:    screen,
		Width:    Value{Rel: 0.9},
		Height:   Value{Rel: 0.85},
		TopLeftX: Value{PreDef: Center},
		TopLeftY: Value{PreDef: Up},
	})
	def.Elements = append(def.Elements, Element{
		Image:    box,
		Width:    Value{Rel: 0.5},
		Height:   Value{Rel: 0.75},
		TopLeftX: Value{PreDef: Left},
		TopLeftY: Value{PreDef: Down},
	})
	def.Elements = append(def.Elements, Element{
		Image:    wheel,
		Width:    Value{Rel: 0.5},
		Height:   Value{Rel: 0.333},
		TopLeftX: Value{PreDef: Right},
		TopLeftY: Value{PreDef: Down},
	})
	return def
}

// StandardFour creates a Def for the Standard 4 image mix.
func StandardFour(screen, box, cart, wheel string) *Def {
	def := &Def{
		Width:  600,
		Height: 400,
	}
	def.Elements = append(def.Elements, Element{
		Image:    screen,
		Width:    Value{Rel: 0.9},
		Height:   Value{Rel: 0.85},
		TopLeftX: Value{PreDef: Center},
		TopLeftY: Value{PreDef: Up},
	})
	def.Elements = append(def.Elements, Element{
		Image:    box,
		Width:    Value{Rel: 0.5},
		Height:   Value{Rel: 0.75},
		TopLeftX: Value{PreDef: Left},
		TopLeftY: Value{PreDef: Down},
	})
	def.Elements = append(def.Elements, Element{
		Image:    cart,
		Width:    Value{Rel: 0.25},
		Height:   Value{Rel: 0.375},
		TopLeftX: Value{Rel: 0.167},
		TopLeftY: Value{PreDef: Down},
	})
	def.Elements = append(def.Elements, Element{
		Image:    wheel,
		Width:    Value{Rel: 0.5},
		Height:   Value{Rel: 0.333},
		TopLeftX: Value{PreDef: Right},
		TopLeftY: Value{PreDef: Down},
	})
	return def
}

// Draw draws the mix image.
func Draw(def *Def) (image.Image, error) {
	m := image.NewRGBA(image.Rect(0, 0, def.Width, def.Height))
	for i, e := range def.Elements {
		f, err := os.Open(e.Image)
		if err != nil {
			return m, fmt.Errorf("%s: %q", e.Image, err)
		}
		img, _, err := image.Decode(f)
		if err != nil {
			return m, fmt.Errorf("%s: %q", e.Image, err)
		}
		f.Close()
		w := e.Width.V(def.Width, 0)
		h := e.Width.V(def.Height, 0)
		img = resize.Thumbnail(uint(w), uint(h), img, resize.Bilinear)
		b := img.Bounds()
		w = b.Dx()
		h = b.Dy()
		x := e.TopLeftX.V(def.Width, w)
		y := e.TopLeftY.V(def.Height, h)
		r := image.Rect(x, y, x+w, y+h)
		if i == 0 {
			draw.Draw(m, r, img, image.ZP, draw.Src)
		} else {
			draw.Draw(m, r, img, image.ZP, draw.Over)
		}
	}
	return m, nil
}

func main() {
	flag.Parse()
	var def *Def
	if *mode == "std4" {
		def = StandardFour(*screenPath, *boxPath, *cartPath, *wheelPath)
	} else {
		def = StandardThree(*screenPath, *boxPath, *wheelPath)
	}
	m, err := Draw(def)
	if err != nil {
		log.Fatal(err)
	}
	o, err := os.Create(*outputPath)
	if err != nil {
		log.Fatal(err)
	}
	err = png.Encode(o, m)
	if err != nil {
		log.Fatal(err)
	}
	o.Close()
}
