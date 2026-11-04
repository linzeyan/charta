package charta

import (
	"fmt"
	"math"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// GroupedBarChart draws several series side by side in each category
// (multi-set bar chart).
type GroupedBarChart struct {
	Title      string   `json:"title,omitempty"`
	Categories []string `json:"categories"`
	Series     []Series `json:"series"`
}

func (c GroupedBarChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if err := checkSeries(c.Categories, c.Series, maxSeries); err != nil {
		return nil, err
	}
	names := make([]string, len(c.Series))
	lo, hi := 0.0, 0.0
	for j, s := range c.Series {
		names[j] = s.Name
		for _, v := range s.Values {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, names, keySwatch)
	f := newCategoryFrame(m, width, height, top, c.Categories, lo, hi, paddingInner, paddingOuter, 0)
	f.grid(&marks)
	f.yLabels(&marks)

	k := len(c.Series)
	fk := float64(k)
	bw := f.band.Bandwidth()
	// Bars in a group touch, separated only by the surface gap.
	thick := 0.0
	if k > 0 {
		thick = math.Max(math.Min(barMax, (bw-float64(surfaceGap*(fk-1)))/fk), 1)
	}
	group := float64(fk*thick) + float64(math.Max(fk-1, 0)*surfaceGap)
	y0 := f.y.scale.Map(0)
	for i, cat := range c.Categories {
		col := f.column(i)
		slice := col.W / float64(max(k, 1))
		for j, s := range c.Series {
			v := s.Values[i]
			y1 := f.y.scale.Map(v)
			length := math.Abs(y1 - y0)
			// The group's column, split evenly: no dead zones between bars.
			area := col
			area.X = col.X + float64(float64(j)*slice)
			area.W = slice
			marks = append(marks, scene.Mark{
				Shape: scene.RectShape{
					Rect: scene.Rect{
						X: f.band.Position(i) + (bw-group)/2 + float64(float64(j)*(thick+surfaceGap)),
						Y: math.Min(y0, y1),
						W: thick,
						H: length,
					},
					Radius: dataEnd(v, thick, length),
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
		Label:  describe(c.Title, fmt.Sprintf("Grouped bar chart with %s and %s.", count(k, "series", "series"), count(len(c.Categories), "category", "categories"))),
		Marks:  marks,
	}, nil
}
