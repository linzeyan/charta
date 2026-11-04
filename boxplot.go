package charta

import (
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/layout"
	"github.com/linzeyan/charta/scene"
)

const (
	boxPaddingInner = 0.3
	boxPaddingOuter = 0.15
	boxMax          = 48.0
	violinMax       = 96.0
	// violinSamples is the number of points along each violin's outline
	// (per side).
	violinSamples = 48
	// violinInner is the width of the interquartile bar inside a violin.
	violinInner = 6.0
	outline     = 1.5
)

// BoxPlotChart draws each group's quartiles as a box, whiskers to the last
// values inside 1.5 × IQR, and the values beyond as outliers. YLabel names
// the measured quantity, above the value axis.
type BoxPlotChart struct {
	Title  string   `json:"title,omitempty"`
	YLabel string   `json:"yLabel,omitempty"`
	Groups []Sample `json:"groups"`
}

// ViolinChart draws each group's distribution as a mirrored density, with
// its quartiles inside. YLabel names the measured quantity, above the value
// axis.
type ViolinChart struct {
	Title  string   `json:"title,omitempty"`
	YLabel string   `json:"yLabel,omitempty"`
	Groups []Sample `json:"groups"`
}

// sampleLayout is what both charts share: validated groups, their summaries
// and the frame.
type sampleLayout struct {
	sorted [][]float64
	stats  []layout.BoxStats
	frame  categoryFrame
	marks  []scene.Mark
}

func layoutSamples(title, yLabel string, groups []Sample, width, height float64, m base.TextMeasure) (sampleLayout, error) {
	if err := checkSize(width, height); err != nil {
		return sampleLayout{}, err
	}
	if err := checkSamples(groups, math.MaxInt, "group"); err != nil {
		return sampleLayout{}, err
	}
	l := sampleLayout{sorted: make([][]float64, len(groups)), stats: make([]layout.BoxStats, len(groups))}
	lo, hi := math.Inf(1), math.Inf(-1)
	for i, g := range groups {
		v := g.sorted()
		l.sorted[i], l.stats[i] = v, layout.Box(v)
		lo, hi = math.Min(lo, v[0]), math.Max(hi, v[len(v)-1])
	}
	if !isFinite(lo) {
		lo, hi = 0, 1
	}

	top := drawTitle(&l.marks, title)
	if yLabel != "" {
		l.marks = append(l.marks, text(pad, top, yLabel, tickSize, base.Regular, scene.Start, scene.Top, base.TextMuted))
		top += tickSize + gap
	}
	l.frame = newCategoryFrame(m, width, height, top, sampleNames(groups), lo, hi, boxPaddingInner, boxPaddingOuter, 0)
	l.frame.grid(&l.marks)
	l.frame.yLabels(&l.marks)
	return l, nil
}

func sampleNames(groups []Sample) []string {
	names := make([]string, len(groups))
	for i, g := range groups {
		names[i] = g.Name
	}
	return names
}

// summary is a group's tooltip: its summary, largest first, as on the plot.
func summary(b layout.BoxStats, n int) []scene.TipRow {
	rows := []scene.TipRow{
		{Name: "upper whisker", Value: base.Value(b.High)},
		{Name: "Q3", Value: base.Approx(b.Q3)},
		{Name: "median", Value: base.Approx(b.Median)},
		{Name: "Q1", Value: base.Approx(b.Q1)},
		{Name: "lower whisker", Value: base.Value(b.Low)},
	}
	if len(b.Outliers) > 0 {
		rows = append(rows, scene.TipRow{Name: "outliers", Value: base.Value(float64(len(b.Outliers)))})
	}
	return append(rows, scene.TipRow{Name: "values", Value: base.Value(float64(n))})
}

func (c BoxPlotChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	l, err := layoutSamples(c.Title, c.YLabel, c.Groups, width, height, m)
	if err != nil {
		return nil, err
	}
	f, marks := l.frame, l.marks
	w := math.Min(boxMax, f.band.Bandwidth())
	y := f.y.scale.Map
	line := func(x1, y1, x2, y2 float64) scene.Mark {
		return scene.Mark{Shape: scene.LineShape{X1: x1, Y1: y1, X2: x2, Y2: y2}, Stroke: &scene.Stroke{Color: base.Series(0), Width: lineWidth}}
	}
	for i, g := range c.Groups {
		b := l.stats[i]
		cx := f.center(i)
		marks = append(marks, line(cx, y(b.Low), cx, y(b.High)))
		for _, v := range []float64{b.Low, b.High} {
			marks = append(marks, line(cx-w/4, y(v), cx+w/4, y(v)))
		}
		col := f.column(i)
		marks = append(marks, scene.Mark{
			Shape: scene.RectShape{
				Rect:   scene.Rect{X: cx - w/2, Y: y(b.Q3), W: w, H: y(b.Q1) - y(b.Q3)},
				Radius: scene.AllCorners(math.Min(2, (y(b.Q1)-y(b.Q3))/2)),
			},
			Fill: base.Series(0),
			Hit:  &scene.Hit{Key: strconv.Itoa(i), Label: g.Name, Rows: summary(b, len(l.sorted[i])), Area: &col},
		})
		// The median cuts the box in the surface color, legible on the fill.
		marks = append(marks, scene.Mark{
			Shape:  scene.LineShape{X1: cx - w/2, Y1: y(b.Median), X2: cx + w/2, Y2: y(b.Median)},
			Stroke: &scene.Stroke{Color: base.Surface, Width: lineWidth},
		})
		for k, v := range b.Outliers {
			marks = append(marks, scene.Mark{
				Shape:  scene.CircleShape{CX: cx, CY: y(v), R: dotRadius - outline/2},
				Fill:   base.Surface,
				Stroke: &scene.Stroke{Color: base.Series(0), Width: outline},
				Hit: &scene.Hit{
					Key:   fmt.Sprintf("%d-o%d", i, k),
					Label: g.Name + " · outlier",
					Value: base.Value(v),
					Area:  &scene.Rect{X: cx - minTarget/2, Y: y(v) - minTarget/2, W: minTarget, H: minTarget},
				},
			})
		}
	}
	f.baseline(&marks)
	f.xLabels(&marks, sampleNames(c.Groups))

	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Box plot of %s.", count(len(c.Groups), "group", "groups"))),
		Marks:  marks,
	}, nil
}

func (c ViolinChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	l, err := layoutSamples(c.Title, c.YLabel, c.Groups, width, height, m)
	if err != nil {
		return nil, err
	}
	f, marks := l.frame, l.marks
	w := math.Min(violinMax, f.band.Bandwidth())
	y := f.y.scale.Map
	// Each violin is trimmed to its own data range; one density scale for
	// all keeps their areas equal, so widths compare.
	xs := make([][]float64, len(l.sorted))
	ds := make([][]float64, len(l.sorted))
	peak := 0.0
	for i, v := range l.sorted {
		xs[i] = layout.Grid(v[0], v[len(v)-1], violinSamples)
		ds[i] = layout.KDE(v, layout.Silverman(v), xs[i])
		for _, d := range ds[i] {
			peak = math.Max(peak, d)
		}
	}
	for i, g := range c.Groups {
		b := l.stats[i]
		cx := f.center(i)
		col := f.column(i)
		hit := &scene.Hit{Key: strconv.Itoa(i), Label: g.Name, Rows: summary(b, len(l.sorted[i])), Area: &col}
		x, d := xs[i], ds[i]
		// A single-value group has no spread to draw; its tooltip moves to the
		// interquartile bar so it can still be hovered.
		if x[0] < x[len(x)-1] {
			half := func(k int) float64 { return float64(d[k] / peak * w / 2) }
			ops := make([]scene.PathOp, 0, 2*len(x)+1)
			ops = append(ops, scene.Move(cx+half(0), y(x[0])))
			for k := 1; k < len(x); k++ {
				ops = append(ops, scene.Line(cx+half(k), y(x[k])))
			}
			for k := len(x) - 1; k >= 0; k-- {
				ops = append(ops, scene.Line(cx-half(k), y(x[k])))
			}
			ops = append(ops, scene.Close())
			marks = append(marks, scene.Mark{
				Shape:  scene.PathShape{Ops: ops},
				Fill:   base.Wash(0),
				Stroke: &scene.Stroke{Color: base.Series(0), Width: outline},
				Hit:    hit,
			})
			hit = nil
		}
		marks = append(marks, scene.Mark{
			Shape: scene.RectShape{
				Rect:   scene.Rect{X: cx - violinInner/2, Y: y(b.Q3), W: violinInner, H: y(b.Q1) - y(b.Q3)},
				Radius: scene.AllCorners(math.Min(violinInner/2, (y(b.Q1)-y(b.Q3))/2)),
			},
			Fill: base.Series(0),
			Hit:  hit,
		})
		marks = append(marks, scene.Mark{
			Shape: scene.CircleShape{CX: cx, CY: y(b.Median), R: violinInner/2 - 0.5},
			Fill:  base.Surface,
		})
	}
	f.baseline(&marks)
	f.xLabels(&marks, sampleNames(c.Groups))

	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Violin plot of %s.", count(len(c.Groups), "group", "groups"))),
		Marks:  marks,
	}, nil
}
