package charta

import (
	"fmt"
	"math"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// Layout pieces shared by charts: title, legend, value axes and the two plot
// frames (categories along x, or categories as rows). Mirrors the Rust
// crate's chart/frame.rs; keep the floating-point operation order identical.

const (
	legendSize    = 12.0
	halo          = 1.0
	legendRow     = 16.0
	legendSwatch  = 10.0
	legendLine    = 16.0
	legendKeyGap  = 6.0
	legendItemGap = 16.0
	// xTickSpacing is the target horizontal distance between value-axis
	// ticks: labels sit side by side, so they need more room than stacked ones.
	xTickSpacing = 80.0
	// sqrtHalf is cos 45°, the same literal as Rust's FRAC_1_SQRT_2.
	sqrtHalf = 0.7071067811865476
)

func text(x, y float64, s string, size float64, w base.Weight, a scene.Anchor, b scene.Baseline, c base.Color) scene.Mark {
	return scene.Mark{
		Shape: scene.TextShape{X: x, Y: y, Text: s, Size: size, Weight: w, Anchor: a, Base: b},
		Fill:  c,
	}
}

func hline(x1, x2, y float64, c base.Color) scene.Mark {
	return scene.Mark{
		Shape:  scene.LineShape{X1: x1, Y1: y, X2: x2, Y2: y},
		Stroke: &scene.Stroke{Color: c, Width: hairline},
	}
}

func vline(x, y1, y2 float64, c base.Color) scene.Mark {
	return scene.Mark{
		Shape:  scene.LineShape{X1: x, Y1: y1, X2: x, Y2: y2},
		Stroke: &scene.Stroke{Color: c, Width: hairline},
	}
}

// haloRect is the backing box haloLabel draws for this text.
func haloRect(m base.TextMeasure, x, y float64, s string, anchor scene.Anchor) scene.Rect {
	w := m.Width(s, tickSize, base.Regular)
	left := x
	switch anchor {
	case scene.Middle:
		left = x - w/2
	case scene.End:
		left = x - w
	}
	return scene.Rect{
		X: left - halo,
		Y: y - tickSize/2 - halo,
		W: w + 2*halo,
		H: tickSize + 2*halo,
	}
}

// haloLabel is a label on a surface-colored backing, so links passing under
// it cannot cross through the text.
func haloLabel(marks *[]scene.Mark, m base.TextMeasure, x, y float64, s string, anchor scene.Anchor, c base.Color) {
	*marks = append(*marks, scene.Mark{
		Shape: scene.RectShape{Rect: haloRect(m, x, y, s, anchor), Radius: scene.AllCorners(2)},
		Fill:  base.Surface,
	})
	*marks = append(*marks, text(x, y, s, tickSize, base.Regular, anchor, scene.Center, c))
}

// rotate turns a category label 45° when labels do not fit their band.
func rotate(m scene.Mark) scene.Mark {
	t := m.Shape.(scene.TextShape)
	t.Anchor, t.Angle = scene.End, -45
	m.Shape = t
	return m
}

// drawTitle draws the title, if any; returns the y where the content below
// starts.
func drawTitle(marks *[]scene.Mark, title string) float64 {
	top := pad
	if title != "" {
		*marks = append(*marks, text(pad, pad, title, titleSize, base.Semibold, scene.Start, scene.Top, base.TextPrimary))
		top += titleSize + 2*gap
	}
	return top
}

// legendKey mirrors the mark: a swatch for bars and areas, a short line for
// lines.
type legendKey int

const (
	keySwatch legendKey = iota
	keyLine
)

// legend draws one row of entries under the title, wrapping when full. Drawn
// only for two or more series: with one, the title already says what is
// plotted. Returns the y where the content below starts.
func legend(marks *[]scene.Mark, m base.TextMeasure, top, width float64, names []string, key legendKey) float64 {
	if len(names) < 2 {
		return top
	}
	keyW, keyH, radius := legendSwatch, legendSwatch, 2.0
	if key == keyLine {
		keyW, keyH, radius = legendLine, 2.0, 1.0
	}
	x, y := pad, top
	for i, name := range names {
		w := keyW + legendKeyGap + m.Width(name, legendSize, base.Regular)
		if x > pad && x+w > width-pad {
			x = pad
			y += legendRow
		}
		cy := y + legendRow/2
		*marks = append(*marks, scene.Mark{
			Shape:  scene.RectShape{Rect: scene.Rect{X: x, Y: cy - keyH/2, W: keyW, H: keyH}, Radius: scene.AllCorners(radius)},
			Fill:   base.Series(i),
			Series: name,
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("legend-%d", i),
				Kind:  scene.HitLegend,
				Label: name,
				Area:  &scene.Rect{X: x - legendKeyGap/2, Y: y, W: w + legendKeyGap, H: legendRow},
			},
		})
		*marks = append(*marks, text(x+keyW+legendKeyGap, cy, name, legendSize, base.Regular, scene.Start, scene.Center, base.TextSecondary))
		x += w + legendItemGap
	}
	return y + legendRow + gap
}

// valueAxis is a nice linear value axis and its tick labels.
type valueAxis struct {
	scale base.LinearScale
	ticks []float64
	fmt   base.TickFormat
}

// newValueAxis widens a degenerate domain (all values equal) upward by one
// unit so the axis still has two ticks.
func newValueAxis(lo, hi, r0, r1 float64, count int) valueAxis {
	if lo == hi {
		hi = lo + 1
	}
	scale := base.LinearScale{D0: lo, D1: hi, R0: r0, R1: r1}.Nice(count)
	ticks := scale.Ticks(count)
	return valueAxis{scale: scale, ticks: ticks, fmt: base.NewTickFormat(ticks)}
}

// exactValueAxis is an axis over exactly [lo, hi] with round ticks inside
// it, for domains that are already meaningful (histogram edges, a density's
// support): nicing would leave empty margins past the data.
func exactValueAxis(lo, hi, r0, r1 float64, count int) valueAxis {
	if lo == hi {
		hi = lo + 1
	}
	scale := base.LinearScale{D0: lo, D1: hi, R0: r0, R1: r1}
	ticks := scale.Ticks(count)
	return valueAxis{scale: scale, ticks: ticks, fmt: base.NewTickFormat(ticks)}
}

// base is where the baseline sits: zero when the domain spans it, otherwise
// the domain's low end.
func (a valueAxis) base() float64 {
	if a.scale.D0 <= 0 && a.scale.D1 >= 0 {
		return 0
	}
	return a.scale.D0
}

func (a valueAxis) label(t float64) string { return a.fmt.Format(t) }

func (a valueAxis) labelWidth(m base.TextMeasure) float64 {
	w := 0.0
	for _, t := range a.ticks {
		w = math.Max(w, m.Width(a.label(t), tickSize, base.Regular))
	}
	return w
}

// lastLabelWidth is the width of the last tick label, which an axis
// centers on its far end.
func (a valueAxis) lastLabelWidth(m base.TextMeasure) float64 {
	if len(a.ticks) == 0 {
		return 0
	}
	return m.Width(a.label(a.ticks[len(a.ticks)-1]), tickSize, base.Regular)
}

func maxWidth(m base.TextMeasure, labels []string) float64 {
	w := 0.0
	for _, l := range labels {
		w = math.Max(w, m.Width(l, tickSize, base.Regular))
	}
	return w
}

// categoryFrame has categories along x (bands) and values up y.
type categoryFrame struct {
	y                        valueAxis
	band                     base.BandScale
	left, right, top, bottom float64
	rotated                  bool
}

// newCategoryFrame lays out a category frame below top. reserveRight keeps
// room for direct labels at the right edge.
func newCategoryFrame(m base.TextMeasure, width, height, top float64, labels []string, lo, hi, paddingInner, paddingOuter, reserveRight float64) categoryFrame {
	// Room for the top tick label, which is centered on its gridline.
	plotTop := top + tickSize/2
	maxLabel := maxWidth(m, labels)
	// The x-label height depends on whether labels fit their band, which
	// depends on the plot width, which depends on the y-label width: lay out
	// once with horizontal labels, and again if they must rotate.
	rotated := false
	xArea := tickSize + gap
	for {
		bottom := height - pad - xArea
		plotH := math.Max(bottom-plotTop, 1)
		count := max(int(math.Round(plotH/tickSpacing)), 2)
		y := newValueAxis(lo, hi, bottom, plotTop, count)
		left := pad + y.labelWidth(m) + gap
		right := math.Max(width-pad-reserveRight, left+1)
		band := base.NewBandScale(len(labels), left, right, paddingInner, paddingOuter)
		if !rotated && len(labels) > 0 && maxLabel > band.Step() {
			rotated = true
			xArea = float64((maxLabel+tickSize)*sqrtHalf) + gap
			continue
		}
		return categoryFrame{y: y, band: band, left: left, right: right, top: plotTop, bottom: bottom, rotated: rotated}
	}
}

func (f categoryFrame) grid(marks *[]scene.Mark) {
	b := f.y.base()
	for _, t := range f.y.ticks {
		if t != b {
			*marks = append(*marks, hline(f.left, f.right, snap(f.y.scale.Map(t)), base.Grid))
		}
	}
}

func (f categoryFrame) yLabels(marks *[]scene.Mark) {
	for _, t := range f.y.ticks {
		*marks = append(*marks, text(f.left-gap, f.y.scale.Map(t), f.y.label(t), tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
	}
}

func (f categoryFrame) baseline(marks *[]scene.Mark) {
	*marks = append(*marks, hline(f.left, f.right, snap(f.y.scale.Map(f.y.base())), base.Axis))
}

func (f categoryFrame) xLabels(marks *[]scene.Mark, labels []string) {
	for i, l := range labels {
		mk := text(f.center(i), f.bottom+gap, l, tickSize, base.Regular, scene.Middle, scene.Top, base.TextMuted)
		if f.rotated {
			mk = rotate(mk)
		}
		*marks = append(*marks, mk)
	}
}

func (f categoryFrame) center(i int) float64 {
	return f.band.Position(i) + f.band.Bandwidth()/2
}

// column is band i's whole column, padding included: the hover target, so
// short marks are as easy to hit as tall ones.
func (f categoryFrame) column(i int) scene.Rect {
	b := f.band
	return scene.Rect{X: b.Position(i) - (b.Step()-b.Bandwidth())/2, Y: f.top, W: b.Step(), H: f.bottom - f.top}
}

// xyFrame has two value axes: x continuous along the bottom, y up the left.
type xyFrame struct {
	x, y                     valueAxis
	left, right, top, bottom float64
}

// newXYFrame lays out an x/y frame below top. xExact keeps the x domain as
// given (see exactValueAxis); y is always niced. yTicks caps the y tick
// count (counts on a histogram never need fractional ticks).
func newXYFrame(m base.TextMeasure, width, height, top, xlo, xhi float64, xExact bool, ylo, yhi float64, yTicks int, xLabel bool) xyFrame {
	plotTop := top + tickSize/2
	xArea := tickSize + gap
	if xLabel {
		xArea += tickSize + gap
	}
	bottom := math.Max(height-pad-xArea, plotTop+1)
	countY := min(max(int(math.Round((bottom-plotTop)/tickSpacing)), 2), yTicks)
	y := newValueAxis(ylo, yhi, bottom, plotTop, countY)
	left := pad + y.labelWidth(m) + gap
	right := math.Max(width-pad, left+1)
	countX := max(int(math.Round((right-left)/xTickSpacing)), 2)
	var x valueAxis
	if xExact {
		x = exactValueAxis(xlo, xhi, left, right, countX)
	} else {
		x = newValueAxis(xlo, xhi, left, right, countX)
	}
	// Keep the last tick label inside the padding when it sits at the edge.
	overhang := 0.0
	if len(x.ticks) > 0 {
		t := x.ticks[len(x.ticks)-1]
		w := m.Width(x.label(t), tickSize, base.Regular) / 2
		overhang = math.Max(x.scale.Map(t)+w-(width-pad), 0)
	}
	x.scale.R1 = math.Max(right-overhang, left+1)
	return xyFrame{x: x, y: y, left: left, right: x.scale.R1, top: plotTop, bottom: bottom}
}

// grid draws horizontal gridlines at every y tick but the baseline's.
func (f xyFrame) grid(marks *[]scene.Mark) {
	b := f.y.base()
	for _, t := range f.y.ticks {
		if t != b {
			*marks = append(*marks, hline(f.left, f.right, snap(f.y.scale.Map(t)), base.Grid))
		}
	}
}

// labels draws both axes' tick labels and, when given, the x quantity's
// name under the x labels at the right end.
func (f xyFrame) labels(marks *[]scene.Mark, xLabel string) {
	for _, t := range f.y.ticks {
		*marks = append(*marks, text(f.left-gap, f.y.scale.Map(t), f.y.label(t), tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
	}
	for _, t := range f.x.ticks {
		*marks = append(*marks, text(f.x.scale.Map(t), f.bottom+gap, f.x.label(t), tickSize, base.Regular, scene.Middle, scene.Top, base.TextMuted))
	}
	if xLabel != "" {
		*marks = append(*marks, text(f.right, f.bottom+gap+tickSize+gap, xLabel, tickSize, base.Regular, scene.End, scene.Top, base.TextMuted))
	}
}

func (f xyFrame) baseline(marks *[]scene.Mark) {
	*marks = append(*marks, hline(f.left, f.right, snap(f.y.scale.Map(f.y.base())), base.Axis))
}

// rowFrame has categories as rows (bands down y) and values along x.
type rowFrame struct {
	x                        valueAxis
	band                     base.BandScale
	left, right, top, bottom float64
}

func newRowFrame(m base.TextMeasure, width, height, top float64, labels []string, lo, hi, paddingInner, paddingOuter float64) rowFrame {
	left := pad + maxWidth(m, labels) + gap
	bottom := math.Max(height-pad-tickSize-gap, top+1)
	right := math.Max(width-pad, left+1)
	count := max(int(math.Round((right-left)/xTickSpacing)), 2)
	x := newValueAxis(lo, hi, left, right, count)
	// The last tick label is centered on the right edge; pull the plot in so
	// it does not spill past the padding.
	last := x.lastLabelWidth(m)
	x.scale.R1 = math.Max(right-last/2, left+1)
	band := base.NewBandScale(len(labels), top, bottom, paddingInner, paddingOuter)
	return rowFrame{x: x, band: band, left: left, right: x.scale.R1, top: top, bottom: bottom}
}

// grid draws gridlines at every tick, zero included: a row frame has no
// baseline (spans float; nothing grows from zero).
func (f rowFrame) grid(marks *[]scene.Mark) {
	for _, t := range f.x.ticks {
		*marks = append(*marks, vline(snap(f.x.scale.Map(t)), f.top, f.bottom, base.Grid))
	}
}

func (f rowFrame) xLabels(marks *[]scene.Mark) {
	for _, t := range f.x.ticks {
		*marks = append(*marks, text(f.x.scale.Map(t), f.bottom+gap, f.x.label(t), tickSize, base.Regular, scene.Middle, scene.Top, base.TextMuted))
	}
}

func (f rowFrame) rowLabels(marks *[]scene.Mark, labels []string) {
	for i, l := range labels {
		*marks = append(*marks, text(f.left-gap, f.center(i), l, tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
	}
}

func (f rowFrame) center(i int) float64 {
	return f.band.Position(i) + f.band.Bandwidth()/2
}

// row is row i's full width and step: the hover target.
func (f rowFrame) row(i int) scene.Rect {
	b := f.band
	return scene.Rect{X: f.left, Y: b.Position(i) - (b.Step()-b.Bandwidth())/2, W: f.right - f.left, H: b.Step()}
}

const (
	swatchW  = 16.0
	swatchH  = 8.0
	scaleRow = 16.0
)

// seqLegend is the sequential scale's legend under the title: low value,
// the ramp's classes, high value. Returns the y where the content below
// starts.
func seqLegend(marks *[]scene.Mark, m base.TextMeasure, top, lo, hi float64) float64 {
	tf := base.NewTickFormat([]float64{lo, hi})
	loS, hiS := tf.Format(lo), tf.Format(hi)
	cy := top + scaleRow/2
	*marks = append(*marks, text(pad, cy, loS, tickSize, base.Regular, scene.Start, scene.Center, base.TextMuted))
	x := pad + m.Width(loS, tickSize, base.Regular) + gap
	for k := range base.SeqClasses {
		*marks = append(*marks, scene.Mark{
			Shape: scene.RectShape{Rect: scene.Rect{X: x, Y: cy - swatchH/2, W: swatchW, H: swatchH}},
			Fill:  base.Seq(k),
		})
		x += swatchW
	}
	*marks = append(*marks, text(x+gap, cy, hiS, tickSize, base.Regular, scene.Start, scene.Center, base.TextMuted))
	return top + scaleRow + gap
}

// seqClass splits [lo, hi] into equal-width classes; the top value lands in
// the last class. A flat field sits in the middle class.
func seqClass(v, lo, hi float64) int {
	if hi == lo {
		return base.SeqClasses / 2
	}
	k := int(math.Floor((v - lo) / (hi - lo) * base.SeqClasses))
	return min(k, base.SeqClasses-1)
}

// sampleKind is how a static key entry draws its sample.
type sampleKind int

const (
	// sampleHollow is an outlined, surface-filled box.
	sampleHollow sampleKind = iota
	sampleSolid
	// sampleLine is a short stroke of the sample's width.
	sampleLine
)

type keySample struct {
	kind  sampleKind
	color base.Color
	width float64
}

type keyItem struct {
	sample keySample
	name   string
}

// staticKey is a legend row that explains an encoding rather than naming
// series: nothing to toggle, so no hit targets. Returns the y below it.
func staticKey(marks *[]scene.Mark, m base.TextMeasure, top float64, items []keyItem) float64 {
	cy := top + legendRow/2
	x := pad
	for _, it := range items {
		s := it.sample
		keyW, keyH := legendSwatch, legendSwatch
		if s.kind == sampleLine {
			keyW, keyH = legendLine, s.width
		}
		r := scene.Rect{X: x, Y: cy - keyH/2, W: keyW, H: keyH}
		if s.kind == sampleHollow {
			*marks = append(*marks, scene.Mark{
				Shape:  scene.PathShape{Ops: rectOps(r)},
				Fill:   base.Surface,
				Stroke: &scene.Stroke{Color: s.color, Width: 1.5},
			})
		} else {
			*marks = append(*marks, scene.Mark{
				Shape: scene.RectShape{Rect: r, Radius: scene.AllCorners(1)},
				Fill:  s.color,
			})
		}
		*marks = append(*marks, text(x+keyW+legendKeyGap, cy, it.name, legendSize, base.Regular, scene.Start, scene.Center, base.TextSecondary))
		x += keyW + legendKeyGap + m.Width(it.name, legendSize, base.Regular) + legendItemGap
	}
	return top + legendRow + gap
}

// rectOps is a rectangle as a closed path, for outlines (rect marks only
// fill).
func rectOps(r scene.Rect) []scene.PathOp {
	return []scene.PathOp{
		scene.Move(r.X, r.Y),
		scene.Line(r.X+r.W, r.Y),
		scene.Line(r.X+r.W, r.Y+r.H),
		scene.Line(r.X, r.Y+r.H),
		scene.Close(),
	}
}

// timeAxis maps UTC epoch ms linearly, with ticks on local round boundaries
// (offset minutes ahead of UTC).
type timeAxis struct {
	scale    base.LinearScale
	ticks    []int64
	interval base.Interval
	offset   int
}

// newTimeAxis widens a zero-length span to one day so a single instant
// still has an axis around it.
func newTimeAxis(lo, hi int64, r0, r1 float64, count, offset int) timeAxis {
	if lo == hi {
		hi = lo + base.Day
	}
	iv := base.IntervalFor(lo, hi, count)
	return timeAxis{
		scale:    base.LinearScale{D0: float64(lo), D1: float64(hi), R0: r0, R1: r1},
		ticks:    base.TimeTicks(lo, hi, iv, offset),
		interval: iv,
		offset:   offset,
	}
}

func (a timeAxis) pos(t int64) float64 { return a.scale.Map(float64(t)) }

func (a timeAxis) label(t int64) string { return base.TimeTickLabel(t, a.interval, a.offset) }

// labels centers a tick label under each tick at y.
func (a timeAxis) labels(marks *[]scene.Mark, y float64) {
	for _, t := range a.ticks {
		*marks = append(*marks, text(a.pos(t), y, a.label(t), tickSize, base.Regular, scene.Middle, scene.Top, base.TextMuted))
	}
}

// grid draws vertical gridlines at every tick between top and bottom.
func (a timeAxis) grid(marks *[]scene.Mark, top, bottom float64) {
	for _, t := range a.ticks {
		*marks = append(*marks, vline(snap(a.pos(t)), top, bottom, base.Grid))
	}
}

// fit pulls the range's end in so the last tick label, centered on its
// tick, stays inside limit.
func (a *timeAxis) fit(m base.TextMeasure, limit float64) {
	if len(a.ticks) == 0 {
		return
	}
	t := a.ticks[len(a.ticks)-1]
	w := m.Width(a.label(t), tickSize, base.Regular) / 2
	over := math.Max(a.pos(t)+w-limit, 0)
	a.scale.R1 = math.Max(a.scale.R1-over, a.scale.R0+1)
}

// timeFrame has time along x and values up y.
type timeFrame struct {
	x                        timeAxis
	y                        valueAxis
	left, right, top, bottom float64
}

// newTimeFrame lays out a time frame below top over [t0, t1] (already
// padded by the caller) and the value range [lo, hi], niced.
func newTimeFrame(m base.TextMeasure, width, height, top float64, t0, t1 int64, lo, hi float64, offset int) timeFrame {
	plotTop := top + tickSize/2
	bottom := math.Max(height-pad-tickSize-gap, plotTop+1)
	countY := max(int(math.Round((bottom-plotTop)/tickSpacing)), 2)
	y := newValueAxis(lo, hi, bottom, plotTop, countY)
	left := pad + y.labelWidth(m) + gap
	right := math.Max(width-pad, left+1)
	countX := max(int(math.Round((right-left)/xTickSpacing)), 2)
	x := newTimeAxis(t0, t1, left, right, countX, offset)
	x.fit(m, width-pad)
	return timeFrame{x: x, y: y, left: left, right: x.scale.R1, top: plotTop, bottom: bottom}
}

// grid draws horizontal gridlines at every y tick but the baseline's.
func (f timeFrame) grid(marks *[]scene.Mark) {
	b := f.y.base()
	for _, t := range f.y.ticks {
		if t != b {
			*marks = append(*marks, hline(f.left, f.right, snap(f.y.scale.Map(t)), base.Grid))
		}
	}
}

func (f timeFrame) labels(marks *[]scene.Mark) {
	for _, t := range f.y.ticks {
		*marks = append(*marks, text(f.left-gap, f.y.scale.Map(t), f.y.label(t), tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
	}
	f.x.labels(marks, f.bottom+gap)
}

func (f timeFrame) baseline(marks *[]scene.Mark) {
	*marks = append(*marks, hline(f.left, f.right, snap(f.y.scale.Map(f.y.base())), base.Axis))
}
