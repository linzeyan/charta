package charta

import (
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	kagiYangWidth       = 3.0
	kagiYinWidth        = 1.5
	kagiDefaultReversal = 4.0
)

// Yang lines (price broke above the last shoulder) are thick and in the
// first slot; yin lines (below the last waist) thin and in the second.
// Thickness is the traditional cue; color repeats it.
var (
	kagiYang = base.Series(0)
	kagiYin  = base.Series(1)
)

// KagiChart draws a line that only turns when the price reverses by a set
// percentage, ignoring time; it thickens on new highs and thins on new lows.
// Data is prices in time order. Reversal is the percent move against the
// line that turns it; nil means 4.
type KagiChart struct {
	Title     string      `json:"title,omitempty"`
	UTCOffset int         `json:"utcOffset,omitempty"`
	Data      []TimeValue `json:"data"`
	Reversal  *float64    `json:"reversal,omitempty"`
}

// kagiStroke is one vertical stroke of the line: from from to to, spanning
// the prices between data indices first and last.
type kagiStroke struct {
	from, to    float64
	first, last int
}

// kagiStrokes: a stroke extends while the price keeps going its way and ends
// when the price retraces pct percent from its extreme.
func kagiStrokes(prices []float64, pct float64) []kagiStroke {
	var out []kagiStroke
	if len(prices) == 0 {
		return out
	}
	p0 := prices[0]
	cur := kagiStroke{from: p0, to: p0}
	dir := 0.0
	for i := 1; i < len(prices); i++ {
		p := prices[i]
		turn := math.Abs(cur.to) * pct / 100
		switch {
		case dir == 0:
			if math.Abs(p-cur.from) >= turn && p != cur.from {
				dir = -1
				if p > cur.from {
					dir = 1
				}
				cur.to = p
				cur.last = i
			}
		case (p-cur.to)*dir > 0:
			cur.to = p
			cur.last = i
		case (cur.to-p)*dir >= turn:
			out = append(out, cur)
			cur = kagiStroke{from: cur.to, to: p, first: cur.last, last: i}
			dir = -dir
		}
	}
	if dir != 0 {
		out = append(out, cur)
	}
	return out
}

// kagiPiece is a run of the line in one thickness.
type kagiPiece struct {
	yang   bool
	points []scene.Point
}

func (c KagiChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if err := checkTimes(c.Data); err != nil {
		return nil, err
	}
	pct := kagiDefaultReversal
	if c.Reversal != nil {
		pct = *c.Reversal
	}
	if !(isFinite(pct) && pct > 0) {
		return nil, invalid("reversal must be a positive percentage")
	}
	offset := c.UTCOffset
	prices := make([]float64, len(c.Data))
	for i, d := range c.Data {
		prices[i] = d.Value
	}
	st := kagiStrokes(prices, pct)
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range prices {
		lo, hi = math.Min(lo, p), math.Max(hi, p)
	}
	if !isFinite(lo) {
		lo, hi = 0, 1
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = staticKey(&marks, m, top, []keyItem{
		{keySample{kind: sampleLine, color: kagiYang, width: kagiYangWidth}, "Yang: above the last high"},
		{keySample{kind: sampleLine, color: kagiYin, width: kagiYinWidth}, "Yin: below the last low"},
	})
	plotTop := top + tickSize/2
	bottom := math.Max(height-pad-tickSize-gap, plotTop+1)
	countY := max(int(math.Round((bottom-plotTop)/tickSpacing)), 2)
	y := newValueAxis(lo, hi, bottom, plotTop, countY)
	left := pad + y.labelWidth(m) + gap
	right := math.Max(width-pad, left+1)
	// Time is not an axis here: each stroke gets an equal column.
	col := (right - left) / float64(max(len(st), 1))
	x := func(k int) float64 { return left + float64(col*(float64(k)+0.5)) }
	py := y.scale.Map

	for _, t := range y.ticks {
		marks = append(marks, hline(left, right, snap(py(t)), base.Grid))
		marks = append(marks, text(left-gap, py(t), y.label(t), tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
	}

	// Walk the line, switching yin/yang where a stroke crosses the last
	// shoulder (top of a rise) or waist (bottom of a fall). The starting
	// price is the first waist (or shoulder) to beat.
	yang := len(st) > 0 && st[0].to > st[0].from
	shoulder, waist := math.Inf(1), math.Inf(-1)
	if len(st) > 0 {
		if yang {
			waist = st[0].from
		} else {
			shoulder = st[0].from
		}
	}
	var pieces []kagiPiece
	push := func(yang bool, a, b scene.Point) {
		if n := len(pieces); n > 0 && pieces[n-1].yang == yang {
			pieces[n-1].points = append(pieces[n-1].points, b)
			return
		}
		pieces = append(pieces, kagiPiece{yang, []scene.Point{a, b}})
	}
	for k, s := range st {
		if k > 0 {
			push(yang, scene.Point{X: x(k - 1), Y: py(s.from)}, scene.Point{X: x(k), Y: py(s.from)})
		}
		up := s.to > s.from
		// A stroke starts on the far side of the level it may break, so the
		// switch happens exactly where it crosses that level.
		level := waist
		breaks := yang && s.to < level
		if up {
			level = shoulder
			breaks = !yang && s.to > level
		}
		if breaks {
			push(yang, scene.Point{X: x(k), Y: py(s.from)}, scene.Point{X: x(k), Y: py(level)})
			yang = up
			push(yang, scene.Point{X: x(k), Y: py(level)}, scene.Point{X: x(k), Y: py(s.to)})
		} else {
			push(yang, scene.Point{X: x(k), Y: py(s.from)}, scene.Point{X: x(k), Y: py(s.to)})
		}
		if up {
			shoulder = s.to
		} else {
			waist = s.to
		}
	}
	for _, p := range pieces {
		stroke := &scene.Stroke{Color: kagiYin, Width: kagiYinWidth}
		if p.yang {
			stroke = &scene.Stroke{Color: kagiYang, Width: kagiYangWidth}
		}
		marks = append(marks, scene.Mark{Shape: scene.PolylineShape{Points: p.points}, Stroke: stroke})
	}
	for k, s := range st {
		a, b := c.Data[s.first], c.Data[s.last]
		marks = append(marks, scene.Mark{
			Shape: scene.RectShape{Rect: scene.Rect{X: x(k) - col/2, Y: plotTop, W: col, H: bottom - plotTop}},
			Hit: &scene.Hit{
				Key:   strconv.Itoa(k),
				Label: base.FormatDate(a.Time, offset) + " – " + base.FormatDate(b.Time, offset),
				Value: base.Value(s.from) + " → " + base.Value(s.to),
			},
		})
	}
	if n := len(c.Data); n > 0 {
		marks = append(marks, text(left, bottom+gap, base.FormatDate(c.Data[0].Time, offset), tickSize, base.Regular, scene.Start, scene.Top, base.TextMuted))
		marks = append(marks, text(right, bottom+gap, base.FormatDate(c.Data[n-1].Time, offset), tickSize, base.Regular, scene.End, scene.Top, base.TextMuted))
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Kagi chart of %s with a %s%% reversal.", count(len(c.Data), "price", "prices"), base.Value(pct))),
		Marks:  marks,
	}, nil
}
