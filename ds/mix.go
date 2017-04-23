package ds

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/nfnt/resize"
)

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
	Image    Image
	Width    Value
	Height   Value
	TopLeftX Value
	TopLeftY Value
	Fill     bool
}

type MixImage struct {
	def *Def
}

func (i MixImage) newWidthHeight(w, h uint) (uint, uint) {
	origW, origH := uint(i.def.Width), uint(i.def.Height)
	newW, newH := origW, origH
	if w != 0 && origW > w {
		newH = origH * w / origW
		if newH < 1 {
			newH = 1
		}
		newW = w
	}
	if h != 0 && newH > h {
		newW = newW * h / newH
		if newW < 1 {
			newW = 1
		}
		newH = h
	}
	return newW, newH
}

func (i MixImage) Get(ctx context.Context, w, h uint) (image.Image, error) {
	nw, nh := i.newWidthHeight(w, h)
	return Draw(ctx, i.def, int(nw), int(nh))
}

func (i MixImage) Save(ctx context.Context, p string, w, h uint) error {
	img, err := i.Get(ctx, w, h)
	if err != nil {
		return err
	}
	out, err := os.Create(p)
	if err != nil {
		return err
	}
	defer out.Close()
	e := filepath.Ext(p)
	switch e {
	case ".jpg":
		return jpeg.Encode(out, img, nil)
	case ".png":
		return png.Encode(out, img)
	default:
		return fmt.Errorf("Invalid image type.")
	}
}

// Def represents the overall mix definition.
type Def struct {
	Width    int
	Height   int
	Elements []Element
}

// StandardThree creates a Def for the Standard 3 image mix.
func StandardThree(screen, box, wheel Image) *Def {
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
		Fill:     true,
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
func StandardFour(screen, box, cart, wheel Image) *Def {
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
		Fill:     true,
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
func Draw(ctx context.Context, def *Def, width, height int) (image.Image, error) {
	m := image.NewRGBA(image.Rect(0, 0, width, height))
	first := true
	for _, e := range def.Elements {
		if e.Image == nil {
			continue
		}
		w := e.Width.V(width, 0)
		h := e.Height.V(height, 0)
		img, err := e.Image.Get(ctx, uint(w), uint(h))
		if err != nil {
			return m, fmt.Errorf("%s: %q", e.Image, err)
		}
		if e.Fill {
			img = resize.Resize(uint(w), uint(h), img, resize.Bilinear)
		}
		b := img.Bounds()
		w = b.Dx()
		h = b.Dy()
		x := e.TopLeftX.V(width, w)
		y := e.TopLeftY.V(height, h)
		r := image.Rect(x, y, x+w, y+h)
		if first {
			first = false
			draw.Draw(m, r, img, image.ZP, draw.Src)
		} else {
			draw.Draw(m, r, img, image.ZP, draw.Over)
		}
	}
	if first {
		return m, errors.New("no elements to draw")
	}
	return m, nil
}
