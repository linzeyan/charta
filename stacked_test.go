package charta

import (
	"testing"

	"github.com/linzeyan/charta/scene"
)

func TestSegmentsStackWithASurfaceGapAndOnlyTheTopIsRounded(t *testing.T) {
	s := build(t, StackedBarChart{Categories: []string{"a"}, Series: []Series{{"x", []float64{10}}, {"y", []float64{20}}}})
	seg := dataRects(s)
	// Series y sits on top of x.
	x, y := rectOf(seg[0]), rectOf(seg[1])
	if g := x.Y - (y.Y + y.H); !near(g, surfaceGap) {
		t.Fatalf("gap %v", g)
	}
	if radiusOf(seg[0]) != (scene.Corners{}) || !(radiusOf(seg[1]).TL > 0 && radiusOf(seg[1]).BL == 0) {
		t.Fatal("only the outermost segment's data end is rounded")
	}
	// Heights stay proportional once the gap is added back.
	perUnit := (x.H + surfaceGap/2) / 10
	if !near(y.H+surfaceGap/2, 20*perUnit) {
		t.Fatal("segment heights must encode their values")
	}
}

func TestNegativesStackDownwardFromTheBaseline(t *testing.T) {
	s := build(t, StackedBarChart{Categories: []string{"a"}, Series: []Series{{"x", []float64{10}}, {"y", []float64{-5}}}})
	seg := dataRects(s)
	if !near(rectOf(seg[1]).Y, rectOf(seg[0]).Y+rectOf(seg[0]).H) {
		t.Fatal("negative segment must start at the baseline")
	}
	// Each direction's outermost segment is rounded at its data end.
	if !(radiusOf(seg[0]).TL > 0 && radiusOf(seg[1]).BL > 0) {
		t.Fatal("both directions need their rounded end")
	}
}
