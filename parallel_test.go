package charta

import (
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func laptops() ParallelChart {
	return ParallelChart{
		Dimensions: []string{"Price", "Rating", "Weight"},
		Rows: []ParallelRow{
			{Label: "A", Group: "Budget", Values: []float64{10, 4.5, 1.2}},
			{Label: "B", Group: "Premium", Values: []float64{40, 3, 2.5}},
			{Label: "C", Group: "Budget", Values: []float64{25, 5, 0.8}},
		},
	}
}

func parallelLines(s *scene.Scene) []scene.Mark {
	var out []scene.Mark
	for _, m := range s.Marks {
		if _, ok := m.Shape.(scene.PolylineShape); ok {
			out = append(out, m)
		}
	}
	return out
}

func TestEachParallelDimensionHasItsOwnScale(t *testing.T) {
	l := parallelLines(build(t, laptops()))
	y := func(i, k int) float64 { return l[i].Shape.(scene.PolylineShape).Points[k].Y }
	// B has the highest price but the lowest rating: its line is the highest
	// on axis 0 and the lowest on axis 1. A shared scale would flatten
	// ratings against prices.
	if !(y(1, 0) < y(0, 0) && y(1, 0) < y(2, 0)) || !(y(1, 1) > y(0, 1) && y(1, 1) > y(2, 1)) {
		t.Fatal("dimensions do not scale independently")
	}
	// Each axis spans its own dimension's data: every line stays on the
	// axes. A scale borrowed from another dimension pushes lines off them.
	top, bottom := 0.0, 0.0
	for _, m := range build(t, laptops()).Marks {
		if a, ok := m.Shape.(scene.LineShape); ok && a.X1 == a.X2 && m.Stroke.Color == base.Axis {
			top, bottom = a.Y1, a.Y2
		}
	}
	for _, m := range l {
		for _, p := range m.Shape.(scene.PolylineShape).Points {
			if p.Y < top || p.Y > bottom {
				t.Fatalf("point %v leaves the axis %v..%v", p, top, bottom)
			}
		}
	}
	// Every line visits every axis at the same x.
	x1 := l[0].Shape.(scene.PolylineShape).Points[1].X
	for _, m := range l {
		p := m.Shape.(scene.PolylineShape).Points
		if len(p) != 3 || p[1].X != x1 {
			t.Fatalf("line %v misses an axis", p)
		}
	}
}

func TestGroupsColorParallelLinesAndTheTooltipListsEveryDimension(t *testing.T) {
	l := parallelLines(build(t, laptops()))
	// The group is the series, so the legend can highlight and hide it.
	if l[0].Series != "Budget" || l[1].Stroke.Color != base.Series(1) {
		t.Fatalf("group coloring: %q %v", l[0].Series, l[1].Stroke.Color)
	}
	h := l[1].Hit
	if h.Label != "B · Premium" || len(h.Rows) != 3 || h.Rows[0].Value != "40" {
		t.Fatalf("tooltip %+v", h)
	}
}

func TestMismatchedParallelRowsAndTooManyGroupsAreRejected(t *testing.T) {
	rejects(t, ParallelChart{Dimensions: []string{"a", "b"}, Rows: []ParallelRow{{Label: "x", Values: []float64{1}}}})
	many := ParallelChart{Dimensions: []string{"a", "b"}}
	for _, g := range []string{"p", "q", "r", "s"} {
		many.Rows = append(many.Rows, ParallelRow{Label: g, Group: g, Values: []float64{1, 2}})
	}
	rejects(t, many)
}
