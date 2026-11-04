package charta

import (
	"fmt"
	"math"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// marimekkoQuarters: percent ticks up the side; quarters read at a glance.
const marimekkoQuarters = 4

// MarimekkoChart has columns as wide as each category's total and split by
// series share: a 100% stacked bar chart whose bar widths also carry a
// value, so every cell's area is its share of the grand total.
type MarimekkoChart struct {
	Title      string   `json:"title,omitempty"`
	Categories []string `json:"categories"`
	Series     []Series `json:"series"`
}

func (c MarimekkoChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if err := checkSeries(c.Categories, c.Series, maxSeries); err != nil {
		return nil, err
	}
	for _, s := range c.Series {
		for _, v := range s.Values {
			if v < 0 {
				return nil, invalid("series %q has a negative value; shares cannot be", s.Name)
			}
		}
	}
	totals := make([]float64, len(c.Categories))
	for i := range totals {
		for _, s := range c.Series {
			totals[i] += s.Values[i]
		}
	}
	grand := 0.0
	for _, t := range totals {
		grand += t
	}
	if len(c.Series) > 0 && len(c.Categories) > 0 && grand <= 0 {
		return nil, invalid("every value is zero: there is no whole to divide")
	}

	names := make([]string, len(c.Series))
	for j, s := range c.Series {
		names[j] = s.Name
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, names, keySwatch)
	plotTop := top + tickSize/2
	bottom := math.Max(height-pad-tickSize-gap, plotTop+1)
	left := pad + m.Width("100%", tickSize, base.Regular) + gap
	right := math.Max(width-pad, left+1)
	plotH := bottom - plotTop
	for k := 0; k <= marimekkoQuarters; k++ {
		frac := float64(k) / marimekkoQuarters
		marks = append(marks, text(left-gap, bottom-float64(frac*plotH), fmt.Sprintf("%d%%", 100*k/marimekkoQuarters),
			tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
	}
	x := left
	for i, cat := range c.Categories {
		colW := 0.0
		if grand > 0 {
			colW = (right - left) * totals[i] / grand
		}
		if totals[i] > 0 {
			y := bottom
			for j, s := range c.Series {
				v := s.Values[i]
				h := plotH * v / totals[i]
				if v > 0 {
					// Half the 2px surface gap on every side.
					marks = append(marks, scene.Mark{
						Shape: scene.RectShape{Rect: scene.Rect{
							X: x + 1,
							Y: y - h + 1,
							W: math.Max(colW-2, 0),
							H: math.Max(h-2, 0),
						}},
						Fill:   base.Series(j),
						Series: s.Name,
						Hit: &scene.Hit{
							Key:   fmt.Sprintf("%d-%d", i, j),
							Label: cat,
							Rows: []scene.TipRow{
								{Color: base.Series(j), Name: s.Name, Value: base.Value(v)},
								{Name: "share of " + cat, Value: base.Approx(v/totals[i]*100) + "%"},
								{Name: "share of total", Value: base.Approx(v/grand*100) + "%"},
							},
						},
					})
				}
				y -= h
			}
			// A label too wide for its column is left to the tooltip.
			if m.Width(cat, tickSize, base.Regular) <= colW-4 {
				marks = append(marks, text(x+colW/2, bottom+gap, cat, tickSize, base.Regular, scene.Middle, scene.Top, base.TextSecondary))
			}
		}
		x += colW
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Marimekko chart of %s across %s.",
			count(len(c.Series), "series", "series"), count(len(c.Categories), "category", "categories"))),
		Marks: marks,
	}, nil
}
