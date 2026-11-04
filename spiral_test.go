package charta

import (
	"fmt"
	"math"
	"testing"

	"github.com/linzeyan/charta/scene"
)

func centroid(ops []scene.PathOp) (float64, float64) {
	pts := scene.Flatten(ops)[0]
	x, y := 0.0, 0.0
	for _, p := range pts {
		x, y = x+p.X, y+p.Y
	}
	n := float64(len(pts))
	return x / n, y / n
}

func spiralData(n int) []Bar {
	out := make([]Bar, n)
	for i := range out {
		out[i] = Bar{Label: fmt.Sprint(i), Value: float64(i)}
	}
	return out
}

func TestTheSamePositionInEachTurnLinesUpAndMovesOutward(t *testing.T) {
	s := buildAt(t, SpiralChart{Period: 4, Data: spiralData(8)}, 600, 600)
	var segs [][2]float64
	for _, m := range s.Marks {
		if p, ok := m.Shape.(scene.PathShape); ok {
			x, y := centroid(p.Ops)
			segs = append(segs, [2]float64{x, y})
		}
	}
	// Position 0 of each turn sits at 45° (half a step past twelve o'clock,
	// clockwise): the second turn's segment lies farther out along that
	// same bearing, up and to the right in equal measure.
	dx, dy := segs[4][0]-segs[0][0], segs[4][1]-segs[0][1]
	if !(dx > 0 && dy < 0) {
		t.Fatalf("%v %v", segs[0], segs[4])
	}
	// Centroids on a widening spiral drift a little off the exact bearing.
	if math.Abs(dx+dy) >= 0.2*dx {
		t.Fatalf("bearing drifts: %v %v", dx, dy)
	}
}

func TestTheOutermostTurnStaysClearOfTheAngleLabels(t *testing.T) {
	names := []string{"N", "E", "S", "W"}
	s := buildAt(t, SpiralChart{Period: 4, AngleLabels: names, Data: spiralData(10)}, 600, 600)
	cx, cy := 0.0, 0.0
	var labels [][2]float64
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok {
			for _, n := range names {
				if tx.Text == n {
					labels = append(labels, [2]float64{tx.X, tx.Y})
					cx, cy = cx+tx.X/4, cy+tx.Y/4
				}
			}
		}
	}
	if len(labels) != 4 {
		t.Fatalf("%d angle labels", len(labels))
	}
	// The labels' ring, less half a label height, bounds the segments.
	ring := math.Hypot(labels[0][0]-cx, labels[0][1]-cy)
	for _, m := range s.Marks {
		if p, ok := m.Shape.(scene.PathShape); ok {
			for _, q := range scene.Flatten(p.Ops)[0] {
				if r := math.Hypot(q.X-cx, q.Y-cy); r >= ring-tickSize/2 {
					t.Fatalf("%v reaches the labels at %v", r, ring)
				}
			}
		}
	}
}

func TestAngleLabelsMustMatchThePeriod(t *testing.T) {
	rejects(t, SpiralChart{Period: 12, AngleLabels: []string{"Jan"}, Data: []Bar{{"a", 1}}})
	rejects(t, SpiralChart{Period: 0, Data: []Bar{{"a", 1}}})
}

func TestSpiralSegmentsAreColoredByValue(t *testing.T) {
	h := hits(buildAt(t, SpiralChart{Period: 4, Data: spiralData(8)}, 600, 600))
	if h[0].Fill == h[7].Fill {
		t.Fatal("lowest and highest values share a ramp class")
	}
	if h[3].Hit.Label != "3" || h[3].Hit.Value != "3" {
		t.Fatalf("tooltip %+v", h[3].Hit)
	}
}
