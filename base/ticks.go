package base

import "math"

// Tick generation and domain nicing, ported from d3-array ticks and d3-scale
// linear.nice (ISC license, Mike Bostock), with powers of ten from a literal
// table instead of log10/pow (see the package doc).

const pow10Min = -20

var pow10Table = [41]float64{
	1e-20, 1e-19, 1e-18, 1e-17, 1e-16, 1e-15, 1e-14, 1e-13, 1e-12, 1e-11, 1e-10, 1e-9, 1e-8, 1e-7,
	1e-6, 1e-5, 1e-4, 1e-3, 1e-2, 1e-1, 1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10,
	1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19, 1e20,
}

// Pow10 is 10^p from the literal table, clamped to ±20: exact, and the
// same double the Rust port's table holds (stem-and-leaf units rely on it).
func Pow10(p int) float64 {
	p = max(pow10Min, min(-pow10Min, p))
	return pow10Table[p-pow10Min]
}

// powerFloor is the largest p with 10^p <= x, clamped to the table.
func powerFloor(x float64) int {
	p := pow10Min
	for p < -pow10Min && Pow10(p+1) <= x {
		p++
	}
	return p
}

// tickSpec returns (i1, i2, inc): ticks are i*inc for i in [i1, i2], or
// i/-inc when inc is negative (d3's trick to keep sub-1 steps exact).
func tickSpec(start, stop, count float64) (float64, float64, float64) {
	step := (stop - start) / math.Max(count, 0)
	power := powerFloor(step)
	e := step / Pow10(power)
	factor := 1.0
	switch {
	case e >= math.Sqrt(50):
		factor = 10
	case e >= math.Sqrt(10):
		factor = 5
	case e >= math.Sqrt(2):
		factor = 2
	}
	var i1, i2, inc float64
	if power < 0 {
		inv := Pow10(-power) / factor
		i1 = math.Round(start * inv)
		i2 = math.Round(stop * inv)
		if i1/inv < start {
			i1++
		}
		if i2/inv > stop {
			i2--
		}
		inc = -inv
	} else {
		s := Pow10(power) * factor
		i1 = math.Round(start / s)
		i2 = math.Round(stop / s)
		if i1*s < start {
			i1++
		}
		if i2*s > stop {
			i2--
		}
		inc = s
	}
	if i2 < i1 && count >= 0.5 && count < 2 {
		return tickSpec(start, stop, count*2)
	}
	return i1, i2, inc
}

// Ticks returns about count round-numbered ticks (multiples of 1, 2 or 5 ×
// 10ⁿ) inside [start, stop]. Requires start <= stop.
func Ticks(start, stop float64, count int) []float64 {
	if count == 0 {
		return nil
	}
	if start == stop {
		return []float64{start}
	}
	i1, i2, inc := tickSpec(start, stop, float64(count))
	if math.IsNaN(i1) || math.IsNaN(i2) || i2 < i1 {
		return nil
	}
	n := int(i2 - i1 + 1)
	out := make([]float64, n)
	for i := range out {
		k := i1 + float64(i)
		if inc < 0 {
			out[i] = k / -inc
		} else {
			out[i] = k * inc
		}
	}
	return out
}

// Nice extends [start, stop] outward to the tick step, so the axis starts
// and ends on a labelled tick. Requires start <= stop.
func Nice(start, stop float64, count int) (float64, float64) {
	var prestep float64
	havePrestep := false
	for range 10 {
		_, _, step := tickSpec(start, stop, float64(count))
		if havePrestep && step == prestep {
			break
		}
		switch {
		case step > 0:
			start = math.Floor(start/step) * step
			stop = math.Ceil(stop/step) * step
		case step < 0:
			start = math.Ceil(start*step) / step
			stop = math.Floor(stop*step) / step
		default:
			return start, stop
		}
		prestep, havePrestep = step, true
	}
	return start, stop
}
