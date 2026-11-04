package raster

import (
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/linzeyan/charta/base"
)

// ppem the outlines are loaded at; scaled to the requested size afterwards,
// so one cached outline serves every size.
const ppem = 1024

// Fonts is a regular and a semibold face, measured and drawn alike: Width
// is the base.TextMeasure a scene is laid out with, and the painter places
// glyphs with the same advances, so labels fit what is drawn. charta embeds
// no font; the caller supplies these (Fyne: its theme's). Characters a face
// lacks come from the fallback (sysfont.Fallback: the CJK a theme font
// lacks), or are left as gaps. Safe for concurrent use.
type Fonts struct {
	mu        sync.Mutex
	buf       sfnt.Buffer
	faces     [2]*sfnt.Font
	fallback  func(rune) *sfnt.Font
	capHeight float64
	glyphs    map[glyphKey]*glyph
}

type glyphKey struct {
	r    rune
	bold bool
}

// glyph is one character's advance and outline in ems, y down, origin on
// the baseline.
type glyph struct {
	advance float64
	outline [][]seg
}

// NewFonts caches glyphs of regular and semibold; a nil semibold falls back
// to regular. fallback, when not nil, supplies a face for a character
// neither has; without one such characters, and every character when
// regular is nil, are gaps. The cap height, which places Top and Center
// baselines, comes from the regular face, or Inter's when the face has
// none.
func NewFonts(regular, semibold *sfnt.Font, fallback func(rune) *sfnt.Font) *Fonts {
	fs := &Fonts{faces: [2]*sfnt.Font{regular, semibold}, fallback: fallback, glyphs: map[glyphKey]*glyph{}, capHeight: base.CapHeight}
	if fs.faces[1] == nil {
		fs.faces[1] = fs.faces[0]
	}
	if f := fs.faces[0]; f != nil {
		if m, err := f.Metrics(&fs.buf, fixed.I(ppem), font.HintingNone); err == nil && m.CapHeight > 0 {
			fs.capHeight = float64(m.CapHeight) / 64 / ppem
		}
	}
	return fs
}

// Width is how wide s is at size in weight w, the same advances the painter
// places glyphs with.
func (fs *Fonts) Width(s string, size float64, w base.Weight) float64 {
	return fs.shape(s, size, w).width
}

func (fs *Fonts) glyph(r rune, bold bool) *glyph {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	k := glyphKey{r, bold}
	if g, ok := fs.glyphs[k]; ok {
		return g
	}
	g := fs.load(r, bold)
	fs.glyphs[k] = g
	return g
}

func (fs *Fonts) load(r rune, bold bool) *glyph {
	f := fs.faces[0]
	if bold {
		f = fs.faces[1]
	}
	var x sfnt.GlyphIndex
	if f != nil {
		x, _ = f.GlyphIndex(&fs.buf, r)
	}
	if x == 0 && fs.fallback != nil {
		if f = fs.fallback(r); f != nil {
			x, _ = f.GlyphIndex(&fs.buf, r)
		}
	}
	if f == nil || x == 0 {
		return nil
	}
	adv, err := f.GlyphAdvance(&fs.buf, x, fixed.I(ppem), font.HintingNone)
	if err != nil {
		return nil
	}
	segs, err := f.LoadGlyph(&fs.buf, x, fixed.I(ppem), nil)
	if err != nil {
		return nil
	}
	em := func(p fixed.Point26_6) pt { return pt{float64(p.X) / 64 / ppem, float64(p.Y) / 64 / ppem} }
	g := &glyph{advance: float64(adv) / 64 / ppem}
	var cur pt
	for _, s := range segs {
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			cur = em(s.Args[0])
			g.outline = append(g.outline, []seg{{kind: segLine, p: [3]pt{cur}}})
		case sfnt.SegmentOpLineTo:
			cur = em(s.Args[0])
			g.add(seg{kind: segLine, p: [3]pt{cur}})
		case sfnt.SegmentOpQuadTo:
			// TrueType quadratics as the equivalent cubic.
			q, to := em(s.Args[0]), em(s.Args[1])
			c1 := pt{cur.x + 2.0/3*(q.x-cur.x), cur.y + 2.0/3*(q.y-cur.y)}
			c2 := pt{to.x + 2.0/3*(q.x-to.x), to.y + 2.0/3*(q.y-to.y)}
			g.add(seg{kind: segCube, p: [3]pt{c1, c2, to}})
			cur = to
		case sfnt.SegmentOpCubeTo:
			cur = em(s.Args[2])
			g.add(seg{kind: segCube, p: [3]pt{em(s.Args[0]), em(s.Args[1]), cur}})
		}
	}
	return g
}

func (g *glyph) add(s seg) {
	if len(g.outline) == 0 {
		g.outline = append(g.outline, nil)
	}
	k := len(g.outline) - 1
	g.outline[k] = append(g.outline[k], s)
}

// run is one line of text resolved to glyphs and pen positions (no kerning
// or shaping: chart labels are short, and Measure uses the same advances).
type run struct {
	glyphs []placed
	width  float64
}

type placed struct {
	pen     float64
	outline [][]seg
}

func (fs *Fonts) shape(s string, size float64, w base.Weight) run {
	var r run
	for _, c := range s {
		g := fs.glyph(c, w == base.Semibold)
		if g == nil {
			// Nothing to draw: leave a gap rather than nothing.
			r.width += 0.6 * size
			continue
		}
		r.glyphs = append(r.glyphs, placed{pen: r.width, outline: g.outline})
		r.width += g.advance * size
	}
	return r
}
