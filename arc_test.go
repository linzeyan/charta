package charta

import (
	"fmt"
	"math"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// apex is the y where a link's arc peaks: the end of its first cubic.
func apex(t *testing.T, s *scene.Scene, key string) float64 {
	t.Helper()
	return markKeyed(t, s, key).Shape.(scene.PathShape).Ops[1].Y
}

func TestLongerLinksArchHigherAndAllStayInThePlot(t *testing.T) {
	// Nodes line up in first-mention order a, b, c, d: a–d spans three
	// steps, too wide for a semicircle in 300px, so every arc is squashed.
	s := buildAt(t, ArcChart{Links: []Link{{"a", "b", 1}, {"c", "d", 1}, {"a", "d", 1}}}, 640, 300)
	if apex(t, s, "link-2") >= apex(t, s, "link-0") {
		t.Fatal("the long link must rise higher")
	}
	// The widest arc is squashed to touch the top exactly, not past it and
	// not left short.
	top := pad + 16 + gap
	if !near(apex(t, s, "link-2")-arcMaxStroke/2, top) {
		t.Fatalf("the tallest arc stops at %v, want %v", apex(t, s, "link-2")-arcMaxStroke/2, top)
	}
	// Squashing is uniform: a–b and c–d (both one step) peak alike, and
	// lower than a semicircle would.
	ops := markKeyed(t, s, "link-0").Shape.(scene.PathShape).Ops
	if !near(apex(t, s, "link-0"), apex(t, s, "link-1")) || ops[0].Y-ops[1].Y >= (ops[2].X-ops[0].X)/2-1e-9 {
		t.Fatal("equal spans must peak alike, squashed by the same factor")
	}
}

func TestAnUnsquashedArcIsASemicircle(t *testing.T) {
	s := build(t, ArcChart{Links: []Link{{"a", "b", 1}, {"b", "c", 2}}})
	ops := markKeyed(t, s, "link-0").Shape.(scene.PathShape).Ops
	base0, x0, x1 := ops[0].Y, ops[0].X, ops[2].X
	rx := (x1 - x0) / 2
	if !near(base0-ops[1].Y, rx) {
		t.Fatalf("rises %v for a %v radius", base0-ops[1].Y, rx)
	}
	// The cubic's control distance is kappa · r, which keeps it on the
	// circle: the curve's midpoint lies at the radius.
	c := ops[1]
	mx := (x0 + 3*c.X1 + 3*c.X2 + c.X) / 8
	my := (base0 + 3*c.Y1 + 3*c.Y2 + c.Y) / 8
	if d := math.Hypot(mx-(x0+rx), my-base0); math.Abs(d-rx) > 0.03/100*rx {
		t.Fatalf("midpoint %v from the center, radius %v", d, rx)
	}
	// Heavier links draw thicker, from 1px up to the max.
	if w0, w1 := markKeyed(t, s, "link-0").Stroke.Width, markKeyed(t, s, "link-1").Stroke.Width; w0 != 2.5 || w1 != arcMaxStroke {
		t.Fatalf("widths %v %v", w0, w1)
	}
	if markKeyed(t, s, "link-1").Stroke.Color != base.Link(1) {
		t.Fatal("a link takes its first node's link color")
	}
}

func TestCrowdedArcLabelsTurn(t *testing.T) {
	var c ArcChart
	for i := range 20 {
		c.Links = append(c.Links, Link{fmt.Sprintf("station %d", i), fmt.Sprintf("station %d", i+1), 1})
	}
	turned := func(s *scene.Scene) bool {
		for _, m := range s.Marks {
			if ts, ok := m.Shape.(scene.TextShape); ok && ts.Angle == -45 {
				return true
			}
		}
		return false
	}
	if !turned(buildAt(t, c, 480, 300)) {
		t.Fatal("crowded labels must turn 45°")
	}
	if turned(build(t, ArcChart{Links: []Link{{"a", "b", 1}}})) {
		t.Fatal("labels with room stay level")
	}
}
