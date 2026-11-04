package charta

import (
	"testing"

	"github.com/linzeyan/charta/scene"
)

func dots(s *scene.Scene) []scene.CircleShape {
	var out []scene.CircleShape
	for _, m := range s.Marks {
		if c, ok := m.Shape.(scene.CircleShape); ok {
			out = append(out, c)
		}
	}
	return out
}

func size(v float64) *float64 { return &v }

func TestSmallDotsGetAFullSizeHoverTarget(t *testing.T) {
	s := build(t, ScatterChart{Series: []PointSeries{{Name: "a", Points: []Point{{X: 1, Y: 1}, {X: 5, Y: 3}}}}})
	d := dots(s)[0]
	if d.R*2 < 8 {
		t.Fatal("markers are at least 8px")
	}
	// 10px off-center is outside the dot but inside the target.
	if i, ok := s.HitTest(d.CX+10, d.CY, nil); !ok || s.Marks[i].Hit == nil {
		t.Fatal("a pinpoint dot must have a forgiving target")
	}
}

func TestBubbleAreaNotRadiusTracksSize(t *testing.T) {
	s := build(t, BubbleChart{Series: []PointSeries{{Name: "a", Points: []Point{
		{X: 0, Y: 0, Size: size(100)}, {X: 1, Y: 1, Size: size(25)},
	}}}})
	d := dots(s)
	// Sorted big first; a quarter of the size is half the radius.
	if !near(d[0].R/d[1].R, 2) {
		t.Fatalf("radii %v %v", d[0].R, d[1].R)
	}
	rejects(t, BubbleChart{Series: []PointSeries{{Name: "a", Points: []Point{{X: 0, Y: 0}}}}})
}

func TestBigBubblesPaintFirstSoSmallOnesStayVisible(t *testing.T) {
	s := build(t, BubbleChart{Series: []PointSeries{{Name: "a", Points: []Point{
		{X: 0, Y: 0, Size: size(1)}, {X: 1, Y: 1, Size: size(50)}, {X: 2, Y: 2, Size: size(9)},
	}}}})
	d := dots(s)
	if !(d[0].R > d[1].R && d[1].R > d[2].R) {
		t.Fatalf("paint order %v", d)
	}
}

func TestMoreThanThreePointSeriesIsRejected(t *testing.T) {
	var c ScatterChart
	for _, n := range []string{"a", "b", "c", "d"} {
		c.Series = append(c.Series, PointSeries{Name: n, Points: []Point{{}}})
	}
	rejects(t, c)
}

func TestPointTooltipNamesPointAndSeries(t *testing.T) {
	s := build(t, ScatterChart{Series: []PointSeries{
		{Name: "Laptops", Points: []Point{{X: 1299, Y: 4.5, Label: "Pro"}}},
		{Name: "Tablets", Points: []Point{{X: 329, Y: 4}}},
	}})
	var labels []string
	for _, m := range s.Marks {
		if m.Hit != nil && m.Hit.Kind == scene.HitMark {
			labels = append(labels, m.Hit.Label+"|"+m.Hit.Value)
		}
	}
	// With several series a point label alone would be ambiguous.
	if len(labels) != 2 || labels[0] != "Pro · Laptops|1,299, 4.5" || labels[1] != "Tablets|329, 4" {
		t.Fatal(labels)
	}
}
