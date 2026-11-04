package dmath

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// inputs are the values the Rust twin evaluates too: a fixed LCG over the
// ranges charts use, plus edge values. Both ports write their results for
// `make test-conformance` to diff.
func inputs() []float64 {
	v := []float64{
		0, math.Copysign(0, -1), 1, -1, 0.5, -0.5,
		0x1p-1022, 5e-324, 1e-300,
		math.Pi, math.Pi / 2, math.Pi / 4,
		1e6, -1e6, 700, -740,
	}
	s := uint64(0x2545f4914f6cdd1d)
	next := func() float64 {
		s = s*6364136223846793005 + 1442695040888963407
		return float64(s>>11) / float64(uint64(1)<<53)
	}
	for _, r := range [][2]float64{{-1, 1}, {-7, 7}, {-30, 30}, {-800, 800}, {0, 1e6}} {
		for range 4000 {
			u := next()
			v = append(v, r[0]+float64(u*(r[1]-r[0])))
		}
	}
	return v
}

func ulps(a, b float64) uint64 {
	if a == b || (math.IsNaN(a) && math.IsNaN(b)) {
		return 0
	}
	k := func(x float64) int64 {
		i := int64(math.Float64bits(x))
		if i < 0 {
			return math.MinInt64 - i
		}
		return i
	}
	d := k(a) - k(b)
	if d < 0 {
		d = -d
	}
	return uint64(d)
}

func TestCloseToGoMath(t *testing.T) {
	// Go's math is no ulp-level reference: Sin(π) is off in the 15th digit
	// and Asin is atan(x/sqrt(1-x²)), worse near ±1. The Rust test holds
	// these functions to an ulp of a real libm and the conformance diff
	// proves this port bit-identical, so here a loose bound only catches a
	// gross slip.
	near := func(a, b float64) bool {
		return a == b || math.Abs(a-b) <= 1e-13*math.Max(1, math.Abs(b))
	}
	for _, x := range inputs() {
		if math.Abs(x) <= 1e5 {
			for _, c := range []struct {
				name     string
				got, std float64
			}{
				{"sin", Sin(x), math.Sin(x)},
				{"cos", Cos(x), math.Cos(x)},
				{"atan", Atan(x), math.Atan(x)},
				{"exp", Exp(x), math.Exp(x)},
			} {
				if !near(c.got, c.std) {
					t.Errorf("%s(%v) = %v, math says %v", c.name, x, c.got, c.std)
				}
			}
		}
		if math.Abs(x) <= 1 && (!near(Asin(x), math.Asin(x)) || !near(Acos(x), math.Acos(x))) {
			t.Errorf("asin/acos(%v) = %v/%v, math says %v/%v", x, Asin(x), Acos(x), math.Asin(x), math.Acos(x))
		}
		if x > 0 {
			// Go's amd64 assembly Log misreads subnormals (log(5e-324) gives
			// -709.09, the log of the smallest normal): scale them into the
			// normal range, where it is right on every arch.
			want := math.Log(x)
			if x < 0x1p-1022 {
				want = math.Log(x*0x1p52) - 52*math.Ln2
			}
			if ulps(Log(x), want) > 2 {
				t.Errorf("log(%v) = %v, math says %v", x, Log(x), want)
			}
		}
	}
}

func TestEdgeValues(t *testing.T) {
	if !math.IsNaN(Sin(math.Inf(1))) || !math.IsNaN(Acos(1.5)) || !math.IsNaN(Log(-1)) {
		t.Error("domain errors are not NaN")
	}
	if Acos(-1) != math.Pi || Asin(1) != math.Pi/2 || Atan2(0, -1) != math.Pi || Atan2(-1, 0) != -math.Pi/2 {
		t.Error("exact angles")
	}
	if Exp(0) != 1 || Exp(-1000) != 0 || !math.IsInf(Exp(1000), 1) || Log(1) != 0 || !math.IsInf(Log(0), -1) {
		t.Error("exp/log limits")
	}
}

func TestWritesResultsForTheCrossLanguageDiff(t *testing.T) {
	var b strings.Builder
	for _, x := range inputs() {
		y := float64(x*0.37) - 1
		s, c := Sincos(x)
		row := []float64{s, c, Tan(x), Asin(x), Acos(x), Atan(x), Atan2(y, x), Exp(x), Log(x), Pow(math.Abs(x)+1, -0.2)}
		fmt.Fprintf(&b, "%016x", math.Float64bits(x))
		for _, r := range row {
			// NaN payloads differ between the languages' constants; any NaN
			// is the same answer.
			if math.IsNaN(r) {
				b.WriteString(" nan")
			} else {
				fmt.Fprintf(&b, " %016x", math.Float64bits(r))
			}
		}
		b.WriteByte('\n')
	}
	dir := filepath.Join("..", "..", "build", "conformance")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dmath-go.txt"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
