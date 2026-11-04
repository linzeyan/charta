package charta

import (
	"math"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func row(h *scene.Hit, name string) string {
	for _, r := range h.Rows {
		if r.Name == name {
			return r.Value
		}
	}
	return ""
}

func TestBoxSpansTheQuartilesAndOutliersSitBeyondTheWhiskers(t *testing.T) {
	s := build(t, BoxPlotChart{Groups: []Sample{
		{"a", []float64{1, 2, 3, 4, 5, 6, 7, 8, 40}},
		{"b", []float64{10, 11, 12}},
	}})
	box := dataRects(s)[0]
	h := box.Hit
	if row(h, "Q1") != "3" || row(h, "median") != "5" || row(h, "Q3") != "7" {
		t.Fatalf("quartiles %+v", h.Rows)
	}
	// The whisker stops at the last value inside the fence, and the
	// outlier is counted, not folded into the whisker.
	if row(h, "upper whisker") != "8" || row(h, "outliers") != "1" {
		t.Fatalf("whisker/outliers %+v", h.Rows)
	}
	for _, m := range s.Marks {
		if c, ok := m.Shape.(scene.CircleShape); ok && m.Hit != nil && m.Hit.Label == "a · outlier" {
			if !(c.CY < rectOf(box).Y) {
				t.Fatalf("outlier at %v is not above the box at %v", c.CY, rectOf(box).Y)
			}
			return
		}
	}
	t.Fatal("no outlier dot")
}

func TestHoveringAnOutlierWinsOverItsColumn(t *testing.T) {
	// The box's hover area is the whole column; the outlier is painted on
	// top and must still report itself.
	s := build(t, BoxPlotChart{Groups: []Sample{{"a", []float64{1, 2, 3, 4, 5, 40}}}})
	for _, m := range s.Marks {
		if c, ok := m.Shape.(scene.CircleShape); ok && m.Hit != nil {
			i, ok := s.HitTest(c.CX+6, c.CY, nil)
			if !ok || s.Marks[i].Hit.Value != "40" {
				t.Fatalf("hover beside the outlier hits %d %v", i, ok)
			}
			return
		}
	}
	t.Fatal("no outlier dot")
}

func outlines(s *scene.Scene) [][]scene.PathOp {
	var out [][]scene.PathOp
	for _, m := range s.Marks {
		if p, ok := m.Shape.(scene.PathShape); ok {
			out = append(out, p.Ops)
		}
	}
	return out
}

func TestViolinsMirrorAndShareOneDensityScale(t *testing.T) {
	var wide []float64
	for i := range 40 {
		wide = append(wide, float64(i%7))
	}
	s := build(t, ViolinChart{Groups: []Sample{{"a", wide}, {"b", []float64{3, 3.1, 2.9, 3, 3.05}}}})
	o := outlines(s)
	if len(o) != 2 {
		t.Fatalf("%d outlines", len(o))
	}
	width := func(ops []scene.PathOp) float64 {
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, op := range ops {
			if op.Verb == scene.MoveTo || op.Verb == scene.LineTo {
				lo, hi = math.Min(lo, op.X), math.Max(hi, op.X)
			}
		}
		return hi - lo
	}
	// One density scale for all: the tight sample is denser, so it is the
	// wider violin. Scaling each to its own peak would make them equal.
	if !(width(o[1]) > width(o[0])) {
		t.Fatalf("tight violin %v not wider than spread one %v", width(o[1]), width(o[0]))
	}
	// Mirrored: the first point (right side) and the last before Close (left
	// side) are equidistant from the center at the same height.
	first, last := o[0][0], o[0][len(o[0])-2]
	if first.Verb != scene.MoveTo || last.Verb != scene.LineTo || o[0][len(o[0])-1].Verb != scene.ClosePath {
		t.Fatalf("unexpected outline %v … %v", first, last)
	}
	cx := (first.X + last.X) / 2
	if first.Y != last.Y || !near(first.X-cx, cx-last.X) {
		t.Fatalf("not mirrored: %v vs %v", first, last)
	}
	// The outline is filled, so the whole violin is the hover target.
	for _, m := range s.Marks {
		if _, ok := m.Shape.(scene.PathShape); ok && (m.Fill != base.Wash(0) || m.Hit == nil) {
			t.Fatalf("violin mark %+v", m)
		}
	}
}

func TestASingleValueGroupStillHasATooltip(t *testing.T) {
	// No spread means no outline to hover; the interquartile bar carries the
	// tooltip instead of the group going silent.
	b := dataRects(build(t, ViolinChart{Groups: []Sample{{"a", []float64{5}}}}))
	if len(b) != 1 {
		t.Fatalf("%d hoverable bars", len(b))
	}
	rejects(t, BoxPlotChart{Groups: []Sample{{"a", nil}}})
}
