package scene

import (
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
)

func rect(x, w float64) Shape { return RectShape{Rect: Rect{X: x, W: w, H: 10}} }

func bar(x float64, key string) Mark {
	return Mark{Shape: rect(x, 10), Fill: base.Series(0), Hit: &Hit{Key: key, Label: key, Value: "1"}}
}

func withSeries(m Mark, s string) Mark {
	m.Series = s
	return m
}

func legendMark(x float64, series string) Mark {
	return Mark{Shape: rect(x, 10), Series: series, Hit: &Hit{Key: "leg", Kind: HitLegend, Label: series}}
}

func sc(marks ...Mark) *Scene { return &Scene{Width: 100, Height: 100, Marks: marks} }

func wantHit(t *testing.T, s *Scene, x, y float64, hidden []string, want int) {
	t.Helper()
	i, ok := s.HitTest(x, y, hidden)
	if want < 0 {
		if ok {
			t.Fatalf("HitTest(%v, %v) = %d, want none", x, y, i)
		}
		return
	}
	if !ok || i != want {
		t.Fatalf("HitTest(%v, %v) = %d %v, want %d", x, y, i, ok, want)
	}
}

func TestHitTestFindsTheMarkUnderThePointer(t *testing.T) {
	s := sc(bar(0, "a"), bar(20, "b"))
	wantHit(t, s, 25, 5, nil, 1)
	wantHit(t, s, 15, 5, nil, -1)
}

func TestTopmostMarkWinsAndDecorationIsIgnored(t *testing.T) {
	// A gridline or label drawn over a bar must not steal its hover.
	s := sc(bar(0, "under"), bar(0, "over"), Mark{Shape: rect(0, 10)})
	wantHit(t, s, 5, 5, nil, 1)
}

func TestHitAreaExtendsBeyondThePaintedShape(t *testing.T) {
	// A short bar is still easy to hover: its area covers the whole band.
	m := bar(0, "a")
	m.Hit.Area = &Rect{W: 10, H: 100}
	wantHit(t, sc(m), 5, 80, nil, 0)
}

func TestThinLinesGetATolerance(t *testing.T) {
	s := sc(Mark{
		Shape:  PolylineShape{Points: []Point{{0, 50}, {50, 50}, {100, 50}}},
		Stroke: &Stroke{Color: base.Series(0), Width: 2},
		Hit:    &Hit{Key: "l"},
	})
	wantHit(t, s, 70, 53.5, nil, 0)
	wantHit(t, s, 70, 55, nil, -1)
}

func TestAreaAndCircleHitInsideOnly(t *testing.T) {
	s := sc(
		Mark{Shape: AreaShape{Points: []AreaPoint{{0, 10, 50}, {40, 30, 50}}}, Hit: &Hit{Key: "a"}},
		Mark{Shape: CircleShape{CX: 80, CY: 80, R: 5}, Hit: &Hit{Key: "c"}},
	)
	// At x=20 the top edge is at y=20.
	wantHit(t, s, 20, 25, nil, 0)
	wantHit(t, s, 20, 15, nil, -1)
	wantHit(t, s, 83, 83, nil, 1)
	wantHit(t, s, 86, 86, nil, -1)
}

func TestHiddenSeriesCannotBeHoveredButTheirLegendCan(t *testing.T) {
	hidden := []string{"A"}
	// The legend entry must stay clickable, or a hidden series could never
	// be shown again.
	wantHit(t, sc(withSeries(bar(0, "a"), "A"), legendMark(0, "A")), 5, 5, hidden, 1)
	wantHit(t, sc(withSeries(bar(0, "a"), "A")), 5, 5, hidden, -1)
}

func TestEmphasisDimsOtherSeriesOnlyWhenThereAreSeveral(t *testing.T) {
	two := sc(withSeries(bar(0, "a"), "A"), withSeries(bar(20, "b"), "B"))
	for _, c := range []struct {
		i, hovered int
		want       Emphasis
	}{{0, 0, Lift}, {1, 0, Dim}, {1, -1, Normal}} {
		if got := two.Emphasis(c.i, c.hovered, nil); got != c.want {
			t.Errorf("Emphasis(%d, %d) = %v, want %v", c.i, c.hovered, got, c.want)
		}
	}
	// One series: nothing to single out, so nothing dims.
	one := sc(withSeries(bar(0, "a"), "A"), withSeries(bar(20, "b"), "A"))
	if got := one.Emphasis(1, 0, nil); got != Normal {
		t.Errorf("single series dimmed: %v", got)
	}
}

func TestCrosshairShowsOnlyWhileHoveredAndDimsNothing(t *testing.T) {
	s := sc(
		withSeries(bar(0, "a"), "A"),
		withSeries(bar(20, "b"), "B"),
		Mark{Shape: LineShape{X1: 5, X2: 5, Y2: 100}, Hit: &Hit{Key: "x", Kind: HitCrosshair, Label: "Jan"}},
	)
	if s.Emphasis(2, -1, nil) != Hidden || s.Emphasis(2, 2, nil) != Normal || s.Emphasis(1, 2, nil) != Normal {
		t.Fatal("crosshair must appear only while hovered and leave the series alone")
	}
}

func TestHiddenSeriesVanishAndTheirLegendEntryGreysOut(t *testing.T) {
	s := sc(withSeries(bar(0, "a"), "A"), legendMark(50, "A"))
	hidden := []string{"A"}
	if s.Emphasis(0, -1, hidden) != Hidden || s.Emphasis(1, -1, hidden) != Dim {
		t.Fatal("hidden series must not paint; its legend entry must dim")
	}
}

func TestHoveringAHiddenSeriesLegendEntryDimsNothing(t *testing.T) {
	// Right after a click hides a series the pointer still rests on its
	// legend entry; dimming the survivors then would grey the whole plot out
	// with nothing highlighted.
	s := sc(withSeries(bar(0, "a"), "A"), withSeries(bar(20, "b"), "B"), legendMark(50, "A"))
	if got := s.Emphasis(1, 2, nil); got != Dim {
		t.Fatalf("visible series legend hover: %v, want Dim", got)
	}
	if got := s.Emphasis(1, 2, []string{"A"}); got != Normal {
		t.Fatalf("hidden series legend hover: %v, want Normal", got)
	}
}

func TestFilledPathsHitInsideByWindingAndOpenOnesNearTheStroke(t *testing.T) {
	// A concave "U": the notch is outside even though it is inside the
	// bounding box, which a convex fill would get wrong.
	u := []PathOp{
		Move(0, 0), Line(30, 0), Line(30, 30), Line(20, 30),
		Line(20, 10), Line(10, 10), Line(10, 30), Line(0, 30), Close(),
	}
	filled := sc(Mark{Shape: PathShape{Ops: u}, Fill: base.Series(0), Hit: &Hit{Key: "u", Label: "u", Value: "1"}})
	wantHit(t, filled, 5, 20, nil, 0)
	wantHit(t, filled, 15, 20, nil, -1)
	// Without a fill only the outline is a target: a violin drawn as a bare
	// stroke must not swallow hovers meant for what shows through it.
	open := sc(Mark{Shape: PathShape{Ops: u}, Stroke: &Stroke{Color: base.Series(0), Width: 2}, Hit: &Hit{Key: "u", Label: "u", Value: "1"}})
	wantHit(t, open, 5, 20, nil, -1)
	wantHit(t, open, 1, 20, nil, 0)
}

func TestCubicsFlattenThroughTheirEndPoints(t *testing.T) {
	ops := []PathOp{Move(0, 0), Cubic(0, 10, 10, 10, 10, 0), Close()}
	subs := Flatten(ops)
	if len(subs) != 1 || len(subs[0]) != CubicSteps+2 {
		t.Fatalf("subpaths %v", subs)
	}
	// The curve must end exactly where the SVG path says, or hit testing and
	// native fills drift from the HTML rendering.
	if subs[0][CubicSteps] != (Point{10, 0}) {
		t.Fatalf("cubic ends at %v", subs[0][CubicSteps])
	}
	if d := PathData(ops); d != "M0 0C0 10 10 10 10 0Z" {
		t.Fatalf("path data %q", d)
	}
}

func TestDumpEscapesStringsAndRoundsCoordinates(t *testing.T) {
	s := sc(bar(1.0/3, `a"b`))
	s.Label = "x\ny"
	d := s.Dump()
	for _, want := range []string{`"label":"x\ny"`, `"rect":[0.33,0,10,10]`, `"key":"a\"b"`} {
		if !strings.Contains(d, want) {
			t.Errorf("dump lacks %s:\n%s", want, d)
		}
	}
}

func TestDumpWritesSeriesKindAndRows(t *testing.T) {
	s := sc(Mark{
		Shape:  LineShape{Y2: 1},
		Series: "A",
		Hit: &Hit{Key: "x0", Kind: HitCrosshair, Label: "Jan",
			Rows: []TipRow{{Color: base.Series(0), Name: "A", Value: "3"}, {Name: "median", Value: "2"}}},
	})
	// A row without a key has no color at all, not an empty token.
	want := `"series":"A","hit":{"key":"x0","kind":"crosshair","label":"Jan","value":"","rows":[{"color":"series-1","name":"A","value":"3"},{"name":"median","value":"2"}]}`
	if d := s.Dump(); !strings.Contains(d, want) {
		t.Fatalf("dump lacks %s:\n%s", want, d)
	}
}
