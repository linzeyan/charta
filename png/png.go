// Package png renders a charta chart to a PNG in pure Go (no cgo, no GUI
// toolkit). charta embeds no font: by default the chart is set in the
// system's (Inter when installed, else the platform's sans-serif), as the
// Rust crate's render_png does, and laid out with the same faces it is
// drawn with, so labels fit. Characters the faces lack, CJK for most Latin
// fonts, come from whichever installed font has them.
//
//	data, err := png.Render(chart, png.Options{Scale: 2})
//
// Pass Regular and Semibold to choose the faces, e.g. for output that does
// not depend on what is installed:
//
//	regular, _ := sfnt.Parse(goregular.TTF) // golang.org/x/image/font/gofont
//	bold, _ := sfnt.Parse(gobold.TTF)
//	data, err := png.Render(chart, png.Options{Regular: regular, Semibold: bold})
//
// PNGs are not part of the conformance fixtures: rasterization and fonts
// differ from platform to platform.
package png

import (
	"bytes"
	"errors"
	"fmt"
	stdpng "image/png"
	"math"

	"golang.org/x/image/font/sfnt"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/internal/raster"
	"github.com/linzeyan/charta/internal/sysfont"
)

// Options control Render. Width and Height are logical pixels (zero:
// 640×400, as charta.Options); the image is Width×Scale by Height×Scale
// device pixels (zero Scale: 1).
type Options struct {
	Width, Height float64
	Scale         float64
	// Dark selects the dark palette.
	Dark bool
	// Regular and Semibold replace the system faces when Regular is set;
	// Semibold then falls back to it (titles and tooltip values lose their
	// weight).
	Regular, Semibold *sfnt.Font
}

// ErrNoFont reports that Options has no Regular face and the system has no
// sans-serif to use instead.
var ErrNoFont = errors.New("png: no system font found; charta embeds no font, so pass Options.Regular (e.g. golang.org/x/image/font/gofont/goregular)")

// Render lays the chart out at Width × Height and paints it at Scale device
// pixels per logical pixel.
func Render(c charta.Chart, opts Options) ([]byte, error) {
	if opts.Regular == nil {
		opts.Regular, opts.Semibold = sysfont.Sans(false), sysfont.Sans(true)
	}
	if opts.Regular == nil {
		return nil, ErrNoFont
	}
	if opts.Width == 0 {
		opts.Width = 640
	}
	if opts.Height == 0 {
		opts.Height = 400
	}
	if opts.Scale == 0 {
		opts.Scale = 1
	}
	if !(opts.Scale > 0) || math.IsInf(opts.Scale, 0) {
		return nil, fmt.Errorf("png: scale %v must be positive and finite", opts.Scale)
	}
	fonts := raster.NewFonts(opts.Regular, opts.Semibold, sysfont.Fallback)
	sc, err := c.Scene(opts.Width, opts.Height, fonts)
	if err != nil {
		return nil, err
	}
	// Rounded as the Rust crate's render_png sizes its pixmap.
	w := max(int(math.Round(sc.Width*opts.Scale)), 1)
	h := max(int(math.Round(sc.Height*opts.Scale)), 1)
	img := raster.Paint(sc, fonts, -1, raster.Pointer{}, nil, w, h, opts.Scale, opts.Dark)
	var buf bytes.Buffer
	if err := stdpng.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("png: encoding failed: %w", err)
	}
	return buf.Bytes(), nil
}
