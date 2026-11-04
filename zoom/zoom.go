// Package zoom holds the part of its data a zoomed chart shows, and how
// wheel, drag and brush gestures move it. Backends own the gestures; the
// arithmetic lives here so every backend, and the Rust crate, zooms alike.
//
// A category axis zooms by Window: the chart implements charta.Zoomable,
// reporting its category count and narrowing itself to a window. A
// continuous axis (time or numbers) zooms by View: the chart implements
// charta.Viewable, reporting where its plot sits and showing a range;
// scatter and bubble charts zoom y as well. Either way the axes then fit
// what is in view, which is the point of zooming in.
package zoom

import (
	"math"

	"github.com/linzeyan/charta/internal/dmath"
	"github.com/linzeyan/charta/scene"
)

// MinSpan: narrower than two categories there is no line to read.
const MinSpan = 2

// WheelStep is the share of the categories each wheel notch shows: in by
// 0.8, out by 1/0.8.
const WheelStep = 0.8

// Window is the visible categories, Start up to (not including) End, out of
// Len.
type Window struct {
	Start, End, Len int
}

// Full is every one of n categories.
func Full(n int) Window { return Window{Start: 0, End: n, Len: n} }

func (w Window) IsFull() bool { return w.Start == 0 && w.End == w.Len }

func (w Window) span() int { return w.End - w.Start }

// Wheel zooms by notches wheel steps (positive is in) keeping the category
// under the pointer, at (a fractional index into the window, from
// ColumnAt), where it is.
func (w Window) Wheel(at, notches float64) Window {
	want := math.Round(float64(w.span()) * dmath.Pow(WheelStep, notches))
	// Clamped as a float: a far zoom out overflows an int conversion, which
	// Go leaves implementation-defined where Rust's `as` saturates.
	lo := float64(min(MinSpan, w.Len))
	want = math.Max(lo, math.Min(want, float64(w.Len)))
	return w.resized(at, int(want))
}

func (w Window) resized(at float64, span int) Window {
	in := math.Max(0, math.Min(at, float64(w.span())))
	share := in / float64(max(w.span(), 1))
	anchor := float64(w.Start) + in
	start := math.Max(0, math.Round(anchor-float64(share*float64(span))))
	return w.placed(int(start), span)
}

// Pan shifts the window by whole categories (positive shows later ones),
// stopping at either end.
func (w Window) Pan(by int) Window {
	return w.placed(max(w.Start+by, 0), w.span())
}

// Brush zooms to the categories between two brushed columns of the window,
// both included. A brush narrower than MinSpan leaves the window as it is.
func (w Window) Brush(a, b int) Window {
	lo, hi := min(a, b), max(a, b)
	if hi-lo+1 < MinSpan {
		return w
	}
	return w.placed(w.Start+lo, hi-lo+1)
}

func (w Window) placed(start, span int) Window {
	span = min(span, w.Len)
	start = min(start, w.Len-span)
	return Window{Start: start, End: start + span, Len: w.Len}
}

// columns are the scene's category columns: its crosshair hit areas, one
// per category, in order.
func columns(sc *scene.Scene) []scene.Rect {
	var cols []scene.Rect
	for _, m := range sc.Marks {
		if m.Hit != nil && m.Hit.Kind == scene.HitCrosshair && m.Hit.Area != nil {
			cols = append(cols, *m.Hit.Area)
		}
	}
	return cols
}

// ColumnAt is where x falls along a scene's category columns, as a
// fractional index from the first column's left edge, clamped to the
// columns; false when the scene has none.
func ColumnAt(sc *scene.Scene, x float64) (float64, bool) {
	cols := columns(sc)
	if len(cols) == 0 {
		return 0, false
	}
	first, last := cols[0], cols[len(cols)-1]
	x = math.Max(first.X, math.Min(x, last.X+last.W))
	k := len(cols) - 1
	for i, c := range cols {
		if x < c.X+c.W {
			k = i
			break
		}
	}
	c := cols[k]
	frac := 0.0
	if c.W > 0 {
		frac = (x - c.X) / c.W
	}
	return float64(k) + frac, true
}

// ColumnX is the x of a fractional column index, the inverse of ColumnAt:
// where to draw a brush edge.
func ColumnX(sc *scene.Scene, at float64) (float64, bool) {
	first, ok := firstColumn(sc)
	if !ok {
		return 0, false
	}
	return first.X + float64(at*first.W), true
}

// BrushEdges is the column edges a brush from column from to to
// (fractional, from ColumnAt) covers, whichever way it was dragged: both end
// columns whole, as Window.Brush will zoom to them.
func BrushEdges(from, to float64) (float64, float64) {
	a, b := math.Floor(from), math.Floor(to)
	return math.Min(a, b), math.Max(a, b) + 1
}

// ColumnWidth is the width of one category column, for turning drag
// distance into a pan.
func ColumnWidth(sc *scene.Scene) (float64, bool) {
	first, ok := firstColumn(sc)
	return first.W, ok
}

func firstColumn(sc *scene.Scene) (scene.Rect, bool) {
	for _, m := range sc.Marks {
		if m.Hit != nil && m.Hit.Kind == scene.HitCrosshair {
			if m.Hit.Area == nil {
				return scene.Rect{}, false
			}
			return *m.Hit.Area, true
		}
	}
	return scene.Rect{}, false
}

// MinShare is the narrowest continuous view: this share of the full extent.
const MinShare = 1e-3

// MinBrush: a brush shorter than this (px) along a zoomed axis is a slip of
// the hand, not a range.
const MinBrush = 4.0

// Span is the shown part Lo..Hi of a continuous axis whose whole extent is
// Full.
type Span struct {
	Lo, Hi float64
	Full   [2]float64
}

// FullSpan is all of lo..hi.
func FullSpan(lo, hi float64) Span { return Span{Lo: lo, Hi: hi, Full: [2]float64{lo, hi}} }

func (s Span) IsFull() bool { return s.Lo == s.Full[0] && s.Hi == s.Full[1] }

func (s Span) width() float64 { return s.Hi - s.Lo }

// Wheel zooms by notches wheel steps (positive is in) keeping the value at
// where it is on screen.
func (s Span) Wheel(at, notches float64) Span {
	w := float64(s.width() * dmath.Pow(WheelStep, notches))
	share := math.Max(0, math.Min((at-s.Lo)/s.width(), 1))
	return s.placed(at-float64(share*w), w)
}

// Pan shifts the span by by (positive shows later values), stopping at
// either end.
func (s Span) Pan(by float64) Span { return s.placed(s.Lo+by, s.width()) }

// Brush zooms to between a and b, either way round.
func (s Span) Brush(a, b float64) Span {
	lo, hi := math.Min(a, b), math.Max(a, b)
	return s.placed(lo, hi-lo)
}

// placed is lo..lo+w, the width kept between MinShare and all of it, slid
// back inside the full extent.
func (s Span) placed(lo, w float64) Span {
	f0, f1 := s.Full[0], s.Full[1]
	w = math.Max(w, float64((f1-f0)*MinShare))
	if w >= f1-f0 {
		return FullSpan(f0, f1)
	}
	lo = math.Max(f0, math.Min(lo, f1-w))
	return Span{Lo: lo, Hi: lo + w, Full: s.Full}
}

// View is what a continuous zoom shows: an x span and, for charts that zoom
// both axes, a y span. Gestures arrive as scene coordinates and map to data
// through the scene.Plot of the scene they happened on.
type View struct {
	X Span
	Y *Span
}

// FullView is everything: the view of a chart whose full scene has plot.
func FullView(plot scene.Plot) View {
	v := View{X: FullSpan(plot.X[0], plot.X[1])}
	if plot.Y != nil {
		y := FullSpan(plot.Y[0], plot.Y[1])
		v.Y = &y
	}
	return v
}

func (v View) IsFull() bool { return v.X.IsFull() && (v.Y == nil || v.Y.IsFull()) }

// Fits reports whether this view belongs to the chart whose full scene has
// plot; a view kept from before the data changed does not.
func (v View) Fits(plot scene.Plot) bool {
	if v.X.Full != plot.X || (v.Y == nil) != (plot.Y == nil) {
		return false
	}
	return v.Y == nil || v.Y.Full == *plot.Y
}

// Equal reports whether two views show the same ranges.
func (v View) Equal(o View) bool {
	if v.X != o.X || (v.Y == nil) != (o.Y == nil) {
		return false
	}
	return v.Y == nil || *v.Y == *o.Y
}

// Ranges is what to pass to charta.Viewable.View.
func (v View) Ranges() ([2]float64, *[2]float64) {
	x := [2]float64{v.X.Lo, v.X.Hi}
	if v.Y == nil {
		return x, nil
	}
	return x, &[2]float64{v.Y.Lo, v.Y.Hi}
}

// withY is v with y replaced by the result of f on it, when v has a y.
func (v View) withY(f func(Span) Span) *Span {
	if v.Y == nil {
		return nil
	}
	y := f(*v.Y)
	return &y
}

// Wheel zooms by notches wheel steps around the pointer at (px, py).
func (v View) Wheel(plot scene.Plot, px, py, notches float64) View {
	x, y := dataAt(plot, px, py)
	return View{X: v.X.Wheel(x, notches), Y: v.withY(func(s Span) Span { return s.Wheel(y, notches) })}
}

// Pan pans so the data follows a pointer that moved by (dx, dy) px.
func (v View) Pan(plot scene.Plot, dx, dy float64) View {
	r := plot.Rect
	return View{
		X: v.X.Pan(float64(-dx*v.X.width()) / math.Max(r.W, 1)),
		Y: v.withY(func(s Span) Span { return s.Pan(float64(dy*s.width()) / math.Max(r.H, 1)) }),
	}
}

// Brush zooms to the range brushed from (x0, y0) to (x1, y1): along x, and
// for a chart that zooms y, along y too. A brush shorter than MinBrush on a
// zoomed axis leaves the view as it is.
func (v View) Brush(plot scene.Plot, x0, y0, x1, y1 float64) View {
	if math.Abs(x1-x0) < MinBrush || (v.Y != nil && math.Abs(y1-y0) < MinBrush) {
		return v
	}
	ax, ay := dataAt(plot, x0, y0)
	bx, by := dataAt(plot, x1, y1)
	return View{X: v.X.Brush(ax, bx), Y: v.withY(func(s Span) Span { return s.Brush(ay, by) })}
}

// BrushRect is what to draw while brushing from (x0, y0) to (x1, y1): a band
// across the plot's height, or for a chart that zooms y, a box; inside the
// plot.
func (v View) BrushRect(plot scene.Plot, x0, y0, x1, y1 float64) scene.Rect {
	r := plot.Rect
	cx := func(x float64) float64 { return math.Max(r.X, math.Min(x, r.X+r.W)) }
	cy := func(y float64) float64 { return math.Max(r.Y, math.Min(y, r.Y+r.H)) }
	l, rt := cx(math.Min(x0, x1)), cx(math.Max(x0, x1))
	t, b := r.Y, r.Y+r.H
	if v.Y != nil {
		t, b = cy(math.Min(y0, y1)), cy(math.Max(y0, y1))
	}
	return scene.Rect{X: l, Y: t, W: rt - l, H: b - t}
}

// dataAt is the data under the scene point (px, py), clamped to the plot: x,
// and y for a plot that has it (screen y grows downward, data y upward).
func dataAt(plot scene.Plot, px, py float64) (float64, float64) {
	r := plot.Rect
	tx := 0.0
	if r.W > 0 {
		tx = math.Max(0, math.Min((px-r.X)/r.W, 1))
	}
	x := plot.X[0] + float64(tx*(plot.X[1]-plot.X[0]))
	if plot.Y == nil {
		return x, 0
	}
	ty := 0.0
	if r.H > 0 {
		ty = math.Max(0, math.Min((r.Y+r.H-py)/r.H, 1))
	}
	return x, plot.Y[0] + float64(ty*(plot.Y[1]-plot.Y[0]))
}
