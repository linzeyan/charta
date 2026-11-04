package charta

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/layout"
	"github.com/linzeyan/charta/scene"
)

// HistogramChart draws the distribution of one variable as counts in
// equal-width bins. XLabel names the measured quantity, under the x axis.
// Bins asks for about that many bins; edges snap to round numbers, so the
// result may differ by one or two. Nil uses Sturges' rule.
type HistogramChart struct {
	Title  string    `json:"title,omitempty"`
	XLabel string    `json:"xLabel,omitempty"`
	Values []float64 `json:"values"`
	Bins   *int      `json:"bins,omitempty"`
}

func (c HistogramChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	for i, v := range c.Values {
		if err := finite(fmt.Sprintf("value %d", i), v); err != nil {
			return nil, err
		}
	}
	if c.Bins != nil && *c.Bins < 1 {
		return nil, invalid("bins must be at least 1")
	}
	sorted := slices.Clone(c.Values)
	slices.Sort(sorted)
	target := layout.Sturges(len(sorted))
	if c.Bins != nil {
		target = *c.Bins
	}
	edges, counts := layout.Bins(sorted, target)
	xlo, xhi := 0.0, 1.0
	if len(edges) > 0 {
		xlo, xhi = edges[0], edges[len(edges)-1]
	}
	most := 0
	for _, n := range counts {
		most = max(most, n)
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	// Counts are whole numbers: never more ticks than the tallest count.
	f := newXYFrame(m, width, height, top, xlo, xhi, true, 0, float64(most), max(most, 1), c.XLabel != "")
	f.grid(&marks)
	y0 := f.y.scale.Map(0)
	for i, n := range counts {
		x0, x1 := f.x.scale.Map(edges[i]), f.x.scale.Map(edges[i+1])
		y1 := f.y.scale.Map(float64(n))
		// Adjacent bins keep a surface-colored gap so each reads as a bar.
		w := math.Max(x1-x0-surfaceGap, 1)
		h := y0 - y1
		r := math.Min(math.Min(barRadius, w/2), h)
		marks = append(marks, scene.Mark{
			Shape: scene.RectShape{
				Rect:   scene.Rect{X: x0 + surfaceGap/2, Y: y1, W: w, H: h},
				Radius: scene.Corners{TL: r, TR: r},
			},
			Fill: base.Series(0),
			Hit: &scene.Hit{
				Key:   strconv.Itoa(i),
				Label: base.Value(edges[i]) + "–" + base.Value(edges[i+1]),
				Value: base.Value(float64(n)),
				Area:  &scene.Rect{X: x0, Y: f.top, W: x1 - x0, H: f.bottom - f.top},
			},
		})
	}
	f.baseline(&marks)
	f.labels(&marks, c.XLabel)

	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Histogram of %s in %s.", count(len(sorted), "value", "values"), count(len(counts), "bin", "bins"))),
		Marks:  marks,
	}, nil
}
