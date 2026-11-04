package charta

import "testing"

func TestPyramidSidesGrowOutwardFromTheGutterOnOneSharedScale(t *testing.T) {
	s := build(t, PyramidChart{
		Categories: []string{"0-9", "10-19"},
		Left:       PyramidSide{"Male", []float64{10, 20}},
		Right:      PyramidSide{"Female", []float64{20, 5}},
	})
	b := dataRects(s)
	male0, female0 := rectOf(b[0]), rectOf(b[1])
	if !(male0.X+male0.W <= 320 && female0.X >= 320) {
		t.Fatal("sides must meet at the center gutter")
	}
	// Same value, same length, on either side.
	if !near(male0.W*2, female0.W) || !near(rectOf(b[2]).W, female0.W) {
		t.Fatal("both sides must share one scale")
	}
}

func TestNegativePyramidCountsAreRejected(t *testing.T) {
	rejects(t, PyramidChart{Categories: []string{"a"}, Left: PyramidSide{"L", []float64{-1}}, Right: PyramidSide{"R", []float64{1}}})
}
