package base

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/linzeyan/charta/assets"
)

// SeriesSlots is the number of categorical series colors. A ninth series is
// never a generated hue; charts must fold extras into "Other" instead.
const SeriesSlots = 8

// Color is a color role. Scenes carry roles, not colors, so HTML can emit
// CSS variables and native backends resolve the toolkit's light/dark mode.
// The zero value means "no color".
type Color int

const (
	NoColor Color = iota
	Surface
	TextPrimary
	TextSecondary
	TextMuted
	Grid
	Axis
	Border
	seriesBase Color = 100
	washBase   Color = 200
	linkBase   Color = 250
	seqBase    Color = 300
	bandBase   Color = 400
	roleEnd    Color = 500
)

// SeqClasses is the number of classes in the sequential ramp. Past ~7,
// adjacent classes blur.
const SeqClasses = 7

// BandClasses is the number of qualitative ranges a bullet graph can show.
const BandClasses = 3

// Series is categorical slot i, zero-based (slot 0 is the palette's series-1).
func Series(i int) Color { return seriesBase + Color(i) }

// Wash is slot i as a ~10% wash, for area fills.
func Wash(i int) Color { return washBase + Color(i) }

// Link is slot i at ~35%, for flows that overlap (ribbons, links): light
// enough that crossings stay visible, strong enough to follow.
func Link(i int) Color { return linkBase + Color(i) }

// Seq is sequential class i, zero-based, low to high.
func Seq(i int) Color { return seqBase + Color(i) }

// Band is qualitative range band i of a bullet graph, zero-based.
func Band(i int) Color { return bandBase + Color(i) }

// Token is the palette token, also the CSS variable name (--ch-<token>).
func (c Color) Token() string {
	switch c {
	case Surface:
		return "surface"
	case TextPrimary:
		return "text-primary"
	case TextSecondary:
		return "text-secondary"
	case TextMuted:
		return "text-muted"
	case Grid:
		return "grid"
	case Axis:
		return "axis"
	case Border:
		return "border"
	}
	switch {
	case c >= seriesBase && c < washBase:
		return fmt.Sprintf("series-%d", int(c-seriesBase)+1)
	case c >= washBase && c < linkBase:
		return fmt.Sprintf("series-%d-wash", int(c-washBase)+1)
	case c >= linkBase && c < seqBase:
		return fmt.Sprintf("series-%d-link", int(c-linkBase)+1)
	case c >= seqBase && c < bandBase:
		return fmt.Sprintf("seq-%d", int(c-seqBase)+1)
	case c >= bandBase && c < roleEnd:
		return fmt.Sprintf("band-%d", int(c-bandBase)+1)
	}
	panic(fmt.Sprintf("color %d has no token", int(c)))
}

// Rgba is an sRGB color with straight (unpremultiplied) alpha.
type Rgba struct{ R, G, B, A uint8 }

// Hex is the CSS hex form; the alpha byte is omitted when opaque.
func (c Rgba) Hex() string {
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

// Theme picks which palette a rendered chart uses.
type Theme int

const (
	// ThemeAuto follows the viewer: OS color scheme, overridden by a
	// data-theme attribute on the page root. HTML only; native backends ask
	// the toolkit.
	ThemeAuto Theme = iota
	ThemeLight
	ThemeDark
)

// Entry is one palette row.
type Entry struct {
	Token       string
	Light, Dark Rgba
}

var palette = sync.OnceValue(func() []Entry {
	var out []Entry
	for _, line := range strings.Split(assets.Palette, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 || strings.HasPrefix(f[0], "#") {
			continue
		}
		if len(f) != 3 {
			panic("malformed palette line: " + line)
		}
		out = append(out, Entry{Token: f[0], Light: parseHex(f[1]), Dark: parseHex(f[2])})
	}
	return out
})

// Palette returns the rows in file order.
func Palette() []Entry { return palette() }

func parseHex(s string) Rgba {
	b := func(i int) uint8 {
		v, err := strconv.ParseUint(s[i:i+2], 16, 8)
		if err != nil {
			panic("palette hex: " + s)
		}
		return uint8(v)
	}
	switch len(s) {
	case 6:
		return Rgba{b(0), b(2), b(4), 255}
	case 8:
		return Rgba{b(0), b(2), b(4), b(6)}
	}
	panic("palette color must be 6 or 8 hex digits: " + s)
}

// Resolve returns the concrete color of a role in light or dark mode.
func Resolve(c Color, dark bool) Rgba {
	token := c.Token()
	for _, e := range Palette() {
		if e.Token == token {
			if dark {
				return e.Dark
			}
			return e.Light
		}
	}
	panic("palette has no " + token)
}
