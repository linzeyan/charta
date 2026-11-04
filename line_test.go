package charta

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func crosshairs(s *scene.Scene) []*scene.Hit {
	var out []*scene.Hit
	for _, m := range s.Marks {
		if m.Hit != nil && m.Hit.Kind == scene.HitCrosshair {
			out = append(out, m.Hit)
		}
	}
	return out
}

func TestCrosshairTooltipListsEverySeriesAtThatX(t *testing.T) {
	s := build(t, LineChart{
		Categories: []string{"Jan", "Feb", "Mar"},
		Series:     []Series{{"North", []float64{1, 2, 3}}, {"South", []float64{3, 2.5, 1250}}},
	})
	x := crosshairs(s)
	if len(x) != 3 || x[2].Label != "Mar" {
		t.Fatalf("%d crosshairs", len(x))
	}
	var rows []string
	for _, r := range x[2].Rows {
		rows = append(rows, r.Name+"="+r.Value)
	}
	if strings.Join(rows, " ") != "North=3 South=1,250" {
		t.Fatal(rows)
	}
	// The columns tile the plot: hovering anywhere over Feb hits Feb.
	feb := x[1].Area
	i, ok := s.HitTest(feb.X+1, feb.Y+1, nil)
	if !ok || s.Marks[i].Hit.Label != "Feb" {
		t.Fatal("Feb's column must catch the pointer")
	}
}

// endLabels counts text marks tied to a series: the direct line-end labels.
func endLabels(s *scene.Scene) int {
	n := 0
	for _, m := range s.Marks {
		if _, ok := m.Shape.(scene.TextShape); ok && m.Series != "" {
			n++
		}
	}
	return n
}

func TestFewSeparatedSeriesAreLabelledAtTheirEnds(t *testing.T) {
	s := build(t, LineChart{Categories: []string{"a", "b"}, Series: []Series{{"High", []float64{10, 100}}, {"Low", []float64{5, 0}}}})
	if n := endLabels(s); n != 2 {
		t.Fatalf("%d end labels", n)
	}
}

func TestCollidingEndLabelsFallBackToTheLegend(t *testing.T) {
	s := build(t, LineChart{Categories: []string{"a", "b"}, Series: []Series{{"One", []float64{10, 50}}, {"Two", []float64{5, 50.5}}}})
	if endLabels(s) != 0 {
		t.Fatal("end labels must not be nudged apart")
	}
}

func TestStackedBandsSitOnEachOtherAndRejectNegatives(t *testing.T) {
	s := build(t, StackedAreaChart{Categories: []string{"a", "b"}, Series: []Series{{"x", []float64{1, 2}}, {"y", []float64{3, 4}}}})
	var areas [][]scene.AreaPoint
	for _, m := range s.Marks {
		if a, ok := m.Shape.(scene.AreaShape); ok {
			areas = append(areas, a.Points)
		}
	}
	// y's bottom edge is x's top edge.
	for i := range areas[0] {
		if !near(areas[0][i].Top, areas[1][i].Bottom) {
			t.Fatal("bands must not overlap or leave gaps")
		}
	}
	rejects(t, StackedAreaChart{Categories: []string{"a"}, Series: []Series{{"x", []float64{-1}}}})
}

func TestAreaFillsWithAWashNotASaturatedBlock(t *testing.T) {
	s := build(t, AreaChart{Categories: []string{"a", "b"}, Series: []Series{{"x", []float64{1, 2}}}})
	wash := false
	for _, m := range s.Marks {
		if _, ok := m.Shape.(scene.AreaShape); ok {
			if m.Fill == base.Series(0) {
				t.Fatal("area filled with the saturated series color")
			}
			wash = wash || m.Fill == base.Wash(0)
		}
	}
	if !wash {
		t.Fatal("no wash fill")
	}
}

func TestASinglePointLineIsADot(t *testing.T) {
	// One category gives a line with no length; it must still be visible.
	s := build(t, LineChart{Categories: []string{"a"}, Series: []Series{{"x", []float64{3}}}})
	for _, m := range s.Marks {
		if c, ok := m.Shape.(scene.CircleShape); ok && c.R == dotRadius && m.Fill == base.Series(0) {
			return
		}
	}
	t.Fatal("no dot for the single point")
}

func paths(s *scene.Scene) [][]scene.PathOp {
	var out [][]scene.PathOp
	for _, m := range s.Marks {
		if p, ok := m.Shape.(scene.PathShape); ok {
			out = append(out, p.Ops)
		}
	}
	return out
}

func TestAMonotoneLinePassesThroughTheDataAndNeverOvershoots(t *testing.T) {
	// A step: a naive spline would dip below the low and bulge above the
	// high, showing values the series never had.
	c := LineChart{Categories: []string{"a", "b", "c", "d", "e"}, Series: []Series{{"x", []float64{0, 0, 10, 10, 10}}}}
	linear := build(t, c)
	if len(paths(linear)) != 0 {
		t.Fatal("linear is the default")
	}
	var data []float64
	for _, m := range linear.Marks {
		if p, ok := m.Shape.(scene.PolylineShape); ok {
			for _, q := range p.Points {
				data = append(data, q.Y)
			}
		}
	}
	c.Curve = CurveMonotone
	p := paths(build(t, c))
	if len(p) != 1 || len(p[0]) != len(data) {
		t.Fatalf("%d paths", len(p))
	}
	for i, o := range p[0] {
		if want := scene.CubicTo; i == 0 && o.Verb != scene.MoveTo || i > 0 && o.Verb != want {
			t.Fatalf("op %d is %+v", i, o)
		}
		if o.Y != data[i] {
			t.Fatalf("the curve misses value %d: %v, want %v", i, o.Y, data[i])
		}
	}
	low, high := data[0], data[4]
	for _, sub := range scene.Flatten(p[0]) {
		for _, q := range sub {
			if q.Y > low+1e-9 || q.Y < high-1e-9 {
				t.Fatalf("overshoot to %v", q.Y)
			}
		}
	}
}

func TestMonotoneStackedBandsStillMeetTheirNeighbors(t *testing.T) {
	s := build(t, StackedAreaChart{
		Categories: []string{"a", "b", "c", "d"},
		Series:     []Series{{"x", []float64{1, 4, 2, 3}}, {"y", []float64{2, 1, 3, 1}}},
		Curve:      CurveMonotone,
	})
	p := paths(s)
	// Two filled bands, then the two top-edge lines.
	if len(p) != 4 {
		t.Fatalf("%d paths", len(p))
	}
	cubics := func(ops []scene.PathOp) []scene.PathOp {
		var out []scene.PathOp
		for _, o := range ops {
			if o.Verb == scene.CubicTo {
				out = append(out, o)
			}
		}
		return out
	}
	// Band x's top edge comes first in its outline; band y's bottom edge
	// runs the same curve backward in the second half of its own.
	top, bottom := cubics(p[0]), cubics(p[1])[3:]
	if len(top) != 6 || len(bottom) != 3 {
		t.Fatalf("%d and %d cubics", len(top), len(bottom))
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	for k, c := range top[:3] {
		b := bottom[2-k]
		// Reversed: b's first control is c's second and vice versa.
		if !near(c.X2, b.X1) || !near(c.Y2, b.Y1) || !near(c.X1, b.X2) || !near(c.Y1, b.Y2) {
			t.Fatalf("the bands part: %+v vs %+v", c, b)
		}
	}
	// The band's top line is the same curve as the fill's top edge.
	if fmt.Sprint(cubics(p[2])) != fmt.Sprint(top[:3]) {
		t.Fatal("the line leaves the band's edge")
	}
}

func TestCurvesSpellTheRustSerdeNames(t *testing.T) {
	c, err := DecodeSpec([]byte(`{"type":"stackedArea","curve":"monotone"}`))
	if err != nil || c.(StackedAreaChart).Curve != CurveMonotone {
		t.Fatalf("%+v %v", c, err)
	}
	if _, err := DecodeSpec([]byte(`{"type":"line","curve":"spline"}`)); err == nil {
		t.Fatal("an unknown curve decoded")
	}
	// Linear is the default and left out, so existing specs round-trip as they were.
	out, _ := json.Marshal(LineChart{Categories: []string{"a"}})
	if strings.Contains(string(out), "curve") {
		t.Fatal(string(out))
	}
	rejects(t, LineChart{Categories: []string{"a"}, Series: []Series{{"x", []float64{1}}}, Curve: 7})
}
