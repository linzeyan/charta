package charta

import (
	"fmt"
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func TestDaysFallInTheirWeekdayRowAndWeekColumn(t *testing.T) {
	// 2026-01-01 is a Thursday (row 3); Monday 2026-01-05 opens week 1.
	s := buildAt(t, CalendarChart{Days: []TimeValue{
		{base.ToMS(2026, 1, 1, 9, 0, 0), 1},
		{base.ToMS(2026, 1, 5, 9, 0, 0), 5},
	}}, 900, 300)
	cells := hits(s)
	if cells[0].Hit.Label != "Thu Jan 1, 2026" || cells[1].Hit.Label != "Mon Jan 5, 2026" {
		t.Fatalf("labels %q %q", cells[0].Hit.Label, cells[1].Hit.Label)
	}
	a, b := *cells[0].Hit.Area, *cells[1].Hit.Area
	if !near(b.X-a.X, a.W) || !near(a.Y-b.Y, 3*a.W) {
		t.Fatalf("Thu %+v, next Mon %+v", a, b)
	}
	// Monday is the top row: its cell is level with the "Mon" label.
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Text == "Mon" {
			if !near(tx.Y, b.Y+b.H/2) {
				t.Fatalf("Mon label at %v, Monday's cell centred at %v", tx.Y, b.Y+b.H/2)
			}
		}
	}
	// Every day of the year gets a cell, with data or not.
	all := 0
	for _, m := range s.Marks {
		if _, ok := m.Shape.(scene.RectShape); ok && (m.Fill == base.Grid || m.Hit != nil) {
			all++
		}
	}
	if all != 365 {
		t.Fatalf("%d day cells in 2026, want 365", all)
	}
}

func TestTheOffsetDecidesWhichDayAValueBelongsTo(t *testing.T) {
	late := base.ToMS(2026, 6, 1, 22, 0, 0)
	s := buildAt(t, CalendarChart{UTCOffset: 180, Days: []TimeValue{{late, 1}}}, 900, 300)
	h := hits(s)[0].Hit
	if h.Label != "Tue Jun 2, 2026" {
		t.Fatalf("label %q", h.Label)
	}
	// The cell itself is Jun 2's, not just its label.
	if want := fmt.Sprintf("d%d", base.DaysFromCivil(2026, 6, 2)); h.Key != want {
		t.Fatalf("value sits in cell %s, want %s", h.Key, want)
	}
}

func TestTwoValuesOnOneDayAreRejected(t *testing.T) {
	d := base.ToMS(2026, 1, 1, 0, 0, 0)
	rejects(t, CalendarChart{Days: []TimeValue{{d, 1}, {d + 3_600_000, 2}}})
}

func TestCalendarCellsSpanTheRampAndBlocksStackByYear(t *testing.T) {
	s := buildAt(t, CalendarChart{Days: []TimeValue{
		{base.ToMS(2025, 12, 31, 0, 0, 0), 0},
		{base.ToMS(2026, 1, 1, 0, 0, 0), 10},
	}}, 900, 400)
	c := hits(s)
	if c[0].Fill != base.Seq(0) || c[1].Fill != base.Seq(base.SeqClasses-1) {
		t.Fatalf("low %v high %v", c[0].Fill, c[1].Fill)
	}
	// The second year's block sits below the first.
	if !(c[1].Hit.Area.Y > c[0].Hit.Area.Y) {
		t.Fatalf("2026 block at %v, 2025 at %v", c[1].Hit.Area.Y, c[0].Hit.Area.Y)
	}
}

func TestMoreYearsThanFitAreRejectedRatherThanDrawnOffTheChart(t *testing.T) {
	days := make([]TimeValue, 30)
	for y := range days {
		days[y] = TimeValue{base.ToMS(int64(1990+y), 6, 1, 0, 0, 0), 1}
	}
	c := CalendarChart{Days: days}
	rejects(t, c)
	// The verb agrees with the count: the message is read by people.
	for _, tc := range []struct {
		days []TimeValue
		want string
	}{
		{days[:1], "1 year does not fit in 40px"},
		{days, "30 years do not fit in 40px"},
	} {
		_, err := CalendarChart{Days: tc.days}.Scene(640, 40, base.InterMetrics{})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("got %v, want %q", err, tc.want)
		}
	}
	s := buildAt(t, c, 640, 4000)
	// With room, every block stays on the chart.
	for _, m := range s.Marks {
		if r, ok := m.Shape.(scene.RectShape); ok && r.Rect.Y+r.Rect.H > 4000-pad {
			t.Fatalf("cell %+v runs off the chart", r.Rect)
		}
	}
}
