// Package geo is the geographic input and the map projections of the map
// charts. It mirrors the Rust crate's geo module; keep the floating-point
// operation order identical.
//
// A Feature's geometry has the shape of a GeoJSON geometry object, so a
// GeoJSON FeatureCollection maps onto []Feature by lifting an id and a name
// out of each feature's properties. Basemaps are registered by name: import
// github.com/linzeyan/charta/geo/world for Natural Earth's 1:110m countries
// as "world".
package geo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/linzeyan/charta/internal/dmath"
)

// Feature is one region: a country, a state.
type Feature struct {
	// ID matches data to the region (ISO 3166 alpha-3 in the world basemap).
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Geometry Geometry `json:"geometry"`
}

// Position is a (longitude, latitude) pair in degrees.
type Position [2]float64

// UnmarshalJSON reads [lon, lat] and rejects any other length, as the Rust
// crate's (f64, f64) does: a third (altitude) value is refused, not dropped.
func (p *Position) UnmarshalJSON(data []byte) error {
	var v []float64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if len(v) != 2 {
		return fmt.Errorf("a position is [longitude, latitude], got %d numbers", len(v))
	}
	*p = Position{v[0], v[1]}
	return nil
}

// Polygon is rings of positions: the exterior ring, then its holes. Ring
// direction does not matter; holes are recognized by position, not winding.
type Polygon [][]Position

// Geometry is a GeoJSON Polygon (one entry in Polygons, Multi false) or
// MultiPolygon (Multi true).
type Geometry struct {
	Polygons []Polygon
	Multi    bool
}

type geometryJSON struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

func (g Geometry) MarshalJSON() ([]byte, error) {
	if g.Multi {
		return json.Marshal(struct {
			Type        string    `json:"type"`
			Coordinates []Polygon `json:"coordinates"`
		}{"MultiPolygon", g.Polygons})
	}
	if len(g.Polygons) != 1 {
		return nil, fmt.Errorf("a Polygon geometry has exactly one polygon, got %d", len(g.Polygons))
	}
	return json.Marshal(struct {
		Type        string  `json:"type"`
		Coordinates Polygon `json:"coordinates"`
	}{"Polygon", g.Polygons[0]})
}

func (g *Geometry) UnmarshalJSON(data []byte) error {
	var raw geometryJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Coordinates == nil {
		return errors.New("a geometry needs coordinates")
	}
	switch raw.Type {
	case "Polygon":
		var p Polygon
		if err := json.Unmarshal(raw.Coordinates, &p); err != nil {
			return err
		}
		*g = Geometry{Polygons: []Polygon{p}}
	case "MultiPolygon":
		var ps []Polygon
		if err := json.Unmarshal(raw.Coordinates, &ps); err != nil {
			return err
		}
		*g = Geometry{Polygons: ps, Multi: true}
	default:
		return fmt.Errorf("unknown geometry type %q: give Polygon or MultiPolygon", raw.Type)
	}
	return nil
}

// deg is π / 180 as a literal: Go evaluates math.Pi / 180 exactly before
// rounding, Rust rounds π first, and the two can differ in the last bit.
const deg = 0.017453292519943295

// mercatorLat is Web Mercator's cutoff, where the map becomes square; the
// poles are at infinity.
const mercatorLat = 85.05112877980659

// ProjectionKind says how the globe is flattened. The zero value is
// NaturalEarth, the Rust crate's default.
type ProjectionKind int

const (
	// NaturalEarth is a compromise for world maps: little distortion of
	// either kind.
	NaturalEarth ProjectionKind = iota
	// Equirectangular is longitude and latitude as x and y: simple, distorts
	// area and shape.
	Equirectangular
	// Mercator is conformal (shapes kept locally), area grossly inflated
	// toward the poles; cut off at ±85.05° latitude.
	Mercator
	// Albers is an equal-area conic for mid-latitude regions (the USA: 29.5°,
	// 45.5°, centered on −96°). Regional only: a feature straddling the
	// meridian opposite Lon0 is torn across the map.
	Albers
)

var projectionNames = [...]string{NaturalEarth: "naturalEarth", Equirectangular: "equirectangular", Mercator: "mercator", Albers: "albers"}

// Projection is a projection kind plus, for Albers, its standard parallels
// Lat1 and Lat2 and central meridian Lon0 (degrees).
type Projection struct {
	Kind             ProjectionKind
	Lat1, Lat2, Lon0 float64
}

// Valid reports whether the kind is one of the four projections.
func (p Projection) Valid() bool { return p.Kind >= 0 && int(p.Kind) < len(projectionNames) }

type albersJSON struct {
	Lat1 *float64 `json:"lat1"`
	Lat2 *float64 `json:"lat2"`
	Lon0 *float64 `json:"lon0"`
}

// MarshalJSON spells the projection the Rust serde way: a camelCase name,
// or {"albers": {"lat1": …, "lat2": …, "lon0": …}}.
func (p Projection) MarshalJSON() ([]byte, error) {
	if !p.Valid() {
		return nil, fmt.Errorf("unknown projection kind %d", int(p.Kind))
	}
	if p.Kind == Albers {
		return json.Marshal(map[string]any{"albers": map[string]float64{"lat1": p.Lat1, "lat2": p.Lat2, "lon0": p.Lon0}})
	}
	return json.Marshal(projectionNames[p.Kind])
}

func (p *Projection) UnmarshalJSON(data []byte) error {
	if t := bytes.TrimSpace(data); len(t) > 0 && t[0] == '"' {
		var s string
		if err := json.Unmarshal(t, &s); err != nil {
			return err
		}
		for k, n := range projectionNames {
			if n == s && ProjectionKind(k) != Albers {
				*p = Projection{Kind: ProjectionKind(k)}
				return nil
			}
		}
		return fmt.Errorf("unknown projection %q", s)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	raw, ok := m["albers"]
	if len(m) != 1 || !ok {
		return errors.New(`a projection is "naturalEarth", "mercator", "equirectangular" or {"albers": {…}}`)
	}
	var a albersJSON
	if err := json.Unmarshal(raw, &a); err != nil {
		return err
	}
	if a.Lat1 == nil || a.Lat2 == nil || a.Lon0 == nil {
		return errors.New("the albers projection needs lat1, lat2 and lon0")
	}
	*p = Projection{Kind: Albers, Lat1: *a.Lat1, Lat2: *a.Lat2, Lon0: *a.Lon0}
	return nil
}

// Project returns the projected point in unit-less map coordinates, y
// growing downward like the screen. It panics on an invalid kind; charts
// check Valid first.
func (p Projection) Project(lon, lat float64) (float64, float64) {
	switch p.Kind {
	case Equirectangular:
		return float64(lon * deg), float64(-lat * deg)
	case Mercator:
		phi := float64(math.Max(-mercatorLat, math.Min(lat, mercatorLat)) * deg)
		return float64(lon * deg), -dmath.Log(dmath.Tan(math.Pi/4 + phi/2))
	case NaturalEarth:
		// Šavrič et al. (2015), as d3-geo's geoNaturalEarth1Raw: polynomials
		// only, no trigonometry. Each product is rounded before the add, as
		// in Rust, instead of fused.
		l, ph := float64(lon*deg), float64(lat*deg)
		p2 := ph * ph
		p4 := p2 * p2
		x := float64(l * (0.8707 - float64(0.131979*p2) +
			float64(p4*(-0.013791+float64(p4*(float64(0.003971*p2)-float64(0.001529*p4)))))))
		y := float64(ph * (1.007226 +
			float64(p2*(0.015085+float64(p4*(-0.044475+float64(0.028874*p2)-float64(0.005916*p4)))))))
		return x, -y
	case Albers:
		// d3-geo's conicEqualAreaRaw.
		l := lon - p.Lon0
		if l > 180 {
			l -= 360
		} else if l < -180 {
			l += 360
		}
		sy0 := dmath.Sin(p.Lat1 * deg)
		n := (sy0 + dmath.Sin(p.Lat2*deg)) / 2
		c := 1 + float64(sy0*(float64(2*n)-sy0))
		r0 := math.Sqrt(c) / n
		r := math.Sqrt(c-float64(float64(2*n)*dmath.Sin(lat*deg))) / n
		sa, ca := dmath.Sincos(float64(l * deg * n))
		return float64(r * sa), -(r0 - float64(r*ca))
	}
	panic(fmt.Sprintf("unknown projection kind %d", int(p.Kind)))
}

// GreatCircle returns the points along the great circle from a to b
// (degrees), steps segments, as d3-geo's geoInterpolate: the shortest route
// on the globe.
func GreatCircle(a, b Position, steps int) []Position {
	x0, y0, x1, y1 := float64(a[0]*deg), float64(a[1]*deg), float64(b[0]*deg), float64(b[1]*deg)
	sy0, cy0 := dmath.Sincos(y0)
	sy1, cy1 := dmath.Sincos(y1)
	kx0, ky0, kx1, ky1 := cy0*dmath.Cos(x0), cy0*dmath.Sin(x0), cy1*dmath.Cos(x1), cy1*dmath.Sin(x1)
	hav := func(v float64) float64 {
		s := dmath.Sin(v / 2)
		return float64(s * s)
	}
	d := 2 * dmath.Asin(math.Sqrt(hav(y1-y0)+float64(float64(cy0*cy1)*hav(x1-x0))))
	out := make([]Position, steps+1)
	if d == 0 {
		for i := range out {
			out[i] = a
		}
		return out
	}
	k := dmath.Sin(d)
	for i := range out {
		t := float64(float64(i) / float64(steps) * d)
		bb, aa := dmath.Sin(t)/k, dmath.Sin(d-t)/k
		x := float64(aa*kx0) + float64(bb*kx1)
		y := float64(aa*ky0) + float64(bb*ky1)
		z := float64(aa*sy0) + float64(bb*sy1)
		out[i] = Position{dmath.Atan2(y, x) / deg, dmath.Atan2(z, math.Sqrt(float64(x*x)+float64(y*y))) / deg}
	}
	return out
}

var (
	basemapsMu sync.RWMutex
	basemaps   = map[string]func() []Feature{}
)

// RegisterBasemap makes a basemap available by name to the map charts'
// Basemap field. load returns the features; it is called on every use, so
// it should cache. Registering a name twice panics.
func RegisterBasemap(name string, load func() []Feature) {
	basemapsMu.Lock()
	defer basemapsMu.Unlock()
	if _, dup := basemaps[name]; dup {
		panic(fmt.Sprintf("basemap %q registered twice", name))
	}
	basemaps[name] = load
}

// Basemap returns the features of a registered basemap.
func Basemap(name string) ([]Feature, bool) {
	basemapsMu.RLock()
	load, ok := basemaps[name]
	basemapsMu.RUnlock()
	if !ok {
		return nil, false
	}
	return load(), true
}

// ParseBasemap reads the compact basemap format documented in
// docs/scripts/gen-world.py: one feature per line, "id\tname\tpolygons";
// polygons split by '|', a polygon's rings (exterior first) by ';', a ring's
// points by ' ', each point "lon,lat". Blank lines and lines starting with
// '#' are skipped.
func ParseBasemap(src string) ([]Feature, error) {
	var out []Feature
	lines := strings.Split(src, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for n, l := range lines {
		l = strings.TrimSuffix(l, "\r")
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		f := strings.SplitN(l, "\t", 3)
		if len(f) != 3 {
			return nil, fmt.Errorf("basemap line %d: want id, name and polygons", n+1)
		}
		var polys []Polygon
		for _, p := range strings.Split(f[2], "|") {
			var poly Polygon
			for _, r := range strings.Split(p, ";") {
				var ring []Position
				for _, pt := range strings.Split(r, " ") {
					xs, ys, ok := strings.Cut(pt, ",")
					if !ok {
						return nil, fmt.Errorf("basemap line %d: point %q is not lon,lat", n+1, pt)
					}
					x, err := strconv.ParseFloat(xs, 64)
					if err != nil {
						return nil, fmt.Errorf("basemap line %d: longitude: %w", n+1, err)
					}
					y, err := strconv.ParseFloat(ys, 64)
					if err != nil {
						return nil, fmt.Errorf("basemap line %d: latitude: %w", n+1, err)
					}
					ring = append(ring, Position{x, y})
				}
				poly = append(poly, ring)
			}
			polys = append(polys, poly)
		}
		out = append(out, Feature{ID: f[0], Name: f[1], Geometry: Geometry{Polygons: polys, Multi: len(polys) != 1}})
	}
	return out, nil
}
