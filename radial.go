package charta

import (
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	// radialHole is the empty middle, as a share of the radius.
	radialHole = 0.3
	// ringFill is the share of each ring a bar fills.
	ringFill = 0.7
)

// Variables, not constants: Go folds constant expressions exactly, and Rust
// rounds 1.5 * PI and 1.0 - COLUMN_FILL at each step.
var (
	// radialSweep is how far the largest value sweeps, three quarters of a
	// turn: beyond that, bars on different rings stop being comparable at a
	// glance (an outer ring's arc is far longer than an inner one's for the
	// same angle), and the free quarter holds the labels.
	radialSweep = 1.5 * piF
	// columnFill is the share of each angular slot a column fills.
	columnFill = 0.8
	piF        = math.Pi
)

// RadialBarChart bends bars into concentric rings, length as the angle swept
// clockwise from twelve o'clock; the first bar is the outermost ring.
type RadialBarChart struct {
	Title string `json:"title,omitempty"`
	Data  []Bar  `json:"data"`
}

func (c RadialBarChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkRadial(c.Data, width, height); err != nil {
		return nil, err
	}
	return radialBar(c.Title, c.Data, width, height, m), nil
}

// RadialColumnChart stands columns outward from a circle, one per category
// around it, length from the inner circle proportional to value.
type RadialColumnChart struct {
	Title string `json:"title,omitempty"`
	Data  []Bar  `json:"data"`
}

func (c RadialColumnChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkRadial(c.Data, width, height); err != nil {
		return nil, err
	}
	return radialColumn(c.Title, c.Data, width, height, m), nil
}

func checkRadial(data []Bar, width, height float64) error {
	if err := checkSize(width, height); err != nil {
		return err
	}
	for i, b := range data {
		if err := finite(fmt.Sprintf("bar %d (%q)", i, b.Label), b.Value); err != nil {
			return err
		}
		if b.Value < 0 {
			return invalid("bar %d (%q) is negative; radial bars start at zero", i, b.Label)
		}
	}
	return nil
}

func maxValue(data []Bar) float64 {
	hi := 0.0
	for _, b := range data {
		hi = math.Max(hi, b.Value)
	}
	return hi
}

func radialBar(title string, data []Bar, width, height float64, m base.TextMeasure) *scene.Scene {
	topV, ticks := radialTicks(maxValue(data))
	var marks []scene.Mark
	top := drawTitle(&marks, title)
	bottom := height - pad
	tickLabels := make([]string, len(ticks))
	for i, t := range ticks {
		tickLabels[i] = base.Value(t)
	}
	roomX, roomY := labelRoom(m, tickLabels)
	cx, cy := width/2, (top+bottom)/2
	radius := polarRadius(width, top, bottom, roomX, roomY)
	inner := radius * radialHole
	ring := (radius - inner) / float64(max(len(data), 1))
	thick := math.Min(ring*ringFill, barMax)
	angle := func(v float64) float64 { return radialSweep * v / topV }

	// Spokes at each tick, with "0" at the start: the scale is the angle.
	for _, t := range append([]float64{0}, ticks...) {
		a := angle(t)
		x1, y1 := polarAt(cx, cy, inner-gap/2, a)
		x2, y2 := polarAt(cx, cy, radius+gap/2, a)
		marks = append(marks, scene.Mark{
			Shape:  scene.LineShape{X1: x1, Y1: y1, X2: x2, Y2: y2},
			Stroke: &scene.Stroke{Color: base.Grid, Width: hairline},
		})
		x, y, anchor, baseline := around(cx, cy, radius+gap, a)
		marks = append(marks, text(x, y, base.Value(t), tickSize, base.Regular, anchor, baseline, base.TextMuted))
	}
	for i, b := range data {
		mid := radius - float64((float64(i)+0.5)*ring)
		r0, r1 := mid-thick/2, mid+thick/2
		// The full-length track shows how far the bar could go; a wash of
		// the bar's hue reads as its background, not as more data.
		marks = append(marks, scene.Mark{
			Shape: scene.PathShape{Ops: sector(cx, cy, r0, r1, 0, radialSweep)},
			Fill:  base.Wash(0),
		})
		if b.Value > 0 {
			marks = append(marks, scene.Mark{
				Shape: scene.PathShape{Ops: sector(cx, cy, r0, r1, 0, angle(b.Value))},
				Fill:  base.Series(0),
				Hit:   &scene.Hit{Key: strconv.Itoa(i), Label: b.Label, Value: base.Value(b.Value)},
			})
		}
		// Labels sit in the free quarter, right-aligned to where bars start.
		marks = append(marks, text(cx-gap/2, cy-mid, b.Label, tickSize, base.Regular, scene.End, scene.Center, base.TextSecondary))
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(title, fmt.Sprintf("Radial bar chart with %s.", count(len(data), "bar", "bars"))),
		Marks:  marks,
	}
}

func radialColumn(title string, data []Bar, width, height float64, m base.TextMeasure) *scene.Scene {
	topV, ticks := radialTicks(maxValue(data))
	var marks []scene.Mark
	top := drawTitle(&marks, title)
	bottom := height - pad
	labels := make([]string, len(data))
	for i, b := range data {
		labels[i] = b.Label
	}
	roomX, roomY := labelRoom(m, labels)
	cx, cy := width/2, (top+bottom)/2
	radius := polarRadius(width, top, bottom, roomX, roomY)
	inner := radius * radialHole
	rad := func(v float64) float64 { return inner + (radius-inner)*v/topV }
	// One slot more than there are columns: the slot at twelve o'clock
	// stays empty and carries the scale.
	step := tau / float64(len(data)+1)
	padA := step * (1 - columnFill) / 2

	marks = append(marks, scene.Mark{
		Shape:  scene.PathShape{Ops: circle(cx, cy, inner)},
		Stroke: &scene.Stroke{Color: base.Axis, Width: hairline},
	})
	for _, t := range ticks {
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: circle(cx, cy, rad(t))},
			Stroke: &scene.Stroke{Color: base.Grid, Width: hairline},
		})
	}
	for i, b := range data {
		a0, a1 := float64((float64(i)+0.5)*step)+padA, float64((float64(i)+1.5)*step)-padA
		if b.Value > 0 {
			marks = append(marks, scene.Mark{
				Shape: scene.PathShape{Ops: sector(cx, cy, inner, rad(b.Value), a0, a1)},
				Fill:  base.Series(0),
				Hit:   &scene.Hit{Key: strconv.Itoa(i), Label: b.Label, Value: base.Value(b.Value)},
			})
		}
		x, y, anchor, baseline := around(cx, cy, radius+gap/2, (a0+a1)/2)
		marks = append(marks, text(x, y, b.Label, tickSize, base.Regular, anchor, baseline, base.TextSecondary))
	}
	for _, t := range ticks {
		tickLabel(&marks, m, cx, cy-rad(t), scene.Middle, t)
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(title, fmt.Sprintf("Radial column chart with %s.", count(len(data), "column", "columns"))),
		Marks:  marks,
	}
}
