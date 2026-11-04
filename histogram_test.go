package charta

import (
	"strconv"
	"testing"
)

func TestHistogramBarHeightsAreCountsAndEveryValueIsCounted(t *testing.T) {
	bins := 4
	b := dataRects(build(t, HistogramChart{Values: []float64{1, 2, 2.5, 3, 3.5, 3.7, 8}, Bins: &bins}))
	total, per := 0.0, 0.0
	for _, m := range b {
		n, err := strconv.ParseFloat(m.Hit.Value, 64)
		if err != nil {
			t.Fatal(err)
		}
		total += n
		if n == 1 {
			per = rectOf(m).H
		}
	}
	// A value dropped at a bin edge would silently understate the data.
	if total != 7 {
		t.Fatalf("bins count %v values, want 7", total)
	}
	// Height is proportional to the count: the bar is the count.
	for _, m := range b {
		n, _ := strconv.ParseFloat(m.Hit.Value, 64)
		if !near(rectOf(m).H, n*per) {
			t.Errorf("bar of %v is %v tall, want %v", n, rectOf(m).H, n*per)
		}
	}
	// Labels name round edges, not raw data extremes.
	if b[0].Hit.Label != "0–2" {
		t.Fatalf("first bin label %q", b[0].Hit.Label)
	}
}

func TestHistogramBinsTouchButKeepASurfaceGap(t *testing.T) {
	bins := 5
	b := dataRects(build(t, HistogramChart{Values: []float64{0, 1, 2, 3, 4, 5}, Bins: &bins}))
	// Bins are a continuous axis: no category padding, but a hairline of
	// surface so neighbours still read as separate bars.
	if gap := rectOf(b[1]).X - (rectOf(b[0]).X + rectOf(b[0]).W); !near(gap, surfaceGap) {
		t.Fatalf("gap %v, want %v", gap, surfaceGap)
	}
	// The gap is split evenly, so each bar stays centred on its bin and the
	// edge labels fall in the gaps.
	for _, m := range b {
		r, a := rectOf(m), m.Hit.Area
		if !near(r.X-a.X, a.X+a.W-(r.X+r.W)) {
			t.Fatalf("bar %v is off-centre in its bin %v", r, *a)
		}
	}
}

func TestEmptyHistogramDrawsAnEmptyFrameAndZeroBinsAreRejected(t *testing.T) {
	if b := dataRects(build(t, HistogramChart{})); len(b) != 0 {
		t.Fatalf("empty data drew %d bins", len(b))
	}
	zero := 0
	rejects(t, HistogramChart{Values: []float64{1}, Bins: &zero})
}
