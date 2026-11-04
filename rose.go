package charta

import (
	"fmt"
	"math"
	"slices"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// RoseChart shows categories as equal wedges around a circle (a Nightingale
// rose or coxcomb), each wedge's area proportional to its value; several
// series stack outward.
type RoseChart struct {
	Title      string   `json:"title,omitempty"`
	Categories []string `json:"categories"`
	Series     []Series `json:"series"`
}

func (c RoseChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if err := checkSeries(c.Categories, c.Series, maxSeries); err != nil {
		return nil, err
	}
	for _, s := range c.Series {
		if slices.ContainsFunc(s.Values, func(v float64) bool { return v < 0 }) {
			return nil, invalid("series %q has a negative value; wedges cannot be", s.Name)
		}
	}
	n := len(c.Categories)
	hi := 0.0
	for i := range n {
		sum := 0.0
		for _, s := range c.Series {
			sum += s.Values[i]
		}
		hi = math.Max(hi, sum)
	}
	topV, ticks := radialTicks(hi)

	names := make([]string, len(c.Series))
	for i, s := range c.Series {
		names[i] = s.Name
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, names, keySwatch)
	bottom := height - pad
	roomX, roomY := labelRoom(m, c.Categories)
	cx, cy := width/2, (top+bottom)/2
	radius := polarRadius(width, top, bottom, roomX, roomY)
	// Area, not radius, carries the value: a wedge twice the value is √2
	// times as long, as Nightingale drew them.
	rad := func(v float64) float64 { return radius * math.Sqrt(v/topV) }

	for _, t := range ticks {
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: circle(cx, cy, rad(t))},
			Stroke: &scene.Stroke{Color: base.Grid, Width: hairline},
		})
	}
	step := tau / float64(max(n, 1))
	stacked := len(c.Series) > 1
	for i := range n {
		cum := 0.0
		for j, s := range c.Series {
			v := s.Values[i]
			r0, r1 := rad(cum), rad(cum+v)
			cum += v
			if v == 0 {
				continue
			}
			cat := c.Categories[i]
			hit := &scene.Hit{Key: fmt.Sprintf("%d-%d", i, j), Label: cat, Value: base.Value(v)}
			if stacked {
				hit.Value = ""
				hit.Rows = []scene.TipRow{{Color: base.Series(j), Name: s.Name, Value: base.Value(v)}}
			}
			marks = append(marks, scene.Mark{
				Shape:  scene.PathShape{Ops: sector(cx, cy, r0, r1, float64(i)*step, float64(i+1)*step)},
				Fill:   base.Series(j),
				Stroke: &scene.Stroke{Color: base.Surface, Width: surfaceGap},
				Series: s.Name,
				Hit:    hit,
			})
		}
	}
	for _, t := range ticks {
		tickLabel(&marks, m, cx+gap/2, cy-rad(t), scene.Start, t)
	}
	for i, cat := range c.Categories {
		x, y, anchor, baseline := around(cx, cy, radius+gap/2, (float64(i)+0.5)*step)
		marks = append(marks, text(x, y, cat, tickSize, base.Regular, anchor, baseline, base.TextSecondary))
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Nightingale rose chart of %s across %s.",
			count(len(c.Series), "series", "series"), count(n, "category", "categories"))),
		Marks: marks,
	}, nil
}
