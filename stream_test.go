package charta

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func streamChart() StreamChart {
	times := make([]int64, 5)
	for i := range times {
		times[i] = int64(i) * base.Week
	}
	return StreamChart{Times: times, Series: []Series{
		{Name: "Rock", Values: []float64{5, 6, 4, 3, 2}},
		{Name: "Pop", Values: []float64{2, 4, 7, 9, 8}},
		{Name: "Jazz", Values: []float64{1, 1, 2, 1, 1}},
	}}
}

func TestEveryStreamLayerIsAClosedSmoothBand(t *testing.T) {
	var layers []scene.PathShape
	for _, m := range build(t, streamChart()).Marks {
		if p, ok := m.Shape.(scene.PathShape); ok {
			layers = append(layers, p)
			if m.Series == "" || m.Stroke == nil || m.Stroke.Color != base.Surface {
				t.Fatalf("layer %+v needs its series and a surface seam", m)
			}
		}
	}
	if len(layers) != 3 {
		t.Fatalf("%d layers, want 3", len(layers))
	}
	for _, p := range layers {
		ops := p.Ops
		if ops[0].Verb != scene.MoveTo || ops[len(ops)-1].Verb != scene.ClosePath {
			t.Fatalf("layer must be one closed outline: %+v", ops)
		}
		if !slices.ContainsFunc(ops, func(o scene.PathOp) bool { return o.Verb == scene.CubicTo }) {
			t.Fatal("layer edges must be curves")
		}
		// The lower edge continues the outline (a line, not a second move).
		if slices.ContainsFunc(ops[1:], func(o scene.PathOp) bool { return o.Verb == scene.MoveTo }) {
			t.Fatal("a second move would split the band into two open edges")
		}
	}
}

func TestTheStreamCrosshairListsEverySeriesAtThatTime(t *testing.T) {
	for _, m := range build(t, streamChart()).Marks {
		if m.Hit != nil && m.Hit.Key == "x2" {
			var got []string
			for _, r := range m.Hit.Rows {
				got = append(got, r.Value)
			}
			if !slices.Equal(got, []string{"4", "7", "2"}) || m.Hit.Kind != scene.HitCrosshair {
				t.Fatalf("rows %v kind %v", got, m.Hit.Kind)
			}
			return
		}
	}
	t.Fatal("no crosshair at x2")
}

func TestStreamLayersKeepTheirThicknessOnScreen(t *testing.T) {
	// Thickness is the only value cue: at the first time, layer heights must
	// be proportional to the values (5, 2, 1).
	var top, bot []float64
	for _, m := range build(t, streamChart()).Marks {
		if p, ok := m.Shape.(scene.PathShape); ok {
			n := len(p.Ops)
			top = append(top, p.Ops[0].Y)
			bot = append(bot, p.Ops[n-2].Y) // the lower edge ends back at the first time
		}
	}
	// Layers paint in stack order; find each by thickness ratio.
	var h []float64
	for i := range top {
		h = append(h, bot[i]-top[i])
	}
	slices.Sort(h)
	if !near(h[1]/h[0], 2) || !near(h[2]/h[0], 5) {
		t.Fatalf("thicknesses %v, want 1:2:5", h)
	}
}

func TestNegativeOrUnorderedStreamsAreRejected(t *testing.T) {
	rejects(t, StreamChart{Times: []int64{0, 1}, Series: []Series{{Name: "a", Values: []float64{1, -1}}}})
	rejects(t, StreamChart{Times: []int64{1, 0}, Series: []Series{{Name: "a", Values: []float64{1, 1}}}})
}

func TestStreamErrorsNameTheTimeAsADate(t *testing.T) {
	// Epoch milliseconds mean nothing to a reader; the date does.
	c := streamChart()
	c.Series[1].Values = []float64{2, 4, math.NaN(), 9, 8}
	_, err := c.Scene(640, 400, base.InterMetrics{})
	if err == nil || !strings.Contains(err.Error(), `"Jan 15, 1970"`) {
		t.Fatalf("%v", err)
	}
	rejects(t, StreamChart{Times: []int64{0}, Series: []Series{{"a", []float64{-1}}}})
}
