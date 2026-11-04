package fyne

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/demo"
	"github.com/linzeyan/charta/internal/raster"
	"github.com/linzeyan/charta/scene"
)

const viewW, viewH = 720, 440

// harness shows one chart in a headless Fyne window; events go through the
// test driver's canvas, so routing to the widget is exercised too.
type harness struct {
	t      *testing.T
	win    fyne.Window
	view   *ChartView
	origin fyne.Position
}

func open(t *testing.T, c charta.Chart, dark bool) *harness {
	t.Helper()
	test.NewTempApp(t)
	v := NewChartView(c)
	v.Dark = dark
	w := test.NewWindow(v)
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(viewW+32, viewH+32))
	v.Resize(fyne.NewSize(viewW, viewH))
	origin := fyne.CurrentApp().Driver().AbsolutePositionForObject(v)
	return &harness{t: t, win: w, view: v, origin: origin}
}

// scene is what the view paints: same size, same measure.
func (h *harness) scene() *scene.Scene {
	h.t.Helper()
	sc, err := h.view.Chart.Scene(viewW, viewH, Measure{})
	if err != nil {
		h.t.Fatal(err)
	}
	return sc
}

func (h *harness) at(x, y float64) fyne.Position {
	return h.origin.Add(fyne.NewPos(float32(x), float32(y)))
}

func (h *harness) capture() image.Image { return h.win.Canvas().Capture() }

func (h *harness) pixel(img image.Image, x, y float64) color.NRGBA {
	p := h.at(x, y)
	return color.NRGBAModel.Convert(img.At(int(p.X), int(p.Y))).(color.NRGBA)
}

func (h *harness) save(name string) {
	h.t.Helper()
	dir := filepath.Join("..", "build", "snapshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "fyne-"+name+".png"))
	if err != nil {
		h.t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, h.capture()); err != nil {
		h.t.Fatal(err)
	}
}

func center(sc *scene.Scene, pick func(scene.Mark) bool) (float64, float64) {
	for _, m := range sc.Marks {
		if m.Hit != nil && m.Hit.Area != nil && pick(m) {
			a := m.Hit.Area
			return a.X + a.W/2, a.Y + a.H/2
		}
	}
	panic("no mark with a hit area matches")
}

func near(a color.NRGBA, b base.Rgba, tol int) bool {
	d := func(x, y uint8) bool { return max(int(x)-int(y), int(y)-int(x)) <= tol }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B)
}

// count is how many pixels of img are within tol of c.
func count(img image.Image, c base.Rgba, tol int) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if near(color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA), c, tol) {
				n++
			}
		}
	}
	return n
}

func modes(t *testing.T, f func(t *testing.T, dark bool)) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) { f(t, dark) })
	}
}

func TestMeasureUsesTheThemeFont(t *testing.T) {
	test.NewTempApp(t)
	m := Measure{}
	// Proportional widths prove the glyphs came from a real face rather than
	// the gap left for missing characters.
	if i, w := m.Width("i", 12, base.Regular), m.Width("W", 12, base.Regular); w < 1.5*i {
		t.Fatalf("i=%v W=%v: not a proportional font", i, w)
	}
	if r, b := m.Width("Revenue", 12, base.Regular), m.Width("Revenue", 12, base.Semibold); b <= r {
		t.Fatalf("semibold %v is not wider than regular %v", b, r)
	}
}

func TestHoverLiftsTheBarAndDrawsItsTooltip(t *testing.T) {
	modes(t, func(t *testing.T, dark bool) {
		h := open(t, demo.All()[0].Chart, dark)
		sc := h.scene()
		var bar scene.Rect
		for _, m := range sc.Marks {
			if r, ok := m.Shape.(scene.RectShape); ok && m.Hit != nil && m.Hit.Label == "Pears" {
				bar = r.Rect
			}
		}
		inside := [2]float64{bar.X + bar.W/2, bar.Y + bar.H - 3}
		idle := h.capture()
		if c := h.pixel(idle, inside[0], inside[1]); !near(c, base.Resolve(base.Series(0), dark), 2) {
			t.Fatalf("Pears bar not painted in series-1: %v", c)
		}

		px, py := bar.X+bar.W/2, bar.Y+4
		test.MoveMouse(h.win.Canvas(), h.at(px, py))
		hovered := h.capture()
		if c := h.pixel(hovered, inside[0], inside[1]); near(c, base.Resolve(base.Series(0), dark), 2) {
			t.Fatalf("hovered bar is not lifted: %v", c)
		}
		// The tooltip sits below-right of the pointer, clear of the bar; its
		// fill is the surface, so look for its border and text instead.
		changed := 0
		for dx := 0.0; dx < 60; dx++ {
			for dy := 0.0; dy < 40; dy++ {
				x, y := px+raster.TipOffset+dx, py+raster.TipOffset+dy
				if h.pixel(idle, x, y) != h.pixel(hovered, x, y) {
					changed++
				}
			}
		}
		if changed < 50 {
			t.Fatalf("no tooltip drawn (%d pixels changed)", changed)
		}
		h.save("bar-hover-" + map[bool]string{false: "light", true: "dark"}[dark])
	})
}

func TestCrosshairListsVisibleSeriesAndLegendHidesOne(t *testing.T) {
	modes(t, func(t *testing.T, dark bool) {
		h := open(t, demo.Line(), dark)
		sc := h.scene()
		ios := base.Resolve(base.Series(1), dark)
		aprX, aprY := center(sc, func(m scene.Mark) bool {
			return m.Hit.Kind == scene.HitCrosshair && m.Hit.Label == "Apr"
		})
		legX, legY := center(sc, func(m scene.Mark) bool {
			return m.Hit.Kind == scene.HitLegend && m.Series == "iOS"
		})

		test.MoveMouse(h.win.Canvas(), h.at(aprX, aprY))
		if i := h.view.hovered(); i < 0 || sc.Marks[i].Hit.Label != "Apr" {
			t.Fatalf("pointer on the Apr column hovers %d", i)
		}
		if n := count(h.capture(), ios, 8); n < 20 {
			t.Fatalf("iOS barely painted before the toggle (%d px)", n)
		}

		test.TapCanvas(h.win.Canvas(), h.at(legX, legY))
		if got := h.view.Hidden(); !slices.Equal(got, []string{"iOS"}) {
			t.Fatalf("hidden = %v after clicking the iOS legend entry", got)
		}
		test.MoveMouse(h.win.Canvas(), h.at(aprX, aprY))
		img := h.capture()
		// Line, end label key, tooltip row: nothing in iOS's color remains
		// (its legend swatch is dimmed, so it blends toward the surface).
		if n := count(img, ios, 8); n != 0 {
			t.Fatalf("hidden iOS series still painted (%d px)", n)
		}
		for _, s := range []int{0, 2} {
			if n := count(img, base.Resolve(base.Series(s), dark), 8); n < 20 {
				t.Fatalf("visible series %d vanished (%d px)", s, n)
			}
		}
		h.save("line-hidden-" + map[bool]string{false: "light", true: "dark"}[dark])

		test.TapCanvas(h.win.Canvas(), h.at(legX, legY))
		if len(h.view.Hidden()) != 0 {
			t.Fatalf("second click did not restore iOS: %v", h.view.Hidden())
		}
	})
}

// TestStatisticTooltipListsUnkeyedRows: hovering a violin shows its label
// and every summary row, with no empty value line on top and no series key
// beside rows that are statistics, not series.
func TestStatisticTooltipListsUnkeyedRows(t *testing.T) {
	modes(t, func(t *testing.T, dark bool) {
		var violin charta.Chart
		for _, d := range demo.All() {
			if d.Name == "Violin" {
				violin = d.Chart
			}
		}
		h := open(t, violin, dark)
		sc := h.scene()
		px, py := center(sc, func(m scene.Mark) bool { return m.Hit.Label == "Organic" })
		test.MoveMouse(h.win.Canvas(), h.at(px, py))
		hit := sc.Marks[h.view.hovered()].Hit
		if hit.Label != "Organic" {
			t.Fatalf("hovering the Organic column hits %q", hit.Label)
		}
		img := h.capture()
		lines := 1 + len(hit.Rows)
		surface := base.Resolve(base.Surface, dark)
		left := px + raster.TipOffset + raster.TipPad
		// The last row sits on the last line: an empty value line on top
		// would push it out of the box.
		ink := 0
		top := py + raster.TipOffset + raster.TipPad + float64(lines-1)*raster.TipLine
		for dx := 0.0; dx < 40; dx++ {
			for dy := 0.0; dy < raster.TipLine; dy++ {
				if !near(h.pixel(img, left+dx, top+dy), surface, 8) {
					ink++
				}
			}
		}
		if ink < 10 {
			t.Errorf("no text on the tooltip's last line (%d px)", ink)
		}
		series := base.Resolve(base.Series(0), dark)
		for dx := 0.0; dx < raster.TipKeyW; dx++ {
			for dy := raster.TipLine; dy < float64(lines)*raster.TipLine; dy++ {
				if near(h.pixel(img, left+dx, py+raster.TipOffset+raster.TipPad+dy), series, 8) {
					t.Fatal("statistic rows got a series key")
				}
			}
		}
		h.save("violin-hover-" + map[bool]string{false: "light", true: "dark"}[dark])
	})
}

// TestViolinOutlineIsFilled: a violin is a filled path; without the fill
// only its outline would show and the shape would read as a line drawing.
func TestViolinOutlineIsFilled(t *testing.T) {
	modes(t, func(t *testing.T, dark bool) {
		var violin charta.Chart
		for _, d := range demo.All() {
			if d.Name == "Violin" {
				violin = d.Chart
			}
		}
		h := open(t, violin, dark)
		x, y := violinInside(h.scene())
		if got := h.pixel(h.capture(), x, y); near(got, base.Resolve(base.Surface, dark), 1) {
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

func TestEveryDemoRenders(t *testing.T) {
	modes(t, func(t *testing.T, dark bool) {
		for _, d := range demo.All() {
			h := open(t, d.Chart, dark)
			h.capture()
			if h.view.err != nil {
				t.Fatalf("%s: %v", d.Name, h.view.err)
			}
			slug := strings.ReplaceAll(strings.ToLower(d.Name), " ", "-")
			h.save("gallery-" + slug + "-" + map[bool]string{false: "light", true: "dark"}[dark])
		}
	})
}

// TestCJKLabelsComeFromTheSystemFonts: Fyne's default theme font has no
// CJK; those characters must still be drawn, from an installed font,
// instead of leaving the title blank.
func TestCJKLabelsComeFromTheSystemFonts(t *testing.T) {
	h := open(t, charta.BarChart{Title: "水果銷量", Data: []charta.Bar{{Label: "蘋果", Value: 30}, {Label: "梨", Value: 80}}}, false)
	img := h.capture()
	ink, paper := base.Resolve(base.TextPrimary, false), base.Resolve(base.Surface, false)
	// Pixels nearer the ink than the paper: thin CJK strokes antialias, so
	// few are exactly the ink.
	dist := func(c color.NRGBA, r, g, b uint8) int {
		d := func(x, y uint8) int { return max(int(x), int(y)) - min(int(x), int(y)) }
		return d(c.R, r) + d(c.G, g) + d(c.B, b)
	}
	n := 0
	for y := 0.0; y < 40; y++ {
		for x := 0.0; x < 160; x++ {
			if c := h.pixel(img, x, y); dist(c, ink.R, ink.G, ink.B) < dist(c, paper.R, paper.G, paper.B) {
				n++
			}
		}
	}
	if n < 10 {
		t.Fatalf("CJK title barely drawn (%d px)", n)
	}
	h.save("cjk-light")
}
