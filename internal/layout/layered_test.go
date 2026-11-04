package layout

import (
	"math"
	"slices"
	"testing"
)

func fill(n int, v float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func TestLayeredEdgesPointDownAndOnlyLoopsAreReversed(t *testing.T) {
	// start → check → do → check (loop back), check → end.
	e := [][2]int{{0, 1}, {1, 2}, {2, 1}, {1, 3}}
	l := Layered(fill(4, 40), e, 20)
	if !slices.Equal(l.Reversed, []bool{false, false, true, false}) {
		t.Fatalf("reversed %v", l.Reversed)
	}
	for k, ed := range e {
		hi, lo := ed[0], ed[1]
		if l.Reversed[k] {
			hi, lo = lo, hi
		}
		if l.Layer[hi] >= l.Layer[lo] {
			t.Errorf("%d→%d does not point down once reversed", ed[0], ed[1])
		}
	}
	dag := Layered(fill(4, 40), [][2]int{{0, 1}, {0, 2}, {1, 3}, {2, 3}}, 20)
	if slices.Contains(dag.Reversed, true) {
		t.Fatalf("a DAG has nothing to reverse: %v", dag.Reversed)
	}
}

func TestLayeredASideEntryJoinsWhereItIsNeeded(t *testing.T) {
	// 0 → 1 → 2 → 3, and 4 feeds only 3: longest-path layering alone puts 4
	// in the top row with a long edge down; it belongs just above 3.
	l := Layered(fill(5, 40), [][2]int{{0, 1}, {1, 2}, {2, 3}, {4, 3}}, 20)
	if l.Layer[4] != l.Layer[3]-1 || l.Layer[0] != 0 {
		t.Fatalf("layers %v", l.Layer)
	}
	if len(l.Bends[3]) != 0 {
		t.Fatalf("the side entry still trails a long edge: %v", l.Bends[3])
	}
}

func TestLayeredALayerKeepsItsGaps(t *testing.T) {
	widths := []float64{60, 30, 50, 40, 20}
	l := Layered(widths, [][2]int{{0, 1}, {0, 2}, {0, 3}, {0, 4}}, 20)
	row := []int{1, 2, 3, 4}
	slices.SortFunc(row, func(a, b int) int { return totalCompare(l.X[a], l.X[b]) })
	for k := 1; k < len(row); k++ {
		a, b := row[k-1], row[k]
		if g := (l.X[b] - widths[b]/2) - (l.X[a] + widths[a]/2); g < 20-1e-9 {
			t.Errorf("%d and %d only %v apart", a, b, g)
		}
	}
	// The parent sits over its children's mean: the least-squares spot for
	// the edges it sends them.
	mean := 0.0
	for _, v := range row {
		mean += l.X[v]
	}
	mean /= 4
	if math.Abs(l.X[0]-mean) >= 1e-9 {
		t.Fatalf("parent at %v, children's mean %v", l.X[0], mean)
	}
	// Positions start at the leftmost box's left edge, and Width reaches
	// the rightmost one's right edge: the caller centers on exactly that.
	left, right := math.Inf(1), 0.0
	for v, w := range widths {
		left, right = math.Min(left, l.X[v]-w/2), math.Max(right, l.X[v]+w/2)
	}
	if math.Abs(left) >= 1e-9 || math.Abs(right-l.Width) >= 1e-9 {
		t.Fatalf("extent %v..%v, width %v", left, right, l.Width)
	}
}

func TestLayeredOrderingRemovesAnAvoidableCrossing(t *testing.T) {
	// The depth-first start puts 0's children (2, then 3) left of nothing of
	// 1's, so 0 → 3 crosses 1 → 2; swapping 2 and 3 clears it, and a reader
	// should never have to untangle that.
	e := [][2]int{{0, 2}, {0, 3}, {1, 2}}
	l := Layered(fill(4, 20), e, 20)
	for i, p := range e {
		for _, q := range e[i+1:] {
			a, b, c, d := p[0], p[1], q[0], q[1]
			if a != c && b != d && (l.X[a] < l.X[c]) != (l.X[b] < l.X[d]) {
				t.Errorf("%d→%d crosses %d→%d", a, b, c, d)
			}
		}
	}
}

func TestLayeredALongEdgeBendsThroughEachLayerItCrosses(t *testing.T) {
	// 0 → 1 → 2 → 3 and the shortcut 0 → 3.
	l := Layered(fill(4, 40), [][2]int{{0, 1}, {1, 2}, {2, 3}, {0, 3}}, 20)
	if len(l.Bends[3]) != 2 {
		t.Fatalf("shortcut bends %v, want one per layer crossed", l.Bends[3])
	}
	for k := range 3 {
		if len(l.Bends[k]) != 0 {
			t.Fatalf("edge %d spans one layer but bends: %v", k, l.Bends[k])
		}
	}
	// The shortcut routes beside the chain, not through its boxes.
	for k, bx := range l.Bends[3] {
		if math.Abs(bx-l.X[k+1]) < 20+20-1e-9 {
			t.Errorf("bend %d at %v runs through box %d at %v", k, bx, k+1, l.X[k+1])
		}
	}
}

func TestLayeredADecisionDiamondIsSymmetric(t *testing.T) {
	l := Layered(fill(4, 40), [][2]int{{0, 1}, {0, 2}, {1, 3}, {2, 3}}, 20)
	if math.Abs(l.X[0]-l.X[3]) >= 1e-9 || math.Abs((l.X[1]+l.X[2])/2-l.X[0]) >= 1e-9 {
		t.Fatalf("%v", l.X)
	}
}

func TestLayeredEdgesPassingThroughALayerBundleCloserThanNodes(t *testing.T) {
	// Three long edges from 0 skip layer 1 beside node 1. Their bends carry
	// no box, so half a gap keeps them apart: a full one would spread
	// parallel lines as if they were steps.
	e := [][2]int{{0, 1}, {1, 2}, {1, 3}, {1, 4}, {0, 2}, {0, 3}, {0, 4}}
	l := Layered(fill(5, 20), e, 20)
	var xs []float64
	for _, b := range l.Bends[4:] {
		xs = append(xs, b[0])
	}
	slices.SortFunc(xs, totalCompare)
	closest := math.Inf(1)
	for k := 1; k < len(xs); k++ {
		closest = math.Min(closest, xs[k]-xs[k-1])
	}
	if math.Abs(closest-10) > 1e-9 {
		t.Fatalf("bends %v: closest pair %v apart, want half the gap", xs, closest)
	}
}
