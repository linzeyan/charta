package charta

import (
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/layout"
	"github.com/linzeyan/charta/scene"
)

const (
	// networkMaxRadius: the most connected node is drawn this large; the
	// least, dotRadius.
	networkMaxRadius = 10.0
	// networkMaxLabels: past this many nodes labels would pile up; tooltips
	// name them instead.
	networkMaxLabels = 30
	// networkMaxNodes: Barnes–Hut keeps the force layout near n log n per
	// tick, but past this a network takes long enough to lay out to notice,
	// and is a hairball anyway: filter or aggregate first.
	networkMaxNodes = 2000
	// networkMaxZoom: small graphs are not blown up to fill the plot; two
	// nodes at opposite corners read as unrelated.
	networkMaxZoom = 3.0
)

// NetworkChart places nodes by a force simulation: linked nodes pull
// together, all nodes push apart, so clusters and bridges show.
type NetworkChart struct {
	Title string `json:"title,omitempty"`
	Links []Link `json:"links"`
}

func (c NetworkChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	g, err := graph(c.Links, false)
	if err != nil {
		return nil, err
	}
	n := len(g.names)
	if n > networkMaxNodes {
		return nil, invalid("%d nodes, at most %d: filter or aggregate the network first", n, networkMaxNodes)
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, g.legend(), keySwatch)

	edges := make([][2]int, len(g.edges))
	for k, e := range g.edges {
		edges[k] = [2]int{e.S, e.T}
	}
	p := layout.Force(n, edges)
	strength := g.strength()
	maxS := 0.0
	for _, s := range strength {
		maxS = math.Max(maxS, s)
	}
	radius := make([]float64, n)
	for i, s := range strength {
		radius[i] = dotRadius + float64((networkMaxRadius-dotRadius)*math.Sqrt(s/maxS))
	}
	labeled := n <= networkMaxLabels
	labelW := 0.0
	if labeled {
		labelW = maxWidth(m, g.names) + gap/2
	}

	// Fit the layout's bounding box into the plot, uniformly, leaving room
	// for the biggest dot all round and for labels on the right.
	left, right := pad+networkMaxRadius, width-pad-networkMaxRadius-labelW
	upper, lower := top+networkMaxRadius, height-pad-networkMaxRadius
	x0, x1, y0, y1 := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, q := range p {
		x0 = math.Min(x0, q.X)
		x1 = math.Max(x1, q.X)
		y0 = math.Min(y0, q.Y)
		y1 = math.Max(y1, q.Y)
	}
	bw, bh := math.Max(x1-x0, 1), math.Max(y1-y0, 1)
	zoom := math.Min(math.Min(math.Max(right-left, 1)/bw, math.Max(lower-upper, 1)/bh), networkMaxZoom)
	ox := (left+right)/2 - float64((x0+x1)/2*zoom)
	oy := (upper+lower)/2 - float64((y0+y1)/2*zoom)
	pos := make([]scene.Point, n)
	for i, q := range p {
		pos[i] = scene.Point{X: ox + float64(q.X*zoom), Y: oy + float64(q.Y*zoom)}
	}
	maxV := 0.0
	for _, e := range g.edges {
		maxV = math.Max(maxV, e.V)
	}

	// Links stay neutral and recede: the nodes carry identity.
	for k, e := range g.edges {
		a, b := pos[e.S], pos[e.T]
		marks = append(marks, scene.Mark{
			Shape:  scene.LineShape{X1: a.X, Y1: a.Y, X2: b.X, Y2: b.Y},
			Stroke: &scene.Stroke{Color: base.Axis, Width: hairline + 2*e.V/maxV},
			Series: g.series(e.S),
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("link-%d", k),
				Label: g.names[e.S] + " – " + g.names[e.T],
				Value: base.Value(e.V),
			},
		})
	}
	for i, name := range g.names {
		marks = append(marks, scene.Mark{
			Shape:  scene.CircleShape{CX: pos[i].X, CY: pos[i].Y, R: radius[i]},
			Fill:   base.Series(g.color(i)),
			Stroke: &scene.Stroke{Color: base.Surface, Width: surfaceGap},
			Series: g.series(i),
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("node-%d", i),
				Label: name,
				Value: base.Value(strength[i]),
				Rows:  []scene.TipRow{{Name: "links", Value: strconv.Itoa(g.links(i))}},
			},
		})
	}
	if labeled {
		for i, name := range g.names {
			haloLabel(&marks, m, pos[i].X+radius[i]+gap/2, pos[i].Y, name, scene.Start, base.TextSecondary)
		}
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Network diagram of %s and %s.",
			count(n, "node", "nodes"), count(len(g.edges), "link", "links"))),
		Marks: marks,
	}, nil
}
