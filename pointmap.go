package charta

import (
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/geo"
	"github.com/linzeyan/charta/scene"
)

const (
	// mapDotR: dots on a dot map are texture, not individually read markers.
	mapDotR = 2.0
	// mapBubbleMax is smaller than the bubble chart's 24px: places on a map
	// crowd together far more than points on a scatter plot do.
	mapBubbleMax = 16.0
	mapBubbleMin = 2.0
)

// Place is a named location with a value.
type Place struct {
	Name  string  `json:"name"`
	Lon   float64 `json:"lon"`
	Lat   float64 `json:"lat"`
	Value float64 `json:"value"`
}

// Dot is one dot, optionally in a group (colored, with a legend entry); an
// empty Group means none.
type Dot struct {
	Lon   float64 `json:"lon"`
	Lat   float64 `json:"lat"`
	Group string  `json:"group,omitempty"`
}

// BubbleMapChart draws circles on a map, area proportional to each place's
// value.
type BubbleMapChart struct {
	Title      string         `json:"title,omitempty"`
	Features   []geo.Feature  `json:"features,omitempty"`
	Basemap    string         `json:"basemap,omitempty"`
	Projection geo.Projection `json:"projection"`
	Places     []Place        `json:"places"`
}

// DotMapChart draws one small dot per item at its location: density shows
// as clusters.
type DotMapChart struct {
	Title      string         `json:"title,omitempty"`
	Features   []geo.Feature  `json:"features,omitempty"`
	Basemap    string         `json:"basemap,omitempty"`
	Projection geo.Projection `json:"projection"`
	Dots       []Dot          `json:"dots"`
}

func (c BubbleMapChart) Scene(width, height float64, _ base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	regs, err := regions(c.Features, c.Basemap, c.Projection)
	if err != nil {
		return nil, err
	}
	for _, p := range c.Places {
		if err := checkLonLat(fmt.Sprintf("%q", p.Name), p.Lon, p.Lat); err != nil {
			return nil, err
		}
		if err := finite(fmt.Sprintf("%q's value", p.Name), p.Value); err != nil {
			return nil, err
		}
		if p.Value < 0 {
			return nil, invalid("%q has a negative value; a bubble's area cannot be", p.Name)
		}
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	pts := make([]geo.Position, len(c.Places))
	for i, p := range c.Places {
		pts[i] = geo.Position{p.Lon, p.Lat}
	}
	fit := newMapFit(c.Projection, regs, pts, pad, top, width-pad, height-pad)
	land(&marks, fit, regs)
	most := 0.0
	for _, p := range c.Places {
		most = math.Max(most, p.Value)
	}
	order := make([]int, len(c.Places))
	for i := range order {
		order[i] = i
	}
	// Big bubbles first, so small ones stay visible on top.
	sort.SliceStable(order, func(a, b int) bool { return totalLess(c.Places[order[b]].Value, c.Places[order[a]].Value) })
	for _, i := range order {
		p := c.Places[i]
		cx, cy := fit.xy(p.Lon, p.Lat)
		r := 0.0
		if most > 0 {
			r = float64(mapBubbleMax * math.Sqrt(p.Value/most))
		}
		r = math.Max(r, mapBubbleMin)
		hit := &scene.Hit{Key: fmt.Sprintf("place-%d", i), Label: p.Name, Value: base.Value(p.Value)}
		if r+surfaceGap < minTarget/2 {
			hit.Area = &scene.Rect{X: cx - minTarget/2, Y: cy - minTarget/2, W: minTarget, H: minTarget}
		}
		marks = append(marks, scene.Mark{
			Shape:  scene.CircleShape{CX: cx, CY: cy, R: r},
			Fill:   base.Series(0),
			Stroke: &scene.Stroke{Color: base.Surface, Width: surfaceGap},
			Hit:    hit,
		})
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Bubble map of %s.", count(len(c.Places), "place", "places"))),
		Marks:  marks,
	}, nil
}

func (c DotMapChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	regs, err := regions(c.Features, c.Basemap, c.Projection)
	if err != nil {
		return nil, err
	}
	var groups []string
	slot := make([]int, len(c.Dots))
	for i, d := range c.Dots {
		if err := checkLonLat(fmt.Sprintf("dot %d", i), d.Lon, d.Lat); err != nil {
			return nil, err
		}
		slot[i] = -1
		if d.Group != "" {
			k := slices.Index(groups, d.Group)
			if k < 0 {
				groups = append(groups, d.Group)
				k = len(groups) - 1
			}
			slot[i] = k
		}
	}
	if len(groups) > maxSeries {
		return nil, invalid(`%d groups, at most %d: fold the rest into "Other"`, len(groups), maxSeries)
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, groups, keySwatch)
	pts := make([]geo.Position, len(c.Dots))
	for i, d := range c.Dots {
		pts[i] = geo.Position{d.Lon, d.Lat}
	}
	fit := newMapFit(c.Projection, regs, pts, pad, top, width-pad, height-pad)
	land(&marks, fit, regs)
	for i, d := range c.Dots {
		cx, cy := fit.xy(d.Lon, d.Lat)
		mk := scene.Mark{Shape: scene.CircleShape{CX: cx, CY: cy, R: mapDotR}, Fill: base.Series(max(slot[i], 0))}
		if slot[i] >= 0 {
			mk.Series = groups[slot[i]]
		}
		marks = append(marks, mk)
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Dot map of %s.", count(len(c.Dots), "dot", "dots"))),
		Marks:  marks,
	}, nil
}
