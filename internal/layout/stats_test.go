package layout

import (
	"math"
	"slices"
	"testing"
)

func TestQuantilesInterpolateBetweenOrderStatistics(t *testing.T) {
	// R type 7 is what d3, NumPy and R default to; another rule would put
	// box edges where readers of those tools do not expect them.
	v := []float64{1, 2, 3, 4}
	for _, c := range []struct{ p, want float64 }{{0.5, 2.5}, {0.25, 1.75}, {0, 1}, {1, 4}} {
		if got := Quantile(v, c.p); got != c.want {
			t.Errorf("Quantile(%v) = %v, want %v", c.p, got, c.want)
		}
	}
	// Interpolation runs between the two order statistics around the rank,
	// which only shows on data that is not evenly spaced.
	if got := Quantile([]float64{1, 2, 4, 8}, 0.5); got != 3 {
		t.Errorf("median of 1 2 4 8 = %v, want 3", got)
	}
	if !math.IsNaN(Quantile(nil, 0.5)) {
		t.Error("an empty sample has no quantile")
	}
}

func TestWhiskersStopAtTheLastValueInsideTheFences(t *testing.T) {
	// Q1 = 3, Q3 = 7, IQR 4: the upper fence is 13, so 40 is an outlier and
	// the whisker ends at 8, the last value inside, not at the fence.
	b := Box([]float64{1, 2, 3, 4, 5, 6, 7, 8, 40})
	if b.Q1 != 3 || b.Median != 5 || b.Q3 != 7 {
		t.Fatalf("quartiles %v %v %v", b.Q1, b.Median, b.Q3)
	}
	if b.Low != 1 || b.High != 8 {
		t.Fatalf("whiskers %v..%v", b.Low, b.High)
	}
	if !slices.Equal(b.Outliers, []float64{40}) {
		t.Fatalf("outliers %v", b.Outliers)
	}
}

func TestKDEIntegratesToOne(t *testing.T) {
	// A density is a probability: the area under it must be ~1, or the
	// curves of two samples of different size are not comparable.
	v := []float64{1, 2, 2.5, 3, 7, 8, 8.5}
	h := Silverman(v)
	xs := Grid(v[0]-4*h, v[6]+4*h, 2001)
	ys := KDE(v, h, xs)
	dx := xs[1] - xs[0]
	area := 0.0
	for k := 1; k < len(ys); k++ {
		area += (ys[k-1] + ys[k]) / 2 * dx
	}
	if math.Abs(area-1) >= 1e-3 {
		t.Fatalf("area %v", area)
	}
}

func TestDegenerateSamplesStillGetABandwidth(t *testing.T) {
	// A zero bandwidth would divide by zero and draw nothing.
	if h := Silverman([]float64{5, 5, 5}); h != 1 {
		t.Fatalf("constant sample bandwidth %v, want 1", h)
	}
	if h := Silverman([]float64{1, 1, 1, 9}); !(h > 0) {
		t.Fatalf("zero-IQR sample bandwidth %v", h)
	}
}

func TestBinsCountEveryValueOnceOnRoundEdges(t *testing.T) {
	var v []float64
	for i := range 37 {
		v = append(v, float64(i*i%53)+0.5)
	}
	slices.Sort(v)
	edges, counts := Bins(v, Sturges(len(v)))
	total := 0
	for _, c := range counts {
		total += c
	}
	if total != len(v) {
		t.Fatalf("counted %d of %d values", total, len(v))
	}
	if edges[0] > v[0] || edges[len(edges)-1] < v[len(v)-1] {
		t.Fatalf("edges %v miss the data %v..%v", edges, v[0], v[len(v)-1])
	}
	if step := edges[1] - edges[0]; !slices.Contains([]float64{1, 2, 5, 10, 20}, step) {
		t.Fatalf("step %v is not round", step)
	}
	// The maximum lands in the last bin even when it sits on the edge.
	if _, c := Bins([]float64{0, 5, 10}, 2); !slices.Equal(c, []int{1, 2}) {
		t.Fatalf("counts %v, want [1 2]", c)
	}
}

func TestSturgesMatchesTheFormula(t *testing.T) {
	for _, c := range []struct{ n, want int }{{1, 1}, {2, 2}, {8, 4}, {9, 5}, {100, 8}} {
		if got := Sturges(c.n); got != c.want {
			t.Errorf("Sturges(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}
