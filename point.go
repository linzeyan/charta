package charta

import (
	"fmt"
	"math"
	"sort"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	// maxPointSeries: scatter and bubble charts compare every series against
	// every other, so the palette's all-pairs check applies and only its
	// first three slots stay distinguishable.
	maxPointSeries = 3
	// bubbleMax is the largest bubble's radius.
	bubbleMax = 24.0
	// bubbleMin keeps a bubble from shrinking below a visible dot.
	bubbleMin = 2.0
)

// Point is one observation. Size (area-proportional) is required by bubble
// charts and ignored by scatterplots.
type Point struct {
	X     float64  `json:"x"`
	Y     float64  `json:"y"`
	Size  *float64 `json:"size,omitempty"`
	Label string   `json:"label,omitempty"`
}

type PointSeries struct {
	Name   string  `json:"name"`
	Points []Point `json:"points"`
}

// ScatterChart draws points on two value axes. XLabel names the x quantity
// (under the x axis), YLabel the y quantity (above the y axis).
type ScatterChart struct {
	Title  string        `json:"title,omitempty"`
	XLabel string        `json:"xLabel,omitempty"`
	YLabel string        `json:"yLabel,omitempty"`
	Series []PointSeries `json:"series"`
}

// BubbleChart is a scatterplot whose point areas encode a third value.
type BubbleChart struct {
	Title  string        `json:"title,omitempty"`
	XLabel string        `json:"xLabel,omitempty"`
	YLabel string        `json:"yLabel,omitempty"`
	Series []PointSeries `json:"series"`
}

func (c ScatterChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	s, _, err := c.laidOut(width, height, m, nil)
	return s, err
}

func (c ScatterChart) PlotScene(width, height float64, m base.TextMeasure) (*scene.Scene, scene.Plot, error) {
	return c.laidOut(width, height, m, nil)
}

func (c ScatterChart) View(x [2]float64, y *[2]float64) Viewable {
	return inView[ScatterChart]{c, ranges{x, y}}
}

func (c ScatterChart) laidOut(width, height float64, m base.TextMeasure, v *ranges) (*scene.Scene, scene.Plot, error) {
	return buildPoints(false, c, width, height, m, v)
}

func (c BubbleChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	s, _, err := c.laidOut(width, height, m, nil)
	return s, err
}

func (c BubbleChart) PlotScene(width, height float64, m base.TextMeasure) (*scene.Scene, scene.Plot, error) {
	return c.laidOut(width, height, m, nil)
}

func (c BubbleChart) View(x [2]float64, y *[2]float64) Viewable {
	return inView[BubbleChart]{c, ranges{x, y}}
}

func (c BubbleChart) laidOut(width, height float64, m base.TextMeasure, v *ranges) (*scene.Scene, scene.Plot, error) {
	return buildPoints(true, ScatterChart(c), width, height, m, v)
}

// buildPoints lays the points out over all the data, niced, or over exactly
// the ranges in v, showing the points inside them. Bubble sizes scale to the
// biggest of all, so they keep their size as the view moves.
func buildPoints(bubble bool, c ScatterChart, width, height float64, m base.TextMeasure, v *ranges) (*scene.Scene, scene.Plot, error) {
	if err := checkSize(width, height); err != nil {
		return nil, scene.Plot{}, err
	}
	if len(c.Series) > maxPointSeries {
		return nil, scene.Plot{}, invalid(`%d series, at most %d on a scatter or bubble chart: fold the rest into "Other" or use small multiples`, len(c.Series), maxPointSeries)
	}
	maxSize := 0.0
	for j, s := range c.Series {
		for _, o := range c.Series[:j] {
			if o.Name == s.Name {
				return nil, scene.Plot{}, invalid("series name %q is used twice", s.Name)
			}
		}
		for p, pt := range s.Points {
			if err := finite(fmt.Sprintf("series %q point %d x", s.Name, p), pt.X); err != nil {
				return nil, scene.Plot{}, err
			}
			if err := finite(fmt.Sprintf("series %q point %d y", s.Name, p), pt.Y); err != nil {
				return nil, scene.Plot{}, err
			}
			if bubble {
				if pt.Size == nil {
					return nil, scene.Plot{}, invalid("series %q point %d has no size", s.Name, p)
				}
				if size := *pt.Size; !(isFinite(size) && size >= 0) {
					return nil, scene.Plot{}, invalid("series %q point %d size must be a non-negative number", s.Name, p)
				}
				maxSize = math.Max(maxSize, *pt.Size)
			}
		}
	}
	xlo, xhi := math.Inf(1), math.Inf(-1)
	ylo, yhi := math.Inf(1), math.Inf(-1)
	for _, s := range c.Series {
		for _, p := range s.Points {
			xlo, xhi = math.Min(xlo, p.X), math.Max(xhi, p.X)
			ylo, yhi = math.Min(ylo, p.Y), math.Max(yhi, p.Y)
		}
	}
	if !isFinite(xlo) {
		xlo, xhi = 0, 0
	}
	if !isFinite(ylo) {
		ylo, yhi = 0, 0
	}
	axis := newValueAxis
	if v != nil {
		xlo, xhi = v.x[0], v.x[1]
		if v.y != nil {
			ylo, yhi = v.y[0], v.y[1]
		}
		axis = exactValueAxis
	}

	names := make([]string, len(c.Series))
	for j, s := range c.Series {
		names[j] = s.Name
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, names, keySwatch)
	if c.YLabel != "" {
		marks = append(marks, text(pad, top, c.YLabel, tickSize, base.Regular, scene.Start, scene.Top, base.TextMuted))
		top += tickSize + gap
	}
	plotTop := top + tickSize/2
	xArea := tickSize + gap
	if c.XLabel != "" {
		xArea += tickSize + gap
	}
	bottom := math.Max(height-pad-xArea, plotTop+1)
	// The data scales stop short of the plot edges by the largest mark's
	// radius, so a point at an extreme never covers the axis or its labels.
	inset := dotRadius + surfaceGap
	if bubble {
		inset = bubbleMax
	}
	countY := max(int(math.Round((bottom-plotTop-float64(2*inset))/tickSpacing)), 2)
	y := axis(ylo, yhi, bottom-inset, plotTop+inset, countY)
	left := pad + y.labelWidth(m) + gap
	spanX := math.Max(width-pad-left-float64(2*inset), 1)
	countX := max(int(math.Round(spanX/xTickSpacing)), 2)
	x := axis(xlo, xhi, left+inset, left+inset+spanX, countX)
	last := x.lastLabelWidth(m)
	xEnd := math.Max(width-pad-math.Max(last/2, inset), left+inset+1)
	x.scale.R1 = xEnd
	right := xEnd + inset

	// Axis lines sit on zero when the domain spans it, else on the plot edge.
	spansZero := func(a valueAxis) bool { return a.scale.D0 <= 0 && a.scale.D1 >= 0 }
	xZero, yZero := spansZero(x), spansZero(y)
	for _, t := range y.ticks {
		if !(yZero && t == 0) {
			marks = append(marks, hline(left, right, snap(y.scale.Map(t)), base.Grid))
		}
	}
	for _, t := range x.ticks {
		if !(xZero && t == 0) {
			marks = append(marks, vline(snap(x.scale.Map(t)), plotTop, bottom, base.Grid))
		}
	}
	for _, t := range y.ticks {
		marks = append(marks, text(left-gap, y.scale.Map(t), y.label(t), tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
	}
	for _, t := range x.ticks {
		marks = append(marks, text(x.scale.Map(t), bottom+gap, x.label(t), tickSize, base.Regular, scene.Middle, scene.Top, base.TextMuted))
	}
	yAxis, xAxis := bottom, left
	if yZero {
		yAxis = y.scale.Map(0)
	}
	if xZero {
		xAxis = x.scale.Map(0)
	}
	marks = append(marks, hline(left, right, snap(yAxis), base.Axis))
	marks = append(marks, vline(snap(xAxis), plotTop, bottom, base.Axis))
	if c.XLabel != "" {
		marks = append(marks, text(right, bottom+gap+tickSize+gap, c.XLabel, tickSize, base.Regular, scene.End, scene.Top, base.TextMuted))
	}

	type dot struct {
		j, p int
		r    float64
	}
	inside := func(pt Point) bool {
		return v == nil || (pt.X >= x.scale.D0 && pt.X <= x.scale.D1 && pt.Y >= y.scale.D0 && pt.Y <= y.scale.D1)
	}
	var order []dot
	for j, s := range c.Series {
		for p, pt := range s.Points {
			if !inside(pt) {
				continue
			}
			r := dotRadius
			if bubble {
				r = 0
				if maxSize > 0 {
					r = float64(bubbleMax * math.Sqrt(sizeOf(pt)/maxSize))
				}
				r = math.Max(r, bubbleMin)
			}
			order = append(order, dot{j, p, r})
		}
	}
	// Big bubbles first, so small ones stay visible on top.
	if bubble {
		sort.SliceStable(order, func(a, b int) bool { return order[a].r > order[b].r })
	}
	single := len(c.Series) == 1
	for _, d := range order {
		s := c.Series[d.j]
		pt := s.Points[d.p]
		cx, cy := x.scale.Map(pt.X), y.scale.Map(pt.Y)
		label := s.Name
		if pt.Label != "" {
			label = pt.Label
			if !single {
				label = pt.Label + " · " + s.Name
			}
		}
		if bubble {
			label += " · size " + base.Value(sizeOf(pt))
		}
		hit := &scene.Hit{Key: fmt.Sprintf("%d-%d", d.j, d.p), Label: label, Value: base.Value(pt.X) + ", " + base.Value(pt.Y)}
		// A small dot is a pinpoint: give it a square target of minTarget.
		if d.r+surfaceGap < minTarget/2 {
			hit.Area = &scene.Rect{X: cx - minTarget/2, Y: cy - minTarget/2, W: minTarget, H: minTarget}
		}
		marks = append(marks, scene.Mark{
			Shape: scene.CircleShape{CX: cx, CY: cy, R: d.r},
			Fill:  base.Series(d.j),
			// The surface ring keeps overlapping dots legible.
			Stroke: &scene.Stroke{Color: base.Surface, Width: surfaceGap},
			Series: s.Name,
			Hit:    hit,
		})
	}

	n := len(order)
	what := "Scatterplot"
	if bubble {
		what = "Bubble chart"
	}
	plot := scene.Plot{
		Rect: scene.Rect{X: x.scale.R0, Y: y.scale.R1, W: x.scale.R1 - x.scale.R0, H: y.scale.R0 - y.scale.R1},
		X:    [2]float64{x.scale.D0, x.scale.D1},
		Y:    &[2]float64{y.scale.D0, y.scale.D1},
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("%s with %s and %s.", what, count(len(c.Series), "series", "series"), count(n, "point", "points"))),
		Marks:  marks,
	}, plot, nil
}

func sizeOf(p Point) float64 {
	if p.Size == nil {
		return 0
	}
	return *p.Size
}
