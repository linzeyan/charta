package png_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	stdpng "image/png"
	"testing"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/png"
)

func faces(t *testing.T) (*sfnt.Font, *sfnt.Font) {
	t.Helper()
	r, err := sfnt.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	b, err := sfnt.Parse(gobold.TTF)
	if err != nil {
		t.Fatal(err)
	}
	return r, b
}

var bars = charta.BarChart{Data: []charta.Bar{{Label: "A", Value: 30}, {Label: "B", Value: 80}}}

func decode(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := stdpng.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func rgb(c color.Color) [3]uint8 {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	return [3]uint8{n.R, n.G, n.B}
}

func TestAPNGIsTheChartAtDeviceSize(t *testing.T) {
	regular, bold := faces(t)
	for _, dark := range []bool{false, true} {
		data, err := png.Render(bars, png.Options{Width: 320, Height: 200, Scale: 2, Dark: dark, Regular: regular, Semibold: bold})
		if err != nil {
			t.Fatal(err)
		}
		img := decode(t, data)
		if b := img.Bounds(); b.Dx() != 640 || b.Dy() != 400 {
			t.Fatalf("dark=%v: %v, want 640×400", dark, b)
		}
		// The palette's surface in the corner, not the other mode's.
		s := base.Resolve(base.Surface, dark)
		if got := rgb(img.At(1, 1)); got != [3]uint8{s.R, s.G, s.B} {
			t.Fatalf("dark=%v: corner %v, want surface %v", dark, got, s)
		}
		// Something besides the surface: bars, axis and labels were drawn.
		bar := base.Resolve(base.Series(0), dark)
		found := false
		for y := 0; y < 400 && !found; y++ {
			for x := 0; x < 640; x++ {
				if rgb(img.At(x, y)) == [3]uint8{bar.R, bar.G, bar.B} {
					found = true
					break
				}
			}
		}
		if !found {
			t.Fatalf("dark=%v: no pixel in the bar color", dark)
		}
	}
}

func TestDefaultsAre640By400AtScaleOne(t *testing.T) {
	regular, _ := faces(t)
	data, err := png.Render(bars, png.Options{Regular: regular})
	if err != nil {
		t.Fatal(err)
	}
	if b := decode(t, data).Bounds(); b.Dx() != 640 || b.Dy() != 400 {
		t.Fatalf("zero options: %v, want 640×400", b)
	}
	data, err = png.Render(bars, png.Options{Width: 100.4, Height: 50, Scale: 1.5, Regular: regular})
	if err != nil {
		t.Fatal(err)
	}
	if b := decode(t, data).Bounds(); b.Dx() != 151 || b.Dy() != 75 {
		t.Fatalf("100.4×50 at 1.5: %v, want 151×75 (rounded)", b)
	}
}

func TestBadSizesAreErrors(t *testing.T) {
	regular, _ := faces(t)
	if _, err := png.Render(bars, png.Options{Scale: -1, Regular: regular}); err == nil {
		t.Fatal("a negative scale rendered")
	}
	if _, err := png.Render(bars, png.Options{Width: -5, Regular: regular}); !errors.Is(err, charta.ErrInvalidSize) {
		t.Fatalf("a negative width: %v", err)
	}
}

// TestTextIsDrawnInTheGivenFace: the layout measures with the caller's
// faces, so a label laid out for Go Regular is painted where its glyphs are.
func TestTextIsDrawnInTheGivenFace(t *testing.T) {
	regular, bold := faces(t)
	titled := charta.BarChart{Title: "Fruit", Data: bars.Data}
	plain, err := png.Render(bars, png.Options{Regular: regular, Semibold: bold})
	if err != nil {
		t.Fatal(err)
	}
	withTitle, err := png.Render(titled, png.Options{Regular: regular, Semibold: bold})
	if err != nil {
		t.Fatal(err)
	}
	a, b := decode(t, plain), decode(t, withTitle)
	ink := base.Resolve(base.TextPrimary, false)
	// The title is the only primary-colored text top left.
	n := 0
	for y := 0; y < 40; y++ {
		for x := 0; x < 120; x++ {
			if rgb(b.At(x, y)) == [3]uint8{ink.R, ink.G, ink.B} && rgb(a.At(x, y)) != rgb(b.At(x, y)) {
				n++
			}
		}
	}
	if n < 10 {
		t.Fatalf("title barely drawn (%d px)", n)
	}
}

// TestWithoutFacesTheSystemsAreUsed: charta embeds no font, so with none
// given the chart is set in the system's, CJK included, rather than
// failing or leaving gaps.
func TestWithoutFacesTheSystemsAreUsed(t *testing.T) {
	titled := charta.BarChart{Title: "Fruit", Data: bars.Data}
	cjk := charta.BarChart{Title: "水果銷量", Data: bars.Data}
	ink, paper := base.Resolve(base.TextPrimary, false), base.Resolve(base.Surface, false)
	// Pixels nearer the ink than the paper: thin CJK strokes antialias, so
	// few are exactly the ink.
	titleInk := func(c charta.Chart) int {
		data, err := png.Render(c, png.Options{})
		if err != nil {
			t.Fatal(err)
		}
		img, n := decode(t, data), 0
		dist := func(p [3]uint8, r, g, b uint8) int {
			d := func(x, y uint8) int { return max(int(x), int(y)) - min(int(x), int(y)) }
			return d(p[0], r) + d(p[1], g) + d(p[2], b)
		}
		for y := 0; y < 40; y++ {
			for x := 0; x < 120; x++ {
				p := rgb(img.At(x, y))
				if dist(p, ink.R, ink.G, ink.B) < dist(p, paper.R, paper.G, paper.B) {
					n++
				}
			}
		}
		return n
	}
	if n := titleInk(titled); n < 10 {
		t.Fatalf("Latin title barely drawn (%d px)", n)
	}
	if n := titleInk(cjk); n < 10 {
		t.Fatalf("CJK title barely drawn (%d px): no installed face supplied the glyphs", n)
	}
}
