package charta

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/dmath"
	"github.com/linzeyan/charta/scene"
)

const (
	// maxSets: past five sets the overlaps stop being readable as circles.
	maxSets = 5
	// vennDisjoint: disjoint sets sit this much further apart than touching,
	// so they do not look like they share a point.
	vennDisjoint = 1.1
	vennOutline  = 1.5
	// vennOnEdge is the slack for a crossing point to count as inside a
	// circle it lies on.
	vennOnEdge = 1e-10
)

// vennDirs are the axis and diagonal directions, for placing circles and
// labels.
var vennDirs = [8][2]float64{
	{1, 0}, {0, 1}, {-1, 0}, {0, -1},
	{sqrtHalf, sqrtHalf}, {-sqrtHalf, sqrtHalf}, {-sqrtHalf, -sqrtHalf}, {sqrtHalf, -sqrtHalf},
}

// VennArea is a set (one name) or the overlap of several, and its size.
type VennArea struct {
	Sets []string `json:"sets"`
	Size float64  `json:"size"`
}

// VennChart draws sets as circles, area proportional to size, placed so the
// overlaps match the data. Pairs alone match exactly for two or three sets
// and as near as possible for four or five; an overlap of three or more sets
// given too brings every given area into the fit. A pair left out shares
// nothing.
type VennChart struct {
	Title string     `json:"title,omitempty"`
	Areas []VennArea `json:"areas"`
}

// pairKind is how a pair's target distance constrains the layout.
type pairKind int

const (
	// pairExact: exactly this far apart.
	pairExact pairKind = iota
	// pairAtLeast: at least this far: any further still has no overlap.
	pairAtLeast
	// pairAtMost: at most this far: any closer still has the smaller one
	// inside.
	pairAtMost
)

type pairTarget struct {
	d    float64
	kind pairKind
}

// lens is the area two circles share at center distance d.
func lens(r1, r2, d float64) float64 {
	if d >= r1+r2 {
		return 0
	}
	if d <= math.Abs(r1-r2) {
		r := math.Min(r1, r2)
		return math.Pi * r * r
	}
	clamp := func(v float64) float64 { return math.Max(-1, math.Min(v, 1)) }
	a1 := clamp((float64(d*d) + float64(r1*r1) - float64(r2*r2)) / float64(2*d*r1))
	a2 := clamp((float64(d*d) + float64(r2*r2) - float64(r1*r1)) / float64(2*d*r2))
	k := (-d + r1 + r2) * (d + r1 - r2) * (d - r1 + r2) * (d + r1 + r2)
	return float64(float64(r1*r1)*dmath.Acos(a1)) + float64(float64(r2*r2)*dmath.Acos(a2)) - float64(0.5*math.Sqrt(math.Max(k, 0)))
}

// vennDistance is the distance at which two circles share overlap, and how
// strictly.
func vennDistance(r1, r2, overlap float64) pairTarget {
	smaller := math.Pi * math.Min(r1, r2) * math.Min(r1, r2)
	if overlap <= 0 {
		return pairTarget{float64(vennDisjoint * (r1 + r2)), pairAtLeast}
	}
	if overlap >= smaller {
		// Inside, centered halfway between concentric and touching.
		return pairTarget{math.Abs(r1-r2) / 2, pairAtMost}
	}
	// The shared area shrinks as the circles part: bisect.
	lo, hi := math.Abs(r1-r2), r1+r2
	for range 60 {
		mid := (lo + hi) / 2
		if lens(r1, r2, mid) > overlap {
			lo = mid
		} else {
			hi = mid
		}
	}
	return pairTarget{(lo + hi) / 2, pairExact}
}

// stress is how far the layout is from every pair's target; only the
// targets among the first len(p) sets count.
func stress(p [][2]float64, target [][]pairTarget) float64 {
	s := 0.0
	for i := range p {
		for j := i + 1; j < len(p); j++ {
			dx, dy := p[i][0]-p[j][0], p[i][1]-p[j][1]
			d := math.Sqrt(float64(dx*dx) + float64(dy*dy))
			t := target[i][j]
			e := d - t.d
			if (t.kind == pairAtLeast && d >= t.d) || (t.kind == pairAtMost && d <= t.d) {
				e = 0
			}
			s += float64(e * e)
		}
	}
	return s
}

type vertex struct {
	x []float64
	v float64
}

// nelderMead minimizes f with the standard coefficients and a fixed
// iteration budget: no randomness and no early exit on timing, so both
// ports take the same steps.
func nelderMead(f func([]float64) float64, start []float64, step float64) []float64 {
	n := len(start)
	simplex := make([]vertex, n+1)
	for k := range simplex {
		x := slices.Clone(start)
		if k > 0 {
			x[k-1] += step
		}
		simplex[k] = vertex{x, f(x)}
	}
	byValue := func(a, b vertex) int {
		switch {
		case totalLess(a.v, b.v):
			return -1
		case totalLess(b.v, a.v):
			return 1
		}
		return 0
	}
	for range 200 * n {
		slices.SortStableFunc(simplex, byValue)
		if simplex[n].v-simplex[0].v < 1e-12 {
			break
		}
		centroid := make([]float64, n)
		for d := range n {
			s := 0.0
			for _, v := range simplex[:n] {
				s += v.x[d]
			}
			centroid[d] = s / float64(n)
		}
		toward := func(t float64) []float64 {
			x := make([]float64, n)
			for d := range n {
				x[d] = centroid[d] + float64(t*(simplex[n].x[d]-centroid[d]))
			}
			return x
		}
		r := toward(-1)
		fr := f(r)
		switch {
		case fr < simplex[0].v:
			e := toward(-2)
			if fe := f(e); fe < fr {
				simplex[n] = vertex{e, fe}
			} else {
				simplex[n] = vertex{r, fr}
			}
		case fr < simplex[n-1].v:
			simplex[n] = vertex{r, fr}
		default:
			t := 0.5
			if fr < simplex[n].v {
				t = -0.5
			}
			c := toward(t)
			if fc := f(c); fc < math.Min(fr, simplex[n].v) {
				simplex[n] = vertex{c, fc}
			} else {
				// Shrink everything toward the best point.
				best := slices.Clone(simplex[0].x)
				for k := 1; k <= n; k++ {
					for d, b := range best {
						simplex[k].x[d] = b + float64(0.5*(simplex[k].x[d]-b))
					}
					simplex[k].v = f(simplex[k].x)
				}
			}
		}
	}
	slices.SortStableFunc(simplex, byValue)
	return simplex[0].x
}

// vennLayout places the circle centers to meet the pairwise targets.
func vennLayout(r []float64, target [][]pairTarget) [][2]float64 {
	n := len(r)
	switch n {
	case 1:
		return [][2]float64{{0, 0}}
	case 2:
		return [][2]float64{{0, 0}, {target[0][1].d, 0}}
	case 3:
		// The triangle with the three target distances as sides, the third
		// set below the first two.
		a, b, c := target[0][1].d, target[0][2].d, target[1][2].d
		x, y := b, 0.0
		if a > 0 {
			x = (float64(a*a) + float64(b*b) - float64(c*c)) / float64(2*a)
			y = math.Sqrt(math.Max(float64(b*b)-float64(x*x), 0))
		}
		return [][2]float64{{0, 0}, {a, 0}, {x, y}}
	}
	// Greedy start: each set at the best of eight spots at its target
	// distance from each placed set, then refine together.
	p := [][2]float64{{0, 0}}
	for i := 1; i < n; i++ {
		bestS, best := math.Inf(1), [2]float64{}
		for j, q := range slices.Clone(p) {
			for _, u := range vennDirs {
				t := target[j][i].d
				c := [2]float64{q[0] + float64(t*u[0]), q[1] + float64(t*u[1])}
				p = append(p, c)
				s := stress(p, target)
				p = p[:len(p)-1]
				if s < bestS {
					bestS, best = s, c
				}
			}
		}
		p = append(p, best)
	}
	flat := make([]float64, 0, 2*n)
	for _, q := range p {
		flat = append(flat, q[0], q[1])
	}
	sumR := 0.0
	for _, v := range r {
		sumR += v
	}
	meanR := sumR / float64(n)
	x := nelderMead(func(x []float64) float64 {
		q := make([][2]float64, len(x)/2)
		for k := range q {
			q[k] = [2]float64{x[2*k], x[2*k+1]}
		}
		return stress(q, target)
	}, flat, meanR/10)
	out := make([][2]float64, n)
	for k := range out {
		out[k] = [2]float64{x[2*k], x[2*k+1]}
	}
	return out
}

// vennCircle is a circle as center and radius.
type vennCircle struct{ x, y, r float64 }

func apart(x, y float64, c vennCircle) float64 {
	dx, dy := x-c.x, y-c.y
	return math.Sqrt(float64(dx*dx) + float64(dy*dy))
}

// crossings are the two points where the circles' edges cross, if they do.
func crossings(a, b vennCircle) ([2][2]float64, bool) {
	dx, dy := b.x-a.x, b.y-a.y
	d := math.Sqrt(float64(dx*dx) + float64(dy*dy))
	if d >= a.r+b.r || d <= math.Abs(a.r-b.r) {
		return [2][2]float64{}, false
	}
	along := (float64(a.r*a.r) - float64(b.r*b.r) + float64(d*d)) / (2 * d)
	h := math.Sqrt(math.Max(float64(a.r*a.r)-float64(along*along), 0))
	x0, y0 := a.x+float64(along*dx)/d, a.y+float64(along*dy)/d
	rx, ry := float64(-dy*(h/d)), float64(-dx*(h/d))
	return [2][2]float64{{x0 + rx, y0 - ry}, {x0 - rx, y0 + ry}}, true
}

// segment is the area a chord cuts off a circle of radius r, w deep.
func segment(r, w float64) float64 {
	return float64(float64(r*r)*dmath.Acos(1-w/r)) - float64((r-w)*math.Sqrt(float64(w*(2*r-w))))
}

type vennCorner struct {
	x, y float64
	on   [2]int
}

// commonArea is the area every circle covers, ported from venn.js
// intersectionArea (MIT license, Ben Frederickson): the polygon through the
// crossing points inside all circles, plus the circular segment beyond each
// of its edges.
func commonArea(c []vennCircle) float64 {
	// Each corner with the two circles it lies on.
	var corners []vennCorner
	for i := range c {
		for j := i + 1; j < len(c); j++ {
			ps, ok := crossings(c[i], c[j])
			if !ok {
				continue
			}
			for _, p := range ps {
				inAll := true
				for _, k := range c {
					inAll = inAll && apart(p[0], p[1], k) <= k.r+vennOnEdge
				}
				if inAll {
					corners = append(corners, vennCorner{p[0], p[1], [2]int{i, j}})
				}
			}
		}
	}
	if len(corners) < 2 {
		// No region with corners: the smallest circle lies inside all the
		// others, or they have nothing in common.
		s := c[0]
		for _, k := range c {
			if k.r < s.r {
				s = k
			}
		}
		for _, k := range c {
			if apart(s.x, s.y, k) > math.Abs(s.r-k.r) {
				return 0
			}
		}
		return math.Pi * s.r * s.r
	}
	m := float64(len(corners))
	mx, my := 0.0, 0.0
	for _, p := range corners {
		mx += p.x
	}
	for _, p := range corners {
		my += p.y
	}
	mx, my = mx/m, my/m
	// Around the centroid, so consecutive corners are the polygon's edges.
	type angled struct {
		a float64
		k int
	}
	around := make([]angled, len(corners))
	for k, p := range corners {
		around[k] = angled{dmath.Atan2(p.x-mx, p.y-my), k}
	}
	slices.SortStableFunc(around, func(a, b angled) int {
		switch {
		case totalLess(b.a, a.a):
			return -1
		case totalLess(a.a, b.a):
			return 1
		}
		return 0
	})
	polygon, arcs := 0.0, 0.0
	p2 := corners[around[len(around)-1].k]
	for _, o := range around {
		p1 := corners[o.k]
		polygon += float64((p2.x + p1.x) * (p1.y - p2.y))
		midX, midY := (p1.x+p2.x)/2, (p1.y+p2.y)/2
		// Of the circles both corners lie on, the edge follows the one whose
		// arc between them bulges least.
		found, aw, ar := false, 0.0, 0.0
		for _, ci := range p1.on {
			if ci != p2.on[0] && ci != p2.on[1] {
				continue
			}
			k := c[ci]
			a1 := dmath.Atan2(p1.x-k.x, p1.y-k.y)
			a2 := dmath.Atan2(p2.x-k.x, p2.y-k.y)
			turn := a2 - a1
			if turn < 0 {
				turn += 2 * math.Pi
			}
			sin, cos := dmath.Sincos(a2 - turn/2)
			w := math.Min(apart(midX, midY, vennCircle{k.x + float64(k.r*sin), k.y + float64(k.r*cos), 0}), 2*k.r)
			if !found || aw > w {
				found, aw, ar = true, w, k.r
			}
		}
		if found {
			arcs += segment(ar, aw)
			p2 = p1
		}
	}
	return arcs + polygon/2
}

// vennGiven is an overlap from the data: its sets and size.
type vennGiven struct {
	ids  []int
	size float64
}

// vennRefine moves the circles so every given overlap comes as close as it
// can to its size, starting from the pairwise layout; pairs that share
// nothing keep their gap instead.
func vennRefine(at [][2]float64, r []float64, target [][]pairTarget, given []vennGiven) [][2]float64 {
	n := len(r)
	// Both errors as shares, of the mean set's area and radius, so neither
	// outweighs the other by units alone.
	sumR, sumArea := 0.0, 0.0
	for _, v := range r {
		sumR += v
	}
	for _, v := range r {
		sumArea += float64(math.Pi * v * v)
	}
	meanR, meanArea := sumR/float64(n), sumArea/float64(n)
	loss := func(x []float64) float64 {
		c := make([]vennCircle, n)
		for i := range c {
			c[i] = vennCircle{x[2*i], x[2*i+1], r[i]}
		}
		e := 0.0
		for i := range n {
			for j := i + 1; j < n; j++ {
				if t := target[i][j]; t.kind == pairAtLeast {
					gap := math.Max(t.d-apart(c[i].x, c[i].y, c[j]), 0) / meanR
					e += float64(gap * gap)
				}
			}
		}
		for _, g := range given {
			var have float64
			switch {
			case len(g.ids) == 2 && target[g.ids[0]][g.ids[1]].kind == pairAtLeast:
				continue
			case len(g.ids) == 2:
				i, j := g.ids[0], g.ids[1]
				have = lens(r[i], r[j], apart(c[i].x, c[i].y, c[j]))
			default:
				sub := make([]vennCircle, len(g.ids))
				for k, i := range g.ids {
					sub[k] = c[i]
				}
				have = commonArea(sub)
			}
			d := (have - g.size) / meanArea
			e += float64(d * d)
		}
		return e
	}
	flat := make([]float64, 0, 2*n)
	for _, q := range at {
		flat = append(flat, q[0], q[1])
	}
	x := nelderMead(loss, flat, meanR/10)
	out := make([][2]float64, n)
	for k := range out {
		out[k] = [2]float64{x[2*k], x[2*k+1]}
	}
	return out
}

func (c VennChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	var names []string
	for _, a := range c.Areas {
		if len(a.Sets) == 1 {
			names = append(names, a.Sets[0])
		}
	}
	if len(names) == 0 || len(names) > maxSets {
		return nil, invalid("%d sets: give 1 to %d, each as an area with one name", len(names), maxSets)
	}
	n := len(names)
	size := make([]float64, n)
	overlap := make([][]float64, n)
	for i := range overlap {
		overlap[i] = make([]float64, n)
	}
	keys := make([][]int, 0, len(c.Areas))
	for _, a := range c.Areas {
		what := strings.Join(a.Sets, " ∩ ")
		if err := finite("the size of "+what, a.Size); err != nil {
			return nil, err
		}
		ids := make([]int, 0, len(a.Sets))
		for _, s := range a.Sets {
			i := slices.Index(names, s)
			if i < 0 {
				return nil, invalid("%s names %q, which has no area of its own", what, s)
			}
			if slices.Contains(ids, i) {
				return nil, invalid("%s names %q twice", what, s)
			}
			ids = append(ids, i)
		}
		slices.Sort(ids)
		if slices.ContainsFunc(keys, func(k []int) bool { return slices.Equal(k, ids) }) {
			return nil, invalid("%s is given twice", what)
		}
		keys = append(keys, ids)
		switch {
		case len(ids) == 1:
			if a.Size <= 0 {
				return nil, invalid("set %s needs a positive size", what)
			}
			size[ids[0]] = a.Size
		case a.Size < 0:
			return nil, invalid("%s has a negative size", what)
		case len(ids) == 2:
			overlap[ids[0]][ids[1]] = a.Size
			overlap[ids[1]][ids[0]] = a.Size
		}
	}
	// An overlap cannot hold more than any set in it.
	for k, a := range c.Areas {
		for _, i := range keys[k] {
			if a.Size > size[i] {
				return nil, invalid("%s (%s) is bigger than %q (%s)",
					strings.Join(a.Sets, " ∩ "), base.Value(a.Size), names[i], base.Value(size[i]))
			}
		}
	}
	// Nor more than an overlap of fewer of its sets; and a pair left out
	// shares nothing, so neither can anything more that holds it.
	for k, a := range c.Areas {
		ids := keys[k]
		if len(ids) < 3 || a.Size == 0 {
			continue
		}
		what := strings.Join(a.Sets, " ∩ ")
		for l, b := range c.Areas {
			sub := keys[l]
			inside := len(sub) > 1 && len(sub) < len(ids)
			for _, i := range sub {
				inside = inside && slices.Contains(ids, i)
			}
			if inside && a.Size > b.Size {
				return nil, invalid("%s (%s) is bigger than %s (%s)", what, base.Value(a.Size), strings.Join(b.Sets, " ∩ "), base.Value(b.Size))
			}
		}
		for x, i := range ids {
			for _, j := range ids[x+1:] {
				if !slices.ContainsFunc(keys, func(k []int) bool { return slices.Equal(k, []int{i, j}) }) {
					return nil, invalid("%s is %s but %s ∩ %s is not given, so they share nothing", what, base.Value(a.Size), names[i], names[j])
				}
			}
		}
	}

	r := make([]float64, n)
	for i, s := range size {
		r[i] = math.Sqrt(s / math.Pi)
	}
	target := make([][]pairTarget, n)
	for i := range target {
		target[i] = make([]pairTarget, n)
		for j := range target[i] {
			if i != j {
				target[i][j] = vennDistance(r[i], r[j], overlap[i][j])
			}
		}
	}
	at := vennLayout(r, target)
	if slices.ContainsFunc(keys, func(k []int) bool { return len(k) > 2 }) {
		var given []vennGiven
		for k, a := range c.Areas {
			if len(keys[k]) > 1 {
				given = append(given, vennGiven{keys[k], a.Size})
			}
		}
		at = vennRefine(at, r, target, given)
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	// Room for labels outside the circles on every side.
	labelW := 0.0
	for _, s := range names {
		labelW = math.Max(labelW, m.Width(s, tickSize, base.Semibold))
	}
	mx, my := labelW+gap, tickSize+gap
	left, right, up, down := pad+mx, width-pad-mx, top+my, height-pad-my
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for i, p := range at {
		x0 = math.Min(x0, p[0]-r[i])
		y0 = math.Min(y0, p[1]-r[i])
		x1 = math.Max(x1, p[0]+r[i])
		y1 = math.Max(y1, p[1]+r[i])
	}
	k := math.Min(math.Max(right-left, 1)/(x1-x0), math.Max(down-up, 1)/(y1-y0))
	ox := (left+right)/2 - float64((x0+x1)/2*k)
	oy := (up+down)/2 - float64((y0+y1)/2*k)
	xy := make([]scene.Point, n)
	gx, gy := 0.0, 0.0
	for i, p := range at {
		xy[i] = scene.Point{X: ox + float64(p[0]*k), Y: oy + float64(p[1]*k)}
	}
	for _, p := range xy {
		gx += p.X
	}
	for _, p := range xy {
		gy += p.Y
	}
	gx, gy = gx/float64(n), gy/float64(n)

	for i := range n {
		var rows []scene.TipRow
		for a, ids := range keys {
			if len(ids) < 2 || !slices.Contains(ids, i) {
				continue
			}
			var others []string
			for _, j := range ids {
				if j != i {
					others = append(others, names[j])
				}
			}
			rows = append(rows, scene.TipRow{Name: "∩ " + strings.Join(others, " ∩ "), Value: base.Value(c.Areas[a].Size)})
		}
		marks = append(marks, scene.Mark{
			Shape:  scene.CircleShape{CX: xy[i].X, CY: xy[i].Y, R: r[i] * k},
			Fill:   base.Link(i),
			Stroke: &scene.Stroke{Color: base.Series(i), Width: vennOutline},
			Hit:    &scene.Hit{Key: fmt.Sprintf("set-%d", i), Label: names[i], Value: base.Value(size[i]), Rows: rows},
		})
	}
	// Each name just outside its circle: straight away from the others if
	// that spot is clear, else whichever of eight directions puts the name
	// over the fewest other circles, inside the chart.
	insideOther := func(i int, px, py float64) bool {
		for j := range n {
			dx, dy := px-xy[j].X, py-xy[j].Y
			if j != i && math.Sqrt(float64(dx*dx)+float64(dy*dy)) < float64(r[j]*k) {
				return true
			}
		}
		return false
	}
	for i := range n {
		cx, cy := xy[i].X, xy[i].Y
		dx, dy := cx-gx, cy-gy
		l := math.Sqrt(float64(dx*dx) + float64(dy*dy))
		away := [2]float64{0, -1}
		if l > 1e-9 {
			away = [2]float64{dx / l, dy / l}
		}
		w := m.Width(names[i], tickSize, base.Semibold)
		reach := float64(r[i]*k) + gap/2
		bestScore, bestX, bestY := math.Inf(1), 0.0, 0.0
		bestAnchor, bestBase := scene.Middle, scene.Center
		for _, u := range append([][2]float64{away}, vennDirs[:]...) {
			ux, uy := u[0], u[1]
			x, y := cx+float64(ux*reach), cy+float64(uy*reach)
			var anchor scene.Anchor
			var baseline scene.Baseline
			var bx, by float64
			switch {
			case math.Abs(ux) >= math.Abs(uy) && ux > 0:
				anchor, baseline, bx, by = scene.Start, scene.Center, x, y-tickSize/2
			case math.Abs(ux) >= math.Abs(uy):
				anchor, baseline, bx, by = scene.End, scene.Center, x-w, y-tickSize/2
			case uy > 0:
				anchor, baseline, bx, by = scene.Middle, scene.Top, x-w/2, y
			default:
				anchor, baseline, bx, by = scene.Middle, scene.Bottom, x-w/2, y-tickSize
			}
			covered := 0
			for _, q := range [5][2]float64{{bx, by}, {bx + w, by}, {bx, by + tickSize}, {bx + w, by + tickSize}, {bx + w/2, by + tickSize/2}} {
				if insideOther(i, q[0], q[1]) {
					covered++
				}
			}
			out := bx < pad || by < top || bx+w > width-pad || by+tickSize > height-pad
			outCost := 0.0
			if out {
				outCost = 100
			}
			score := float64(10*float64(covered)) + outCost - (float64(ux*away[0]) + float64(uy*away[1]))
			if score < bestScore {
				bestScore, bestX, bestY, bestAnchor, bestBase = score, x, y, anchor, baseline
			}
		}
		marks = append(marks, text(bestX, bestY, names[i], tickSize, base.Semibold, bestAnchor, bestBase, base.TextSecondary))
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Venn diagram of %s.", count(n, "set", "sets"))),
		Marks:  marks,
	}, nil
}
