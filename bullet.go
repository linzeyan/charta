package charta

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	bulletPaddingInner = 0.3
	bulletPaddingOuter = 0.1
	// measureRatio: the measure bar is a third of the range band's thickness.
	measureRatio = 1.0 / 3.0
	// targetRatio is the share of the range band the target marker spans.
	targetRatio = 0.75
	// tickGap is the space between a gauge and its tick labels.
	tickGap = 4.0
)

// Bullet is one measure against its target and qualitative ranges. Ranges
// are the upper bounds of up to three ranges (e.g. poor, fair, good), each
// starting at zero.
type Bullet struct {
	Label  string    `json:"label"`
	Value  float64   `json:"value"`
	Target *float64  `json:"target,omitempty"`
	Ranges []float64 `json:"ranges,omitempty"`
}

// BulletChart draws bullet graphs: compact horizontal gauges, one per
// measure. Measures rarely share units, so each gauge has its own scale and
// tick labels.
type BulletChart struct {
	Title string   `json:"title,omitempty"`
	Data  []Bullet `json:"data"`
}

func (c BulletChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	type domain struct{ lo, hi float64 }
	domains := make([]domain, 0, len(c.Data))
	for _, d := range c.Data {
		if err := finite(fmt.Sprintf("%q value", d.Label), d.Value); err != nil {
			return nil, err
		}
		lo, hi := math.Min(d.Value, 0), math.Max(d.Value, 0)
		if d.Target != nil {
			t := *d.Target
			if err := finite(fmt.Sprintf("%q target", d.Label), t); err != nil {
				return nil, err
			}
			lo, hi = math.Min(lo, t), math.Max(hi, t)
		}
		if len(d.Ranges) > base.BandClasses {
			return nil, invalid("%q has %d ranges, at most %d", d.Label, len(d.Ranges), base.BandClasses)
		}
		for _, r := range d.Ranges {
			if !(isFinite(r) && r >= 0) {
				return nil, invalid("%q ranges must be non-negative numbers", d.Label)
			}
			hi = math.Max(hi, r)
		}
		domains = append(domains, domain{lo, hi})
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	labels := make([]string, len(c.Data))
	for i, d := range c.Data {
		labels[i] = d.Label
	}
	left := pad + maxWidth(m, labels) + gap
	right := math.Max(width-pad, left+1)
	bottom := math.Max(height-pad, top+1)
	band := base.NewBandScale(len(c.Data), top, bottom, bulletPaddingInner, bulletPaddingOuter)
	bw := band.Bandwidth()
	thick := math.Max(math.Min(barMax, bw-tickGap-tickSize), 1)
	ticks := max(int(math.Round((right-left)/xTickSpacing)), 2)
	axes := make([]valueAxis, len(domains))
	for i, d := range domains {
		axes[i] = newValueAxis(d.lo, d.hi, left, right, ticks)
	}
	// Every gauge ends at the same x: pull them all in by the widest last
	// tick label, which is centered on the end.
	last := 0.0
	for _, a := range axes {
		last = math.Max(last, a.lastLabelWidth(m))
	}
	right = math.Max(right-last/2, left+1)
	for i := range axes {
		axes[i].scale.R1 = right
	}

	for i, d := range c.Data {
		axis := axes[i]
		y := band.Position(i)
		cy := y + thick/2
		x0 := axis.scale.Map(0)
		ranges := slices.Clone(d.Ranges)
		slices.Sort(ranges)
		// Widest range first and lightest; narrower ones darker on top.
		offset := base.BandClasses - len(ranges)
		for k := len(ranges) - 1; k >= 0; k-- {
			marks = append(marks, scene.Mark{
				Shape: scene.RectShape{Rect: scene.Rect{X: x0, Y: y, W: axis.scale.Map(ranges[k]) - x0, H: thick}},
				Fill:  base.Band(offset + k),
			})
		}
		mh := thick * measureRatio
		x1 := axis.scale.Map(d.Value)
		length := math.Abs(x1 - x0)
		r := math.Min(math.Min(barRadius, mh/2), length)
		radius := scene.Corners{TR: r, BR: r}
		if d.Value < 0 {
			radius = scene.Corners{TL: r, BL: r}
		}
		label := d.Label
		if d.Target != nil {
			label = d.Label + " · target " + base.Value(*d.Target)
		}
		marks = append(marks, scene.Mark{
			Shape: scene.RectShape{Rect: scene.Rect{X: math.Min(x0, x1), Y: cy - mh/2, W: length, H: mh}, Radius: radius},
			Fill:  base.Series(0),
			Hit: &scene.Hit{
				Key:   strconv.Itoa(i),
				Label: label,
				Value: base.Value(d.Value),
				Area:  &scene.Rect{X: left, Y: y - (band.Step()-bw)/2, W: right - left, H: band.Step()},
			},
		})
		if d.Target != nil {
			x := axis.scale.Map(*d.Target)
			h := thick * targetRatio
			marks = append(marks, scene.Mark{
				Shape:  scene.LineShape{X1: x, Y1: cy - h/2, X2: x, Y2: cy + h/2},
				Stroke: &scene.Stroke{Color: base.TextPrimary, Width: lineWidth},
			})
		}
		marks = append(marks, text(left-gap, cy, d.Label, tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
		for _, t := range axis.ticks {
			marks = append(marks, text(axis.scale.Map(t), y+thick+tickGap, axis.label(t), tickSize, base.Regular, scene.Middle, scene.Top, base.TextMuted))
		}
	}

	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Bullet graph with %s.", count(len(c.Data), "measure", "measures"))),
		Marks:  marks,
	}, nil
}
