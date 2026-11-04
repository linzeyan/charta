package charta

import (
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func radarDots(s *scene.Scene) []scene.CircleShape {
	var out []scene.CircleShape
	for _, m := range s.Marks {
		if c, ok := m.Shape.(scene.CircleShape); ok && m.Hit != nil {
			out = append(out, c)
		}
	}
	return out
}

func TestDistanceFromTheCenterIsProportionalToValue(t *testing.T) {
	c := RadarChart{Axes: []string{"a", "b", "c", "d"}, Series: []Series{{"s", []float64{10, 20, 40, 5}}}}
	d := radarDots(buildAt(t, c, 400, 400))
	// Axis a points up, c points down: both sit on the vertical through the
	// center, c four times as far from it as a.
	cy := (d[0].CY*4 + d[2].CY) / 5
	if math.Abs(d[0].CX-d[2].CX) >= 1e-9 || math.Abs((d[2].CY-cy)/(cy-d[0].CY)-4) >= 1e-9 {
		t.Fatalf("%v", d)
	}
	// b points right at twice a's distance.
	if math.Abs((d[1].CX-d[0].CX)/(cy-d[0].CY)-2) >= 1e-9 {
		t.Fatalf("%v", d)
	}
}

func TestFewerThanThreeAxesOrTooManySeriesAreRejected(t *testing.T) {
	rejects(t, RadarChart{Axes: []string{"a", "b"}, Series: []Series{{"s", []float64{1, 2}}}})
	c := RadarChart{Axes: []string{"a", "b", "c"}}
	for i := range 4 {
		c.Series = append(c.Series, Series{fmt.Sprint(i), []float64{1, 1, 1}})
	}
	rejects(t, c)
	c.Series = c.Series[:3]
	buildAt(t, c, 400, 400)
	rejects(t, RadarChart{Axes: []string{"a", "b", "c"}, Series: []Series{{"s", []float64{1, -1, 1}}}})
}

func TestEveryVertexNamesItsAxisAndSeries(t *testing.T) {
	c := RadarChart{Axes: []string{"Speed", "Range", "Price"}, Series: []Series{{"A", []float64{1, 2, 3}}, {"B", []float64{3, 2, 1}}}}
	for _, m := range buildAt(t, c, 400, 400).Marks {
		if h := m.Hit; h != nil && h.Key == "1-2" {
			if h.Label != "Price" || h.Rows[0].Name != "B" || h.Rows[0].Value != "1" || h.Rows[0].Color != base.Series(1) {
				t.Fatalf("%+v", h)
			}
			// Small dots get a full-size hover target.
			if h.Area == nil || h.Area.W != minTarget {
				t.Fatalf("area %+v", h.Area)
			}
			return
		}
	}
	t.Fatal("no vertex 1-2")
}

func TestRadarRingsArePolygonsThroughTheSpokes(t *testing.T) {
	// Values between spokes are undefined, so the outer ring is a polygon
	// whose corners are the spokes' ends, and the series outline closes.
	s := buildAt(t, RadarChart{Axes: []string{"a", "b", "c", "d"}, Series: []Series{{"s", []float64{10, 10, 10, 10}}}}, 400, 400)
	var rings [][]scene.PathOp
	var spokes []scene.LineShape
	for _, m := range s.Marks {
		switch sh := m.Shape.(type) {
		case scene.PathShape:
			if m.Fill == base.NoColor {
				rings = append(rings, sh.Ops)
			} else if sh.Ops[len(sh.Ops)-1].Verb != scene.ClosePath || m.Fill != base.Wash(0) {
				t.Fatalf("series outline %+v", m)
			}
		case scene.LineShape:
			spokes = append(spokes, sh)
		}
	}
	if len(spokes) != 4 || len(rings) == 0 {
		t.Fatalf("%d spokes, %d rings", len(spokes), len(rings))
	}
	outer := rings[len(rings)-1]
	for k, sp := range spokes {
		if outer[k].Verb == scene.CubicTo || !near(outer[k].X, sp.X2) || !near(outer[k].Y, sp.Y2) {
			t.Fatalf("ring corner %d %+v, spoke end %+v", k, outer[k], sp)
		}
	}
	// The outer ring's value is not labelled: it would sit on the first
	// axis's name. Every other ring carries its value's label on the first
	// spoke.
	labelled := 0
	for _, m := range s.Marks {
		tx, ok := m.Shape.(scene.TextShape)
		if !ok || tx.Text == "a" || tx.Text == "b" || tx.Text == "c" || tx.Text == "d" {
			continue
		}
		if tx.Text == "10" {
			t.Fatal("outer ring labelled")
		}
		v, err := strconv.ParseFloat(tx.Text, 64)
		if err != nil {
			t.Fatalf("unexpected text %q", tx.Text)
		}
		on := false
		for _, r := range rings[:len(rings)-1] {
			on = on || near(r[0].Y, tx.Y)
		}
		// Rings scale linearly: a ring's corner on the first spoke sits at
		// its share of the outer ring's height.
		want := spokes[0].Y1 - (spokes[0].Y1-spokes[0].Y2)*v/10
		if !on || !near(tx.Y, want) {
			t.Fatalf("label %q at %v, want a ring at %v", tx.Text, tx.Y, want)
		}
		labelled++
	}
	if labelled != len(rings)-1 {
		t.Fatalf("%d ring labels for %d inner rings", labelled, len(rings)-1)
	}
}
