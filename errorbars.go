package charta

import (
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// whiskerCap is half the width of a whisker's end cap.
const whiskerCap = 4.0

// ErrorBar is a value and the uncertainty around it.
type ErrorBar struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Low   float64 `json:"low"`
	High  float64 `json:"high"`
}

// ErrorBarChart draws one estimate per category with its error range as a
// whisker.
type ErrorBarChart struct {
	Title string     `json:"title,omitempty"`
	Data  []ErrorBar `json:"data"`
}

func (c ErrorBarChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	for _, d := range c.Data {
		for _, f := range []struct {
			what string
			v    float64
		}{{"value", d.Value}, {"low", d.Low}, {"high", d.High}} {
			if err := finite(fmt.Sprintf("%q %s", d.Label, f.what), f.v); err != nil {
				return nil, err
			}
		}
		if !(d.Low <= d.Value && d.Value <= d.High) {
			return nil, invalid("%q needs low <= value <= high", d.Label)
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
	f := newCategoryFrame(m, width, height, top, labels, lo, hi, paddingInner, paddingOuter, 0)
	f.grid(&marks)
	f.yLabels(&marks)
	f.baseline(&marks)
	line := func(x1, y1, x2, y2 float64) scene.Mark {
		return scene.Mark{Shape: scene.LineShape{X1: x1, Y1: y1, X2: x2, Y2: y2}, Stroke: &scene.Stroke{Color: base.Series(0), Width: lineWidth}}
	}
	for i, d := range c.Data {
		cx := f.center(i)
		yLow, yHigh := f.y.scale.Map(d.Low), f.y.scale.Map(d.High)
		col := f.column(i)
		marks = append(marks,
			line(cx, yHigh, cx, yLow),
			line(cx-whiskerCap, yHigh, cx+whiskerCap, yHigh),
			line(cx-whiskerCap, yLow, cx+whiskerCap, yLow),
			scene.Mark{
				Shape:  scene.CircleShape{CX: cx, CY: f.y.scale.Map(d.Value), R: dotRadius},
				Fill:   base.Series(0),
				Stroke: &scene.Stroke{Color: base.Surface, Width: surfaceGap},
				Hit: &scene.Hit{
					Key:   strconv.Itoa(i),
					Label: d.Label + " · " + base.Value(d.Low) + " – " + base.Value(d.High),
					Value: base.Value(d.Value),
					Area:  &col,
				},
			})
	}
	f.xLabels(&marks, labels)

	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Error bar chart with %s.", count(len(c.Data), "estimate", "estimates"))),
		Marks:  marks,
	}, nil
}
