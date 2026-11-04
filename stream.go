package charta

import (
	"fmt"
	"math"
	"slices"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/layout"
	"github.com/linzeyan/charta/scene"
)

// streamSeam is the thin surface outline that separates touching layers.
const streamSeam = 1.0

// StreamChart stacks layers flowing around a moving center line: totals and
// their mix over time, with the wiggle of each layer minimized. Times are
// the sample instants (UTC epoch ms), increasing; each series has one value
// ≥ 0 per time. UTCOffset is the minutes the displayed clock is ahead of
// UTC.
type StreamChart struct {
	Title     string   `json:"title,omitempty"`
	UTCOffset int      `json:"utcOffset,omitempty"`
	Times     []int64  `json:"times"`
	Series    []Series `json:"series"`
}

func (c StreamChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	s, _, err := c.laidOut(width, height, m, nil)
	return s, err
}

func (c StreamChart) PlotScene(width, height float64, m base.TextMeasure) (*scene.Scene, scene.Plot, error) {
	return c.laidOut(width, height, m, nil)
}

func (c StreamChart) View(x [2]float64, y *[2]float64) Viewable {
	return inView[StreamChart]{c, ranges{x, y}}
}

// laidOut, zoomed, keeps the shape the whole stream gives the layers: they
// are laid out over all the data and cut at the view's edges, and only the
// height refits to what is in view.
func (c StreamChart) laidOut(width, height float64, m base.TextMeasure, v *ranges) (*scene.Scene, scene.Plot, error) {
	if err := checkSize(width, height); err != nil {
		return nil, scene.Plot{}, err
	}
	offset := c.UTCOffset
	// Dates, not epoch milliseconds, name the time in errors.
	labels := make([]string, len(c.Times))
	for i, t := range c.Times {
		labels[i] = base.FormatDatetime(t, offset)
	}
	if err := checkSeries(labels, c.Series, maxSeries); err != nil {
		return nil, scene.Plot{}, err
	}
	for i := 1; i < len(c.Times); i++ {
		if c.Times[i] <= c.Times[i-1] {
			return nil, scene.Plot{}, invalid("time %d: times must increase", i)
		}
	}
	for _, s := range c.Series {
		if slices.ContainsFunc(s.Values, func(v float64) bool { return v < 0 }) {
			return nil, scene.Plot{}, invalid("series %q has a negative value; layers cannot be negative", s.Name)
		}
	}
	values := make([][]float64, len(c.Series))
	for i, s := range c.Series {
		values[i] = s.Values
	}
	order := layout.InsideOut(values)
	edges := layout.Wiggle(values, order)
	n := len(c.Times)
	t0, t1 := int64(0), base.Day
	switch {
	case v != nil:
		t0, t1 = int64(math.Round(v.x[0])), int64(math.Round(v.x[1]))
	case n > 0:
		t0, t1 = c.Times[0], c.Times[n-1]
	}
	// The times in view, and for the height one more each side: a layer's
	// curve between two samples stays within their values.
	inside := func(t int64) bool { return t >= t0 && t <= t1 }
	first := slices.IndexFunc(c.Times, func(t int64) bool { return t >= t0 })
	if first < 0 {
		first = n
	}
	last := -1
	for k := n - 1; k >= 0; k-- {
		if c.Times[k] <= t1 {
			last = k
			break
		}
	}
	from, to := max(first-1, 0), max(min(last+1, n-1), 0)
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, es := range edges {
		if from >= len(es) || to >= len(es) {
			continue
		}
		for _, e := range es[from : to+1] {
			lo, hi = math.Min(lo, e.Lo), math.Max(hi, e.Hi)
		}
	}
	if !(lo < hi) {
		lo, hi = 0, 1
	}

	names := make([]string, len(c.Series))
	for i, s := range c.Series {
		names[i] = s.Name
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, names, keySwatch)
	bottom := math.Max(height-pad-tickSize-gap, top+1)
	left, right := pad, math.Max(width-pad, pad+1)
	countX := max(int(math.Round((right-left)/xTickSpacing)), 2)
	x := newTimeAxis(t0, t1, left, right, countX, offset)
	x.fit(m, width-pad)
	// No value axis: the stream's offset is arbitrary, only thickness
	// carries value. The crosshair reads exact values instead.
	y := base.LinearScale{D0: lo, D1: hi, R0: bottom, R1: top}
	x.grid(&marks, top, bottom)
	px0, px1 := x.scale.R0, x.scale.R1
	for _, si := range order {
		upper := make([]scene.Point, n)
		lower := make([]scene.Point, n)
		for k := range n {
			px := x.pos(c.Times[k])
			upper[k] = scene.Point{X: px, Y: y.Map(edges[si][k].Hi)}
			lower[n-1-k] = scene.Point{X: px, Y: y.Map(edges[si][k].Lo)}
		}
		up, down := layout.Monotone(upper, false), layout.Monotone(lower, true)
		if v != nil {
			up, down = layout.ClipX(up, px0, px1, false), layout.ClipX(down, px0, px1, true)
		}
		ops := append(up, down...)
		ops = append(ops, scene.Close())
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: ops},
			Fill:   base.Series(si),
			Stroke: &scene.Stroke{Color: base.Surface, Width: streamSeam},
			Series: c.Series[si].Name,
		})
	}
	shown := 0
	for i, t := range c.Times {
		if !inside(t) {
			continue
		}
		shown++
		cx := x.pos(t)
		halfLeft, halfRight := 0.0, 0.0
		if i > 0 {
			halfLeft = (cx - x.pos(c.Times[i-1])) / 2
		}
		if i+1 < len(c.Times) {
			halfRight = (x.pos(c.Times[i+1]) - cx) / 2
		}
		rows := make([]scene.TipRow, len(c.Series))
		for j, s := range c.Series {
			rows[j] = scene.TipRow{Color: base.Series(j), Name: s.Name, Value: base.Value(s.Values[i])}
		}
		marks = append(marks, scene.Mark{
			Shape:  scene.LineShape{X1: snap(cx), Y1: top, X2: snap(cx), Y2: bottom},
			Stroke: &scene.Stroke{Color: base.Axis, Width: hairline},
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("x%d", i),
				Kind:  scene.HitCrosshair,
				Label: base.FormatDatetime(t, offset),
				Rows:  rows,
				Area:  &scene.Rect{X: cx - halfLeft, Y: top, W: halfLeft + halfRight, H: bottom - top},
			},
		})
		if v != nil {
			// A column reaching past the view's edge stops at it.
			a, b := math.Max(cx-halfLeft, px0), math.Min(cx+halfRight, px1)
			marks[len(marks)-1].Hit.Area = &scene.Rect{X: a, Y: top, W: b - a, H: bottom - top}
		}
	}
	x.labels(&marks, bottom+gap)
	plot := scene.Plot{
		Rect: scene.Rect{X: px0, Y: top, W: px1 - px0, H: bottom - top},
		X:    [2]float64{float64(t0), float64(t1)},
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Stream graph of %s over %s.",
			count(len(c.Series), "series", "series"), count(shown, "time", "times"))),
		Marks: marks,
	}, plot, nil
}
