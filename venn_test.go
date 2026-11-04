package charta

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func vennCircles(s *scene.Scene) []scene.CircleShape {
	var out []scene.CircleShape
	for _, m := range s.Marks {
		if c, ok := m.Shape.(scene.CircleShape); ok {
			out = append(out, c)
		}
	}
	return out
}

// shared is the overlap as a share of the first circle's area, which
// survives the fit's scaling.
func shared(c []scene.CircleShape, i, j int) float64 {
	d := math.Hypot(c[i].CX-c[j].CX, c[i].CY-c[j].CY)
	return lens(c[i].R, c[j].R, d) / (math.Pi * c[i].R * c[i].R)
}

func venn(areas ...VennArea) VennChart { return VennChart{Areas: areas} }

func set(size float64, names ...string) VennArea { return VennArea{Sets: names, Size: size} }

func TestTwoAndThreeSetsMatchEveryPairwiseOverlap(t *testing.T) {
	s := build(t, venn(set(100, "A"), set(60, "B"), set(40, "C"), set(20, "A", "B"), set(10, "A", "C"), set(8, "B", "C")))
	c := vennCircles(s)
	for _, p := range []struct {
		i, j int
		want float64
	}{{0, 1, 0.20}, {0, 2, 0.10}, {1, 2, 8.0 / 60.0}} {
		if got := shared(c, p.i, p.j); math.Abs(got-p.want) > 1e-6 {
			t.Errorf("%d∩%d shares %v, want %v", p.i, p.j, got, p.want)
		}
	}
	// Areas track sizes.
	if r := math.Pow(c[1].R/c[0].R, 2); math.Abs(r-0.6) > 1e-9 {
		t.Errorf("B/A area %v", r)
	}
	// Two sets: the one overlap, exactly.
	two := vennCircles(build(t, venn(set(40, "Cats"), set(55, "Dogs"), set(15, "Cats", "Dogs"))))
	if got := shared(two, 0, 1); math.Abs(got-15.0/40.0) > 1e-6 {
		t.Errorf("two sets share %v", got)
	}
	// A set's tooltip lists the overlaps it is part of.
	if rows := markKeyed(t, s, "set-0").Hit.Rows; len(rows) != 2 || rows[0] != (scene.TipRow{Name: "∩ B", Value: "20"}) {
		t.Errorf("%+v", rows)
	}
}

func TestFourSetsComeCloseAndDisjointOnesStayApart(t *testing.T) {
	s := build(t, venn(set(100, "A"), set(100, "B"), set(100, "C"), set(30, "D"),
		set(30, "A", "B"), set(30, "B", "C"), set(30, "A", "C"), set(10, "A", "D")))
	c := vennCircles(s)
	for _, p := range []struct {
		i, j int
		want float64
	}{{0, 1, 0.3}, {1, 2, 0.3}, {0, 2, 0.3}, {0, 3, 0.1}} {
		if got := shared(c, p.i, p.j); math.Abs(got-p.want) > 0.02 {
			t.Errorf("%d∩%d shares %v, want %v", p.i, p.j, got, p.want)
		}
	}
	// D overlaps nothing but A.
	if shared(c, 1, 3) != 0 || shared(c, 2, 3) != 0 {
		t.Errorf("D touches B or C: %v %v", shared(c, 1, 3), shared(c, 2, 3))
	}
}

func TestNamesAvoidTheOtherCircles(t *testing.T) {
	// Frontend sits near the middle, so "away from the others" points into
	// Design's circle; the name has to find a clear side.
	s := build(t, venn(set(30, "Design"), set(45, "Frontend"), set(50, "Backend"), set(25, "Data"),
		set(12, "Design", "Frontend"), set(15, "Frontend", "Backend"), set(10, "Backend", "Data"), set(4, "Frontend", "Data")))
	c := vennCircles(s)
	var names []scene.TextShape
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Size == tickSize && tx.Weight == base.Semibold {
			names = append(names, tx)
		}
	}
	if len(names) != 4 {
		t.Fatalf("%d names", len(names))
	}
	for _, n := range names {
		for _, ci := range c {
			if math.Hypot(n.X-ci.CX, n.Y-ci.CY) < ci.R {
				t.Errorf("%s at (%v, %v) is inside a circle", n.Text, n.X, n.Y)
			}
		}
	}
}

func TestASubsetSitsInside(t *testing.T) {
	c := vennCircles(build(t, venn(set(100, "All"), set(20, "Some"), set(20, "All", "Some"))))
	if got := shared(c, 1, 0); math.Abs(got-1) > 1e-9 {
		t.Fatalf("Some shares %v of itself with All", got)
	}
	// One set alone is a circle in the middle.
	one := vennCircles(build(t, venn(set(5, "Only"))))
	if len(one) != 1 || !near(one[0].CX, 320) {
		t.Fatalf("%+v", one)
	}
	// With no others to point away from, the name goes above.
	for _, m := range build(t, venn(set(5, "Only"))).Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && (tx.Y >= one[0].CY-one[0].R || tx.Base != scene.Bottom) {
			t.Fatalf("the name sits at %v (baseline %v), the circle tops out at %v", tx.Y, tx.Base, one[0].CY-one[0].R)
		}
	}
}

func TestImpossibleOverlapsAreNamed(t *testing.T) {
	_, err := venn(set(10, "A"), set(5, "B"), set(6, "A", "B")).Scene(640, 400, base.InterMetrics{})
	if err == nil || !strings.Contains(err.Error(), `bigger than "B"`) {
		t.Fatalf("%v", err)
	}
	rejects(t, venn(set(10, "A"), set(1, "A", "Z")))
	rejects(t, venn(set(10, "A"), set(10, "A")))
	rejects(t, venn(set(10, "A"), set(5, "B"), set(1, "A", "B"), set(1, "B", "A")))
	rejects(t, venn(set(10, "A"), set(5, "B"), set(1, "A", "A")))
	rejects(t, venn(set(0, "A")))
	rejects(t, venn(set(10, "A"), set(5, "B"), set(-1, "A", "B")))
	rejects(t, venn(set(1, "A", "B")))
	rejects(t, venn(set(1, "A"), set(1, "B"), set(1, "C"), set(1, "D"), set(1, "E"), set(1, "F")))
}

func TestLensAndDistanceAgree(t *testing.T) {
	// The bisection must land where the lens area equals the overlap asked
	// for, and the two edge cases bracket it.
	r1, r2 := 3.0, 2.0
	for _, o := range []float64{0.5, 3, 8} {
		p := vennDistance(r1, r2, o)
		if p.kind != pairExact || math.Abs(lens(r1, r2, p.d)-o) > 1e-9 {
			t.Errorf("overlap %v: d %v gives %v", o, p.d, lens(r1, r2, p.d))
		}
	}
	if p := vennDistance(r1, r2, 0); p.kind != pairAtLeast || !near(p.d, vennDisjoint*(r1+r2)) {
		t.Errorf("disjoint: %+v", p)
	}
	if p := vennDistance(r1, r2, math.Pi*4); p.kind != pairAtMost || !near(p.d, 0.5) {
		t.Errorf("inside: %+v", p)
	}
	if lens(r1, r2, 5) != 0 || !near(lens(r1, r2, 1), math.Pi*4) {
		t.Errorf("lens at touching %v, inside %v", lens(r1, r2, 5), lens(r1, r2, 1))
	}
}

func TestStressCountsOnlyBrokenConstraints(t *testing.T) {
	// "At least" is satisfied by any larger distance, "at most" by any
	// smaller one; only an exact target is a spring both ways.
	p := [][2]float64{{0, 0}, {5, 0}}
	at := func(k pairKind, d float64) [][]pairTarget {
		return [][]pairTarget{{{}, {d, k}}, {{d, k}, {}}}
	}
	if stress(p, at(pairAtLeast, 3)) != 0 || stress(p, at(pairAtMost, 7)) != 0 {
		t.Fatal("satisfied one-sided targets cost something")
	}
	if stress(p, at(pairAtLeast, 7)) != 4 || stress(p, at(pairAtMost, 3)) != 4 || stress(p, at(pairExact, 3)) != 4 || stress(p, at(pairExact, 7)) != 4 {
		t.Fatal("a target off by 2 must cost 4")
	}
}

func TestNelderMeadFindsAMinimum(t *testing.T) {
	x := nelderMead(func(v []float64) float64 { return (v[0]-3)*(v[0]-3) + (v[1]+1)*(v[1]+1) }, []float64{0, 0}, 1)
	if math.Abs(x[0]-3) > 1e-4 || math.Abs(x[1]+1) > 1e-4 {
		t.Fatalf("%v", x)
	}
}

func TestNamesStayInsideTheChart(t *testing.T) {
	// Side by side in a narrow chart the circles fill the width, so a name
	// pushed straight out sideways would be cut off: it goes above or below.
	for _, w := range []float64{160, 240, 640} {
		s := buildAt(t, venn(set(10, "Alpha"), set(10, "Beta"), set(2, "Alpha", "Beta")), w, 400)
		for _, m := range s.Marks {
			tx, ok := m.Shape.(scene.TextShape)
			if !ok || tx.Size != tickSize {
				continue
			}
			tw := (base.InterMetrics{}).Width(tx.Text, tickSize, base.Semibold)
			x := tx.X
			switch tx.Anchor {
			case scene.Middle:
				x -= tw / 2
			case scene.End:
				x -= tw
			}
			if x < pad-1e-9 || x+tw > w-pad+1e-9 {
				t.Errorf("at width %v %q spans %v..%v", w, tx.Text, x, x+tw)
			}
		}
	}
}

// counted is the area inside every circle, counted on a fine grid.
func counted(c []vennCircle) float64 {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, k := range c {
		x0, y0 = math.Min(x0, k.x-k.r), math.Min(y0, k.y-k.r)
		x1, y1 = math.Max(x1, k.x+k.r), math.Max(y1, k.y+k.r)
	}
	const steps = 1200
	sx, sy := (x1-x0)/steps, (y1-y0)/steps
	hits := 0
	for a := range steps {
		for b := range steps {
			x, y := x0+(float64(a)+0.5)*sx, y0+(float64(b)+0.5)*sy
			in := true
			for _, k := range c {
				in = in && apart(x, y, k) <= k.r
			}
			if in {
				hits++
			}
		}
	}
	return float64(hits) * sx * sy
}

func TestTheCommonAreaOfAnyCirclesMatchesACount(t *testing.T) {
	for _, c := range [][]vennCircle{
		// Two: the lens.
		{{0, 0, 3}, {4, 0, 2}},
		// Three overlapping unevenly.
		{{0, 0, 3}, {3, 0.5, 2.5}, {1.2, 2.4, 2}},
		// Four, one corner region cut by every circle.
		{{0, 0, 3}, {2.5, 0, 3}, {1.2, 2, 2.6}, {1, 0.6, 1.8}},
		// A small circle inside two larger ones.
		{{0, 0, 5}, {1, 0, 5}, {0.5, 0.5, 1}},
		// Pairwise overlapping, nothing common to all three.
		{{0, 0, 2}, {3, 0, 2}, {1.5, 2.8, 2}},
	} {
		want, got := counted(c), commonArea(c)
		if math.Abs(got-want) > 0.005*math.Max(want, 0.05) {
			t.Errorf("%v: %v vs counted %v", c, got, want)
		}
	}
	two := []vennCircle{{0, 0, 3}, {4, 0, 2}}
	if math.Abs(commonArea(two)-lens(3, 2, 4)) > 1e-9 {
		t.Error("two circles' common area is not their lens")
	}
}

func TestAThreeWayOverlapGivenShapesTheLayout(t *testing.T) {
	// Placed by pairs alone, three equal sets each sharing 30% have about
	// 14% in common to all three; the data says 25%.
	pairs := venn(set(100, "A"), set(100, "B"), set(100, "C"), set(30, "A", "B"), set(30, "A", "C"), set(30, "B", "C"))
	all := venn(append(slices.Clone(pairs.Areas), set(25, "A", "B", "C"))...)
	// Squared error of the pairs, and the error of the three-way share.
	fit := func(v VennChart) (float64, float64) {
		c := vennCircles(build(t, v))
		p := 0.0
		for _, ij := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
			p += math.Pow(shared(c, ij[0], ij[1])-0.3, 2)
		}
		vc := make([]vennCircle, len(c))
		for i, k := range c {
			vc[i] = vennCircle{k.CX, k.CY, k.R}
		}
		return p, commonArea(vc)/(math.Pi*c[0].R*c[0].R) - 0.25
	}
	p0, t0 := fit(pairs)
	p1, t1 := fit(all)
	if p0 > 1e-9 || t0 > -0.1 {
		t.Fatalf("pairs alone: %v, %v", p0, t0)
	}
	if math.Abs(t1) >= math.Abs(t0)-0.02 {
		t.Fatalf("%v is no nearer than %v", t1, t0)
	}
	if p1+t1*t1 >= t0*t0 {
		t.Fatalf("worse overall: %v + %v²", p1, t1)
	}
}

func TestOverlapsMustNest(t *testing.T) {
	three := func(more ...VennArea) VennChart {
		return venn(append([]VennArea{set(10, "A"), set(10, "B"), set(10, "C"), set(3, "A", "B"), set(3, "A", "C")}, more...)...)
	}
	errText := func(v VennChart) string {
		_, err := v.Scene(640, 400, base.InterMetrics{})
		if err == nil {
			t.Fatal("accepted")
		}
		return err.Error()
	}
	if e := errText(three(set(3, "B", "C"), set(4, "A", "B", "C"))); !strings.Contains(e, "is bigger than A ∩ B (3)") {
		t.Error(e)
	}
	if e := errText(three(set(1, "A", "B", "C"))); !strings.Contains(e, "B ∩ C is not given") {
		t.Error(e)
	}
	// A three-way overlap of nothing needs no pairs.
	build(t, three(set(0, "A", "B", "C")))
}
