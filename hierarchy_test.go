package charta

import (
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/linzeyan/charta/scene"
)

// Builders for hierarchy test data.
func nleaf(name string, v float64) Node      { return Node{Name: name, Value: &v} }
func nbranch(name string, kids ...Node) Node { return Node{Name: name, Children: kids} }

// markNamed is the data mark (not a legend entry) whose tooltip label is
// label.
func markNamed(t *testing.T, s *scene.Scene, label string) scene.Mark {
	t.Helper()
	for _, m := range s.Marks {
		if m.Hit != nil && m.Hit.Kind == scene.HitMark && m.Hit.Label == label {
			return m
		}
	}
	t.Fatalf("no mark labelled %q", label)
	return scene.Mark{}
}

func hierarchySample() Node {
	return nbranch("root", nbranch("A", nleaf("a1", 1), nleaf("a2", 2)), nleaf("B", 4))
}

func TestFlatteningSumsValuesAndRecordsPathsAndBranches(t *testing.T) {
	root := hierarchySample()
	f, err := flatten(&root, true)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, n := range f.nodes {
		names = append(names, n.Name)
	}
	if !slices.Equal(names, []string{"root", "A", "a1", "a2", "B"}) {
		t.Errorf("preorder %v", names)
	}
	if !slices.Equal(f.tree.Value, []float64{7, 3, 1, 2, 4}) {
		t.Errorf("values %v", f.tree.Value)
	}
	if f.path[3] != "A / a2" {
		t.Errorf("path %q", f.path[3])
	}
	if !slices.Equal(f.branch, []int{-1, 0, 0, 0, 1}) {
		t.Errorf("branches %v", f.branch)
	}
	if !slices.Equal(f.legend(), []string{"A", "B"}) {
		t.Errorf("legend %v", f.legend())
	}
}

func TestReSortingTheTreeKeepsBranchColorsAndLegendInDataOrder(t *testing.T) {
	// B is larger, so a by-value sort puts it first; its hue and legend slot
	// must still be the second, or the legend would name the wrong colors.
	root := hierarchySample()
	f, err := flatten(&root, true)
	if err != nil {
		t.Fatal(err)
	}
	f.tree.SortByValue()
	if !slices.Equal(f.legend(), []string{"A", "B"}) {
		t.Errorf("legend %v", f.legend())
	}
	if f.color(4) != 1 || f.series(4) != "B" {
		t.Errorf("B has color %d, series %q", f.color(4), f.series(4))
	}
}

func TestOnlyTwoToEightBranchesGetTheirOwnHues(t *testing.T) {
	// One branch needs no key; nine would need a hue the palette lacks.
	for _, tc := range []struct {
		branches int
		colored  bool
	}{{1, false}, {2, true}, {8, true}, {9, false}} {
		root := Node{Name: "r"}
		for k := range tc.branches {
			root.Children = append(root.Children, nleaf(string(rune('a'+k)), 1))
		}
		f, err := flatten(&root, true)
		if err != nil {
			t.Fatal(err)
		}
		last := tc.branches
		if got := f.colored(); got != tc.colored {
			t.Errorf("%d branches: colored %v", tc.branches, got)
		}
		if want := map[bool]int{true: tc.branches - 1, false: 0}[tc.colored]; f.color(last) != want {
			t.Errorf("%d branches: last color %d, want %d", tc.branches, f.color(last), want)
		}
		if tc.colored != (f.series(last) != "") || tc.colored != (len(f.legend()) == tc.branches) {
			t.Errorf("%d branches: series %q, legend %v", tc.branches, f.series(last), f.legend())
		}
	}
}

func TestValuesMustBeOnLeavesOnly(t *testing.T) {
	leafless := nbranch("r", Node{Name: "x"})
	if _, err := flatten(&leafless, true); err == nil {
		t.Error("a leaf without a value must be rejected when sizing by value")
	}
	if _, err := flatten(&leafless, false); err != nil {
		t.Errorf("without values, a bare leaf is fine: %v", err)
	}
	both := hierarchySample()
	three := 3.0
	both.Children[0].Value = &three
	if _, err := flatten(&both, true); err == nil {
		t.Error("an inner node with its own value must be rejected")
	}
	// A positive sibling keeps the total positive: the leaf itself is what
	// must be refused.
	for _, v := range []float64{-1, math.NaN()} {
		bad := nbranch("r", nleaf("x", v), nleaf("y", 5))
		if _, err := flatten(&bad, true); err == nil {
			t.Errorf("leaf value %v must be rejected", v)
		}
	}
	zero := nbranch("r", nleaf("x", 0))
	if _, err := flatten(&zero, true); err == nil {
		t.Error("an all-zero hierarchy must be rejected")
	}
}

func TestDecodeSpecReadsHierarchies(t *testing.T) {
	// The fixtures spell nodes the Rust serde way: value only on leaves,
	// children only on branches.
	c, err := DecodeSpec([]byte(`{"type":"treemap","title":"T","root":{"name":"r","children":[{"name":"a","value":2},{"name":"b","children":[{"name":"c","value":1}]}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := TreemapChart{Title: "T", Root: nbranch("r", nleaf("a", 2), nbranch("b", nleaf("c", 1)))}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("%+v", c)
	}
	c, err = DecodeSpec([]byte(`{"type":"marimekko","categories":["A"],"series":[{"name":"x","values":[1]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if mk := c.(MarimekkoChart); mk.Categories[0] != "A" || mk.Series[0].Values[0] != 1 {
		t.Fatalf("%+v", mk)
	}
}
