package charta

import (
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	pfCellMax = 18.0
	// pfCellMin: smaller boxes than this stop reading as X and O.
	pfCellMin         = 8.0
	pfStroke          = 1.5
	pfDefaultReversal = 3
	// pfEps: prices exactly on a box edge must count as reaching it even
	// after the division leaves them a hair short.
	pfEps = 1e-9
)

var (
	pfRise = base.Series(0)
	pfFall = base.Series(1)
	// pfAutoBoxes are the box counts across the price range tried when no
	// box size is given.
	pfAutoBoxes = [...]int{10, 15, 20, 30, 40, 60}
)

// PointFigureChart draws columns of X (rising) and O (falling) boxes; a new
// column starts only when the price reverses by several boxes. Data is
// prices in time order. BoxSize is the price per box; nil picks a round
// step that fills the plot. Reversal is the boxes the price must reverse to
// start a new column; nil means 3.
type PointFigureChart struct {
	Title     string      `json:"title,omitempty"`
	UTCOffset int         `json:"utcOffset,omitempty"`
	Data      []TimeValue `json:"data"`
	BoxSize   *float64    `json:"boxSize,omitempty"`
	Reversal  *int        `json:"reversal,omitempty"`
}

// pfColumn is a column of boxes low..=high (box k spans k·size ..
// (k+1)·size), begun at data index first.
type pfColumn struct {
	rising    bool
	low, high int64
	first     int
}

// pfColumns: an X box k fills once the price reaches its top edge; an O box
// once it reaches its bottom edge.
func pfColumns(prices []float64, size float64, reversal int) []pfColumn {
	topBox := func(p float64) int64 { return saturatingInt64(math.Floor(p/size+pfEps)) - 1 }
	bottomBox := func(p float64) int64 { return saturatingInt64(math.Ceil(p/size - pfEps)) }
	r := int64(reversal)
	var out []pfColumn
	if len(prices) == 0 {
		return out
	}
	start := saturatingInt64(math.Floor(prices[0]/size + pfEps))
	for i := 1; i < len(prices); i++ {
		p := prices[i]
		if len(out) == 0 {
			if topBox(p) >= start {
				out = append(out, pfColumn{rising: true, low: start, high: topBox(p), first: i})
			} else if bottomBox(p) < start {
				out = append(out, pfColumn{rising: false, low: bottomBox(p), high: start - 1, first: i})
			}
			continue
		}
		c := &out[len(out)-1]
		if c.rising {
			if topBox(p) > c.high {
				c.high = topBox(p)
			} else if bottomBox(p) <= c.high-r {
				out = append(out, pfColumn{rising: false, low: bottomBox(p), high: c.high - 1, first: i})
			}
		} else if bottomBox(p) < c.low {
			c.low = bottomBox(p)
		} else if topBox(p) >= c.low+r {
			out = append(out, pfColumn{rising: true, low: c.low + 1, high: topBox(p), first: i})
		}
	}
	return out
}

// pfBoxPrice is box k's lower edge as an exact decimal: 3 × 0.1 must print
// as 0.3.
func pfBoxPrice(size float64, k int64) float64 {
	inv := math.Round(1 / size)
	if size < 1 && math.Abs(1/inv-size) <= size*1e-12 {
		return float64(k) / inv
	}
	return float64(k) * size
}

// pfFit is the grid for one box size: columns, their box range and square
// cells.
type pfFit struct {
	size   float64
	cols   []pfColumn
	b0, b1 int64
	left   float64
	cell   float64
}

func pfLayout(prices []float64, size float64, reversal int, m base.TextMeasure, width, plotH float64) pfFit {
	cols := pfColumns(prices, size, reversal)
	if len(cols) == 0 {
		return pfFit{size: size, cols: cols, left: pad}
	}
	b0, b1 := int64(math.MaxInt64), int64(math.MinInt64)
	for _, c := range cols {
		b0, b1 = min(b0, c.low), max(b1, c.high)
	}
	labelW := 0.0
	for k := b0; k <= b1+1; k++ {
		labelW = math.Max(labelW, m.Width(base.Value(pfBoxPrice(size, k)), tickSize, base.Regular))
	}
	left := pad + labelW + gap
	cell := math.Min(math.Min((width-pad-left)/float64(len(cols)), plotH/float64(b1-b0+1)), pfCellMax)
	return pfFit{size: size, cols: cols, b0: b0, b1: b1, left: left, cell: cell}
}

// pfFill ranks a fit: legible cells first, then the share of the plot the
// grid covers. Compared like Rust's (bool, f64) tuple.
type pfFill struct {
	legible bool
	used    float64
}

func (a pfFill) better(b pfFill) bool {
	if a.legible != b.legible {
		return a.legible
	}
	return a.used > b.used
}

// saturatingInt64 is Rust's `f as i64`: NaN is 0, out-of-range values
// clamp. Go leaves those conversions implementation-defined.
func saturatingInt64(f float64) int64 {
	switch {
	case math.IsNaN(f):
		return 0
	case f >= 9.223372036854775807e18:
		return math.MaxInt64
	case f <= -9.223372036854775808e18:
		return math.MinInt64
	}
	return int64(f)
}

func (c PointFigureChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if err := checkTimes(c.Data); err != nil {
		return nil, err
	}
	offset := c.UTCOffset
	reversal := pfDefaultReversal
	if c.Reversal != nil {
		reversal = *c.Reversal
	}
	if reversal < 1 {
		return nil, invalid("reversal must be at least 1 box")
	}
	prices := make([]float64, len(c.Data))
	for i, d := range c.Data {
		prices[i] = d.Value
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range prices {
		lo, hi = math.Min(lo, p), math.Max(hi, p)
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = staticKey(&marks, m, top, []keyItem{
		{keySample{kind: sampleLine, color: pfRise, width: 2}, "X: rising"},
		{keySample{kind: sampleLine, color: pfFall, width: 2}, "O: falling"},
	})
	bottom := height - pad - tickSize - gap
	plotH := bottom - top - tickSize/2
	var f pfFit
	switch {
	case c.BoxSize != nil && isFinite(*c.BoxSize) && *c.BoxSize > 0:
		f = pfLayout(prices, *c.BoxSize, reversal, m, width, plotH)
	case c.BoxSize != nil:
		return nil, invalid("box size must be positive")
	case lo < hi:
		// The box size sets both how much noise is filtered and the grid's
		// shape; with cells square, pick the nice size whose grid fills the
		// plot best, among those whose cells stay legible.
		var sizes []float64
		for _, n := range pfAutoBoxes {
			if t := base.Ticks(lo, hi, n); len(t) >= 2 {
				if s := t[1] - t[0]; len(sizes) == 0 || sizes[len(sizes)-1] != s {
					sizes = append(sizes, s)
				}
			}
		}
		area := (width - pad) * plotH
		fill := func(f pfFit) pfFill {
			used := f.cell * float64(len(f.cols)) * f.cell * float64(f.b1-f.b0+1)
			return pfFill{f.cell >= pfCellMin, used / area}
		}
		if len(sizes) == 0 {
			f = pfLayout(prices, 1, reversal, m, width, plotH)
			break
		}
		f = pfLayout(prices, sizes[0], reversal, m, width, plotH)
		for _, s := range sizes[1:] {
			if g := pfLayout(prices, s, reversal, m, width, plotH); fill(g).better(fill(f)) {
				f = g
			}
		}
	default:
		f = pfLayout(prices, 1, reversal, m, width, plotH)
	}
	size, cols, b0, b1, left, cell := f.size, f.cols, f.b0, f.b1, f.left, f.cell
	price := func(k int64) float64 { return pfBoxPrice(size, k) }
	if len(cols) == 0 {
		return &scene.Scene{
			Width:  width,
			Height: height,
			Label:  describe(c.Title, "Point and figure chart with no columns."),
			Marks:  marks,
		}, nil
	}
	rows := float64(b1 - b0 + 1)
	gridBottom := bottom
	gridTop := gridBottom - float64(rows*cell)
	right := left + float64(float64(len(cols))*cell)
	yOf := func(k int64) float64 { return gridBottom - float64(float64(k-b0)*cell) }
	// Label every n-th box edge so labels stay apart.
	every := saturatingInt64(math.Max(math.Ceil(tickSize*2/cell), 1))
	for k := b0; k <= b1+1; k++ {
		if (k-b0)%every == 0 {
			marks = append(marks, hline(left, right, snap(yOf(k)), base.Grid))
			marks = append(marks, text(left-gap, yOf(k), base.Value(price(k)), tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
		}
	}
	inset := cell * 0.2
	for ci, col := range cols {
		x0 := left + float64(float64(ci)*cell)
		for k := col.low; k <= col.high; k++ {
			y1, y0 := yOf(k+1), yOf(k)
			var mk scene.Mark
			if col.rising {
				mk = scene.Mark{
					Shape: scene.PathShape{Ops: []scene.PathOp{
						scene.Move(x0+inset, y1+inset),
						scene.Line(x0+cell-inset, y0-inset),
						scene.Move(x0+cell-inset, y1+inset),
						scene.Line(x0+inset, y0-inset),
					}},
					Stroke: &scene.Stroke{Color: pfRise, Width: pfStroke},
				}
			} else {
				mk = scene.Mark{
					Shape:  scene.CircleShape{CX: x0 + cell/2, CY: (y0 + y1) / 2, R: cell/2 - inset},
					Stroke: &scene.Stroke{Color: pfFall, Width: pfStroke},
				}
			}
			marks = append(marks, mk)
		}
		dir := "Falling"
		if col.rising {
			dir = "Rising"
		}
		marks = append(marks, scene.Mark{
			Shape: scene.RectShape{Rect: scene.Rect{X: x0, Y: gridTop, W: cell, H: gridBottom - gridTop}},
			Hit: &scene.Hit{
				Key:   strconv.Itoa(ci),
				Label: dir + " from " + base.FormatDate(c.Data[col.first].Time, offset),
				Value: base.Value(price(col.low)) + "–" + base.Value(price(col.high+1)),
				Rows:  []scene.TipRow{{Name: "boxes", Value: strconv.FormatInt(col.high-col.low+1, 10)}},
			},
		})
	}
	if n := len(c.Data); n > 0 {
		first, last := base.FormatDate(c.Data[0].Time, offset), base.FormatDate(c.Data[n-1].Time, offset)
		both := m.Width(first, tickSize, base.Regular) + m.Width(last, tickSize, base.Regular) + gap
		marks = append(marks, text(left, gridBottom+gap, first, tickSize, base.Regular, scene.Start, scene.Top, base.TextMuted))
		// A narrow grid keeps only the start date rather than overprint.
		if both <= right-left {
			marks = append(marks, text(right, gridBottom+gap, last, tickSize, base.Regular, scene.End, scene.Top, base.TextMuted))
		}
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Point and figure chart with %s, box size %s, %d-box reversal.",
			count(len(cols), "column", "columns"), base.Value(size), reversal)),
		Marks: marks,
	}, nil
}
