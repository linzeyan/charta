package layout

import (
	"math"
	"slices"
	"testing"
)

// energyFlows are energy-style flows: two sources merge, then split three
// ways, one branch skipping a column.
func energyFlows() []Flow {
	return []Flow{{0, 2, 30}, {1, 2, 20}, {2, 3, 25}, {2, 4, 15}, {2, 5, 10}, {3, 5, 5}, {0, 6, 12}}
}

func mustSankey(t *testing.T, n int, e []Flow) *SankeyLayout {
	t.Helper()
	s, cycle, ok := Sankey(n, e, 0, 300, 8)
	if !ok {
		t.Fatalf("unexpected cycle through %d", cycle)
	}
	return s
}

func TestSankeyLinksRunLeftToRightAndSinksLineUpOnTheRight(t *testing.T) {
	e := energyFlows()
	s := mustSankey(t, 7, e)
	for _, f := range e {
		if s.Layer[f.S] >= s.Layer[f.T] {
			t.Errorf("%d → %d does not run right", f.S, f.T)
		}
	}
	// 4, 5 and 6 have no outgoing links: all end in the last column, where
	// a reader looks for final destinations.
	for _, i := range []int{4, 5, 6} {
		if s.Layer[i] != s.Columns-1 {
			t.Errorf("sink %d in column %d of %d", i, s.Layer[i], s.Columns)
		}
	}
}

func TestSankeyNodesInAColumnKeepApartAndInside(t *testing.T) {
	s := mustSankey(t, 7, energyFlows())
	for c := range s.Columns {
		var col []int
		for i := range 7 {
			if s.Layer[i] == c {
				col = append(col, i)
			}
		}
		slices.SortFunc(col, func(a, b int) int { return totalCompare(s.Y0[a], s.Y0[b]) })
		for k := 1; k < len(col); k++ {
			if s.Y0[col[k]]-s.Y1[col[k-1]] < 8-1e-6 {
				t.Errorf("column %d: %d and %d closer than the padding", c, col[k-1], col[k])
			}
		}
		for _, i := range col {
			if s.Y0[i] < -1e-6 || s.Y1[i] > 300+1e-6 {
				t.Errorf("node %d spills out: %v..%v", i, s.Y0[i], s.Y1[i])
			}
		}
	}
}

func TestSankeyHeightsAreThroughputAndLinksTileEachNode(t *testing.T) {
	e := energyFlows()
	s := mustSankey(t, 7, e)
	ky := s.Width[0] / e[0].V
	for i := range 7 {
		if math.Abs((s.Y1[i]-s.Y0[i])-s.Value[i]*ky) >= 1e-9 {
			t.Errorf("node %d is not as tall as its throughput", i)
		}
		// Links leaving a node stack edge to edge from its top, in the order
		// their targets sit, so none cross at the node.
		var outs []int
		for k := range e {
			if e[k].S == i {
				outs = append(outs, k)
			}
		}
		slices.SortFunc(outs, func(a, b int) int { return totalCompare(s.SourceY[a], s.SourceY[b]) })
		for k := 1; k < len(outs); k++ {
			if s.Y0[e[outs[k-1]].T] > s.Y0[e[outs[k]].T]+1e-9 {
				t.Errorf("node %d: links %d and %d cross at the node", i, outs[k-1], outs[k])
			}
		}
		y := s.Y0[i]
		for _, k := range outs {
			if math.Abs(s.SourceY[k]-s.Width[k]/2-y) >= 1e-9 {
				t.Errorf("node %d: link %d leaves a gap or overlaps", i, k)
			}
			y += s.Width[k]
		}
	}
}

func TestSankeyReportsACycle(t *testing.T) {
	// 0 feeds the loop 1 → 2 → 1 but is not on it.
	_, cycle, ok := Sankey(3, []Flow{{0, 1, 1}, {1, 2, 1}, {2, 1, 1}}, 0, 100, 8)
	if ok || cycle != 1 {
		t.Fatalf("ok %v, cycle through %d; want node 1", ok, cycle)
	}
}

// randomFlows are deterministic pseudo-random DAGs: links only run from a
// lower index to a higher one, values 1..20.
func randomFlows(count int) [][]Flow {
	g := lcg{11}
	var out [][]Flow
	for range count {
		n := 3 + int(g.next()*10)
		var e []Flow
		for s := range n - 1 {
			// Every node but the last feeds at least one later node.
			for k := 0; k == 0 || g.next() < 0.3; k++ {
				t := s + 1 + int(g.next()*float64(n-1-s))
				if !slices.ContainsFunc(e, func(f Flow) bool { return f.S == s && f.T == t }) {
					e = append(e, Flow{s, t, 1 + math.Floor(g.next()*20)})
				}
			}
		}
		out = append(out, e)
	}
	return out
}

func TestSankeyPropertiesHoldOnRandomFlows(t *testing.T) {
	for r, e := range randomFlows(200) {
		n := 0
		for _, f := range e {
			n = max(n, f.T+1)
		}
		s := mustSankey(t, n, e)
		tallest := 0
		for c := range s.Columns {
			var col []int
			for i := range n {
				if s.Layer[i] == c {
					col = append(col, i)
				}
			}
			tallest = max(tallest, len(col))
		}
		py := 8.0
		if tallest > 1 {
			py = math.Min(8, 300/float64(tallest-1))
		}
		for c := range s.Columns {
			var col []int
			for i := range n {
				if s.Layer[i] == c {
					col = append(col, i)
				}
			}
			slices.SortFunc(col, func(a, b int) int { return totalCompare(s.Y0[a], s.Y0[b]) })
			for k, i := range col {
				if s.Y0[i] < -1e-6 || s.Y1[i] > 300+1e-6 {
					t.Fatalf("flows %d: node %d spills out (%v..%v)", r, i, s.Y0[i], s.Y1[i])
				}
				if k > 0 && s.Y0[i]-s.Y1[col[k-1]] < py-1e-6 {
					t.Fatalf("flows %d: nodes %d and %d closer than %v", r, col[k-1], i, py)
				}
			}
		}
		// Where links meet a node they stack in the order of their other
		// ends, so none cross there, and tile the node from its top.
		for i := range n {
			for _, side := range []bool{true, false} {
				var ks []int
				for k, f := range e {
					if (side && f.S == i) || (!side && f.T == i) {
						ks = append(ks, k)
					}
				}
				at := func(k int) float64 {
					if side {
						return s.SourceY[k]
					}
					return s.TargetY[k]
				}
				other := func(k int) float64 {
					if side {
						return s.Y0[e[k].T]
					}
					return s.Y0[e[k].S]
				}
				slices.SortFunc(ks, func(a, b int) int { return totalCompare(at(a), at(b)) })
				y := s.Y0[i]
				for j, k := range ks {
					if j > 0 && other(ks[j-1]) > other(k)+1e-9 {
						t.Fatalf("flows %d: links %d and %d cross at node %d", r, ks[j-1], k, i)
					}
					if math.Abs(at(k)-s.Width[k]/2-y) > 1e-9 {
						t.Fatalf("flows %d: link %d does not tile node %d", r, k, i)
					}
					y += s.Width[k]
				}
			}
		}
	}
}

func TestSankeyRelaxationStraightensLinks(t *testing.T) {
	// Stacked in input order, 0 (small) sits over 1 (big) while 0's target
	// 3 sits under 1's target 2: both links cross. Relaxing toward the
	// sources swaps the second column, and both links run level.
	s := mustSankey(t, 4, []Flow{{0, 3, 10}, {1, 2, 30}})
	for k := range 2 {
		if math.Abs(s.SourceY[k]-s.TargetY[k]) > 1 {
			t.Fatalf("link %d drops from %v to %v", k, s.SourceY[k], s.TargetY[k])
		}
	}
}
