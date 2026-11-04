package charta

import (
	"math"
	"testing"

	"github.com/linzeyan/charta/scene"
)

// outerRadii are each wedge's outer radius: the distance from the center to
// its first point (the outer arc's start).
func outerRadii(s *scene.Scene, cx, cy float64) []float64 {
	var out []float64
	for _, ops := range hitPaths(s) {
		out = append(out, math.Hypot(ops[0].X-cx, ops[0].Y-cy))
	}
	return out
}

func TestWedgeAreaNotLengthIsProportionalToValue(t *testing.T) {
	s := buildAt(t, RoseChart{Categories: []string{"a", "b"}, Series: []Series{{"v", []float64{25, 100}}}}, 400, 400)
	r := outerRadii(s, 200, 200)
	// Four times the value, twice the radius (equal angles).
	if math.Abs(r[1]/r[0]-2) >= 1e-9 {
		t.Fatalf("%v", r)
	}
}

func TestSeriesStackOutwardFromTheCenter(t *testing.T) {
	s := buildAt(t, RoseChart{Categories: []string{"a"}, Series: []Series{{"x", []float64{1}}, {"y", []float64{3}}}}, 400, 400)
	wedges := hitPaths(s)
	// The first series is a pie slice from the center; the second starts
	// where it ends (at the radius of 1 out of the total 4).
	var cx, cy float64
	found := false
	for _, o := range wedges[0] {
		if o.Verb == scene.LineTo {
			cx, cy, found = o.X, o.Y, true
			break
		}
	}
	if !found || !near(cx, 200) {
		t.Fatalf("%+v", wedges[0])
	}
	r := outerRadii(s, cx, cy)
	if math.Abs(r[0]/r[1]-0.5) >= 1e-9 {
		t.Fatalf("%v", r)
	}
	// The outer wedge's inner edge is the inner wedge's outer edge.
	for _, o := range wedges[1] {
		if o.Verb == scene.LineTo {
			if !near(math.Hypot(o.X-cx, o.Y-cy), r[0]) {
				t.Fatalf("outer wedge starts at radius %v, inner ends at %v", math.Hypot(o.X-cx, o.Y-cy), r[0])
			}
			break
		}
	}
	// A stacked wedge's tooltip names its series.
	for _, m := range s.Marks {
		if h := m.Hit; h != nil && h.Key == "0-1" && (h.Value != "" || h.Rows[0].Name != "y" || h.Rows[0].Value != "3") {
			t.Fatalf("%+v", h)
		}
	}
}

func TestWedgesGoClockwiseFromTwelveOneCategoryEach(t *testing.T) {
	// Four categories: the second wedge spans three to six o'clock, and its
	// label sits outside the middle of that span, below and to the right.
	s := buildAt(t, RoseChart{Categories: []string{"N", "E", "S", "W"}, Series: []Series{{"v", []float64{1, 1, 1, 1}}}}, 400, 400)
	ops := hitPaths(s)[1]
	if ops[0].X <= 200 || math.Abs(ops[0].Y-200) > 1e-6 {
		t.Fatalf("second wedge starts at %v, %v", ops[0].X, ops[0].Y)
	}
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Text == "E" {
			if tx.X <= 200 || tx.Y <= 200 || tx.Anchor != scene.Start || tx.Base != scene.Top {
				t.Fatalf("%+v", tx)
			}
			return
		}
	}
	t.Fatal("no category label")
}

func TestRoseRingsLabelTheirValues(t *testing.T) {
	// The scale's rings read in value, above the center, on a backing.
	s := buildAt(t, RoseChart{Categories: []string{"a", "b"}, Series: []Series{{"v", []float64{30, 100}}}}, 400, 400)
	r := outerRadii(s, 200, 200)
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Text == "100" {
			if !near(200-tx.Y, r[1]) || tx.X <= 200 {
				t.Fatalf("label at %v, %v; outer ring radius %v", tx.X, tx.Y, r[1])
			}
			return
		}
	}
	t.Fatal("no ring label")
}

func TestRoseNegativeValuesAreRejected(t *testing.T) {
	rejects(t, RoseChart{Categories: []string{"a"}, Series: []Series{{"v", []float64{-1}}}})
}
