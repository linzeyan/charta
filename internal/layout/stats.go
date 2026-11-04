// Package layout holds data → geometry algorithms shared by charts. It
// mirrors the Rust crate's private layout module, file for file; keep the
// floating-point operation order identical, products wrapped in float64()
// so they are not fused into FMA.
package layout

import (
	"math"
	"math/bits"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/dmath"
)

// Summary statistics, kernel density estimation and binning. Every function
// takes values already sorted ascending, so a chart sorts once and reuses
// the order for quantiles, extents and the KDE.

// Quantile is the p-quantile (0..1) by linear interpolation between order
// statistics (R type 7, d3's quantileSorted). Empty input gives NaN.
func Quantile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return math.NaN()
	}
	if n == 1 || p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[n-1]
	}
	h := float64(float64(n-1) * p)
	i := int(math.Floor(h))
	lo, hi := sorted[i], sorted[min(i+1, n-1)]
	return lo + float64((h-float64(i))*(hi-lo))
}

func Mean(v []float64) float64 {
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}

// StdDev is the sample standard deviation (n − 1); zero for fewer than two
// values.
func StdDev(v []float64) float64 {
	if len(v) < 2 {
		return 0
	}
	m := Mean(v)
	ss := 0.0
	for _, x := range v {
		ss += float64((x - m) * (x - m))
	}
	return math.Sqrt(ss / float64(len(v)-1))
}

// BoxStats is Tukey's five-number summary with 1.5 × IQR fences: whiskers
// end at the most extreme values inside the fences; the rest are outliers.
type BoxStats struct {
	Q1, Median, Q3 float64
	Low, High      float64
	Outliers       []float64
}

func Box(sorted []float64) BoxStats {
	q1, median, q3 := Quantile(sorted, 0.25), Quantile(sorted, 0.5), Quantile(sorted, 0.75)
	reach := 1.5 * (q3 - q1)
	loFence, hiFence := q1-reach, q3+reach
	b := BoxStats{Q1: q1, Median: median, Q3: q3, Low: math.Inf(1), High: math.Inf(-1)}
	for _, v := range sorted {
		if v >= loFence && v <= hiFence {
			b.Low, b.High = math.Min(b.Low, v), math.Max(b.High, v)
		} else {
			b.Outliers = append(b.Outliers, v)
		}
	}
	return b
}

// Silverman is Silverman's rule-of-thumb bandwidth,
// 0.9 · min(σ, IQR / 1.34) · n^(−1/5). Falls back to σ when the IQR is zero,
// and to 1 when every value is the same, so a degenerate sample still draws
// a bump instead of dividing by 0.
func Silverman(sorted []float64) float64 {
	sd := StdDev(sorted)
	iqr := (Quantile(sorted, 0.75) - Quantile(sorted, 0.25)) / 1.34
	spread := sd
	if iqr > 0 {
		spread = math.Min(sd, iqr)
	}
	h := 0.9 * spread * dmath.Pow(float64(len(sorted)), -0.2)
	if h > 0 {
		return h
	}
	return 1
}

// invSqrt2Pi is 1 / √(2π), the same literal as the Rust port's.
const invSqrt2Pi = 0.3989422804014327

// KDE is the Gaussian kernel density of sorted at each of xs, bandwidth h.
func KDE(sorted []float64, h float64, xs []float64) []float64 {
	scale := invSqrt2Pi / float64(float64(len(sorted))*h)
	out := make([]float64, len(xs))
	for i, x := range xs {
		sum := 0.0
		for _, v := range sorted {
			u := (x - v) / h
			sum += dmath.Exp(float64(-0.5*u) * u)
		}
		out[i] = sum * scale
	}
	return out
}

// Grid is n evenly spaced points from lo to hi inclusive.
func Grid(lo, hi float64, n int) []float64 {
	if n < 2 {
		return []float64{lo}
	}
	out := make([]float64, n)
	for i := range out {
		out[i] = lo + float64((hi-lo)*float64(i)/float64(n-1))
	}
	return out
}

// Sturges is Sturges' bin count, ⌈log₂ n⌉ + 1, computed on integers so both
// ports agree exactly.
func Sturges(n int) int {
	if n <= 1 {
		return 1
	}
	return bits.Len(uint(n-1)) + 1
}

// Bins are histogram bins on round edges: about count bins whose edges are
// the ticks of the niced extent (d3's bin with nice). The last bin includes
// its upper edge so the maximum is counted. Identical values get one bin one
// unit wide.
func Bins(sorted []float64, count int) (edges []float64, counts []int) {
	if len(sorted) == 0 {
		return nil, nil
	}
	lo, hi := sorted[0], sorted[len(sorted)-1]
	if lo == hi {
		edges = []float64{lo, lo + 1}
	} else {
		a, b := base.Nice(lo, hi, count)
		edges = base.Ticks(a, b, count)
	}
	counts = make([]int, len(edges)-1)
	last := len(counts) - 1
	i := 0
	for _, v := range sorted {
		for i < last && v >= edges[i+1] {
			i++
		}
		counts[i]++
	}
	return edges, counts
}
