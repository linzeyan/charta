package charta

import (
	"math"
	"slices"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

type mapCircle struct {
	scene.CircleShape
	label string
}

func mapCircles(s *scene.Scene) []mapCircle {
	var out []mapCircle
	for _, m := range s.Marks {
		if c, ok := m.Shape.(scene.CircleShape); ok {
			l := ""
			if m.Hit != nil {
				l = m.Hit.Label
			}
			out = append(out, mapCircle{c, l})
		}
	}
	return out
}

func TestBubbleAreaTracksValueAndBigOnesSitUnderneath(t *testing.T) {
	c := mapCircles(build(t, BubbleMapChart{Places: []Place{{"Small", 10, 10, 25}, {"Big", 20, 20, 100}}}))
	if c[0].label != "Big" {
		t.Fatalf("drawn first: %q", c[0].label)
	}
	if r := c[1].R / c[0].R; math.Abs(r-0.5) > 1e-9 {
		t.Errorf("a quarter of the value, half the radius: %v", r)
	}
	if c[0].R != mapBubbleMax {
		t.Errorf("the biggest bubble is %v, want %v", c[0].R, mapBubbleMax)
	}
	// Zero still shows as the smallest dot, and a negative value is refused.
	z := mapCircles(build(t, BubbleMapChart{Places: []Place{{"A", 0, 0, 0}, {"B", 5, 5, 0}}}))
	if z[0].R != mapBubbleMin || z[1].R != mapBubbleMin {
		t.Errorf("zeros: %v %v", z[0].R, z[1].R)
	}
	rejects(t, BubbleMapChart{Places: []Place{{"A", 0, 0, -1}}})
	rejects(t, BubbleMapChart{Places: []Place{{"A", 0, 100, 1}}})
}

func TestSmallBubblesGetAFullHoverTarget(t *testing.T) {
	s := build(t, BubbleMapChart{Places: []Place{{"Small", 10, 10, 1}, {"Big", 20, 20, 100}}})
	big, small := markKeyed(t, s, "place-1"), markKeyed(t, s, "place-0")
	if big.Hit.Area != nil {
		t.Errorf("a big bubble is its own target: %+v", big.Hit.Area)
	}
	if a := small.Hit.Area; a == nil || a.W != minTarget || a.H != minTarget {
		t.Errorf("a small bubble's target: %+v", a)
	}
}

func TestPlacesLandWhereTheProjectionPutsThem(t *testing.T) {
	// East is right, north is up.
	c := mapCircles(build(t, BubbleMapChart{Places: []Place{{"West", -10, 0, 1}, {"North east", 10, 20, 1}}}))
	i := slices.IndexFunc(c, func(m mapCircle) bool { return m.label == "West" })
	w, ne := c[i], c[1-i]
	if !(ne.CX > w.CX && ne.CY < w.CY) {
		t.Fatalf("west %+v, north east %+v", w, ne)
	}
}

func TestDotsTakeTheirGroupsColor(t *testing.T) {
	s := build(t, DotMapChart{Dots: []Dot{{0, 0, "a"}, {1, 1, "b"}, {2, 2, "a"}, {3, 3, ""}}})
	var fills []base.Color
	var series []string
	for _, m := range s.Marks {
		if _, ok := m.Shape.(scene.CircleShape); ok {
			fills = append(fills, m.Fill)
			series = append(series, m.Series)
		}
	}
	if !slices.Equal(fills, []base.Color{base.Series(0), base.Series(1), base.Series(0), base.Series(0)}) {
		t.Errorf("fills %v", fills)
	}
	// Grouped dots belong to their group's series (the legend toggles
	// them); an ungrouped one to none.
	if !slices.Equal(series, []string{"a", "b", "a", ""}) {
		t.Errorf("series %q", series)
	}
	rejects(t, DotMapChart{Dots: []Dot{{200, 0, ""}}})
	var nine []Dot
	for _, g := range "abcdefghi" {
		nine = append(nine, Dot{0, 0, string(g)})
	}
	rejects(t, DotMapChart{Dots: nine})
	build(t, DotMapChart{Dots: nine[:8]})
}
