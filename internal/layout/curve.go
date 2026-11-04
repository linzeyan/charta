package layout

import (
	"math"

	"github.com/linzeyan/charta/scene"
)

// Monotone cubic interpolation through points sorted by x, ported from
// d3-shape curveMonotoneX (Steffen 1990; ISC license, Mike Bostock). The
// curve never overshoots the data: between two points it stays within their
// y range, so a smoothed series never shows a peak that is not there.

// sign is d3's rule: zero counts as positive.
func sign(x float64) float64 {
	if x < 0 {
		return -1
	}
	return 1
}

// divisor is a segment's width as a divisor. A zero-width segment divides
// by a zero signed like its neighbor, as in d3, so its slope is an infinity
// pointing the right way instead of NaN.
func divisor(h, neighbor float64) float64 {
	switch {
	case h != 0:
		return h
	case neighbor < 0:
		return math.Copysign(0, -1)
	}
	return 0
}

// rustMin is Rust's f64::min: a NaN operand is ignored, where math.Min
// would return it. Zero-width segments make infinite slopes, and inf·0 is
// NaN, so the difference shows.
func rustMin(a, b float64) float64 {
	switch {
	case math.IsNaN(a):
		return b
	case math.IsNaN(b):
		return a
	}
	return math.Min(a, b)
}

// slope3 is the tangent at the middle of three points.
func slope3(p0, p1, p2 scene.Point) float64 {
	h0, h1 := p1.X-p0.X, p2.X-p1.X
	s0 := (p1.Y - p0.Y) / divisor(h0, h1)
	s1 := (p2.Y - p1.Y) / divisor(h1, h0)
	p := (float64(s0*h1) + float64(s1*h0)) / (h0 + h1)
	t := (sign(s0) + sign(s1)) * rustMin(rustMin(math.Abs(s0), math.Abs(s1)), 0.5*math.Abs(p))
	if math.IsNaN(t) {
		return 0
	}
	return t
}

// slope2 is the tangent at an end point, from the segment and the
// neighbor's tangent.
func slope2(p0, p1 scene.Point, t float64) float64 {
	h := p1.X - p0.X
	if h != 0 {
		return (float64(3*(p1.Y-p0.Y))/h - t) / 2
	}
	return t
}

// subCubic is the cubic p between t0 and t1 (de Casteljau, twice).
func subCubic(p [4]scene.Point, t0, t1 float64) [4]scene.Point {
	lerp := func(a, b scene.Point, t float64) scene.Point {
		return scene.Point{X: a.X + float64((b.X-a.X)*t), Y: a.Y + float64((b.Y-a.Y)*t)}
	}
	// The part before t1: its points are the left edges of the triangle.
	a, b, c := lerp(p[0], p[1], t1), lerp(p[1], p[2], t1), lerp(p[2], p[3], t1)
	d, e := lerp(a, b, t1), lerp(b, c, t1)
	left := [4]scene.Point{p[0], a, d, lerp(d, e, t1)}
	if t0 <= 0 {
		return left
	}
	// Of that, the part after t0, rescaled to it.
	t := t0 / t1
	a, b, c = lerp(left[0], left[1], t), lerp(left[1], left[2], t), lerp(left[2], left[3], t)
	d, e = lerp(a, b, t), lerp(b, c, t)
	return [4]scene.Point{lerp(d, e, t), e, c, left[3]}
}

// ClipX is the part of an x-monotone path (a move, then lines and cubics
// whose controls sit at thirds of their x span, as Monotone makes) between
// x0 and x1. Along such a cubic x moves linearly with t, so a cut at an x is
// a split at a known t: the curve keeps its exact shape. With continuePath,
// the path starts with a line instead of a move; the input's first point may
// be either.
func ClipX(ops []scene.PathOp, x0, x1 float64, continuePath bool) []scene.PathOp {
	var out []scene.PathOp
	if len(ops) == 0 || (ops[0].Verb != scene.MoveTo && ops[0].Verb != scene.LineTo) {
		return out
	}
	at := scene.Point{X: ops[0].X, Y: ops[0].Y}
	for _, op := range ops[1:] {
		end := scene.Point{X: op.X, Y: op.Y}
		var seg [4]scene.Point
		switch op.Verb {
		case scene.MoveTo:
			at = end
			continue
		case scene.LineTo:
			// A line as a cubic with its controls at thirds, so x moves
			// linearly with t along it too.
			third := func(k float64) scene.Point {
				return scene.Point{X: at.X + float64((end.X-at.X)*k)/3, Y: at.Y + float64((end.Y-at.Y)*k)/3}
			}
			seg = [4]scene.Point{at, third(1), third(2), end}
		case scene.CubicTo:
			seg = [4]scene.Point{at, {X: op.X1, Y: op.Y1}, {X: op.X2, Y: op.Y2}, end}
		default:
			continue
		}
		xa, xb := seg[0].X, seg[3].X
		ta, tb := 1.0, 0.0
		if xb == xa {
			if xa >= x0 && xa <= x1 {
				ta, tb = 0, 1
			}
		} else {
			u, v := (x0-xa)/(xb-xa), (x1-xa)/(xb-xa)
			ta, tb = math.Max(math.Min(u, v), 0), math.Min(math.Max(u, v), 1)
		}
		at = end
		if ta >= tb {
			continue
		}
		part := seg
		if ta != 0 || tb != 1 {
			part = subCubic(seg, ta, tb)
		}
		if len(out) == 0 {
			if continuePath {
				out = append(out, scene.Line(part[0].X, part[0].Y))
			} else {
				out = append(out, scene.Move(part[0].X, part[0].Y))
			}
		}
		if op.Verb == scene.LineTo {
			out = append(out, scene.Line(part[3].X, part[3].Y))
		} else {
			out = append(out, scene.Cubic(part[1].X, part[1].Y, part[2].X, part[2].Y, part[3].X, part[3].Y))
		}
	}
	return out
}

// Monotone is the path from a move to the first point through the rest.
// With continuePath, the first point is a line instead of a move, so a curve
// can extend an outline already in progress.
func Monotone(points []scene.Point, continuePath bool) []scene.PathOp {
	ops := make([]scene.PathOp, 0, len(points)+1)
	if len(points) == 0 {
		return ops
	}
	p0 := points[0]
	if continuePath {
		ops = append(ops, scene.Line(p0.X, p0.Y))
	} else {
		ops = append(ops, scene.Move(p0.X, p0.Y))
	}
	n := len(points)
	if n == 2 {
		ops = append(ops, scene.Line(points[1].X, points[1].Y))
	}
	if n <= 2 {
		return ops
	}
	t := make([]float64, n)
	for i := 1; i < n-1; i++ {
		t[i] = slope3(points[i-1], points[i], points[i+1])
	}
	t[0] = slope2(points[0], points[1], t[1])
	t[n-1] = slope2(points[n-2], points[n-1], t[n-2])
	for i := 0; i < n-1; i++ {
		a, b := points[i], points[i+1]
		dx := (b.X - a.X) / 3
		ops = append(ops, scene.Cubic(a.X+dx, a.Y+float64(dx*t[i]), b.X-dx, b.Y-float64(dx*t[i+1]), b.X, b.Y))
	}
	return ops
}
