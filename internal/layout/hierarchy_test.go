package layout

import (
	"math"
	"slices"
	"testing"
)

type spec struct {
	parent int
	value  float64
}

func tree(s []spec) *Tree {
	t := &Tree{}
	for _, n := range s {
		t.Push(n.parent, n.value)
	}
	t.Sum()
	return t
}

func TestSquarifiedCellsTileTheRectWithAreasProportionalToValue(t *testing.T) {
	v := []float64{6, 6, 4, 3, 2, 2, 1}
	r := Squarify(v, Cell{0, 0, 600, 400})
	total := 0.0
	for _, x := range v {
		total += x
	}
	area := 0.0
	for k, c := range r {
		a := (c.X1 - c.X0) * (c.Y1 - c.Y0)
		if math.Abs(a/(600*400)-v[k]/total) >= 1e-9 {
			t.Errorf("cell %d has area %v, not its share", k, a)
		}
		if c.X0 < -1e-9 || c.Y0 < -1e-9 || c.X1 > 600+1e-9 || c.Y1 > 400+1e-9 {
			t.Errorf("cell %d %v leaves the rect", k, c)
		}
		area += a
		for j, o := range r[:k] {
			overlap := math.Max(math.Min(c.X1, o.X1)-math.Max(c.X0, o.X0), 0) * math.Max(math.Min(c.Y1, o.Y1)-math.Max(c.Y0, o.Y0), 0)
			if overlap >= 1e-6 {
				t.Errorf("cells %d and %d overlap", k, j)
			}
		}
	}
	if math.Abs(area-600*400) >= 1e-6 {
		t.Errorf("cells cover %v, want the whole rect", area)
	}
}

func TestSquarifiedCellsStayCloseToSquare(t *testing.T) {
	// Bruls et al.'s example: the worst aspect ratio stays small, where
	// slicing in one direction would give slivers.
	worst := 0.0
	for _, c := range Squarify([]float64{6, 6, 4, 3, 2, 2, 1}, Cell{0, 0, 6, 4}) {
		w, h := c.X1-c.X0, c.Y1-c.Y0
		worst = math.Max(worst, math.Max(w, h)/math.Min(w, h))
	}
	if worst >= 3 {
		t.Fatalf("worst aspect ratio %v", worst)
	}
}

func TestSquarifiedZerosGetEmptyCellsAndTheRestStillTile(t *testing.T) {
	// A zero between values must neither take room nor stop the row.
	r := Squarify([]float64{3, 0, 1}, Cell{0, 0, 40, 10})
	if a := (r[1].X1 - r[1].X0) * (r[1].Y1 - r[1].Y0); a != 0 {
		t.Errorf("the zero got area %v", a)
	}
	a0 := (r[0].X1 - r[0].X0) * (r[0].Y1 - r[0].Y0)
	a2 := (r[2].X1 - r[2].X0) * (r[2].Y1 - r[2].Y0)
	if math.Abs(a0-300) > 1e-9 || math.Abs(a2-100) > 1e-9 {
		t.Errorf("areas %v and %v, want 300 and 100: %v", a0, a2, r)
	}
}

func TestPackedCirclesNeverOverlapAndStayInsideTheirParent(t *testing.T) {
	s := []spec{{-1, 0}}
	for g := range 3 {
		s = append(s, spec{0, 0})
		p := len(s) - 1
		for k := range 4 + g*3 {
			s = append(s, spec{p, float64(1 + k*7%5)})
		}
	}
	tr := tree(s)
	tr.SortByValue()
	c := Pack(tr, 500, 400, 3)
	if math.Abs(c[0].R-200) >= 1e-9 || math.Abs(c[0].X-250) >= 1e-9 || math.Abs(c[0].Y-200) >= 1e-9 {
		t.Fatalf("root %+v does not fill the box's short side, centered", c[0])
	}
	for i := 1; i < tr.Len(); i++ {
		p := c[tr.Parent[i]]
		if d := math.Hypot(c[i].X-p.X, c[i].Y-p.Y); d+c[i].R > p.R+1e-6 {
			t.Errorf("%d pokes out of its parent", i)
		}
		for _, j := range tr.Children[tr.Parent[i]] {
			if j < i {
				if d := math.Hypot(c[i].X-c[j].X, c[i].Y-c[j].Y); d < c[i].R+c[j].R-1e-6 {
					t.Errorf("%d overlaps %d", i, j)
				}
			}
		}
	}
}

func TestPackedSiblingsKeepThePaddingBetweenThem(t *testing.T) {
	// Padding is room between siblings, not just a smaller drawing. As in
	// d3, it is applied at a scale estimated from an unpadded pass, so the
	// final gap is close to the padding rather than exact.
	tr := tree([]spec{{-1, 0}, {0, 1}, {0, 1}, {0, 1}})
	c := Pack(tr, 300, 300, 10)
	gap := math.Inf(1)
	for i := 1; i < 4; i++ {
		for j := 1; j < i; j++ {
			gap = math.Min(gap, math.Hypot(c[i].X-c[j].X, c[i].Y-c[j].Y)-c[i].R-c[j].R)
		}
	}
	if gap < 8 || gap > 12 {
		t.Fatalf("closest siblings are %v apart, want about the 10px padding", gap)
	}
}

func TestPackedLeafAreasAreProportionalToValue(t *testing.T) {
	c := Pack(tree([]spec{{-1, 0}, {0, 1}, {0, 4}, {0, 9}}), 300, 300, 0)
	if math.Abs(c[2].R/c[1].R-2) >= 1e-9 || math.Abs(c[3].R/c[1].R-3) >= 1e-9 {
		t.Fatalf("radii %v %v %v are not √value", c[1].R, c[2].R, c[3].R)
	}
}

func TestTheEnclosingCircleIsTheSmallestOne(t *testing.T) {
	// Two equal circles side by side: the enclosure spans exactly both.
	e := enclose([]Circle{{-1, 0, 1}, {1, 0, 1}, {0, 0, 0.5}}, &lcg{1})
	if math.Abs(e.R-2) >= 1e-9 || math.Abs(e.X) >= 1e-9 || math.Abs(e.Y) >= 1e-9 {
		t.Fatalf("%+v", e)
	}
	// Three circles that each touch the enclosure need the three-circle
	// basis: a pair would leave the third poking out.
	e = enclose([]Circle{{0, 2, 1}, {-math.Sqrt(3), -1, 1}, {math.Sqrt(3), -1, 1}, {0, 0, 0.1}}, &lcg{1})
	if math.Abs(e.R-3) >= 1e-9 || math.Abs(e.X) >= 1e-9 || math.Abs(e.Y) >= 1e-9 {
		t.Fatalf("%+v", e)
	}
	// Unequal pair: the enclosure is tangent to both, off-center.
	e = enclose([]Circle{{0, 0, 1}, {4, 0, 2}}, &lcg{1})
	if math.Abs(e.R-3.5) >= 1e-9 || math.Abs(e.X-2.5) >= 1e-9 {
		t.Fatalf("%+v", e)
	}
}

func TestTheShuffleIsD3sGenerator(t *testing.T) {
	// d3's lcg, seeded with 1: the same sequence keeps packs reproducible
	// across ports.
	g := lcg{1}
	for _, s := range []float64{1015568748, 1586005467, 2165703038} {
		if v := g.next(); v != s/4294967296 {
			t.Fatalf("got %v, want %v", v, s/4294967296)
		}
	}
}

func TestSortingByValueIsLargestFirstAndStable(t *testing.T) {
	tr := tree([]spec{{-1, 0}, {0, 1}, {0, 5}, {0, 1}, {0, 3}})
	tr.SortByValue()
	if want := []int{2, 4, 1, 3}; !slices.Equal(tr.Children[0], want) {
		t.Fatalf("children %v, want %v (equal values keep data order)", tr.Children[0], want)
	}
}

func TestTraversalsFollowTheSortedChildren(t *testing.T) {
	//      0
	//    /   \
	//   1     4      sorted: 4 (5) before 1 (2)
	//  / \
	// 2   3
	tr := tree([]spec{{-1, 0}, {0, 0}, {1, 1}, {1, 1}, {0, 5}})
	tr.SortByValue()
	if got, want := tr.preOrder(), []int{0, 4, 1, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("preorder %v, want %v", got, want)
	}
	if got, want := tr.postOrder(), []int{4, 2, 3, 1, 0}; !slices.Equal(got, want) {
		t.Errorf("postorder %v, want %v", got, want)
	}
	if !slices.Equal(tr.Value, []float64{7, 2, 1, 1, 5}) || !slices.Equal(tr.Depth, []int{0, 1, 2, 2, 1}) {
		t.Errorf("values %v depths %v", tr.Value, tr.Depth)
	}
}

func sameSep(same bool, _ int) float64 {
	if same {
		return 1
	}
	return 2
}

func TestTidyTreesCenterParentsAndKeepNeighboursApart(t *testing.T) {
	//        0
	//     /  |  \
	//    1   4   5
	//   / \      |
	//  2   3     6
	x := Tidy(tree([]spec{{-1, 0}, {0, 0}, {1, 1}, {1, 1}, {0, 1}, {0, 0}, {5, 1}}), sameSep)
	if math.Abs(x[1]-(x[2]+x[3])/2) >= 1e-9 {
		t.Error("parent not over its children")
	}
	if math.Abs(x[0]-(x[1]+x[5])/2) >= 1e-9 {
		t.Error("root not over its children")
	}
	unit := x[3] - x[2]
	// Same depth, different parents: exactly two units apart, as close as
	// allowed (a tidy tree is compact, not merely non-overlapping); siblings
	// at least one.
	if math.Abs(x[6]-x[3]-2*unit) >= 1e-9 {
		t.Errorf("cousins %v apart, want exactly %v", x[6]-x[3], 2*unit)
	}
	if x[4]-x[1] < unit-1e-9 {
		t.Errorf("siblings %v apart, want at least %v", x[4]-x[1], unit)
	}
	for i, v := range x {
		if v < 0 || v > 1 {
			t.Errorf("node %d at %v, outside [0, 1]", i, v)
		}
	}
	// Half a separation at either end: the extremes sit half a gap in.
	if math.Abs(x[2]-unit) >= 1e-9 || math.Abs(1-x[6]-unit) >= 1e-9 {
		t.Errorf("ends at %v and %v, want %v from the edges", x[2], x[6], unit)
	}
}

func TestTidySpreadsASmallSubtreeBetweenBigOnes(t *testing.T) {
	// Root with three children; the outer two have wide subtrees. The
	// middle leaf is placed evenly between them, not crammed left.
	s := []spec{{-1, 0}, {0, 0}}
	for range 4 {
		s = append(s, spec{1, 1})
	}
	s = append(s, spec{0, 1}) // 6: the middle leaf
	s = append(s, spec{0, 0}) // 7
	for range 4 {
		s = append(s, spec{7, 1})
	}
	x := Tidy(tree(s), sameSep)
	if math.Abs((x[6]-x[1])-(x[7]-x[6])) >= 1e-9 {
		t.Fatalf("%v", x)
	}
}

func TestTidyThreadsContoursThroughUnevenSubtrees(t *testing.T) {
	// A deep left subtree beside a shallow right one: the right subtree's
	// deep cousins must clear the left one's, which the contour threads
	// carry down past the shallow side.
	//        0
	//      /   \
	//     1     5
	//     |    / \
	//     2   6   7
	//    / \      |
	//   3   4     8
	x := Tidy(tree([]spec{{-1, 0}, {0, 0}, {1, 0}, {2, 1}, {2, 1}, {0, 0}, {5, 1}, {5, 0}, {7, 1}}), sameSep)
	unit := x[4] - x[3]
	if x[8]-x[4] < 2*unit-1e-9 {
		t.Errorf("deep cousins %v apart, want at least %v: %v", x[8]-x[4], 2*unit, x)
	}
	if x[6]-x[2] < 2*unit-1e-9 {
		t.Errorf("cousins %v apart, want at least %v: %v", x[6]-x[2], 2*unit, x)
	}
	if math.Abs(x[5]-(x[6]+x[7])/2) >= 1e-9 || math.Abs(x[0]-(x[1]+x[5])/2) >= 1e-9 {
		t.Errorf("parents not centered: %v", x)
	}
}

func TestSquarifyingSkipsALeadingZeroInsteadOfGluingTheRow(t *testing.T) {
	// Two equal values in a 10×40 strip make two 10×20 squares-ish cells;
	// a zero at the start of a row must not make them one 5×40 row.
	for _, c := range Squarify([]float64{0, 2, 2}, Cell{0, 0, 10, 40})[1:] {
		if w, h := c.X1-c.X0, c.Y1-c.Y0; math.Abs(w-10) > 1e-9 || math.Abs(h-20) > 1e-9 {
			t.Fatalf("cell %v is %v×%v, want 10×20", c, w, h)
		}
	}
}

func TestTidySpreadsSeveralSmallSubtreesEvenly(t *testing.T) {
	// Two leaves between wide subtrees share the slack in equal steps.
	s := []spec{{-1, 0}, {0, 0}}
	for range 4 {
		s = append(s, spec{1, 1})
	}
	s = append(s, spec{0, 1}, spec{0, 1}) // 6, 7: the middle leaves
	s = append(s, spec{0, 0})             // 8
	for range 4 {
		s = append(s, spec{8, 1})
	}
	x := Tidy(tree(s), sameSep)
	if d1, d2, d3 := x[6]-x[1], x[7]-x[6], x[8]-x[7]; math.Abs(d1-d2) >= 1e-9 || math.Abs(d2-d3) >= 1e-9 {
		t.Fatalf("steps %v %v %v: %v", d1, d2, d3, x)
	}
	// And the wide subtrees end up exactly a cousin gap apart, no wider:
	// spreading the middle must not push the outer ones out.
	if unit := x[3] - x[2]; math.Abs(x[9]-x[5]-2*unit) >= 1e-9 {
		t.Fatalf("facing leaves %v apart, want %v: %v", x[9]-x[5], 2*unit, x)
	}
}

// randomTrees are deterministic pseudo-random hierarchies for property
// checks: varied depth, fan-out and leaf values.
func randomTrees(count int) []*Tree {
	g := lcg{7}
	var out []*Tree
	for range count {
		tr := &Tree{}
		tr.Push(-1, 0)
		n := 2 + int(g.next()*40)
		for i := 1; i < n; i++ {
			// Favor recent nodes as parents so some branches run deep.
			p := i - 1 - int(g.next()*g.next()*float64(i))
			tr.Push(p, 0)
		}
		for i := range tr.Len() {
			if len(tr.Children[i]) == 0 {
				tr.Value[i] = 1 + math.Floor(g.next()*20)
			}
		}
		tr.Sum()
		out = append(out, tr)
	}
	return out
}

func TestTidyIsCompactWithoutOverlapOnRandomTrees(t *testing.T) {
	one := func(bool, int) float64 { return 1 }
	for k, tr := range randomTrees(300) {
		x := Tidy(tr, one)
		// With a uniform separation, the leftmost node sits half a unit in.
		unit := 2 * slices.Min(x)
		rows := map[int][]int{}
		for _, v := range tr.preOrder() {
			rows[tr.Depth[v]] = append(rows[tr.Depth[v]], v)
			if c := tr.Children[v]; len(c) > 0 && math.Abs(x[v]-(x[c[0]]+x[c[len(c)-1]])/2) > 1e-9 {
				t.Fatalf("tree %d: node %d not centered over its children", k, v)
			}
			if x[v] < -1e-9 || x[v] > 1+1e-9 {
				t.Fatalf("tree %d: node %d at %v", k, v, x[v])
			}
		}
		for d, row := range rows {
			for i := 1; i < len(row); i++ {
				if x[row[i]]-x[row[i-1]] < unit-1e-9 {
					t.Fatalf("tree %d depth %d: nodes %d and %d are %v apart, unit %v", k, d, row[i-1], row[i], x[row[i]]-x[row[i-1]], unit)
				}
			}
		}
		// Compact: every last child's subtree touches its elder siblings'
		// somewhere, exactly a unit apart; a wider gap is wasted room.
		for p := range tr.Len() {
			kids := tr.Children[p]
			if len(kids) < 2 {
				continue
			}
			rightOf, leftOf := map[int]float64{}, map[int]float64{}
			for i, c := range kids {
				for _, v := range subtree(tr, c) {
					d := tr.Depth[v]
					if i < len(kids)-1 {
						if r, ok := rightOf[d]; !ok || x[v] > r {
							rightOf[d] = x[v]
						}
					} else if l, ok := leftOf[d]; !ok || x[v] < l {
						leftOf[d] = x[v]
					}
				}
			}
			tight := false
			for d, l := range leftOf {
				if r, ok := rightOf[d]; ok && math.Abs(l-r-unit) < 1e-9 {
					tight = true
				}
			}
			if !tight {
				t.Fatalf("tree %d: node %d's last child is pushed further than it needs", k, p)
			}
		}
	}
}

func TestTidyIsCompactWithCousinSeparation(t *testing.T) {
	// The same compactness with cousins two units apart and siblings one,
	// as the tree diagram draws: a contour thread that carries the wrong
	// separation only ever leaves extra room, which no overlap check
	// notices.
	g := lcg{7}
	for range 50 {
		n := 2 + int(g.next()*30)
		s := []spec{{-1, 1}}
		for i := 1; i < n; i++ {
			s = append(s, spec{int(g.next() * float64(i)), 1})
		}
		tr := tree(s)
		sep := func(a, b int) float64 {
			if tr.Parent[a] == tr.Parent[b] {
				return 1
			}
			return 2
		}
		x := Tidy(tr, sameSep)
		// The tightest gap, in separation units, is the layout's unit.
		unit := math.Inf(1)
		for a := range n {
			for b := range n {
				if a != b && tr.Depth[a] == tr.Depth[b] && x[b] > x[a] {
					unit = math.Min(unit, (x[b]-x[a])/sep(a, b))
				}
			}
		}
		if math.IsInf(unit, 1) {
			continue
		}
		for p := range n {
			kids := tr.Children[p]
			if len(kids) < 2 {
				continue
			}
			last := subtree(tr, kids[len(kids)-1])
			closest := math.Inf(1)
			for _, k := range kids[:len(kids)-1] {
				for _, a := range subtree(tr, k) {
					for _, b := range last {
						if tr.Depth[a] == tr.Depth[b] {
							closest = math.Min(closest, (x[b]-x[a])/sep(a, b))
						}
					}
				}
			}
			if math.Abs(closest-unit) >= 1e-9 {
				t.Fatalf("%v: node %d's last child sits %v units from its elders, want %v", s, p, closest, unit)
			}
		}
	}
}

func TestPackNeverOverlapsOnRandomTrees(t *testing.T) {
	for k, tr := range randomTrees(150) {
		tr.SortByValue()
		c := Pack(tr, 400, 300, 2)
		for i := 1; i < tr.Len(); i++ {
			p := c[tr.Parent[i]]
			if math.Hypot(c[i].X-p.X, c[i].Y-p.Y)+c[i].R > p.R+1e-6 {
				t.Fatalf("tree %d: %d pokes out of its parent", k, i)
			}
			for _, j := range tr.Children[tr.Parent[i]] {
				if j < i && math.Hypot(c[i].X-c[j].X, c[i].Y-c[j].Y) < c[i].R+c[j].R-1e-6 {
					t.Fatalf("tree %d: %d overlaps %d", k, i, j)
				}
			}
		}
	}
}

func TestTheEnclosureContainsEveryCircleAndTouchesTheOutermost(t *testing.T) {
	g := lcg{3}
	for k := range 500 {
		n := 1 + int(g.next()*8)
		cs := make([]Circle, n)
		for i := range cs {
			cs[i] = Circle{g.next()*100 - 50, g.next()*100 - 50, g.next() * 30}
		}
		in := slices.Clone(cs)
		e := enclose(in, &lcg{1})
		gapMin := math.Inf(1)
		for _, c := range cs {
			gap := e.R - math.Hypot(c.X-e.X, c.Y-e.Y) - c.R
			if gap < -1e-6 {
				t.Fatalf("set %d: %+v pokes out of %+v", k, c, e)
			}
			gapMin = math.Min(gapMin, gap)
		}
		// Smallest: some circle touches it (a looser one would not).
		if gapMin > 1e-6 {
			t.Fatalf("set %d: enclosure %+v touches nothing", k, e)
		}
	}
}

func TestTidyFollowsThreadsPastAShallowSibling(t *testing.T) {
	// A leaf, then two deep subtrees with a leaf between them: the last
	// subtree's grandchildren find the first one's through the middle
	// leaf's thread, and must sit exactly a cousin gap from them.
	//        0
	//   / /  |  \
	//  1 2   5   6
	//    |       |
	//    3       7
	//   / \     / \
	//  4   8   9   10    (indices in push order)
	s := []spec{{-1, 0}, {0, 1}, {0, 0}, {2, 0}, {3, 1}, {0, 1}, {0, 0}, {6, 0}, {3, 1}, {7, 1}, {7, 1}}
	x := Tidy(tree(s), sameSep)
	unit := x[8] - x[4]
	if math.Abs(x[9]-x[8]-2*unit) >= 1e-9 {
		t.Fatalf("deep cousins %v apart, want exactly %v: %v", x[9]-x[8], 2*unit, x)
	}
	if x[2]-x[1] < unit-1e-9 {
		t.Fatalf("siblings too close: %v", x)
	}
}

func TestPackingManySiblingsNeverOverlaps(t *testing.T) {
	// A wide fan-out runs the front chain all the way around: every new
	// circle must be checked against the whole chain, not just its
	// neighbours.
	s := []spec{{-1, 0}}
	for k := range 60 {
		s = append(s, spec{0, float64(1 + k*37%11)})
	}
	tr := tree(s)
	tr.SortByValue()
	c := Pack(tr, 500, 500, 1)
	for i := 1; i < tr.Len(); i++ {
		for j := 1; j < i; j++ {
			if math.Hypot(c[i].X-c[j].X, c[i].Y-c[j].Y) < c[i].R+c[j].R-1e-6 {
				t.Fatalf("%d overlaps %d", i, j)
			}
		}
	}
}

// subtree lists v and its descendants.
func subtree(t *Tree, v int) []int {
	out := []int{v}
	for i := 0; i < len(out); i++ {
		out = append(out, t.Children[out[i]]...)
	}
	return out
}
