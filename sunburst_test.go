package charta

import (
	"math"
	"slices"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func sunburstSample() SunburstChart {
	return SunburstChart{Root: nbranch("All", nbranch("A", nleaf("a1", 1), nleaf("a2", 3)), nleaf("B", 4))}
}

func TestChildrenShareTheirParentsAngleInProportionAndRingOutward(t *testing.T) {
	s := buildAt(t, sunburstSample(), 400, 400)
	ops := func(l string) []scene.PathOp { return markNamed(t, s, l).Shape.(scene.PathShape).Ops }
	a, a1, a2, b := ops("A"), ops("A / a1"), ops("A / a2"), ops("B")
	// A spans half the circle (4 of 8), from twelve o'clock; a1 takes the
	// first quarter of that, a2 the rest, one ring further out.
	aEnd, bStart := a[2], b[0]
	if !near(aEnd.X, bStart.X) || !near(aEnd.X, 200) {
		t.Fatalf("B starts at %v, A ends at %v: not straight down", bStart, aEnd)
	}
	center := scene.Point{X: 200, Y: (aEnd.Y + a[0].Y) / 2}
	// a2 starts a quarter of A's half-turn in: at 45°.
	if math.Abs((a2[0].X-center.X)-(center.Y-a2[0].Y)) >= 1e-6 {
		t.Errorf("a2 starts at %v, not 45° from %v", a2[0], center)
	}
	r := func(p scene.PathOp) float64 { return math.Hypot(p.X-center.X, p.Y-center.Y) }
	if r(a1[0]) <= r(a[0]) {
		t.Error("a1 is not one ring out")
	}
	// The hole is one ring wide: the first level spans the second ring.
	if outer := r(a[0]); math.Abs(r(a1[0])-outer*1.5) >= 1e-6 {
		t.Errorf("rings at %v and %v are not equal widths around a one-ring hole", outer, r(a1[0]))
	}
}

func TestTheCenterShowsTheTotal(t *testing.T) {
	got := texts(buildAt(t, sunburstSample(), 400, 400))
	if !slices.Contains(got, "8") || !slices.Contains(got, "All") {
		t.Fatalf("center labels %v", got)
	}
	// An unnamed root reads as the total.
	c := sunburstSample()
	c.Root.Name = ""
	if got := texts(buildAt(t, c, 400, 400)); !slices.Contains(got, "Total") {
		t.Fatalf("center labels %v", got)
	}
}

func TestSunburstKeepsDataOrderAndBranchHues(t *testing.T) {
	// Sorting by size would put big first; it must not: the order around
	// the circle often means something.
	s := buildAt(t, SunburstChart{Root: nbranch("r", nleaf("small", 1), nleaf("big", 9))}, 400, 400)
	small := markNamed(t, s, "small")
	if p := small.Shape.(scene.PathShape).Ops[0]; !near(p.X, 200) {
		t.Errorf("the first datum starts at %v, not twelve o'clock", p)
	}
	if small.Fill != base.Series(0) || markNamed(t, s, "big").Fill != base.Series(1) {
		t.Errorf("hues follow data order")
	}
	if h := small.Hit; h.Value != "1" || h.Rows[0].Value != "10%" {
		t.Errorf("tooltip %+v", h)
	}
}

func TestSunburstDescribesItsItemsAndLevels(t *testing.T) {
	if l := buildAt(t, sunburstSample(), 400, 400).Label; l != "Sunburst of 3 items in 2 levels." {
		t.Fatalf("%q", l)
	}
}
