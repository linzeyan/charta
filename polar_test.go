package charta

import (
	"math"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// hitPaths are the ops of the interactive path marks (slices, wedges, bars),
// in paint order.
func hitPaths(s *scene.Scene) [][]scene.PathOp {
	var out [][]scene.PathOp
	for _, m := range s.Marks {
		if p, ok := m.Shape.(scene.PathShape); ok && m.Hit != nil {
			out = append(out, p.Ops)
		}
	}
	return out
}

func TestArcsStayOnTheCircle(t *testing.T) {
	// A cubic per quarter turn deviates from the true circle by under 0.03%
	// of the radius; anything worse shows as lumpy pies.
	for _, p := range scene.Flatten(circle(0, 0, 100))[0] {
		if r := math.Hypot(p.X, p.Y); math.Abs(r-100) >= 0.03 {
			t.Fatalf("point at radius %v", r)
		}
	}
}

func TestArcsRunEitherWayAQuarterTurnAtATime(t *testing.T) {
	// Backwards from six to three o'clock: one cubic ending at three.
	ops := arc(nil, 0, 0, 10, math.Pi, math.Pi/2)
	if len(ops) != 1 || !near(ops[0].X, 10) || math.Abs(ops[0].Y) > 1e-9 {
		t.Fatalf("%+v", ops)
	}
	// Just over a half turn needs three pieces, each ending on the circle.
	ops = arc(nil, 0, 0, 10, 0, math.Pi+0.1)
	if len(ops) != 3 {
		t.Fatalf("%d pieces", len(ops))
	}
	for _, o := range ops {
		if !near(math.Hypot(o.X, o.Y), 10) {
			t.Fatalf("%+v off the circle", o)
		}
	}
}

func TestASectorCoversItsWedgeAndNothingElse(t *testing.T) {
	// Quarter ring from three to six o'clock, radii 50..100.
	s := scene.Scene{Width: 1, Height: 1, Marks: []scene.Mark{{
		Shape: scene.PathShape{Ops: sector(0, 0, 50, 100, math.Pi/2, math.Pi)},
		Fill:  base.Series(0),
		Hit:   &scene.Hit{Key: "0", Label: "wedge"},
	}}}
	inside := func(x, y float64) bool { _, ok := s.HitTest(x, y, nil); return ok }
	if !inside(53, 53) {
		t.Fatal("middle of the wedge")
	}
	for _, p := range [][2]float64{{20, 20}, {-53, 53}, {53, -53}} {
		if inside(p[0], p[1]) {
			t.Fatalf("%v: the hole, the next quarter and the quarter before stay out", p)
		}
	}
}

func TestLabelsFaceAwayFromTheCenter(t *testing.T) {
	for _, c := range []struct {
		a      float64
		anchor scene.Anchor
		base   scene.Baseline
	}{
		{math.Pi / 2, scene.Start, scene.Center},
		{0, scene.Middle, scene.Bottom},
		{math.Pi, scene.Middle, scene.Top},
		{1.5 * math.Pi, scene.End, scene.Center},
		// Past the snap threshold on both axes: a corner anchor.
		{math.Pi / 6, scene.Start, scene.Bottom},
		{7 * math.Pi / 6, scene.End, scene.Top},
		// Within it: still centered on the axis it is near.
		{0.2, scene.Middle, scene.Bottom},
	} {
		if _, _, a, b := around(0, 0, 10, c.a); a != c.anchor || b != c.base {
			t.Errorf("angle %v: %v %v, want %v %v", c.a, a, b, c.anchor, c.base)
		}
	}
}

func TestTheRadialScaleTopsOutAtARoundNumberAndSkipsZero(t *testing.T) {
	// The outer ring is a round value at or past the data, so the largest
	// value never pokes out; zero is the center and gets no ring.
	top, ticks := radialTicks(87)
	if top != 100 || len(ticks) == 0 || ticks[0] <= 0 || ticks[len(ticks)-1] != 100 {
		t.Fatalf("%v %v", top, ticks)
	}
	// All-zero data still gets a scale to divide by.
	if top, _ := radialTicks(0); top != 1 {
		t.Fatalf("top %v", top)
	}
}

func TestTickLabelsSitOnASurfaceBacking(t *testing.T) {
	m := base.InterMetrics{}
	w := m.Width("40", tickSize, base.Regular)
	for _, c := range []struct {
		anchor scene.Anchor
		left   float64
	}{{scene.Start, 100}, {scene.Middle, 100 - w/2}} {
		var marks []scene.Mark
		tickLabel(&marks, m, 100, 50, c.anchor, 40)
		if len(marks) != 2 {
			t.Fatalf("%d marks", len(marks))
		}
		// The backing paints first, under the text, and wraps it.
		r := rectOf(marks[0])
		tx := marks[1].Shape.(scene.TextShape)
		if marks[0].Fill != base.Surface || !near(r.X, c.left-polarHalo) || !near(r.W, w+2*polarHalo) ||
			!near(r.Y+r.H/2, 50) || r.H <= tickSize {
			t.Fatalf("%v backing %+v", c.anchor, r)
		}
		if tx.Text != "40" || tx.Anchor != c.anchor || tx.Base != scene.Center {
			t.Fatalf("%+v", tx)
		}
	}
}
