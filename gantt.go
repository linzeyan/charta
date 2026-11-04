package charta

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	ganttPaddingInner = 0.3
	ganttPaddingOuter = 0.15
	ganttBarMax       = 20.0
	ganttRadius       = 4.0
)

// Task is a span of time, or a milestone when Start == End (UTC epoch ms).
// Tasks in the same Group share a color; groups get a legend. An empty
// Group is none.
type Task struct {
	Name  string `json:"name"`
	Start int64  `json:"start"`
	End   int64  `json:"end"`
	Group string `json:"group,omitempty"`
}

// GanttChart draws tasks as bars on a shared time axis, one row each, in
// the given order. UTCOffset is the minutes the displayed clock is ahead of
// UTC.
type GanttChart struct {
	Title     string `json:"title,omitempty"`
	UTCOffset int    `json:"utcOffset,omitempty"`
	Tasks     []Task `json:"tasks"`
}

func (c GanttChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	s, _, err := c.laidOut(width, height, m, nil)
	return s, err
}

func (c GanttChart) PlotScene(width, height float64, m base.TextMeasure) (*scene.Scene, scene.Plot, error) {
	return c.laidOut(width, height, m, nil)
}

func (c GanttChart) View(x [2]float64, y *[2]float64) Viewable {
	return inView[GanttChart]{c, ranges{x, y}}
}

// laidOut keeps every task's row when zoomed; bars are cut at the view's
// edges, squarely, so a cut does not read as the task's start or end.
func (c GanttChart) laidOut(width, height float64, m base.TextMeasure, v *ranges) (*scene.Scene, scene.Plot, error) {
	if err := checkSize(width, height); err != nil {
		return nil, scene.Plot{}, err
	}
	offset := c.UTCOffset
	var groups []string
	for i, t := range c.Tasks {
		if t.End < t.Start {
			return nil, scene.Plot{}, invalid("task %d (%q) ends before it starts", i, t.Name)
		}
		if t.Group != "" && !slices.Contains(groups, t.Group) {
			groups = append(groups, t.Group)
		}
	}
	if len(groups) > maxSeries {
		return nil, scene.Plot{}, invalid("%d groups, at most %d: merge some", len(groups), maxSeries)
	}
	t0, t1 := int64(math.MaxInt64), int64(math.MinInt64)
	for _, t := range c.Tasks {
		t0, t1 = min(t0, t.Start), max(t1, t.End)
	}
	switch {
	case v != nil:
		t0, t1 = int64(math.Round(v.x[0])), int64(math.Round(v.x[1]))
	case t0 > t1:
		t0, t1 = 0, base.Day
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, groups, keySwatch)
	names := make([]string, len(c.Tasks))
	for i, t := range c.Tasks {
		names[i] = t.Name
	}
	left := pad + maxWidth(m, names) + gap
	right := math.Max(width-pad, left+1)
	bottom := math.Max(height-pad-tickSize-gap, top+1)
	countX := max(int(math.Round((right-left)/xTickSpacing)), 2)
	x := newTimeAxis(t0, t1, left, right, countX, offset)
	// Keep the last tick label inside the padding.
	x.fit(m, width-pad)
	right = x.scale.R1
	rows := base.NewBandScale(len(c.Tasks), top, bottom, ganttPaddingInner, ganttPaddingOuter)
	x.grid(&marks, top, bottom)
	thick := math.Min(rows.Bandwidth(), ganttBarMax)
	for i, t := range c.Tasks {
		cy := rows.Position(i) + rows.Bandwidth()/2
		g := slices.Index(groups, t.Group)
		color := base.Series(max(g, 0))
		start, end := max(t.Start, t0), min(t.End, t1)
		shown := start < end
		if t.Start == t.End {
			shown = t.Start >= t0 && t.Start <= t1
		}
		xa, xb := x.pos(start), x.pos(end)
		var shape scene.Shape
		if t.Start == t.End {
			// A milestone is a diamond on its date.
			r := thick / 2
			shape = scene.PathShape{Ops: []scene.PathOp{
				scene.Move(xa, cy-r),
				scene.Line(xa+r, cy),
				scene.Line(xa, cy+r),
				scene.Line(xa-r, cy),
				scene.Close(),
			}}
		} else {
			r := math.Min(math.Min(ganttRadius, (xb-xa)/2), thick/2)
			rs, re := r, r
			if start > t.Start {
				rs = 0
			}
			if end < t.End {
				re = 0
			}
			shape = scene.RectShape{
				Rect:   scene.Rect{X: xa, Y: cy - thick/2, W: xb - xa, H: thick},
				Radius: scene.Corners{TL: rs, TR: re, BR: re, BL: rs},
			}
		}
		label := t.Name
		if t.Group != "" && len(groups) > 1 {
			label = t.Name + " · " + t.Group
		}
		value := "milestone"
		tip := []scene.TipRow{{Name: "start", Value: base.FormatDatetime(t.Start, offset)}}
		if t.End != t.Start {
			value = base.FormatDuration(t.End - t.Start)
			tip = append(tip, scene.TipRow{Name: "end", Value: base.FormatDatetime(t.End, offset)})
		}
		if shown {
			marks = append(marks, scene.Mark{
				Shape:  shape,
				Fill:   color,
				Series: t.Group,
				Hit: &scene.Hit{
					Key:   strconv.Itoa(i),
					Label: label,
					Value: value,
					Rows:  tip,
					Area:  &scene.Rect{X: left, Y: rows.Position(i) - (rows.Step()-rows.Bandwidth())/2, W: right - left, H: rows.Step()},
				},
			})
		}
		marks = append(marks, text(left-gap, cy, t.Name, tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
	}
	x.labels(&marks, bottom+gap)
	plot := scene.Plot{
		Rect: scene.Rect{X: left, Y: top, W: right - left, H: bottom - top},
		X:    [2]float64{float64(t0), float64(t1)},
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Gantt chart of %s.", count(len(c.Tasks), "task", "tasks"))),
		Marks:  marks,
	}, plot, nil
}
