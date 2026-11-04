package base

import (
	"slices"
	"testing"
)

func TestCivilRoundTripsAndKnowsLeapYearsAndWeekdays(t *testing.T) {
	// Dates either side of the epoch and across century leap rules: an
	// off-by-one here shifts every label on every time axis.
	for _, d := range []struct {
		y    int64
		m, d int
	}{{1970, 1, 1}, {2000, 2, 29}, {2024, 12, 31}, {1969, 12, 31}, {2100, 3, 1}} {
		c := ToCivil(ToMS(d.y, d.m, d.d, 13, 45, 0), 0)
		if c.Year != d.y || c.Month != d.m || c.Day != d.d || c.Hour != 13 || c.Minute != 45 {
			t.Errorf("%v round-trips to %+v", d, c)
		}
	}
	// 2026-01-05 is a Monday; 2000-02-29 a Tuesday.
	if w := ToCivil(ToMS(2026, 1, 5, 0, 0, 0), 0).Weekday; w != 0 {
		t.Errorf("2026-01-05 weekday %d, want 0", w)
	}
	if w := ToCivil(ToMS(2000, 2, 29, 0, 0, 0), 0).Weekday; w != 1 {
		t.Errorf("2000-02-29 weekday %d, want 1", w)
	}
	if DaysFromCivil(1970, 1, 1) != 0 {
		t.Error("the epoch is day 0")
	}
}

func TestTheOffsetMovesTheDisplayNotTheInstant(t *testing.T) {
	// 23:30 UTC is already the next day at UTC+8.
	ms := ToMS(2026, 3, 1, 23, 30, 0)
	if c := ToCivil(ms, 480); c.Day != 2 || c.Hour != 7 {
		t.Errorf("UTC+8 sees %+v", c)
	}
	if ToMS(2026, 3, 2, 7, 30, 480) != ms {
		t.Error("a local time must map back to the same instant")
	}
}

func TestIntervalsAimForTheRequestedCount(t *testing.T) {
	day0 := ToMS(2026, 1, 1, 0, 0, 0)
	for _, c := range []struct {
		span  int64
		count int
		want  Interval
	}{
		{10 * Day, 10, Interval{FixedInterval, Day}},
		{6 * Hour, 6, Interval{FixedInterval, Hour}},
		{365 * Day, 12, Interval{MonthInterval, 1}},
		{365 * Day, 4, Interval{MonthInterval, 3}},
		{40 * 365 * Day, 5, Interval{YearInterval, 10}},
	} {
		if got := IntervalFor(day0, day0+c.span, c.count); got != c.want {
			t.Errorf("span %d / %d: %+v, want %+v", c.span, c.count, got, c.want)
		}
	}
}

func TestIntervalChoiceComparesRatiosPastInt64(t *testing.T) {
	// Between 3 months and a year the comparison squares ~1e10 ms, past
	// int64: a wrapped product would pick the wrong neighbour. The geometric
	// mean of 90 and 365 days is ~181 days; just either side must split.
	day0 := ToMS(2026, 1, 1, 0, 0, 0)
	if got := IntervalFor(day0, day0+180*Day, 1); got != (Interval{MonthInterval, 3}) {
		t.Errorf("180 days: %+v", got)
	}
	if got := IntervalFor(day0, day0+182*Day, 1); got != (Interval{YearInterval, 1}) {
		t.Errorf("182 days: %+v", got)
	}
}

func TestTicksLandOnLocalBoundaries(t *testing.T) {
	lo, hi := ToMS(2026, 1, 3, 10, 0, 0), ToMS(2026, 4, 20, 0, 0, 0)
	var labels []string
	for _, x := range TimeTicks(lo, hi, Interval{MonthInterval, 1}, 0) {
		labels = append(labels, TimeTickLabel(x, Interval{MonthInterval, 1}, 0))
	}
	if !slices.Equal(labels, []string{"Feb", "Mar", "Apr"}) {
		t.Errorf("month ticks %v", labels)
	}
	// Weeks start on Monday.
	for _, x := range TimeTicks(lo, hi, Interval{FixedInterval, Week}, 0) {
		if ToCivil(x, 0).Weekday != 0 {
			t.Errorf("week tick %d is not a Monday", x)
		}
	}
	// Local midnight at UTC−5 is 05:00 UTC.
	d := TimeTicks(lo, lo+3*Day, Interval{FixedInterval, Day}, -300)
	if len(d) == 0 {
		t.Fatal("no day ticks")
	}
	for _, x := range d {
		if ToCivil(x, -300).Hour != 0 || ToCivil(x, 0).Hour != 5 {
			t.Errorf("day tick %d is not local midnight", x)
		}
	}
	// Years round down to a multiple of the step, before the epoch too.
	y := TimeTicks(ToMS(1937, 6, 1, 0, 0, 0), ToMS(1961, 1, 1, 0, 0, 0), Interval{YearInterval, 10}, 0)
	if !slices.Equal(y, []int64{ToMS(1940, 1, 1, 0, 0, 0), ToMS(1950, 1, 1, 0, 0, 0), ToMS(1960, 1, 1, 0, 0, 0)}) {
		t.Errorf("decade ticks %v", y)
	}
}

func TestLabelsNameTheUnitThatTurnsOver(t *testing.T) {
	ny := ToMS(2027, 1, 1, 0, 0, 0)
	for _, c := range []struct{ got, want string }{
		{TimeTickLabel(ny, Interval{MonthInterval, 1}, 0), "2027"},
		{TimeTickLabel(ny, Interval{FixedInterval, Day}, 0), "2027"},
		{TimeTickLabel(ny+Day, Interval{FixedInterval, Day}, 0), "Jan 2"},
		{TimeTickLabel(ny+6*Hour, Interval{FixedInterval, 6 * Hour}, 0), "06:00"},
		{TimeTickLabel(ny+Day, Interval{FixedInterval, 6 * Hour}, 0), "Jan 2"},
		{TimeTickLabel(ny+15*Second, Interval{FixedInterval, 15 * Second}, 0), "00:00:15"},
		{FormatDatetime(ny+14*Hour+30*Minute, 0), "Jan 1, 2027 14:30"},
		{FormatDatetime(ny, 0), "Jan 1, 2027"},
		{FormatDuration(14 * Day), "14 days"},
		{FormatDuration(Day), "1 day"},
		{FormatDuration(3*Hour + 20*Minute), "3 h 20 min"},
		{FormatDuration(45 * Minute), "45 min"},
		{FormatDuration(2 * Hour), "2 h"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}
