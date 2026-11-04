package charta

import (
	"testing"

	"github.com/linzeyan/charta/scene"
)

func TestWhiskerSpansLowToHighThroughTheEstimate(t *testing.T) {
	s := build(t, ErrorBarChart{Data: []ErrorBar{{"A", 10, 8, 13}, {"B", 20, 19, 22}}})
	var dot scene.Mark
	for _, m := range s.Marks {
		if m.Hit != nil {
			dot = m
			break
		}
	}
	cy := dot.Shape.(scene.CircleShape).CY
	found := false
	for _, m := range s.Marks {
		if l, ok := m.Shape.(scene.LineShape); ok && l.X1 == l.X2 && m.Stroke.Width == lineWidth {
			if !(l.Y1 < cy && cy < l.Y2) {
				t.Fatalf("whisker %v..%v misses the estimate at %v", l.Y1, l.Y2, cy)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no whisker")
	}
	if dot.Hit.Label != "A · 8 – 13" {
		t.Fatal(dot.Hit.Label)
	}
}

func TestInvertedErrorRangesAreRejected(t *testing.T) {
	rejects(t, ErrorBarChart{Data: []ErrorBar{{"A", 10, 12, 13}}})
}
