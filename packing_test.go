package charta

import (
	"math"
	"slices"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func circleOf(m scene.Mark) scene.CircleShape { return m.Shape.(scene.CircleShape) }

func TestLeavesSitInsideTheirGroupAndTheChart(t *testing.T) {
	s := build(t, CirclePackingChart{Root: nbranch("r",
		nbranch("A", nleaf("a1", 3), nleaf("a2", 2), nleaf("a3", 1)),
		nbranch("B", nleaf("b1", 4), nleaf("b2", 1)))})
	for _, p := range [][2]string{{"A / a1", "A"}, {"A / a3", "A"}, {"B / b2", "B"}} {
		l, g := circleOf(markNamed(t, s, p[0])), circleOf(markNamed(t, s, p[1]))
		if math.Hypot(l.CX-g.CX, l.CY-g.CY)+l.R > g.R+1e-6 {
			t.Errorf("%s leaves %s", p[0], p[1])
		}
	}
	for _, m := range s.Marks {
		if c, ok := m.Shape.(scene.CircleShape); ok {
			if c.CX-c.R < pad-1e-6 || c.CX+c.R > 640-pad+1e-6 || c.CY+c.R > 400-pad+1e-6 {
				t.Errorf("%+v leaves the chart", c)
			}
		}
	}
	// Groups are outlines; the leaves inside carry the branch hue.
	if g := markNamed(t, s, "B"); g.Fill != base.NoColor || g.Stroke.Color != base.Border {
		t.Errorf("group B: %+v", g)
	}
	if l := markNamed(t, s, "B / b1"); l.Fill != base.Wash(1) || l.Stroke.Color != base.Series(1) || l.Series != "B" {
		t.Errorf("leaf b1: %+v", l)
	}
	if h := markNamed(t, s, "A").Hit; h.Value != "6" || len(h.Rows) != 2 || h.Rows[1] != (scene.TipRow{Name: "items", Value: "3"}) {
		t.Errorf("group tooltip %+v", h)
	}
}

func TestLeafAreaIsProportionalToValue(t *testing.T) {
	s := build(t, CirclePackingChart{Root: nbranch("r", nleaf("a", 1), nleaf("b", 4))})
	if r := circleOf(markNamed(t, s, "b")).R / circleOf(markNamed(t, s, "a")).R; math.Abs(r-2) >= 1e-9 {
		t.Fatalf("radius ratio %v", r)
	}
}

func TestHoveringALeafBeatsItsGroup(t *testing.T) {
	s := build(t, CirclePackingChart{Root: nbranch("r",
		nbranch("A", nleaf("a1", 1), nleaf("a2", 1)), nleaf("B", 1))})
	leaf := circleOf(markNamed(t, s, "A / a1"))
	i, ok := s.HitTest(leaf.CX, leaf.CY, nil)
	if !ok || s.Marks[i].Hit.Label != "A / a1" {
		t.Fatalf("hit %v %v", i, ok)
	}
}

func TestPackedLeavesAreNamedOnlyWhenTheNameFits(t *testing.T) {
	s := build(t, CirclePackingChart{Root: nbranch("r", nbranch("g", nleaf("Big", 100), nleaf("A rather long name", 1)))})
	got := texts(s)
	if !slices.Contains(got, "Big") || slices.Contains(got, "A rather long name") {
		t.Fatalf("labels %v", got)
	}
	big := circleOf(markNamed(t, s, "g / Big"))
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Text == "Big" && (tx.X != big.CX || tx.Y != big.CY) {
			t.Errorf("label at %v, %v, not centered in %+v", tx.X, tx.Y, big)
		}
	}
}

func TestZeroLeavesGetNoCircle(t *testing.T) {
	s := build(t, CirclePackingChart{Root: nbranch("r", nleaf("a", 1), nleaf("z", 0), nleaf("b", 2))})
	n := 0
	for _, m := range s.Marks {
		if _, ok := m.Shape.(scene.CircleShape); ok {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%d circles", n)
	}
}
