package layout

import (
	"cmp"
	"math"
	"slices"
)

// Sankey placement, ported from d3-sankey with its defaults: nodes in
// columns by depth with sinks pushed to the last column ("justify"), node
// heights proportional to throughput, then six rounds of relaxation that
// slide each node toward the links feeding it while keeping a column's nodes
// apart.
//
// One deliberate difference: link order at every node is re-sorted after the
// last round, so links never cross where they meet a node.

// Flow is a weighted link from node S to node T.
type Flow struct {
	S, T int
	V    float64
}

// SankeyLayout is the placed nodes and links, in input order. Vertical only:
// the caller turns Layer into x.
type SankeyLayout struct {
	Columns int
	Layer   []int
	Value   []float64
	Y0, Y1  []float64
	// Per link: its thickness, and the y of its center where it leaves the
	// source and where it enters the target.
	Width, SourceY, TargetY []float64
}

const sankeyIterations = 6

type sankeyState struct {
	edges  []Flow
	layer  []int
	y0, y1 []float64
	width  []float64
	// out and inc are the link indices leaving / entering each node, kept
	// sorted by the other end's position.
	out, inc        [][]int
	py, top, bottom float64
}

// totalCompare is Rust's f64::total_cmp as a three-way comparison.
func totalCompare(a, b float64) int {
	switch {
	case totalLess(a, b):
		return -1
	case totalLess(b, a):
		return 1
	}
	return 0
}

// Sankey lays n nodes and their edges out between top and bottom, nodes in
// a column padding apart at most. It fails on a cycle, which has no
// left-to-right order: ok is false and cycle names a node on it.
func Sankey(n int, edges []Flow, top, bottom, padding float64) (l *SankeyLayout, cycle int, ok bool) {
	out := make([][]int, n)
	inc := make([][]int, n)
	for k, e := range edges {
		out[e.S] = append(out[e.S], k)
		inc[e.T] = append(inc[e.T], k)
	}
	value := make([]float64, n)
	for i := range n {
		o, in := 0.0, 0.0
		for _, k := range out[i] {
			o += edges[k].V
		}
		for _, k := range inc[i] {
			in += edges[k].V
		}
		value[i] = max(o, in)
	}

	// Depth: the longest path from any source. A path longer than the node
	// count must revisit a node.
	depth := make([]int, n)
	current := make([]int, n)
	for i := range current {
		current[i] = i
	}
	x := 0
	for len(current) > 0 {
		var next []int
		for _, i := range current {
			depth[i] = x
			for _, k := range out[i] {
				if !slices.Contains(next, edges[k].T) {
					next = append(next, edges[k].T)
				}
			}
		}
		x++
		if x > n {
			return nil, onCycle(edges, out), false
		}
		current = next
	}
	columnsN := 0
	for _, d := range depth {
		columnsN = max(columnsN, d+1)
	}
	layer := make([]int, n)
	for i := range n {
		if len(out[i]) == 0 {
			layer[i] = columnsN - 1
		} else {
			layer[i] = depth[i]
		}
	}
	columns := make([][]int, columnsN)
	for i := range n {
		columns[layer[i]] = append(columns[layer[i]], i)
	}

	tallest := 1
	if len(columns) > 0 {
		tallest = 0
		for _, c := range columns {
			tallest = max(tallest, len(c))
		}
	}
	py := padding
	if tallest > 1 {
		py = min(padding, (bottom-top)/float64(tallest-1))
	}
	ky := math.Inf(1)
	for _, c := range columns {
		total := 0.0
		for _, i := range c {
			total += value[i]
		}
		ky = math.Min(ky, (bottom-top-float64(float64(len(c)-1)*py))/total)
	}

	s := &sankeyState{
		edges:  edges,
		layer:  layer,
		y0:     make([]float64, n),
		y1:     make([]float64, n),
		width:  make([]float64, len(edges)),
		out:    out,
		inc:    inc,
		py:     py,
		top:    top,
		bottom: bottom,
	}
	for k, e := range edges {
		s.width[k] = e.V * ky
	}
	// Stack each column from the top, then spread its slack evenly.
	for _, c := range columns {
		y := top
		for _, i := range c {
			s.y0[i] = y
			s.y1[i] = y + float64(value[i]*ky)
			y = s.y1[i] + py
		}
		slack := (bottom - y + py) / float64(len(c)+1)
		for k, i := range c {
			s.y0[i] += float64(slack * float64(k+1))
			s.y1[i] += float64(slack * float64(k+1))
		}
	}
	for i := range n {
		s.sortLinks(i)
	}

	alpha := 1.0
	for it := range sankeyIterations {
		beta := max(1-alpha, float64(it+1)/sankeyIterations)
		s.relaxRightToLeft(columns, alpha, beta)
		s.relaxLeftToRight(columns, alpha, beta)
		alpha *= 0.99
	}
	for i := range n {
		s.sortLinks(i)
	}

	sourceY := make([]float64, len(edges))
	targetY := make([]float64, len(edges))
	for i := range n {
		y := s.y0[i]
		for _, k := range s.out[i] {
			sourceY[k] = y + s.width[k]/2
			y += s.width[k]
		}
		y = s.y0[i]
		for _, k := range s.inc[i] {
			targetY[k] = y + s.width[k]/2
			y += s.width[k]
		}
	}
	return &SankeyLayout{
		Columns: columnsN,
		Layer:   s.layer,
		Value:   value,
		Y0:      s.y0,
		Y1:      s.y1,
		Width:   s.width,
		SourceY: sourceY,
		TargetY: targetY,
	}, 0, true
}

// onCycle is the first node (in input order) that can reach itself, for the
// error.
func onCycle(edges []Flow, out [][]int) int {
	for i := range out {
		seen := make([]bool, len(out))
		var stack []int
		for _, k := range out[i] {
			stack = append(stack, edges[k].T)
		}
		for len(stack) > 0 {
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if j == i {
				return i
			}
			if !seen[j] {
				seen[j] = true
				for _, k := range out[j] {
					stack = append(stack, edges[k].T)
				}
			}
		}
	}
	panic("a path longer than the node count revisits a node")
}

// byTarget orders link indices by their target's top, input order breaking
// ties; bySource the same by source.
func (s *sankeyState) byTarget(a, b int) int {
	return cmp.Or(totalCompare(s.y0[s.edges[a].T], s.y0[s.edges[b].T]), cmp.Compare(a, b))
}

func (s *sankeyState) bySource(a, b int) int {
	return cmp.Or(totalCompare(s.y0[s.edges[a].S], s.y0[s.edges[b].S]), cmp.Compare(a, b))
}

// sortLinks orders a node's outgoing links by target position and incoming
// links by source position, so links fan out without crossing at the node.
func (s *sankeyState) sortLinks(i int) {
	slices.SortStableFunc(s.out[i], s.byTarget)
	slices.SortStableFunc(s.inc[i], s.bySource)
}

// resortNeighbors: after node i moves, its neighbors' link orders may be
// stale.
func (s *sankeyState) resortNeighbors(i int) {
	for _, k := range slices.Clone(s.inc[i]) {
		src := s.edges[k].S
		slices.SortStableFunc(s.out[src], s.byTarget)
	}
	for _, k := range slices.Clone(s.out[i]) {
		t := s.edges[k].T
		slices.SortStableFunc(s.inc[t], s.bySource)
	}
}

// targetTop is the target top that would make link source → target run
// straight.
func (s *sankeyState) targetTop(source, target int) float64 {
	y := s.y0[source] - float64(len(s.out[source])-1)*s.py/2
	for _, k := range s.out[source] {
		if s.edges[k].T == target {
			break
		}
		y += s.width[k] + s.py
	}
	for _, k := range s.inc[target] {
		if s.edges[k].S == source {
			break
		}
		y -= s.width[k]
	}
	return y
}

// sourceTop is the source top that would make link source → target run
// straight.
func (s *sankeyState) sourceTop(source, target int) float64 {
	y := s.y0[target] - float64(len(s.inc[target])-1)*s.py/2
	for _, k := range s.inc[target] {
		if s.edges[k].S == source {
			break
		}
		y += s.width[k] + s.py
	}
	for _, k := range s.out[source] {
		if s.edges[k].T == target {
			break
		}
		y -= s.width[k]
	}
	return y
}

func (s *sankeyState) relaxLeftToRight(columns [][]int, alpha, beta float64) {
	for _, column := range columns[min(1, len(columns)):] {
		for _, t := range column {
			y, w := 0.0, 0.0
			for _, k := range s.inc[t] {
				e := s.edges[k]
				v := e.V * float64(s.layer[t]-s.layer[e.S])
				y += float64(s.targetTop(e.S, t) * v)
				w += v
			}
			if w > 0 {
				dy := (y/w - s.y0[t]) * alpha
				s.y0[t] += dy
				s.y1[t] += dy
				s.resortNeighbors(t)
			}
		}
		s.settle(column, beta)
	}
}

func (s *sankeyState) relaxRightToLeft(columns [][]int, alpha, beta float64) {
	for c := len(columns) - 2; c >= 0; c-- {
		column := columns[c]
		for _, src := range column {
			y, w := 0.0, 0.0
			for _, k := range s.out[src] {
				e := s.edges[k]
				v := e.V * float64(s.layer[e.T]-s.layer[src])
				y += float64(s.sourceTop(src, e.T) * v)
				w += v
			}
			if w > 0 {
				dy := (y/w - s.y0[src]) * alpha
				s.y0[src] += dy
				s.y1[src] += dy
				s.resortNeighbors(src)
			}
		}
		s.settle(column, beta)
	}
}

// settle re-sorts a column by position, then pushes overlapping nodes apart
// outward from the middle one and back inside the extent.
func (s *sankeyState) settle(column []int, alpha float64) {
	slices.SortStableFunc(column, func(a, b int) int { return totalCompare(s.y0[a], s.y0[b]) })
	mid := len(column) / 2
	subject := column[mid]
	s.pushUp(column, s.y0[subject]-s.py, mid-1, alpha)
	s.pushDown(column, s.y1[subject]+s.py, mid+1, alpha)
	s.pushUp(column, s.bottom, len(column)-1, alpha)
	s.pushDown(column, s.top, 0, alpha)
}

func (s *sankeyState) pushDown(column []int, y float64, from int, alpha float64) {
	for _, i := range column[min(from, len(column)):] {
		dy := (y - s.y0[i]) * alpha
		if dy > 1e-6 {
			s.y0[i] += dy
			s.y1[i] += dy
		}
		y = s.y1[i] + s.py
	}
}

func (s *sankeyState) pushUp(column []int, y float64, from int, alpha float64) {
	for k := from; k >= 0; k-- {
		i := column[k]
		dy := (s.y1[i] - y) * alpha
		if dy > 1e-6 {
			s.y0[i] -= dy
			s.y1[i] -= dy
		}
		y = s.y0[i] - s.py
	}
}
