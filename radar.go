package charta

import (
	"fmt"
	"math"
	"slices"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// maxRadarSeries: overlapping translucent polygons turn to mud past three.
const maxRadarSeries = 3

// RadarChart puts several measures on spokes sharing one scale from the
// center, each series a closed polygon: profiles compared at a glance.
type RadarChart struct {
	Title  string   `json:"title,omitempty"`
	Axes   []string `json:"axes"`
	Series []Series `json:"series"`
}

func polygon(points []scene.Point) []scene.PathOp {
	ops := make([]scene.PathOp, 0, len(points)+1)
	for k, p := range points {
		if k == 0 {
			ops = append(ops, scene.Move(p.X, p.Y))
		} else {
			ops = append(ops, scene.Line(p.X, p.Y))
		}
	}
	return append(ops, scene.Close())
}

func (c RadarChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if len(c.Axes) < 3 {
		return nil, invalid("%d axes: a radar needs at least 3 (use a bar chart for fewer)", len(c.Axes))
	}
	if err := checkSeries(c.Axes, c.Series, maxRadarSeries); err != nil {
		return nil, err
	}
	for _, s := range c.Series {
		if slices.ContainsFunc(s.Values, func(v float64) bool { return v < 0 }) {
			return nil, invalid("series %q has a negative value; spokes start at zero", s.Name)
		}
	}
	hi := 0.0
	for _, s := range c.Series {
		for _, v := range s.Values {
			hi = math.Max(hi, v)
		}
	}
	topV, ticks := radialTicks(hi)

	names := make([]string, len(c.Series))
	for i, s := range c.Series {
		names[i] = s.Name
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, names, keyLine)
	bottom := height - pad
	roomX, roomY := labelRoom(m, c.Axes)
	cx, cy := width/2, (top+bottom)/2
	radius := polarRadius(width, top, bottom, roomX, roomY)
	n := len(c.Axes)
	angle := func(k int) float64 { return tau * float64(k) / float64(n) }
	point := func(k int, v float64) scene.Point {
		x, y := polarAt(cx, cy, radius*v/topV, angle(k))
		return scene.Point{X: x, Y: y}
	}

	// Rings are polygons, not circles: values between spokes are not
	// defined, and a circle would suggest they are.
	for _, t := range ticks {
		ring := make([]scene.Point, n)
		for k := range ring {
			ring[k] = point(k, t)
		}
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: polygon(ring)},
			Stroke: &scene.Stroke{Color: base.Grid, Width: hairline},
		})
	}
	for k := range n {
		p := point(k, topV)
		marks = append(marks, scene.Mark{
			Shape:  scene.LineShape{X1: cx, Y1: cy, X2: p.X, Y2: p.Y},
			Stroke: &scene.Stroke{Color: base.Grid, Width: hairline},
		})
	}
	for j, s := range c.Series {
		pts := make([]scene.Point, n)
		for k := range pts {
			pts[k] = point(k, s.Values[k])
		}
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: polygon(pts)},
			Fill:   base.Wash(j),
			Stroke: &scene.Stroke{Color: base.Series(j), Width: lineWidth},
			Series: s.Name,
		})
	}
	// Dots above every polygon, so each stays reachable.
	for j, s := range c.Series {
		for k := range n {
			p := point(k, s.Values[k])
			marks = append(marks, scene.Mark{
				Shape:  scene.CircleShape{CX: p.X, CY: p.Y, R: dotRadius},
				Fill:   base.Series(j),
				Stroke: &scene.Stroke{Color: base.Surface, Width: surfaceGap},
				Series: s.Name,
				Hit: &scene.Hit{
					Key:   fmt.Sprintf("%d-%d", j, k),
					Label: c.Axes[k],
					Rows:  []scene.TipRow{{Color: base.Series(j), Name: s.Name, Value: base.Value(s.Values[k])}},
					Area:  &scene.Rect{X: p.X - minTarget/2, Y: p.Y - minTarget/2, W: minTarget, H: minTarget},
				},
			})
		}
	}
	// The outermost ring's label would sit on the first axis's name; the
	// rings inside it give the step.
	for _, t := range ticks {
		if t < topV {
			tickLabel(&marks, m, cx+gap/2, point(0, t).Y, scene.Start, t)
		}
	}
	for k, a := range c.Axes {
		x, y, anchor, baseline := around(cx, cy, radius+gap/2, angle(k))
		marks = append(marks, text(x, y, a, tickSize, base.Regular, anchor, baseline, base.TextSecondary))
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Radar chart of %s on %s.",
			count(len(c.Series), "series", "series"), count(n, "axis", "axes"))),
		Marks: marks,
	}, nil
}
