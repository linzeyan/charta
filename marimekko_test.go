package charta

import (
	"math"
	"slices"
	"testing"

	"github.com/linzeyan/charta/base"
)

func TestEveryCellsAreaIsItsShareOfTheGrandTotal(t *testing.T) {
	s := build(t, MarimekkoChart{Categories: []string{"A", "B"}, Series: []Series{
		{Name: "x", Values: []float64{30, 10}}, {Name: "y", Values: []float64{10, 50}}}})
	cs := dataRects(s)
	// Add back the 1px gap on each side, which is not data.
	area := func(i int) float64 { r := rectOf(cs[i]); return (r.W + 2) * (r.H + 2) }
	total := 0.0
	for i := range cs {
		total += area(i)
	}
	share := func(cat, ser string) float64 {
		for i, m := range cs {
			if m.Hit.Label == cat && m.Hit.Rows[0].Name == ser {
				return area(i) / total
			}
		}
		t.Fatalf("no cell %s/%s", cat, ser)
		return 0
	}
	if math.Abs(share("A", "x")-0.3) >= 1e-9 || math.Abs(share("B", "y")-0.5) >= 1e-9 {
		t.Fatalf("shares %v %v", share("A", "x"), share("B", "y"))
	}
}

func TestColumnsAreAsWideAsTheirTotalsAndFillTheHeight(t *testing.T) {
	s := build(t, MarimekkoChart{Categories: []string{"A", "B"}, Series: []Series{
		{Name: "x", Values: []float64{10, 20}}, {Name: "y", Values: []float64{10, 40}}}})
	cs := dataRects(s)
	a, b := rectOf(cs[0]), rectOf(cs[2])
	if math.Abs((b.W+2)/(a.W+2)-3) >= 1e-9 {
		t.Error("B totals three times A")
	}
	h1 := rectOf(cs[0]).H + rectOf(cs[1]).H + 4
	h2 := rectOf(cs[2]).H + rectOf(cs[3]).H + 4
	if math.Abs(h1-h2) >= 1e-9 {
		t.Error("every column is 100%")
	}
	// The first series sits at the bottom, stacking upward.
	if rectOf(cs[0]).Y <= rectOf(cs[1]).Y {
		t.Error("series stack bottom-up")
	}
	// Columns sit side by side from the left edge, touching.
	if math.Abs(rectOf(cs[0]).X+rectOf(cs[0]).W+2-rectOf(cs[2]).X) >= 1e-9 {
		t.Error("columns do not touch")
	}
}

func TestMarimekkoTooltipsGiveBothShares(t *testing.T) {
	s := build(t, MarimekkoChart{Categories: []string{"A", "B"}, Series: []Series{
		{Name: "x", Values: []float64{30, 10}}, {Name: "y", Values: []float64{10, 50}}}})
	h := dataRects(s)[0].Hit
	if h.Label != "A" || h.Value != "" || h.Key != "0-0" {
		t.Fatalf("%+v", h)
	}
	want := []string{"x=30", "share of A=75%", "share of total=30%"}
	var got []string
	for _, r := range h.Rows {
		got = append(got, r.Name+"="+r.Value)
	}
	if !slices.Equal(got, want) || h.Rows[0].Color != base.Series(0) {
		t.Fatalf("rows %v", h.Rows)
	}
}

func TestPercentTicksRunUpTheSide(t *testing.T) {
	got := texts(build(t, MarimekkoChart{Categories: []string{"A"}, Series: []Series{{Name: "x", Values: []float64{1}}}}))
	for _, p := range []string{"0%", "25%", "50%", "75%", "100%"} {
		if !slices.Contains(got, p) {
			t.Errorf("no %s tick in %v", p, got)
		}
	}
}

func TestNarrowColumnsLeaveTheirLabelToTheTooltip(t *testing.T) {
	got := texts(build(t, MarimekkoChart{Categories: []string{"Wide", "A long sliver name"},
		Series: []Series{{Name: "x", Values: []float64{100, 1}}}}))
	if !slices.Contains(got, "Wide") || slices.Contains(got, "A long sliver name") {
		t.Fatalf("labels %v", got)
	}
}

func TestNegativeAndAllZeroDataAreRejected(t *testing.T) {
	// The positive series keeps the grand total positive: the negative
	// value itself is what must be refused.
	rejects(t, MarimekkoChart{Categories: []string{"A"}, Series: []Series{{Name: "x", Values: []float64{-1}}, {Name: "y", Values: []float64{5}}}})
	rejects(t, MarimekkoChart{Categories: []string{"A"}, Series: []Series{{Name: "x", Values: []float64{0}}}})
}
