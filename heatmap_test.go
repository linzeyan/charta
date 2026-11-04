package charta

import (
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func TestSeqClassesSpanTheRampFromLowToHigh(t *testing.T) {
	for _, c := range []struct {
		v, lo, hi float64
		want      int
	}{{0, 0, 70, 0}, {70, 0, 70, base.SeqClasses - 1}, {35, 0, 70, 3}, {5, 5, 5, 3}} {
		if got := seqClass(c.v, c.lo, c.hi); got != c.want {
			t.Errorf("seqClass(%v, %v, %v) = %d, want %d", c.v, c.lo, c.hi, got, c.want)
		}
	}
}

func TestHeatmapCellsAreSeparatedAndHoverCoversTheGap(t *testing.T) {
	s := build(t, HeatmapChart{X: []string{"a", "b"}, Y: []string{"r"}, Values: [][]float64{{1, 2}}})
	var cells []scene.Mark
	for _, m := range s.Marks {
		if m.Hit != nil {
			cells = append(cells, m)
		}
	}
	a, b := rectOf(cells[0]), rectOf(cells[1])
	if !near(b.X-(a.X+a.W), surfaceGap) {
		t.Fatal("touching cells need the surface gap")
	}
	// The two hover targets touch: no dead strip between cells.
	if !near(cells[0].Hit.Area.X+cells[0].Hit.Area.W, cells[1].Hit.Area.X) {
		t.Fatal("dead strip between hover targets")
	}
}

func TestRaggedHeatmapRowsAreRejected(t *testing.T) {
	rejects(t, HeatmapChart{X: []string{"a", "b"}, Y: []string{"r"}, Values: [][]float64{{1}}})
}
