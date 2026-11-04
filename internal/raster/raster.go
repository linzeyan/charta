// Package raster paints a scene into an image in pure Go
// (golang.org/x/image/vector, no cgo): marks, hover emphasis and the
// tooltip, with text drawn from caller-supplied fonts. The Fyne backend and
// PNG output share it.
package raster

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"slices"

	"golang.org/x/image/vector"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// Tooltip geometry (logical px), exported for backends' tests that aim at
// the tooltip.
const (
	TipPad = 8.0
	// TipLine is the tooltip's line height.
	TipLine = tipSize * 1.4
	// TipOffset places the tooltip beside the pointer, clear of the cursor.
	TipOffset = 12.0
	// TipKeyW is the width of a crosshair row's series key, a short stroke in
	// the series color.
	TipKeyW = 12.0
)

const (
	// lift lightens the hovered mark toward white, like the HTML runtime's
	// brightness(1.12).
	lift = 0.12
	// dim is the opacity of other series while one series is highlighted.
	dim       = 0.35
	tipSize   = 12.0
	tipPad    = TipPad
	tipLine   = TipLine
	tipOffset = TipOffset
	tipRadius = 6.0
	tipKeyW   = TipKeyW
	tipKeyH   = 2.0
	tipGap    = 6.0
	// kappa places cubic control points so each corner is a quarter circle.
	kappa = 0.5522848
)

// canvasState paints one frame: logical coordinates in, device pixels out.
type canvasState struct {
	dst   *image.RGBA
	scale float64
	dark  bool
	fonts *Fonts
	z     vector.Rasterizer
}

// Pointer is where the pointer is, in logical px: the tooltip sits beside
// it.
type Pointer struct{ X, Y float64 }

// pt is a device-space point.
type pt struct{ x, y float64 }

// path is a list of closed subpaths in device space. The vector rasterizer
// has no stroking, so strokes are expanded into filled outlines; every
// outline that should add coverage is traced with positive area, holes
// with negative area, because the rasterizer sums signed coverage.
type path struct {
	subs [][]seg
}

type segKind uint8

const (
	segLine segKind = iota
	segCube
)

type seg struct {
	kind segKind
	p    [3]pt
}

func (p *path) begin(at pt) { p.subs = append(p.subs, []seg{{kind: segLine, p: [3]pt{at}}}) }

func (p *path) line(to pt) {
	k := len(p.subs) - 1
	p.subs[k] = append(p.subs[k], seg{kind: segLine, p: [3]pt{to}})
}

func (p *path) cube(c1, c2, to pt) {
	k := len(p.subs) - 1
	p.subs[k] = append(p.subs[k], seg{kind: segCube, p: [3]pt{c1, c2, to}})
}

// polygon adds a closed polygon, reversed when needed so its signed area has
// the requested sign.
func (p *path) polygon(pts []pt, positive bool) {
	area := 0.0
	for i, a := range pts {
		b := pts[(i+1)%len(pts)]
		area += a.x*b.y - b.x*a.y
	}
	if (area > 0) != positive {
		pts = slices.Clone(pts)
		slices.Reverse(pts)
	}
	p.begin(pts[0])
	for _, q := range pts[1:] {
		p.line(q)
	}
}

// roundedRect traces a rect with per-corner radii (device px, radii clamped
// to half the shorter side); positive area, or negative for a hole.
func (p *path) roundedRect(x, y, w, h float64, c scene.Corners, s float64, positive bool) {
	lim := math.Min(w, h) / 2
	r := func(v float64) float64 { return math.Max(0, math.Min(v*s, lim)) }
	tl, tr, br, bl := r(c.TL), r(c.TR), r(c.BR), r(c.BL)
	if !positive {
		// Mirror horizontally to flip the winding; the outline is the same.
		tl, tr, bl, br = tr, tl, br, bl
		defer p.mirror(len(p.subs), x+w/2)
	}
	k := 1 - kappa
	p.begin(pt{x + tl, y})
	p.line(pt{x + w - tr, y})
	p.cube(pt{x + w - tr*k, y}, pt{x + w, y + tr*k}, pt{x + w, y + tr})
	p.line(pt{x + w, y + h - br})
	p.cube(pt{x + w, y + h - br*k}, pt{x + w - br*k, y + h}, pt{x + w - br, y + h})
	p.line(pt{x + bl, y + h})
	p.cube(pt{x + bl*k, y + h}, pt{x, y + h - bl*k}, pt{x, y + h - bl})
	p.line(pt{x, y + tl})
	p.cube(pt{x, y + tl*k}, pt{x + tl*k, y}, pt{x + tl, y})
}

// mirror reflects subpaths from index `from` on around x = cx.
func (p *path) mirror(from int, cx float64) {
	for _, sub := range p.subs[from:] {
		for i := range sub {
			for j := range sub[i].p {
				sub[i].p[j].x = 2*cx - sub[i].p[j].x
			}
		}
	}
}

func (p *path) circle(cx, cy, r float64, positive bool) {
	if r <= 0 {
		return
	}
	p.roundedRect(cx-r, cy-r, 2*r, 2*r, scene.AllCorners(r), 1, positive)
}

// stroke expands a polyline of the given width. Round joins and caps add a
// disc at every vertex, so the union has no gaps at corners.
func (p *path) stroke(pts []pt, width float64, round bool) {
	hw := width / 2
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		dx, dy := b.x-a.x, b.y-a.y
		l := math.Hypot(dx, dy)
		if l == 0 {
			continue
		}
		nx, ny := -dy/l*hw, dx/l*hw
		p.polygon([]pt{{a.x + nx, a.y + ny}, {b.x + nx, b.y + ny}, {b.x - nx, b.y - ny}, {a.x - nx, a.y - ny}}, true)
	}
	if round {
		for _, q := range pts {
			p.circle(q.x, q.y, hw, true)
		}
	}
}

// fill rasterizes the path over its bounding box only, so a hairline does
// not cost a full-canvas coverage buffer.
func (c *canvasState) fill(p *path, col color.NRGBA) {
	if len(p.subs) == 0 || col.A == 0 {
		return
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, sub := range p.subs {
		for _, s := range sub {
			n := 1
			if s.kind == segCube {
				n = 3
			}
			for _, q := range s.p[:n] {
				minX, minY = math.Min(minX, q.x), math.Min(minY, q.y)
				maxX, maxY = math.Max(maxX, q.x), math.Max(maxY, q.y)
			}
		}
	}
	box := image.Rect(int(math.Floor(minX)), int(math.Floor(minY)), int(math.Ceil(maxX))+1, int(math.Ceil(maxY))+1).Intersect(c.dst.Bounds())
	if box.Empty() {
		return
	}
	ox, oy := float64(box.Min.X), float64(box.Min.Y)
	f := func(q pt) (float32, float32) { return float32(q.x - ox), float32(q.y - oy) }
	c.z.Reset(box.Dx(), box.Dy())
	for _, sub := range p.subs {
		for i, s := range sub {
			switch {
			case i == 0:
				c.z.MoveTo(f(s.p[0]))
			case s.kind == segLine:
				c.z.LineTo(f(s.p[0]))
			default:
				x1, y1 := f(s.p[0])
				x2, y2 := f(s.p[1])
				x3, y3 := f(s.p[2])
				c.z.CubeTo(x1, y1, x2, y2, x3, y3)
			}
		}
		c.z.ClosePath()
	}
	c.z.Draw(c.dst, box, image.NewUniform(col), image.Point{})
}

func (c *canvasState) color(col base.Color) color.NRGBA {
	r := base.Resolve(col, c.dark)
	return color.NRGBA{R: r.R, G: r.G, B: r.B, A: r.A}
}

func (c *canvasState) styled(col base.Color, e scene.Emphasis) color.NRGBA {
	n := c.color(col)
	switch e {
	case scene.Lift:
		mix := func(x uint8) uint8 { return x + uint8(math.Round(float64(255-x)*lift)) }
		n.R, n.G, n.B = mix(n.R), mix(n.G), mix(n.B)
	case scene.Dim:
		n.A = uint8(math.Round(float64(n.A) * dim))
	}
	return n
}

func (c *canvasState) px(x, y float64) pt { return pt{x * c.scale, y * c.scale} }

func newCanvas(f *Fonts, w, h int, scale float64, dark bool) *canvasState {
	c := &canvasState{dst: image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1))), scale: scale, dark: dark, fonts: f}
	draw.Draw(c.dst, c.dst.Bounds(), image.NewUniform(c.color(base.Surface)), image.Point{}, draw.Src)
	return c
}

// Paint draws sc with the interaction state into a w×h pixel image, text
// in f (which the scene should have been measured with); scale is device
// pixels per logical unit. hovered < 0 means none.
func Paint(sc *scene.Scene, f *Fonts, hovered int, pointer Pointer, hidden []string, w, h int, scale float64, dark bool) *image.RGBA {
	c := newCanvas(f, w, h, scale, dark)
	for i, m := range sc.Marks {
		if e := sc.Emphasis(i, hovered, hidden); e != scene.Hidden {
			c.mark(m, e)
		}
	}
	if hovered >= 0 {
		if hit := sc.Marks[hovered].Hit; hit != nil && hit.Kind != scene.HitLegend {
			c.tooltip(hit, hidden, pt{pointer.X, pointer.Y}, sc.Width, sc.Height)
		}
	}
	return c.dst
}

// PaintError shows why the chart could not be laid out.
func PaintError(f *Fonts, msg string, w, h int, scale float64, dark bool) *image.RGBA {
	c := newCanvas(f, w, h, scale, dark)
	c.text(8, 8, msg, tipSize, base.Regular, scene.Start, scene.Top, 0, c.color(base.TextPrimary))
	return c.dst
}

func (c *canvasState) mark(m scene.Mark, e scene.Emphasis) {
	s := c.scale
	var sw float64
	var strokeCol color.NRGBA
	if m.Stroke != nil {
		sw, strokeCol = m.Stroke.Width*s, c.styled(m.Stroke.Color, e)
	}
	switch sh := m.Shape.(type) {
	case scene.RectShape:
		r := sh.Rect
		x, y, w, h := r.X*s, r.Y*s, r.W*s, r.H*s
		if m.Fill != base.NoColor {
			var p path
			p.roundedRect(x, y, w, h, sh.Radius, s, true)
			c.fill(&p, c.styled(m.Fill, e))
		}
		if m.Stroke != nil {
			// A frame centered on the edge, as SVG strokes are: the rect
			// grown by half the width minus the rect shrunk by half. Square
			// corners stay square (a miter join); round ones stay concentric.
			hw := sw / 2
			grow := func(d float64) scene.Corners {
				g := func(v float64) float64 {
					if v <= 0 {
						return 0
					}
					return math.Max(v+d/s, 0)
				}
				return scene.Corners{TL: g(sh.Radius.TL), TR: g(sh.Radius.TR), BR: g(sh.Radius.BR), BL: g(sh.Radius.BL)}
			}
			var p path
			p.roundedRect(x-hw, y-hw, w+sw, h+sw, grow(hw), s, true)
			if w > sw && h > sw {
				p.roundedRect(x+hw, y+hw, w-sw, h-sw, grow(-hw), s, false)
			}
			c.fill(&p, strokeCol)
		}
	case scene.LineShape:
		if m.Stroke == nil {
			return
		}
		var p path
		p.stroke([]pt{c.px(sh.X1, sh.Y1), c.px(sh.X2, sh.Y2)}, sw, false)
		c.fill(&p, strokeCol)
	case scene.PolylineShape:
		if m.Stroke == nil {
			return
		}
		pts := make([]pt, len(sh.Points))
		for i, q := range sh.Points {
			pts[i] = c.px(q.X, q.Y)
		}
		var p path
		p.stroke(pts, sw, true)
		c.fill(&p, strokeCol)
	case scene.AreaShape:
		if m.Fill == base.NoColor || len(sh.Points) < 2 {
			return
		}
		pts := make([]pt, 0, 2*len(sh.Points))
		for _, q := range sh.Points {
			pts = append(pts, c.px(q.X, q.Top))
		}
		for k := len(sh.Points) - 1; k >= 0; k-- {
			pts = append(pts, c.px(sh.Points[k].X, sh.Points[k].Bottom))
		}
		var p path
		p.polygon(pts, true)
		c.fill(&p, c.styled(m.Fill, e))
	case scene.PathShape:
		if m.Fill != base.NoColor {
			// The rasterizer clamps the accumulated winding, which is the
			// nonzero rule; fill closes every subpath.
			var p path
			for _, op := range sh.Ops {
				switch {
				case op.Verb == scene.MoveTo:
					p.begin(c.px(op.X, op.Y))
				case len(p.subs) == 0:
					// Nothing to extend before the first move.
				case op.Verb == scene.LineTo:
					p.line(c.px(op.X, op.Y))
				case op.Verb == scene.CubicTo:
					p.cube(c.px(op.X1, op.Y1), c.px(op.X2, op.Y2), c.px(op.X, op.Y))
				}
			}
			c.fill(&p, c.styled(m.Fill, e))
		}
		if m.Stroke != nil {
			var p path
			for _, sub := range scene.Flatten(sh.Ops) {
				pts := make([]pt, len(sub))
				for i, q := range sub {
					pts[i] = c.px(q.X, q.Y)
				}
				p.stroke(pts, sw, true)
			}
			c.fill(&p, strokeCol)
		}
	case scene.CircleShape:
		cx, cy, r := sh.CX*s, sh.CY*s, sh.R*s
		if m.Fill != base.NoColor {
			var p path
			p.circle(cx, cy, r, true)
			c.fill(&p, c.styled(m.Fill, e))
		}
		if m.Stroke != nil {
			// A ring: the outer edge minus the inner one.
			var p path
			p.circle(cx, cy, r+sw/2, true)
			p.circle(cx, cy, r-sw/2, false)
			c.fill(&p, strokeCol)
		}
	case scene.TextShape:
		col := base.TextPrimary
		if m.Fill != base.NoColor {
			col = m.Fill
		}
		c.text(sh.X, sh.Y, sh.Text, sh.Size, sh.Weight, sh.Anchor, sh.Base, sh.Angle, c.styled(col, e))
	}
}

// text draws one line with its anchor at (x, y) logical, rotated clockwise
// by angle degrees around that point.
func (c *canvasState) text(x, y float64, s string, size float64, w base.Weight, a scene.Anchor, b scene.Baseline, angle float64, col color.NRGBA) {
	run := c.fonts.shape(s, size, w)
	dx := 0.0
	switch a {
	case scene.Middle:
		dx = -run.width / 2
	case scene.End:
		dx = -run.width
	}
	cap := c.fonts.capHeight * size
	dy := 0.0
	switch b {
	case scene.Top:
		dy = cap
	case scene.Center:
		dy = cap / 2
	}
	sin, cos := math.Sincos(angle * math.Pi / 180)
	// Local (pen + offset, baseline-relative) → rotated → placed → device.
	place := func(lx, ly float64) pt {
		return pt{(x + lx*cos - ly*sin) * c.scale, (y + lx*sin + ly*cos) * c.scale}
	}
	var p path
	for _, g := range run.glyphs {
		for _, sub := range g.outline {
			for i, sg := range sub {
				q := func(k int) pt { return place(dx+g.pen+sg.p[k].x*size, dy+sg.p[k].y*size) }
				switch {
				case i == 0:
					p.begin(q(0))
				case sg.kind == segLine:
					p.line(q(0))
				default:
					p.cube(q(0), q(1), q(2))
				}
			}
		}
	}
	c.fill(&p, col)
}

// tipRow is one tooltip line: an optional series key, a strong part and a
// weak part.
type tipRow struct {
	key          base.Color
	strong, weak string
}

// tipRows: the value leads when there is one, the label follows, then any
// rows, minus those of hidden series. A row keyed by a series color (a
// crosshair's) gets a short stroke of it; a statistic row has none. Same
// content and order as the HTML runtime and the Rust backends.
func tipRows(hit *scene.Hit, hidden []string) []tipRow {
	var rows []tipRow
	if hit.Value != "" {
		rows = append(rows, tipRow{key: base.NoColor, strong: hit.Value})
	}
	rows = append(rows, tipRow{key: base.NoColor, weak: hit.Label})
	for _, r := range hit.Rows {
		if !slices.Contains(hidden, r.Name) {
			rows = append(rows, tipRow{key: r.Color, strong: r.Value, weak: r.Name})
		}
	}
	return rows
}

// tooltip draws the hovered mark's tooltip beside the pointer (logical
// coordinates), flipping left or up when it would leave the chart.
func (c *canvasState) tooltip(hit *scene.Hit, hidden []string, at pt, w, h float64) {
	rows := tipRows(hit, hidden)
	type offsets struct{ strongX, weakX float64 }
	lay := make([]offsets, len(rows))
	width := 0.0
	for k, r := range rows {
		x := 0.0
		if r.key != base.NoColor {
			x += tipKeyW + tipGap
		}
		lay[k].strongX = x
		if r.strong != "" {
			x += c.fonts.shape(r.strong, tipSize, base.Semibold).width
			if r.weak != "" {
				x += tipGap
			}
		}
		lay[k].weakX = x
		if r.weak != "" {
			x += c.fonts.shape(r.weak, tipSize, base.Regular).width
		}
		width = math.Max(width, x)
	}
	bw, bh := width+2*tipPad, float64(len(rows))*tipLine+2*tipPad
	x, y := at.x+tipOffset, at.y+tipOffset
	if x+bw > w {
		x = at.x - tipOffset - bw
	}
	if y+bh > h {
		y = at.y - tipOffset - bh
	}
	x, y = math.Max(0, x), math.Max(0, y)

	s := c.scale
	var box path
	box.roundedRect(x*s, y*s, bw*s, bh*s, scene.AllCorners(tipRadius), s, true)
	c.fill(&box, c.color(base.Surface))
	var border path
	border.roundedRect(x*s, y*s, bw*s, bh*s, scene.AllCorners(tipRadius), s, true)
	border.roundedRect((x+1)*s, (y+1)*s, (bw-2)*s, (bh-2)*s, scene.AllCorners(tipRadius-1), s, false)
	c.fill(&border, c.color(base.Border))
	for k, r := range rows {
		mid := y + tipPad + (float64(k)+0.5)*tipLine
		left := x + tipPad
		if r.key != base.NoColor {
			var key path
			key.roundedRect(left*s, (mid-tipKeyH/2)*s, tipKeyW*s, tipKeyH*s, scene.AllCorners(tipKeyH/2), s, true)
			c.fill(&key, c.color(r.key))
		}
		if r.strong != "" {
			c.text(left+lay[k].strongX, mid, r.strong, tipSize, base.Semibold, scene.Start, scene.Center, 0, c.color(base.TextPrimary))
		}
		if r.weak != "" {
			c.text(left+lay[k].weakX, mid, r.weak, tipSize, base.Regular, scene.Start, scene.Center, 0, c.color(base.TextSecondary))
		}
	}
}
