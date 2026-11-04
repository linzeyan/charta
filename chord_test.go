package charta

import (
	"fmt"
	"math"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func migration() ChordChart {
	return ChordChart{Links: []Link{
		{"North", "South", 30}, {"South", "North", 10}, {"North", "East", 20}, {"East", "West", 15}, {"West", "South", 25},
	}}
}

func TestChordArcsAreAsLongAsEachGroupsExchange(t *testing.T) {
	s := buildAt(t, migration(), 640, 480)
	// A ring sector's area grows linearly with its angle. North exchanges
	// 30 + 10 + 20 = 60, East 20 + 15 = 35.
	north := area(markKeyed(t, s, "group-0").Shape.(scene.PathShape).Ops)
	east := area(markKeyed(t, s, "group-2").Shape.(scene.PathShape).Ops)
	if math.Abs(north/east-60.0/35) >= 1e-3 {
		t.Fatalf("North/East %v, want %v", north/east, 60.0/35)
	}
	if h := markKeyed(t, s, "group-0").Hit; h.Value != "60" || h.Rows[0].Value != "50" || h.Rows[1].Value != "10" {
		t.Fatalf("North's tooltip %+v", h)
	}
}

func TestARibbonTakesTheColorOfTheBiggerSender(t *testing.T) {
	s := buildAt(t, migration(), 640, 480)
	// North (slot 0) sends 30, South (slot 1) sends 10.
	r := markNamed(t, s, "North ↔ South")
	if r.Fill != base.Link(0) || r.Series != "North" || r.Hit.Value != "40" {
		t.Fatalf("%+v %+v", r, r.Hit)
	}
	if r.Hit.Rows[0].Name != "North → South" || r.Hit.Rows[0].Value != "30" || r.Hit.Rows[1].Value != "10" || r.Hit.Rows[1].Color != base.Series(1) {
		t.Fatalf("rows %+v", r.Hit.Rows)
	}
	if w := markNamed(t, s, "South ↔ West"); w.Fill != base.Link(3) {
		t.Fatalf("West sends to South, got %v", w.Fill)
	}
}

func TestAChordTieGoesToTheFirstGroup(t *testing.T) {
	s := build(t, ChordChart{Links: []Link{{"A", "B", 5}, {"B", "A", 5}}})
	if f := markNamed(t, s, "A ↔ B").Fill; f != base.Link(0) {
		t.Fatalf("fill %v", f)
	}
}

func TestHoveringInsideARibbonFindsIt(t *testing.T) {
	s := buildAt(t, ChordChart{Links: []Link{{"A", "B", 1}}}, 400, 400)
	// Two groups, one ribbon: the middle of the circle is the ribbon. The
	// legend (one row) sits above the circle.
	top := pad + 16 + gap
	i, ok := s.HitTest(200, (top+400-pad)/2, nil)
	if !ok || s.Marks[i].Hit.Label != "A ↔ B" {
		t.Fatalf("hit %v %v", i, ok)
	}
}

func TestRibbonsToNeighborsLeaveFromTheFacingEnds(t *testing.T) {
	// Each arc lists its partners nearest counterclockwise first: with
	// three groups, 0's ribbon to 2 (its counterclockwise neighbor) starts
	// at 0's start and its ribbon to 1 at the far end.
	s := build(t, ChordChart{Links: []Link{{"A", "B", 1}, {"A", "C", 1}, {"B", "C", 1}}})
	cx := 320.0
	startX := func(key string) float64 { return markKeyed(t, s, key).Shape.(scene.PathShape).Ops[0].X }
	// 0 → 2 begins just past twelve o'clock, 0 → 1 a third of a turn later.
	if a, b := startX("ribbon-0-2"), startX("ribbon-0-1"); !(a > cx && b > a) {
		t.Fatalf("ribbon starts %v, %v", a, b)
	}
}

func TestANinthChordGroupIsRefused(t *testing.T) {
	var c ChordChart
	for i := 1; i < 9; i++ {
		c.Links = append(c.Links, Link{"hub", fmt.Sprintf("g%d", i), 1})
	}
	rejects(t, c)
	c.Links = c.Links[:7]
	build(t, c)
}

func TestNonRibbonLabelsNeverReadUpsideDown(t *testing.T) {
	var c NonRibbonChordChart
	for i := 1; i < 12; i++ {
		c.Links = append(c.Links, Link{"n0", fmt.Sprintf("n%d", i), float64(i)})
	}
	s := buildAt(t, c, 640, 480)
	left := 0
	for _, m := range s.Marks {
		if ts, ok := m.Shape.(scene.TextShape); ok {
			if ts.Angle < -90 || ts.Angle > 90 {
				t.Errorf("%s at %v°", ts.Text, ts.Angle)
			}
			if ts.Anchor == scene.End {
				left++
			}
		}
	}
	if left == 0 {
		t.Fatal("left-half labels must flip to end-anchored")
	}
	// Heavier links draw thicker.
	w := func(k string) float64 { return markKeyed(t, s, k).Stroke.Width }
	if w("link-10") <= w("link-0") || w("link-10") != chordMaxStroke {
		t.Fatalf("widths %v %v", w("link-0"), w("link-10"))
	}
}

func TestNonRibbonLinksCurveThroughTheMiddle(t *testing.T) {
	s := build(t, NonRibbonChordChart{Links: []Link{{"a", "b", 1}, {"b", "c", 1}}})
	ops := markKeyed(t, s, "link-0").Shape.(scene.PathShape).Ops
	if len(ops) != 2 || ops[1].Verb != scene.CubicTo {
		t.Fatalf("%+v", ops)
	}
	// The quadratic's control point is the circle's center (320, cy): the
	// cubic's controls lie 2/3 of the way there from each end.
	cx := 320.0
	if !near(ops[1].X1, ops[0].X+2.0/3*(cx-ops[0].X)) || !near(ops[1].X2, ops[1].X+2.0/3*(cx-ops[1].X)) {
		t.Fatalf("%+v", ops)
	}
}
