package charta

import (
	"slices"
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// daily turns prices into one value per day from the epoch.
func daily(prices ...float64) []TimeValue {
	out := make([]TimeValue, len(prices))
	for i, v := range prices {
		out[i] = TimeValue{Time: int64(i) * base.Day, Value: v}
	}
	return out
}

func TestSmallMovesAgainstTheLineAreIgnored(t *testing.T) {
	// 100 → 110, a 2% dip (ignored), 115, then a 10% drop turns it.
	s := kagiStrokes([]float64{100, 110, 107.8, 115, 103, 101}, 4)
	if len(s) != 2 {
		t.Fatalf("%d strokes, want 2: %+v", len(s), s)
	}
	if s[0].from != 100 || s[0].to != 115 || s[1].from != 115 || s[1].to != 101 {
		t.Fatalf("strokes %+v", s)
	}
	// Strokes join: each starts where the last ended.
	if s[1].first != s[0].last {
		t.Fatalf("stroke 1 starts at %d, stroke 0 ends at %d", s[1].first, s[0].last)
	}
	// A move of exactly the reversal turns the line (4% of 125 is 5); one
	// just short does not.
	if n := len(kagiStrokes([]float64{100, 125, 120}, 4)); n != 2 {
		t.Fatalf("an exact 4%% retrace gave %d strokes", n)
	}
	if n := len(kagiStrokes([]float64{100, 125, 120.5}, 4)); n != 1 {
		t.Fatalf("a 3.6%% retrace gave %d strokes", n)
	}
}

func TestTheKagiLineThickensOnlyAboveTheLastShoulder(t *testing.T) {
	s := build(t, KagiChart{Data: daily(100, 120, 105, 125, 90)})
	var widths []float64
	for _, m := range s.Marks {
		if _, ok := m.Shape.(scene.PolylineShape); ok {
			widths = append(widths, m.Stroke.Width)
		}
	}
	// Up (yang), down to 105 stays yang (above the 100 waist), up past 120
	// stays yang, down past 105 flips to yin: yang then yin.
	if !slices.Equal(widths, []float64{kagiYangWidth, kagiYinWidth}) {
		t.Fatalf("widths %v", widths)
	}
}

func TestKagiStrokesGetEqualColumnsAndNameTheirDates(t *testing.T) {
	s := build(t, KagiChart{Data: daily(100, 120, 105, 125, 90)})
	cols := hits(s)
	if len(cols) != 4 {
		t.Fatalf("%d stroke columns, want 4", len(cols))
	}
	for k, c := range cols[1:] {
		if !near(rectOf(c).W, rectOf(cols[0]).W) || !near(rectOf(c).X, rectOf(cols[k]).X+rectOf(cols[k]).W) {
			t.Fatal("time is not an axis here: strokes get equal, touching columns")
		}
	}
	// The columns fill the plot to the right padding, not past it.
	last := rectOf(cols[len(cols)-1])
	if !near(last.X+last.W, 640-pad) {
		t.Fatalf("last column ends at %v", last.X+last.W)
	}
	// Each vertical stroke runs down the middle of its column.
	for _, m := range s.Marks {
		if p, ok := m.Shape.(scene.PolylineShape); ok {
			if c := rectOf(cols[0]); !near(p.Points[0].X, c.X+c.W/2) {
				t.Fatalf("first stroke at x %v, column centre %v", p.Points[0].X, c.X+c.W/2)
			}
			break
		}
	}
	if h := cols[0].Hit; h.Label != "Jan 1, 1970 – Jan 2, 1970" || h.Value != "100 → 120" {
		t.Fatalf("first stroke %q %q", h.Label, h.Value)
	}
}

func TestKagiReversalMustBePositive(t *testing.T) {
	for _, r := range []float64{0, -1} {
		rejects(t, KagiChart{Data: daily(1, 2), Reversal: &r})
	}
	rejects(t, KagiChart{Data: []TimeValue{{Time: 1, Value: 1}, {Time: 1, Value: 2}}})
}

func TestKagiAndPointFigureLabelsCountInGoodEnglish(t *testing.T) {
	if l := build(t, KagiChart{Data: daily(10)}).Label; !strings.Contains(l, "of 1 price with") {
		t.Fatalf("%q", l)
	}
	if l := build(t, KagiChart{Data: daily(10, 11)}).Label; !strings.Contains(l, "of 2 prices with") {
		t.Fatalf("%q", l)
	}
	if l := build(t, PointFigureChart{Data: daily(10, 10.5)}).Label; !strings.Contains(l, "with 1 column,") {
		t.Fatalf("%q", l)
	}
}
