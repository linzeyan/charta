package charta

import (
	"math"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// swept is the angle a bar's outer edge sweeps, from its start (straight up)
// to the end point of its last outer cubic.
func swept(ops []scene.PathOp, cx, cy float64) float64 {
	var end scene.PathOp
	for _, o := range ops {
		if o.Verb == scene.LineTo {
			break
		}
		if o.Verb == scene.CubicTo {
			end = o
		}
	}
	a := math.Atan2(end.X-cx, cy-end.Y)
	if a < 0 {
		a += tau
	}
	return a
}

func TestRadialBarAngleIsProportionalToValue(t *testing.T) {
	p := hitPaths(buildAt(t, RadialBarChart{Data: []Bar{{"a", 20}, {"b", 40}}}, 400, 400))
	const cy = (pad + 400 - pad) / 2
	a, b := swept(p[0], 200, cy), swept(p[1], 200, cy)
	if math.Abs(b/a-2) >= 1e-9 {
		t.Fatalf("%v %v", a, b)
	}
	// The scale tops out at a round number, never past three quarters.
	if b > radialSweep+1e-9 {
		t.Fatalf("%v sweeps past three quarters", b)
	}
}

func TestTheFirstRadialBarIsTheOutermostRing(t *testing.T) {
	p := hitPaths(buildAt(t, RadialBarChart{Data: []Bar{{"outer", 1}, {"inner", 1}}}, 400, 400))
	const cy = (pad + 400 - pad) / 2
	if !(cy-p[0][0].Y > cy-p[1][0].Y) {
		t.Fatalf("%v %v", p[0][0], p[1][0])
	}
}

func TestRadialBarsRunOnFullLengthTracksAndAreLabelledInTheFreeQuarter(t *testing.T) {
	s := buildAt(t, RadialBarChart{Data: []Bar{{"a", 50}, {"b", 0}}}, 400, 400)
	const cy = (pad + 400 - pad) / 2
	var tracks [][]scene.PathOp
	for _, m := range s.Marks {
		if p, ok := m.Shape.(scene.PathShape); ok && m.Hit == nil {
			tracks = append(tracks, p.Ops)
			if m.Fill != base.Wash(0) {
				t.Fatalf("track fill %v", m.Fill)
			}
		}
	}
	// Every bar has a track, a zero bar included, and it sweeps the full
	// three quarters.
	if len(tracks) != 2 || len(hitPaths(s)) != 1 {
		t.Fatalf("%d tracks, %d bars", len(tracks), len(hitPaths(s)))
	}
	if a := swept(tracks[0], 200, cy); math.Abs(a-radialSweep) > 1e-9 {
		t.Fatalf("track sweeps %v", a)
	}
	// Each label ends just left of twelve o'clock, level with its ring's
	// middle, inside the empty quarter.
	bar := hitPaths(s)[0]
	outer := cy - bar[0].Y
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Text == "a" {
			if tx.Anchor != scene.End || tx.X >= 200 || !(cy-tx.Y < outer) || cy-tx.Y < outer-barMax {
				t.Fatalf("%+v, ring outer radius %v", tx, outer)
			}
			return
		}
	}
	t.Fatal("no bar label")
}

func TestRadialBarsFillMostOfTheirRingUpToTheBarCap(t *testing.T) {
	const cy = (pad + 400 - pad) / 2
	for _, n := range []int{2, 8} {
		data := make([]Bar, n)
		for i := range data {
			data[i] = Bar{string(rune('a' + i)), 1}
		}
		p := hitPaths(buildAt(t, RadialBarChart{Data: data}, 400, 400))
		radii := func(ops []scene.PathOp) (float64, float64) {
			for _, o := range ops {
				if o.Verb == scene.LineTo {
					return math.Hypot(o.X-200, o.Y-cy), cy - ops[0].Y
				}
			}
			return 0, 0
		}
		i0, o0 := radii(p[0])
		_, o1 := radii(p[1])
		ring := o0 - o1
		if want := math.Min(ring*ringFill, barMax); !near(o0-i0, want) {
			t.Fatalf("%d bars: thickness %v, want %v (ring %v)", n, o0-i0, want, ring)
		}
	}
}

func TestRadialColumnLengthFromTheInnerCircleIsProportionalToValue(t *testing.T) {
	p := hitPaths(buildAt(t, RadialColumnChart{Data: []Bar{{"a", 10}, {"b", 20}, {"c", 0}, {"d", 5}}}, 400, 400))
	if len(p) != 3 {
		t.Fatalf("%d columns: a zero column draws nothing", len(p))
	}
	const cy = (pad + 400 - pad) / 2
	dist := func(x, y float64) float64 { return math.Hypot(x-200, y-cy) }
	// Outer radius from the Move; inner radius from the Line onto the inner
	// arc.
	radii := func(ops []scene.PathOp) (float64, float64) {
		for _, o := range ops {
			if o.Verb == scene.LineTo {
				return dist(o.X, o.Y), dist(ops[0].X, ops[0].Y)
			}
		}
		t.Fatalf("%+v", ops)
		return 0, 0
	}
	i0, o0 := radii(p[0])
	i1, o1 := radii(p[1])
	if math.Abs(i0-i1) >= 1e-9 {
		t.Fatal("one shared inner circle")
	}
	if math.Abs((o1-i1)/(o0-i0)-2) >= 1e-9 {
		t.Fatalf("%v %v %v %v", i0, o0, i1, o1)
	}
}

func TestTheSlotAtTwelveStaysEmptyForTheScale(t *testing.T) {
	// Three columns share four slots; the first starts past an eighth of a
	// turn and the last ends before seven eighths, leaving twelve o'clock to
	// the centered ring labels.
	s := buildAt(t, RadialColumnChart{Data: []Bar{{"a", 1}, {"b", 1}, {"c", 1}}}, 400, 400)
	const cy = (pad + 400 - pad) / 2
	p := hitPaths(s)
	first := math.Atan2(p[0][0].X-200, cy-p[0][0].Y)
	if first <= tau/8 || first >= tau/4 {
		t.Fatalf("first column starts at %v", first)
	}
	// Columns fill 80% of their slot, centered in it.
	end := swept(p[0], 200, cy)
	if math.Abs((end-first)-0.8*tau/4) > 1e-9 || math.Abs((first+end)/2-tau/4) > 1e-9 {
		t.Fatalf("column spans %v..%v", first, end)
	}
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Text == "1" {
			if tx.X != 200 || tx.Anchor != scene.Middle {
				t.Fatalf("%+v", tx)
			}
			return
		}
	}
	t.Fatal("no ring label")
}

func TestRadialNegativeValuesAreRejected(t *testing.T) {
	rejects(t, RadialBarChart{Data: []Bar{{"a", -1}}})
	rejects(t, RadialColumnChart{Data: []Bar{{"a", -1}}})
	rejects(t, RadialColumnChart{Data: []Bar{{"a", math.Inf(1)}}})
}
