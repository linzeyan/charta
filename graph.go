package charta

import (
	"github.com/linzeyan/charta/internal/layout"
	"github.com/linzeyan/charta/scene"
)

// The input shared by the relationship charts: weighted links between named
// nodes. Nodes are not listed separately; they appear in the order links
// first mention them, so the same data always lays out the same way.

// Link is a weighted connection from Source to Target. Charts that ignore
// direction (network, arc, non-ribbon chord) treat it as undirected.
type Link struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Value  float64 `json:"value"`
}

// linkGraph is links resolved to node indices; edges keep input order.
type linkGraph struct {
	names []string
	edges []layout.Flow
}

// colored: every node gets its own palette slot when there are few enough;
// past that, one hue for all rather than a generated ninth.
func (g *linkGraph) colored() bool { return len(g.names) <= maxSeries }

func (g *linkGraph) color(i int) int {
	if g.colored() {
		return i
	}
	return 0
}

// legend is the node names for the legend: only when nodes carry distinct
// colors.
func (g *linkGraph) legend() []string {
	if g.colored() {
		return g.names
	}
	return nil
}

// series is node i's highlight identity, when nodes are colored apart.
func (g *linkGraph) series(i int) string {
	if g.colored() {
		return g.names[i]
	}
	return ""
}

// strength is the sum of the values of the links touching each node.
func (g *linkGraph) strength() []float64 {
	s := make([]float64, len(g.names))
	for _, e := range g.edges {
		s[e.S] += e.V
		s[e.T] += e.V
	}
	return s
}

// links counts the links touching node i.
func (g *linkGraph) links(i int) int {
	n := 0
	for _, e := range g.edges {
		if e.S == i || e.T == i {
			n++
		}
	}
	return n
}

// graph resolves links. Values must be positive and finite (a zero link is
// no link), no node links to itself, and each pair appears once: in both
// directions too when the chart ignores direction, since the two would draw
// over each other.
func graph(links []Link, directed bool) (*linkGraph, error) {
	g := &linkGraph{edges: make([]layout.Flow, 0, len(links))}
	index := func(n string) int {
		for i, m := range g.names {
			if m == n {
				return i
			}
		}
		g.names = append(g.names, n)
		return len(g.names) - 1
	}
	for _, l := range links {
		if !(isFinite(l.Value) && l.Value > 0) {
			return nil, invalid("link %q → %q needs a positive value", l.Source, l.Target)
		}
		if l.Source == l.Target {
			return nil, invalid("link %q → %q connects a node to itself", l.Source, l.Target)
		}
		a := index(l.Source)
		b := index(l.Target)
		for _, e := range g.edges {
			if (e.S == a && e.T == b) || (!directed && e.S == b && e.T == a) {
				return nil, invalid("link %q → %q is given twice", l.Source, l.Target)
			}
		}
		g.edges = append(g.edges, layout.Flow{S: a, T: b, V: l.Value})
	}
	return g, nil
}

// flowBand is a flow band of thickness w from (x0, y0) to (x1, y1) (top
// edges), easing horizontally at both ends so it meets each bar square-on.
func flowBand(x0, y0, x1, y1, w float64) []scene.PathOp {
	xm := (x0 + x1) / 2
	return []scene.PathOp{
		scene.Move(x0, y0),
		scene.Cubic(xm, y0, xm, y1, x1, y1),
		scene.Line(x1, y1+w),
		scene.Cubic(xm, y1+w, xm, y0+w, x0, y0+w),
		scene.Close(),
	}
}

// quad appends a quadratic Bézier from the current point (fx, fy) through
// the control point (cx, cy) to (tx, ty), as the cubic that traces it
// exactly (paths carry cubics only).
func quad(ops []scene.PathOp, fx, fy, cx, cy, tx, ty float64) []scene.PathOp {
	// 2.0 / 3.0 rounds to the same double Rust computes; each product is
	// rounded before the add, as in Rust, instead of fused.
	return append(ops, scene.Cubic(
		fx+float64(2.0/3.0*(cx-fx)),
		fy+float64(2.0/3.0*(cy-fy)),
		tx+float64(2.0/3.0*(cx-tx)),
		ty+float64(2.0/3.0*(cy-ty)),
		tx,
		ty,
	))
}
