package charta

import (
	"math"
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func viewed(t *testing.T, v Viewable) (*scene.Scene, scene.Plot) {
	t.Helper()
	s, p, err := v.PlotScene(640, 400, base.InterMetrics{})
	if err != nil {
		t.Fatal(err)
	}
	return s, p
}

func hasText(s *scene.Scene, want string) bool {
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Text == want {
			return true
		}
	}
	return false
}

func TestAViewShowsThePeriodsThatFitAndRefitsPrices(t *testing.T) {
	d := func(n int) float64 { return float64(base.ToMS(2026, 3, n, 0, 0, 0)) }
	c := CandlestickChart{Data: week()}
	full := build(t, c)
	// Mar 4 and 5 whole; the edge cuts Mar 3's body, so it is left out.
	x := [2]float64{d(3) + 0.2*float64(base.Day), d(5) + 0.5*float64(base.Day)}
	s, p := viewed(t, c.View(x, nil))
	if p.X != [2]float64{math.Round(x[0]), math.Round(x[1])} {
		t.Fatalf("plot x %v", p.X)
	}
	if n := len(hits(s)); n != 2 || !strings.Contains(s.Label, "2 periods") {
		t.Fatalf("%d bars, %q", n, s.Label)
	}
	// The y axis fits Mar 4 and 5 (95 to 102): 104 was a tick before.
	if !hasText(full, "104") || hasText(s, "104") {
		t.Fatal("the prices did not refit")
	}
}

func TestAViewShowsThePointsInsideOverExactlyItsRanges(t *testing.T) {
	var pts []Point
	for i := range 10 {
		pts = append(pts, Point{X: float64(i), Y: float64(i * i)})
	}
	c := ScatterChart{Series: []PointSeries{{Name: "a", Points: pts}}}
	if _, full := viewed(t, c); full.Y == nil || full.Y[0] != 0 {
		t.Fatalf("the full view is niced: %v", full.Y)
	}
	s, p := viewed(t, c.View([2]float64{2, 5}, &[2]float64{0, 20}))
	if p.X != [2]float64{2, 5} || p.Y == nil || *p.Y != [2]float64{0, 20} {
		t.Fatalf("plot %v %v", p.X, p.Y)
	}
	// (2, 4), (3, 9) and (4, 16); (5, 25) is above the view.
	d := dots(s)
	if len(d) != 3 {
		t.Fatalf("%d dots", len(d))
	}
	edge := false
	for _, c := range d {
		if !p.Rect.Contains(c.CX, c.CY) {
			t.Fatalf("(%v, %v) outside %v", c.CX, c.CY, p.Rect)
		}
		edge = edge || math.Abs(c.CX-p.Rect.X) < 1e-9
	}
	if !edge {
		t.Fatal("the point at x = 2 is not on the plot's left edge")
	}
}

func TestAViewCutsBarsSquarelyAndKeepsEveryRow(t *testing.T) {
	d := func(n int) float64 { return float64(base.ToMS(2026, 4, n, 0, 0, 0)) }
	s, p := viewed(t, plan().View([2]float64{d(4), d(15)}, nil))
	var bars []scene.RectShape
	for _, m := range s.Marks {
		if r, ok := m.Shape.(scene.RectShape); ok && m.Hit != nil && m.Hit.Kind == scene.HitMark {
			bars = append(bars, r)
		}
		if _, ok := m.Shape.(scene.PathShape); ok {
			t.Fatal("the launch milestone is out of view")
		}
	}
	// Design (1–8) is cut on the left, Build (6–20) on the right.
	if len(bars) != 2 {
		t.Fatalf("%d bars", len(bars))
	}
	design, buildBar := bars[0], bars[1]
	if math.Abs(design.Rect.X-p.Rect.X) > 1e-9 || design.Radius.TL != 0 || design.Radius.BL != 0 || design.Radius.TR == 0 {
		t.Fatalf("design %+v", design)
	}
	if math.Abs(buildBar.Rect.X+buildBar.Rect.W-(p.Rect.X+p.Rect.W)) > 1e-9 || buildBar.Radius.TR != 0 || buildBar.Radius.BR != 0 {
		t.Fatalf("build %+v", buildBar)
	}
	for _, name := range []string{"Design", "Build", "Launch"} {
		if !hasText(s, name) {
			t.Fatalf("row %s is gone", name)
		}
	}
}

func TestAViewCutsTheLayersAtItsEdges(t *testing.T) {
	x := [2]float64{0.5 * float64(base.Week), 2.5 * float64(base.Week)}
	s, p := viewed(t, streamChart().View(x, nil))
	l, r := p.Rect.X, p.Rect.X+p.Rect.W
	layers := 0
	for _, m := range s.Marks {
		ps, ok := m.Shape.(scene.PathShape)
		if !ok {
			continue
		}
		layers++
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, q := range scene.Flatten(ps.Ops)[0] {
			lo, hi = math.Min(lo, q.X), math.Max(hi, q.X)
		}
		if math.Abs(lo-l) > 1e-6 || math.Abs(hi-r) > 1e-6 {
			t.Fatalf("%v..%v vs %v..%v", lo, hi, l, r)
		}
	}
	// Weeks 1 and 2 are in view: two hover columns.
	if layers != 3 || !strings.Contains(s.Label, "2 times") {
		t.Fatalf("%d layers, %q", layers, s.Label)
	}
}
