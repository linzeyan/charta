package charta

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	// maxStems: auto leaf units stop at the smallest power of ten giving this
	// many stems or fewer; past it the display is a column of near-empty rows.
	maxStems = 15
	// stemRowMax lets rows grow up to this height so a short display still
	// fills its space legibly; digits scale with them up to the title size.
	stemRowMax = 28.0
	leafGap    = 6.0
	// unitMin and unitMax are the smallest and largest auto leaf unit, as
	// powers of ten.
	unitMin, unitMax = -6, 12
)

// StemLeafChart splits values into a stem (leading digits) and a leaf (the
// next digit), one row per stem: a histogram that keeps every value
// readable. Values must be non-negative. LeafUnit is what one leaf digit is
// worth, a power of ten (0.1, 1, 10, …); nil picks the finest unit that
// needs at most 15 stems.
type StemLeafChart struct {
	Title    string    `json:"title,omitempty"`
	Values   []float64 `json:"values"`
	LeafUnit *float64  `json:"leafUnit,omitempty"`
}

// leafUnit is a leaf unit as a power of ten: n units is n × 10^p, computed
// as a division for negative p so 29 × 0.01 prints as 0.29.
type leafUnit int

func (u leafUnit) of(n int64) float64 {
	if u < 0 {
		return float64(n) / base.Pow10(int(-u))
	}
	return float64(n) * base.Pow10(int(u))
}

func (u leafUnit) count(v float64) int64 {
	if u < 0 {
		return int64(math.Round(v * base.Pow10(int(-u))))
	}
	return int64(math.Round(v / base.Pow10(int(u))))
}

// Leaves are never negative (negative values are rejected), so Go's
// truncating / and % agree with Rust's div_euclid and rem_euclid here.

func (c StemLeafChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	for i, v := range c.Values {
		if err := finite(fmt.Sprintf("value %d", i), v); err != nil {
			return nil, err
		}
		if v < 0 {
			return nil, invalid("value %d is negative; a stem-and-leaf plot takes values ≥ 0", i)
		}
	}
	sorted := slices.Clone(c.Values)
	slices.Sort(sorted)
	var unit leafUnit
	if c.LeafUnit != nil {
		p := unitMin
		for p <= unitMax && base.Pow10(p) != *c.LeafUnit {
			p++
		}
		if p > unitMax {
			return nil, invalid("leaf unit %s is not a power of ten", strconv.FormatFloat(*c.LeafUnit, 'f', -1, 64))
		}
		unit = leafUnit(p)
	} else {
		lo, hi := 0.0, 0.0
		if len(sorted) > 0 {
			lo, hi = sorted[0], sorted[len(sorted)-1]
		}
		unit = unitMax
		for p := unitMin; p <= unitMax; p++ {
			u := leafUnit(p)
			if u.count(hi)/10-u.count(lo)/10 < maxStems {
				unit = u
				break
			}
		}
	}
	leaves := make([]int64, len(sorted))
	for i, v := range sorted {
		leaves[i] = unit.count(v)
	}
	first, last := int64(0), int64(-1)
	if len(leaves) > 0 {
		first, last = leaves[0]/10, leaves[len(leaves)-1]/10
	}
	var stems []int64
	for s := first; s <= last; s++ {
		stems = append(stems, s)
	}
	rows := make([][]int64, len(stems))
	for k, s := range stems {
		for _, l := range leaves {
			if l/10 == s {
				rows[k] = append(rows[k], l%10)
			}
		}
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	keyH := tickSize + gap
	avail := math.Max(height-pad-keyH-top, 1)
	rowH := math.Min(stemRowMax, avail/float64(max(len(stems), 1)))
	size := math.Min(titleSize, rowH*0.6)
	stemW := 0.0
	for _, s := range stems {
		stemW = math.Max(stemW, m.Width(strconv.FormatInt(s, 10), size, base.Regular))
	}
	digitW := 0.0
	for d := range 10 {
		digitW = math.Max(digitW, m.Width(strconv.Itoa(d), size, base.Regular))
	}
	xStem := pad + stemW
	xSep := xStem + gap
	xLeaf := xSep + gap
	most := 0
	for _, r := range rows {
		most = max(most, len(r))
	}
	most = max(most, 1)
	step := math.Min(digitW+leafGap, (width-pad-xLeaf)/float64(most))

	for k, s := range stems {
		row := rows[k]
		y := top + float64(float64(k)*rowH)
		mid := y + rowH/2
		// A row band that shows only under the pointer: the "crosshair" of
		// this chart, with the row's range and count.
		marks = append(marks, scene.Mark{
			Shape: scene.RectShape{Rect: scene.Rect{X: pad / 2, Y: y, W: width - pad, H: rowH}, Radius: scene.AllCorners(2)},
			Fill:  base.Grid,
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("r%d", k),
				Kind:  scene.HitCrosshair,
				Label: base.Value(unit.of(s*10)) + "–" + base.Value(unit.of(s*10+9)),
				Value: count(len(row), "value", "values"),
			},
		})
		marks = append(marks, text(xStem, mid, strconv.FormatInt(s, 10), size, base.Regular, scene.End, scene.Center, base.TextSecondary))
		// Each leaf sits in a fixed slot, so columns align whatever the font's
		// digit widths.
		for i, l := range row {
			marks = append(marks, text(xLeaf+float64((float64(i)+0.5)*step), mid, strconv.FormatInt(l, 10), size, base.Regular, scene.Middle, scene.Center, base.TextPrimary))
		}
	}
	bottom := top + float64(float64(len(stems))*rowH)
	if len(stems) > 0 {
		marks = append(marks, vline(snap(xSep), top, bottom, base.Axis))
	}
	// The key is what makes the digits readable as values.
	if len(leaves) > 0 {
		l := leaves[0]
		marks = append(marks, text(pad, bottom+gap, fmt.Sprintf("Key: %d | %d = %s", l/10, l%10, base.Value(unit.of(l))), tickSize, base.Regular, scene.Start, scene.Top, base.TextMuted))
	}

	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Stem-and-leaf plot of %s.", count(len(sorted), "value", "values"))),
		Marks:  marks,
	}, nil
}
