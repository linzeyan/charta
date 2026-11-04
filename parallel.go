package charta

import (
	"fmt"
	"math"
	"slices"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	// maxParallelGroups: lines cross every other line, so the all-pairs
	// palette limit applies.
	maxParallelGroups = 3
	parallelLine      = 1.5
	parallelHalo      = 1.0
)

// ParallelRow is one observation: a value on every dimension, optionally in
// a group ("" for none).
type ParallelRow struct {
	Label  string    `json:"label"`
	Group  string    `json:"group,omitempty"`
	Values []float64 `json:"values"`
}

// ParallelChart draws many numeric dimensions side by side, each on its own
// vertical axis; every observation is a line through its values.
type ParallelChart struct {
	Title      string        `json:"title,omitempty"`
	Dimensions []string      `json:"dimensions"`
	Rows       []ParallelRow `json:"rows"`
}

func (c ParallelChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	d := len(c.Dimensions)
	if d < 2 {
		return nil, invalid("a parallel coordinates plot needs at least 2 dimensions")
	}
	var groups []string
	for r, row := range c.Rows {
		if len(row.Values) != d {
			return nil, invalid("row %d (%q) has %d values for %d dimensions", r, row.Label, len(row.Values), d)
		}
		for k, v := range row.Values {
			if err := finite(fmt.Sprintf("row %d %q", r, c.Dimensions[k]), v); err != nil {
				return nil, err
			}
		}
		if row.Group != "" && !slices.Contains(groups, row.Group) {
			groups = append(groups, row.Group)
		}
	}
	if len(groups) > maxParallelGroups {
		return nil, invalid("%d groups, at most %d: lines cross, so every pair of colors must stay distinguishable", len(groups), maxParallelGroups)
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, groups, keyLine)
	namesY := top
	plotTop := top + tickSize + gap + tickSize/2
	bottom := math.Max(height-pad-tickSize/2, plotTop+1)
	ticks := max(int(math.Round((bottom-plotTop)/tickSpacing)), 2)
	axes := make([]valueAxis, d)
	for k := range axes {
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, r := range c.Rows {
			lo, hi = math.Min(lo, r.Values[k]), math.Max(hi, r.Values[k])
		}
		if !isFinite(lo) {
			lo, hi = 0, 1
		}
		axes[k] = newValueAxis(lo, hi, bottom, plotTop, ticks)
	}
	nameW := func(k int) float64 { return m.Width(c.Dimensions[k], tickSize, base.Semibold) }
	tickW := 0.0
	for _, t := range axes[0].ticks {
		tickW = math.Max(tickW, m.Width(axes[0].label(t), tickSize, base.Regular))
	}
	left := pad + math.Max(tickW+gap/2, nameW(0)/2)
	right := math.Max(width-pad-nameW(d-1)/2, left+1)
	x := func(k int) float64 { return left + float64((right-left)*float64(k)/float64(d-1)) }

	for k := range d {
		marks = append(marks, vline(snap(x(k)), plotTop, bottom, base.Axis))
	}
	multi := len(groups) > 1
	for r, row := range c.Rows {
		points := make([]scene.Point, d)
		rows := make([]scene.TipRow, d)
		for k := range d {
			points[k] = scene.Point{X: x(k), Y: axes[k].scale.Map(row.Values[k])}
			rows[k] = scene.TipRow{Name: c.Dimensions[k], Value: base.Value(row.Values[k])}
		}
		g := max(slices.Index(groups, row.Group), 0)
		label := row.Label
		if row.Group != "" && multi {
			label += " · " + row.Group
		}
		marks = append(marks, scene.Mark{
			Shape:  scene.PolylineShape{Points: points},
			Stroke: &scene.Stroke{Color: base.Series(g), Width: parallelLine},
			Series: row.Group,
			Hit:    &scene.Hit{Key: fmt.Sprintf("r%d", r), Label: label, Rows: rows},
		})
	}
	// Labels go on top of the lines they would otherwise hide under.
	for k, axis := range axes {
		marks = append(marks, text(x(k), namesY, c.Dimensions[k], tickSize, base.Semibold, scene.Middle, scene.Top, base.TextSecondary))
		for _, t := range axis.ticks {
			label := axis.label(t)
			// A surface-colored backing keeps the number legible where lines
			// run under it.
			w := m.Width(label, tickSize, base.Regular)
			marks = append(marks, scene.Mark{
				Shape: scene.RectShape{
					Rect: scene.Rect{
						X: x(k) - gap/2 - w - parallelHalo,
						Y: axis.scale.Map(t) - tickSize/2 - parallelHalo,
						W: w + 2*parallelHalo,
						H: tickSize + 2*parallelHalo,
					},
					Radius: scene.AllCorners(2),
				},
				Fill: base.Surface,
			})
			marks = append(marks, text(x(k)-gap/2, axis.scale.Map(t), label, tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
		}
	}

	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Parallel coordinates plot of %s across %s.",
			count(len(c.Rows), "row", "rows"), count(d, "dimension", "dimensions"))),
		Marks: marks,
	}, nil
}
