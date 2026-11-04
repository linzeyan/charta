package base

import (
	"math"
	"strconv"
	"strings"
)

// Coord formats a coordinate for SVG and scene dumps: two decimals, trailing
// zeros trimmed.
func Coord(v float64) string {
	r := math.Round(v*100) / 100
	return trimZeros(strconv.FormatFloat(positiveZero(r), 'f', 2, 64))
}

// Value formats a data value for tooltips and tables: every significant
// digit the input had, with thousands grouped.
func Value(v float64) string {
	return group(strconv.FormatFloat(positiveZero(v), 'f', -1, 64))
}

// Approx formats a computed statistic (quartile, mean, bandwidth) for a
// tooltip: three significant digits, never fewer than the integer part,
// thousands grouped. Data values use Value instead, which keeps every digit
// given.
func Approx(v float64) string {
	a := math.Abs(v)
	decimals := 0
	if a != 0 && a < 100 {
		decimals = max(0, min(6, 2-powerFloor(a)))
	}
	// Round half away from zero first, as Coord does: formatting alone
	// rounds exact ties to even (41.25 → "41.2").
	m := Pow10(decimals)
	r := math.Round(v*m) / m
	s := trimZeros(strconv.FormatFloat(positiveZero(r), 'f', decimals, 64))
	if s == "-0" {
		s = "0"
	}
	return group(s)
}

// TickFormat is one label style for a whole axis: in full below 10,000,
// compact (10K, 2.5M) once any tick reaches it, with just enough decimals
// for the step.
type TickFormat struct {
	divisor  float64
	suffix   string
	decimals int
}

func NewTickFormat(ticks []float64) TickFormat {
	m := 0.0
	for _, t := range ticks {
		m = math.Max(m, math.Abs(t))
	}
	divisor, suffix := 1.0, ""
	switch {
	case m >= 1e12:
		divisor, suffix = 1e12, "T"
	case m >= 1e9:
		divisor, suffix = 1e9, "B"
	case m >= 1e6:
		divisor, suffix = 1e6, "M"
	case m >= 1e4:
		divisor, suffix = 1e3, "K"
	}
	step := 0.0
	if len(ticks) >= 2 {
		step = math.Abs(ticks[1] - ticks[0])
	}
	return TickFormat{divisor: divisor, suffix: suffix, decimals: decimalsFor(step / divisor)}
}

// Format: zero is always "0". Full labels keep a fixed number of decimals so
// a column of them aligns; compact ones drop trailing zeros ("1M", not
// "1.0M"), since the suffix already breaks alignment.
func (f TickFormat) Format(v float64) string {
	x := positiveZero(v / f.divisor)
	if x == 0 {
		return "0"
	}
	digits := strconv.FormatFloat(x, 'f', f.decimals, 64)
	if f.suffix != "" {
		digits = trimZeros(digits)
	}
	return group(digits) + f.suffix
}

// decimalsFor is the fewest decimals (at most 6) that show step exactly.
func decimalsFor(step float64) int {
	if math.IsNaN(step) || step <= 0 {
		return 0
	}
	scaled := step
	for d := range 6 {
		if math.Abs(scaled-math.Round(scaled)) <= 1e-9*math.Max(scaled, 1) {
			return d
		}
		scaled *= 10
	}
	return 6
}

func positiveZero(v float64) float64 {
	if v == 0 {
		return 0
	}
	return v
}

func trimZeros(s string) string {
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimSuffix(s, ".")
	}
	return s
}

func group(s string) string {
	sign := ""
	if rest, ok := strings.CutPrefix(s, "-"); ok {
		sign, s = "-", rest
	}
	intPart, frac, hasFrac := strings.Cut(s, ".")
	var b strings.Builder
	b.WriteString(sign)
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if hasFrac {
		b.WriteByte('.')
		b.WriteString(frac)
	}
	return b.String()
}
