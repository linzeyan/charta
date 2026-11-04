package charta

import (
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func TestBarsInAGroupAreSeparatedByTheSurfaceGap(t *testing.T) {
	s := build(t, GroupedBarChart{
		Categories: []string{"Q1", "Q2"},
		Series:     []Series{{"2024", []float64{10, 20}}, {"2025", []float64{15, 5}}},
	})
	b := dataRects(s)
	if len(b) != 4 {
		t.Fatalf("%d bars", len(b))
	}
	if g := rectOf(b[1]).X - (rectOf(b[0]).X + rectOf(b[0]).W); !near(g, surfaceGap) {
		t.Fatalf("gap %v", g)
	}
	// Color follows the series, not the position.
	if b[0].Fill != base.Series(0) || b[1].Fill != base.Series(1) || b[3].Series != "2025" {
		t.Fatalf("%v %v %q", b[0].Fill, b[1].Fill, b[3].Series)
	}
}

func TestTwoSeriesGetALegendWithOneEntryEach(t *testing.T) {
	s := build(t, GroupedBarChart{Categories: []string{"a"}, Series: []Series{{"x", []float64{1}}, {"y", []float64{2}}}})
	n := 0
	for _, m := range s.Marks {
		if m.Hit != nil && m.Hit.Kind == scene.HitLegend {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%d legend entries", n)
	}
}

func TestMismatchedLengthsAndDuplicateNamesAreRejected(t *testing.T) {
	rejects(t, GroupedBarChart{Categories: []string{"a", "b"}, Series: []Series{{"x", []float64{1}}}})
	rejects(t, GroupedBarChart{Categories: []string{"a"}, Series: []Series{{"x", []float64{1}}, {"x", []float64{2}}}})
}

func TestLabelReadsAsEnglishAndIgnoresAnEmptyTitle(t *testing.T) {
	// Screen readers speak this label; "seriess" or ". Grouped…" is noise.
	c := GroupedBarChart{
		Categories: []string{"Q1", "Q2"},
		Series:     []Series{{Name: "North", Values: []float64{1, 2}}, {Name: "South", Values: []float64{3, 4}}},
	}
	s := build(t, c)
	if want := "Grouped bar chart with 2 series and 2 categories."; s.Label != want {
		t.Fatalf("label = %q, want %q", s.Label, want)
	}
}
