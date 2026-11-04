package base

import (
	"fmt"
	"math"
	"math/bits"
	"strconv"
)

// Time: civil dates from UTC epoch milliseconds, round tick intervals and
// their labels. Mirrors the Rust crate's base/time.rs.
//
// Inputs are always UTC epoch milliseconds; a chart's UTC offset (minutes)
// shifts only what is displayed, never the data. Everything is integer
// arithmetic (Howard Hinnant's civil_from_days), so both ports land on the
// same dates and ticks bit for bit, with no time zone database.

const (
	Second int64 = 1000
	Minute       = 60 * Second
	Hour         = 60 * Minute
	Day          = 24 * Hour
	Week         = 7 * Day
)

var (
	monthNames   = [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	weekdayNames = [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
)

// MonthName is the short English name of month m (1–12).
func MonthName(m int) string { return monthNames[m-1] }

// WeekdayName is the short English name of weekday w (0 = Monday).
func WeekdayName(w int) string { return weekdayNames[w] }

// Civil is a local calendar time.
type Civil struct {
	Year int64
	// Month is 1–12, Day 1–31.
	Month, Day           int
	Hour, Minute, Second int
	// Weekday is 0 = Monday … 6 = Sunday.
	Weekday int
}

// floorDiv and floorMod are Rust's div_euclid and rem_euclid for a positive
// divisor: Go's / and % truncate toward zero instead.
func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b < 0 {
		q--
	}
	return q
}

func floorMod(a, b int64) int64 {
	r := a % b
	if r < 0 {
		r += b
	}
	return r
}

// DaysFromCivil is the number of days since 1970-01-01 of a proleptic
// Gregorian date.
func DaysFromCivil(y int64, m, d int) int64 {
	if m <= 2 {
		y--
	}
	era := floorDiv(y, 400)
	yoe := y - era*400
	mp := (int64(m) + 9) % 12
	doy := (153*mp+2)/5 + int64(d) - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146_097 + doe - 719_468
}

func civilFromDays(z int64) (int64, int, int) {
	z += 719_468
	era := floorDiv(z, 146_097)
	doe := z - era*146_097
	yoe := (doe - doe/1460 + doe/36_524 - doe/146_096) / 365
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d := int(doy - (153*mp+2)/5 + 1)
	m := int(mp + 3)
	if mp >= 10 {
		m = int(mp - 9)
	}
	y := yoe + era*400
	if m <= 2 {
		y++
	}
	return y, m, d
}

// ToCivil is the local time at ms with the clock offset minutes ahead of
// UTC.
func ToCivil(ms int64, offset int) Civil {
	local := ms + int64(offset)*Minute
	days := floorDiv(local, Day)
	rem := floorMod(local, Day)
	y, m, d := civilFromDays(days)
	return Civil{
		Year:   y,
		Month:  m,
		Day:    d,
		Hour:   int(rem / Hour),
		Minute: int(rem % Hour / Minute),
		Second: int(rem % Minute / Second),
		// 1970-01-01 was a Thursday.
		Weekday: int(floorMod(days+3, 7)),
	}
}

// ToMS is the UTC milliseconds of a local date and time.
func ToMS(y int64, m, d, h, min, offset int) int64 {
	return DaysFromCivil(y, m, d)*Day + int64(h)*Hour + int64(min)*Minute - int64(offset)*Minute
}

// FormatDate is "Jan 5, 2026".
func FormatDate(ms int64, offset int) string {
	c := ToCivil(ms, offset)
	return fmt.Sprintf("%s %d, %d", MonthName(c.Month), c.Day, c.Year)
}

// FormatDatetime is "Jan 5, 2026 14:30", or just the date at local midnight.
func FormatDatetime(ms int64, offset int) string {
	c := ToCivil(ms, offset)
	if c.Hour == 0 && c.Minute == 0 && c.Second == 0 {
		return FormatDate(ms, offset)
	}
	return fmt.Sprintf("%s %02d:%02d", FormatDate(ms, offset), c.Hour, c.Minute)
}

// FormatDuration is a span of time in words: "14 days", "3 h 20 min",
// "45 min".
func FormatDuration(ms int64) string {
	if ms < 0 {
		ms = -ms
	}
	if ms >= Day && ms%Day == 0 {
		d := ms / Day
		if d == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", d)
	}
	h, m := ms/Hour, ms%Hour/Minute
	switch {
	case h == 0:
		return fmt.Sprintf("%d min", m)
	case m == 0:
		return fmt.Sprintf("%d h", h)
	}
	return fmt.Sprintf("%d h %d min", h, m)
}

// IntervalKind is the unit of an Interval.
type IntervalKind int

const (
	// FixedInterval is a fixed number of milliseconds (seconds up to weeks).
	FixedInterval IntervalKind = iota
	MonthInterval
	YearInterval
)

// Interval is the tick spacing a time axis settled on: N milliseconds, N
// months or N years. It also decides the label style.
type Interval struct {
	Kind IntervalKind
	N    int64
}

// fixedIntervals are d3's tick intervals: fixed durations, then calendar
// months and years.
var fixedIntervals = [15]int64{
	Second, 5 * Second, 15 * Second, 30 * Second,
	Minute, 5 * Minute, 15 * Minute, 30 * Minute,
	Hour, 3 * Hour, 6 * Hour, 12 * Hour,
	Day, 2 * Day, Week,
}

// Average lengths used only to choose an interval, not to place ticks.
const (
	monthApprox = 30 * Day
	yearApprox  = 365 * Day
)

func (iv Interval) approx() int64 {
	switch iv.Kind {
	case MonthInterval:
		return iv.N * monthApprox
	case YearInterval:
		return iv.N * yearApprox
	}
	return iv.N
}

// IntervalFor is the interval whose length is closest (by ratio) to a
// count-th of the span, as d3's ticks picks it. Past a year, years step by
// 1, 2 or 5 × 10ⁿ.
func IntervalFor(lo, hi int64, count int) Interval {
	target := (hi - lo) / int64(max(count, 1))
	all := make([]Interval, 0, len(fixedIntervals)+3)
	for _, d := range fixedIntervals {
		all = append(all, Interval{FixedInterval, d})
	}
	all = append(all, Interval{MonthInterval, 1}, Interval{MonthInterval, 3}, Interval{YearInterval, 1})
	if target > yearApprox {
		years := float64(target) / float64(yearApprox)
		ticks := Ticks(0, years*float64(count), count)
		step := 1.0
		if len(ticks) >= 2 {
			step = ticks[1] - ticks[0]
		}
		return Interval{YearInterval, max(roundToInt64(step), 1)}
	}
	i := len(all) - 1
	for k, v := range all {
		if v.approx() >= target {
			i = k
			break
		}
	}
	if i == 0 {
		return all[0]
	}
	a, b := all[i-1], all[i]
	// Compare target/a with b/target without division: target² vs a·b. Both
	// sides overflow int64 (a year squared is ~1e21), so compare the 128-bit
	// products, as the Rust port does with i128. All three are positive
	// here: target exceeds all[0], the smallest interval.
	tHi, tLo := bits.Mul64(uint64(target), uint64(target))
	pHi, pLo := bits.Mul64(uint64(a.approx()), uint64(b.approx()))
	if tHi < pHi || (tHi == pHi && tLo < pLo) {
		return a
	}
	return b
}

// roundToInt64 is Rust's `f.round() as i64`: half away from zero, then
// saturating (NaN becomes 0).
func roundToInt64(f float64) int64 {
	r := math.Round(f)
	switch {
	case math.IsNaN(r):
		return 0
	case r >= 9.223372036854775807e18:
		return 1<<63 - 1
	case r <= -9.223372036854775808e18:
		return -1 << 63
	}
	return int64(r)
}

// TimeTicks are the tick times in [lo, hi] on local round boundaries of iv.
func TimeTicks(lo, hi int64, iv Interval, offset int) []int64 {
	off := int64(offset) * Minute
	var out []int64
	switch iv.Kind {
	case FixedInterval:
		d := iv.N
		// Weeks start on Monday; 1970-01-05 was one.
		anchor := int64(0)
		if d == Week {
			anchor = 4 * Day
		}
		local := lo + off - anchor
		t := floorDiv(local, d)*d + anchor - off
		if t < lo {
			t += d
		}
		for ; t <= hi; t += d {
			out = append(out, t)
		}
	case MonthInterval:
		c := ToCivil(lo, offset)
		idx := c.Year*12 + int64(c.Month) - 1
		idx -= floorMod(idx, iv.N)
		for ; ; idx += iv.N {
			t := ToMS(floorDiv(idx, 12), int(floorMod(idx, 12)+1), 1, 0, 0, offset)
			if t > hi {
				break
			}
			if t >= lo {
				out = append(out, t)
			}
		}
	case YearInterval:
		y := ToCivil(lo, offset).Year
		y -= floorMod(y, iv.N)
		for ; ; y += iv.N {
			t := ToMS(y, 1, 1, 0, 0, offset)
			if t > hi {
				break
			}
			if t >= lo {
				out = append(out, t)
			}
		}
	}
	return out
}

// TimeTickLabel is a tick label: the coarsest unit that changes at that
// tick, so the year shows where it turns over and the date where a day
// starts.
func TimeTickLabel(ms int64, iv Interval, offset int) string {
	c := ToCivil(ms, offset)
	midnight := c.Hour == 0 && c.Minute == 0 && c.Second == 0
	newYear := midnight && c.Month == 1 && c.Day == 1
	year := strconv.FormatInt(c.Year, 10)
	switch {
	case iv.Kind == YearInterval:
		return year
	case iv.Kind == MonthInterval && c.Month == 1:
		return year
	case iv.Kind == MonthInterval:
		return MonthName(c.Month)
	case iv.N >= Day:
		if newYear {
			return year
		}
		return fmt.Sprintf("%s %d", MonthName(c.Month), c.Day)
	case midnight:
		return fmt.Sprintf("%s %d", MonthName(c.Month), c.Day)
	case iv.N < Minute:
		return fmt.Sprintf("%02d:%02d:%02d", c.Hour, c.Minute, c.Second)
	}
	return fmt.Sprintf("%02d:%02d", c.Hour, c.Minute)
}
