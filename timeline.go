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
	timelineLabelSize = 12.0
	timelineDot       = 5.0
	// timelineBlock is a label block: date line, then the event's name.
	timelineBlock = tickSize + 4.0 + timelineLabelSize
	// timelineStemMin is the distance from the axis to the nearest block.
	timelineStemMin = 10.0
)

// Event is something that happened at an instant (UTC epoch ms).
type Event struct {
	Time  int64  `json:"time"`
	Label string `json:"label"`
}

// TimelineChart places events in time order along one line, labels stacked
// above and below so they never overlap. UTCOffset is the minutes the
// displayed clock is ahead of UTC.
type TimelineChart struct {
	Title     string  `json:"title,omitempty"`
	UTCOffset int     `json:"utcOffset,omitempty"`
	Events    []Event `json:"events"`
}

// totalLess orders floats like Rust's f64::total_cmp.
func totalLess(a, b float64) bool {
	key := func(f float64) int64 {
		v := int64(math.Float64bits(f))
		return v ^ int64(uint64(v>>63)>>1)
	}
	return key(a) < key(b)
}

// timelineLevels picks a level for each label extent [x0, x1] in x order:
// the first of above-1, below-1, above-2, below-2 … whose previous label
// ends at least gap before this one starts; when none is free, the level
// whose last label ends earliest. Levels are ±1, ±2, … (positive is above).
func timelineLevels(extents [][2]float64, perSide int) []int {
	var order []int
	for n := 1; n <= max(perSide, 1); n++ {
		order = append(order, n, -n)
	}
	ends := make([]float64, len(order))
	for k := range ends {
		ends[k] = math.Inf(-1)
	}
	out := make([]int, len(extents))
	for i, e := range extents {
		k := -1
		for j := range order {
			if ends[j]+gap <= e[0] {
				k = j
				break
			}
		}
		if k < 0 {
			k = 0
			for j := 1; j < len(ends); j++ {
				if totalLess(ends[j], ends[k]) {
					k = j
				}
			}
		}
		ends[k] = e[1]
		out[i] = order[k]
	}
	return out
}

func (c TimelineChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	offset := c.UTCOffset
	order := make([]int, len(c.Events))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return c.Events[order[a]].Time < c.Events[order[b]].Time })

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	bottom := height - pad
	axisY := (top + bottom) / 2
	left, right := pad, math.Max(width-pad, pad+1)
	t0, t1 := int64(0), base.Day
	if len(order) > 0 {
		t0, t1 = c.Events[order[0]].Time, c.Events[order[len(order)-1]].Time
	}
	// Pad the ends so the first and last dots are not on the edge.
	tpad := max((t1-t0)/20, base.Hour)
	x := base.LinearScale{D0: float64(t0 - tpad), D1: float64(t1 + tpad), R0: left, R1: right}
	marks = append(marks, hline(left, right, snap(axisY), base.Axis))

	dates := make([]string, len(order))
	for k, i := range order {
		dates[k] = base.FormatDate(c.Events[i].Time, offset)
	}
	// Each label starts at its dot and runs right, unless that would leave
	// the chart; then it ends at its dot.
	extents := make([][2]float64, len(order))
	anchors := make([]scene.Anchor, len(order))
	for k, i := range order {
		e := c.Events[i]
		w := math.Max(m.Width(e.Label, timelineLabelSize, base.Semibold), m.Width(dates[k], tickSize, base.Regular))
		cx := x.Map(float64(e.Time))
		if cx+w <= right {
			extents[k], anchors[k] = [2]float64{cx, cx + w}, scene.Start
		} else {
			extents[k], anchors[k] = [2]float64{cx - w, cx}, scene.End
		}
	}
	const step = timelineBlock + gap
	perSide := max(int(math.Floor((axisY-top-timelineStemMin)/step)), 1)
	lv := timelineLevels(extents, perSide)

	for k, i := range order {
		e := c.Events[i]
		cx := x.Map(float64(e.Time))
		x0, x1, anchor := extents[k][0], extents[k][1], anchors[k]
		n := float64(lv[k])
		if lv[k] < 0 {
			n = -n
		}
		var blockTop, stemEnd, near float64
		if lv[k] > 0 {
			blockTop = axisY - timelineStemMin - timelineBlock - float64((n-1)*step)
			stemEnd, near = blockTop+timelineBlock, blockTop+timelineBlock+2
		} else {
			blockTop = axisY + timelineStemMin + float64((n-1)*step)
			stemEnd, near = blockTop, blockTop-2
		}
		marks = append(marks, vline(snap(cx), math.Min(axisY, near), math.Max(axisY, near), base.Grid))
		marks = append(marks, text(cx, blockTop, dates[k], tickSize, base.Regular, anchor, scene.Top, base.TextMuted))
		marks = append(marks, text(cx, blockTop+tickSize+4, e.Label, timelineLabelSize, base.Semibold, anchor, scene.Top, base.TextPrimary))
		area := scene.Rect{
			X: math.Min(x0, cx-timelineDot) - 4,
			Y: math.Min(blockTop, axisY-timelineDot),
			W: math.Max(x1, cx+timelineDot) - math.Min(x0, cx-timelineDot) + 8,
			H: math.Abs(math.Max(stemEnd, axisY+timelineDot) - math.Min(blockTop, axisY-timelineDot)),
		}
		marks = append(marks, scene.Mark{
			Shape:  scene.CircleShape{CX: cx, CY: axisY, R: timelineDot},
			Fill:   base.Series(0),
			Stroke: &scene.Stroke{Color: base.Surface, Width: surfaceGap},
			Hit:    &scene.Hit{Key: strconv.Itoa(i), Label: base.FormatDatetime(e.Time, offset), Value: e.Label, Area: &area},
		})
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Timeline of %s.", count(len(c.Events), "event", "events"))),
		Marks:  marks,
	}, nil
}
