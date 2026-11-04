package charta

import (
	"slices"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func TestTimelineLabelsAlternateAndClimbOnlyWhenTheyWouldCollide(t *testing.T) {
	// Three close labels: above, below, then above again one level up. A
	// far one drops back to the first level.
	l := timelineLevels([][2]float64{{0, 50}, {10, 60}, {20, 70}, {200, 250}}, 3)
	if !slices.Equal(l, []int{1, -1, 2, 1}) {
		t.Fatalf("levels %v", l)
	}
}

func TestWhenEveryLevelIsTakenTheLeastCrowdedOneIsReused(t *testing.T) {
	l := timelineLevels([][2]float64{{0, 50}, {10, 60}, {20, 30}}, 1)
	if !slices.Equal(l, []int{1, -1, 1}) {
		t.Fatalf("levels %v", l)
	}
}

func TestEventsSitOnTheAxisInTimeOrder(t *testing.T) {
	s := build(t, TimelineChart{Events: []Event{{3 * base.Day, "C"}, {0, "A"}, {base.Day, "B"}}})
	var xs []float64
	var names []string
	for _, m := range s.Marks {
		if c, ok := m.Shape.(scene.CircleShape); ok && m.Hit != nil {
			xs, names = append(xs, c.CX), append(names, m.Hit.Value)
		}
	}
	if !slices.Equal(names, []string{"A", "B", "C"}) {
		t.Fatalf("order %v", names)
	}
	// Spacing follows time: C is twice as far from B as B from A.
	if r := (xs[2] - xs[1]) / (xs[1] - xs[0]); !near(r, 2) {
		t.Fatalf("spacing ratio %v", r)
	}
}

func TestTimelineLabelsNearTheRightEdgeEndAtTheirDot(t *testing.T) {
	// The last event sits near the right edge; a label running right from it
	// would leave the chart.
	s := build(t, TimelineChart{Events: []Event{{0, "Start"}, {base.Day, "A rather long final event name"}}})
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Text == "A rather long final event name" {
			if tx.Anchor != scene.End || tx.X > 640-pad {
				t.Fatalf("label %+v spills past the edge", tx)
			}
			return
		}
	}
	t.Fatal("label missing")
}
