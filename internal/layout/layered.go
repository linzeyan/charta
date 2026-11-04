package layout

import (
	"math"
	"slices"
)

// Layered (Sugiyama) placement for directed graphs drawn top to bottom:
// break cycles, assign layers, order each layer to reduce crossings, then
// give every node an x that keeps it near its neighbors.
//
// Each stage is the simple, well-known choice rather than the strongest: DFS
// back-edge reversal, longest-path layering, barycenter sweeps, and
// least-squares x placement per layer. Flowcharts are small; the simple
// versions are readable, and both ports can follow them step by step.

// LayeredLayout is the positions of the real nodes, plus the route of every
// edge.
type LayeredLayout struct {
	Layers int
	Layer  []int
	// X is the center x of each node, the leftmost edge at 0.
	X []float64
	// Width is the total width.
	Width float64
	// Bends, per input edge, is the x of each intermediate layer it passes
	// through, in order from its upper end to its lower end.
	Bends [][]float64
	// Reversed marks an edge that points up (it closes a loop): its source
	// is the lower end.
	Reversed []bool
}

const (
	layeredSweeps = 4
	layeredRounds = 6
)

// Layered lays out nodes of the given widths joined by edges (no self-loops,
// no duplicates), keeping gap between neighbors in a layer.
func Layered(widths []float64, edges [][2]int, gap float64) *LayeredLayout {
	n := len(widths)
	reversed := backEdges(n, edges)
	down := make([][2]int, len(edges))
	for k, e := range edges {
		if reversed[k] {
			down[k] = [2]int{e[1], e[0]}
		} else {
			down[k] = e
		}
	}
	layer := layers(n, down)
	layersN := 0
	for _, l := range layer {
		layersN = max(layersN, l+1)
	}

	// Split long edges with zero-width dummies, one per layer crossed.
	w := slices.Clone(widths)
	lay := slices.Clone(layer)
	chains := make([][]int, 0, len(down))
	succ := make([][]int, n)
	pred := make([][]int, n)
	for _, e := range down {
		s, t := e[0], e[1]
		chain := []int{s}
		for l := layer[s] + 1; l < layer[t]; l++ {
			w = append(w, 0)
			lay = append(lay, l)
			succ = append(succ, nil)
			pred = append(pred, nil)
			chain = append(chain, len(w)-1)
		}
		chain = append(chain, t)
		for i := 1; i < len(chain); i++ {
			succ[chain[i-1]] = append(succ[chain[i-1]], chain[i])
			pred[chain[i]] = append(pred[chain[i]], chain[i-1])
		}
		chains = append(chains, chain)
	}
	total := len(w)

	// Initial order: the order a depth-first walk from the sources meets
	// each node, which already keeps most subtrees together.
	rows := make([][]int, layersN)
	seen := make([]bool, total)
	for root := range total {
		if seen[root] || len(pred[root]) > 0 {
			continue
		}
		stack := []int{root}
		for len(stack) > 0 {
			v := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if seen[v] {
				continue
			}
			seen[v] = true
			rows[lay[v]] = append(rows[lay[v]], v)
			for i := len(succ[v]) - 1; i >= 0; i-- {
				if u := succ[v][i]; !seen[u] {
					stack = append(stack, u)
				}
			}
		}
	}
	order(rows, pred, succ)

	sep := func(a, b int) float64 {
		g := gap
		if a >= n && b >= n {
			g = gap / 2
		}
		return (w[a]+w[b])/2 + g
	}
	x := make([]float64, total)
	for _, row := range rows {
		at := 0.0
		for k, v := range row {
			if k == 0 {
				at = w[v] / 2
			} else {
				at += sep(row[k-1], v)
			}
			x[v] = at
		}
	}
	mean := func(of []int, v int) float64 {
		if len(of) == 0 {
			return x[v]
		}
		sum := 0.0
		for _, u := range of {
			sum += x[u]
		}
		return sum / float64(len(of))
	}
	for round := 0; round <= layeredRounds; round++ {
		for _, row := range rows {
			want := make([]float64, len(row))
			for k, v := range row {
				if round == layeredRounds {
					want[k] = mean(slices.Concat(pred[v], succ[v]), v)
				} else {
					want[k] = mean(pred[v], v)
				}
			}
			placeRow(row, want, sep, x)
		}
		if round == layeredRounds {
			break
		}
		for r := len(rows) - 1; r >= 0; r-- {
			row := rows[r]
			want := make([]float64, len(row))
			for k, v := range row {
				want[k] = mean(succ[v], v)
			}
			placeRow(row, want, sep, x)
		}
	}
	left := math.Inf(1)
	for v := range total {
		left = math.Min(left, x[v]-w[v]/2)
	}
	if math.IsInf(left, 0) || math.IsNaN(left) {
		left = 0
	}
	for v := range x {
		x[v] -= left
	}
	width := 0.0
	for v := range total {
		width = rustMax(width, x[v]+w[v]/2)
	}
	bends := make([][]float64, len(chains))
	for k, c := range chains {
		bends[k] = []float64{}
		for _, d := range c[1 : len(c)-1] {
			bends[k] = append(bends[k], x[d])
		}
	}
	return &LayeredLayout{
		Layers:   layersN,
		Layer:    layer,
		X:        x[:n],
		Width:    width,
		Bends:    bends,
		Reversed: reversed,
	}
}

// backEdges marks the edges a depth-first walk (nodes and edges in input
// order) finds pointing back to a node still on its path. Reversing exactly
// those leaves a graph without cycles.
func backEdges(n int, edges [][2]int) []bool {
	out := make([][]int, n)
	for k, e := range edges {
		out[e[0]] = append(out[e[0]], k)
	}
	const (
		unvisited = iota
		onPath
		done
	)
	state := make([]int, n)
	back := make([]bool, len(edges))
	type frame struct{ v, next int }
	for root := range n {
		if state[root] != unvisited {
			continue
		}
		state[root] = onPath
		stack := []frame{{root, 0}}
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			v := top.v
			if top.next == len(out[v]) {
				state[v] = done
				stack = stack[:len(stack)-1]
				continue
			}
			k := out[v][top.next]
			top.next++
			t := edges[k][1]
			switch state[t] {
			case unvisited:
				state[t] = onPath
				stack = append(stack, frame{t, 0})
			case onPath:
				back[k] = true
			}
		}
	}
	return back
}

// layers is the longest path from the sources, then each source moved down
// to sit just above its highest child: a side entry joins where it is needed
// instead of trailing a long edge from the top row.
func layers(n int, edges [][2]int) []int {
	indeg := make([]int, n)
	out := make([][]int, n)
	for _, e := range edges {
		indeg[e[1]]++
		out[e[0]] = append(out[e[0]], e[1])
	}
	layer := make([]int, n)
	var ready []int
	for v := range n {
		if indeg[v] == 0 {
			ready = append(ready, v)
		}
	}
	for k := 0; k < len(ready); k++ {
		v := ready[k]
		for _, t := range out[v] {
			layer[t] = max(layer[t], layer[v]+1)
			indeg[t]--
			if indeg[t] == 0 {
				ready = append(ready, t)
			}
		}
	}
	for v := range n {
		isSource := !slices.ContainsFunc(edges, func(e [2]int) bool { return e[1] == v })
		if isSource && len(out[v]) > 0 {
			m := layer[out[v][0]]
			for _, t := range out[v][1:] {
				m = min(m, layer[t])
			}
			layer[v] = m - 1
		}
	}
	return layer
}

func crossings(rows [][]int, succ [][]int) int {
	pos := make([]int, len(succ))
	for _, row := range rows {
		for i, v := range row {
			pos[v] = i
		}
	}
	count := 0
	for _, row := range rows {
		var segs [][2]int
		for _, v := range row {
			for _, u := range succ[v] {
				segs = append(segs, [2]int{pos[v], pos[u]})
			}
		}
		for i, a := range segs {
			for _, b := range segs[i+1:] {
				if (a[0] < b[0] && a[1] > b[1]) || (a[0] > b[0] && a[1] < b[1]) {
					count++
				}
			}
		}
	}
	return count
}

// order runs barycenter sweeps, down then up, keeping the best order seen.
// A node with no neighbors on the fixed side keeps its current slot.
func order(rows [][]int, pred, succ [][]int) {
	snapshot := func() [][]int {
		out := make([][]int, len(rows))
		for i, r := range rows {
			out[i] = slices.Clone(r)
		}
		return out
	}
	best := snapshot()
	fewest := crossings(rows, succ)
	pos := make([]float64, len(succ))
	sync := func() {
		for _, row := range rows {
			for i, v := range row {
				pos[v] = float64(i)
			}
		}
	}
	for range layeredSweeps {
		for pass := range 2 {
			var ls []int
			if pass == 0 {
				for l := 1; l < len(rows); l++ {
					ls = append(ls, l)
				}
			} else {
				for l := len(rows) - 2; l >= 0; l-- {
					ls = append(ls, l)
				}
			}
			nb := pred
			if pass == 1 {
				nb = succ
			}
			for _, l := range ls {
				sync()
				key := make([]float64, len(rows[l]))
				for k, v := range rows[l] {
					if len(nb[v]) == 0 {
						key[k] = pos[v]
					} else {
						sum := 0.0
						for _, u := range nb[v] {
							sum += pos[u]
						}
						key[k] = sum / float64(len(nb[v]))
					}
				}
				idx := make([]int, len(rows[l]))
				for i := range idx {
					idx[i] = i
				}
				slices.SortStableFunc(idx, func(a, b int) int { return totalCompare(key[a], key[b]) })
				next := make([]int, len(idx))
				for k, i := range idx {
					next[k] = rows[l][i]
				}
				rows[l] = next
			}
			if c := crossings(rows, succ); c < fewest {
				fewest = c
				best = snapshot()
			}
		}
	}
	copy(rows, best)
}

// placeRow sets the x for each node of row closest (least squares) to want
// that keeps the row's order and separations: pool-adjacent-violators on the
// positions with the separations subtracted out.
func placeRow(row []int, want []float64, sep func(a, b int) float64, x []float64) {
	offset := make([]float64, 0, len(row))
	acc := 0.0
	for k, v := range row {
		if k > 0 {
			acc += sep(row[k-1], v)
		}
		offset = append(offset, acc)
	}
	// Blocks of (sum, count): each block's members share one shifted value.
	type block struct {
		sum   float64
		count int
	}
	var blocks []block
	for k := range row {
		blocks = append(blocks, block{want[k] - offset[k], 1})
		for len(blocks) > 1 {
			b1 := blocks[len(blocks)-1]
			b0 := blocks[len(blocks)-2]
			if b0.sum/float64(b0.count) <= b1.sum/float64(b1.count) {
				break
			}
			blocks = blocks[:len(blocks)-1]
			blocks[len(blocks)-1] = block{b0.sum + b1.sum, b0.count + b1.count}
		}
	}
	k := 0
	for _, b := range blocks {
		for range b.count {
			x[row[k]] = b.sum/float64(b.count) + offset[k]
			k++
		}
	}
}
