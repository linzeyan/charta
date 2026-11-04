package charta

import (
	"fmt"
	"math"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// StackedBarChart stacks series into one bar per category. Positive values
// stack up from the baseline, negative ones down, each direction on its own.
type StackedBarChart struct {
	Title      string   `json:"title,omitempty"`
	Categories []string `json:"categories"`
	Series     []Series `json:"series"`
}

func (c StackedBarChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if err := checkSeries(c.Categories, c.Series, maxSeries); err != nil {
		return nil, err
	}
	names := make([]string, len(c.Series))
	for j, s := range c.Series {
		names[j] = s.Name
	}
	lo, hi := 0.0, 0.0
	for i := range c.Categories {
		pos, neg := 0.0, 0.0
		for _, s := range c.Series {
			if v := s.Values[i]; v > 0 {
				pos += v
			} else {
				neg += v
			}
		}
		hi = math.Max(hi, pos)
		lo = math.Min(lo, neg)
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, names, keySwatch)
	f := newCategoryFrame(m, width, height, top, c.Categories, lo, hi, paddingInner, paddingOuter, 0)
	f.grid(&marks)
	f.yLabels(&marks)

	bw := f.band.Bandwidth()
	thick := math.Min(barMax, bw)
	const halfGap = surfaceGap / 2
	for i, cat := range c.Categories {
		col := f.column(i)
		// The outermost segment in each direction gets the rounded end.
		lastPos, lastNeg := -1, -1
		for j, s := range c.Series {
			if s.Values[i] > 0 {
				lastPos = j
			} else if s.Values[i] < 0 {
				lastNeg = j
			}
		}
		pos, neg := 0.0, 0.0
		for j, s := range c.Series {
			v := s.Values[i]
			if v == 0 {
				continue
			}
			var from, to float64
			var outer bool
			if v > 0 {
				pos += v
				from, to, outer = pos-v, pos, lastPos == j
			} else {
				neg += v
				from, to, outer = neg-v, neg, lastNeg == j
			}
			// from is the end nearer the baseline. Both ends shrink by half
			// the surface gap where they touch another segment.
			yFrom, yTo := f.y.scale.Map(from), f.y.scale.Map(to)
			insetFrom, insetTo := 0.0, halfGap
			if from != 0 {
				insetFrom = halfGap
			}
			if outer {
				insetTo = 0
			}
			var topY, bottomY float64
			if v > 0 {
				topY, bottomY = yTo+insetTo, yFrom-insetFrom
			} else {
				topY, bottomY = yFrom+insetFrom, yTo-insetTo
			}
			h := math.Max(bottomY-topY, 0)
			var radius scene.Corners
			if outer {
				radius = dataEnd(v, thick, h)
			}
			area := col
			area.Y, area.H = topY-halfGap, h+surfaceGap
			marks = append(marks, scene.Mark{
				Shape: scene.RectShape{
					Rect:   scene.Rect{X: f.band.Position(i) + (bw-thick)/2, Y: topY, W: thick, H: h},
					Radius: radius,
				},
				Fill:   base.Series(j),
				Series: s.Name,
				Hit:    &scene.Hit{Key: fmt.Sprintf("%d-%d", i, j), Label: s.Name + " · " + cat, Value: base.Value(v), Area: &area},
			})
		}
	}
	f.baseline(&marks)
	f.xLabels(&marks, c.Categories)

	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Stacked bar chart with %s and %s.", count(len(c.Series), "series", "series"), count(len(c.Categories), "category", "categories"))),
		Marks:  marks,
	}, nil
}
