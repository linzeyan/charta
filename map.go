package charta

import (
	"math"
	"slices"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/geo"
	"github.com/linzeyan/charta/scene"
)

// Pieces shared by the map charts: the basemap, fitting a projection to the
// plot, and regions as paths. Mirrors the Rust crate's chart/map.rs.

// mapBorder is the country border width: thinner than a hairline, at world
// scale a full pixel turns small countries into blobs of outline.
const mapBorder = 0.5

// regions are the regions to draw: the chart's own, or a named basemap (JSON
// specs say "basemap": "world"), not both. The projection is checked here
// too, since every map chart starts by resolving its regions.
func regions(own []geo.Feature, basemap string, proj geo.Projection) ([]geo.Feature, error) {
	if !proj.Valid() {
		return nil, invalid("unknown projection kind %d", int(proj.Kind))
	}
	if basemap == "" {
		return own, nil
	}
	if len(own) > 0 {
		return nil, invalid("give either features or a basemap, not both")
	}
	if f, ok := geo.Basemap(basemap); ok {
		return f, nil
	}
	if basemap == "world" {
		return nil, invalid(`basemap "world" needs import _ "github.com/linzeyan/charta/geo/world"`)
	}
	return nil, invalid(`unknown basemap %q; the built-in one is "world"`, basemap)
}

// checkLonLat: longitude and latitude in degrees, in range.
func checkLonLat(what string, lon, lat float64) error {
	if err := finite(what+"'s longitude", lon); err != nil {
		return err
	}
	if err := finite(what+"'s latitude", lat); err != nil {
		return err
	}
	if !(lon >= -180 && lon <= 180) || !(lat >= -90 && lat <= 90) {
		return invalid("%s at (%s, %s) is off the globe: longitude runs −180 to 180, latitude −90 to 90",
			what, px(lon), px(lat))
	}
	return nil
}

// mapFit is a projection scaled and centered into the plot box.
type mapFit struct {
	proj      geo.Projection
	k, ox, oy float64
}

// newMapFit fits the projected bounds of regions and points (lon, lat) into
// (left, top, right, bottom), keeping the aspect ratio.
func newMapFit(proj geo.Projection, regions []geo.Feature, points []geo.Position, left, top, right, bottom float64) mapFit {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	grow := func(p geo.Position) {
		x, y := proj.Project(p[0], p[1])
		x0 = math.Min(x0, x)
		y0 = math.Min(y0, y)
		x1 = math.Max(x1, x)
		y1 = math.Max(y1, y)
	}
	for _, f := range regions {
		for _, poly := range f.Geometry.Polygons {
			// Holes lie inside their exterior.
			if len(poly) == 0 {
				continue
			}
			for _, p := range poly[0] {
				grow(p)
			}
		}
	}
	for _, p := range points {
		grow(p)
	}
	if x0 > x1 {
		x0, y0, x1, y1 = 0, 0, 0, 0
	}
	w, h := math.Max(right-left, 1), math.Max(bottom-top, 1)
	bw, bh := x1-x0, y1-y0
	k := 1.0
	if bw > 0 || bh > 0 {
		kw, kh := math.Inf(1), math.Inf(1)
		if bw > 0 {
			kw = w / bw
		}
		if bh > 0 {
			kh = h / bh
		}
		k = math.Min(kw, kh)
	}
	return mapFit{
		proj: proj,
		k:    k,
		ox:   (left+right)/2 - float64((x0+x1)/2*k),
		oy:   (top+bottom)/2 - float64((y0+y1)/2*k),
	}
}

func (f mapFit) xy(lon, lat float64) (float64, float64) {
	x, y := f.proj.Project(lon, lat)
	return f.ox + float64(x*f.k), f.oy + float64(y*f.k)
}

// region is a geometry as one path, each polygon's exterior wound one way
// and its holes the other, whatever the input's winding: the nonzero fill
// rule then leaves the holes empty.
func (f mapFit) region(g geo.Geometry) []scene.PathOp {
	var ops []scene.PathOp
	for _, poly := range g.Polygons {
		for r, ring := range poly {
			if len(ring) < 3 {
				continue
			}
			pts := make([]scene.Point, len(ring))
			for i, p := range ring {
				pts[i].X, pts[i].Y = f.xy(p[0], p[1])
			}
			area := 0.0
			for i := range pts {
				a, b := pts[i], pts[(i+1)%len(pts)]
				area += float64(a.X*b.Y) - float64(b.X*a.Y)
			}
			if (area < 0) == (r == 0) {
				slices.Reverse(pts)
			}
			ops = append(ops, scene.Move(pts[0].X, pts[0].Y))
			for _, p := range pts[1:] {
				ops = append(ops, scene.Line(p.X, p.Y))
			}
			ops = append(ops, scene.Close())
		}
	}
	return ops
}

// land draws the regions as a quiet backdrop for marks placed on the map.
func land(marks *[]scene.Mark, fit mapFit, regions []geo.Feature) {
	for _, f := range regions {
		if ops := fit.region(f.Geometry); len(ops) > 0 {
			*marks = append(*marks, scene.Mark{
				Shape:  scene.PathShape{Ops: ops},
				Fill:   base.Grid,
				Stroke: &scene.Stroke{Color: base.Surface, Width: mapBorder},
			})
		}
	}
}
