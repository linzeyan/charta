package charta

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/linzeyan/charta/scene"
)

// area is a flattened path's polygon area (shoelace), which for a slice is
// its share of the disc.
func area(ops []scene.PathOp) float64 {
	sum := 0.0
	for _, p := range scene.Flatten(ops) {
		s := 0.0
		for k := range p {
			a, b := p[k], p[(k+1)%len(p)]
			s += a.X*b.Y - b.X*a.Y
		}
		sum += s / 2
	}
	return math.Abs(sum)
}

func TestSliceAreasAreProportionalToValues(t *testing.T) {
	// The one thing a pie must get right: area (and angle) is share.
	p := hitPaths(build(t, PieChart{Data: []Bar{{"a", 1}, {"b", 2}, {"c", 5}}}))
	a := []float64{area(p[0]), area(p[1]), area(p[2])}
	if math.Abs(a[1]/a[0]-2) >= 0.01 || math.Abs(a[2]/a[0]-5) >= 0.01 {
		t.Fatalf("%v", a)
	}
}

func TestTheFirstSliceStartsAtTwelveOClockGoingClockwise(t *testing.T) {
	ops := hitPaths(buildAt(t, PieChart{Data: []Bar{{"a", 1}, {"b", 3}}}, 400, 400))[0]
	if ops[0].Verb != scene.MoveTo || ops[1].Verb != scene.CubicTo {
		t.Fatalf("%+v", ops)
	}
	const cx = 200.0
	if !near(ops[0].X, cx) || ops[0].Y >= 200 {
		t.Fatalf("starts at %v, %v, not straight up", ops[0].X, ops[0].Y)
	}
	if ops[1].X <= cx {
		t.Fatal("runs to the left first")
	}
}

func TestLabelsOnOneSideNeverOverlap(t *testing.T) {
	// Many thin slices all crowd the right side's top.
	s := build(t, PieChart{Data: []Bar{{"a", 1}, {"b", 1}, {"c", 1}, {"d", 1}, {"e", 1}, {"f", 30}}})
	type label struct {
		right bool
		y     float64
	}
	var ys []label
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && strings.HasSuffix(tx.Text, "%") {
			ys = append(ys, label{tx.Anchor == scene.Start, tx.Y})
		}
	}
	if len(ys) != 6 {
		t.Fatalf("%d labels", len(ys))
	}
	sort.Slice(ys, func(a, b int) bool {
		if ys[a].right != ys[b].right {
			return !ys[a].right
		}
		return ys[a].y < ys[b].y
	})
	for k := 1; k < len(ys); k++ {
		if ys[k].right == ys[k-1].right && ys[k].y-ys[k-1].y < pieRow-1e-9 {
			t.Fatalf("%v", ys)
		}
	}
}

func TestLeaderLinesRunFromTheRimToTheirLabel(t *testing.T) {
	// A slice pointing left gets an end-anchored label on the left, joined
	// by a leader from just past the rim out to the label's edge. Sides go by
	// where the slice points horizontally: "a" points down and right, "b" up
	// and left.
	s := build(t, PieChart{Data: []Bar{{"a", 3}, {"b", 1}}})
	var leaders []scene.PolylineShape
	var labels []scene.TextShape
	for _, m := range s.Marks {
		switch sh := m.Shape.(type) {
		case scene.PolylineShape:
			leaders = append(leaders, sh)
		case scene.TextShape:
			if strings.HasSuffix(sh.Text, "%") {
				labels = append(labels, sh)
			}
		}
	}
	if len(leaders) != 2 || len(labels) != 2 {
		t.Fatalf("%d leaders, %d labels", len(leaders), len(labels))
	}
	for k, l := range labels {
		end := leaders[k].Points[2]
		right := l.Text == "a 75%"
		dir := 1.0
		if !right {
			dir = -1
		}
		if right != (l.Anchor == scene.Start) || !near(end.Y, l.Y) || !near((l.X-end.X)*dir, gap/2) {
			t.Fatalf("label %+v, leader %+v", l, leaders[k])
		}
		if right != (end.X > 320) {
			t.Fatalf("%q on the wrong side", l.Text)
		}
	}
}

func TestSlicesTileTheDiscInDataOrder(t *testing.T) {
	// Each slice starts where the one before it ends, and the last closes
	// the circle back at twelve o'clock.
	const cx, cy = 200.0, 200.0
	p := hitPaths(buildAt(t, PieChart{Data: []Bar{{"a", 1}, {"b", 2}, {"c", 5}}}, 400, 400))
	outerEnd := func(ops []scene.PathOp) scene.PathOp {
		var end scene.PathOp
		for _, o := range ops {
			if o.Verb == scene.LineTo {
				break
			}
			end = o
		}
		return end
	}
	for k := 1; k < len(p); k++ {
		if e := outerEnd(p[k-1]); math.Hypot(e.X-p[k][0].X, e.Y-p[k][0].Y) > 1e-9 {
			t.Fatalf("slice %d starts at %+v, the one before ends at %+v", k, p[k][0], e)
		}
	}
	if e := outerEnd(p[2]); !near(e.X, cx) || e.Y >= cy {
		t.Fatalf("last slice ends at %+v", e)
	}
}

func TestCrowdedLabelsPushDownFromWhereTheirSlicesPoint(t *testing.T) {
	// Thin slices at the top all point near twelve; with room below, each
	// label sits at or below its leader's elbow instead of climbing above
	// the topmost slice.
	s := build(t, PieChart{Data: []Bar{{"a", 1}, {"b", 1}, {"c", 1}, {"d", 1}, {"e", 1}, {"f", 60}}})
	var elbows []scene.Point
	var ys []float64
	for _, m := range s.Marks {
		switch sh := m.Shape.(type) {
		case scene.PolylineShape:
			elbows = append(elbows, sh.Points[1])
		case scene.TextShape:
			if strings.HasSuffix(sh.Text, "%") {
				ys = append(ys, sh.Y)
			}
		}
	}
	if len(ys) != 6 {
		t.Fatalf("%d labels", len(ys))
	}
	for k := range ys {
		if ys[k] < elbows[k].Y-1e-9 {
			t.Fatalf("label %d at %v climbs above its elbow at %v", k, ys[k], elbows[k].Y)
		}
	}
}

func TestADonutHasAHoleAndShowsTheTotal(t *testing.T) {
	s := build(t, DonutChart{Data: []Bar{{"a", 40}, {"b", 60}}})
	if _, ok := s.HitTest(320, (pad+400-pad)/2, nil); ok {
		t.Fatal("the center is empty")
	}
	found := false
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Text == "100" {
			found = true
		}
	}
	if !found {
		t.Fatal("no total")
	}
}

func TestTooltipsGiveTheValueAndItsShare(t *testing.T) {
	s := build(t, PieChart{Data: []Bar{{"a", 1}, {"b", 2}}})
	var slices []*scene.Hit
	for _, m := range s.Marks {
		if _, ok := m.Shape.(scene.PathShape); ok && m.Hit != nil {
			slices = append(slices, m.Hit)
		}
	}
	if h := slices[1]; h.Value != "2" || h.Rows[0].Value != "66.7%" {
		t.Fatalf("%+v", h)
	}
}

func TestNegativeDuplicateAndAllZeroSlicesAreRejected(t *testing.T) {
	rejects(t, PieChart{Data: []Bar{{"a", -1}}})
	rejects(t, PieChart{Data: []Bar{{"a", 1}, {"a", 2}}})
	rejects(t, PieChart{Data: []Bar{{"a", 0}}})
	nine := make([]Bar, 9)
	for i := range nine {
		nine[i] = Bar{fmt.Sprint(i), 1}
	}
	rejects(t, PieChart{Data: nine})
	rejects(t, DonutChart{Data: []Bar{{"a", math.NaN()}}})
}

func TestSliversReadAsUnderOnePercent(t *testing.T) {
	for share, want := range map[float64]string{0.004: "<1%", 0.425: "43%", 0: "0%"} {
		if got := percent(share); got != want {
			t.Errorf("percent(%v) = %q, want %q", share, got, want)
		}
	}
}
