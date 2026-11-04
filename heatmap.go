package charta

import (
	"fmt"
	"math"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const cellRadius = 2.0

// HeatmapChart is a grid of cells colored by magnitude on a one-hue
// sequential ramp. X labels the columns, Y the rows top to bottom; Values
// has one row per Y label and one value per X label.
type HeatmapChart struct {
	Title  string      `json:"title,omitempty"`
	X      []string    `json:"x"`
	Y      []string    `json:"y"`
	Values [][]float64 `json:"values"`
}

func (c HeatmapChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if len(c.Values) != len(c.Y) {
		return nil, invalid("%d value rows for %d y labels", len(c.Values), len(c.Y))
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for r, row := range c.Values {
		if len(row) != len(c.X) {
			return nil, invalid("row %q has %d values for %d x labels", c.Y[r], len(row), len(c.X))
		}
		for col, v := range row {
			if !isFinite(v) {
				return nil, invalid("cell (%q, %q) is not finite", c.X[col], c.Y[r])
			}
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	if isFinite(lo) {
		top = seqLegend(&marks, m, top, lo, hi)
	}

	left := pad + maxWidth(m, c.Y) + gap
	right := math.Max(width-pad, left+1)
	cols := base.NewBandScale(len(c.X), left, right, 0, 0)
	maxX := maxWidth(m, c.X)
	rotated := len(c.X) > 0 && maxX > cols.Step()
	xArea := tickSize + gap
	if rotated {
		xArea = float64((maxX+tickSize)*sqrtHalf) + gap
	}
	bottom := math.Max(height-pad-xArea, top+1)
	rows := base.NewBandScale(len(c.Y), top, bottom, 0, 0)

	const halfGap = surfaceGap / 2
	for r, row := range c.Values {
		for col, v := range row {
			cell := scene.Rect{X: cols.Position(col), Y: rows.Position(r), W: cols.Step(), H: rows.Step()}
			marks = append(marks, scene.Mark{
				Shape: scene.RectShape{
					// Touching cells are separated by the surface gap.
					Rect: scene.Rect{
						X: cell.X + halfGap,
						Y: cell.Y + halfGap,
						W: math.Max(cell.W-surfaceGap, 0),
						H: math.Max(cell.H-surfaceGap, 0),
					},
					Radius: scene.AllCorners(cellRadius),
				},
				Fill: base.Seq(seqClass(v, lo, hi)),
				Hit:  &scene.Hit{Key: fmt.Sprintf("%d-%d", r, col), Label: c.X[col] + " · " + c.Y[r], Value: base.Value(v), Area: &cell},
			})
		}
	}
	for r, l := range c.Y {
		marks = append(marks, text(left-gap, rows.Position(r)+rows.Step()/2, l, tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
	}
	for col, l := range c.X {
		mk := text(cols.Position(col)+cols.Step()/2, bottom+gap, l, tickSize, base.Regular, scene.Middle, scene.Top, base.TextMuted)
		if rotated {
			mk = rotate(mk)
		}
		marks = append(marks, mk)
	}

	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Heatmap with %s and %s.", count(len(c.Y), "row", "rows"), count(len(c.X), "column", "columns"))),
		Marks:  marks,
	}, nil
}
