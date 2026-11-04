package charta

import (
	"math"
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func TestARouteAcrossTheDateLineBreaksInsteadOfSpanningTheMap(t *testing.T) {
	s := build(t, ConnectionMapChart{
		Places: []Location{{"Tokyo", 139.7, 35.7}, {"San Francisco", -122.4, 37.8}},
		Links:  []Link{{"Tokyo", "San Francisco", 1}},
	})
	ops := markKeyed(t, s, "route-0").Shape.(scene.PathShape).Ops
	moves := 0
	for _, o := range ops {
		if o.Verb == scene.MoveTo {
			moves++
		}
	}
	if moves != 2 {
		t.Fatalf("%d subpaths, want the route cut in two", moves)
	}
	// No segment jumps more than a fraction of the width.
	for _, sub := range scene.Flatten(ops) {
		for k := 1; k < len(sub); k++ {
			if math.Abs(sub[k].X-sub[k-1].X) >= 100 {
				t.Fatalf("a segment jumps %v px", sub[k].X-sub[k-1].X)
			}
		}
	}
	// A route that stays on one side is one stroke.
	s = build(t, ConnectionMapChart{Places: []Location{{"A", 0, 0}, {"B", 40, 10}}, Links: []Link{{"A", "B", 1}}})
	if ops := markKeyed(t, s, "route-0").Shape.(scene.PathShape).Ops; ops[0].Verb != scene.MoveTo || len(ops) != routeSteps+1 || ops[1].Verb != scene.LineTo {
		t.Fatalf("%d ops", len(ops))
	}
}

func TestRoutesThickenWithValue(t *testing.T) {
	s := build(t, ConnectionMapChart{
		Places: []Location{{"A", 0, 0}, {"B", 40, 10}, {"C", -40, -10}},
		Links:  []Link{{"A", "B", 4}, {"A", "C", 1}},
	})
	if w := markKeyed(t, s, "route-0").Stroke.Width; w != routeMax {
		t.Errorf("the biggest route is %v wide", w)
	}
	if w := markKeyed(t, s, "route-1").Stroke.Width; !near(w, 1+(routeMax-1)/4) {
		t.Errorf("a quarter of the value is %v wide", w)
	}
	if h := markKeyed(t, s, "route-0").Hit; h.Label != "A – B" || h.Value != "4" {
		t.Errorf("%+v", h)
	}
	// A route takes its source's color and series, so hovering a place's
	// legend entry lights up the routes out of it.
	if m := markKeyed(t, s, "route-1"); m.Series != "A" || m.Stroke.Color != base.Link(0) {
		t.Errorf("route series %q color %v", m.Series, m.Stroke.Color.Token())
	}
	// Routes have no direction, so the same pair both ways is one route
	// given twice; flows do have one.
	rejects(t, ConnectionMapChart{Places: []Location{{"A", 0, 0}, {"B", 9, 9}}, Links: []Link{{"A", "B", 1}, {"B", "A", 1}}})
	build(t, FlowMapChart{Places: []Location{{"A", 0, 0}, {"B", 9, 9}}, Links: []Link{{"A", "B", 1}, {"B", "A", 1}}})
}

// labelsAndDots is every label's backing box, and every dot's center.
func labelsAndDots(s *scene.Scene) ([]scene.Rect, []scene.Point) {
	var halos []scene.Rect
	var dots []scene.Point
	for _, m := range s.Marks {
		switch sh := m.Shape.(type) {
		case scene.RectShape:
			if m.Fill == base.Surface {
				halos = append(halos, sh.Rect)
			}
		case scene.CircleShape:
			dots = append(dots, scene.Point{X: sh.CX, Y: sh.CY})
		}
	}
	return halos, dots
}

func assertClear(t *testing.T, s *scene.Scene) {
	t.Helper()
	halos, dots := labelsAndDots(s)
	meet := func(a, b scene.Rect) bool { return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H }
	for i, a := range halos {
		for _, b := range halos[i+1:] {
			if meet(a, b) {
				t.Errorf("labels %+v and %+v overlap", a, b)
			}
		}
		for _, d := range dots {
			if a.Contains(d.X, d.Y) {
				t.Errorf("a label covers the dot at (%v, %v)", d.X, d.Y)
			}
		}
	}
}

func TestCrowdedLabelsDodgeTheDots(t *testing.T) {
	// London and Paris sit a few pixels apart at world scale: drawn naively,
	// London's label runs over Paris's dot.
	s := build(t, ConnectionMapChart{
		Places: []Location{{"London", -0.1, 51.5}, {"Paris", 2.4, 48.9}, {"Brussels", 4.4, 50.8}, {"Sydney", 151.2, -33.9}},
		Links:  []Link{{"London", "Sydney", 1}, {"Paris", "Sydney", 1}, {"Brussels", "Sydney", 1}},
	})
	assertClear(t, s)
	if h, _ := labelsAndDots(s); len(h) < 3 {
		t.Fatalf("only %d names fit", len(h))
	}
}

func TestStackedLabelsDodgeEachOther(t *testing.T) {
	// One place a label's height above another: the dots stay clear of both
	// labels, so only label against label can catch the clash.
	s := build(t, ConnectionMapChart{
		Places: []Location{{"Upper", 0, 3.2}, {"Lower", 0, 0}, {"West", -100, -20}, {"East", 100, -20}},
		Links:  []Link{{"Upper", "Lower", 1}, {"Lower", "West", 1}, {"Lower", "East", 1}},
	})
	_, dots := labelsAndDots(s)
	if dy := math.Abs(dots[0].Y - dots[1].Y); !(dy >= 2*dotRadius && dy < 15) {
		t.Fatalf("the setup: %v", dy)
	}
	assertClear(t, s)
	halos, _ := labelsAndDots(s)
	if len(halos) != 4 {
		t.Fatalf("%d labels: the hub Lower keeps the right side, Upper moves left", len(halos))
	}
	// Lower (the hub, three links) is labelled first and keeps the right.
	for _, m := range s.Marks {
		tx, ok := m.Shape.(scene.TextShape)
		if !ok || tx.Size != tickSize {
			continue // the legend names them too
		}
		if (tx.Text == "Lower" && tx.Anchor != scene.Start) || (tx.Text == "Upper" && tx.Anchor != scene.End) {
			t.Errorf("%s's label anchors %v", tx.Text, tx.Anchor)
		}
	}
}

func TestLabelsStayInsideTheChart(t *testing.T) {
	// East's label would spill off the right edge, so it goes left. West's
	// right side is blocked by Near's dot and its left side is off the
	// chart, so it goes unlabelled rather than half drawn.
	s := build(t, ConnectionMapChart{Places: []Location{{"West", -180, 0}, {"Near", -178, 0}, {"East", 180, 0}}})
	halos, _ := labelsAndDots(s)
	for _, h := range halos {
		if h.X < 0 || h.X+h.W > 640 {
			t.Fatalf("a label runs off the chart: %+v", h)
		}
	}
	var labelled []string
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Size == tickSize {
			labelled = append(labelled, tx.Text)
		}
	}
	if strings.Join(labelled, ",") != "Near,East" {
		t.Fatalf("labelled %v", labelled)
	}
}

func TestLabelsKeepAGapFromEachOther(t *testing.T) {
	// Two labels 14px apart do not touch (a label box is 13px tall) but sit
	// inside the 2px gap, so the second one moves aside.
	s := build(t, ConnectionMapChart{
		Places: []Location{{"Upper", 0, 4.05}, {"Lower", 0, 0}, {"West", -100, -20}, {"East", 100, -20}},
		Links:  []Link{{"Upper", "Lower", 1}, {"Lower", "West", 1}, {"Lower", "East", 1}},
	})
	_, dots := labelsAndDots(s)
	if dy := dots[1].Y - dots[0].Y; !(dy > tickSize+2*halo && dy < tickSize+2*halo+placeLabelGap) {
		t.Fatalf("the setup: %v apart", dy)
	}
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Size == tickSize && tx.Text == "Upper" && tx.Anchor != scene.End {
			t.Fatal("Upper's label crowds Lower's")
		}
	}
}

func TestPlacesPastTheLabelLimitGoUnlabelled(t *testing.T) {
	var places []Location
	for i := range maxPlaceLabels + 1 {
		places = append(places, Location{string(rune('A'+i)) + "x", float64(i*8 - 80), 0})
	}
	s := build(t, ConnectionMapChart{Places: places})
	if h, d := labelsAndDots(s); len(h) != 0 || len(d) != maxPlaceLabels+1 {
		t.Fatalf("%d labels, %d dots", len(h), len(d))
	}
	if h, _ := labelsAndDots(build(t, ConnectionMapChart{Places: places[:maxPlaceLabels]})); len(h) == 0 {
		t.Fatal("at the limit, names are drawn")
	}
	// A place's tooltip counts its links; places no link names still show.
	s = build(t, ConnectionMapChart{Places: []Location{{"A", 0, 0}, {"B", 10, 0}, {"C", 20, 0}}, Links: []Link{{"A", "B", 1}}})
	if h := markKeyed(t, s, "place-2").Hit; h.Label != "C" || h.Rows[0].Value != "0" {
		t.Fatalf("%+v", h)
	}
	if h := markKeyed(t, s, "place-0").Hit; h.Rows[0] != (scene.TipRow{Name: "links", Value: "1"}) {
		t.Fatalf("%+v", h.Rows)
	}
}

func TestALinkToAnUnknownPlaceIsNamed(t *testing.T) {
	_, err := ConnectionMapChart{Places: []Location{{"A", 0, 0}}, Links: []Link{{"A", "B", 1}}}.Scene(640, 400, base.InterMetrics{})
	if err == nil || !strings.Contains(err.Error(), `"B"`) {
		t.Fatalf("%v", err)
	}
	rejects(t, ConnectionMapChart{Places: []Location{{"A", 0, 0}, {"A", 1, 1}}})
	rejects(t, FlowMapChart{Places: []Location{{"A", 0, 95}}})
}

func TestOppositeFlowsCurveApartAndBiggerFlowsAreThicker(t *testing.T) {
	s := build(t, FlowMapChart{
		Places: []Location{{"A", 0, 0}, {"B", 40, 0}},
		Links:  []Link{{"A", "B", 10}, {"B", "A", 2}},
	})
	midY := func(key string) float64 {
		pts := scene.Flatten(markKeyed(t, s, key).Shape.(scene.PathShape).Ops)[0]
		return pts[len(pts)/2].Y
	}
	if ab, ba := midY("flow-0"), midY("flow-1"); math.Abs(ab-ba) <= 20 {
		t.Fatalf("A→B at %v, B→A at %v", ab, ba)
	}
	w := func(key string) float64 { return markKeyed(t, s, key).Stroke.Width }
	if w("flow-0") != flowMax || !(w("flow-1") < w("flow-0")) || !near(w("flow-1"), flowMin+(flowMax-flowMin)/5) {
		t.Fatalf("widths %v %v", w("flow-0"), w("flow-1"))
	}
}

func TestTheArrowheadTipIsTheDestination(t *testing.T) {
	s := build(t, FlowMapChart{Places: []Location{{"A", 0, 0}, {"B", 40, 10}}, Links: []Link{{"A", "B", 1}}})
	dest := markKeyed(t, s, "place-1").Shape.(scene.CircleShape)
	var head []scene.PathOp
	for _, m := range s.Marks {
		if p, ok := m.Shape.(scene.PathShape); ok && m.Fill != base.NoColor && m.Stroke == nil {
			head = p.Ops
		}
	}
	if head == nil || !near(head[0].X, dest.CX) || !near(head[0].Y, dest.CY) {
		t.Fatalf("head %v, destination (%v, %v)", head, dest.CX, dest.CY)
	}
	// The shaft stops at the head's base instead of running under the tip.
	shaft := markKeyed(t, s, "flow-0").Shape.(scene.PathShape).Ops
	end := shaft[len(shaft)-1]
	baseX, baseY := (head[1].X+head[2].X)/2, (head[1].Y+head[2].Y)/2
	if !near(end.X, baseX) || !near(end.Y, baseY) {
		t.Fatalf("shaft ends at (%v, %v), head base at (%v, %v)", end.X, end.Y, baseX, baseY)
	}
	if l := math.Hypot(dest.CX-baseX, dest.CY-baseY); !near(l, 2*flowMax+4) {
		t.Fatalf("head length %v", l)
	}
}

func TestAFlowBetweenPlacesAtTheSamePointDrawsNothing(t *testing.T) {
	// It has no length and so no direction to bow or point: drawing it would
	// put NaN coordinates in the scene.
	s := build(t, FlowMapChart{
		Places: []Location{{"Here", 10, 10}, {"There", 10, 10}, {"Far", 60, 40}},
		Links:  []Link{{"Here", "There", 5}, {"Here", "Far", 1}},
	})
	for _, m := range s.Marks {
		if m.Hit != nil && m.Hit.Key == "flow-0" {
			t.Fatal("the zero-length flow was drawn")
		}
		if p, ok := m.Shape.(scene.PathShape); ok {
			for _, op := range p.Ops {
				for _, v := range []float64{op.X1, op.Y1, op.X2, op.Y2, op.X, op.Y} {
					if math.IsNaN(v) {
						t.Fatalf("NaN in %+v", p.Ops)
					}
				}
			}
		}
	}
	markKeyed(t, s, "flow-1")
}
