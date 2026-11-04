package base

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/linzeyan/charta/assets"
)

// Weight is a font weight charts use.
type Weight int

const (
	Regular Weight = iota
	Semibold
)

// TextMeasure measures the advance width of a single line of text, in pixels.
type TextMeasure interface {
	Width(text string, size float64, weight Weight) float64
}

// CapHeight is Inter's cap height as a fraction of the font size, used to
// place text by its visible top or middle rather than by font ascent.
const CapHeight = 0.727

// fallbackEm is the width of a character missing from the table and not
// full-width.
const fallbackEm = 0.6

// InterMetrics is the default measurer: Inter metrics for Latin text, 1em
// for CJK. Layout only needs advance widths, so it reads a generated table
// instead of embedding a font.
type InterMetrics struct{}

type widthTable struct {
	upem   uint64
	widths map[rune][2]uint64
}

var table = sync.OnceValue(func() widthTable {
	t := widthTable{widths: map[rune][2]uint64{}}
	for _, line := range strings.Split(assets.InterWidths, "\n") {
		f := strings.Fields(line)
		switch {
		case len(f) == 0 || strings.HasPrefix(f[0], "#"):
		case len(f) == 2 && f[0] == "upem":
			t.upem = mustUint(f[1], 10)
		case len(f) == 3:
			cp := mustUint(f[0], 16)
			t.widths[rune(cp)] = [2]uint64{mustUint(f[1], 10), mustUint(f[2], 10)}
		default:
			panic("malformed inter-widths line: " + line)
		}
	}
	if t.upem == 0 {
		panic("inter-widths.txt lacks upem")
	}
	return t
})

func mustUint(s string, base int) uint64 {
	v, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		panic(fmt.Sprintf("inter-widths: %v", err))
	}
	return v
}

// isWide reports East Asian wide characters, which CJK fonts draw one em wide.
func isWide(c rune) bool {
	switch {
	case c >= 0x1100 && c <= 0x115F,
		c >= 0x2E80 && c <= 0xA4CF,
		c >= 0xAC00 && c <= 0xD7A3,
		c >= 0xF900 && c <= 0xFAFF,
		c >= 0xFE30 && c <= 0xFE4F,
		c >= 0xFF00 && c <= 0xFF60,
		c >= 0xFFE0 && c <= 0xFFE6,
		c >= 0x20000 && c <= 0x3FFFD:
		return true
	}
	return false
}

func (InterMetrics) Width(text string, size float64, weight Weight) float64 {
	t := table()
	var units, wide, other uint64
	for _, c := range text {
		w, ok := t.widths[c]
		switch {
		case ok && weight == Regular:
			units += w[0]
		case ok:
			units += w[1]
		case isWide(c):
			wide++
		default:
			other++
		}
	}
	return float64(float64(units)*size/float64(t.upem)) + float64(float64(wide)*size) + float64(float64(other)*size*fallbackEm)
}
