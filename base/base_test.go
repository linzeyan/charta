package base

import (
	"math"
	"slices"
	"testing"
)

func TestNiceDomainStartsAndEndsOnATick(t *testing.T) {
	// An axis that stops short of its last label looks broken.
	for _, c := range []struct {
		a, b float64
		n    int
	}{{0, 87, 5}, {-3.2, 41.7, 8}, {0, 0.37, 6}, {12, 13, 4}} {
		lo, hi := Nice(c.a, c.b, c.n)
		if lo > c.a || hi < c.b {
			t.Fatalf("%v..%v does not cover %v..%v", lo, hi, c.a, c.b)
		}
		ts := Ticks(lo, hi, c.n)
		if ts[0] != lo || ts[len(ts)-1] != hi {
			t.Fatalf("%v..%v: ticks %v", c.a, c.b, ts)
		}
	}
}

func TestSubUnitTicksAreExactDecimals(t *testing.T) {
	// Accumulating 0.1 steps yields 0.30000000000000004.
	if got := Ticks(0, 0.3, 3); !slices.Equal(got, []float64{0, 0.1, 0.2, 0.3}) {
		t.Fatal(got)
	}
	if powerFloor(1000) != 3 || powerFloor(999) != 2 {
		t.Fatal("power of ten boundary")
	}
}

func TestFormat(t *testing.T) {
	cases := []struct{ got, want string }{
		{Coord(12.345), "12.35"},
		{Coord(-0.001), "0"},
		{Value(1234567.5), "1,234,567.5"},
		{Value(-1200), "-1,200"},
		// Full below 10,000, compact once an axis reaches it.
		{NewTickFormat([]float64{0, 2000, 8000}).Format(8000), "8,000"},
		{NewTickFormat([]float64{0, 5000, 10000}).Format(5000), "5K"},
		{NewTickFormat([]float64{0, 2e5, 1.2e6}).Format(1e6), "1M"},
		{NewTickFormat([]float64{0, 2e5, 1.2e6}).Format(0), "0"},
		{NewTickFormat([]float64{0, 0.25, 0.5}).Format(0.5), "0.50"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestApproxKeepsThreeSignificantDigits(t *testing.T) {
	// A quartile printed to 15 digits is noise; one rounded to even on a tie
	// would disagree with the Rust port and with what people expect.
	for _, c := range []struct {
		v    float64
		want string
	}{
		{2.313452, "2.31"}, {0.012345, "0.0123"}, {41.25, "41.3"}, {1234.5, "1,235"},
		{3, "3"}, {-0.0001, "-0.0001"}, {-0.0000001, "0"},
	} {
		if got := Approx(c.v); got != c.want {
			t.Errorf("Approx(%v) = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestTextMeasure(t *testing.T) {
	m := InterMetrics{}
	if w := m.Width("營收", 12, Regular); w != 24 {
		t.Errorf("CJK must be 1em per character, got %v", w)
	}
	if w := m.Width("n", 2048, Regular); w != 1210 {
		t.Errorf("n = %v, want Inter's 1210 units", w)
	}
	if m.Width("Revenue", 12, Semibold) <= m.Width("Revenue", 12, Regular) {
		t.Error("semibold must be wider")
	}
}

func TestBandScaleIsCentered(t *testing.T) {
	b := NewBandScale(4, 100, 500, 0.2, 0.1)
	end := b.Position(3) + b.Bandwidth()
	if math.Abs((b.Position(0)-100)-(500-end)) > 1e-9 {
		t.Fatalf("leftover differs: %v vs %v", b.Position(0)-100, 500-end)
	}
}

func TestDarkModeIsSelectedNotFlipped(t *testing.T) {
	if Resolve(Series(0), false).Hex() != "#2a78d6" || Resolve(Series(0), true).Hex() != "#3987e5" {
		t.Fatal("series-1 must use the palette's selected light/dark steps")
	}
}

func TestEveryRoleResolvesInBothModes(t *testing.T) {
	// A role without a palette row would panic mid-paint in a native backend.
	roles := []Color{Surface, TextPrimary, TextSecondary, TextMuted, Grid, Axis, Border}
	for i := range SeriesSlots {
		roles = append(roles, Series(i), Wash(i), Link(i))
	}
	for i := range SeqClasses {
		roles = append(roles, Seq(i))
	}
	for i := range BandClasses {
		roles = append(roles, Band(i))
	}
	for _, c := range roles {
		Resolve(c, false)
		Resolve(c, true)
	}
	if Wash(0).Token() != "series-1-wash" || Link(7).Token() != "series-8-link" || Seq(6).Token() != "seq-7" || Band(2).Token() != "band-3" {
		t.Fatal("tokens must match the palette file and the Rust port")
	}
}
