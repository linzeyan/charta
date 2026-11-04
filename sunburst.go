package charta

import (
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const sunburstTotalSize = 18.0

// SunburstChart shows a hierarchy as rings: the root's children around the
// center, each level outward, every node's angle proportional to its value.
type SunburstChart struct {
	Title string `json:"title,omitempty"`
	Root  Node   `json:"root"`
}

func (c SunburstChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	// Data order is kept: around a circle, the order often means something
	// (months, stages) that sorting by size would scramble.
	flat, err := flatten(&c.Root, true)
	if err != nil {
		return nil, err
	}
	t := &flat.tree
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, flat.legend(), keySwatch)
	bottom := height - pad
	cx, cy := width/2, (top+bottom)/2
	radius := math.Max(math.Min(width-2*pad, bottom-top)/2, 1)
	depth := flat.maxDepth()
	// The center (the root's ring) is a hole as wide as one ring.
	ring := radius / float64(depth+1)
	total := t.Value[0]

	type span struct{ a0, a1 float64 }
	spans := make([]span, t.Len())
	for i := range spans {
		spans[i] = span{0, tau}
	}
	for i := range t.Len() {
		a0, a1 := spans[i].a0, spans[i].a1
		a := a0
		for _, ch := range t.Children[i] {
			w := 0.0
			if t.Value[i] > 0 {
				w = (a1 - a0) * t.Value[ch] / t.Value[i]
			}
			spans[ch] = span{a, a + w}
			a += w
		}
		if i == 0 || t.Value[i] <= 0 {
			continue
		}
		d := float64(t.Depth[i])
		col := flat.color(i)
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: sector(cx, cy, d*ring, (d+1)*ring, a0, a1)},
			Fill:   base.Series(col),
			Stroke: &scene.Stroke{Color: base.Surface, Width: surfaceGap},
			Series: flat.series(i),
			Hit: &scene.Hit{
				Key:   strconv.Itoa(i),
				Label: flat.path[i],
				Value: base.Value(t.Value[i]),
				Rows:  []scene.TipRow{{Name: "share", Value: base.Approx(t.Value[i]/total*100) + "%"}},
			},
		})
	}
	if len(c.Root.Children) > 0 {
		label := base.Value(total)
		if m.Width(label, sunburstTotalSize, base.Semibold) <= 1.6*ring {
			name := c.Root.Name
			if name == "" {
				name = "Total"
			}
			marks = append(marks, text(cx, cy, label, sunburstTotalSize, base.Semibold, scene.Middle, scene.Bottom, base.TextPrimary))
			marks = append(marks, text(cx, cy+gap/2, name, tickSize, base.Regular, scene.Middle, scene.Top, base.TextMuted))
		}
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Sunburst of %s in %s.",
			count(flat.leaves(), "item", "items"), count(depth, "level", "levels"))),
		Marks: marks,
	}, nil
}
