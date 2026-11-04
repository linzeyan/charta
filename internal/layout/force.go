package layout

import (
	"math"

	"github.com/linzeyan/charta/scene"
)

// Force-directed placement after d3-force's defaults: links pull toward a
// rest length, every pair of nodes repels, weak x/y forces keep separate
// components from drifting apart, and the system cools over 300 ticks.
//
// The many-body force is Barnes–Hut, as in d3: a far enough cell of a
// quadtree pushes as one body at its centroid. Both ports build the tree the
// same way (nodes inserted in index order, a fixed quadrant order, cells
// summed children first) and walk it in the same order, so the
// approximation, and every jiggle drawn on the way, comes out bit-identical.

const (
	forceTicks = 300
	// forceAlphaDecay is 1 − 0.001^(1/300), d3's default: alpha falls from 1
	// to 0.001 over the ticks. A literal so neither port depends on its pow.
	forceAlphaDecay = 0.02276277904418933
	// forceVelocityKeep is d3's velocity decay of 0.4: each tick keeps 60%
	// of the velocity.
	forceVelocityKeep = 0.6
	forceDistance     = 30.0
	forceCharge       = -30.0
	forceGravity      = 0.05
	// forceTheta2 is d3's Barnes–Hut θ of 0.9, squared: a cell narrower than
	// θ times its distance pushes as one body.
	forceTheta2 = 0.81
	// forceMaxDepth: cells stop splitting this deep; nodes closer than that
	// share a leaf.
	forceMaxDepth = 32
)

// cell is a quadtree cell over the square at (x, y) of side size: how many
// nodes it holds and their centroid, and either four children or, as a
// leaf, the nodes themselves.
type cell struct {
	x, y, size float64
	depth      int
	count      int
	cx, cy     float64
	kids       *[4]int
	nodes      []int
}

// quadrant is which child holds (px, py): right adds 1, lower adds 2.
func (c *cell) quadrant(px, py float64) int {
	half := c.size / 2
	q := 0
	if px >= c.x+half {
		q++
	}
	if py >= c.y+half {
		q += 2
	}
	return q
}

// quadtree is the tree over p, root first; a child always comes after its
// parent.
func quadtree(p []scene.Point) []cell {
	x0, y0, x1, y1 := p[0].X, p[0].Y, p[0].X, p[0].Y
	for _, q := range p {
		x0, y0 = math.Min(x0, q.X), math.Min(y0, q.Y)
		x1, y1 = math.Max(x1, q.X), math.Max(y1, q.Y)
	}
	size := math.Max(x1-x0, y1-y0)
	if !(size > 0) {
		size = 1
	}
	cells := []cell{{x: x0, y: y0, size: size}}
	for i, q := range p {
		c := 0
		for {
			if k := cells[c].kids; k != nil {
				c = k[cells[c].quadrant(q.X, q.Y)]
				continue
			}
			cl := &cells[c]
			shared := len(cl.nodes) > 0 && p[cl.nodes[0]] == q
			if len(cl.nodes) == 0 || shared || cl.depth == forceMaxDepth {
				cl.nodes = append(cl.nodes, i)
				break
			}
			// Split: the nodes here (all at one spot) move down a level, and
			// the new one tries again.
			x, y, half, depth := cl.x, cl.y, cl.size/2, cl.depth+1
			moved := cl.nodes
			first := len(cells)
			for k := range 4 {
				cells = append(cells, cell{x: x + float64(float64(k%2)*half), y: y + float64(float64(k/2)*half), size: half, depth: depth})
			}
			cl = &cells[c]
			cl.nodes = nil
			j := moved[0]
			cells[first+cl.quadrant(p[j].X, p[j].Y)].nodes = moved
			cl.kids = &[4]int{first, first + 1, first + 2, first + 3}
		}
	}
	// Children come after parents, so backward is children first.
	for c := len(cells) - 1; c >= 0; c-- {
		cl := &cells[c]
		count, sx, sy := 0, 0.0, 0.0
		if cl.kids != nil {
			for _, k := range cl.kids {
				w := float64(cells[k].count)
				count += cells[k].count
				sx += float64(w * cells[k].cx)
				sy += float64(w * cells[k].cy)
			}
		} else {
			for _, j := range cl.nodes {
				count++
				sx += p[j].X
				sy += p[j].Y
			}
		}
		cl.count = count
		if count > 0 {
			cl.cx, cl.cy = sx/float64(count), sy/float64(count)
		}
	}
	return cells
}

// manyBody adds every node's push from all the others to v, Barnes–Hut
// style.
func manyBody(p, v []scene.Point, alpha float64, rng *lcg) {
	cells := quadtree(p)
	var stack []int
	for i := range p {
		stack = append(stack, 0)
		for len(stack) > 0 {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			cl := &cells[c]
			if cl.count == 0 {
				continue
			}
			x, y := cl.cx-p[i].X, cl.cy-p[i].Y
			l := float64(x*x) + float64(y*y)
			far := float64(cl.size*cl.size)/forceTheta2 < l
			if !far {
				if cl.kids != nil {
					// Pushed backward, so the children are visited in order.
					for k := 3; k >= 0; k-- {
						stack = append(stack, cl.kids[k])
					}
					continue
				}
				if len(cl.nodes) == 1 && cl.nodes[0] == i {
					continue
				}
			}
			if x == 0 {
				x = jiggle(rng)
				l += float64(x * x)
			}
			if y == 0 {
				y = jiggle(rng)
				l += float64(y * y)
			}
			// d3's minimum distance of 1 keeps near-coincident nodes from
			// flinging each other off to infinity.
			if l < 1 {
				l = math.Sqrt(l)
			}
			others := cl.count
			if !far {
				others = 0
				for _, j := range cl.nodes {
					if j != i {
						others++
					}
				}
			}
			w := forceCharge * float64(others) * alpha / l
			v[i].X += float64(x * w)
			v[i].Y += float64(y * w)
		}
	}
}

func jiggle(rng *lcg) float64 { return (rng.next() - 0.5) * 1e-6 }

// Force returns positions for n nodes joined by edges, in layout units
// centered on the origin; the caller scales them to the plot.
func Force(n int, edges [][2]int) []scene.Point {
	rng := lcg{1}
	// Random starts over d3's phyllotaxis radius; d3's own spiral needs sin
	// and cos, whose last bit differs between the ports' math libraries.
	spread := 10 * math.Sqrt(float64(n))
	p := make([]scene.Point, n)
	for i := range p {
		x := (float64(rng.next()*2) - 1) * spread
		y := (float64(rng.next()*2) - 1) * spread
		p[i] = scene.Point{X: x, Y: y}
	}
	v := make([]scene.Point, n)

	// d3's link defaults: a link pulls less on a node with many links, and
	// moves the less-connected end more.
	degree := make([]float64, n)
	for _, e := range edges {
		degree[e[0]]++
		degree[e[1]]++
	}
	strength := make([]float64, len(edges))
	bias := make([]float64, len(edges))
	for k, e := range edges {
		s, t := e[0], e[1]
		strength[k] = 1 / math.Min(degree[s], degree[t])
		bias[k] = degree[s] / (degree[s] + degree[t])
	}

	alpha := 1.0
	for range forceTicks {
		alpha -= float64(alpha * forceAlphaDecay)
		for k, e := range edges {
			s, t := e[0], e[1]
			x := p[t].X + v[t].X - p[s].X - v[s].X
			y := p[t].Y + v[t].Y - p[s].Y - v[s].Y
			if x == 0 {
				x = jiggle(&rng)
			}
			if y == 0 {
				y = jiggle(&rng)
			}
			d := math.Sqrt(float64(x*x) + float64(y*y))
			l := (d - forceDistance) / d * alpha * strength[k]
			x *= l
			y *= l
			b := bias[k]
			v[t].X -= float64(x * b)
			v[t].Y -= float64(y * b)
			v[s].X += float64(x * (1 - b))
			v[s].Y += float64(y * (1 - b))
		}
		manyBody(p, v, alpha, &rng)
		for i := range n {
			v[i].X -= float64(p[i].X * forceGravity * alpha)
			v[i].Y -= float64(p[i].Y * forceGravity * alpha)
			v[i].X *= forceVelocityKeep
			v[i].Y *= forceVelocityKeep
			p[i].X += v[i].X
			p[i].Y += v[i].Y
		}
	}
	return p
}
