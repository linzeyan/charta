package charta

import (
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// Span is a range from low to high.
type Span struct {
	Label string  `json:"label"`
	Low   float64 `json:"low"`
	High  float64 `json:"high"`
}

// SpanChart draws horizontal bars spanning each category's range. A span
// has no baseline, so both ends are data ends and both are rounded.
type SpanChart struct {
	Title string `json:"title,omitempty"`
	Data  []Span `json:"data"`
}

func (c SpanChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	for _, d := range c.Data {
		if err := finite(fmt.Sprintf("%q low", d.Label), d.Low); err != nil {
			return nil, err
		}
		if err := finite(fmt.Sprintf("%q high", d.Label), d.High); err != nil {
			return nil, err
		}
		if d.Low > d.High {
			return nil, invalid("%q has low above high", d.Label)
		}
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	labels := make([]string, len(c.Data))
	for i, d := range c.Data {
		lo, hi = math.Min(lo, d.Low), math.Max(hi, d.High)
		labels[i] = d.Label
	}
	if !isFinite(lo) {
		lo, hi = 0, 0
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	f := newRowFrame(m, width, height, top, labels, lo, hi, paddingInner, paddingOuter)
	f.grid(&marks)
	bw := f.band.Bandwidth()
	thick := math.Min(barMax, bw)
	for i, d := range c.Data {
		x0, x1 := f.x.scale.Map(d.Low), f.x.scale.Map(d.High)
		length := x1 - x0
		row := f.row(i)
		marks = append(marks, scene.Mark{
			Shape: scene.RectShape{
				Rect:   scene.Rect{X: x0, Y: f.band.Position(i) + (bw-thick)/2, W: length, H: thick},
				Radius: scene.AllCorners(math.Min(math.Min(barRadius, thick/2), length/2)),
			},
			Fill: base.Series(0),
			Hit:  &scene.Hit{Key: strconv.Itoa(i), Label: d.Label, Value: base.Value(d.Low) + " – " + base.Value(d.High), Area: &row},
		})
	}
	f.rowLabels(&marks, labels)
	f.xLabels(&marks)

	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Span chart with %s.", count(len(c.Data), "range", "ranges"))),
		Marks:  marks,
	}, nil
}
