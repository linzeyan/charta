package charta

import (
	"fmt"
	"math"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/layout"
	"github.com/linzeyan/charta/scene"
)

const (
	sankeyNodeW = 12.0
	// sankeyNodeGap is the room between nodes in a column: more than a
	// label's height, so labels of neighboring thin nodes cannot collide.
	sankeyNodeGap = 16.0
)

// SankeyChart shows flows between stages: each node a bar as tall as what
// passes through it, each link a band as thick as its value.
type SankeyChart struct {
	Title string `json:"title,omitempty"`
	Links []Link `json:"links"`
}

func (c SankeyChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	g, err := graph(c.Links, true)
	if err != nil {
		return nil, err
	}
	sc := &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Sankey diagram of %s between %s.",
			count(len(g.edges), "flow", "flows"), count(len(g.names), "node", "nodes"))),
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, g.legend(), keySwatch)
	if len(g.edges) == 0 {
		sc.Marks = marks
		return sc, nil
	}
	bottom := math.Max(height-pad, top+1)
	s, cycle, ok := layout.Sankey(len(g.names), g.edges, top, bottom, sankeyNodeGap)
	if !ok {
		return nil, invalid("links loop back to %q: a Sankey diagram's flows only move forward", g.names[cycle])
	}

	// The first column's labels go left of it and the last column's right of
	// it, outside the flows; the rest sit beside their bar.
	last := s.Columns - 1
	labelW := func(col int) float64 {
		w := 0.0
		for i, name := range g.names {
			if s.Layer[i] == col {
				w = math.Max(w, m.Width(name, tickSize, base.Regular))
			}
		}
		return w
	}
	left := pad + labelW(0) + gap/2
	right := math.Max(width-pad-labelW(last)-gap/2, left+sankeyNodeW)
	kx := 0.0
	if last > 0 {
		kx = (right - left - sankeyNodeW) / float64(last)
	}
	nodeX := func(i int) float64 { return left + float64(float64(s.Layer[i])*kx) }

	out := make([]float64, len(g.names))
	inc := make([]float64, len(g.names))
	for _, e := range g.edges {
		out[e.S] += e.V
		inc[e.T] += e.V
	}
	for k, e := range g.edges {
		// Hairline flows still draw: a missing link reads as no flow.
		w := math.Max(s.Width[k], 1)
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: flowBand(nodeX(e.S)+sankeyNodeW, s.SourceY[k]-w/2, nodeX(e.T), s.TargetY[k]-w/2, w)},
			Fill:   base.Link(g.color(e.S)),
			Series: g.series(e.S),
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("link-%d", k),
				Label: g.names[e.S] + " → " + g.names[e.T],
				Value: base.Value(e.V),
				Rows: []scene.TipRow{{
					Name:  "of " + g.names[e.S] + "'s outflow",
					Value: base.Approx(e.V/out[e.S]*100) + "%",
				}},
			},
		})
	}

	for i, name := range g.names {
		x, y0 := nodeX(i), s.Y0[i]
		h := math.Max(s.Y1[i]-y0, 1)
		var rows []scene.TipRow
		if inc[i] > 0 && out[i] > 0 {
			rows = []scene.TipRow{{Name: "in", Value: base.Value(inc[i])}, {Name: "out", Value: base.Value(out[i])}}
		}
		cy := y0 + h/2
		targetH := math.Max(h, minTarget)
		marks = append(marks, scene.Mark{
			Shape:  scene.RectShape{Rect: scene.Rect{X: x, Y: y0, W: sankeyNodeW, H: h}},
			Fill:   base.Series(g.color(i)),
			Series: g.series(i),
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("node-%d", i),
				Label: name,
				Value: base.Value(s.Value[i]),
				Rows:  rows,
				Area:  &scene.Rect{X: x, Y: cy - targetH/2, W: sankeyNodeW, H: targetH},
			},
		})
	}
	// Labels last, over every flow.
	for i, name := range g.names {
		cy := (s.Y0[i] + s.Y1[i]) / 2
		switch s.Layer[i] {
		case 0:
			marks = append(marks, text(nodeX(i)-gap/2, cy, name, tickSize, base.Regular, scene.End, scene.Center, base.TextPrimary))
		case last:
			marks = append(marks, text(nodeX(i)+sankeyNodeW+gap/2, cy, name, tickSize, base.Regular, scene.Start, scene.Center, base.TextPrimary))
		default:
			haloLabel(&marks, m, nodeX(i)+sankeyNodeW+gap/2, cy, name, scene.Start, base.TextPrimary)
		}
	}
	sc.Marks = marks
	return sc, nil
}
