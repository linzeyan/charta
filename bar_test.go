package charta

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/html"
	"github.com/linzeyan/charta/scene"
)

type bar struct {
	r scene.Rect
	c scene.Corners
	h *scene.Hit
}

func bars(t *testing.T, c BarChart) (*scene.Scene, []bar) {
	t.Helper()
	s, err := c.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		t.Fatal(err)
	}
	var out []bar
	for _, m := range s.Marks {
		if r, ok := m.Shape.(scene.RectShape); ok && m.Hit != nil {
			out = append(out, bar{r.Rect, r.Radius, m.Hit})
		}
	}
	return s, out
}

func TestBarHeightIsProportionalToValue(t *testing.T) {
	// The whole point of a bar chart: length encodes magnitude.
	_, b := bars(t, BarChart{Data: []Bar{{"a", 10}, {"b", 20}, {"c", 35}}})
	perUnit := b[0].r.H / 10
	if math.Abs(b[1].r.H-20*perUnit) > 1e-9 || math.Abs(b[2].r.H-35*perUnit) > 1e-9 {
		t.Fatal(b)
	}
}

func TestNegativeBarsHangBelowTheBaselineWithTheirOwnRoundedEnd(t *testing.T) {
	_, b := bars(t, BarChart{Data: []Bar{{"up", 5}, {"down", -3}}})
	if math.Abs(b[1].r.Y-(b[0].r.Y+b[0].r.H)) > 1e-9 {
		t.Fatal("negative bar must start at the baseline")
	}
	if !(b[0].c.TL > 0 && b[0].c.BL == 0 && b[1].c.BL > 0 && b[1].c.TL == 0) {
		t.Fatalf("rounded end must be the data end: %+v %+v", b[0].c, b[1].c)
	}
}

func TestBarsAreCappedAndHoverableAcrossTheBand(t *testing.T) {
	s, b := bars(t, BarChart{Data: []Bar{{"a", 100}, {"b", 1}}})
	if b[0].r.W > barMax {
		t.Fatal("bar thicker than the cap")
	}
	i, ok := s.HitTest(b[1].r.X+b[1].r.W/2, 60, nil)
	if !ok || s.Marks[i].Hit.Label != "b" {
		t.Fatal("hovering high above a short bar must still hit it")
	}
}

func TestCrowdedLabelsRotate(t *testing.T) {
	var data []Bar
	for i := range 30 {
		data = append(data, Bar{fmt.Sprintf("Category %d", i), float64(i)})
	}
	s, _ := bars(t, BarChart{Data: data})
	rotated := 0
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Angle == -45 {
			rotated++
		}
	}
	if rotated != 30 {
		t.Fatalf("rotated %d labels", rotated)
	}
}

func TestTooltipCarriesLabelAndFormattedValue(t *testing.T) {
	_, b := bars(t, BarChart{Data: []Bar{{"Revenue", 1234567}}})
	if b[0].h.Label != "Revenue" || b[0].h.Value != "1,234,567" {
		t.Fatalf("%+v", b[0].h)
	}
}

func TestEmptyDataStillDrawsAxes(t *testing.T) {
	s, b := bars(t, BarChart{})
	if len(b) != 0 {
		t.Fatal("no data, no bars")
	}
	for _, m := range s.Marks {
		if _, ok := m.Shape.(scene.LineShape); ok {
			return
		}
	}
	t.Fatal("an empty chart must still show its frame")
}

func TestInvalidInputIsRejected(t *testing.T) {
	_, err := BarChart{Data: []Bar{{"ok", 1}, {"bad", math.NaN()}}}.Scene(640, 400, base.InterMetrics{})
	var de *InvalidDataError
	if !errors.As(err, &de) || !strings.Contains(de.Msg, "bad") {
		t.Fatalf("got %v", err)
	}
	if _, err := (BarChart{}).Scene(0, 10, base.InterMetrics{}); !errors.Is(err, ErrInvalidSize) {
		t.Fatalf("got %v", err)
	}
}

func TestHTMLEscapesLabelsAndShipsRuntime(t *testing.T) {
	// Category names come from user data; a label must never inject markup.
	page, err := RenderHTML(BarChart{Data: []Bar{{"<img src=x onerror=alert(1)>", 1}}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(page, "<img") || strings.Count(page, "<script>") != 1 {
		t.Fatal(page)
	}
	svg, _ := RenderHTML(BarChart{Data: []Bar{{"A", 30}}}, Options{Format: html.FormatSVG})
	if strings.Contains(svg, "<script") || !strings.Contains(svg, "<title>A: 30</title>") {
		t.Fatal(svg)
	}
}
