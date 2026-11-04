package charta

import (
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	calendarCellMax    = 16.0
	calendarCellRadius = 2.0
	// calendarWeeks is the weeks a year can touch (53 full, plus a partial
	// one).
	calendarWeeks    = 54.0
	calendarYearSize = 12.0
)

// CalendarChart draws a year per block, a column per week and a row per
// weekday (Monday on top): one cell per day, colored by its value on a
// sequential ramp. Each day's Time is any instant in that (local) day.
// UTCOffset is the minutes the displayed clock is ahead of UTC; it decides
// which day an instant belongs to.
type CalendarChart struct {
	Title     string      `json:"title,omitempty"`
	UTCOffset int         `json:"utcOffset,omitempty"`
	Days      []TimeValue `json:"days"`
}

// calendarDay is a value keyed by its local day number.
type calendarDay struct {
	day   int64
	value float64
	time  int64
}

func (c CalendarChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	offset := c.UTCOffset
	byDay := make([]calendarDay, 0, len(c.Days))
	for i, d := range c.Days {
		if err := finite(fmt.Sprintf("day %d", i), d.Value); err != nil {
			return nil, err
		}
		cv := base.ToCivil(d.Time, offset)
		byDay = append(byDay, calendarDay{base.DaysFromCivil(cv.Year, cv.Month, cv.Day), d.Value, d.Time})
	}
	sort.SliceStable(byDay, func(a, b int) bool { return byDay[a].day < byDay[b].day })
	for k := 1; k < len(byDay); k++ {
		if byDay[k-1].day == byDay[k].day {
			return nil, invalid("two values for %s", base.FormatDate(byDay[k].time, offset))
		}
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, e := range byDay {
		lo, hi = math.Min(lo, e.value), math.Max(hi, e.value)
	}
	var years []int64
	if n := len(byDay); n > 0 {
		year := func(d int64) int64 { return base.ToCivil(d*base.Day, 0).Year }
		for y := year(byDay[0].day); y <= year(byDay[n-1].day); y++ {
			years = append(years, y)
		}
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	if isFinite(lo) {
		top = seqLegend(&marks, m, top, lo, hi)
	}
	wdW := maxWidth(m, []string{"Mon", "Wed", "Fri"})
	yearW := 0.0
	for _, y := range years {
		yearW = math.Max(yearW, m.Width(strconv.FormatInt(y, 10), calendarYearSize, base.Semibold))
	}
	left := pad + math.Max(wdW, yearW) + gap
	const header = tickSize + gap
	const blockGap = 2 * gap
	n := float64(max(len(years), 1))
	avail := height - pad - top
	tall := (avail - float64(n*(header+blockGap)) + blockGap) / float64(7*n)
	// Past this, year blocks would run off the bottom of the chart.
	if tall < 1 && len(years) > 0 {
		return nil, invalid("%s not fit in %spx: show fewer years or a taller chart",
			count(len(years), "year does", "years do"), strconv.FormatFloat(height, 'f', -1, 64))
	}
	cell := math.Max(1, math.Min(calendarCellMax, math.Min((width-pad-left)/calendarWeeks, tall)))
	const half = surfaceGap / 2

	idx := 0
	for yi, year := range years {
		blockTop := top + float64(float64(yi)*(header+float64(7*cell)+blockGap))
		gridTop := blockTop + header
		jan1 := base.DaysFromCivil(year, 1, 1)
		// floorMod: (jan1 + 3) is negative before 1969-12-29.
		firstMonday := jan1 - (((jan1+3)%7)+7)%7
		end := base.DaysFromCivil(year+1, 1, 1)
		marks = append(marks, text(pad, blockTop, strconv.FormatInt(year, 10), calendarYearSize, base.Semibold, scene.Start, scene.Top, base.TextSecondary))
		for _, wd := range []struct {
			row  int
			name string
		}{{0, "Mon"}, {2, "Wed"}, {4, "Fri"}} {
			marks = append(marks, text(left-gap, gridTop+float64((float64(wd.row)+0.5)*cell), wd.name, tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
		}
		for mo := 1; mo <= 12; mo++ {
			d := base.DaysFromCivil(year, mo, 1)
			marks = append(marks, text(left+float64(float64((d-firstMonday)/7)*cell), blockTop+(calendarYearSize-tickSize),
				base.MonthName(mo), tickSize, base.Regular, scene.Start, scene.Top, base.TextMuted))
		}
		for d := jan1; d < end; d++ {
			col, row := (d-firstMonday)/7, (d-firstMonday)%7
			slot := scene.Rect{X: left + float64(float64(col)*cell), Y: gridTop + float64(float64(row)*cell), W: cell, H: cell}
			shape := scene.RectShape{
				Rect: scene.Rect{
					X: slot.X + half,
					Y: slot.Y + half,
					W: math.Max(cell-surfaceGap, 0.5),
					H: math.Max(cell-surfaceGap, 0.5),
				},
				Radius: scene.AllCorners(math.Min(calendarCellRadius, cell/4)),
			}
			for idx < len(byDay) && byDay[idx].day < d {
				idx++
			}
			mk := scene.Mark{Shape: shape, Fill: base.Grid}
			if idx < len(byDay) && byDay[idx].day == d {
				e := byDay[idx]
				cv := base.ToCivil(e.time, offset)
				mk.Fill = base.Seq(seqClass(e.value, lo, hi))
				mk.Hit = &scene.Hit{
					Key:   fmt.Sprintf("d%d", d),
					Label: base.WeekdayName(cv.Weekday) + " " + base.FormatDate(e.time, offset),
					Value: base.Value(e.value),
					Area:  &slot,
				}
			}
			marks = append(marks, mk)
		}
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Calendar heatmap of %s across %s.",
			count(len(byDay), "day", "days"), count(len(years), "year", "years"))),
		Marks: marks,
	}, nil
}
