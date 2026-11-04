package charta

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// filled are the paths filled with c, in paint order.
func filled(s *scene.Scene, c base.Color) [][]scene.PathOp {
	var out [][]scene.PathOp
	for _, m := range s.Marks {
		if p, ok := m.Shape.(scene.PathShape); ok && m.Fill == c {
			out = append(out, p.Ops)
		}
	}
	return out
}

func TestTheLastIconIsCutToTheFractionLeftOver(t *testing.T) {
	s := buildAt(t, PictogramChart{Unit: 1000, Icon: Icon{Kind: IconSquare}, Data: []Bar{{"A", 2500}}}, 640, 200)
	// The key icon, two whole icons and the half one, over one track.
	solid, track := filled(s, base.Series(0)), filled(s, base.Grid)
	if len(solid) != 4 || len(track) != 1 {
		t.Fatalf("%d solid, %d track", len(solid), len(track))
	}
	if r := area(solid[3]) / area(track[0]); math.Abs(r-0.5) > 1e-9 {
		t.Fatalf("the cut icon is %v of its track", r)
	}
	// The value follows the last icon, and the row is one hover target.
	h := markKeyed(t, s, "0")
	tx := s.Marks[slices.IndexFunc(s.Marks, func(m scene.Mark) bool { return m.Hit == h.Hit })].Shape.(scene.TextShape)
	trackX := scene.Flatten(track[0])[0]
	right := 0.0
	for _, p := range trackX {
		right = math.Max(right, p.X)
	}
	if tx.Text != "2,500" || tx.X <= right {
		t.Fatalf("value %q at %v, last icon ends at %v", tx.Text, tx.X, right)
	}
}

func TestWholeNumbersGetNoSliverFromFloatNoise(t *testing.T) {
	// 0.3 / 0.1 is 2.9999999999999996 in floating point.
	s := buildAt(t, PictogramChart{Unit: 0.1, Data: []Bar{{"A", 0.3}}}, 640, 200)
	if len(filled(s, base.Grid)) != 0 || len(filled(s, base.Series(0))) != 1+3 {
		t.Fatalf("%d track, %d solid", len(filled(s, base.Grid)), len(filled(s, base.Series(0))))
	}
	// And a hair over a whole number (2.1 / 0.3 is 7.000000000000001) is
	// not a sliver either.
	s = buildAt(t, PictogramChart{Unit: 0.3, Data: []Bar{{"A", 2.1}}}, 640, 200)
	if len(filled(s, base.Grid)) != 0 || len(filled(s, base.Series(0))) != 1+7 {
		t.Fatalf("2.1 / 0.3: %d track, %d solid", len(filled(s, base.Grid)), len(filled(s, base.Series(0))))
	}
}

func TestTooManyIconsAskForABiggerUnit(t *testing.T) {
	_, err := PictogramChart{Unit: 1, Data: []Bar{{"A", 500}}}.Scene(640, 200, base.InterMetrics{})
	if err == nil || !strings.Contains(err.Error(), "raise the unit") {
		t.Fatalf("%v", err)
	}
	// 60 icons would be 6px each: legible icons or none.
	rejects(t, PictogramChart{Unit: 1, Data: []Bar{{"A", 60}}})
	rejects(t, PictogramChart{Unit: 0, Data: []Bar{{"A", 5}}})
	rejects(t, PictogramChart{Unit: 0, Data: []Bar{{"A", 0}}})
	rejects(t, PictogramChart{Unit: 1, Data: []Bar{{"A", -5}}})
	rejects(t, PictogramChart{Unit: 1, Icon: Icon{Kind: IconPath, D: "M0 0 A1 1 0 0 1 2 2", Size: 24}, Data: []Bar{{"A", 5}}})
	// Icons shrink to fit before they give up, but never past iconMax.
	small := buildAt(t, PictogramChart{Unit: 1, Icon: Icon{Kind: IconSquare}, Data: []Bar{{"A", 3}}}, 640, 200)
	if w := scene.Flatten(filled(small, base.Series(0))[1])[0][1].X - scene.Flatten(filled(small, base.Series(0))[1])[0][0].X; !near(w, iconMax*20/24) {
		t.Fatalf("few icons draw %v wide", w)
	}
}

func TestAFullRowOfIconsStillFitsItsValue(t *testing.T) {
	// Icons shrink so the whole row, gaps included, ends where the value
	// column starts: the value after the last icon stays inside the chart.
	s := buildAt(t, PictogramChart{Unit: 1, Icon: Icon{Kind: IconSquare}, Data: []Bar{{"A", 40}}}, 640, 200)
	m := markKeyed(t, s, "0")
	tx := m.Shape.(scene.TextShape)
	if end := tx.X + (base.InterMetrics{}).Width(tx.Text, tickSize, base.Regular); end > 640-pad+1e-9 {
		t.Fatalf("the value ends at %v, past the padding", end)
	}
	if n := len(filled(s, base.Series(0))); n != 1+40 {
		t.Fatalf("%d icons", n)
	}
}

func tallySegments(t *testing.T, s *scene.Scene) [][2]scene.Point {
	t.Helper()
	i := slices.IndexFunc(s.Marks, func(m scene.Mark) bool { return m.Stroke != nil })
	ops := s.Marks[i].Shape.(scene.PathShape).Ops
	var segs [][2]scene.Point
	for k := 0; k+1 < len(ops); k += 2 {
		if ops[k].Verb != scene.MoveTo || ops[k+1].Verb != scene.LineTo {
			t.Fatalf("op %d is not a move and a line", k)
		}
		segs = append(segs, [2]scene.Point{{X: ops[k].X, Y: ops[k].Y}, {X: ops[k+1].X, Y: ops[k+1].Y}})
	}
	return segs
}

func TestTalliesGroupInFivesWithACrossingStroke(t *testing.T) {
	segs := tallySegments(t, buildAt(t, TallyChart{Data: []Bar{{"A", 7}}}, 640, 200))
	// 4 upright + 1 diagonal, then 2 upright.
	var upright []bool
	for _, s := range segs {
		upright = append(upright, s[0].X == s[1].X)
	}
	if !slices.Equal(upright, []bool{true, true, true, true, false, true, true}) {
		t.Fatalf("%v", upright)
	}
	// The diagonal spans all four uprights.
	d := segs[4]
	if !(d[0].X < segs[0][0].X && d[1].X > segs[3][0].X) {
		t.Fatalf("diagonal %v, uprights %v..%v", d, segs[0][0].X, segs[3][0].X)
	}
	// The next group starts a group step after the first.
	if !near(segs[5][0].X-segs[0][0].X, groupStep) {
		t.Fatalf("groups %v apart", segs[5][0].X-segs[0][0].X)
	}
	// Exactly five closes the group with its diagonal and nothing loose.
	if n := len(tallySegments(t, buildAt(t, TallyChart{Data: []Bar{{"A", 5}}}, 640, 200))); n != 5 {
		t.Fatalf("five draws %d strokes", n)
	}
	rejects(t, TallyChart{Data: []Bar{{"A", 2.5}}})
	// Past 500 marks a tally is refused however wide the chart: nobody
	// counts that many strokes.
	if _, err := (TallyChart{Data: []Bar{{"A", 501}}}).Scene(1e5, 400, base.InterMetrics{}); err == nil || !strings.Contains(err.Error(), "up to 500") {
		t.Fatalf("501 marks: %v", err)
	}
	buildAt(t, TallyChart{Data: []Bar{{"A", 500}}}, 1e5, 400)
	// A zero row draws no strokes but keeps its label and value.
	z := buildAt(t, TallyChart{Data: []Bar{{"A", 0}}}, 640, 200)
	if slices.ContainsFunc(z.Marks, func(m scene.Mark) bool { return m.Stroke != nil }) || len(z.Marks) != 2 {
		t.Fatalf("%+v", z.Marks)
	}
	// Too many marks for the width ask for a bar chart instead.
	if _, err := (TallyChart{Data: []Bar{{"A", 500}}}).Scene(320, 200, base.InterMetrics{}); err == nil || !strings.Contains(err.Error(), "bar chart") {
		t.Fatalf("%v", err)
	}
}

func dotFills(s *scene.Scene) []base.Color {
	var out []base.Color
	for _, m := range s.Marks {
		if _, ok := m.Shape.(scene.CircleShape); ok {
			out = append(out, m.Fill)
		}
	}
	return out
}

func TestDotsRoundPerCategoryAndFillInOrder(t *testing.T) {
	s := build(t, DotMatrixChart{Unit: 10, Data: []Bar{{"A", 26}, {"B", 14}}})
	if f := dotFills(s); !slices.Equal(f, []base.Color{base.Series(0), base.Series(0), base.Series(0), base.Series(1)}) {
		t.Fatalf("%v", f)
	}
	// Each dot belongs to its category's series, so the legend can toggle it.
	if m := markKeyed(t, s, "dot-3"); m.Series != "B" || m.Hit.Label != "B" || m.Hit.Value != "14" {
		t.Fatalf("%+v", m)
	}
	if !strings.Contains(s.Label, "4 dots in 2 categories, each dot 10") {
		t.Fatalf("%q", s.Label)
	}
	// One category is one series: nothing to dim it against.
	if m := markKeyed(t, build(t, DotMatrixChart{Unit: 1, Data: []Bar{{"A", 2}}}), "dot-0"); m.Series != "" {
		t.Fatalf("a lone category has series %q", m.Series)
	}
	var nine []Bar
	for _, c := range "abcdefghi" {
		nine = append(nine, Bar{string(c), 1})
	}
	rejects(t, DotMatrixChart{Unit: 1, Data: nine})
	build(t, DotMatrixChart{Unit: 1, Data: nine[:8]})
	rejects(t, DotMatrixChart{Unit: 1, Data: []Bar{{"A", 1}, {"A", 2}}})
	rejects(t, DotMatrixChart{Unit: 1, Data: []Bar{{"A", 2501}}})
	rejects(t, DotMatrixChart{Unit: -1, Data: []Bar{{"A", 1}}})
}

func TestTheDotGridTakesTheColumnCountWithTheBiggestCells(t *testing.T) {
	// 6 dots in a wide plot: one row of six beats two rows of three.
	s := buildAt(t, DotMatrixChart{Unit: 1, Data: []Bar{{"A", 6}}}, 640, 200)
	var ys []float64
	var r float64
	for _, m := range s.Marks {
		if c, ok := m.Shape.(scene.CircleShape); ok {
			ys = append(ys, c.CY)
			r = c.R
		}
	}
	if len(ys) != 6 || slices.Min(ys) != slices.Max(ys) {
		t.Fatalf("%v", ys)
	}
	// Cells cap at 20px, the dot filling 80% of its cell.
	if r != 8 {
		t.Fatalf("dot radius %v", r)
	}
	// Equally big cells in two rows of three, four or five: fewest columns
	// wins, so the rows come out even.
	even := buildAt(t, DotMatrixChart{Unit: 1, Data: []Bar{{"A", 6}}}, 640, 276)
	rows := map[float64]int{}
	for _, m := range even.Marks {
		if c, ok := m.Shape.(scene.CircleShape); ok {
			rows[c.CY]++
		}
	}
	if len(rows) != 2 {
		t.Fatalf("rows %v", rows)
	}
	for _, n := range rows {
		if n != 3 {
			t.Fatalf("rows %v, want three and three", rows)
		}
	}
	// A tall plot stacks instead.
	tall := buildAt(t, DotMatrixChart{Unit: 1, Data: []Bar{{"A", 6}}}, 120, 600)
	ys = ys[:0]
	for _, m := range tall.Marks {
		if c, ok := m.Shape.(scene.CircleShape); ok {
			ys = append(ys, c.CY)
		}
	}
	if slices.Min(ys) == slices.Max(ys) {
		t.Fatalf("a narrow plot still lays six in a row: %v", ys)
	}
}

func TestShapeAreaTracksValue(t *testing.T) {
	for _, shape := range []AreaShape{AreaCircle, AreaSquare} {
		s := build(t, ProportionalAreaChart{Shape: shape, Data: []Bar{{"A", 100}, {"B", 25}, {"C", 1}}})
		a := filled(s, base.Series(0))
		if r := area(a[1]) / area(a[0]); math.Abs(r-0.25) > 1e-3 {
			t.Errorf("%v: B is %v of A", shape, r)
		}
		// The tiny one's label still gets a slot as wide as itself, so the
		// shapes stay apart.
		type span struct{ lo, hi float64 }
		var boxes []span
		for _, ops := range a {
			b := span{math.Inf(1), math.Inf(-1)}
			for _, p := range scene.Flatten(ops)[0] {
				b.lo, b.hi = math.Min(b.lo, p.X), math.Max(b.hi, p.X)
			}
			boxes = append(boxes, b)
		}
		for k := 1; k < len(boxes); k++ {
			if boxes[k-1].hi+slotGap > boxes[k].lo+1e-9 {
				t.Errorf("%v: shapes %d and %d crowd: %v", shape, k-1, k, boxes)
			}
		}
		// Every shape sits on one baseline, the biggest filling the height.
		bottom := func(ops []scene.PathOp) float64 {
			y := math.Inf(-1)
			for _, p := range scene.Flatten(ops)[0] {
				y = math.Max(y, p.Y)
			}
			return y
		}
		if !near(bottom(a[0]), bottom(a[2])) {
			t.Errorf("%v: baselines %v and %v", shape, bottom(a[0]), bottom(a[2]))
		}
	}
	// A zero gets its labels but no shape.
	z := build(t, ProportionalAreaChart{Data: []Bar{{"A", 4}, {"B", 0}}})
	if n := len(filled(z, base.Series(0))); n != 1 {
		t.Fatalf("%d shapes", n)
	}
	rejects(t, ProportionalAreaChart{Data: []Bar{{"A", -1}}})
	rejects(t, ProportionalAreaChart{Shape: 5, Data: []Bar{{"A", 1}}})
	// Labels too wide to sit side by side are refused rather than overlapped.
	var many []Bar
	for i := range 40 {
		many = append(many, Bar{strings.Repeat("x", 10) + string(rune('A'+i)), 1})
	}
	rejects(t, ProportionalAreaChart{Data: many})
}

func TestProportionalShapesShrinkToFitAcross(t *testing.T) {
	// Five equal values in a narrow chart: width, not height, bounds them,
	// and the bisection packs the row to the plot's width.
	s := buildAt(t, ProportionalAreaChart{Shape: AreaSquare, Data: []Bar{{"A", 1}, {"B", 1}, {"C", 1}, {"D", 1}, {"E", 1}}}, 400, 400)
	a := filled(s, base.Series(0))
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, ops := range a {
		for _, p := range scene.Flatten(ops)[0] {
			lo, hi = math.Min(lo, p.X), math.Max(hi, p.X)
		}
	}
	if math.Abs(lo-pad) > 1e-6 || math.Abs(hi-(400-pad)) > 1e-6 {
		t.Fatalf("shapes span %v..%v", lo, hi)
	}
}
