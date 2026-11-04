package layout

import (
	"math"
	"slices"
	"testing"

	"github.com/linzeyan/charta/scene"
)

func dist(a, b scene.Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

func TestForceKeepsClustersTogetherAndApart(t *testing.T) {
	// Two triangles joined by one bridge (2–3): the reason to draw a network
	// at all is that this structure shows.
	p := Force(6, [][2]int{{0, 1}, {1, 2}, {2, 0}, {3, 4}, {4, 5}, {5, 3}, {2, 3}})
	within := (dist(p[0], p[1]) + dist(p[3], p[4])) / 2
	across := (dist(p[0], p[4]) + dist(p[1], p[5])) / 2
	if across <= 1.5*within {
		t.Fatalf("within %v, across %v", within, across)
	}
}

func TestForceLandsNoTwoNodesOnEachOther(t *testing.T) {
	// A star: every leaf is pulled to the same hub, only repulsion keeps
	// them apart.
	var e [][2]int
	for i := 1; i < 12; i++ {
		e = append(e, [2]int{0, i})
	}
	p := Force(12, e)
	// d3 scales a link's pull by its less-connected end, so the hub's many
	// links do not weaken each leaf's: every leaf settles alike, near the
	// rest length.
	lo, hi := math.Inf(1), 0.0
	for i := 1; i < 12; i++ {
		lo, hi = math.Min(lo, dist(p[0], p[i])), math.Max(hi, dist(p[0], p[i]))
	}
	if hi-lo > 1 || hi > 40 {
		t.Fatalf("leaves %v..%v from the hub", lo, hi)
	}
	for i := range 12 {
		for j := i + 1; j < 12; j++ {
			if dist(p[i], p[j]) <= 5 {
				t.Fatalf("%d and %d are %v apart", i, j, dist(p[i], p[j]))
			}
		}
	}
}

func TestForceKeepsSeparateComponentsFromDrifting(t *testing.T) {
	p := Force(6, [][2]int{{0, 1}, {2, 3}, {4, 5}})
	var c scene.Point
	for _, q := range p {
		if math.Abs(q.X) >= 150 || math.Abs(q.Y) >= 150 {
			t.Fatalf("%v drifted off", q)
		}
		c.X, c.Y = c.X+q.X/6, c.Y+q.Y/6
	}
	// Repulsion alone would keep pushing the pairs apart; the weak pull to
	// the center holds them in one compact group.
	for _, q := range p {
		if d := dist(q, c); d >= 60 {
			t.Fatalf("%v is %v from the group's center", q, d)
		}
	}
}

func TestForceSettlesLinkedNodesNearTheRestLength(t *testing.T) {
	p := Force(2, [][2]int{{0, 1}})
	if d := dist(p[0], p[1]); d <= 20 || d >= 60 {
		t.Fatalf("%v apart", d)
	}
}

// exactPush sums every pair exactly, as the layout did before Barnes–Hut.
func exactPush(p []scene.Point) []scene.Point {
	v := make([]scene.Point, len(p))
	for i := range p {
		for j := range p {
			if i != j {
				x, y := p[j].X-p[i].X, p[j].Y-p[i].Y
				l := x*x + y*y
				if l < 1 {
					l = math.Sqrt(l)
				}
				v[i].X += x * forceCharge / l
				v[i].Y += y * forceCharge / l
			}
		}
	}
	return v
}

func TestTheTreePushesAboutAsHardAsEveryPair(t *testing.T) {
	// Clustered like a laid-out graph: a few dense blobs and a sparse field,
	// where far cells stand in for many nodes at once.
	rng := lcg{7}
	blobs := [3][3]float64{{0, 0, 40}, {150, 30, 20}, {-90, 120, 10}}
	p := make([]scene.Point, 600)
	for i := range p {
		b := blobs[i%3]
		spread := b[2]
		if i%5 == 0 {
			spread = 300
		}
		x := b[0] + (rng.next()-0.5)*spread
		p[i] = scene.Point{X: x, Y: b[1] + (rng.next()-0.5)*spread}
	}
	want := exactPush(p)
	got := make([]scene.Point, len(p))
	one := lcg{1}
	manyBody(p, got, 1, &one)
	errs := make([]float64, len(p))
	for i := range p {
		errs[i] = math.Hypot(got[i].X-want[i].X, got[i].Y-want[i].Y) / math.Hypot(want[i].X, want[i].Y)
	}
	slices.Sort(errs)
	// θ = 0.9 measured about 1.4% and 6.9%.
	if median, p95 := errs[len(errs)/2], errs[len(errs)*95/100]; median > 0.03 || p95 > 0.12 {
		t.Fatalf("median %v, 95th %v", median, p95)
	}
}
