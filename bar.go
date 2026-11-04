package charta

import (
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	// barMax caps bar thickness: bars never fill their slot.
	barMax = 24.0
	// barRadius rounds the data end; the baseline end stays square.
	barRadius    = 4.0
	paddingInner = 0.2
	paddingOuter = 0.1
)

// Bar is one category and its value.
type Bar struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// BarChart draws vertical bars comparing one value across categories.
type BarChart struct {
	Title string `json:"title,omitempty"`
	Data  []Bar  `json:"data"`
}

func (c BarChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	for i, b := range c.Data {
		if !isFinite(b.Value) {
			return nil, invalid("bar %d (%q) has a non-finite value", i, b.Label)
		}
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	labels := make([]string, len(c.Data))
	lo, hi := 0.0, 0.0
	for i, b := range c.Data {
		labels[i] = b.Label
		lo = math.Min(lo, b.Value)
		hi = math.Max(hi, b.Value)
	}
	f := newCategoryFrame(m, width, height, top, labels, lo, hi, paddingInner, paddingOuter, 0)

	f.grid(&marks)
	f.yLabels(&marks)
	y0 := f.y.scale.Map(0)
	bw := f.band.Bandwidth()
	thick := math.Min(barMax, bw)
	for i, b := range c.Data {
		y1 := f.y.scale.Map(b.Value)
		h := math.Abs(y1 - y0)
		col := f.column(i)
		marks = append(marks, scene.Mark{
			Shape: scene.RectShape{
				Rect:   scene.Rect{X: f.band.Position(i) + (bw-thick)/2, Y: math.Min(y0, y1), W: thick, H: h},
				Radius: dataEnd(b.Value, thick, h),
			},
			Fill: base.Series(0),
			Hit:  &scene.Hit{Key: strconv.Itoa(i), Label: b.Label, Value: base.Value(b.Value), Area: &col},
		})
	}
	f.baseline(&marks)
	f.xLabels(&marks, labels)

	n := len(c.Data)
	plural := "s"
	if n == 1 {
		plural = ""
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Bar chart with %d bar%s.", n, plural)),
		Marks:  marks,
	}, nil
}

// dataEnd rounds the data end of a vertical bar: the top for positive
// values, the bottom for negative ones. The radius never exceeds half the
// thickness or the bar's own length.
func dataEnd(value, thick, length float64) scene.Corners {
	r := math.Min(math.Min(barRadius, thick/2), length)
	if value >= 0 {
		return scene.Corners{TL: r, TR: r}
	}
	return scene.Corners{BR: r, BL: r}
}
