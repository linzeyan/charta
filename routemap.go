package charta

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/geo"
	"github.com/linzeyan/charta/scene"
)

const (
	// routeSteps is segments per great-circle route: smooth at any map size,
	// few points.
	routeSteps = 32
	// Route widths run from 1px to routeMax; flow widths from flowMin.
	routeMax = 3.0
	flowMin  = 1.5
	flowMax  = 10.0
	// flowBow is how far a flow bows sideways, as a share of its length:
	// enough that flows in opposite directions between two places separate.
	flowBow = 0.2
	// maxPlaceLabels: past this many places, labels would pile up; tooltips
	// name them.
	maxPlaceLabels = 20
	// placeLabelGap is the space between a label and the dots or labels
	// around it.
	placeLabelGap = 2.0
)

// Location is a named place.
type Location struct {
	Name string  `json:"name"`
	Lon  float64 `json:"lon"`
	Lat  float64 `json:"lat"`
}

// ConnectionMapChart joins places by their shortest routes on the globe
// (great circles): which places connect. Links are undirected.
type ConnectionMapChart struct {
	Title      string         `json:"title,omitempty"`
	Features   []geo.Feature  `json:"features,omitempty"`
	Basemap    string         `json:"basemap,omitempty"`
	Projection geo.Projection `json:"projection"`
	Places     []Location     `json:"places"`
	Links      []Link         `json:"links"`
}

// FlowMapChart draws curved arrows from place to place, as thick as what
// moves: where things go and how much.
type FlowMapChart struct {
	Title      string         `json:"title,omitempty"`
	Features   []geo.Feature  `json:"features,omitempty"`
	Basemap    string         `json:"basemap,omitempty"`
	Projection geo.Projection `json:"projection"`
	Places     []Location     `json:"places"`
	Links      []Link         `json:"links"`
}

// routeGraph is the links resolved against the places: graph node i sits at
// at[i]. Places no link mentions are still drawn, after the linked ones.
type routeGraph struct {
	g  *linkGraph
	at []geo.Position
}

func routes(places []Location, links []Link, directed bool) (*routeGraph, error) {
	for i, p := range places {
		if err := checkLonLat(fmt.Sprintf("%q", p.Name), p.Lon, p.Lat); err != nil {
			return nil, err
		}
		for _, o := range places[:i] {
			if o.Name == p.Name {
				return nil, invalid("place %q is listed twice", p.Name)
			}
		}
	}
	g, err := graph(links, directed)
	if err != nil {
		return nil, err
	}
	at := make([]geo.Position, 0, len(places))
	for _, name := range g.names {
		i := slices.IndexFunc(places, func(p Location) bool { return p.Name == name })
		if i < 0 {
			return nil, invalid("a link names %q, which is not a place", name)
		}
		at = append(at, geo.Position{places[i].Lon, places[i].Lat})
	}
	for _, p := range places {
		if !slices.Contains(g.names, p.Name) {
			g.names = append(g.names, p.Name)
			at = append(at, geo.Position{p.Lon, p.Lat})
		}
	}
	return &routeGraph{g, at}, nil
}

// drawPlaces draws the place dots, then labels on top of everything when
// there are few.
func drawPlaces(marks *[]scene.Mark, m base.TextMeasure, r *routeGraph, xy []scene.Point, width float64) {
	g := r.g
	for i, name := range g.names {
		c := xy[i]
		*marks = append(*marks, scene.Mark{
			Shape:  scene.CircleShape{CX: c.X, CY: c.Y, R: dotRadius},
			Fill:   base.Series(g.color(i)),
			Stroke: &scene.Stroke{Color: base.Surface, Width: surfaceGap},
			Series: g.series(i),
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("place-%d", i),
				Label: name,
				Rows:  []scene.TipRow{{Name: "links", Value: strconv.Itoa(g.links(i))}},
				Area:  &scene.Rect{X: c.X - minTarget/2, Y: c.Y - minTarget/2, W: minTarget, H: minTarget},
			},
		})
	}
	if len(g.names) > maxPlaceLabels {
		return
	}
	// Places cluster (Europe, East Asia), so labels go greedily: hubs first,
	// each to the right of its dot or else the left, and dropped when both
	// sides hit a placed label, another dot or the chart edge. A dropped name
	// is still in the dot's tooltip.
	order := make([]int, len(g.names))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return g.links(order[a]) > g.links(order[b]) })
	dots := make([]scene.Rect, len(xy))
	for i, c := range xy {
		dots[i] = scene.Rect{X: c.X - dotRadius, Y: c.Y - dotRadius, W: 2 * dotRadius, H: 2 * dotRadius}
	}
	var placed []scene.Rect
	clashes := func(a, b scene.Rect) bool {
		return a.X < b.X+b.W+placeLabelGap && b.X < a.X+a.W+placeLabelGap &&
			a.Y < b.Y+b.H+placeLabelGap && b.Y < a.Y+a.H+placeLabelGap
	}
	off := dotRadius + gap/2
	for _, i := range order {
		name := g.names[i]
		for _, spot := range []struct {
			x      float64
			anchor scene.Anchor
		}{{xy[i].X + off, scene.Start}, {xy[i].X - off, scene.End}} {
			b := haloRect(m, spot.x, xy[i].Y, name, spot.anchor)
			if b.X < 0 || b.X+b.W > width || slices.ContainsFunc(placed, func(p scene.Rect) bool { return clashes(p, b) }) {
				continue
			}
			hit := false
			for j, d := range dots {
				if j != i && clashes(d, b) {
					hit = true
					break
				}
			}
			if hit {
				continue
			}
			placed = append(placed, b)
			haloLabel(marks, m, spot.x, xy[i].Y, name, spot.anchor, base.TextSecondary)
			break
		}
	}
}

func (c ConnectionMapChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	regs, err := regions(c.Features, c.Basemap, c.Projection)
	if err != nil {
		return nil, err
	}
	r, err := routes(c.Places, c.Links, false)
	if err != nil {
		return nil, err
	}
	g := r.g
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, g.legend(), keySwatch)
	fit := newMapFit(c.Projection, regs, r.at, pad, top, width-pad, height-pad)
	land(&marks, fit, regs)
	most := 0.0
	for _, e := range g.edges {
		most = math.Max(most, e.V)
	}
	for k, e := range g.edges {
		// A route crossing the antimeridian leaves one edge of the map and
		// re-enters at the other: break it there instead of drawing a line
		// across the whole world.
		prev := 0.0
		var ops []scene.PathOp
		for n, p := range geo.GreatCircle(r.at[e.S], r.at[e.T], routeSteps) {
			x, y := fit.xy(p[0], p[1])
			if n > 0 && math.Abs(p[0]-prev) <= 180 {
				ops = append(ops, scene.Line(x, y))
			} else {
				ops = append(ops, scene.Move(x, y))
			}
			prev = p[0]
		}
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: ops},
			Stroke: &scene.Stroke{Color: base.Link(g.color(e.S)), Width: 1 + float64((routeMax-1)*e.V)/most},
			Series: g.series(e.S),
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("route-%d", k),
				Label: g.names[e.S] + " – " + g.names[e.T],
				Value: base.Value(e.V),
			},
		})
	}
	xy := make([]scene.Point, len(r.at))
	for i, p := range r.at {
		xy[i].X, xy[i].Y = fit.xy(p[0], p[1])
	}
	drawPlaces(&marks, m, r, xy, width)
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Connection map of %s between %s.",
			count(len(g.edges), "route", "routes"), count(len(g.names), "place", "places"))),
		Marks: marks,
	}, nil
}

func (c FlowMapChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	regs, err := regions(c.Features, c.Basemap, c.Projection)
	if err != nil {
		return nil, err
	}
	r, err := routes(c.Places, c.Links, true)
	if err != nil {
		return nil, err
	}
	g := r.g
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, g.legend(), keySwatch)
	fit := newMapFit(c.Projection, regs, r.at, pad, top, width-pad, height-pad)
	land(&marks, fit, regs)
	xy := make([]scene.Point, len(r.at))
	for i, p := range r.at {
		xy[i].X, xy[i].Y = fit.xy(p[0], p[1])
	}
	most := 0.0
	for _, e := range g.edges {
		most = math.Max(most, e.V)
	}
	for k, e := range g.edges {
		p, q := xy[e.S], xy[e.T]
		dx, dy := q.X-p.X, q.Y-p.Y
		l := math.Sqrt(float64(dx*dx) + float64(dy*dy))
		if l == 0 {
			continue
		}
		// Bow to the right of the direction of travel, so A → B and B → A
		// curve apart instead of overlapping.
		cx, cy := (p.X+q.X)/2-float64(dy*flowBow), (p.Y+q.Y)/2+float64(dx*flowBow)
		w := flowMin + float64((flowMax-flowMin)*e.V)/most
		// The head points along the curve's final tangent (from the control
		// point to the end) and the shaft stops at its base.
		tx, ty := q.X-cx, q.Y-cy
		tl := math.Sqrt(float64(tx*tx) + float64(ty*ty))
		ux, uy := tx/tl, ty/tl
		head, half := float64(2*w)+4, w+3
		bx, by := q.X-float64(ux*head), q.Y-float64(uy*head)
		shaft := quad([]scene.PathOp{scene.Move(p.X, p.Y)}, p.X, p.Y, cx, cy, bx, by)
		color := base.Link(g.color(e.S))
		marks = append(marks,
			scene.Mark{
				Shape:  scene.PathShape{Ops: shaft},
				Stroke: &scene.Stroke{Color: color, Width: w},
				Series: g.series(e.S),
				Hit: &scene.Hit{
					Key:   fmt.Sprintf("flow-%d", k),
					Label: g.names[e.S] + " → " + g.names[e.T],
					Value: base.Value(e.V),
				},
			},
			scene.Mark{
				Shape: scene.PathShape{Ops: []scene.PathOp{
					scene.Move(q.X, q.Y),
					scene.Line(bx-float64(uy*half), by+float64(ux*half)),
					scene.Line(bx+float64(uy*half), by-float64(ux*half)),
					scene.Close(),
				}},
				Fill:   color,
				Series: g.series(e.S),
			})
	}
	drawPlaces(&marks, m, r, xy, width)
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Flow map of %s between %s.",
			count(len(g.edges), "flow", "flows"), count(len(g.names), "place", "places"))),
		Marks: marks,
	}, nil
}
