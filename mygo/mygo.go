// Package mygo draws charta charts in MyGo's native UI
// (github.com/egoist/mygo/ui) and drives hover, tooltips, legend toggles and
// zoom from scene.HitTest and scene.Emphasis, matching the HTML runtime and
// the Gio, Fyne and egui backends.
//
//	view := &mygo.ChartView{Chart: chart} // keep it across frames
//	view.Build(c).Grow(1)                 // in a ui.View function
//
// MyGo needs no cgo, so neither does this package: it cross-compiles like
// the charta core.
package mygo

import (
	"math"
	"slices"

	"github.com/egoist/mygo/ui"

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
	// tipLine is the tooltip's line height (DIPs).
	tipLine = tipSize * 1.4
	// tipOffset places the tooltip beside the pointer, clear of the cursor.
	tipOffset = 12.0
	// tipKeyW and tipKeyH size a crosshair row's series key, a short stroke
	// in the series color.
	tipKeyW = 12.0
	tipKeyH = 2.0
	tipGap  = 6.0

	// dragSlop: a press that moves less than this (DIPs) before release is
	// a click, not a drag (egui's max_click_dist).
	dragSlop = 6.0
	// wheelNotch is how many DIPs of a touchpad's precise scrolling make
	// one wheel notch, as Gio's backend counts them. A mouse wheel's
	// notches come as imprecise events, one notch each, whatever DIPs the
	// platform turns them into.
	wheelNotch = 10.0
)

// ChartView is a chart as a MyGo element. Keep it across frames: it
// remembers the pointer, which series the viewer hid by clicking legend
// entries, and the zoom. A chart with a category axis (charta.Zoomable,
// more than zoom.MinSpan categories) or a continuous one (charta.Viewable)
// zooms: the wheel zooms around the pointer, dragging pans, shift-dragging
// brushes a range (a box, on a chart that zooms y too) to zoom to, and a
// double click shows everything again.
type ChartView struct {
	Chart charta.Chart

	dark     bool
	pointer  point
	hovering bool
	hidden   []string

	// The last scene drawn, in the element's DIPs: input arrives between
	// frames and is resolved against what the viewer saw. plot is where its
	// plot sits, full the whole chart's (charta.Viewable only).
	sc         *scene.Scene
	plot, full scene.Plot

	// Zoom state, as in the Gio backend. window.Len is zero until a
	// zoomable chart is laid out; view likewise for a viewable chart.
	window zoom.Window
	view   zoom.View
	// wheel and pan are the notches and DIPs not yet a whole step; they
	// carry over so touchpad fractions and slow drags still move.
	wheel, pan float64
	pressed    bool
	press      point
	// clicks is the press's count of quick successive presses (2 for a
	// double click), as the platform reports it.
	clicks   int
	shift    bool
	dragging bool
	dragLast point
	// brushing: a shift-drag is under way from column brushFrom, or on a
	// continuous axis from brushAt.
	brushing  bool
	brushFrom float64
	brushAt   point
}

type point struct{ X, Y float64 }

// Hidden lists the series the viewer has hidden through the legend.
func (v *ChartView) Hidden() []string { return slices.Clone(v.hidden) }

// Build adds the chart to c as an element and returns it, for the caller
// to size (Grow, Size, Fill). The chart follows the theme's light or dark
// appearance.
func (v *ChartView) Build(c *ui.Context) ui.Element {
	v.dark = c.Theme().Dark
	el := ui.Box(c)
	if !el.Hovered() && !v.pressed {
		v.hovering = false
	}
	if v.hovering && v.sc != nil {
		if i, ok := v.sc.HitTest(v.pointer.X, v.pointer.Y, v.hidden); ok && v.sc.Marks[i].Hit.Kind == scene.HitLegend {
			el = el.Cursor(ui.CursorPointer)
		}
	}
	return el.HandleInput(v.input).Draw(v.draw)
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

// zoomInput is one event's zoom gesture, applied to the last scene drawn.
type zoomInput struct {
	notches   float64
	scrollAt  point
	dragged   float64 // DIPs moved while dragging
	draggedY  float64 // and down, for a chart that zooms y
	brushFrom *point  // a shift-drag started here
	stopAt    point   // a drag ended here
	stopped   bool
	double    bool
}

// input takes the pointer as it comes. MyGo hands it over before the next
// frame is built, so a gesture applied here is drawn by the next Draw and
// never paints a stale chart.
func (v *ChartView) input(ev ui.InputEvent) bool {
	_, zoomOK := v.zoomable()
	_, viewOK := v.Chart.(charta.Viewable)
	zooms := zoomOK || viewOK
	at := point{float64(ev.X), float64(ev.Y)}
	var in zoomInput
	switch ev.Kind {
	case ui.InputPointerMove:
		v.pointer, v.hovering = at, true
		if !v.pressed || !zooms {
			break
		}
		if !v.dragging {
			if math.Hypot(at.X-v.press.X, at.Y-v.press.Y) < dragSlop {
				break
			}
			v.dragging, v.dragLast = true, v.press
			if v.shift {
				from := v.press
				in.brushFrom = &from
			}
		}
		// A brush ignores this: gestures pans only when no brush is under way.
		in.dragged += at.X - v.dragLast.X
		in.draggedY += at.Y - v.dragLast.Y
		v.dragLast = at
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		v.pointer, v.hovering = at, true
		v.pressed, v.press, v.dragging = true, at, false
		v.shift, v.clicks = ev.Mods&ui.Shift != 0, ev.Clicks
	case ui.InputPointerUp:
		if ev.Button != 0 {
			return false
		}
		switch {
		case ev.Canceled || !v.pressed:
			v.brushing = false
		case v.dragging:
			in.stopAt, in.stopped = at, true
		case v.sc != nil:
			legend := false
			if i, ok := v.sc.HitTest(at.X, at.Y, v.hidden); ok && v.sc.Marks[i].Hit.Kind == scene.HitLegend {
				legend = true
				v.toggle(v.sc.Marks[i].Series)
			}
			// Two quick legend toggles are two toggles, not a request to
			// show the whole axis. The platform counts on (3 for a triple
			// click); as in the other backends, the click after a double
			// click starts a new pair.
			in.double = v.clicks >= 2 && v.clicks%2 == 0 && !legend
		}
		v.pressed, v.dragging = false, false
	case ui.InputScroll:
		if !zooms {
			// Leave the wheel to an enclosing scroll container.
			return false
		}
		// Up (negative) zooms in, as egui's positive wheel delta does.
		d := -float64(ev.DY) / wheelNotch
		if !ev.Precise {
			d = -math.Copysign(1, float64(ev.DY))
			if ev.DY == 0 {
				d = 0
			}
		}
		in.notches = math.Max(-1, math.Min(d, 1))
		in.scrollAt = at
	default:
		return false
	}
	v.apply(in)
	return true
}

// apply moves the zoom by one event's gesture, against the last scene
// drawn; the next Draw lays the chart out anew.
func (v *ChartView) apply(in zoomInput) {
	if v.sc == nil {
		return
	}
	if _, ok := v.zoomable(); ok && v.window.Len > 0 {
		v.window = v.gestures(v.sc, in)
	}
	if _, ok := v.Chart.(charta.Viewable); ok && v.full.Rect.W > 0 {
		v.view = v.viewGestures(v.plot, v.full, in)
	}
}

// gestures moves the zoom window by one event's wheel, drag, shift-drag
// and double click. Pointer x maps to categories through sc's crosshair
// columns; the arithmetic is package zoom's, shared with every backend.
func (v *ChartView) gestures(sc *scene.Scene, in zoomInput) zoom.Window {
	w := v.window
	col := func(p point) (float64, bool) { return zoom.ColumnAt(sc, p.X) }
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

// viewGestures moves the continuous view by one event's wheel, drag,
// shift-drag and double click. Pointer positions map to data through plot,
// the plot on screen; full is the whole chart's, for a reset.
func (v *ChartView) viewGestures(plot, full scene.Plot, in zoomInput) zoom.View {
	next := v.view
	if in.notches != 0 {
		next = next.Wheel(plot, in.scrollAt.X, in.scrollAt.Y, in.notches)
	}
	if in.brushFrom != nil {
		v.brushing, v.brushAt = true, *in.brushFrom
	}
	if v.brushing {
		if in.stopped {
			next = next.Brush(plot, v.brushAt.X, v.brushAt.Y, in.stopAt.X, in.stopAt.Y)
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

// layout lays the chart out at w × h DIPs in the current zoom. A viewable
// chart is laid out whole first: its plot is the extent a view moves in,
// and a view kept from other data starts over.
func (v *ChartView) layout(w, h float64) (*scene.Scene, error) {
	m := Measure{}
	z, zoomOK := v.zoomable()
	if zoomOK && v.window.Len != z.CategoryCount() {
		v.window = zoom.Full(z.CategoryCount())
	}
	if zoomOK && !v.window.IsFull() {
		return z.Window(v.window.Start, v.window.End).Scene(w, h, m)
	}
	vz, viewOK := v.Chart.(charta.Viewable)
	if !viewOK {
		return v.Chart.Scene(w, h, m)
	}
	sc, full, err := vz.PlotScene(w, h, m)
	if err != nil {
		return nil, err
	}
	v.full, v.plot = full, full
	if !v.view.Fits(full) {
		v.view = zoom.FullView(full)
	}
	if v.view.IsFull() {
		return sc, nil
	}
	sc, v.plot, err = vz.View(v.view.Ranges()).PlotScene(w, h, m)
	return sc, err
}

// draw lays the chart out at the element's size and paints it, with the
// hovered mark lifted and its tooltip beside the pointer.
func (v *ChartView) draw(p *ui.Painter, r ui.Rect) {
	g := painter{p: p, ox: r.X, oy: r.Y, dark: v.dark}
	sc, err := v.layout(float64(r.W), float64(r.H))
	if err != nil {
		v.sc = nil
		g.label(8, 8, err.Error(), 12, base.Regular, scene.Start, scene.Top, 0, ui.RGB(200, 0, 0))
		return
	}
	v.sc = sc
	hovered := -1
	if v.hovering {
		if i, ok := sc.HitTest(v.pointer.X, v.pointer.Y, v.hidden); ok {
			hovered = i
		}
	}
	p.Fill(r, g.color(base.Surface), 0)
	for i, mk := range sc.Marks {
		if e := sc.Emphasis(i, hovered, v.hidden); e != scene.Hidden {
			g.mark(mk, e)
		}
	}
	if _, ok := v.zoomable(); ok {
		v.brushArea(g, sc, float64(r.H))
	}
	if _, ok := v.Chart.(charta.Viewable); ok && v.brushing {
		b := v.view.BrushRect(v.plot, v.brushAt.X, v.brushAt.Y, v.pointer.X, v.pointer.Y)
		g.fillRect(b.X, b.Y, b.W, b.H, g.color(base.Link(0)))
	}
	if hovered >= 0 {
		if hit := sc.Marks[hovered].Hit; hit.Kind != scene.HitLegend {
			v.tooltip(g, float64(r.W), float64(r.H), hit)
		}
	}
}

// brushArea shades the columns a shift-drag has covered so far, in the
// translucent link color, as the other backends do.
func (v *ChartView) brushArea(g painter, sc *scene.Scene, h float64) {
	if !v.brushing {
		return
	}
	to, ok := zoom.ColumnAt(sc, v.pointer.X)
	if !ok {
		return
	}
	lo, hi := zoom.BrushEdges(v.brushFrom, to)
	x0, ok0 := zoom.ColumnX(sc, lo)
	x1, ok1 := zoom.ColumnX(sc, hi)
	if ok0 && ok1 {
		g.fillRect(x0, 0, x1-x0, h, g.color(base.Link(0)))
	}
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

// tipRow is one tooltip line: an optional series key, a strong part and a
// weak part.
type tipRow struct {
	key          base.Color
	strong, weak string
}

// tipRows: the value leads when there is one, the label follows, then any
// rows, minus those of hidden series. Same content and order as the HTML
// runtime and the other backends.
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

func (v *ChartView) tooltip(g painter, w, h float64, hit *scene.Hit) {
	m := Measure{}
	rows := tipRows(hit, v.hidden)
	type layoutRow struct{ strongX, weakX float64 }
	lay := make([]layoutRow, len(rows))
	inner := 0.0
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
		inner = math.Max(inner, x)
	}
	bw, bh := inner+2*tipPad, float64(len(rows))*tipLine+2*tipPad
	x, y := v.pointer.X+tipOffset, v.pointer.Y+tipOffset
	if x+bw > w {
		x = v.pointer.X - tipOffset - bw
	}
	if y+bh > h {
		y = v.pointer.Y - tipOffset - bh
	}
	x, y = max(0, x), max(0, y)
	box := g.rect(x, y, bw, bh)
	g.p.Fill(box, g.color(base.Surface), 6)
	g.p.Stroke(box, g.color(base.Border), 6, 1)
	capH := base.CapHeight * tipSize
	for k, r := range rows {
		x0, y0 := x+tipPad, y+tipPad+float64(k)*tipLine
		if r.key != base.NoColor {
			// Centered on the cap height, like the text beside it.
			g.p.Fill(g.rect(x0, y0+(capH-tipKeyH)/2, tipKeyW, tipKeyH), g.color(r.key), 1)
		}
		if r.strong != "" {
			g.label(x0+lay[k].strongX, y0, r.strong, tipSize, base.Semibold, scene.Start, scene.Top, 0, g.color(base.TextPrimary))
		}
		if r.weak != "" {
			g.label(x0+lay[k].weakX, y0, r.weak, tipSize, base.Regular, scene.Start, scene.Top, 0, g.color(base.TextSecondary))
		}
	}
}

// painter paints scene marks, given in the element's DIPs, at the
// element's origin (ox, oy) in the window.
type painter struct {
	p      *ui.Painter
	ox, oy float32
	dark   bool
}

func (g painter) color(c base.Color) ui.Color {
	r := base.Resolve(c, g.dark)
	return ui.Color{R: r.R, G: r.G, B: r.B, A: r.A}
}

func (g painter) styled(c base.Color, e scene.Emphasis) ui.Color {
	col := g.color(c)
	switch e {
	case scene.Lift:
		mix := func(x uint8) uint8 { return x + uint8(math.Round(float64(255-x)*lift)) }
		col.R, col.G, col.B = mix(col.R), mix(col.G), mix(col.B)
	case scene.Dim:
		col.A = uint8(math.Round(float64(col.A) * dim))
	}
	return col
}

func (g painter) pt(x, y float64) (float32, float32) {
	return g.ox + float32(x), g.oy + float32(y)
}

func (g painter) rect(x, y, w, h float64) ui.Rect {
	X, Y := g.pt(x, y)
	return ui.Rect{X: X, Y: Y, W: float32(w), H: float32(h)}
}

func (g painter) fillRect(x, y, w, h float64, c ui.Color) {
	g.p.Fill(g.rect(x, y, w, h), c, 0)
}

func (g painter) mark(m scene.Mark, e scene.Emphasis) {
	var stroke ui.Color
	var width float32
	if m.Stroke != nil {
		stroke, width = g.styled(m.Stroke.Color, e), float32(m.Stroke.Width)
	}
	switch sh := m.Shape.(type) {
	case scene.RectShape:
		r := sh.Rect
		if m.Fill != base.NoColor {
			// The GPU's own rounded rectangles: on macOS their corners are
			// continuous (squircles), which at a chart's few-DIP radii looks
			// no different from the circular corners elsewhere.
			c := sh.Radius
			g.p.FillRadii(g.rect(r.X, r.Y, r.W, r.H), [4]float32{float32(c.TL), float32(c.TR), float32(c.BR), float32(c.BL)}, g.styled(m.Fill, e))
		}
		if m.Stroke != nil {
			// Centered on the edge, as SVG strokes are, with SVG's default
			// miter joins.
			path := g.roundedRect(r, sh.Radius)
			g.p.StrokePath(path.Join(ui.MiterJoin).Cap(ui.ButtCap), width, stroke)
		}
	case scene.LineShape:
		if m.Stroke == nil {
			return
		}
		x1, y1 := g.pt(sh.X1, sh.Y1)
		x2, y2 := g.pt(sh.X2, sh.Y2)
		switch {
		case y1 == y2:
			// Axes and grid lines: a rectangle the GPU draws, butt-ended as
			// SVG's default line cap.
			g.p.Fill(ui.Rect{X: min(x1, x2), Y: y1 - width/2, W: abs(x2 - x1), H: width}, stroke, 0)
		case x1 == x2:
			g.p.Fill(ui.Rect{X: x1 - width/2, Y: min(y1, y2), W: width, H: abs(y2 - y1)}, stroke, 0)
		default:
			path := (&ui.Path{}).MoveTo(x1, y1).LineTo(x2, y2).Cap(ui.ButtCap)
			g.p.StrokePath(path, width, stroke)
		}
	case scene.PolylineShape:
		if len(sh.Points) < 2 || m.Stroke == nil {
			return
		}
		// Round joins and caps, as the HTML runtime's series lines. MyGo
		// draws a translucent (dimmed) line through a mask, so its joints
		// do not show darker.
		pts := make([][2]float32, len(sh.Points))
		for i, q := range sh.Points {
			x, y := g.pt(q.X, q.Y)
			pts[i] = [2]float32{x, y}
		}
		g.p.StrokePolyline(pts, width, stroke)
	case scene.AreaShape:
		// One polygon, along the top edge and back along the bottom: one
		// shape means no seams between slices.
		if m.Fill == base.NoColor || len(sh.Points) < 2 {
			return
		}
		var path ui.Path
		path.MoveTo(g.pt(sh.Points[0].X, sh.Points[0].Top))
		for _, q := range sh.Points[1:] {
			path.LineTo(g.pt(q.X, q.Top))
		}
		for k := len(sh.Points) - 1; k >= 0; k-- {
			path.LineTo(g.pt(sh.Points[k].X, sh.Points[k].Bottom))
		}
		path.Close()
		g.p.FillPath(&path, g.styled(m.Fill, e))
	case scene.PathShape:
		if m.Fill != base.NoColor {
			// MyGo fills with the nonzero rule by default, the scene's rule
			// for paths; a fill closes every open subpath.
			g.p.FillPath(g.path(sh.Ops, true), g.styled(m.Fill, e))
		}
		if m.Stroke != nil {
			// A stroke traces only the segments the path has, with round
			// joins and caps as the HTML runtime's.
			g.p.StrokePath(g.path(sh.Ops, false), width, stroke)
		}
	case scene.CircleShape:
		// A path, not a fully rounded rectangle: those are squircles on
		// macOS.
		cx, cy := g.pt(sh.CX, sh.CY)
		circle := (&ui.Path{}).Circle(cx, cy, float32(sh.R))
		if m.Fill != base.NoColor {
			g.p.FillPath(circle, g.styled(m.Fill, e))
		}
		if m.Stroke != nil {
			g.p.StrokePath(circle, width, stroke)
		}
	case scene.TextShape:
		c := base.TextPrimary
		if m.Fill != base.NoColor {
			c = m.Fill
		}
		g.label(sh.X, sh.Y, sh.Text, sh.Size, sh.Weight, sh.Anchor, sh.Base, sh.Angle, g.styled(c, e))
	}
}

// path converts scene path ops; close closes open subpaths, as a fill does.
func (g painter) path(ops []scene.PathOp, close bool) *ui.Path {
	var p ui.Path
	open := false
	for _, op := range ops {
		switch op.Verb {
		case scene.MoveTo:
			if open && close {
				p.Close()
			}
			p.MoveTo(g.pt(op.X, op.Y))
			open = true
		case scene.LineTo:
			p.LineTo(g.pt(op.X, op.Y))
		case scene.CubicTo:
			x1, y1 := g.pt(op.X1, op.Y1)
			x2, y2 := g.pt(op.X2, op.Y2)
			x, y := g.pt(op.X, op.Y)
			p.CubeTo(x1, y1, x2, y2, x, y)
		case scene.ClosePath:
			p.Close()
			open = false
		}
	}
	if open && close {
		p.Close()
	}
	return &p
}

// kappa places cubic control points so each corner is a quarter circle.
const kappa = 0.5522848

// roundedRect traces r with per-corner radii, for strokes.
func (g painter) roundedRect(r scene.Rect, c scene.Corners) *ui.Path {
	x0, y0 := g.pt(r.X, r.Y)
	x1, y1 := g.pt(r.X+r.W, r.Y+r.H)
	tl, tr, br, bl := float32(c.TL), float32(c.TR), float32(c.BR), float32(c.BL)
	var p ui.Path
	p.MoveTo(x0+tl, y0)
	p.LineTo(x1-tr, y0)
	p.CubeTo(x1-tr+tr*kappa, y0, x1, y0+tr-tr*kappa, x1, y0+tr)
	p.LineTo(x1, y1-br)
	p.CubeTo(x1, y1-br+br*kappa, x1-br+br*kappa, y1, x1-br, y1)
	p.LineTo(x0+bl, y1)
	p.CubeTo(x0+bl-bl*kappa, y1, x0, y1-bl+bl*kappa, x0, y1-bl)
	p.LineTo(x0, y0+tl)
	p.CubeTo(x0, y0+tl-tl*kappa, x0+tl-tl*kappa, y0, x0+tl, y0)
	p.Close()
	return &p
}

// label draws one line of text with its anchor point at (x, y), rotated
// clockwise by angle degrees around that point. The scene places text by
// its cap height, so the baseline is computed from base.CapHeight rather
// than the system font's ascent.
func (g painter) label(x, y float64, txt string, size float64, w base.Weight, a scene.Anchor, b scene.Baseline, angle float64, col ui.Color) {
	glyphs := ui.Shape(txt, font(size, w))
	var ax float32
	switch a {
	case scene.Middle:
		ax = advance(glyphs) / 2
	case scene.End:
		ax = advance(glyphs)
	}
	capH := float32(base.CapHeight * size)
	var dy float32
	switch b {
	case scene.Top:
		dy = capH
	case scene.Center:
		dy = capH / 2
	}
	X, Y := g.pt(x, y)
	draw := func() { g.p.Glyphs(glyphs, X-ax, Y+dy, col) }
	if angle == 0 {
		draw()
		return
	}
	g.p.Transform(ui.Rotation(float32(angle)), X, Y, draw)
}

func font(size float64, w base.Weight) ui.Font {
	weight := 400
	if w == base.Semibold {
		weight = 600
	}
	return ui.Font{Size: float32(size), Weight: weight}
}

// advance is the width of shaped text: the right edge of its last glyph,
// whichever order a right-to-left run puts them in.
func advance(glyphs []ui.Glyph) float32 {
	var w float32
	for _, gl := range glyphs {
		w = max(w, gl.X+gl.Advance)
	}
	return w
}

func abs(x float32) float32 { return float32(math.Abs(float64(x))) }

// Measure sizes text with the system text engine MyGo draws with, in DIPs,
// so labels fit the glyphs on screen. ChartView lays charts out with it;
// use it to get the same scene the view paints.
type Measure struct{}

func (Measure) Width(s string, size float64, w base.Weight) float64 {
	return float64(advance(ui.Shape(s, font(size, w))))
}
