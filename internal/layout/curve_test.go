package layout

import (
	"math"
	"slices"
	"testing"

	"github.com/linzeyan/charta/scene"
)

func TestTheCurvePassesThroughEveryPoint(t *testing.T) {
	// A smoothed line that misses its data points misreports the data.
	p := pts(0, 10, 10, 30, 20, 20, 35, 25)
	ops := Monotone(p, false)
	if ops[0] != scene.Move(0, 10) {
		t.Fatalf("starts with %+v", ops[0])
	}
	var ends []scene.Point
	for _, o := range ops[1:] {
		if o.Verb != scene.CubicTo {
			t.Fatalf("expected cubics, got %+v", o)
		}
		ends = append(ends, scene.Point{X: o.X, Y: o.Y})
	}
	if !slices.Equal(ends, p[1:]) {
		t.Fatalf("segments end at %v, want %v", ends, p[1:])
	}
}

func TestItNeverOvershootsBetweenPoints(t *testing.T) {
	// A step: a naive spline would dip below 0 and rise above 10.
	p := pts(0, 0, 10, 0, 20, 10, 30, 10)
	for _, p := range scene.Flatten(Monotone(p, false))[0] {
		if p.Y < -1e-9 || p.Y > 10+1e-9 {
			t.Fatalf("overshoot to %v", p.Y)
		}
	}
	// Tangents follow the data's direction: a rise between two flats has a
	// positive slope in the middle, flat at the turning points.
	if s := slope3(p[1], p[2], p[3]); s != 0 {
		t.Errorf("tangent at the top of the step %v, want 0", s)
	}
	if s := slope3(scene.Point{X: 0, Y: 0}, scene.Point{X: 10, Y: 5}, scene.Point{X: 20, Y: 10}); s != 0.5 {
		t.Errorf("tangent on a straight rise %v, want 0.5", s)
	}
}

func TestTheCurveIsSmoothThroughEveryPoint(t *testing.T) {
	// No kink at a data point: the control points either side of it lie on
	// one line through it, so the curve's direction is continuous. Rising
	// data, so the tangents are not all zero (a zigzag's are).
	p := pts(0, 0, 10, 10, 20, 30, 30, 35)
	ops := Monotone(p, false)
	for i := 1; i < len(p)-1; i++ {
		in, out := ops[i], ops[i+1]
		slopeIn := (p[i].Y - in.Y2) / (p[i].X - in.X2)
		slopeOut := (out.Y1 - p[i].Y) / (out.X1 - p[i].X)
		if math.Abs(slopeIn-slopeOut) > 1e-9 {
			t.Errorf("kink at point %d: %v in, %v out", i, slopeIn, slopeOut)
		}
	}
}

func TestTwoPointsAreAStraightLine(t *testing.T) {
	ops := Monotone(pts(0, 0, 5, 5), true)
	if !slices.Equal(ops, []scene.PathOp{scene.Line(0, 0), scene.Line(5, 5)}) {
		t.Fatalf("%+v", ops)
	}
}

func TestZeroWidthSegmentsFollowRustsMin(t *testing.T) {
	// One zero-width segment gives an infinite slope that the min clamps
	// away: the tangent stays finite.
	s := slope3(scene.Point{X: 0, Y: 0}, scene.Point{X: 0, Y: 5}, scene.Point{X: 10, Y: 10})
	if s != 1 {
		t.Fatalf("tangent beside a vertical segment %v, want 1", s)
	}
	// Two make p = inf·0 = NaN. Rust's f64::min ignores the NaN and keeps
	// the infinite slope; math.Min would return NaN and the tangent would
	// collapse to 0, moving the control points away from the Rust port's.
	if s := slope3(scene.Point{X: 0, Y: 0}, scene.Point{X: 0, Y: 5}, scene.Point{X: 0, Y: 10}); !math.IsInf(s, 1) {
		t.Fatalf("tangent on a vertical run %v, want +Inf", s)
	}
}

// pts pairs up coordinates: pts(x0, y0, x1, y1, …).
func pts(xy ...float64) []scene.Point {
	out := make([]scene.Point, len(xy)/2)
	for i := range out {
		out[i] = scene.Point{X: xy[2*i], Y: xy[2*i+1]}
	}
	return out
}

func TestClippingCutsAtTheEdgesAndKeepsTheShape(t *testing.T) {
	p := pts(0, 10, 10, 30, 20, 20, 35, 25)
	ops := Monotone(p, false)
	cut := scene.Flatten(ClipX(ops, 4, 27, false))[0]
	if first, last := cut[0], cut[len(cut)-1]; math.Abs(first.X-4) > 1e-9 || math.Abs(last.X-27) > 1e-9 {
		t.Fatalf("cut from %v to %v", first, last)
	}
	// Every kept point lies on the uncut curve: the cut pieces are the same
	// curve, not a new one through the edge points.
	whole := scene.Flatten(ops)[0]
	for _, q := range cut {
		for k := 1; k < len(whole); k++ {
			a, b := whole[k-1], whole[k]
			if a.X <= q.X && q.X <= b.X {
				if y := a.Y + (b.Y-a.Y)*(q.X-a.X)/(b.X-a.X); math.Abs(q.Y-y) > 0.05 {
					t.Fatalf("%v is off the curve (%v)", q, y)
				}
				break
			}
		}
	}
	// Inside the edges, nothing is cut; outside, nothing is left.
	if !slices.Equal(ClipX(ops, -5, 40, false), ops) || len(ClipX(ops, 50, 60, false)) != 0 {
		t.Fatal("clipping changed an uncut path or kept an outside one")
	}
	// A straight piece is cut where it crosses the edge.
	line := ClipX([]scene.PathOp{scene.Move(0, 0), scene.Line(10, 20)}, 2.5, 5, false)
	if !slices.Equal(line, []scene.PathOp{scene.Move(2.5, 5), scene.Line(5, 10)}) {
		t.Fatalf("%v", line)
	}
	// Backward (a band's lower edge) works the same way.
	back := slices.Clone(p)
	slices.Reverse(back)
	if c := ClipX(Monotone(back, true), 4, 27, true); c[0].Verb != scene.LineTo || math.Abs(c[0].X-27) > 1e-9 {
		t.Fatalf("%v", c[0])
	}
}
