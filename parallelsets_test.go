package charta

import (
	"cmp"
	"slices"
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func titanicSets() ParallelSetsChart {
	return ParallelSetsChart{
		Dimensions: []string{"Class", "Sex", "Survived"},
		Combinations: []Combination{
			{[]string{"First", "Female", "Yes"}, 141},
			{[]string{"First", "Male", "No"}, 118},
			{[]string{"First", "Male", "Yes"}, 62},
			{[]string{"Crew", "Male", "No"}, 670},
			{[]string{"Crew", "Male", "Yes"}, 192},
			{[]string{"Crew", "Female", "Yes"}, 20},
		},
	}
}

func marksKeyedWith(s *scene.Scene, prefix string) []scene.Mark {
	var out []scene.Mark
	for _, m := range s.Marks {
		if m.Hit != nil && m.Hit.Kind == scene.HitMark && strings.HasPrefix(m.Hit.Key, prefix) {
			out = append(out, m)
		}
	}
	return out
}

func TestEveryParallelSetsAxisDividesTheSameTotal(t *testing.T) {
	s := build(t, titanicSets())
	var perAxis [3]float64
	for _, m := range marksKeyedWith(s, "cat-") {
		perAxis[m.Hit.Key[4]-'0'] += rectOf(m).H
	}
	if !near(perAxis[0], perAxis[1]) || !near(perAxis[0], perAxis[2]) {
		t.Fatalf("%v", perAxis)
	}
	// Only the first axis wears the palette: it is the ribbons' color key.
	if m := markNamed(t, s, "Class: Crew"); m.Fill != base.Series(1) || m.Series != "Crew" {
		t.Fatalf("%v %q", m.Fill, m.Series)
	}
	if m := markNamed(t, s, "Sex: Male"); m.Fill != base.TextMuted || m.Series != "" {
		t.Fatalf("%v %q", m.Fill, m.Series)
	}
}

func TestParallelSetsCategoriesKeepTheirGap(t *testing.T) {
	// More than a label's height between categories, so labels of small
	// neighbors cannot collide.
	s := build(t, titanicSets())
	a, b := rectOf(markNamed(t, s, "Sex: Female")), rectOf(markNamed(t, s, "Sex: Male"))
	if !near(b.Y-(a.Y+a.H), parallelSetsCatGap) {
		t.Fatalf("Female ends %v, Male starts %v", a.Y+a.H, b.Y)
	}
}

func TestRibbonsTileEachCategoryAndKeepTheFirstColor(t *testing.T) {
	s := build(t, titanicSets())
	male := rectOf(markNamed(t, s, "Sex: Male"))
	// Ribbons leaving Male toward Survived stack from Male's top to its
	// bottom with no gap or overlap.
	type span struct {
		y0, y1 float64
		fill   base.Color
	}
	var spans []span
	for _, m := range marksKeyedWith(s, "ribbon-1-") {
		if strings.HasPrefix(m.Hit.Label, "Male") {
			ops := m.Shape.(scene.PathShape).Ops
			spans = append(spans, span{ops[0].Y, ops[3].Y, m.Fill})
		}
	}
	slices.SortFunc(spans, func(a, b span) int { return cmp.Compare(a.y0, b.y0) })
	if !near(spans[0].y0, male.Y) || !near(spans[len(spans)-1].y1, male.Y+male.H) {
		t.Fatalf("%v within %+v", spans, male)
	}
	for k := 1; k < len(spans); k++ {
		if !near(spans[k-1].y1, spans[k].y0) {
			t.Fatalf("gap or overlap: %v", spans)
		}
	}
	// On the right, ribbons entering Survived: No stack from its top down,
	// with no gap or overlap either.
	no := rectOf(markNamed(t, s, "Survived: No"))
	var ends [][2]float64
	for _, m := range marksKeyedWith(s, "ribbon-1-") {
		if strings.HasSuffix(m.Hit.Label, "→ No") {
			ops := m.Shape.(scene.PathShape).Ops
			ends = append(ends, [2]float64{ops[1].Y, ops[2].Y})
		}
	}
	slices.SortFunc(ends, func(a, b [2]float64) int { return cmp.Compare(a[0], b[0]) })
	if !near(ends[0][0], no.Y) || !near(ends[len(ends)-1][1], no.Y+no.H) {
		t.Fatalf("%v within %+v", ends, no)
	}
	for k := 1; k < len(ends); k++ {
		if !near(ends[k-1][1], ends[k][0]) {
			t.Fatalf("gap or overlap: %v", ends)
		}
	}
	// Male → No ribbons carry First (slot 0) and Crew (slot 1) colors.
	has := func(c base.Color) bool {
		return slices.ContainsFunc(spans, func(s span) bool { return s.fill == c })
	}
	if !has(base.Link(0)) || !has(base.Link(1)) {
		t.Fatalf("%v", spans)
	}
}

func TestRibbonsLeavingACategoryStackByWhereTheyGo(t *testing.T) {
	// On the left a category's ribbons stack by right category, so the ones
	// headed to the same place leave side by side.
	s := build(t, titanicSets())
	var no, yes []float64
	for _, m := range marksKeyedWith(s, "ribbon-1-") {
		y := m.Shape.(scene.PathShape).Ops[0].Y
		switch m.Hit.Label {
		case "Male → No":
			no = append(no, y)
		case "Male → Yes":
			yes = append(yes, y)
		}
	}
	// Survived lists Yes first (first appearance), so Yes ribbons leave
	// Male above the No ones.
	if slices.Max(yes) >= slices.Min(no) {
		t.Fatalf("Yes %v, No %v interleave", yes, no)
	}
}

func TestMismatchedAndRepeatedCombinationsAreRejected(t *testing.T) {
	rejects(t, ParallelSetsChart{Dimensions: []string{"A", "B"}, Combinations: []Combination{{[]string{"x"}, 1}}})
	rejects(t, ParallelSetsChart{Dimensions: []string{"A", "B"}, Combinations: []Combination{{[]string{"x", "y"}, 1}, {[]string{"x", "y"}, 2}}})
	rejects(t, ParallelSetsChart{Dimensions: []string{"A"}, Combinations: []Combination{{[]string{"x"}, 1}}})
	rejects(t, ParallelSetsChart{Dimensions: []string{"A", "B"}, Combinations: []Combination{{[]string{"x", "y"}, 0}}})
	var nine []Combination
	for _, c := range "abcdefghi" {
		nine = append(nine, Combination{[]string{string(c), "y"}, 1})
	}
	rejects(t, ParallelSetsChart{Dimensions: []string{"A", "B"}, Combinations: nine})
	build(t, ParallelSetsChart{Dimensions: []string{"A", "B"}, Combinations: nine[:8]})
}

func TestNoCombinationsIsAnErrorNotABlankChart(t *testing.T) {
	// With nothing to trace, a frame of empty axes would read as "all
	// zero"; the chart refuses instead, as the Rust port does.
	rejects(t, ParallelSetsChart{Dimensions: []string{"a", "b"}})
}
