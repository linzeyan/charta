package fyne

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"golang.org/x/image/font/sfnt"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/raster"
	"github.com/linzeyan/charta/internal/sysfont"
)

// Measure sizes text with the theme fonts the view draws with, so labels
// fit the glyphs on screen. ChartView lays charts out with it; use it to get
// the same scene the view paints.
type Measure struct{}

func (Measure) Width(s string, size float64, w base.Weight) float64 {
	return fonts().Width(s, size, w)
}

// fonts is the current Fyne theme's regular and bold text fonts. charta
// embeds no font: it draws with whatever the app's theme uses, as every
// other Fyne widget does. Characters the theme font lacks (CJK with the
// default theme) come from the system's fonts.
var fonts = sync.OnceValue(func() *raster.Fonts {
	var faces [2]*sfnt.Font
	for i, r := range []fyne.Resource{theme.TextFont(), theme.TextBoldFont()} {
		if r == nil {
			continue
		}
		if f, err := sfnt.Parse(r.Content()); err == nil {
			faces[i] = f
		}
	}
	return raster.NewFonts(faces[0], faces[1], sysfont.Fallback)
})
