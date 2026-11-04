package charta

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	minutesPerDay      = 24 * 60
	timetableAccent    = 3.0
	timetableRadius    = 4.0
	timetableLabelSize = 11.0
	// timetableHourMin: hour rows closer than this label every other hour.
	timetableHourMin = 20.0
)

// Slot is one booking: Start and End in minutes after midnight on Day. An
// empty Group is none.
type Slot struct {
	Day   string `json:"day"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Label string `json:"label"`
	Group string `json:"group,omitempty"`
}

// TimetableChart is a weekly schedule: a column per day, time of day
// running down, each booking a block; overlapping bookings share their
// column side by side.
type TimetableChart struct {
	Title string   `json:"title,omitempty"`
	Days  []string `json:"days"`
	Slots []Slot   `json:"slots"`
}

func clock(minutes int) string { return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60) }

// placedSlot is a slot's day index and minutes.
type placedSlot struct{ day, start, end int }

// timetableLanes is the lane per slot (index into slots) and the lanes used
// per day: each slot takes the first lane free at its start, in start order.
func timetableLanes(slots []placedSlot, days int) ([]int, []int) {
	lane := make([]int, len(slots))
	used := make([]int, days)
	order := make([]int, len(slots))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		x, y := slots[order[a]], slots[order[b]]
		if x.day != y.day {
			return x.day < y.day
		}
		if x.start != y.start {
			return x.start < y.start
		}
		return x.end < y.end
	})
	ends := make([][]int, days)
	for _, i := range order {
		s := slots[i]
		k := len(ends[s.day])
		for j, e := range ends[s.day] {
			if e <= s.start {
				k = j
				break
			}
		}
		if k == len(ends[s.day]) {
			ends[s.day] = append(ends[s.day], s.end)
		} else {
			ends[s.day][k] = s.end
		}
		lane[i] = k
		used[s.day] = max(used[s.day], k+1)
	}
	return lane, used
}

func (c TimetableChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	var groups []string
	placed := make([]placedSlot, 0, len(c.Slots))
	for i, s := range c.Slots {
		d := slices.Index(c.Days, s.Day)
		if d < 0 {
			return nil, invalid("slot %d (%q) is on unknown day %q", i, s.Label, s.Day)
		}
		// Negative minutes cannot reach the Rust port (u32); reject them
		// with the same message.
		if s.Start < 0 || s.Start >= s.End || s.End > minutesPerDay {
			return nil, invalid("slot %d (%q) must start before it ends, within the day", i, s.Label)
		}
		if s.Group != "" && !slices.Contains(groups, s.Group) {
			groups = append(groups, s.Group)
		}
		placed = append(placed, placedSlot{d, s.Start, s.End})
	}
	if len(groups) > maxSeries {
		return nil, invalid("%d groups, at most %d: merge some", len(groups), maxSeries)
	}
	first, last := math.MaxUint32, 0
	for _, s := range c.Slots {
		first, last = min(first, s.Start), max(last, s.End)
	}
	h0, h1 := 8, 18
	if first < last {
		h0, h1 = first/60, (last+59)/60
	}
	lane, used := timetableLanes(placed, len(c.Days))

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, groups, keySwatch)
	header := top
	gridTop := top + tickSize + gap + tickSize/2
	bottom := math.Max(height-pad-tickSize/2, gridTop+1)
	left := pad + m.Width("00:00", tickSize, base.Regular) + gap
	right := math.Max(width-pad, left+1)
	y := base.LinearScale{D0: float64(h0 * 60), D1: float64(h1 * 60), R0: gridTop, R1: bottom}
	cols := base.NewBandScale(len(c.Days), left, right, 0.08, 0)
	hourPx := (bottom - gridTop) / float64(max(h1-h0, 1))
	every := 1
	if hourPx < timetableHourMin {
		every = 2
	}
	for h := h0; h <= h1; h++ {
		if (h-h0)%every != 0 {
			continue
		}
		py := y.Map(float64(h * 60))
		marks = append(marks, hline(left, right, snap(py), base.Grid))
		marks = append(marks, text(left-gap, py, clock(h*60), tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
	}
	for d, name := range c.Days {
		marks = append(marks, text(cols.Position(d)+cols.Bandwidth()/2, header, name, tickSize, base.Semibold, scene.Middle, scene.Top, base.TextSecondary))
	}
	for i, s := range c.Slots {
		d := placed[i].day
		laneW := cols.Bandwidth() / float64(max(used[d], 1))
		ci := max(slices.Index(groups, s.Group), 0)
		r := scene.Rect{
			X: cols.Position(d) + float64(float64(lane[i])*laneW) + 1,
			Y: y.Map(float64(s.Start)) + 1,
			W: math.Max(laneW-2, 1),
			H: math.Max(y.Map(float64(s.End))-y.Map(float64(s.Start))-2, 1),
		}
		var rows []scene.TipRow
		if s.Group != "" {
			rows = append(rows, scene.TipRow{Name: "group", Value: s.Group})
		}
		// A light wash with an accent edge: the label stays dark text on
		// every palette hue, which a solid fill would not allow.
		marks = append(marks, scene.Mark{
			Shape:  scene.RectShape{Rect: r, Radius: scene.AllCorners(math.Min(math.Min(timetableRadius, r.W/2), r.H/2))},
			Fill:   base.Wash(ci),
			Series: s.Group,
			Hit: &scene.Hit{
				Key:   strconv.Itoa(i),
				Label: s.Label + " · " + s.Day,
				Value: clock(s.Start) + "–" + clock(s.End),
				Rows:  rows,
			},
		})
		ar := math.Min(math.Min(timetableRadius, r.H/2), timetableAccent)
		accent := r
		accent.W = math.Min(timetableAccent, r.W)
		marks = append(marks, scene.Mark{
			Shape:  scene.RectShape{Rect: accent, Radius: scene.Corners{TL: ar, BL: ar}},
			Fill:   base.Series(ci),
			Series: s.Group,
		})
		tw := m.Width(s.Label, timetableLabelSize, base.Semibold)
		if r.H >= timetableLabelSize+8 && tw <= r.W-timetableAccent-8 {
			t := text(r.X+timetableAccent+4, r.Y+4, s.Label, timetableLabelSize, base.Semibold, scene.Start, scene.Top, base.TextPrimary)
			t.Series = s.Group
			marks = append(marks, t)
		}
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Timetable of %s across %s.",
			count(len(c.Slots), "slot", "slots"), count(len(c.Days), "day", "days"))),
		Marks: marks,
	}, nil
}
