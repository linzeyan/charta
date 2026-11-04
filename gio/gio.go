// Package gio draws charta charts in Gio (gioui.org) and drives hover,
// tooltips and legend toggles from scene.HitTest and scene.Emphasis,
// matching the HTML runtime and the Rust egui backend.
//
//	view := &gio.ChartView{Chart: chart, Shaper: text.NewShaper(text.WithCollection(gofont.Collection()))}
//	view.Layout(gtx) // keep view across frames: it remembers the pointer and hidden series
//
// Gio itself needs cgo on macOS and Linux; the charta core does not.
package gio

import (
	"image"
	"image/color"
	"math"
	"slices"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"golang.org/x/image/math/fixed"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
	"github.com/linzeyan/charta/zoom"
)

const (
	// lift lightens the hovered mark toward white, like the HTML runtime's
	// brightness(1.12).
	lift = 0.12
	// dim is the opacity of other series while one series is highlighted.
	dim     = 0.35
	tipSize = 12.0
	tipPad  = 8.0
	// tipLine is the tooltip's line height (dp).
	tipLine = tipSize * 1.4
	// tipOffset places the tooltip beside the pointer, clear of the cursor.
	tipOffset = 12.0
	// tipKeyW and tipKeyH size a crosshair row's series key, a short stroke
	// in the series color.
	tipKeyW = 12.0
	tipKeyH = 2.0
	tipGap  = 6.0
)

// ChartView is a chart as a Gio widget filling its constraints. Keep it
// across frames: it remembers the pointer between events, which series the
// viewer hid by clicking legend entries, and the zoom. A chart with a
// category axis (charta.Zoomable, more than zoom.MinSpan categories) or a
// continuous one (charta.Viewable) zooms: the wheel zooms around the
// pointer, dragging pans, shift-dragging brushes a range (a box, on a chart
// that zooms y too) to zoom to, and a double click shows everything again.
type ChartView struct {
	Chart  charta.Chart
	Shaper *text.Shaper
	// Dark selects the dark palette; Gio has no app-wide theme to ask.
	Dark bool

	pointer  f32.Point
	hovering bool
	pressed  bool
	clicks   []f32.Point
	hidden   []string

	// Zoom state. window.Len is zero until a zoomable chart is laid out,
	// and resets when the category count changes; view likewise for a
	// viewable chart, when its full extent changes.
	window zoom.Window
	view   zoom.View
	// wheel and pan are the notches and dp not yet a whole step; they
	// carry over so trackpad fractions and slow drags still move.
	wheel, pan float64
	// press is where and when the primary button went down; shift is
	// whether shift was held then, which makes the drag a brush.
	press    f32.Point
	pressAt  time.Duration
	shift    bool
	dragging bool
	dragLast f32.Point
	// brushing: a shift-drag is under way from column brushFrom, or on a
	// continuous axis from brushAt (px).
	brushing  bool
	brushFrom float64
	brushAt   f32.Point
	// lastClick is when the previous click's press was, for double clicks.
	lastClick    time.Duration
	hasLastClick bool
}

const (
	// dragSlop: a press that moves less than this (dp) before release is a
	// click, not a drag (egui's max_click_dist).
	dragSlop = 6.0
	// doubleClick is Gio's own double-click window (gesture.Click).
	doubleClick = 200 * time.Millisecond
	// wheelNotch is one wheel notch of scroll distance (dp): Gio reports a
	// notch as 10 on macOS, X11 and Wayland. Windows sends 120 per notch,
	// so one event counts at most one notch.
	wheelNotch = 10.0
)

// Hidden lists the series the viewer has hidden through the legend.
func (v *ChartView) Hidden() []string { return slices.Clone(v.hidden) }

// zoomInput is one frame's zoom gestures, collected from pointer events and
// applied once the frame's scene maps pointer x to categories.
type zoomInput struct {
	notches   float64
	scrollAt  f32.Point
	dragged   float64    // dp moved while dragging
	draggedY  float64    // and down, for a chart that zooms y
	brushFrom *f32.Point // a shift-drag started here
	stopAt    f32.Point  // a drag ended here
	stopped   bool
	double    bool
}

// zoomable is the chart as a Zoomable, when it has enough categories to
// zoom into.
func (v *ChartView) zoomable() (charta.Zoomable, bool) {
	z, ok := v.Chart.(charta.Zoomable)
	if !ok || z.CategoryCount() <= zoom.MinSpan {
		return nil, false
	}
	return z, true
}

// Layout handles pointer events, lays the chart out in dp and paints it.
func (v *ChartView) Layout(gtx layout.Context) layout.Dimensions {
	s := gtx.Metric.PxPerDp
	z, zoomOK := v.zoomable()
	vz, viewOK := v.Chart.(charta.Viewable)
	filter := pointer.Filter{
		Target: v,
		Kinds:  pointer.Move | pointer.Drag | pointer.Enter | pointer.Leave | pointer.Press | pointer.Release | pointer.Cancel,
	}
	if zoomOK || viewOK {
		// Only a zoomable chart takes the wheel; others leave it to an
		// enclosing scrollable.
		filter.Kinds |= pointer.Scroll
		filter.ScrollY = pointer.ScrollRange{Min: math.MinInt32, Max: math.MaxInt32}
	}
	var in zoomInput
	for {
		ev, ok := gtx.Event(filter)
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		if pe.Kind == pointer.Scroll {
			// Up (negative) zooms in, as egui's positive wheel delta does.
			d := -float64(pe.Scroll.Y/s) / wheelNotch
			in.notches += math.Max(-1, math.Min(d, 1))
			in.scrollAt = pe.Position
		}
		v.pointer = pe.Position
		v.hovering = pe.Kind != pointer.Leave && pe.Kind != pointer.Cancel
		switch pe.Kind {
		case pointer.Press:
			v.pressed = pe.Buttons.Contain(pointer.ButtonPrimary)
			v.press, v.pressAt, v.dragging = pe.Position, pe.Time, false
			v.shift = pe.Modifiers.Contain(key.ModShift)
		case pointer.Drag:
			if !v.pressed || !(zoomOK || viewOK) {
				break
			}
			if !v.dragging {
				if d := pe.Position.Sub(v.press); math.Hypot(float64(d.X/s), float64(d.Y/s)) < dragSlop {
					break
				}
				v.dragging, v.dragLast = true, v.press
				if v.shift {
					from := v.press
					in.brushFrom = &from
				}
			}
			// A brush ignores this: gestures pans only when no brush is under way.
			in.dragged += float64((pe.Position.X - v.dragLast.X) / s)
			in.draggedY += float64((pe.Position.Y - v.dragLast.Y) / s)
			v.dragLast = pe.Position
		case pointer.Release:
			if v.pressed && v.dragging {
				in.stopAt, in.stopped = pe.Position, true
			} else if v.pressed {
				v.clicks = append(v.clicks, pe.Position)
				if v.hasLastClick && v.pressAt-v.lastClick < doubleClick {
					in.double, v.hasLastClick = true, false
				} else {
					v.lastClick, v.hasLastClick = v.pressAt, true
				}
			}
			v.pressed, v.dragging = false, false
		case pointer.Cancel:
			v.pressed, v.dragging, v.brushing = false, false, false
		}
	}

	size := gtx.Constraints.Max
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, v)

	dp := func(p f32.Point) (float64, float64) { return float64(p.X / s), float64(p.Y / s) }
	m := Measure{Metric: gtx.Metric, Shaper: v.Shaper}
	wDp, hDp := float64(float32(size.X)/s), float64(float32(size.Y)/s)
	// A viewable chart is laid out whole first: its plot is the extent a
	// view moves in, and a view kept from other data starts over.
	var (
		fullSc  *scene.Scene
		full    scene.Plot
		fullErr error
	)
	if viewOK {
		fullSc, full, fullErr = vz.PlotScene(wDp, hDp, m)
		if fullErr == nil && !v.view.Fits(full) {
			v.view = zoom.FullView(full)
		}
	}
	layOut := func() (*scene.Scene, scene.Plot, error) {
		switch {
		case zoomOK && !v.window.IsFull():
			sc, err := z.Window(v.window.Start, v.window.End).Scene(wDp, hDp, m)
			return sc, scene.Plot{}, err
		case viewOK && (fullErr != nil || v.view.IsFull()):
			return fullSc, full, fullErr
		case viewOK:
			return vz.View(v.view.Ranges()).PlotScene(wDp, hDp, m)
		}
		sc, err := v.Chart.Scene(wDp, hDp, m)
		return sc, scene.Plot{}, err
	}
	if zoomOK && v.window.Len != z.CategoryCount() {
		v.window = zoom.Full(z.CategoryCount())
	}
	sc, plot, err := layOut()
	if err == nil {
		for _, c := range v.clicks {
			x, y := dp(c)
			if i, ok := sc.HitTest(x, y, v.hidden); ok && sc.Marks[i].Hit.Kind == scene.HitLegend {
				v.toggle(sc.Marks[i].Series)
			}
		}
		v.clicks = v.clicks[:0]
		if in.double {
			// Two quick legend toggles are two toggles, not a request to
			// show the whole axis.
			x, y := dp(v.press)
			if i, ok := sc.HitTest(x, y, v.hidden); ok && sc.Marks[i].Hit.Kind == scene.HitLegend {
				in.double = false
			}
		}
		if zoomOK {
			if next := v.gestures(sc, in, s); next != v.window {
				// Lay out the new window now rather than next frame, so the
				// gesture never paints a stale chart.
				v.window = next
				sc, plot, err = layOut()
			}
		}
		if viewOK {
			if next := v.viewGestures(plot, full, in, s); !next.Equal(v.view) {
				v.view = next
				sc, plot, err = layOut()
			}
		}
	}
	if err != nil {
		v.label(gtx, f32.Pt(8*s, 8*s), err.Error(), 12, base.Regular, scene.Start, scene.Top, 0, color.NRGBA{R: 200, A: 255})
		return layout.Dimensions{Size: size}
	}
	hovered := -1
	if v.hovering {
		x, y := dp(v.pointer)
		if i, ok := sc.HitTest(x, y, v.hidden); ok {
			hovered = i
		}
	}
	var hit *scene.Hit
	if hovered >= 0 {
		hit = sc.Marks[hovered].Hit
		if hit.Kind == scene.HitLegend {
			pointer.CursorPointer.Add(gtx.Ops)
		}
	}

	paint.FillShape(gtx.Ops, v.color(base.Surface), clip.Rect{Max: size}.Op())
	for i, mk := range sc.Marks {
		if e := sc.Emphasis(i, hovered, v.hidden); e != scene.Hidden {
			v.paint(gtx, s, mk, e)
		}
	}
	if zoomOK {
		v.brushArea(gtx, sc, size)
	}
	if viewOK {
		v.brushBox(gtx, plot)
	}
	if hit != nil && hit.Kind != scene.HitLegend {
		v.tooltip(gtx, size, hit)
	}
	return layout.Dimensions{Size: size}
}

// gestures moves the zoom window by one frame's wheel, drag, shift-drag
// and double click. Pointer x maps to categories through sc's crosshair
// columns; the arithmetic is package zoom's, shared with the Rust egui
// backend.
func (v *ChartView) gestures(sc *scene.Scene, in zoomInput, s float32) zoom.Window {
	w := v.window
	col := func(p f32.Point) (float64, bool) { return zoom.ColumnAt(sc, float64(p.X/s)) }
	last := max(w.End-w.Start-1, 0)
	next := w
	if in.notches != 0 {
		if at, ok := col(in.scrollAt); ok {
			v.wheel += in.notches
			whole := math.Trunc(v.wheel)
			if whole != 0 {
				next = next.Wheel(at, whole)
			}
			v.wheel -= whole
		}
	}
	if in.brushFrom != nil {
		if at, ok := col(*in.brushFrom); ok {
			v.brushing, v.brushFrom = true, at
		}
	}
	if v.brushing {
		if in.stopped {
			if to, ok := col(in.stopAt); ok {
				next = next.Brush(min(int(v.brushFrom), last), min(int(to), last))
			}
			v.brushing = false
		}
	} else if in.dragged != 0 {
		// Pan by whole categories; the remainder carries over so a slow
		// drag still moves.
		v.pan += in.dragged
		if cw, ok := zoom.ColumnWidth(sc); ok && cw > 0 {
			if cols := math.Trunc(v.pan / cw); cols != 0 {
				next = next.Pan(-int(cols))
				v.pan -= float64(cols * cw)
			}
		}
	}
	if in.stopped {
		v.pan = 0
	}
	if in.double {
		next = zoom.Full(w.Len)
	}
	return next
}

// viewGestures moves the continuous view by one frame's wheel, drag,
// shift-drag and double click. Pointer positions map to data through plot,
// the plot on screen; full is the whole chart's, for a reset. Unlike a
// category window, a view moves by any amount, so nothing carries over.
func (v *ChartView) viewGestures(plot, full scene.Plot, in zoomInput, s float32) zoom.View {
	dp := func(p f32.Point) (float64, float64) { return float64(p.X / s), float64(p.Y / s) }
	next := v.view
	if in.notches != 0 {
		x, y := dp(in.scrollAt)
		next = next.Wheel(plot, x, y, in.notches)
	}
	if in.brushFrom != nil {
		v.brushing, v.brushAt = true, *in.brushFrom
	}
	if v.brushing {
		if in.stopped {
			x0, y0 := dp(v.brushAt)
			x1, y1 := dp(in.stopAt)
			next = next.Brush(plot, x0, y0, x1, y1)
			v.brushing = false
		}
	} else if in.dragged != 0 || in.draggedY != 0 {
		next = next.Pan(plot, in.dragged, in.draggedY)
	}
	if in.double {
		next = zoom.FullView(full)
	}
	return next
}

// brushBox shades what a shift-drag on a continuous axis has covered so
// far: a band, or a box on a chart that zooms y.
func (v *ChartView) brushBox(gtx layout.Context, plot scene.Plot) {
	if !v.brushing {
		return
	}
	s := float64(gtx.Metric.PxPerDp)
	r := v.view.BrushRect(plot, float64(v.brushAt.X)/s, float64(v.brushAt.Y)/s, float64(v.pointer.X)/s, float64(v.pointer.Y)/s)
	px := func(d float64) int { return int(math.Round(d * s)) }
	area := image.Rect(px(r.X), px(r.Y), px(r.X+r.W), px(r.Y+r.H))
	paint.FillShape(gtx.Ops, v.color(base.Link(0)), clip.Rect(area).Op())
}

// brushArea shades the columns a shift-drag has covered so far, in the
// translucent link color, as egui does.
func (v *ChartView) brushArea(gtx layout.Context, sc *scene.Scene, size image.Point) {
	if !v.brushing {
		return
	}
	s := gtx.Metric.PxPerDp
	to, ok := zoom.ColumnAt(sc, float64(v.pointer.X/s))
	if !ok {
		return
	}
	lo, hi := zoom.BrushEdges(v.brushFrom, to)
	x0, ok0 := zoom.ColumnX(sc, lo)
	x1, ok1 := zoom.ColumnX(sc, hi)
	if !ok0 || !ok1 {
		return
	}
	l, r := x0*float64(s), x1*float64(s)
	area := image.Rect(int(math.Round(l)), 0, int(math.Round(r)), size.Y)
	paint.FillShape(gtx.Ops, v.color(base.Link(0)), clip.Rect(area).Op())
}

// toggle hides a visible series or shows a hidden one.
func (v *ChartView) toggle(series string) {
	if series == "" {
		return
	}
	if k := slices.Index(v.hidden, series); k >= 0 {
		v.hidden = slices.Delete(v.hidden, k, k+1)
	} else {
		v.hidden = append(v.hidden, series)
	}
}

func (v *ChartView) color(c base.Color) color.NRGBA {
	r := base.Resolve(c, v.Dark)
	return color.NRGBA{R: r.R, G: r.G, B: r.B, A: r.A}
}

func (v *ChartView) styled(c base.Color, e scene.Emphasis) color.NRGBA {
	col := v.color(c)
	switch e {
	case scene.Lift:
		mix := func(x uint8) uint8 { return x + uint8(math.Round(float64(255-x)*lift)) }
		col.R, col.G, col.B = mix(col.R), mix(col.G), mix(col.B)
	case scene.Dim:
		col.A = uint8(math.Round(float64(col.A) * dim))
	}
	return col
}

func (v *ChartView) paint(gtx layout.Context, s float32, m scene.Mark, e scene.Emphasis) {
	px := func(x, y float64) f32.Point { return f32.Pt(float32(x)*s, float32(y)*s) }
	stroke := func(path clip.PathSpec) {
		if m.Stroke != nil {
			paint.FillShape(gtx.Ops, v.styled(m.Stroke.Color, e), clip.Stroke{Path: path, Width: float32(m.Stroke.Width) * s}.Op())
		}
	}
	switch sh := m.Shape.(type) {
	case scene.RectShape:
		r := sh.Rect
		rect := func() clip.PathSpec {
			var p clip.Path
			p.Begin(gtx.Ops)
			roundedRect(&p, px(r.X, r.Y), px(r.X+r.W, r.Y+r.H), sh.Radius, s)
			return p.End()
		}
		if m.Fill != base.NoColor {
			paint.FillShape(gtx.Ops, v.styled(m.Fill, e), clip.Outline{Path: rect()}.Op())
		}
		// clip.Stroke centers the outline on the edge, as SVG strokes are.
		stroke(rect())
	case scene.LineShape:
		var p clip.Path
		p.Begin(gtx.Ops)
		p.MoveTo(px(sh.X1, sh.Y1))
		p.LineTo(px(sh.X2, sh.Y2))
		stroke(p.End())
	case scene.PolylineShape:
		if len(sh.Points) < 2 || m.Stroke == nil {
			return
		}
		pts := make([]f32.Point, len(sh.Points))
		for i, q := range sh.Points {
			pts[i] = px(q.X, q.Y)
		}
		var p clip.Path
		p.Begin(gtx.Ops)
		roundStroke(&p, pts, float32(m.Stroke.Width)*s)
		paint.FillShape(gtx.Ops, v.styled(m.Stroke.Color, e), clip.Outline{Path: p.End()}.Op())
	case scene.AreaShape:
		// Gio fills any simple outline, so the band is one polygon: along
		// the top edge, back along the bottom. One shape means no seams
		// between slices.
		if m.Fill == base.NoColor || len(sh.Points) < 2 {
			return
		}
		var p clip.Path
		p.Begin(gtx.Ops)
		p.MoveTo(px(sh.Points[0].X, sh.Points[0].Top))
		for _, q := range sh.Points[1:] {
			p.LineTo(px(q.X, q.Top))
		}
		for k := len(sh.Points) - 1; k >= 0; k-- {
			p.LineTo(px(sh.Points[k].X, sh.Points[k].Bottom))
		}
		p.Close()
		paint.FillShape(gtx.Ops, v.styled(m.Fill, e), clip.Outline{Path: p.End()}.Op())
	case scene.PathShape:
		if m.Fill != base.NoColor {
			// Outline fills with the nonzero rule, the scene's rule for
			// paths; a fill closes every open subpath.
			var p clip.Path
			p.Begin(gtx.Ops)
			open := false
			for _, op := range sh.Ops {
				switch op.Verb {
				case scene.MoveTo:
					if open {
						p.Close()
					}
					p.MoveTo(px(op.X, op.Y))
					open = true
				case scene.LineTo:
					p.LineTo(px(op.X, op.Y))
				case scene.CubicTo:
					p.CubeTo(px(op.X1, op.Y1), px(op.X2, op.Y2), px(op.X, op.Y))
				case scene.ClosePath:
					p.Close()
					open = false
				}
			}
			if open {
				p.Close()
			}
			paint.FillShape(gtx.Ops, v.styled(m.Fill, e), clip.Outline{Path: p.End()}.Op())
		}
		if m.Stroke != nil {
			// A stroke traces only the segments the path has.
			var p clip.Path
			p.Begin(gtx.Ops)
			for _, sub := range scene.Flatten(sh.Ops) {
				pts := make([]f32.Point, len(sub))
				for i, q := range sub {
					pts[i] = px(q.X, q.Y)
				}
				roundStroke(&p, pts, float32(m.Stroke.Width)*s)
			}
			paint.FillShape(gtx.Ops, v.styled(m.Stroke.Color, e), clip.Outline{Path: p.End()}.Op())
		}
	case scene.CircleShape:
		circle := func() clip.PathSpec {
			var p clip.Path
			p.Begin(gtx.Ops)
			roundedRect(&p, px(sh.CX-sh.R, sh.CY-sh.R), px(sh.CX+sh.R, sh.CY+sh.R), scene.AllCorners(sh.R), s)
			return p.End()
		}
		if m.Fill != base.NoColor {
			paint.FillShape(gtx.Ops, v.styled(m.Fill, e), clip.Outline{Path: circle()}.Op())
		}
		stroke(circle())
	case scene.TextShape:
		c := base.TextPrimary
		if m.Fill != base.NoColor {
			c = m.Fill
		}
		v.label(gtx, px(sh.X, sh.Y), sh.Text, sh.Size, sh.Weight, sh.Anchor, sh.Base, sh.Angle, v.styled(c, e))
	}
}

// kappa places cubic control points so each corner is a quarter circle.
const kappa = 0.5522848

// roundedRect traces a rect with per-corner radii (dp, scaled by s); with
// every radius at half the side it traces a circle.
func roundedRect(p *clip.Path, min, max f32.Point, c scene.Corners, s float32) {
	tl, tr, br, bl := float32(c.TL)*s, float32(c.TR)*s, float32(c.BR)*s, float32(c.BL)*s
	p.MoveTo(f32.Pt(min.X+tl, min.Y))
	p.LineTo(f32.Pt(max.X-tr, min.Y))
	p.CubeTo(f32.Pt(max.X-tr+tr*kappa, min.Y), f32.Pt(max.X, min.Y+tr-tr*kappa), f32.Pt(max.X, min.Y+tr))
	p.LineTo(f32.Pt(max.X, max.Y-br))
	p.CubeTo(f32.Pt(max.X, max.Y-br+br*kappa), f32.Pt(max.X-br+br*kappa, max.Y), f32.Pt(max.X-br, max.Y))
	p.LineTo(f32.Pt(min.X+bl, max.Y))
	p.CubeTo(f32.Pt(min.X+bl-bl*kappa, max.Y), f32.Pt(min.X, max.Y-bl+bl*kappa), f32.Pt(min.X, max.Y-bl))
	p.LineTo(f32.Pt(min.X, min.Y+tl))
	p.CubeTo(f32.Pt(min.X, min.Y+tl-tl*kappa), f32.Pt(min.X+tl-tl*kappa, min.Y), f32.Pt(min.X+tl, min.Y))
	p.Close()
}

// roundStroke traces a polyline stroked width px wide with round joins and
// caps, as the other backends draw lines: a rectangle per segment and a
// disc per vertex, every piece wound the same way so the nonzero fill of
// clip.Outline unions them. Gio's clip.Stroke has no join style, and discs
// painted over it would double the alpha of a dimmed or translucent line
// where they overlap.
func roundStroke(p *clip.Path, pts []f32.Point, width float32) {
	hw := width / 2
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		d := b.Sub(a)
		l := float32(math.Hypot(float64(d.X), float64(d.Y)))
		if l == 0 {
			continue
		}
		n := f32.Pt(-d.Y/l*hw, d.X/l*hw)
		quad := [4]f32.Point{a.Add(n), b.Add(n), b.Sub(n), a.Sub(n)}
		// roundedRect winds positively (shoelace, y down); match it.
		area := float32(0)
		for k, q := range quad {
			r := quad[(k+1)%4]
			area += q.X*r.Y - r.X*q.Y
		}
		if area < 0 {
			quad[1], quad[3] = quad[3], quad[1]
		}
		p.MoveTo(quad[0])
		for _, q := range quad[1:] {
			p.LineTo(q)
		}
		p.Close()
	}
	for _, q := range pts {
		roundedRect(p, q.Sub(f32.Pt(hw, hw)), q.Add(f32.Pt(hw, hw)), scene.AllCorners(float64(hw)), 1)
	}
}

// label draws one line of text with its anchor point at `at` (px), rotated
// clockwise by angle degrees around that point.
func (v *ChartView) label(gtx layout.Context, at f32.Point, txt string, size float64, w base.Weight, a scene.Anchor, b scene.Baseline, angle float64, col color.NRGBA) {
	lgtx := gtx
	lgtx.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
	macro := op.Record(gtx.Ops)
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	material := macro.Stop()
	macro = op.Record(gtx.Ops)
	dims := widget.Label{MaxLines: 1}.Layout(lgtx, v.Shaper, fontFor(w), unit.Sp(size), txt, material)
	call := macro.Stop()

	var ax float32
	switch a {
	case scene.Middle:
		ax = float32(dims.Size.X) / 2
	case scene.End:
		ax = float32(dims.Size.X)
	}
	ascent := float32(dims.Size.Y - dims.Baseline)
	cap := float32(base.CapHeight * float64(gtx.Sp(unit.Sp(size))))
	ay := ascent
	switch b {
	case scene.Top:
		ay = ascent - cap
	case scene.Center:
		ay = ascent - cap/2
	}
	// Move the anchor onto `at`, then rotate around it.
	tr := f32.AffineId().Offset(at.Sub(f32.Pt(ax, ay))).Rotate(at, float32(angle*math.Pi/180))
	defer op.Affine(tr).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
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
		rows = append(rows, tipRow{strong: hit.Value})
	}
	rows = append(rows, tipRow{weak: hit.Label})
	for _, r := range hit.Rows {
		if !slices.Contains(hidden, r.Name) {
			rows = append(rows, tipRow{key: r.Color, strong: r.Value, weak: r.Name})
		}
	}
	return rows
}

func (v *ChartView) tooltip(gtx layout.Context, size image.Point, hit *scene.Hit) {
	s := gtx.Metric.PxPerDp
	m := Measure{Metric: gtx.Metric, Shaper: v.Shaper}
	rows := tipRows(hit, v.hidden)
	// Each part's x offset within its row (dp), and the widest row.
	type layoutRow struct{ strongX, weakX float64 }
	lay := make([]layoutRow, len(rows))
	wDp := 0.0
	for k, r := range rows {
		x := 0.0
		if r.key != base.NoColor {
			x += tipKeyW + tipGap
		}
		lay[k].strongX = x
		if r.strong != "" {
			x += m.Width(r.strong, tipSize, base.Semibold)
			if r.weak != "" {
				x += tipGap
			}
		}
		lay[k].weakX = x
		if r.weak != "" {
			x += m.Width(r.weak, tipSize, base.Regular)
		}
		wDp = math.Max(wDp, x)
	}
	w := float32(wDp+2*tipPad) * s
	h := float32(float64(len(rows))*tipLine+2*tipPad) * s
	x, y := v.pointer.X+tipOffset*s, v.pointer.Y+tipOffset*s
	if x+w > float32(size.X) {
		x = v.pointer.X - tipOffset*s - w
	}
	if y+h > float32(size.Y) {
		y = v.pointer.Y - tipOffset*s - h
	}
	x, y = max(0, x), max(0, y)
	box := image.Rect(int(x), int(y), int(x+w), int(y+h))
	rr := clip.UniformRRect(box, int(6*s))
	paint.FillShape(gtx.Ops, v.color(base.Surface), rr.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, v.color(base.Border), clip.Stroke{Path: rr.Path(gtx.Ops), Width: s}.Op())
	cap := float32(base.CapHeight * tipSize)
	for k, r := range rows {
		x0, y0 := x+tipPad*s, y+(tipPad+float32(k)*tipLine)*s
		if r.key != base.NoColor {
			// Centered on the cap height, like the text beside it.
			ky := y0 + (cap-tipKeyH)/2*s
			key := image.Rect(int(x0), int(ky), int(x0+tipKeyW*s), int(ky+tipKeyH*s))
			paint.FillShape(gtx.Ops, v.color(r.key), clip.UniformRRect(key, int(s)).Op(gtx.Ops))
		}
		if r.strong != "" {
			v.label(gtx, f32.Pt(x0+float32(lay[k].strongX)*s, y0), r.strong, tipSize, base.Semibold, scene.Start, scene.Top, 0, v.color(base.TextPrimary))
		}
		if r.weak != "" {
			v.label(gtx, f32.Pt(x0+float32(lay[k].weakX)*s, y0), r.weak, tipSize, base.Regular, scene.Start, scene.Top, 0, v.color(base.TextSecondary))
		}
	}
}

func fontFor(w base.Weight) font.Font {
	if w == base.Semibold {
		return font.Font{Weight: font.SemiBold}
	}
	return font.Font{}
}

// Measure sizes text with the shaper Gio draws with, in dp, so labels fit
// the glyphs on screen. ChartView lays charts out with it; use it to get
// the same scene the view paints.
type Measure struct {
	Metric unit.Metric
	Shaper *text.Shaper
}

func (m Measure) Width(s string, size float64, w base.Weight) float64 {
	px := m.Metric.Sp(unit.Sp(size))
	// MaxWidth must be set: with MaxLines 1 and the zero MaxWidth the shaper
	// truncates every string to a lone ellipsis glyph.
	m.Shaper.LayoutString(text.Parameters{Font: fontFor(w), PxPerEm: fixed.I(px), MaxLines: 1, MaxWidth: 1 << 20}, s)
	var adv fixed.Int26_6
	for {
		g, ok := m.Shaper.NextGlyph()
		if !ok {
			break
		}
		adv += g.Advance
	}
	return float64(adv) / 64 / float64(m.Metric.PxPerDp)
}
