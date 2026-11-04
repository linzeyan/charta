package layout

import "sort"

// The stream graph's stack: d3-shape's stackOrderInsideOut and
// stackOffsetWiggle (Byron & Wattenberg 2008; ISC license, Mike Bostock).

// InsideOut orders layers with the earliest-peaking series in the middle and
// later ones alternating outward, balancing the total weight on each side.
// The result lists series bottom to top.
func InsideOut(series [][]float64) []int {
	peak := func(v []float64) int {
		best := 0
		for j, x := range v {
			if x > v[best] {
				best = j
			}
		}
		return best
	}
	byPeak := make([]int, len(series))
	for i := range byPeak {
		byPeak[i] = i
	}
	// Stable, like Rust's sort_by_key: equal peaks keep series order.
	sort.SliceStable(byPeak, func(a, b int) bool { return peak(series[byPeak[a]]) < peak(series[byPeak[b]]) })
	top, bottom := 0.0, 0.0
	var tops, bottoms []int
	for _, i := range byPeak {
		sum := 0.0
		for _, v := range series[i] {
			sum += v
		}
		if top < bottom {
			top += sum
			tops = append(tops, i)
		} else {
			bottom += sum
			bottoms = append(bottoms, i)
		}
	}
	out := make([]int, 0, len(series))
	for k := len(bottoms) - 1; k >= 0; k-- {
		out = append(out, bottoms[k])
	}
	return append(out, tops...)
}

// Edge is a layer's lower and upper y at one x.
type Edge struct{ Lo, Hi float64 }

// Wiggle stacks the series in order on a baseline that minimizes the
// layers' weighted wiggle (slope). Indexed by series, then x.
func Wiggle(series [][]float64, order []int) [][]Edge {
	m := 0
	if len(series) > 0 {
		m = len(series[0])
	}
	base := make([]float64, m)
	y := 0.0
	for j := 1; j < m; j++ {
		s1, s2 := 0.0, 0.0
		for i, si := range order {
			v := series[si][j]
			s3 := (v - series[si][j-1]) / 2
			for _, sk := range order[:i] {
				s3 += series[sk][j] - series[sk][j-1]
			}
			s1 += v
			s2 += float64(s3 * v)
		}
		if s1 != 0 {
			y -= s2 / s1
		}
		base[j] = y
	}
	out := make([][]Edge, len(series))
	for i := range out {
		out[i] = make([]Edge, 0, m)
	}
	for j, b := range base {
		lo := b
		for _, si := range order {
			hi := lo + series[si][j]
			out[si] = append(out[si], Edge{lo, hi})
			lo = hi
		}
	}
	return out
}
