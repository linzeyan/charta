package fyne

import (
	"image/color"
	"math"
	"slices"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/raster"
	"github.com/linzeyan/charta/scene"
	"github.com/linzeyan/charta/zoom"
)

// zoomOf is what a chart zooms, as of its last layout.
type zoomOf int

const (
	zoomOff zoomOf = iota
	zoomCategories
	// zoomRange is a continuous axis: view within full, the whole chart's
	// plot; plot is the plot on screen, which gestures map through.
	zoomRange
)

// wheelNotch is one wheel notch of scroll distance: Fyne reports a notch
// as 10 on macOS and 25 elsewhere, so one event counts at most one notch,
// as in Gio.
const wheelNotch = 10.0

// zoomLayer lies over a chart that zooms and takes its wheel and drags.
// Fyne sends each to the topmost object that takes it, so a chart that does
// not zoom hides the layer and leaves them to whatever encloses it, a
// scroll container say. Clicks land on the layer too, and pass through.
type zoomLayer struct {
	widget.BaseWidget
	v *ChartView
}

var (
	_ fyne.Scrollable   = (*zoomLayer)(nil)
	_ fyne.Draggable    = (*zoomLayer)(nil)
	_ desktop.Mouseable = (*zoomLayer)(nil)
	_ fyne.Tappable     = (*zoomLayer)(nil)
)

func newZoomLayer(v *ChartView) *zoomLayer {
	l := &zoomLayer{v: v}
	l.ExtendBaseWidget(l)
	l.Hide()
	return l
}

func (l *zoomLayer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (l *zoomLayer) Scrolled(e *fyne.ScrollEvent)    { l.v.scrolled(e.Scrolled.DY) }
func (l *zoomLayer) Dragged(e *fyne.DragEvent)       { l.v.dragged(e) }
func (l *zoomLayer) DragEnd()                        { l.v.dragEnd() }
func (l *zoomLayer) MouseDown(e *desktop.MouseEvent) { l.v.mouseDown(e) }
func (l *zoomLayer) MouseUp(*desktop.MouseEvent)     {}
func (l *zoomLayer) Tapped(e *fyne.PointEvent)       { l.v.Tapped(e) }

// layOut lays the chart out as zoomed, and records what it zooms.
func (v *ChartView) layOut(w, h float64) (*scene.Scene, error) {
	m := Measure{}
	if z, ok := v.Chart.(charta.Zoomable); ok && z.CategoryCount() > zoom.MinSpan {
		n := z.CategoryCount()
		if v.zoomOf != zoomCategories || v.window.Len != n {
			v.window = zoom.Full(n)
		}
		v.zoomOf = zoomCategories
		if !v.window.IsFull() {
			return z.Window(v.window.Start, v.window.End).Scene(w, h, m)
		}
		return v.Chart.Scene(w, h, m)
	}
	vz, ok := v.Chart.(charta.Viewable)
	if !ok {
		v.zoomOf = zoomOff
		return v.Chart.Scene(w, h, m)
	}
	sc, full, err := vz.PlotScene(w, h, m)
	if err != nil {
		return nil, err
	}
	// A view kept from other data starts over.
	if v.zoomOf != zoomRange || !v.view.Fits(full) {
		v.view = zoom.FullView(full)
	}
	v.zoomOf, v.full, v.plot = zoomRange, full, full
	if v.view.IsFull() {
		return sc, nil
	}
	sc, v.plot, err = vz.View(v.view.Ranges()).PlotScene(w, h, m)
	return sc, err
}

// scrolled zooms by a wheel turn of dy (Fyne's DY; up is positive and
// zooms in) around the pointer.
func (v *ChartView) scrolled(dy float32) {
	if v.layout() != nil {
		return
	}
	notches := math.Max(-1, math.Min(float64(dy)/wheelNotch, 1))
	at := v.pointer
	switch v.zoomOf {
	case zoomCategories:
		col, ok := zoom.ColumnAt(v.sc, at.X)
		if !ok {
			return
		}
		// Fractions of a notch add up to whole categories.
		v.wheel += notches
		whole := math.Trunc(v.wheel)
		v.wheel -= whole
		if whole != 0 {
			v.rezoomWindow(v.window.Wheel(col, whole))
		}
	case zoomRange:
		v.rezoomView(v.view.Wheel(v.plot, at.X, at.Y, notches))
	}
}

// mouseDown notes whether shift is held, which makes a drag from here a
// brush, and resets the zoom on the second press of a double click. A
// double click on a legend entry toggles its series twice and leaves the
// zoom alone. Fyne's own DoubleTapped would hold every single tap back
// until the double-tap delay passed, legend toggles included.
func (v *ChartView) mouseDown(e *desktop.MouseEvent) {
	if e.Button != desktop.MouseButtonPrimary {
		return
	}
	v.shift = e.Modifier&fyne.KeyModifierShift != 0
	now := time.Now()
	double := now.Sub(v.lastDown) < fyne.CurrentApp().Driver().DoubleTapDelay()
	// The press after a double click starts a new pair.
	if double {
		v.lastDown = time.Time{}
	} else {
		v.lastDown = now
	}
	v.pointer, v.hovering = raster.Pointer{X: float64(e.Position.X), Y: float64(e.Position.Y)}, true
	if !double || v.layout() != nil {
		return
	}
	if i := v.hovered(); i >= 0 && v.sc.Marks[i].Hit.Kind == scene.HitLegend {
		return
	}
	switch v.zoomOf {
	case zoomCategories:
		v.rezoomWindow(zoom.Full(v.window.Len))
	case zoomRange:
		v.rezoomView(zoom.FullView(v.full))
	}
}

// dragged pans by a drag's movement, or follows it with the brush.
func (v *ChartView) dragged(e *fyne.DragEvent) {
	if v.layout() != nil {
		return
	}
	to := raster.Pointer{X: float64(e.Position.X), Y: float64(e.Position.Y)}
	dx, dy := float64(e.Dragged.DX), float64(e.Dragged.DY)
	if !v.dragging {
		// Fyne reports a drag once it passes its threshold; the first
		// event's delta reaches back to where the button went down.
		v.dragging = true
		v.dragFrom = raster.Pointer{X: to.X - dx, Y: to.Y - dy}
	}
	v.pointer, v.hovering = to, true
	if v.shift {
		v.raster.Refresh()
		return
	}
	switch v.zoomOf {
	case zoomCategories:
		// Pan by whole categories; the remainder carries over so a slow
		// drag still moves.
		if cw, ok := zoom.ColumnWidth(v.sc); ok && cw > 0 {
			v.pan += dx
			cols := math.Trunc(v.pan / cw)
			v.pan -= cols * cw
			v.rezoomWindow(v.window.Pan(-int(cols)))
		}
	case zoomRange:
		v.rezoomView(v.view.Pan(v.plot, dx, dy))
	}
}

// dragEnd zooms to what a brush covered.
func (v *ChartView) dragEnd() {
	from, to, brushing := v.dragFrom, v.pointer, v.dragging && v.shift
	v.dragging, v.shift, v.pan = false, false, 0
	if !brushing || v.layout() != nil {
		return
	}
	switch v.zoomOf {
	case zoomCategories:
		last := max(v.window.End-v.window.Start-1, 0)
		a, okA := zoom.ColumnAt(v.sc, from.X)
		b, okB := zoom.ColumnAt(v.sc, to.X)
		if okA && okB {
			v.rezoomWindow(v.window.Brush(min(int(a), last), min(int(b), last)))
		}
	case zoomRange:
		v.rezoomView(v.view.Brush(v.plot, from.X, from.Y, to.X, to.Y))
	}
	// The shading goes, zoomed or not.
	v.raster.Refresh()
}

func (v *ChartView) rezoomWindow(next zoom.Window) {
	if next != v.window {
		v.window = next
		v.relayout()
	}
}

func (v *ChartView) rezoomView(next zoom.View) {
	if !next.Equal(v.view) {
		v.view = next
		v.relayout()
	}
}

func (v *ChartView) relayout() {
	v.sc = nil
	v.raster.Refresh()
}

// withBrush is sc with what a shift-drag under way covers shaded: the
// columns it touched, or a band or box of the plot.
func (v *ChartView) withBrush(sc *scene.Scene) *scene.Scene {
	if !v.dragging || !v.shift {
		return sc
	}
	from, to := v.dragFrom, v.pointer
	var r scene.Rect
	switch v.zoomOf {
	case zoomCategories:
		a, okA := zoom.ColumnAt(sc, from.X)
		b, okB := zoom.ColumnAt(sc, to.X)
		if !okA || !okB {
			return sc
		}
		lo, hi := zoom.BrushEdges(a, b)
		x0, ok0 := zoom.ColumnX(sc, lo)
		x1, ok1 := zoom.ColumnX(sc, hi)
		if !ok0 || !ok1 {
			return sc
		}
		r = scene.Rect{X: x0, W: x1 - x0, H: sc.Height}
	case zoomRange:
		r = v.view.BrushRect(v.plot, from.X, from.Y, to.X, to.Y)
	default:
		return sc
	}
	shaded := *sc
	shaded.Marks = append(slices.Clip(sc.Marks), scene.Mark{Shape: scene.RectShape{Rect: r}, Fill: base.Link(0)})
	return &shaded
}
