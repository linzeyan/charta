package charta

import (
	"fmt"
	"math"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/layout"
	"github.com/linzeyan/charta/scene"
)

const (
	// maxDensitySeries: overlapping curves are compared all against all, so
	// only the palette's first three slots stay distinguishable (as on a
	// scatterplot).
	maxDensitySeries = 3
	// densitySamples is how many points each curve is sampled at; the curves
	// share one grid so their areas line up.
	densitySamples = 96
	// densityTail extends the support this many bandwidths past the data,
	// where the Gaussian tails have all but vanished.
	densityTail = 3.0
)

// DensityChart draws smoothed distributions (Gaussian kernel density
// estimates) of one or more samples on a shared value axis. XLabel names the
// measured quantity, under the x axis.
type DensityChart struct {
	Title  string   `json:"title,omitempty"`
	XLabel string   `json:"xLabel,omitempty"`
	Series []Sample `json:"series"`
}

func (c DensityChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if err := checkSamples(c.Series, maxDensitySeries, "series"); err != nil {
		return nil, err
	}
	sorted := make([][]float64, len(c.Series))
	bandwidths := make([]float64, len(c.Series))
	for j, s := range c.Series {
		sorted[j] = s.sorted()
		bandwidths[j] = layout.Silverman(sorted[j])
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for j, v := range sorted {
		h := bandwidths[j]
		lo = math.Min(lo, v[0]-float64(densityTail*h))
		hi = math.Max(hi, v[len(v)-1]+float64(densityTail*h))
	}
	if !isFinite(lo) {
		lo, hi = 0, 1
	}
	xs := layout.Grid(lo, hi, densitySamples)
	curves := make([][]float64, len(sorted))
	peak := 0.0
	for j, v := range sorted {
		curves[j] = layout.KDE(v, bandwidths[j], xs)
		for _, d := range curves[j] {
			peak = math.Max(peak, d)
		}
	}

	names := make([]string, len(c.Series))
	for j, s := range c.Series {
		names[j] = s.Name
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, names, keyLine)
	f := newXYFrame(m, width, height, top, lo, hi, true, 0, peak, math.MaxInt, c.XLabel != "")
	f.grid(&marks)
	y0 := f.y.scale.Map(0)
	for j, s := range c.Series {
		pts := make([]scene.Point, len(xs))
		area := make([]scene.AreaPoint, len(xs))
		for k, x := range xs {
			px, py := f.x.scale.Map(x), f.y.scale.Map(curves[j][k])
			pts[k] = scene.Point{X: px, Y: py}
			area[k] = scene.AreaPoint{X: px, Top: py, Bottom: y0}
		}
		marks = append(marks, scene.Mark{Shape: scene.AreaShape{Points: area}, Fill: base.Wash(j), Series: s.Name})
		v := sorted[j]
		rows := []scene.TipRow{
			{Name: "values", Value: base.Value(float64(len(v)))},
			{Name: "median", Value: base.Approx(layout.Quantile(v, 0.5))},
			{Name: "bandwidth", Value: base.Approx(bandwidths[j])},
		}
		marks = append(marks, scene.Mark{
			Shape:  scene.PolylineShape{Points: pts},
			Stroke: &scene.Stroke{Color: base.Series(j), Width: lineWidth},
			Series: s.Name,
			Hit:    &scene.Hit{Key: fmt.Sprintf("s%d", j), Label: s.Name, Rows: rows},
		})
	}
	f.baseline(&marks)
	f.labels(&marks, c.XLabel)

	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Density plot of %s.", count(len(c.Series), "series", "series"))),
		Marks:  marks,
	}, nil
}
