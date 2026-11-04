package charta

import (
	"math"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/dmath"
	"github.com/linzeyan/charta/scene"
)

// Geometry shared by charts laid out around a center, mirroring the Rust
// crate's chart/polar.rs. Angles are radians running clockwise from twelve
// o'clock, the way people read a clock face and the way pie charts are
// conventionally drawn.

const (
	// tau is a full turn; as a float64 it is the same double as Rust's TAU.
	tau       = 2 * math.Pi
	quarter   = math.Pi / 2
	polarHalo = 1.0
	polarSide = 0.25
)

// polarAt is the point at radius r and angle a around (cx, cy).
func polarAt(cx, cy, r, a float64) (float64, float64) {
	s, c := dmath.Sincos(a)
	return cx + float64(r*s), cy - float64(r*c)
}

// arc appends cubic Béziers tracing the circle of radius r from angle a0 to
// a1 (either direction), starting from the current point. Each piece spans
// at most a quarter turn, where the cubic stays within 0.03% of the true
// arc.
func arc(ops []scene.PathOp, cx, cy, r, a0, a1 float64) []scene.PathOp {
	n := int(math.Max(math.Ceil(math.Abs(a1-a0)/quarter), 1))
	step := (a1 - a0) / float64(n)
	// 4.0 / 3.0 rounds to the same double Rust divides out; the products
	// that follow round one at a time, left to right, as in Rust.
	k := 4.0 / 3.0 * dmath.Tan(step/4) * r
	for i := range n {
		s, e := a0+float64(step*float64(i)), a0+float64(step*float64(i+1))
		x0, y0 := polarAt(cx, cy, r, s)
		x3, y3 := polarAt(cx, cy, r, e)
		ss, sc := dmath.Sincos(s)
		es, ec := dmath.Sincos(e)
		// The tangent at angle a (clockwise from twelve) is (cos a, sin a).
		ops = append(ops, scene.Cubic(
			x0+float64(k*sc),
			y0+float64(k*ss),
			x3-float64(k*ec),
			y3-float64(k*es),
			x3,
			y3,
		))
	}
	return ops
}

// sector is the ring segment between radii r0 < r1 and angles a0 < a1; with
// r0 zero, a pie slice.
func sector(cx, cy, r0, r1, a0, a1 float64) []scene.PathOp {
	x, y := polarAt(cx, cy, r1, a0)
	ops := arc([]scene.PathOp{scene.Move(x, y)}, cx, cy, r1, a0, a1)
	if r0 > 0 {
		x, y := polarAt(cx, cy, r0, a1)
		ops = append(ops, scene.Line(x, y))
		ops = arc(ops, cx, cy, r0, a1, a0)
	} else {
		ops = append(ops, scene.Line(cx, cy))
	}
	return append(ops, scene.Close())
}

func circle(cx, cy, r float64) []scene.PathOp {
	ops := arc([]scene.PathOp{scene.Move(cx, cy-r)}, cx, cy, r, 0, tau)
	return append(ops, scene.Close())
}

// around is where a label for angle a goes just outside radius r: anchored
// on the side facing away from the center, so it never runs back over the
// chart.
func around(cx, cy, r, a float64) (float64, float64, scene.Anchor, scene.Baseline) {
	x, y := polarAt(cx, cy, r, a)
	s, c := dmath.Sincos(a)
	anchor := scene.Middle
	switch {
	case s > polarSide:
		anchor = scene.Start
	case s < -polarSide:
		anchor = scene.End
	}
	baseline := scene.Center
	switch {
	case c > polarSide:
		baseline = scene.Bottom
	case c < -polarSide:
		baseline = scene.Top
	}
	return x, y, anchor, baseline
}

// labelRoom is the room labels placed with around need beyond the radius:
// horizontally the widest label, vertically one line.
func labelRoom(m base.TextMeasure, labels []string) (float64, float64) {
	return maxWidth(m, labels) + gap, tickSize + gap
}

// polarRadius is the largest radius that leaves room for labels around the
// circle, centered between top and bottom.
func polarRadius(width, top, bottom, roomX, roomY float64) float64 {
	return math.Max(math.Min((width-2*pad)/2-roomX, (bottom-top)/2-roomY), 1)
}

// radialTicks is the radial value scale's top (a round number at or above
// hi) and its ticks, zero excluded: the center already reads as zero.
func radialTicks(hi float64) (float64, []float64) {
	top := 1.0
	if hi > 0 {
		_, top = base.Nice(0, hi, 4)
	}
	var ticks []float64
	for _, t := range base.Ticks(0, top, 4) {
		if t > 0 {
			ticks = append(ticks, t)
		}
	}
	return top, ticks
}

// tickLabel is a value label over the plot, on a surface-colored backing so
// the data drawn beneath cannot swallow it. anchor is Start or Middle.
func tickLabel(marks *[]scene.Mark, m base.TextMeasure, x, y float64, anchor scene.Anchor, v float64) {
	label := base.Value(v)
	w := m.Width(label, tickSize, base.Regular)
	left := x
	if anchor == scene.Middle {
		left = x - w/2
	}
	*marks = append(*marks, scene.Mark{
		Shape: scene.RectShape{
			Rect: scene.Rect{
				X: left - polarHalo,
				Y: y - tickSize/2 - polarHalo,
				W: w + 2*polarHalo,
				H: tickSize + 2*polarHalo,
			},
			Radius: scene.AllCorners(2),
		},
		Fill: base.Surface,
	})
	*marks = append(*marks, text(x, y, label, tickSize, base.Regular, anchor, scene.Center, base.TextMuted))
}
