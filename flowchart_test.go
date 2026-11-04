package charta

import (
	"cmp"
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func orderSteps() FlowChart {
	return FlowChart{
		Steps: []Step{
			{"start", "Order received", Terminal},
			{"stock", "In stock?", Decision},
			{"ship", "Ship", Process},
			{"wait", "Restock", Process},
			{"end", "Done", Terminal},
		},
		Arrows: []Arrow{{"start", "stock", ""}, {"stock", "ship", "Yes"}, {"stock", "wait", "No"}, {"wait", "stock", ""}, {"ship", "end", ""}},
	}
}

type stepBox struct {
	name string
	r    scene.Rect
}

// stepBoxes are the steps' outlines (a diamond as its bounding box).
func stepBoxes(s *scene.Scene) []stepBox {
	var out []stepBox
	for _, m := range s.Marks {
		if m.Hit == nil || m.Hit.Kind != scene.HitMark {
			continue
		}
		switch sh := m.Shape.(type) {
		case scene.RectShape:
			out = append(out, stepBox{m.Hit.Label, sh.Rect})
		case scene.PathShape:
			x0, x1, y0, y1 := math.MaxFloat64, -math.MaxFloat64, math.MaxFloat64, -math.MaxFloat64
			for _, p := range slices.Concat(scene.Flatten(sh.Ops)...) {
				x0, x1, y0, y1 = math.Min(x0, p.X), math.Max(x1, p.X), math.Min(y0, p.Y), math.Max(y1, p.Y)
			}
			out = append(out, stepBox{m.Hit.Label, scene.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}})
		}
	}
	return out
}

func boxNamed(t *testing.T, b []stepBox, name string) scene.Rect {
	t.Helper()
	for _, x := range b {
		if x.name == name {
			return x.r
		}
	}
	t.Fatalf("no step %q", name)
	return scene.Rect{}
}

func TestStepsFollowTheArrowsDownwardWithoutOverlapping(t *testing.T) {
	b := stepBoxes(buildAt(t, orderSteps(), 480, 360))
	for _, p := range [][2]string{{"Order received", "In stock?"}, {"In stock?", "Ship"}, {"Ship", "Done"}} {
		from, to := boxNamed(t, b, p[0]), boxNamed(t, b, p[1])
		if from.Y+from.H >= to.Y {
			t.Errorf("%s not above %s", p[0], p[1])
		}
	}
	for i, x := range b {
		a := x.r
		for _, y := range b[i+1:] {
			c := y.r
			if !(a.X+a.W <= c.X || c.X+c.W <= a.X || a.Y+a.H <= c.Y || c.Y+c.H <= a.Y) {
				t.Errorf("%s overlaps %s", x.name, y.name)
			}
		}
		if a.X < pad-1e-9 || a.X+a.W > 480-pad+1e-9 {
			t.Errorf("%s leaves the chart", x.name)
		}
	}
}

func TestStepKindsHaveTheirOutlines(t *testing.T) {
	s := buildAt(t, orderSteps(), 480, 360)
	if r := radiusOf(markNamed(t, s, "Order received")); r.TL != flowBoxH/2 {
		t.Fatalf("a terminal is a pill, radius %v", r.TL)
	}
	if r := radiusOf(markNamed(t, s, "Ship")); r.TL != 4 {
		t.Fatalf("a process is a rounded box, radius %v", r.TL)
	}
	d := markNamed(t, s, "In stock?")
	if ops := d.Shape.(scene.PathShape).Ops; len(ops) != 5 || d.Fill != base.Wash(0) || d.Stroke.Color != base.Series(0) {
		t.Fatalf("a decision is a diamond: %+v", d)
	}
	// Every step has an opaque base under its wash, so loops behind it do
	// not show through.
	for i, m := range s.Marks {
		if m.Hit != nil && m.Hit.Kind == scene.HitMark && s.Marks[i-1].Fill != base.Surface {
			t.Fatalf("%s has no surface base", m.Hit.Label)
		}
	}
}

func TestADecisionsTextClearsItsSlantedSides(t *testing.T) {
	s := buildAt(t, orderSteps(), 480, 360)
	d := boxNamed(t, stepBoxes(s), "In stock?")
	// At the text's top and bottom edge the diamond is narrower than at its
	// middle; the label must still fit there with half the box padding to
	// spare on each side.
	tw := (base.InterMetrics{}).Width("In stock?", legendSize, base.Regular)
	room := d.W * (1 - legendSize/d.H)
	if room < tw+flowBoxPad-1e-9 {
		t.Fatalf("%v of room for %v of text", room, tw)
	}
}

func TestAFlowChartIsCenteredAcross(t *testing.T) {
	// No loops: the boxes' extent is centered between the paddings.
	c := FlowChart{
		Steps:  []Step{{"a", "A", Terminal}, {"b", "Branch?", Decision}, {"c", "C", Process}, {"d", "D", Process}},
		Arrows: []Arrow{{"a", "b", ""}, {"b", "c", "Yes"}, {"b", "d", "No"}},
	}
	left, right := math.Inf(1), math.Inf(-1)
	for _, b := range stepBoxes(build(t, c)) {
		left, right = math.Min(left, b.r.X), math.Max(right, b.r.X+b.r.W)
	}
	if !near(left-pad, 640-pad-right) {
		t.Fatalf("extent %v..%v", left, right)
	}
}

func TestADecisionNamesWhereEachAnswerLeads(t *testing.T) {
	h := markNamed(t, buildAt(t, orderSteps(), 480, 360), "In stock?").Hit
	if h.Value != "Decision" {
		t.Fatalf("value %q", h.Value)
	}
	var rows [][2]string
	for _, r := range h.Rows {
		rows = append(rows, [2]string{r.Name, r.Value})
	}
	if !slices.Equal(rows, [][2]string{{"Yes", "Ship"}, {"No", "Restock"}}) {
		t.Fatalf("rows %v", rows)
	}
	if r := markNamed(t, buildAt(t, orderSteps(), 480, 360), "Restock").Hit.Rows; len(r) != 1 || r[0].Name != "next" {
		t.Fatalf("an unlabeled arrow reads as next: %+v", r)
	}
}

func TestALoopBackEntersFromTheRight(t *testing.T) {
	s := buildAt(t, orderSteps(), 480, 360)
	stock := boxNamed(t, stepBoxes(s), "In stock?")
	// The left-pointing arrowhead's tip touches the diamond's right corner.
	found := false
	for _, m := range s.Marks {
		ps, ok := m.Shape.(scene.PathShape)
		if !ok || m.Fill != base.TextMuted || ps.Ops[1].X <= ps.Ops[0].X {
			continue
		}
		found = true
		if !near(ps.Ops[0].X, stock.X+stock.W) || !near(ps.Ops[0].Y, stock.Y+stock.H/2) {
			t.Fatalf("tip at %v,%v; corner at %v,%v", ps.Ops[0].X, ps.Ops[0].Y, stock.X+stock.W, stock.Y+stock.H/2)
		}
	}
	if !found {
		t.Fatal("no left-pointing arrowhead")
	}
}

func TestALongerLoopRunsOutsideAShorterOne(t *testing.T) {
	// "Ask for details" → "Investigate" (2 rows up) starts right of where
	// "Resolved?" → "Triage" (5 rows up) starts and ends: only the per-row
	// step keeps the long loop's run outside the short one's.
	c := FlowChart{
		Steps: []Step{
			{"open", "Ticket opened", Terminal}, {"triage", "Triage", Process}, {"known", "Known issue?", Decision},
			{"investigate", "Investigate", Process}, {"repro", "Reproduced?", Decision}, {"details", "Ask for details", Process},
			{"fix", "Send fix", Process}, {"resolved", "Resolved?", Decision}, {"close", "Close ticket", Terminal},
		},
		Arrows: []Arrow{
			{"open", "triage", ""}, {"triage", "known", ""}, {"known", "fix", "Yes"}, {"known", "investigate", "No"},
			{"investigate", "repro", ""}, {"repro", "fix", "Yes"}, {"repro", "details", "No"}, {"details", "investigate", ""},
			{"fix", "resolved", ""}, {"resolved", "close", "Yes"}, {"resolved", "triage", "No"},
		},
	}
	s := buildAt(t, c, 640, 560)
	// Loops are the stroked paths with a vertical run as fourth op.
	type run struct{ x, length float64 }
	var runs []run
	for _, m := range s.Marks {
		ps, ok := m.Shape.(scene.PathShape)
		if !ok || m.Stroke == nil || len(ps.Ops) != 6 {
			continue
		}
		if a, b := ps.Ops[2], ps.Ops[3]; a.Verb == scene.CubicTo && b.Verb == scene.LineTo && a.X == b.X {
			runs = append(runs, run{a.X, a.Y - b.Y})
		}
	}
	if len(runs) != 2 {
		t.Fatalf("%d loops", len(runs))
	}
	slices.SortFunc(runs, func(a, b run) int { return cmp.Compare(a.length, b.length) })
	if runs[1].x <= runs[0].x {
		t.Fatalf("%v", runs)
	}
}

func TestTooLittleRoomIsAnErrorNotAnOverlap(t *testing.T) {
	// 186px leaves the four rows 10px gaps: room, but not for arrowheads.
	for _, size := range [][2]float64{{480, 120}, {480, 186}, {120, 360}} {
		var de *InvalidDataError
		if _, err := orderSteps().Scene(size[0], size[1], base.InterMetrics{}); err == nil || !errors.As(err, &de) {
			t.Errorf("%v: %v", size, err)
		}
	}
}

func TestBadArrowsAreRejected(t *testing.T) {
	base := func() FlowChart {
		return FlowChart{Steps: []Step{{"a", "A", Process}, {"b", "B", Process}}}
	}
	c := base()
	c.Arrows = []Arrow{{"b", "zz", ""}}
	rejects(t, c)
	c.Arrows = []Arrow{{"a", "a", ""}}
	rejects(t, c)
	c.Arrows = []Arrow{{"a", "b", ""}, {"a", "b", "x"}}
	rejects(t, c)
	c = base()
	c.Steps = append(c.Steps, Step{"a", "again", Process})
	rejects(t, c)
	c = base()
	c.Steps[0].Kind = 7
	rejects(t, c)
}

func TestArrowLabelsSitOnABacking(t *testing.T) {
	s := buildAt(t, orderSteps(), 480, 360)
	for i, m := range s.Marks {
		if ts, ok := m.Shape.(scene.TextShape); ok && ts.Text == "Yes" {
			if s.Marks[i-1].Fill != base.Surface || ts.Anchor != scene.Middle {
				t.Fatal("an arrow label must not be crossed by the arrow")
			}
			return
		}
	}
	t.Fatal("no Yes label")
}
