package charta

import (
	"slices"
	"testing"

	"github.com/linzeyan/charta/scene"
)

func curves(s *scene.Scene) [][]scene.Point {
	var out [][]scene.Point
	for _, m := range s.Marks {
		if p, ok := m.Shape.(scene.PolylineShape); ok {
			out = append(out, p.Points)
		}
	}
	return out
}

func TestDensityCurvesShareOneGridAndPeakWhereTheDataClusters(t *testing.T) {
	s := build(t, DensityChart{Series: []Sample{
		{"a", []float64{1, 1.2, 1.1, 0.9, 5}},
		{"b", []float64{4, 4.1, 3.9, 4.2}},
	}})
	k := curves(s)
	if len(k) != 2 {
		t.Fatalf("%d curves", len(k))
	}
	xs := func(c []scene.Point) []float64 {
		var out []float64
		for _, p := range c {
			out = append(out, p.X)
		}
		return out
	}
	// One grid for all, or the areas under the curves do not line up.
	if !slices.Equal(xs(k[0]), xs(k[1])) {
		t.Fatal("curves are sampled on different grids")
	}
	// Higher density is higher up the screen (smaller y).
	peakX := func(c []scene.Point) float64 {
		return slices.MinFunc(c, func(a, b scene.Point) int {
			if a.Y < b.Y {
				return -1
			}
			if a.Y > b.Y {
				return 1
			}
			return 0
		}).X
	}
	if !(peakX(k[0]) < peakX(k[1])) {
		t.Fatalf("a peaks at %v, b at %v: a clusters lower", peakX(k[0]), peakX(k[1]))
	}
}

func TestHoveringADensityCurveSummarisesItsSample(t *testing.T) {
	s := build(t, DensityChart{Series: []Sample{{"a", []float64{1, 2, 3}}}})
	for _, m := range s.Marks {
		if m.Hit == nil {
			continue
		}
		// A curve has no single value; its tooltip says how many values it
		// rests on and where their middle is, with no series keys.
		if m.Hit.Value != "" || m.Hit.Rows[0].Value != "3" || m.Hit.Rows[1].Value != "2" || m.Hit.Rows[0].Color != 0 {
			t.Fatalf("%+v", m.Hit)
		}
		return
	}
	t.Fatal("no hoverable curve")
}

func TestEmptySamplesAndAFourthDensitySeriesAreRejected(t *testing.T) {
	rejects(t, DensityChart{Series: []Sample{{"a", nil}}})
	rejects(t, DensityChart{Series: []Sample{{"a", []float64{1}}, {"b", []float64{1}}, {"c", []float64{1}}, {"d", []float64{1}}}})
}
