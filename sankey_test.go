package charta

import (
	"math"
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func energy() SankeyChart {
	return SankeyChart{Links: []Link{
		{"Coal", "Power", 40}, {"Gas", "Power", 25}, {"Gas", "Homes", 15}, {"Power", "Homes", 30}, {"Power", "Industry", 35},
	}}
}

func TestASankeyNodeIsAsTallAsItsThroughput(t *testing.T) {
	s := build(t, energy())
	// Power takes in 65 and sends out 65; Homes only receives 45.
	power, homes := rectOf(markNamed(t, s, "Power")), rectOf(markNamed(t, s, "Homes"))
	if math.Abs(power.H/homes.H-65.0/45) >= 1e-9 {
		t.Fatalf("Power %v, Homes %v", power.H, homes.H)
	}
	h := markNamed(t, s, "Power").Hit
	if h.Value != "65" || len(h.Rows) != 2 || h.Rows[0].Value != "65" {
		t.Fatalf("a pass-through node lists in and out: %+v", h)
	}
	if h := markNamed(t, s, "Homes").Hit; len(h.Rows) != 0 {
		t.Fatalf("a sink has nothing to split into in and out: %+v", h.Rows)
	}
}

func TestSankeyLinksJoinTheBarsTheyConnect(t *testing.T) {
	s := build(t, energy())
	coal, power := rectOf(markNamed(t, s, "Coal")), rectOf(markNamed(t, s, "Power"))
	link := markNamed(t, s, "Coal → Power")
	ops := link.Shape.(scene.PathShape).Ops
	if ops[0].X != coal.X+coal.W || ops[1].X != power.X {
		t.Fatalf("link runs %v..%v, bars end %v and start %v", ops[0].X, ops[1].X, coal.X+coal.W, power.X)
	}
	// All of Coal's output is this link: it spans Coal's whole bar.
	if !near(ops[0].Y, coal.Y) || !near(ops[2].Y-ops[1].Y, coal.H) {
		t.Fatalf("link %v..%v, Coal %v+%v", ops[0].Y, ops[2].Y, coal.Y, coal.H)
	}
	if ops[1].Y < power.Y-1e-9 || ops[1].Y > power.Y+power.H {
		t.Fatal("the link must enter Power's bar")
	}
	if link.Fill != base.Link(0) || link.Hit.Rows[0].Value != "100%" || link.Hit.Rows[0].Name != "of Coal's outflow" {
		t.Fatalf("%v %+v", link.Fill, link.Hit.Rows)
	}
}

func TestTheLastSankeyColumnIsLabeledOutsideOnTheRight(t *testing.T) {
	s := build(t, energy())
	industry := rectOf(markNamed(t, s, "Industry"))
	// The legend names Industry too; node labels are drawn last.
	var label scene.TextShape
	for _, m := range s.Marks {
		if ts, ok := m.Shape.(scene.TextShape); ok && ts.Text == "Industry" {
			label = ts
		}
	}
	if label.X <= industry.X+industry.W || label.Anchor != scene.Start {
		t.Fatalf("%+v", label)
	}
	if label.X+(base.InterMetrics{}).Width("Industry", tickSize, base.Regular) > 640-pad {
		t.Fatal("the label runs off the chart")
	}
	// The first column's labels sit left of it, end-anchored.
	coal := rectOf(markNamed(t, s, "Coal"))
	for _, m := range s.Marks {
		if ts, ok := m.Shape.(scene.TextShape); ok && ts.Text == "Coal" {
			label = ts
		}
	}
	if label.Anchor != scene.End || label.X >= coal.X {
		t.Fatalf("%+v", label)
	}
}

func TestMiddleSankeyLabelsSitOnABacking(t *testing.T) {
	s := build(t, energy())
	for i, m := range s.Marks {
		if ts, ok := m.Shape.(scene.TextShape); ok && ts.Text == "Power" && i > 0 {
			if s.Marks[i-1].Fill == base.Surface {
				return
			}
		}
	}
	t.Fatal("Power's label crosses flows on both sides: it needs a halo")
}

func TestASankeyLoopIsRejectedByName(t *testing.T) {
	_, err := SankeyChart{Links: []Link{{"a", "b", 1}, {"b", "c", 1}, {"c", "b", 1}}}.Scene(640, 400, base.InterMetrics{})
	if err == nil || !strings.Contains(err.Error(), `"b"`) {
		t.Fatalf("%v", err)
	}
}

func TestAHairlineSankeyFlowStillDraws(t *testing.T) {
	// A flow far thinner than a pixel must still show: a missing band reads
	// as no flow at all.
	s := build(t, SankeyChart{Links: []Link{{"a", "b", 10000}, {"a", "c", 0.01}}})
	ops := markNamed(t, s, "a → c").Shape.(scene.PathShape).Ops
	if w := ops[2].Y - ops[1].Y; w < 1 {
		t.Fatalf("band %v thick", w)
	}
}

func TestAnEmptySankeyDrawsNothing(t *testing.T) {
	if s := build(t, SankeyChart{Title: "T"}); len(s.Marks) != 1 {
		t.Fatalf("%d marks", len(s.Marks))
	}
}
