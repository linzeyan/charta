package charta

import (
	"fmt"
	"math"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// PyramidSide is one side of a population pyramid: a name and a value per
// group.
type PyramidSide struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
}

// PyramidChart draws two distributions back to back: groups as rows (top to
// bottom), one side growing left and the other right from a shared center
// gutter that holds the group labels.
type PyramidChart struct {
	Title      string      `json:"title,omitempty"`
	Categories []string    `json:"categories"`
	Left       PyramidSide `json:"left"`
	Right      PyramidSide `json:"right"`
}

func (c PyramidChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if c.Left.Name == c.Right.Name {
		return nil, invalid("the two sides need different names")
	}
	sides := [2]PyramidSide{c.Left, c.Right}
	maxV := 0.0
	for _, side := range sides {
		if len(side.Values) != len(c.Categories) {
			return nil, invalid("side %q has %d values for %d groups", side.Name, len(side.Values), len(c.Categories))
		}
		for i, v := range side.Values {
			if !(isFinite(v) && v >= 0) {
				return nil, invalid("side %q at %q must be a non-negative number", side.Name, c.Categories[i])
			}
		}
	}
	for _, side := range sides {
		for _, v := range side.Values {
			maxV = math.Max(maxV, v)
		}
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, []string{c.Left.Name, c.Right.Name}, keySwatch)
	labelW := maxWidth(m, c.Categories)
	mid := width / 2
	gutter := labelW + 2*gap
	zeroL, zeroR := mid-gutter/2, mid+gutter/2
	half := math.Max(zeroL-pad, 1)
	countX := max(int(math.Round(half/xTickSpacing)), 2)
	axis := newValueAxis(0, maxV, 0, half, countX)
	// The outermost tick labels are centered on the plot edges.
	last := axis.lastLabelWidth(m)
	axis.scale.R1 = math.Max(half-last/2, 1)
	reach := axis.scale.R1
	bottom := math.Max(height-pad-tickSize-gap, top+1)
	band := base.NewBandScale(len(c.Categories), top, bottom, paddingInner, paddingOuter)

	for _, t := range axis.ticks {
		if t != 0 {
			d := axis.scale.Map(t)
			marks = append(marks, vline(snap(zeroL-d), top, bottom, base.Grid), vline(snap(zeroR+d), top, bottom, base.Grid))
		}
	}
	bw := band.Bandwidth()
	thick := math.Min(barMax, bw)
	for i, cat := range c.Categories {
		y := band.Position(i) + (bw-thick)/2
		rowY := band.Position(i) - (band.Step()-bw)/2
		for j, side := range sides {
			v := side.Values[i]
			length := axis.scale.Map(v)
			r := math.Min(math.Min(barRadius, thick/2), length)
			// Rounded at the outer (data) end; square at the gutter.
			x, radius, areaX := zeroR, scene.Corners{TR: r, BR: r}, zeroR
			if j == 0 {
				x, radius, areaX = zeroL-length, scene.Corners{TL: r, BL: r}, zeroL-reach
			}
			marks = append(marks, scene.Mark{
				Shape:  scene.RectShape{Rect: scene.Rect{X: x, Y: y, W: length, H: thick}, Radius: radius},
				Fill:   base.Series(j),
				Series: side.Name,
				Hit: &scene.Hit{
					Key:   fmt.Sprintf("%d-%d", i, j),
					Label: side.Name + " · " + cat,
					Value: base.Value(v),
					Area:  &scene.Rect{X: areaX, Y: rowY, W: reach, H: band.Step()},
				},
			})
		}
	}
	marks = append(marks, vline(snap(zeroL), top, bottom, base.Axis), vline(snap(zeroR), top, bottom, base.Axis))
	for i, cat := range c.Categories {
		marks = append(marks, text(mid, band.Position(i)+bw/2, cat, tickSize, base.Regular, scene.Middle, scene.Center, base.TextMuted))
	}
	for _, t := range axis.ticks {
		d := axis.scale.Map(t)
		for _, x := range [2]float64{zeroL - d, zeroR + d} {
			marks = append(marks, text(x, bottom+gap, axis.label(t), tickSize, base.Regular, scene.Middle, scene.Top, base.TextMuted))
		}
	}

	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Population pyramid of %s and %s across %s.",
			c.Left.Name, c.Right.Name, count(len(c.Categories), "group", "groups"))),
		Marks: marks,
	}, nil
}
