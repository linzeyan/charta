package gio

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/demo"
	"github.com/linzeyan/charta/scene"
	"github.com/linzeyan/charta/zoom"
)

// These tests drive real pointer events through Gio's router into a headless
// GPU window and check the painted pixels, in both palettes. They also write
// build/snapshots/gio-*.png, every gallery demo included, so the result can
// be eyeballed without opening a window.

// pxPerDp renders at 2x so the snapshots match a Retina window.
const (
	pxPerDp  = 2
	wDp, hDp = 720, 440
)

// harness is one chart in a headless window, plus the scene the view lays
// out (same size, same measure) to aim the pointer with.
type harness struct {
	t      *testing.T
	win    *headless.Window
	view   *ChartView
	router input.Router
	ops    op.Ops
	scene  *scene.Scene
	dark   bool
}

func open(t *testing.T, chart charta.Chart, dark bool) *harness {
	t.Helper()
	win, err := headless.NewWindow(wDp*pxPerDp, hDp*pxPerDp)
	if err != nil {
		t.Fatalf("headless GPU window: %v", err)
	}
	t.Cleanup(win.Release)
	shaper := text.NewShaper(text.WithCollection(gofont.Collection()))
	metric := unit.Metric{PxPerDp: pxPerDp, PxPerSp: pxPerDp}
	sc, err := chart.Scene(wDp, hDp, Measure{Metric: metric, Shaper: shaper})
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, win: win, view: &ChartView{Chart: chart, Shaper: shaper, Dark: dark}, scene: sc, dark: dark}
	// The router only routes events to handlers a frame has registered.
	h.frame()
	return h
}

func (h *harness) frame() *image.RGBA {
	h.t.Helper()
	size := image.Pt(wDp*pxPerDp, hDp*pxPerDp)
	h.ops.Reset()
	h.view.Layout(layout.Context{
		Ops:         &h.ops,
		Metric:      unit.Metric{PxPerDp: pxPerDp, PxPerSp: pxPerDp},
		Constraints: layout.Exact(size),
		Source:      h.router.Source(),
	})
	h.router.Frame(&h.ops)
	if err := h.win.Frame(&h.ops); err != nil {
		h.t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := h.win.Screenshot(img); err != nil {
		h.t.Fatal(err)
	}
	return img
}

// hover moves the pointer to p (px) and paints the result.
func (h *harness) hover(p f32.Point) *image.RGBA {
	h.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: p})
	return h.frame()
}

func (h *harness) click(p f32.Point) {
	h.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: p},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: p},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: p},
	)
	h.frame()
}

// target is the center (px) of the hit area of the first mark matching kind
// and named by label or series.
func (h *harness) target(kind scene.HitKind, name string) f32.Point {
	h.t.Helper()
	for _, m := range h.scene.Marks {
		if m.Hit == nil || m.Hit.Kind != kind || (m.Hit.Label != name && m.Series != name) {
			continue
		}
		var r scene.Rect
		switch {
		case m.Hit.Area != nil:
			r = *m.Hit.Area
		default:
			r = m.Shape.(scene.RectShape).Rect
		}
		return f32.Pt(float32(r.X+r.W/2)*pxPerDp, float32(r.Y+r.H/2)*pxPerDp)
	}
	h.t.Fatalf("no %v hit named %s", kind, name)
	return f32.Point{}
}

func (h *harness) snapshot(img *image.RGBA, name string) {
	h.t.Helper()
	mode := "light"
	if h.dark {
		mode = "dark"
	}
	out := filepath.Join("..", "build", "snapshots")
	if err := os.MkdirAll(out, 0o755); err != nil {
		h.t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(out, "gio-"+name+"-"+mode+".png"))
	if err != nil {
		h.t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		h.t.Fatal(err)
	}
}

func bothModes(t *testing.T, f func(t *testing.T, dark bool)) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) { f(t, dark) })
	}
}

// TestHoverLiftsBarAndShowsTooltip: the hovered bar must lift toward white
// and a tooltip must appear beside the pointer.
func TestHoverLiftsBarAndShowsTooltip(t *testing.T) {
	bothModes(t, func(t *testing.T, dark bool) {
		h := open(t, demo.All()[0].Chart, dark)
		// The bar itself, not just its column: the pixel check needs paint.
		var bar f32.Point
		for _, m := range h.scene.Marks {
			if m.Hit != nil && m.Hit.Label == "Pears" {
				r := m.Shape.(scene.RectShape).Rect
				bar = f32.Pt(float32(r.X+r.W/2)*pxPerDp, float32(r.Y+r.H/2)*pxPerDp)
			}
		}
		before := h.frame()
		after := h.hover(bar)

		series := base.Resolve(base.Series(0), dark)
		mix := func(x uint8) uint8 { return x + uint8(math.Round(float64(255-x)*lift)) }
		want := color.RGBA{R: mix(series.R), G: mix(series.G), B: mix(series.B), A: 255}
		if got := after.RGBAAt(int(bar.X), int(bar.Y)); !near(got, want) {
			t.Errorf("hovered bar pixel = %v, want lifted %v", got, want)
		}
		if !changed(before, after, tipBox(bar, 2)) {
			t.Errorf("nothing painted in the tooltip box %v", tipBox(bar, 2))
		}
		h.snapshot(after, "bar")
	})
}

// TestCrosshairListsVisibleSeriesAndLegendHidesOne: hovering a line chart's
// column lists every series in the tooltip; clicking a legend entry removes
// that series from the plot and from the tooltip, and leaves the rest.
func TestCrosshairListsVisibleSeriesAndLegendHidesOne(t *testing.T) {
	bothModes(t, func(t *testing.T, dark bool) {
		h := open(t, demo.Line(), dark)
		apr := h.target(scene.HitCrosshair, "Apr")
		web, ios, android := seriesColor(0, dark), seriesColor(1, dark), seriesColor(2, dark)

		img := h.hover(apr)
		keys := tipKeys(apr, 3)
		for name, c := range map[string]color.RGBA{"Web": web, "iOS": ios, "Android": android} {
			if !has(img, keys, c) {
				t.Errorf("crosshair tooltip has no %s row (no %v key in %v)", name, c, keys)
			}
		}
		if !has(img, img.Rect, ios) {
			t.Fatal("iOS line not painted before hiding")
		}
		h.snapshot(img, "line-crosshair")

		h.click(h.target(scene.HitLegend, "iOS"))
		if got := h.view.Hidden(); len(got) != 1 || got[0] != "iOS" {
			t.Fatalf("hidden = %v after clicking the iOS legend entry", got)
		}
		img = h.hover(apr)
		// Its legend entry stays, dimmed, so it can be clicked back; nothing
		// else may carry the series color.
		if has(img, img.Rect, ios) {
			t.Error("hidden iOS series still painted (plot or tooltip)")
		}
		keys = tipKeys(apr, 2)
		if !has(img, keys, web) || !has(img, keys, android) {
			t.Error("visible series left the tooltip")
		}
		h.snapshot(img, "line-hidden")

		h.click(h.target(scene.HitLegend, "iOS"))
		if len(h.view.Hidden()) != 0 {
			t.Fatal("a second click must show the series again")
		}
	})
}

// TestStatisticTooltipListsUnkeyedRows: hovering a violin shows its label
// and every summary row, with no empty value line on top and no series key
// beside rows that are statistics, not series.
func TestStatisticTooltipListsUnkeyedRows(t *testing.T) {
	bothModes(t, func(t *testing.T, dark bool) {
		var violin charta.Chart
		for _, d := range demo.All() {
			if d.Name == "Violin" {
				violin = d.Chart
			}
		}
		h := open(t, violin, dark)
		at := h.target(scene.HitMark, "Organic")
		img := h.hover(at)
		var hit *scene.Hit
		for _, m := range h.scene.Marks {
			if m.Hit != nil && m.Hit.Label == "Organic" {
				hit = m.Hit
			}
		}
		lines := 1 + len(hit.Rows)
		surface := base.Resolve(base.Surface, dark)
		bg := color.RGBA{R: surface.R, G: surface.G, B: surface.B, A: 255}
		// The last row sits on the last line: an empty value line on top
		// would push it out of the box.
		x0 := at.X + (tipOffset+tipPad)*pxPerDp
		y0 := at.Y + float32(tipOffset+tipPad+float64(lines-1)*tipLine)*pxPerDp
		last := image.Rect(int(x0), int(y0), int(x0+40*pxPerDp), int(y0+tipLine*pxPerDp))
		if !hasOtherIn(img, last, bg) {
			t.Errorf("no text on the tooltip's last line %v", last)
		}
		if has(img, tipKeys(at, lines-1), seriesColor(0, dark)) {
			t.Error("statistic rows got a series key")
		}
		h.snapshot(img, "violin-hover")
	})
}

// TestTooltipRowsLeadWithTheValueOnlyWhenThereIsOne pins the shared
// tooltip rule: an empty value must not leave a blank first line, only
// series rows carry a key, and hidden series drop out of the rows.
func TestTooltipRowsLeadWithTheValueOnlyWhenThereIsOne(t *testing.T) {
	summary := &scene.Hit{Label: "a", Rows: []scene.TipRow{{Name: "median", Value: "2"}, {Name: "values", Value: "3"}}}
	got := tipRows(summary, nil)
	if len(got) != 3 || got[0].weak != "a" || got[0].strong != "" || got[1].key != base.NoColor || got[1].strong != "2" {
		t.Fatalf("summary rows %+v", got)
	}
	mark := &scene.Hit{Label: "Pears", Value: "18"}
	if got := tipRows(mark, nil); len(got) != 2 || got[0].strong != "18" || got[1].weak != "Pears" {
		t.Fatalf("mark rows %+v", got)
	}
	cross := &scene.Hit{Label: "Apr", Kind: scene.HitCrosshair, Rows: []scene.TipRow{{Color: base.Series(0), Name: "Web", Value: "1"}, {Color: base.Series(1), Name: "iOS", Value: "2"}}}
	if got := tipRows(cross, []string{"iOS"}); len(got) != 2 || got[1].key != base.Series(0) || got[1].weak != "Web" {
		t.Fatalf("crosshair rows %+v", got)
	}
}

// TestViolinOutlineIsFilled: a violin is a filled path; without the fill
// only its outline would show and the shape would read as a line drawing.
func TestViolinOutlineIsFilled(t *testing.T) {
	bothModes(t, func(t *testing.T, dark bool) {
		var violin charta.Chart
		for _, d := range demo.All() {
			if d.Name == "Violin" {
				violin = d.Chart
			}
		}
		h := open(t, violin, dark)
		x, y := violinInside(h.scene)
		img := h.frame()
		surface := base.Resolve(base.Surface, dark)
		if got := img.RGBAAt(int(x*pxPerDp), int(y*pxPerDp)); near(got, color.RGBA{R: surface.R, G: surface.G, B: surface.B, A: 255}) {
			t.Fatalf("violin interior at (%v, %v) is bare surface %v", x, y, got)
		}
	})
}

// violinInside is a point (dp) halfway between the Organic violin's center
// line and its widest edge: inside the fill, clear of the outline and the
// interquartile bar.
func violinInside(sc *scene.Scene) (float64, float64) {
	for _, m := range sc.Marks {
		p, ok := m.Shape.(scene.PathShape)
		if !ok || m.Hit == nil || m.Hit.Label != "Organic" {
			continue
		}
		a := m.Hit.Area
		cx := a.X + a.W/2
		widest := p.Ops[0]
		for _, op := range p.Ops {
			if op.Verb != scene.ClosePath && op.X > widest.X {
				widest = op
			}
		}
		return (cx + widest.X) / 2, widest.Y
	}
	panic("no Organic violin")
}

// TestEveryDemoRenders paints each gallery demo and writes its snapshot.
func TestEveryDemoRenders(t *testing.T) {
	bothModes(t, func(t *testing.T, dark bool) {
		for _, d := range demo.All() {
			h := open(t, d.Chart, dark)
			img := h.frame()
			surface := base.Resolve(base.Surface, dark)
			if !hasOther(img, color.RGBA{R: surface.R, G: surface.G, B: surface.B, A: 255}) {
				t.Errorf("%s painted nothing", d.Name)
			}
			h.snapshot(img, "gallery-"+strings.ReplaceAll(strings.ToLower(d.Name), " ", "-"))
		}
	})
}

// tipBox is the tooltip's top-left part for a pointer at p (px) and a
// tooltip of the given number of lines: the region it surely covers.
func tipBox(p f32.Point, lines int) image.Rectangle {
	x0, y0 := p.X+tipOffset*pxPerDp, p.Y+tipOffset*pxPerDp
	return image.Rect(int(x0), int(y0), int(x0+40*pxPerDp), int(y0+float32(float64(lines)*tipLine+2*tipPad)*pxPerDp))
}

// tipKeys is where a crosshair tooltip's series keys sit for rows rows: the
// key column below the label line. The tooltip is opaque, so nothing from
// the plot shows through.
func tipKeys(p f32.Point, rows int) image.Rectangle {
	x0 := p.X + (tipOffset+tipPad)*pxPerDp
	y0 := p.Y + float32(tipOffset+tipPad+tipLine)*pxPerDp
	return image.Rect(int(x0), int(y0), int(x0+tipKeyW*pxPerDp), int(y0+float32(float64(rows)*tipLine)*pxPerDp))
}

func seriesColor(i int, dark bool) color.RGBA {
	c := base.Resolve(base.Series(i), dark)
	return color.RGBA{R: c.R, G: c.G, B: c.B, A: 255}
}

// near allows for the GPU's sRGB round trip.
func near(a, b color.RGBA) bool {
	d := func(x, y uint8) bool { return math.Abs(float64(x)-float64(y)) <= 2 }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B)
}

func has(img *image.RGBA, r image.Rectangle, c color.RGBA) bool {
	r = r.Intersect(img.Rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if near(img.RGBAAt(x, y), c) {
				return true
			}
		}
	}
	return false
}

func hasOther(img *image.RGBA, c color.RGBA) bool {
	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			if !near(img.RGBAAt(x, y), c) {
				return true
			}
		}
	}
	return false
}

func hasOtherIn(img *image.RGBA, r image.Rectangle, c color.RGBA) bool {
	r = r.Intersect(img.Rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if !near(img.RGBAAt(x, y), c) {
				return true
			}
		}
	}
	return false
}

func changed(a, b *image.RGBA, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				return true
			}
		}
	}
	return false
}

// fixedScene is a chart whose scene is given, for backend-only checks.
type fixedScene struct{ s scene.Scene }

func (f fixedScene) Scene(float64, float64, base.TextMeasure) (*scene.Scene, error) {
	s := f.s
	return &s, nil
}

// TestRectOutlinesAreDrawn: treemap cells and mind-map pills are outlined
// rects; an outline the painter dropped would leave them unbounded.
func TestRectOutlinesAreDrawn(t *testing.T) {
	h := open(t, fixedScene{scene.Scene{Width: wDp, Height: hDp, Marks: []scene.Mark{{
		Shape:  scene.RectShape{Rect: scene.Rect{X: 10, Y: 10, W: 20, H: 20}},
		Stroke: &scene.Stroke{Color: base.Series(1), Width: 2},
	}}}}, false)
	img := h.frame()
	edge := seriesColor(1, false)
	if got := img.RGBAAt(10*pxPerDp, 20*pxPerDp); !near(got, edge) {
		t.Fatalf("left edge is %v, want the outline %v", got, edge)
	}
	if got := img.RGBAAt(20*pxPerDp, 20*pxPerDp); near(got, edge) {
		t.Fatalf("an unfilled rect's interior is %v, the outline color", got)
	}
}

// months is a year of visits, enough categories to zoom into.
func months() charta.LineChart {
	cats := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	vals := make([]float64, len(cats))
	for i := range vals {
		vals[i] = float64(i+1) * 10
	}
	return charta.LineChart{Categories: cats, Series: []charta.Series{{Name: "Visits", Values: vals}}}
}

// queue sends events and runs one frame.
func (h *harness) queue(evs ...event.Event) *image.RGBA {
	h.router.Queue(evs...)
	return h.frame()
}

// scroll is a wheel event of the given notches (positive zooms in) at p.
func scroll(p f32.Point, notches float32) pointer.Event {
	return pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: p, Scroll: f32.Pt(0, -notches*wheelNotch*pxPerDp)}
}

func press(p f32.Point, at time.Duration, mods key.Modifiers) pointer.Event {
	return pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: p, Time: at, Modifiers: mods}
}

func release(p f32.Point, at time.Duration) pointer.Event {
	return pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: p, Time: at}
}

func move(p f32.Point) pointer.Event {
	return pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: p}
}

// drag presses at from, moves to to in ten frames, and releases there.
// mid, when set, sees the frame halfway through.
func (h *harness) drag(from, to f32.Point, mods key.Modifiers, mid func(*image.RGBA)) {
	h.queue(move(from), press(from, 0, mods))
	for k := 1; k <= 10; k++ {
		img := h.queue(move(from.Add(to.Sub(from).Mul(float32(k) / 10))))
		if k == 5 && mid != nil {
			mid(img)
		}
	}
	h.queue(release(to, 0))
}

// TestACategoryAxisZoomsPansBrushesAndResets: the wheel zooms around the
// pointer (fractions of a notch add up, one event is at most one notch),
// a drag pans by whole categories (slow steps carry over), a double click
// shows everything and a shift-drag zooms to the brushed months, shading
// them while it lasts. Mirrors the egui test
// a_category_axis_zooms_pans_brushes_and_resets.
func TestACategoryAxisZoomsPansBrushesAndResets(t *testing.T) {
	chart := months()
	h := open(t, chart, false)
	metric := unit.Metric{PxPerDp: pxPerDp, PxPerSp: pxPerDp}
	mid := f32.Pt(wDp*pxPerDp/2, hDp*pxPerDp/2)
	at, ok := zoom.ColumnAt(h.scene, wDp/2)
	if !ok {
		t.Fatal("a line chart has no category columns")
	}
	h.hover(mid)
	span := func() int { return h.view.window.End - h.view.window.Start }

	// Half a notch is not a step yet; the second half is, and the frame
	// that handled it already paints the zoomed chart.
	h.queue(scroll(mid, 0.5))
	if h.view.window != zoom.Full(12) {
		t.Fatalf("half a notch zoomed: %+v", h.view.window)
	}
	img := h.queue(scroll(mid, 0.5))
	if want := zoom.Full(12).Wheel(at, 1); h.view.window != want {
		t.Fatalf("two half notches: %+v, want %+v", h.view.window, want)
	}
	if changed(img, h.frame(), img.Rect) {
		t.Fatal("the zooming frame painted the chart before the zoom")
	}
	// Windows sends 120 per notch: one event is one notch either way.
	h.queue(scroll(mid, 12))
	if span() != 8 {
		t.Fatalf("a 12-notch event in from 10 months shows %d", span())
	}
	h.queue(scroll(mid, -12))
	if span() != 10 {
		t.Fatalf("a 12-notch event out from 8 months shows %d", span())
	}
	h.queue(scroll(mid, 1))
	zoomed := h.view.window
	if !(zoomed.Start > 0 && zoomed.End < 12 && zoomed.End-zoomed.Start == 8) {
		t.Fatalf("zoomed in over the middle: %+v", zoomed)
	}

	// Dragging left by a column and a bit, in ten steps far narrower than a
	// column, brings one later month in: the remainder carries between
	// frames, and the movement before the drag threshold counts too.
	zsc, err := chart.Window(zoomed.Start, zoomed.End).Scene(wDp, hDp, Measure{Metric: metric, Shaper: h.view.Shaper})
	if err != nil {
		t.Fatal(err)
	}
	cw, _ := zoom.ColumnWidth(zsc)
	if cw/10 <= dragSlop || zoomed.End > 11 {
		t.Fatalf("a %vdp column in %+v leaves no room for the test", cw, zoomed)
	}
	h.drag(mid, mid.Sub(f32.Pt(float32(cw+4)*pxPerDp, 0)), 0, nil)
	panned := h.view.window
	if panned != zoomed.Pan(1) {
		t.Fatalf("dragging %vdp of %vdp columns: %+v → %+v", cw+4, cw, zoomed, panned)
	}
	if h.view.pan != 0 {
		t.Fatalf("the pan remainder outlived the drag: %v", h.view.pan)
	}

	// A click may jitter a little and stay a click. Two clicks half a
	// second apart are not a double click; a third right after the second
	// is, and shows the whole year again.
	click := func(at time.Duration) {
		h.queue(press(mid, at, 0), move(mid.Add(f32.Pt(3*pxPerDp, 0))), release(mid, at))
	}
	click(time.Second)
	click(1500 * time.Millisecond)
	if h.view.window != panned {
		t.Fatalf("slow clicks moved the window: %+v → %+v", panned, h.view.window)
	}
	click(1600 * time.Millisecond)
	if !h.view.window.IsFull() {
		t.Fatalf("a double click left %+v", h.view.window)
	}
	// The click after a double click starts a new pair.
	h.queue(scroll(mid, 1))
	again := h.view.window
	click(1650 * time.Millisecond)
	if h.view.window != again {
		t.Fatalf("a third quick click reset the zoom: %+v", h.view.window)
	}
	click(1700 * time.Millisecond)
	if !h.view.window.IsFull() {
		t.Fatalf("a second double click left %+v", h.view.window)
	}

	surface := base.Resolve(base.Surface, false)
	bg := color.RGBA{R: surface.R, G: surface.G, B: surface.B, A: 255}
	// The top edge is bare surface but for the shading.
	top := func(img *image.RGBA, xDp float64) color.RGBA { return img.RGBAAt(int(xDp*pxPerDp), 1) }
	bare := func(img *image.RGBA) bool {
		return !hasOtherIn(img, image.Rect(0, 0, wDp*pxPerDp, 2), bg)
	}

	// A cancelled shift-drag leaves no brush and no shading.
	fullCW, _ := zoom.ColumnWidth(h.scene)
	x29, _ := zoom.ColumnX(h.scene, 2.9)
	x65, _ := zoom.ColumnX(h.scene, 6.5)
	left, right := f32.Pt(float32(x29)*pxPerDp, mid.Y), f32.Pt(float32(x65)*pxPerDp, mid.Y)
	h.queue(move(left), press(left, 0, key.ModShift))
	h.queue(move(right))
	if !h.view.brushing {
		t.Fatal("a shift-drag did not start a brush")
	}
	img = h.queue(pointer.Event{Kind: pointer.Cancel})
	if h.view.brushing || !h.view.window.IsFull() || !bare(img) {
		t.Fatalf("after a cancel: brushing=%v window=%+v", h.view.brushing, h.view.window)
	}

	// Shift-dragging across about a third of the plot zooms to the months
	// it touched; the drag starts where the button went down.
	want := zoom.Full(12).Brush(2, 6)
	shaded := func(img *image.RGBA) {
		// Halfway, the pointer is in column 4: columns 2 to 4 are shaded.
		x2, _ := zoom.ColumnX(h.scene, 2)
		x4, _ := zoom.ColumnX(h.scene, 4)
		x5, _ := zoom.ColumnX(h.scene, 5)
		for _, x := range []float64{x2 + fullCW/2, x4 + fullCW/2} {
			if near(top(img, x), bg) {
				t.Errorf("no shading at %vdp, inside the brush", x)
			}
		}
		for _, x := range []float64{x2 - fullCW/2, x5 + fullCW/2} {
			if !near(top(img, x), bg) {
				t.Errorf("shading at %vdp, outside the brush", x)
			}
		}
	}
	h.drag(left, right, key.ModShift, shaded)
	if h.view.window != want {
		t.Fatalf("brushed columns 2.9..6.5: %+v, want %+v", h.view.window, want)
	}
	if n := want.End - want.Start; n < 2 || n > 5 || want.Start < 2 {
		t.Fatalf("the brush picked %+v, not about a third of the year", want)
	}
	if img := h.frame(); h.view.brushing || !bare(img) {
		t.Fatal("the brush outlived the drag")
	}
	h.snapshot(h.frame(), "zoomed")

	// Brushing past the last column stops at it: columns 1.. of the
	// window, so categories 3..6.
	bsc, err := chart.Window(want.Start, want.End).Scene(wDp, hDp, Measure{Metric: metric, Shaper: h.view.Shaper})
	if err != nil {
		t.Fatal(err)
	}
	x15, _ := zoom.ColumnX(bsc, 1.5)
	from := f32.Pt(float32(x15)*pxPerDp, mid.Y)
	h.drag(from, f32.Pt(wDp*pxPerDp-2, mid.Y), key.ModShift, nil)
	if got := h.view.window; got != (zoom.Window{Start: 3, End: 7, Len: 12}) {
		t.Fatalf("brushed to the right edge: %+v", got)
	}

	// A different chart starts over at its whole axis.
	h.view.Chart = charta.LineChart{Categories: chart.Categories[:6], Series: []charta.Series{{Name: "Visits", Values: chart.Series[0].Values[:6]}}}
	h.frame()
	if h.view.window != zoom.Full(6) {
		t.Fatalf("a new chart kept the old window: %+v", h.view.window)
	}
}

// TestALegendDoubleClickKeepsTheZoom: toggling a series twice in quick
// succession is two legend clicks, not a double click on the plot, so the
// zoom stays. Mirrors the legend step of the egui test.
func TestALegendDoubleClickKeepsTheZoom(t *testing.T) {
	chart := months()
	signups := make([]float64, len(chart.Categories))
	for i := range signups {
		signups[i] = float64(i+1) * 4
	}
	chart.Series = append(chart.Series, charta.Series{Name: "Signups", Values: signups})
	h := open(t, chart, false)
	mid := f32.Pt(wDp*pxPerDp/2, hDp*pxPerDp/2)
	h.hover(mid)
	h.queue(scroll(mid, 1))
	zoomed := h.view.window
	if zoomed.IsFull() {
		t.Fatal("a notch did not zoom in")
	}
	legend := h.target(scene.HitLegend, "Signups")
	for _, at := range []time.Duration{time.Second, 1100 * time.Millisecond} {
		h.queue(move(legend), press(legend, at, 0), release(legend, at))
	}
	if h.view.window != zoomed {
		t.Fatalf("a legend double click moved the window: %+v → %+v", zoomed, h.view.window)
	}
	if len(h.view.hidden) != 0 {
		t.Fatalf("two legend clicks left %v hidden", h.view.hidden)
	}
}

// TestGesturesCarryRemainders: wheel fractions and drag distance short of a
// step are kept for the next frame, and only what makes a whole step is
// spent, so a slow trackpad or drag moves as far as a fast one.
func TestGesturesCarryRemainders(t *testing.T) {
	var marks []scene.Mark
	for i := range 12 {
		marks = append(marks, scene.Mark{Shape: scene.LineShape{}, Hit: &scene.Hit{Kind: scene.HitCrosshair, Area: &scene.Rect{X: float64(i) * 10, W: 10}}})
	}
	sc := &scene.Scene{Marks: marks}
	v := &ChartView{window: zoom.Window{Start: 3, End: 9, Len: 12}}
	step := func(in zoomInput) zoom.Window {
		v.window = v.gestures(sc, in, 1)
		return v.window
	}
	if got := step(zoomInput{dragged: -15}); got != (zoom.Window{Start: 4, End: 10, Len: 12}) || v.pan != -5 {
		t.Fatalf("1.5 columns left: %+v, %v left over", got, v.pan)
	}
	if got := step(zoomInput{dragged: -6}); got != (zoom.Window{Start: 5, End: 11, Len: 12}) || v.pan != -1 {
		t.Fatalf("0.6 more: %+v, %v left over", got, v.pan)
	}
	if step(zoomInput{stopped: true, stopAt: f32.Pt(30, 0)}); v.pan != 0 {
		t.Fatalf("a drag's remainder outlived it: %v", v.pan)
	}
	before := v.window
	if got := step(zoomInput{notches: 0.6, scrollAt: f32.Pt(30, 0)}); got != before {
		t.Fatalf("0.6 notches zoomed: %+v", got)
	}
	if got := step(zoomInput{notches: 0.6, scrollAt: f32.Pt(30, 0)}); got != before.Wheel(3, 1) || math.Abs(v.wheel-0.2) > 1e-9 {
		t.Fatalf("1.2 notches: %+v, %v left over", got, v.wheel)
	}
	// A scene without columns maps no pointer: nothing moves, nothing is
	// kept.
	v.wheel, before = 0, v.window
	if got := v.gestures(&scene.Scene{}, zoomInput{notches: 2, dragged: 50}, 1); got != before || v.wheel != 0 {
		t.Fatalf("gestures without columns: %+v, wheel %v", got, v.wheel)
	}
	// Zero-width columns (a degenerate layout) do not turn a drag into an
	// infinite pan.
	flat := &scene.Scene{Marks: []scene.Mark{{Shape: scene.LineShape{}, Hit: &scene.Hit{Kind: scene.HitCrosshair, Area: &scene.Rect{X: 5}}}}}
	if got := v.gestures(flat, zoomInput{dragged: 50}, 1); got != before {
		t.Fatalf("a drag over zero-width columns: %+v", got)
	}
}

// TestOnlyChartsWithACategoryAxisZoom: a bar chart, or a line chart too
// short to narrow, ignores the wheel and drags, so an enclosing list still
// scrolls and clicks still toggle the legend.
func TestOnlyChartsWithACategoryAxisZoom(t *testing.T) {
	short := charta.LineChart{Categories: []string{"a", "b"}, Series: []charta.Series{{Name: "s", Values: []float64{1, 2}}}}
	for name, c := range map[string]charta.Chart{"bar": demo.All()[0].Chart, "two categories": short} {
		h := open(t, c, false)
		mid := f32.Pt(wDp*pxPerDp/2, hDp*pxPerDp/2)
		h.hover(mid)
		h.queue(scroll(mid, 3))
		h.drag(mid, mid.Sub(f32.Pt(300*pxPerDp, 0)), 0, func(*image.RGBA) {
			if h.view.dragging {
				t.Errorf("%s: a drag started", name)
			}
		})
		if h.view.window != (zoom.Window{}) {
			t.Errorf("%s: zoom state %+v", name, h.view.window)
		}
	}
	// Exactly MinSpan+1 categories do zoom.
	three := charta.LineChart{Categories: []string{"a", "b", "c"}, Series: []charta.Series{{Name: "s", Values: []float64{1, 2, 3}}}}
	h := open(t, three, false)
	mid := f32.Pt(wDp*pxPerDp/2, hDp*pxPerDp/2)
	h.hover(mid)
	h.queue(scroll(mid, 1))
	if w := h.view.window; w.Len != 3 || w.End-w.Start != 2 {
		t.Fatalf("three categories, one notch in: %+v", w)
	}
}

func TestLinesJoinRoundWithoutDoublingTranslucency(t *testing.T) {
	// A sharp peak in a translucent color: the other backends join lines
	// round, and the join must not paint the overlap twice.
	line := scene.Mark{
		Shape:  scene.PolylineShape{Points: []scene.Point{{X: 100, Y: 300}, {X: 200, Y: 100}, {X: 300, Y: 300}}},
		Fill:   base.NoColor,
		Stroke: &scene.Stroke{Color: base.Link(0), Width: 12},
	}
	h := open(t, fixedScene{scene.Scene{Width: wDp, Height: hDp, Marks: []scene.Mark{line}}}, false)
	img := h.frame()
	at := func(x, y float64) color.RGBA { return img.RGBAAt(int(x*pxPerDp), int(y*pxPerDp)) }
	mid := at(150, 200)
	surface := base.Resolve(base.Surface, false)
	if near(mid, color.RGBA{R: surface.R, G: surface.G, B: surface.B, A: 255}) {
		t.Fatal("the line is not drawn")
	}
	// 0.9 of the half width above the peak: inside a round join, outside a
	// bevel (0.45 here) or no join at all.
	if c := at(200, 100-0.9*6); !near(c, mid) {
		t.Errorf("no round join above the peak: %v, the line is %v", c, mid)
	}
	if c := at(200, 100); !near(c, mid) {
		t.Errorf("the peak, where segments and join overlap, is %v; the line is %v", c, mid)
	}
}

// plotOf is where chart's full plot sits, as the view lays it out.
func (h *harness) plotOf(chart charta.Viewable) scene.Plot {
	h.t.Helper()
	m := Measure{Metric: unit.Metric{PxPerDp: pxPerDp, PxPerSp: pxPerDp}, Shaper: h.view.Shaper}
	_, p, err := chart.PlotScene(wDp, hDp, m)
	if err != nil {
		h.t.Fatal(err)
	}
	return p
}

// across is the point (px) tx of the way across plot and ty down it.
func across(plot scene.Plot, tx, ty float64) f32.Point {
	r := plot.Rect
	return f32.Pt(float32(r.X+tx*r.W)*pxPerDp, float32(r.Y+ty*r.H)*pxPerDp)
}

func dpOf(p f32.Point) (float64, float64) { return float64(p.X / pxPerDp), float64(p.Y / pxPerDp) }

// patch is a few px around p, to compare frames by.
func patch(p f32.Point) image.Rectangle {
	x, y := int(p.X), int(p.Y)
	return image.Rect(x-3, y-3, x+3, y+3)
}

// TestAScatterplotZoomsBothAxesByWheelDragAndBox: the wheel zooms both axes
// around the data under the pointer, a drag pans both, a double click shows
// everything and a shift-drag zooms to the box brushed, shaded while it
// lasts. Mirrors the egui test of the same name.
func TestAScatterplotZoomsBothAxesByWheelDragAndBox(t *testing.T) {
	var pts []charta.Point
	for i := range 60 {
		pts = append(pts, charta.Point{X: float64(i), Y: float64(i * 37 % 100)})
	}
	chart := charta.ScatterChart{Series: []charta.PointSeries{{Name: "Trials", Points: pts}}}
	h := open(t, chart, false)
	plot := h.plotOf(chart)
	full := zoom.FullView(plot)

	at := across(plot, 1.0/3, 1.0/3)
	h.hover(at)
	img := h.queue(scroll(at, 1))
	x, y := dpOf(at)
	if want := full.Wheel(plot, x, y, 1); !h.view.view.Equal(want) {
		t.Fatalf("a notch: %+v, want %+v", h.view.view, want)
	}
	if changed(img, h.frame(), img.Rect) {
		t.Fatal("the zooming frame painted the chart before the zoom")
	}
	zoomed := h.view.view

	// Dragging left and down: the data follows the pointer, so later x and
	// higher y come into view, the span kept.
	mid := across(plot, 0.5, 0.5)
	h.drag(mid, mid.Add(f32.Pt(-120*pxPerDp, 60*pxPerDp)), 0, nil)
	panned := h.view.view
	if !(panned.X.Lo > zoomed.X.Lo && panned.Y.Lo > zoomed.Y.Lo) {
		t.Fatalf("dragging left and down: %+v → %+v", zoomed, panned)
	}
	if math.Abs((panned.X.Hi-panned.X.Lo)-(zoomed.X.Hi-zoomed.X.Lo)) > 1e-9 {
		t.Fatalf("panning changed the span: %+v → %+v", zoomed, panned)
	}

	for _, at := range []time.Duration{time.Second, 1100 * time.Millisecond} {
		h.queue(move(mid), press(mid, at, 0), release(mid, at))
	}
	if !h.view.view.IsFull() {
		t.Fatalf("a double click left %+v", h.view.view)
	}

	// The brush is a box: halfway it shades from where the button went down
	// to the pointer, and nothing below or right of that.
	from, to := across(plot, 0.25, 0.25), across(plot, 0.6, 0.7)
	before := h.hover(from)
	shaded := func(img *image.RGBA) {
		if !changed(before, img, patch(across(plot, 0.3, 0.3))) {
			t.Error("no shading inside the box")
		}
		for _, out := range []f32.Point{across(plot, 0.3, 0.9), across(plot, 0.9, 0.3)} {
			if changed(before, img, patch(out)) {
				t.Errorf("shading at %v, outside the box", out)
			}
		}
	}
	h.drag(from, to, key.ModShift, shaded)
	x0, y0 := dpOf(from)
	x1, y1 := dpOf(to)
	if want := full.Brush(plot, x0, y0, x1, y1); !h.view.view.Equal(want) {
		t.Fatalf("brushed: %+v, want %+v", h.view.view, want)
	}
	if h.view.brushing {
		t.Fatal("the brush outlived the drag")
	}
	h.snapshot(h.frame(), "zoomed-scatter")

	// New data with a wider extent starts over at all of it.
	wide := chart
	wide.Series = []charta.PointSeries{{Name: "Trials", Points: append(slices.Clone(pts), charta.Point{X: 500, Y: 500})}}
	h.view.Chart = wide
	h.frame()
	if want := zoom.FullView(h.plotOf(wide)); !h.view.view.Equal(want) {
		t.Fatalf("new data kept the old view: %+v", h.view.view)
	}
}

// TestATimeAxisZoomsAlongXOnly: a candlestick chart zooms and pans time
// alone, and its brush is a band across the plot's height. Mirrors the egui
// test of the same name.
func TestATimeAxisZoomsAlongXOnly(t *testing.T) {
	var candles []charta.Candle
	for i := range 30 {
		o := 100 + float64(i*7%11)
		candles = append(candles, charta.Candle{Time: base.ToMS(2026, 1, i+1, 0, 0, 0), Open: o, High: o + 4, Low: o - 3, Close: o + 1})
	}
	chart := charta.CandlestickChart{Data: candles}
	h := open(t, chart, false)
	plot := h.plotOf(chart)
	full := zoom.FullView(plot)
	if full.Y != nil {
		t.Fatal("a time chart zooms y")
	}

	mid := across(plot, 0.5, 0.5)
	h.hover(mid)
	h.queue(scroll(mid, 1))
	zoomed := h.view.view
	if zoomed.IsFull() || zoomed.Y != nil {
		t.Fatalf("a notch in: %+v", zoomed)
	}
	// A drag straight down has nothing to pan.
	h.drag(mid, mid.Add(f32.Pt(0, 120*pxPerDp)), 0, nil)
	if !h.view.view.Equal(zoomed) {
		t.Fatalf("a vertical drag moved the view: %+v → %+v", zoomed, h.view.view)
	}

	for _, at := range []time.Duration{time.Second, 1100 * time.Millisecond} {
		h.queue(move(mid), press(mid, at, 0), release(mid, at))
	}
	from, to := across(plot, 0.2, 0.5), across(plot, 0.5, 0.5)
	before := h.hover(from)
	band := func(img *image.RGBA) {
		// Halfway the pointer is at 0.35 across: the band reaches the
		// plot's top and bottom there, and stops short of 0.45.
		for _, in := range []f32.Point{across(plot, 0.27, 0.03), across(plot, 0.27, 0.97)} {
			if !changed(before, img, patch(in)) {
				t.Errorf("no shading at %v, inside the band", in)
			}
		}
		if changed(before, img, patch(across(plot, 0.45, 0.03))) {
			t.Error("shading past the pointer")
		}
	}
	h.drag(from, to, key.ModShift, band)
	x0, y0 := dpOf(from)
	x1, y1 := dpOf(to)
	if want := full.Brush(plot, x0, y0, x1, y1); !h.view.view.Equal(want) {
		t.Fatalf("brushed: %+v, want %+v", h.view.view, want)
	}
	h.snapshot(h.frame(), "zoomed-candles")
}
