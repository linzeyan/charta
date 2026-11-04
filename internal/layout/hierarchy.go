package layout

import (
	"math"
	"sort"
)

// Hierarchy layouts, ported from d3-hierarchy (ISC license, Mike Bostock)
// so the results match what people expect from the reference
// implementation: squarified treemaps, circle packing (front-chain
// placement plus Welzl's enclosing circle) and Buchheim's tidy tree. Nodes
// live in an arena indexed in preorder. Mirrors the Rust crate's
// layout/hierarchy.rs; products feeding a sum are wrapped in float64() so
// the compiler cannot fuse them, which Rust never does.

// Tree is a hierarchy in preorder: node 0 is the root; every parent precedes
// its children, and children are listed in order.
type Tree struct {
	// Parent is -1 for the root.
	Parent   []int
	Children [][]int
	Depth    []int
	// Value holds leaf values, and for inner nodes the sum of their leaves.
	Value []float64
}

func (t *Tree) Len() int { return len(t.Parent) }

// Push adds a node under parent (-1 for the root) and returns its index.
func (t *Tree) Push(parent int, value float64) int {
	i := t.Len()
	t.Parent = append(t.Parent, parent)
	t.Children = append(t.Children, nil)
	depth := 0
	if parent >= 0 {
		depth = t.Depth[parent] + 1
	}
	t.Depth = append(t.Depth, depth)
	t.Value = append(t.Value, value)
	if parent >= 0 {
		t.Children[parent] = append(t.Children[parent], i)
	}
	return i
}

// Sum sets every inner node's value to the sum of its children's, in
// reverse preorder so children are summed before their parents.
func (t *Tree) Sum() {
	for i := t.Len() - 1; i >= 0; i-- {
		if len(t.Children[i]) > 0 {
			s := 0.0
			for _, c := range t.Children[i] {
				s += t.Value[c]
			}
			t.Value[i] = s
		}
	}
}

// SortByValue orders every node's children by value, largest first
// (stable), as treemaps and packs lay out best that way.
func (t *Tree) SortByValue() {
	for i := range t.Children {
		c := t.Children[i]
		sort.SliceStable(c, func(a, b int) bool { return totalLess(t.Value[c[b]], t.Value[c[a]]) })
	}
}

// postOrder lists nodes children left to right before their parent.
func (t *Tree) postOrder() []int {
	type frame struct{ n, k int }
	out := make([]int, 0, t.Len())
	stack := []frame{{0, 0}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.k < len(t.Children[top.n]) {
			c := t.Children[top.n][top.k]
			top.k++
			stack = append(stack, frame{c, 0})
		} else {
			out = append(out, top.n)
			stack = stack[:len(stack)-1]
		}
	}
	return out
}

// preOrder lists nodes in preorder by traversal (which differs from index
// order once children have been re-sorted).
func (t *Tree) preOrder() []int {
	out := make([]int, 0, t.Len())
	stack := []int{0}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		out = append(out, n)
		for k := len(t.Children[n]) - 1; k >= 0; k-- {
			stack = append(stack, t.Children[n][k])
		}
	}
	return out
}

// totalLess is Rust's f64::total_cmp as a less-than.
func totalLess(a, b float64) bool {
	key := func(f float64) int64 {
		v := int64(math.Float64bits(f))
		return v ^ int64(uint64(v>>63)>>1)
	}
	return key(a) < key(b)
}

// rustMax is Rust's f64::max: a NaN operand is ignored.
func rustMax(a, b float64) float64 {
	switch {
	case math.IsNaN(a):
		return b
	case math.IsNaN(b):
		return a
	}
	return math.Max(a, b)
}

// phi is the golden ratio d3 targets for treemap cell aspect ratios.
const phi = 1.618033988749895

// Cell is a rectangle by its corners: x0 ≤ x1, y0 ≤ y1.
type Cell struct{ X0, Y0, X1, Y1 float64 }

// Squarify is a squarified treemap of one level: boxes for values (in
// order; zeros get empty boxes) tiling b.
func Squarify(values []float64, b Cell) []Cell {
	x0, y0, x1, y1 := b.X0, b.Y0, b.X1, b.Y1
	n := len(values)
	out := make([]Cell, n)
	for i := range out {
		out[i] = Cell{x0, y0, x0, y0}
	}
	value := 0.0
	for _, v := range values {
		value += v
	}
	i0, i1 := 0, 0
	for i0 < n {
		dx, dy := x1-x0, y1-y0
		// Find the next non-empty node.
		var sumValue float64
		for {
			sumValue = values[i1]
			i1++
			if sumValue != 0 || i1 >= n {
				break
			}
		}
		minValue, maxValue := sumValue, sumValue
		alpha := rustMax(dy/dx, dx/dy) / (value * phi)
		beta := sumValue * sumValue * alpha
		minRatio := rustMax(maxValue/beta, beta/minValue)
		// Keep adding nodes while the aspect ratio maintains or improves.
		for i1 < n {
			nodeValue := values[i1]
			sumValue += nodeValue
			if nodeValue < minValue {
				minValue = nodeValue
			}
			if nodeValue > maxValue {
				maxValue = nodeValue
			}
			beta = sumValue * sumValue * alpha
			newRatio := rustMax(maxValue/beta, beta/minValue)
			if newRatio > minRatio {
				sumValue -= nodeValue
				break
			}
			minRatio = newRatio
			i1++
		}
		row := values[i0:i1]
		if dx < dy {
			// Dice: the row runs across the top.
			y := y1
			if value != 0 {
				y = y0 + dy*sumValue/value
			}
			k := 0.0
			if sumValue != 0 {
				k = (x1 - x0) / sumValue
			}
			x := x0
			for j, v := range row {
				nx := x + float64(v*k)
				out[i0+j] = Cell{x, y0, nx, y}
				x = nx
			}
			y0 = y
		} else {
			// Slice: the row runs down the left side.
			x := x1
			if value != 0 {
				x = x0 + dx*sumValue/value
			}
			k := 0.0
			if sumValue != 0 {
				k = (y1 - y0) / sumValue
			}
			y := y0
			for j, v := range row {
				ny := y + float64(v*k)
				out[i0+j] = Cell{x0, y, x, ny}
				y = ny
			}
			x0 = x
		}
		value -= sumValue
		i0 = i1
	}
	return out
}

// Circle is a packed circle: center and radius.
type Circle struct{ X, Y, R float64 }

// lcg is d3's linear congruential generator for the enclosing circle's
// shuffle (and the force layout's start positions): the shuffle only guards
// against worst-case input order, so a fixed sequence keeps layouts
// reproducible across runs and ports.
type lcg struct{ s uint64 }

func (l *lcg) next() float64 {
	l.s = (1664525*l.s + 1013904223) % 4294967296
	return float64(l.s) / 4294967296
}

func place(b, a Circle, c *Circle) {
	dx, dy := b.X-a.X, b.Y-a.Y
	d2 := float64(dx*dx) + float64(dy*dy)
	if d2 != 0 {
		a2 := (a.R + c.R) * (a.R + c.R)
		b2 := (b.R + c.R) * (b.R + c.R)
		if a2 > b2 {
			x := (d2 + b2 - a2) / (2 * d2)
			y := math.Sqrt(rustMax(b2/d2-float64(x*x), 0))
			c.X = b.X - float64(x*dx) - float64(y*dy)
			c.Y = b.Y - float64(x*dy) + float64(y*dx)
		} else {
			x := (d2 + a2 - b2) / (2 * d2)
			y := math.Sqrt(rustMax(a2/d2-float64(x*x), 0))
			c.X = a.X + float64(x*dx) - float64(y*dy)
			c.Y = a.Y + float64(x*dy) + float64(y*dx)
		}
	} else {
		c.X = a.X + c.R
		c.Y = a.Y
	}
}

func intersects(a, b Circle) bool {
	dr := a.R + b.R - 1e-6
	dx, dy := b.X-a.X, b.Y-a.Y
	return dr > 0 && float64(dr*dr) > float64(dx*dx)+float64(dy*dy)
}

// packSiblings packs circles (radii given) side by side around the origin;
// returns the radius of the circle enclosing them all, centered on the
// origin.
func packSiblings(circles []Circle, rng *lcg) float64 {
	n := len(circles)
	if n == 0 {
		return 0
	}
	circles[0].X = 0
	circles[0].Y = 0
	if n == 1 {
		return circles[0].R
	}
	circles[0].X = -circles[1].R
	circles[1].X = circles[0].R
	circles[1].Y = 0
	if n == 2 {
		return circles[0].R + circles[1].R
	}
	place(circles[1], circles[0], &circles[2])
	// The front chain as a circular doubly linked list over indices.
	next := make([]int, n)
	prev := make([]int, n)
	a, b := 0, 1
	next[0] = 1
	prev[1] = 0
	next[1] = 2
	prev[2] = 1
	next[2] = 0
	prev[0] = 2
	score := func(i int) float64 {
		p, q := circles[i], circles[next[i]]
		ab := p.R + q.R
		dx := (float64(p.X*q.R) + float64(q.X*p.R)) / ab
		dy := (float64(p.Y*q.R) + float64(q.Y*p.R)) / ab
		return float64(dx*dx) + float64(dy*dy)
	}
	i := 3
pack:
	for i < n {
		place(circles[a], circles[b], &circles[i])
		c := i
		// Find the closest intersecting circle on the front chain, if any.
		j, k := next[b], prev[a]
		sj, sk := circles[b].R, circles[a].R
		for {
			if sj <= sk {
				if intersects(circles[j], circles[c]) {
					b = j
					next[a] = b
					prev[b] = a
					continue pack
				}
				sj += circles[j].R
				j = next[j]
			} else {
				if intersects(circles[k], circles[c]) {
					a = k
					next[a] = b
					prev[b] = a
					continue pack
				}
				sk += circles[k].R
				k = prev[k]
			}
			if j == next[k] {
				break
			}
		}
		// Insert c between a and b.
		prev[c] = a
		next[c] = b
		next[a] = c
		prev[b] = c
		b = c
		// The new closest pair to the centroid.
		aa := score(a)
		for d := next[c]; d != b; d = next[d] {
			if ca := score(d); ca < aa {
				a = d
				aa = ca
			}
		}
		b = next[a]
		i++
	}
	// Enclose the front chain and center it on the origin.
	chain := []Circle{circles[b]}
	for c := next[b]; c != b; c = next[c] {
		chain = append(chain, circles[c])
	}
	e := enclose(chain, rng)
	for k := range circles {
		circles[k].X -= e.X
		circles[k].Y -= e.Y
	}
	return e.R
}

func enclosesNot(a, b Circle) bool {
	dr := a.R - b.R
	dx, dy := b.X-a.X, b.Y-a.Y
	return dr < 0 || float64(dr*dr) < float64(dx*dx)+float64(dy*dy)
}

func enclosesWeak(a, b Circle) bool {
	dr := a.R - b.R + float64(rustMax(rustMax(a.R, b.R), 1)*1e-9)
	dx, dy := b.X-a.X, b.Y-a.Y
	return dr > 0 && float64(dr*dr) > float64(dx*dx)+float64(dy*dy)
}

func enclosesWeakAll(a Circle, basis []Circle) bool {
	for _, b := range basis {
		if !enclosesWeak(a, b) {
			return false
		}
	}
	return true
}

func basis2(a, b Circle) Circle {
	x21, y21, r21 := b.X-a.X, b.Y-a.Y, b.R-a.R
	l := math.Sqrt(float64(x21*x21) + float64(y21*y21))
	return Circle{
		X: (a.X + b.X + float64(x21/l*r21)) / 2,
		Y: (a.Y + b.Y + float64(y21/l*r21)) / 2,
		R: (l + a.R + b.R) / 2,
	}
}

func basis3(a, b, c Circle) Circle {
	x1, y1, r1 := a.X, a.Y, a.R
	x2, y2, r2 := b.X, b.Y, b.R
	x3, y3, r3 := c.X, c.Y, c.R
	a2, a3 := x1-x2, x1-x3
	b2, b3 := y1-y2, y1-y3
	c2, c3 := r2-r1, r3-r1
	d1 := float64(x1*x1) + float64(y1*y1) - float64(r1*r1)
	d2 := d1 - float64(x2*x2) - float64(y2*y2) + float64(r2*r2)
	d3 := d1 - float64(x3*x3) - float64(y3*y3) + float64(r3*r3)
	ab := float64(a3*b2) - float64(a2*b3)
	xa := (float64(b2*d3)-float64(b3*d2))/(ab*2) - x1
	xb := (float64(b3*c2) - float64(b2*c3)) / ab
	ya := (float64(a3*d2)-float64(a2*d3))/(ab*2) - y1
	yb := (float64(a2*c3) - float64(a3*c2)) / ab
	qa := float64(xb*xb) + float64(yb*yb) - 1
	qb := 2 * (r1 + float64(xa*xb) + float64(ya*yb))
	qc := float64(xa*xa) + float64(ya*ya) - float64(r1*r1)
	var r float64
	if math.Abs(qa) > 1e-6 {
		r = -((qb + math.Sqrt(float64(qb*qb)-float64(4*qa*qc))) / (2 * qa))
	} else {
		r = -(qc / qb)
	}
	return Circle{X: x1 + xa + float64(xb*r), Y: y1 + ya + float64(yb*r), R: r}
}

func basis(b []Circle) Circle {
	switch len(b) {
	case 1:
		return b[0]
	case 2:
		return basis2(b[0], b[1])
	case 3:
		return basis3(b[0], b[1], b[2])
	}
	panic("a basis has one to three circles")
}

func extendBasis(b []Circle, p Circle) []Circle {
	if enclosesWeakAll(p, b) {
		return []Circle{p}
	}
	for _, bi := range b {
		if enclosesNot(p, bi) && enclosesWeakAll(basis2(bi, p), b) {
			return []Circle{bi, p}
		}
	}
	for i := 0; i < len(b)-1; i++ {
		for j := i + 1; j < len(b); j++ {
			if enclosesNot(basis2(b[i], b[j]), p) &&
				enclosesNot(basis2(b[i], p), b[j]) &&
				enclosesNot(basis2(b[j], p), b[i]) &&
				enclosesWeakAll(basis3(b[i], b[j], p), b) {
				return []Circle{b[i], b[j], p}
			}
		}
	}
	panic("Welzl's basis always extends")
}

// enclose is the smallest circle enclosing circles (Welzl's algorithm,
// after a shuffle); it reorders circles.
func enclose(circles []Circle, rng *lcg) Circle {
	for m := len(circles); m > 0; {
		i := int(rng.next() * float64(m))
		m--
		circles[m], circles[i] = circles[i], circles[m]
	}
	var b []Circle
	var e Circle
	have := false
	for i := 0; i < len(circles); {
		p := circles[i]
		if have && enclosesWeak(e, p) {
			i++
			continue
		}
		b = extendBasis(b, p)
		e = basis(b)
		have = true
		i = 0
	}
	return e
}

// Pack is a circle packing of the whole tree into a w × h box: leaf areas
// proportional to value, about padding px between siblings and around them
// inside their parent. Values must be summed first. Only about: as in d3, the
// padding is converted to layout units with the scale of an unpadded first
// pass, and padding changes that scale (10px comes out near 9px).
func Pack(t *Tree, w, h, padding float64) []Circle {
	// Leaf radii start as √value, so areas follow values.
	c := make([]Circle, t.Len())
	for i := range c {
		if len(t.Children[i]) == 0 {
			c[i].R = math.Sqrt(rustMax(t.Value[i], 0))
		}
	}
	rng := lcg{1}
	post := t.postOrder()
	packChildren := func(k, pad float64) {
		for _, i := range post {
			kids := t.Children[i]
			if len(kids) == 0 {
				continue
			}
			r := pad * k
			sib := make([]Circle, len(kids))
			for j, kid := range kids {
				sib[j] = c[kid]
			}
			if r != 0 {
				for j := range sib {
					sib[j].R += r
				}
			}
			e := packSiblings(sib, &rng)
			if r != 0 {
				for j := range sib {
					sib[j].R -= r
				}
			}
			for j, kid := range kids {
				c[kid] = sib[j]
			}
			c[i].R = e + r
		}
	}
	packChildren(1, 0)
	k := c[0].R / math.Min(w, h)
	packChildren(k, padding)
	k = math.Min(w, h) / (2 * c[0].R)
	c[0].X = w / 2
	c[0].Y = h / 2
	for _, i := range t.preOrder() {
		c[i].R *= k
		if p := t.Parent[i]; p >= 0 {
			c[i].X = c[p].X + float64(k*c[i].X)
			c[i].Y = c[p].Y + float64(k*c[i].Y)
		}
	}
	return c
}

// Tidy is Buchheim's linear-time tidy tree (Reingold–Tilford): each node's
// breadth position in [0, 1], subtrees never overlapping, parents centered
// over their children. separation(sameParent, depth) is the minimum gap
// between neighbours, in units of the tightest gap.
func Tidy(t *Tree, separation func(sameParent bool, depth int) float64) []float64 {
	n := t.Len()
	if n == 0 {
		return nil
	}
	// Node n is a pseudo-root above the real one, as in d3.
	parent := make([]int, n)
	for i, p := range t.Parent {
		if p < 0 {
			p = n
		}
		parent[i] = p
	}
	rootKids := []int{0}
	kids := func(v int) []int {
		if v == n {
			return rootKids
		}
		return t.Children[v]
	}
	index := make([]int, n)
	for v := range n {
		for k, c := range t.Children[v] {
			index[c] = k
		}
	}
	ancestorOf := make([]int, n+1) // `a`
	bigA := make([]int, n+1)       // `A`; -1 for none
	thread := make([]int, n+1)     // `t`; -1 for none
	for i := range ancestorOf {
		ancestorOf[i], bigA[i], thread[i] = i, -1, -1
	}
	z, m, c, s := make([]float64, n+1), make([]float64, n+1), make([]float64, n+1), make([]float64, n+1)
	sep := func(a, b int) float64 { return separation(parent[a] == parent[b], t.Depth[a]) }
	nextLeft := func(v int) int {
		if k := kids(v); len(k) > 0 {
			return k[0]
		}
		return thread[v]
	}
	nextRight := func(v int) int {
		if k := kids(v); len(k) > 0 {
			return k[len(k)-1]
		}
		return thread[v]
	}

	for _, v := range t.postOrder() {
		siblings := kids(parent[v])
		w := -1
		if index[v] > 0 {
			w = siblings[index[v]-1]
		}
		ch := kids(v)
		if len(ch) > 0 {
			// Execute shifts.
			shift, change := 0.0, 0.0
			for k := len(ch) - 1; k >= 0; k-- {
				x := ch[k]
				z[x] += shift
				m[x] += shift
				change += c[x]
				shift += s[x] + change
			}
			midpoint := (z[ch[0]] + z[ch[len(ch)-1]]) / 2
			if w >= 0 {
				z[v] = z[w] + sep(v, w)
				m[v] = z[v] - midpoint
			} else {
				z[v] = midpoint
			}
		} else if w >= 0 {
			z[v] = z[w] + sep(v, w)
		}
		// Apportion.
		ancestor := bigA[parent[v]]
		if ancestor < 0 {
			ancestor = siblings[0]
		}
		if w >= 0 {
			vip, vop, vim := v, v, w
			vom := siblings[0]
			sip, sop, sim, som := m[vip], m[vop], m[vim], m[vom]
			nim, nip := nextRight(vim), nextLeft(vip)
			for nim >= 0 && nip >= 0 {
				vim, vip = nim, nip
				vom = nextLeft(vom)
				vop = nextRight(vop)
				ancestorOf[vop] = v
				shift := z[vim] + sim - z[vip] - sip + sep(vim, vip)
				if shift > 0 {
					wm := ancestor
					if parent[ancestorOf[vim]] == parent[v] {
						wm = ancestorOf[vim]
					}
					// Move subtree.
					change := shift / (float64(index[v]) - float64(index[wm]))
					c[v] -= change
					s[v] += shift
					c[wm] += change
					z[v] += shift
					m[v] += shift
					sip += shift
					sop += shift
				}
				sim += m[vim]
				sip += m[vip]
				som += m[vom]
				sop += m[vop]
				nim, nip = nextRight(vim), nextLeft(vip)
			}
			if nim >= 0 && nextRight(vop) < 0 {
				thread[vop] = nim
				m[vop] += sim - sop
			}
			if nip >= 0 && nextLeft(vom) < 0 {
				thread[vom] = nip
				m[vom] += sip - som
				ancestor = v
			}
		}
		bigA[parent[v]] = ancestor
	}
	m[n] = -z[0]
	x := make([]float64, n)
	pre := t.preOrder()
	for _, v := range pre {
		x[v] = z[v] + m[parent[v]]
		m[v] += m[parent[v]]
	}
	// Normalize to [0, 1], leaving half a separation at either end.
	left, right := 0, 0
	for _, v := range pre {
		if x[v] < x[left] {
			left = v
		}
		if x[v] > x[right] {
			right = v
		}
	}
	sp := 1.0
	if left != right {
		sp = sep(left, right) / 2
	}
	tx := sp - x[left]
	kx := 1 / (x[right] + sp + tx)
	for i, v := range x {
		x[i] = (v + tx) * kx
	}
	return x
}
