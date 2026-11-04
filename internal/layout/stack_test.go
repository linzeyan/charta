package layout

import (
	"math"
	"slices"
	"testing"
)

func TestLayersTouchAndKeepTheirThickness(t *testing.T) {
	// A gap between layers, or a layer thicker than its value, would make
	// the stream lie about its totals.
	s := [][]float64{{1, 2, 3}, {4, 1, 2}, {2, 2, 5}}
	order := InsideOut(s)
	e := Wiggle(s, order)
	for j := range 3 {
		for k := 1; k < len(order); k++ {
			if e[order[k-1]][j].Hi != e[order[k]][j].Lo {
				t.Errorf("gap between layers at %d", j)
			}
		}
		for i, v := range s {
			if math.Abs(e[i][j].Hi-e[i][j].Lo-v[j]) > 1e-12 {
				t.Errorf("series %d at %d is %v thick, want %v", i, j, e[i][j].Hi-e[i][j].Lo, v[j])
			}
		}
	}
}

func TestAGrowingLayerDoesNotTiltTheWholeStream(t *testing.T) {
	// One steady series and one that grows: wiggle keeps the steady one
	// flatter than a zero baseline would (where it rides up the growth).
	s := [][]float64{{5, 5, 5, 5}, {1, 3, 6, 10}}
	e := Wiggle(s, []int{1, 0})
	mid := func(j int) float64 { return (e[0][j].Lo + e[0][j].Hi) / 2 }
	// On a zero baseline its midline would climb from 3.5 to 12.5.
	if d := math.Abs(mid(3) - mid(0)); d >= 9 {
		t.Fatalf("drift %v", d)
	}
	// d3's wiggle at step 1: the growth of 2 under the steady layer moves
	// the baseline by −(3·1 + 5·2) / (3 + 5) = −13/8, each layer's slope
	// weighted by its value.
	if e[1][1].Lo != -1.625 {
		t.Fatalf("baseline at step 1 %v, want -1.625", e[1][1].Lo)
	}
}

func TestInsideOutPutsEarlyPeaksInTheMiddle(t *testing.T) {
	s := [][]float64{{0, 0, 9}, {9, 0, 0}, {0, 9, 0}}
	// Peaks: series 1 first, then 2, then 0. Series 1 goes to the bottom
	// list first, 2 on top of it, 0 below.
	if got := InsideOut(s); !slices.Equal(got, []int{0, 1, 2}) {
		t.Fatalf("order %v", got)
	}
	// Equal peaks keep their input order (a stable sort), so the layer order
	// does not depend on the sort implementation.
	if got := InsideOut([][]float64{{1, 0}, {2, 0}, {3, 0}}); !slices.Equal(got, []int{2, 0, 1}) {
		t.Fatalf("tied order %v", got)
	}
}
