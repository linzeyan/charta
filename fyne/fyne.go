// Package fyne draws charta charts in Fyne (fyne.io) and drives hover,
// tooltips and legend toggles from scene.HitTest and scene.Emphasis,
// matching the HTML runtime and the Gio backend.
//
// Fyne's canvas has no general path drawing, so the chart is rasterized
// with golang.org/x/image/vector into a canvas.Raster and redrawn on hover;
// the tooltip is drawn into the same image.
//
//	view := fyne.NewChartView(chart)
//	window.SetContent(view)
//
// A chart with a category axis (charta.Zoomable, more than zoom.MinSpan
// categories) or a continuous one (charta.Viewable) zooms: the wheel zooms
// around the pointer, dragging pans, shift-dragging brushes a range (a box,
// on a chart that zooms y too) to zoom to, and a double click shows
// everything again.
//
// Fyne itself needs cgo on desktop platforms; the charta core does not.
package fyne

import (
	"image"
	"slices"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/internal/raster"
	"github.com/linzeyan/charta/scene"
	"github.com/linzeyan/charta/zoom"
)

// ChartView is a chart as a Fyne widget. It remembers the pointer, which
// series the viewer hid by clicking legend entries, and the zoom.
type ChartView struct {
	widget.BaseWidget
	Chart charta.Chart
	// Dark selects the dark palette.
	Dark bool

	raster   *canvas.Raster
	sc       *scene.Scene
	err      error
	pointer  raster.Pointer
	hovering bool
	hidden   []string

	// Zoom state; see zoom.go. window resets when the category count
	// changes, view when the full extent does.
	layer      *zoomLayer
	zoomOf     zoomOf
	window     zoom.Window
	view       zoom.View
	full, plot scene.Plot
	// wheel and pan are the notches and distance not yet a whole category;
	// they carry over so trackpad fractions and slow drags still move.
	wheel, pan float64
	// shift was held when the button went down, so a drag is a brush;
	// dragFrom is where that drag started.
	shift, dragging bool
	dragFrom        raster.Pointer
	lastDown        time.Time
}

var (
	_ desktop.Hoverable  = (*ChartView)(nil)
	_ desktop.Cursorable = (*ChartView)(nil)
	_ fyne.Tappable      = (*ChartView)(nil)
)

func NewChartView(c charta.Chart) *ChartView {
	v := &ChartView{Chart: c}
	v.raster = canvas.NewRaster(v.draw)
	v.layer = newZoomLayer(v)
	v.ExtendBaseWidget(v)
	return v
}

func (v *ChartView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(v.raster, v.layer))
}

func (v *ChartView) MinSize() fyne.Size { return fyne.NewSize(160, 120) }

// Refresh re-lays the chart out, e.g. after changing Chart or Dark.
func (v *ChartView) Refresh() {
	v.sc = nil
	v.raster.Refresh()
}

// Hidden lists the series the viewer has hidden through the legend.
func (v *ChartView) Hidden() []string { return slices.Clone(v.hidden) }

func (v *ChartView) MouseIn(e *desktop.MouseEvent) { v.MouseMoved(e) }

// MouseMoved redraws only when the highlight changes or a tooltip follows
// the pointer.
func (v *ChartView) MouseMoved(e *desktop.MouseEvent) {
	before := v.hovered()
	v.pointer, v.hovering = raster.Pointer{X: float64(e.Position.X), Y: float64(e.Position.Y)}, true
	after := v.hovered()
	if before != after || (after >= 0 && v.sc.Marks[after].Hit.Kind != scene.HitLegend) {
		v.raster.Refresh()
	}
}

func (v *ChartView) MouseOut() {
	was := v.hovered() >= 0
	v.hovering = false
	if was {
		v.raster.Refresh()
	}
}

// Tapped toggles the series of a tapped legend entry.
func (v *ChartView) Tapped(e *fyne.PointEvent) {
	v.pointer, v.hovering = raster.Pointer{X: float64(e.Position.X), Y: float64(e.Position.Y)}, true
	i := v.hovered()
	if i < 0 || v.sc.Marks[i].Hit.Kind != scene.HitLegend || v.sc.Marks[i].Series == "" {
		return
	}
	s := v.sc.Marks[i].Series
	if k := slices.Index(v.hidden, s); k >= 0 {
		v.hidden = slices.Delete(v.hidden, k, k+1)
	} else {
		v.hidden = append(v.hidden, s)
	}
	v.raster.Refresh()
}

// Cursor is a pointing hand over legend entries, which are clickable.
func (v *ChartView) Cursor() desktop.Cursor {
	if i := v.hovered(); i >= 0 && v.sc.Marks[i].Hit.Kind == scene.HitLegend {
		return desktop.PointerCursor
	}
	return desktop.DefaultCursor
}

func (v *ChartView) hovered() int {
	if !v.hovering || v.layout() != nil {
		return -1
	}
	if i, ok := v.sc.HitTest(v.pointer.X, v.pointer.Y, v.hidden); ok {
		return i
	}
	return -1
}

// layout builds the scene for the widget's current size and zoom, once
// per change.
func (v *ChartView) layout() error {
	size := v.Size()
	w, h := float64(size.Width), float64(size.Height)
	if v.sc != nil && v.sc.Width == w && v.sc.Height == h {
		return nil
	}
	sc, err := v.layOut(w, h)
	v.sc, v.err = sc, err
	if err != nil {
		v.sc = nil
	}
	if zooms := v.zoomOf != zoomOff; zooms != v.layer.Visible() {
		if zooms {
			v.layer.Show()
		} else {
			v.layer.Hide()
		}
	}
	return err
}

// draw is the raster generator: w×h device pixels for the widget's size.
func (v *ChartView) draw(w, h int) image.Image {
	size := v.Size()
	scale := 1.0
	if size.Width > 0 {
		scale = float64(w) / float64(size.Width)
	}
	if err := v.layout(); err != nil {
		return raster.PaintError(fonts(), err.Error(), w, h, scale, v.Dark)
	}
	return raster.Paint(v.withBrush(v.sc), fonts(), v.hovered(), v.pointer, v.hidden, w, h, scale, v.Dark)
}
