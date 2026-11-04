package charta

import (
	"fmt"
	"math"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/geo"
	"github.com/linzeyan/charta/scene"
)

// Region is a value for the region with this id.
type Region struct {
	ID    string  `json:"id"`
	Value float64 `json:"value"`
}

// ChoroplethChart shades regions by value on the sequential ramp; regions
// without a value stay neutral. The regions are Features, or a registered
// Basemap ("world" once geo/world is imported), not both.
type ChoroplethChart struct {
	Title      string         `json:"title,omitempty"`
	Features   []geo.Feature  `json:"features,omitempty"`
	Basemap    string         `json:"basemap,omitempty"`
	Projection geo.Projection `json:"projection"`
	Values     []Region       `json:"values"`
}

func (c ChoroplethChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	regs, err := regions(c.Features, c.Basemap, c.Projection)
	if err != nil {
		return nil, err
	}
	value := make([]*float64, len(regs))
	for k, r := range c.Values {
		if err := finite(fmt.Sprintf("the value for %q", r.ID), r.Value); err != nil {
			return nil, err
		}
		for _, o := range c.Values[:k] {
			if o.ID == r.ID {
				return nil, invalid("region %q has two values", r.ID)
			}
		}
		// An unmatched id is almost always a code mismatch (ISO 2 vs 3
		// letters, a typo): silently dropping it would hide data.
		i := -1
		for j, f := range regs {
			if f.ID == r.ID {
				i = j
				break
			}
		}
		if i < 0 {
			return nil, invalid("no region has id %q", r.ID)
		}
		v := r.Value
		value[i] = &v
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, r := range c.Values {
		lo, hi = math.Min(lo, r.Value), math.Max(hi, r.Value)
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	if len(c.Values) > 0 {
		top = seqLegend(&marks, m, top, lo, hi)
	}
	fit := newMapFit(c.Projection, regs, nil, pad, top, width-pad, height-pad)
	for i, f := range regs {
		ops := fit.region(f.Geometry)
		if len(ops) == 0 {
			continue
		}
		fill, shown := base.Grid, "no data"
		if v := value[i]; v != nil {
			fill, shown = base.Seq(seqClass(*v, lo, hi)), base.Value(*v)
		}
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: ops},
			Fill:   fill,
			Stroke: &scene.Stroke{Color: base.Surface, Width: mapBorder},
			Hit:    &scene.Hit{Key: fmt.Sprintf("region-%d", i), Label: f.Name, Value: shown},
		})
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Choropleth map of %s, %d with data.", count(len(regs), "region", "regions"), len(c.Values))),
		Marks:  marks,
	}, nil
}
