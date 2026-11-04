package fyne

import (
	"image"
	"image/color"
	"math"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/internal/demo"
	"github.com/linzeyan/charta/scene"
	"github.com/linzeyan/charta/zoom"
)

// months is a year of visits and signups, enough categories to zoom into.
func months() charta.LineChart {
	cats := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	visits, signups := make([]float64, 12), make([]float64, 12)
	for i := range 12 {
		visits[i], signups[i] = float64(i+1)*10, float64(i+1)*4
	}
	return charta.LineChart{Categories: cats, Series: []charta.Series{{Name: "Visits", Values: visits}, {Name: "Signups", Values: signups}}}
}

// press is the primary button going down at (x, y) of the view, on the
// layer that takes it.
func (h *harness) press(x, y float64, mods fyne.KeyModifier) {
	h.view.layer.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(float32(x), float32(y))}, Button: desktop.MouseButtonPrimary, Modifier: mods})
}

// drag moves from one view point to another in ten steps, as Fyne reports
// a drag, and ends it; mid sees the view halfway.
func (h *harness) drag(x0, y0, x1, y1 float64, mid func(image.Image)) {
	last := fyne.NewPos(float32(x0), float32(y0))
	for k := 1; k <= 10; k++ {
		t := float64(k) / 10
		p := fyne.NewPos(float32(x0+(x1-x0)*t), float32(y0+(y1-y0)*t))
		h.view.layer.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: p}, Dragged: fyne.NewDelta(p.X-last.X, p.Y-last.Y)})
		last = p
		if k == 5 && mid != nil {
			mid(h.capture())
		}
	}
	h.view.layer.DragEnd()
}

// changed reports whether a few px around view point (x, y) differ.
func (h *harness) changed(a, b image.Image, x, y float64) bool {
	for dy := -2.0; dy <= 2; dy++ {
		for dx := -2.0; dx <= 2; dx++ {
			if h.pixel(a, x+dx, y+dy) != h.pixel(b, x+dx, y+dy) {
				return true
			}
		}
	}
	return false
}

// TestACategoryAxisZoomsPansBrushesAndResets mirrors the Gio and Slint
// tests: the wheel zooms around the pointer (fractions add up), a drag pans
// by whole categories, a double click shows everything, a shift-drag zooms
// to the brushed months, shading them while it lasts, and a double click on
// a legend entry is two toggles.
func TestACategoryAxisZoomsPansBrushesAndResets(t *testing.T) {
	h := open(t, months(), false)
	c := h.win.Canvas()
	sc := h.scene()
	midX, midY := float64(viewW/2), float64(viewH/2)
	col, _ := zoom.ColumnAt(sc, midX)
	test.MoveMouse(c, h.at(midX, midY))

	test.Scroll(c, h.at(midX, midY), 0, 5)
	if h.view.window != zoom.Full(12) {
		t.Fatalf("half a notch zoomed: %+v", h.view.window)
	}
	test.Scroll(c, h.at(midX, midY), 0, 5)
	if want := zoom.Full(12).Wheel(col, 1); h.view.window != want {
		t.Fatalf("two half notches: %+v, want %+v", h.view.window, want)
	}
	// Outside macOS Fyne sends 25 per notch: one event is one notch either
	// way.
	span := func() int { return h.view.window.End - h.view.window.Start }
	test.Scroll(c, h.at(midX, midY), 0, 25)
	if span() != 8 {
		t.Fatalf("a 25 event in from 10 months shows %d", span())
	}
	test.Scroll(c, h.at(midX, midY), 0, -25)
	if span() != 10 {
		t.Fatalf("a 25 event out from 8 months shows %d", span())
	}
	if err := h.view.layout(); err != nil {
		t.Fatal(err)
	}
	zoomed := h.view.window
	cols := 0
	for _, m := range h.view.sc.Marks {
		if m.Hit != nil && m.Hit.Kind == scene.HitCrosshair {
			cols++
		}
	}
	if cols != zoomed.End-zoomed.Start {
		t.Fatalf("the zoom was not laid out: %d columns in %+v", cols, zoomed)
	}

	// Dragging left by a column and a bit brings one later month in.
	cw, _ := zoom.ColumnWidth(h.view.sc)
	test.Drag(c, h.at(midX, midY), -float32(cw+4), 0)
	if want := zoomed.Pan(1); h.view.window != want {
		t.Fatalf("dragged a column left: %+v, want %+v", h.view.window, want)
	}
	if h.view.pan != 0 {
		t.Fatalf("the pan remainder outlived the drag: %v", h.view.pan)
	}

	h.press(midX, midY, 0)
	h.press(midX, midY, 0)
	if !h.view.window.IsFull() {
		t.Fatalf("a double click left %+v", h.view.window)
	}

	x := func(at float64) float64 { v, _ := zoom.ColumnX(sc, at); return v }
	before := h.capture()
	h.press(x(2.9), midY, fyne.KeyModifierShift)
	h.drag(x(2.9), midY, x(6.5), midY, func(img image.Image) {
		if !h.changed(before, img, x(3.5), 2) {
			t.Error("no shading in the brushed columns")
		}
		if h.changed(before, img, x(6.5), 2) {
			t.Error("shading ahead of the pointer")
		}
	})
	if want := zoom.Full(12).Brush(2, 6); h.view.window != want {
		t.Fatalf("brushed columns 2.9..6.5: %+v, want %+v", h.view.window, want)
	}
	if h.view.dragging || h.view.shift {
		t.Fatal("the brush outlived the drag")
	}

	kept := h.view.window
	if err := h.view.layout(); err != nil {
		t.Fatal(err)
	}
	legX, legY := center(h.view.sc, func(m scene.Mark) bool { return m.Hit.Kind == scene.HitLegend })
	for range 2 {
		h.press(legX, legY, 0)
		test.TapCanvas(c, h.at(legX, legY))
	}
	if h.view.window != kept || len(h.view.Hidden()) != 0 {
		t.Fatalf("a legend double click: window %+v, hidden %v", h.view.window, h.view.Hidden())
	}
}

// TestAScatterplotZoomsBothAxesByWheelDragAndBox mirrors the Gio, egui and
// Slint tests of the same name.
func TestAScatterplotZoomsBothAxesByWheelDragAndBox(t *testing.T) {
	var pts []charta.Point
	for i := range 60 {
		pts = append(pts, charta.Point{X: float64(i), Y: float64(i * 37 % 100)})
	}
	chart := charta.ScatterChart{Series: []charta.PointSeries{{Name: "Trials", Points: pts}}}
	h := open(t, chart, false)
	c := h.win.Canvas()
	_, plot, err := chart.PlotScene(viewW, viewH, Measure{})
	if err != nil {
		t.Fatal(err)
	}
	full := zoom.FullView(plot)
	across := func(tx, ty float64) (float64, float64) {
		r := plot.Rect
		return float64(float32(r.X + tx*r.W)), float64(float32(r.Y + ty*r.H))
	}

	x, y := across(1.0/3, 1.0/3)
	test.MoveMouse(c, h.at(x, y))
	test.Scroll(c, h.at(x, y), 0, 10)
	if want := full.Wheel(plot, x, y, 1); !h.view.view.Equal(want) {
		t.Fatalf("a notch: %+v, want %+v", h.view.view, want)
	}
	zoomed := h.view.view

	// Dragging left and down: later x and higher y, the spans kept.
	mx, my := across(0.5, 0.5)
	h.drag(mx, my, mx-120, my+60, nil)
	panned := h.view.view
	if !(panned.X.Lo > zoomed.X.Lo && panned.Y.Lo > zoomed.Y.Lo) {
		t.Fatalf("dragging left and down: %+v → %+v", zoomed, panned)
	}
	if math.Abs((panned.X.Hi-panned.X.Lo)-(zoomed.X.Hi-zoomed.X.Lo)) > 1e-9 {
		t.Fatalf("panning changed the span: %+v → %+v", zoomed, panned)
	}

	h.press(mx, my, 0)
	h.press(mx, my, 0)
	if !h.view.view.IsFull() {
		t.Fatalf("a double click left %+v", h.view.view)
	}

	// The brush is a box: halfway it shades from where the button went
	// down to the pointer, and nothing below or right of that.
	x0, y0 := across(0.25, 0.25)
	x1, y1 := across(0.6, 0.7)
	test.MoveMouse(c, h.at(x0, y0))
	before := h.capture()
	h.press(x0, y0, fyne.KeyModifierShift)
	h.drag(x0, y0, x1, y1, func(img image.Image) {
		if !h.changed(before, img, plot.Rect.X+0.3*plot.Rect.W, plot.Rect.Y+0.32*plot.Rect.H) {
			t.Error("no shading inside the box")
		}
		for _, p := range [][2]float64{{0.3, 0.9}, {0.9, 0.32}} {
			if h.changed(before, img, plot.Rect.X+p[0]*plot.Rect.W, plot.Rect.Y+p[1]*plot.Rect.H) {
				t.Errorf("shading at %v of the plot, outside the box", p)
			}
		}
	})
	if want := full.Brush(plot, x0, y0, x1, y1); !h.view.view.Equal(want) {
		t.Fatalf("brushed: %+v, want %+v", h.view.view, want)
	}
	h.save("zoomed-scatter")

	// New data with a wider extent starts over at all of it.
	wide := chart
	wide.Series = []charta.PointSeries{{Name: "Trials", Points: append(slices.Clone(pts), charta.Point{X: 500, Y: 500})}}
	h.view.Chart = wide
	h.view.Refresh()
	if err := h.view.layout(); err != nil {
		t.Fatal(err)
	}
	if !h.view.view.IsFull() || h.view.view.X.Full[1] < 500 {
		t.Fatalf("new data kept the old view: %+v", h.view.view)
	}
}

// TestOnlyChartsThatZoomTakeTheWheel: in a scroll container, the wheel
// over a bar chart scrolls the container, and over a line chart zooms the
// chart and leaves the container where it is.
func TestOnlyChartsThatZoomTakeTheWheel(t *testing.T) {
	test.NewTempApp(t)
	bar, line := NewChartView(demo.All()[0].Chart), NewChartView(months())
	filler := canvas.NewRectangle(color.Transparent)
	filler.SetMinSize(fyne.NewSize(viewW, 2000))
	scroll := container.NewVScroll(container.NewVBox(bar, line, filler))
	w := test.NewWindow(scroll)
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(viewW, 400))
	for _, v := range []*ChartView{bar, line} {
		v.Resize(fyne.NewSize(viewW, 160))
		v.layout()
	}
	at := func(v *ChartView) fyne.Position {
		return fyne.CurrentApp().Driver().AbsolutePositionForObject(v).Add(fyne.NewPos(viewW/2, 80))
	}
	if bar.layer.Visible() || !line.layer.Visible() {
		t.Fatalf("layers shown: bar %v, line %v", bar.layer.Visible(), line.layer.Visible())
	}

	test.MoveMouse(w.Canvas(), at(line))
	test.Scroll(w.Canvas(), at(line), 0, 10)
	if line.window.IsFull() || scroll.Offset.Y != 0 {
		t.Fatalf("the wheel over the line chart: window %+v, scrolled to %v", line.window, scroll.Offset.Y)
	}
	test.Scroll(w.Canvas(), at(bar), 0, -10)
	if scroll.Offset.Y == 0 {
		t.Fatal("the wheel over the bar chart did not scroll the container")
	}
}
