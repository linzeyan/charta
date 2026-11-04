package zoom_test

import (
	"math"
	"testing"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
	"github.com/linzeyan/charta/zoom"
)

func span(w zoom.Window) int { return w.End - w.Start }

func TestTheWheelZoomsAroundThePointer(t *testing.T) {
	w := zoom.Full(100)
	// Pointer at category 75: it stays at three quarters across.
	z := w.Wheel(75, 1)
	if z != (zoom.Window{Start: 15, End: 95, Len: 100}) {
		t.Fatalf("one notch in at 75: %+v", z)
	}
	if back := z.Wheel(60, -1); back != zoom.Full(100) {
		t.Fatalf("one notch back out: %+v", back)
	}
	// Two notches at once are one step squared, not one step.
	if z2 := w.Wheel(50, 2); span(z2) != 64 || z2.Start != 18 {
		t.Fatalf("two notches in at the middle: %+v", z2)
	}
}

// The anchor is clamped to the window: a pointer left of the first column
// zooms around the left edge, one right of the last around the right edge.
func TestTheWheelAnchorStaysInsideTheWindow(t *testing.T) {
	w := zoom.Window{Start: 10, End: 30, Len: 40}
	if z := w.Wheel(-5, 1); z != (zoom.Window{Start: 10, End: 26, Len: 40}) {
		t.Fatalf("anchored left of the window: %+v", z)
	}
	if z := w.Wheel(99, 1); z != (zoom.Window{Start: 14, End: 30, Len: 40}) {
		t.Fatalf("anchored right of the window: %+v", z)
	}
}

func TestZoomStopsAtTwoCategoriesAndTheWholeAxis(t *testing.T) {
	w := zoom.Full(10)
	for range 20 {
		w = w.Wheel(5, 1)
	}
	if span(w) != zoom.MinSpan {
		t.Fatalf("zoomed all the way in: %+v", w)
	}
	for range 20 {
		w = w.Wheel(0, -1)
	}
	if !w.IsFull() {
		t.Fatalf("zoomed all the way out: %+v", w)
	}
	// A far zoom out in one go is the whole axis, not an overflow.
	if z := (zoom.Window{Start: 3, End: 5, Len: 10}).Wheel(1, -500); !z.IsFull() {
		t.Fatalf("500 notches out: %+v", z)
	}
	// An axis narrower than MinSpan cannot zoom in past itself.
	if z := zoom.Full(1).Wheel(0, 3); z != zoom.Full(1) {
		t.Fatalf("one category zoomed in: %+v", z)
	}
}

func TestPanAndBrushStayInsideTheAxis(t *testing.T) {
	w := zoom.Window{Start: 10, End: 20, Len: 30}
	if got := w.Pan(15); got != (zoom.Window{Start: 20, End: 30, Len: 30}) {
		t.Fatalf("pan past the end: %+v", got)
	}
	if got := w.Pan(-15); got != (zoom.Window{Start: 0, End: 10, Len: 30}) {
		t.Fatalf("pan past the start: %+v", got)
	}
	if got := w.Pan(3); got != (zoom.Window{Start: 13, End: 23, Len: 30}) {
		t.Fatalf("pan by three: %+v", got)
	}
	// Columns 2 and 6 of the window are categories 12 and 16.
	if got := w.Brush(6, 2); got != (zoom.Window{Start: 12, End: 17, Len: 30}) {
		t.Fatalf("brush 6..2: %+v", got)
	}
	if got := w.Brush(3, 4); got != (zoom.Window{Start: 13, End: 15, Len: 30}) {
		t.Fatalf("brush of two columns: %+v", got)
	}
	if got := w.Brush(3, 3); got != w {
		t.Fatalf("one column is not a range: %+v", got)
	}
}

func TestIsFullNeedsBothEnds(t *testing.T) {
	if !zoom.Full(5).IsFull() {
		t.Fatal("Full is not full")
	}
	for _, w := range []zoom.Window{{Start: 1, End: 5, Len: 5}, {Start: 0, End: 4, Len: 5}} {
		if w.IsFull() {
			t.Fatalf("%+v is not the whole axis", w)
		}
	}
}

func lineScene(t *testing.T, c charta.Chart) *scene.Scene {
	t.Helper()
	sc, err := c.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

func firstColumn(sc *scene.Scene) scene.Rect {
	for _, m := range sc.Marks {
		if m.Hit != nil && m.Hit.Kind == scene.HitCrosshair {
			return *m.Hit.Area
		}
	}
	panic("no crosshair")
}

func TestColumnsComeFromTheCrosshairAreas(t *testing.T) {
	sc := lineScene(t, charta.LineChart{Categories: []string{"a", "b", "c", "d"}, Series: []charta.Series{{Name: "s", Values: []float64{1, 2, 3, 4}}}})
	w, ok := zoom.ColumnWidth(sc)
	if !ok || w <= 0 {
		t.Fatalf("column width %v %v", w, ok)
	}
	first := firstColumn(sc)
	if w != first.W {
		t.Fatalf("column width %v, first column %v", w, first.W)
	}
	check := func(x, want float64) {
		t.Helper()
		if got, ok := zoom.ColumnAt(sc, x); !ok || got != want {
			t.Fatalf("ColumnAt(%v) = %v %v, want %v", x, got, ok, want)
		}
	}
	check(first.X+2.5*w, 2.5)
	check(first.X+0.25*w, 0.25)
	check(-100, 0)
	check(1e6, 4)
	if x, ok := zoom.ColumnX(sc, 2.5); !ok || x != first.X+2.5*w {
		t.Fatalf("ColumnX(2.5) = %v %v", x, ok)
	}
}

func TestAChartWithoutColumnsHasNoneToMap(t *testing.T) {
	sc := lineScene(t, charta.BarChart{Data: []charta.Bar{{Label: "A", Value: 1}}})
	if _, ok := zoom.ColumnAt(sc, 10); ok {
		t.Fatal("a bar chart has no crosshair columns")
	}
	if _, ok := zoom.ColumnX(sc, 1); ok {
		t.Fatal("ColumnX on a scene without columns")
	}
	if _, ok := zoom.ColumnWidth(sc); ok {
		t.Fatal("ColumnWidth on a scene without columns")
	}
}

// A zero-width last column (a degenerate hit area) maps to its own index
// rather than dividing by zero; x on a shared edge belongs to the column
// that starts there.
func TestAZeroWidthColumnIsItsIndex(t *testing.T) {
	col := func(x, w float64) scene.Mark {
		return scene.Mark{Shape: scene.LineShape{}, Hit: &scene.Hit{Kind: scene.HitCrosshair, Area: &scene.Rect{X: x, W: w}}}
	}
	sc := &scene.Scene{Marks: []scene.Mark{col(0, 10), col(10, 0)}}
	if got, _ := zoom.ColumnAt(sc, 1e6); got != 1 {
		t.Fatalf("x past the zero-width last column: %v", got)
	}
	if got, _ := zoom.ColumnAt(sc, 5); got != 0.5 {
		t.Fatalf("x halfway through the first column: %v", got)
	}
	sc = &scene.Scene{Marks: []scene.Mark{col(0, 10), col(10, 10)}}
	if got, _ := zoom.ColumnAt(sc, 10); got != 1 {
		t.Fatalf("x on the edge between two columns: %v", got)
	}
}

func crosshairs(sc *scene.Scene) int {
	n := 0
	for _, m := range sc.Marks {
		if m.Hit != nil && m.Hit.Kind == scene.HitCrosshair {
			n++
		}
	}
	return n
}

func tickText(sc *scene.Scene, want ...string) bool {
	for _, m := range sc.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok {
			for _, w := range want {
				if tx.Text == w {
					return true
				}
			}
		}
	}
	return false
}

// TestAWindowNarrowsTheChartAndRefitsY: zooming in is for reading the
// visible values, so the y axis must fit them, not the whole series.
func TestAWindowNarrowsTheChartAndRefitsY(t *testing.T) {
	cats := []string{"a", "b", "c", "d"}
	vals := []float64{1, 2, 3, 400}
	charts := map[string]charta.Chart{
		"line":    charta.LineChart{Categories: cats, Series: []charta.Series{{Name: "s", Values: vals}}},
		"area":    charta.AreaChart{Categories: cats, Series: []charta.Series{{Name: "s", Values: vals}}},
		"stacked": charta.StackedAreaChart{Categories: cats, Series: []charta.Series{{Name: "s", Values: vals}, {Name: "t", Values: vals}}},
	}
	for name, c := range charts {
		z, ok := c.(charta.Zoomable)
		if !ok {
			t.Fatalf("%s is not zoomable", name)
		}
		if z.CategoryCount() != 4 {
			t.Fatalf("%s: %d categories", name, z.CategoryCount())
		}
		full := lineScene(t, c)
		if !tickText(full, "100", "400") {
			t.Fatalf("%s: the whole chart has no tick at 100 or 400", name)
		}
		sc := lineScene(t, z.Window(0, 3))
		if n := crosshairs(sc); n != 3 {
			t.Fatalf("%s: %d columns in a window of 3", name, n)
		}
		// The 400 is out of view, so no tick reaches 100.
		if tickText(sc, "100", "400") {
			t.Fatalf("%s: the y axis still fits the hidden 400", name)
		}
		if tickText(sc, "d") || !tickText(sc, "a", "c") {
			t.Fatalf("%s: the x axis is not categories a..c", name)
		}
	}
}

func TestAWindowIsACopyClippedToTheData(t *testing.T) {
	c := charta.LineChart{Title: "T", Categories: []string{"a", "b", "c"}, Series: []charta.Series{{Name: "s", Values: []float64{1, 2, 3}}}}
	w := c.Window(1, 9).(charta.LineChart)
	if w.Title != "T" || len(w.Categories) != 2 || w.Categories[0] != "b" || w.Series[0].Name != "s" || len(w.Series[0].Values) != 2 || w.Series[0].Values[1] != 3 {
		t.Fatalf("window 1..9 of 3 categories: %+v", w)
	}
	w.Series[0].Values[0] = 99
	w.Categories[0] = "z"
	if c.Series[0].Values[1] != 2 || c.Categories[1] != "b" {
		t.Fatal("narrowing a window changed the original chart")
	}
	if e := c.Window(2, 1).(charta.LineChart); len(e.Categories) != 0 || len(e.Series[0].Values) != 0 {
		t.Fatalf("an empty range: %+v", e)
	}
	if e := c.Window(-3, 1).(charta.LineChart); len(e.Categories) != 1 || e.Categories[0] != "a" {
		t.Fatalf("a range starting before 0: %+v", e)
	}
}

func TestOnlyCategoryChartsZoom(t *testing.T) {
	if _, ok := charta.Chart(charta.BarChart{}).(charta.Zoomable); ok {
		t.Fatal("a bar chart claims to zoom")
	}
	spec, err := charta.DecodeSpec([]byte(`{"type":"stackedArea","categories":["a","b"],"series":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if z, ok := spec.(charta.Zoomable); !ok || z.CategoryCount() != 2 {
		t.Fatal("a decoded stacked area spec does not zoom")
	}
}

// A brush given columns past the window's end (a caller that did not clamp)
// still yields a window inside the axis.
func TestABrushPastTheAxisStaysInside(t *testing.T) {
	if got := (zoom.Window{Start: 0, End: 5, Len: 10}).Brush(0, 20); got != zoom.Full(10) {
		t.Fatalf("brush 0..20 of 10 categories: %+v", got)
	}
}

// Dragged right from inside column 2 to inside column 5, or left from 5 to
// 2, a brush covers columns 2 through 5: edges 2 and 6. Mirrors the Rust
// a_brush_covers_its_end_columns_either_way.
func TestABrushCoversItsEndColumnsEitherWay(t *testing.T) {
	for _, c := range [][2]float64{{2.4, 5.7}, {5.7, 2.4}} {
		if lo, hi := zoom.BrushEdges(c[0], c[1]); lo != 2 || hi != 6 {
			t.Fatalf("brush %v..%v: edges %v, %v", c[0], c[1], lo, hi)
		}
	}
}

// Columns need not be equally wide: x past the last one is the last
// column's right edge, counted in that column's width.
func TestUnevenColumnsMapByTheirOwnWidth(t *testing.T) {
	col := func(x, w float64) scene.Mark {
		return scene.Mark{Shape: scene.LineShape{}, Hit: &scene.Hit{Kind: scene.HitCrosshair, Area: &scene.Rect{X: x, W: w}}}
	}
	sc := &scene.Scene{Marks: []scene.Mark{col(0, 10), col(10, 20)}}
	if got, _ := zoom.ColumnAt(sc, 1e6); got != 2 {
		t.Fatalf("x past uneven columns: %v", got)
	}
	if got, _ := zoom.ColumnAt(sc, 15); got != 1.25 {
		t.Fatalf("x a quarter into the wide column: %v", got)
	}
	// ColumnX and ColumnWidth read the first crosshair; one without an area
	// gives no answer rather than borrowing the next one's.
	bare := scene.Mark{Shape: scene.LineShape{}, Hit: &scene.Hit{Kind: scene.HitCrosshair}}
	sc = &scene.Scene{Marks: []scene.Mark{bare, col(10, 20)}}
	if _, ok := zoom.ColumnWidth(sc); ok {
		t.Fatal("ColumnWidth skipped a crosshair without an area")
	}
	if _, ok := zoom.ColumnX(sc, 1); ok {
		t.Fatal("ColumnX skipped a crosshair without an area")
	}
}

func plot2D() scene.Plot {
	return scene.Plot{Rect: scene.Rect{X: 100, Y: 50, W: 400, H: 200}, X: [2]float64{0, 100}, Y: &[2]float64{-10, 10}}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestASpanZoomsAroundAValueAndStaysInsideItsExtent(t *testing.T) {
	s := zoom.FullSpan(0, 100)
	// The value under the pointer, 75, stays three quarters across.
	if z := s.Wheel(75, 1); !near(z.Lo, 15) || !near(z.Hi, 95) {
		t.Fatalf("%+v", z)
	} else if z.Wheel(60, -1) != s {
		t.Fatal("zooming back out is not everything")
	}
	deep := s
	for range 100 {
		deep = deep.Wheel(50, 1)
	}
	if !near(deep.Hi-deep.Lo, 100*zoom.MinShare) {
		t.Fatalf("deepest %+v", deep)
	}
	mid := zoom.Span{Lo: 40, Hi: 60, Full: [2]float64{0, 100}}
	if p := mid.Pan(50); p.Lo != 80 || p.Hi != 100 {
		t.Fatalf("%+v", p)
	}
	if p := mid.Pan(-50); p.Lo != 0 || p.Hi != 20 {
		t.Fatalf("%+v", p)
	}
	if b := s.Brush(70, 30); b.Lo != 30 || b.Hi != 70 {
		t.Fatalf("%+v", b)
	}
}

func TestAViewMapsGesturesThroughThePlot(t *testing.T) {
	p := plot2D()
	v := zoom.FullView(p)
	// A quarter across and three quarters up: x 25, y 5.
	z := v.Wheel(p, 200, 100, 2)
	zp := p
	zp.X, zp.Y = z.Ranges()
	again := zoom.FullView(zp)
	if w := again.Wheel(zp, 200, 100, 0); !near(w.X.Lo, z.X.Lo) {
		t.Fatal("a zero wheel moved the view")
	}
	// The data under the pointer stays under it: zooming the zoomed plot back
	// out by the same notches lands on the original values there.
	if !near(z.X.Lo+0.25*(z.X.Hi-z.X.Lo), 25) || !near(z.Y.Lo+0.75*(z.Y.Hi-z.Y.Lo), 5) {
		t.Fatalf("the pointer's data moved: %+v %+v", z.X, *z.Y)
	}
	// Dragging right and down shows earlier x and higher y: the data follows
	// the pointer.
	if m := z.Pan(zp, 40, 20); !(m.X.Lo < z.X.Lo && m.Y.Lo > z.Y.Lo) {
		t.Fatalf("%+v %+v", m.X, *m.Y)
	}
	// A brush is a box on a 2D plot; too short on either axis, nothing.
	b := v.Brush(p, 100, 250, 300, 150)
	if b.X.Lo != 0 || b.X.Hi != 50 || b.Y.Lo != -10 || b.Y.Hi != 0 {
		t.Fatalf("%+v %+v", b.X, *b.Y)
	}
	if !v.Brush(p, 100, 250, 300, 248).Equal(v) {
		t.Fatal("a flat brush zoomed")
	}
	if r := v.BrushRect(p, 300, 150, 900, 120); r != (scene.Rect{X: 300, Y: 120, W: 200, H: 30}) {
		t.Fatalf("%+v", r)
	}
	// Along x only, the band spans the plot's height.
	p1 := p
	p1.Y = nil
	if r := zoom.FullView(p1).BrushRect(p1, 300, 150, 200, 160); r.Y != 50 || r.H != 200 {
		t.Fatalf("%+v", r)
	}
}

func TestAViewOfNewDataDoesNotFit(t *testing.T) {
	p := plot2D()
	v := zoom.FullView(p).Wheel(p, 300, 150, 1)
	q := p
	q.X = [2]float64{0, 200}
	if !v.Fits(p) || v.Fits(q) {
		t.Fatal("fits")
	}
}
