package charta

import (
	"math"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func week() []Candle {
	d := func(n int) int64 { return base.ToMS(2026, 3, n, 0, 0, 0) }
	return []Candle{
		{d(2), 100, 104, 98, 103},
		{d(3), 103, 105, 99, 100},
		{d(4), 100, 101, 95, 96},
		{d(5), 96, 102, 96, 101},
	}
}

func hits(s *scene.Scene) []scene.Mark {
	var out []scene.Mark
	for _, m := range s.Marks {
		if m.Hit != nil {
			out = append(out, m)
		}
	}
	return out
}

func TestRisingCandlesAreHollowAndFallingOnesSolid(t *testing.T) {
	bodies := hits(build(t, CandlestickChart{Data: week()}))
	if len(bodies) != 4 {
		t.Fatalf("%d bodies, want 4", len(bodies))
	}
	// Shape repeats the color cue, so the direction reads without color.
	p, ok := bodies[0].Shape.(scene.PathShape)
	if !ok || bodies[0].Fill != base.Surface || bodies[0].Stroke == nil || bodies[0].Stroke.Color != priceRise {
		t.Fatalf("rising body %+v", bodies[0])
	}
	if _, ok := bodies[1].Shape.(scene.RectShape); !ok || bodies[1].Fill != priceFall {
		t.Fatalf("falling body %+v", bodies[1])
	}
	// The body spans open to close on the price scale: both are 3 units.
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, op := range p.Ops {
		if op.Verb == scene.MoveTo || op.Verb == scene.LineTo {
			lo, hi = math.Min(lo, op.Y), math.Max(hi, op.Y)
		}
	}
	if !near(hi-lo, rectOf(bodies[1]).H) || hi-lo <= 0 {
		t.Fatalf("hollow body %v tall, solid %v", hi-lo, rectOf(bodies[1]).H)
	}
	// The wick runs from the high to the low behind the body.
	var wick *scene.LineShape
	for _, m := range build(t, CandlestickChart{Data: week()}).Marks {
		if l, ok := m.Shape.(scene.LineShape); ok && m.Stroke != nil && m.Stroke.Width == priceOutline {
			wick = &l
			break
		}
	}
	if wick == nil || wick.X1 != wick.X2 || !(wick.Y2-wick.Y1 > hi-lo) {
		t.Fatalf("first wick %+v must be vertical and longer than its body", wick)
	}
}

func TestPriceTooltipsListOpenHighLowCloseWithTheDate(t *testing.T) {
	h := hits(build(t, OHLCChart{Data: week()}))[0].Hit
	if h.Label != "Mar 2, 2026" {
		t.Fatalf("label %q", h.Label)
	}
	want := []scene.TipRow{{Name: "open", Value: "100"}, {Name: "high", Value: "104"}, {Name: "low", Value: "98"}, {Name: "close", Value: "103"}}
	if len(h.Rows) != len(want) {
		t.Fatalf("rows %+v", h.Rows)
	}
	for i, r := range want {
		if h.Rows[i] != r {
			t.Errorf("row %d %+v, want %+v", i, h.Rows[i], r)
		}
	}
}

func TestOHLCTicksPointOpenLeftAndCloseRight(t *testing.T) {
	s := build(t, OHLCChart{Data: week()})
	var lines []scene.LineShape
	for _, m := range s.Marks {
		if l, ok := m.Shape.(scene.LineShape); ok && m.Stroke != nil && m.Stroke.Width == priceOutline {
			lines = append(lines, l)
		}
	}
	// open tick, close tick, range line per period.
	open, cl, rng := lines[0], lines[1], lines[2]
	if !(open.X1 < open.X2 && open.X2 == rng.X1 && cl.X1 == rng.X1 && cl.X2 > cl.X1) {
		t.Fatalf("open %+v close %+v range %+v", open, cl, rng)
	}
}

func TestInconsistentPricesAndUnorderedTimesAreRejected(t *testing.T) {
	bad := week()
	bad[1].Low = 101
	rejects(t, CandlestickChart{Data: bad})
	back := week()
	back[0], back[1] = back[1], back[0]
	rejects(t, OHLCChart{Data: back})
}

func TestPricePeriodsShareTheSmallestGap(t *testing.T) {
	// A weekend gap must not widen the candles: each period owns one
	// smallest step, and its body 60% of that.
	var d []Candle
	for i := range 40 {
		day := int64(i + 2*(i/5)) // five trading days, then a weekend
		d = append(d, Candle{Time: day * base.Day, Open: 10, High: 12, Low: 9, Close: 11})
	}
	b := hits(build(t, CandlestickChart{Data: d}))
	cx := func(m scene.Mark) float64 { return m.Hit.Area.X + m.Hit.Area.W/2 }
	slot := cx(b[1]) - cx(b[0])
	for _, m := range b {
		if !near(m.Hit.Area.W, slot) {
			t.Fatalf("slot %v, want the one-day step %v", m.Hit.Area.W, slot)
		}
	}
	if slot*priceFill >= barMax {
		t.Fatal("test needs uncapped bodies")
	}
	// A hollow body is a path; take its width from the outline.
	p := b[0].Shape.(scene.PathShape)
	if w := p.Ops[1].X - p.Ops[0].X; !near(w, slot*priceFill) {
		t.Fatalf("body %v wide for a %v slot", w, slot)
	}
}

func TestTimeAxisLabelsStayInsideThePadding(t *testing.T) {
	// Two periods: the axis ends half a day after the last, on a tick whose
	// centered label would spill past the edge unless the plot pulls in.
	d := week()[:2]
	tickLabelsInside(t, build(t, CandlestickChart{Data: d}), 640)
}

// tickLabelsInside checks that centered tick labels end inside the right
// padding.
func tickLabelsInside(t *testing.T, s *scene.Scene, width float64) {
	t.Helper()
	n := 0
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Anchor == scene.Middle && tx.Base == scene.Top {
			n++
			if r := tx.X + (base.InterMetrics{}).Width(tx.Text, tx.Size, tx.Weight)/2; r > width-pad+1e-9 {
				t.Errorf("label %q ends at %v, past %v", tx.Text, r, width-pad)
			}
		}
	}
	if n == 0 {
		t.Fatal("no tick labels")
	}
}
