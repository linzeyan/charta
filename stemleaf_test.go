package charta

import (
	"slices"
	"testing"

	"github.com/linzeyan/charta/scene"
)

func texts(s *scene.Scene) []string {
	var out []string
	for _, m := range s.Marks {
		if t, ok := m.Shape.(scene.TextShape); ok {
			out = append(out, t.Text)
		}
	}
	return out
}

func stemRows(s *scene.Scene) []*scene.Hit {
	var out []*scene.Hit
	for _, m := range s.Marks {
		if m.Hit != nil {
			out = append(out, m.Hit)
		}
	}
	return out
}

func TestEveryValueBecomesOneLeafUnderItsStem(t *testing.T) {
	s := build(t, StemLeafChart{Values: []float64{12, 15, 21, 23, 23, 38}})
	r := stemRows(s)
	if len(r) != 3 || r[1].Label != "20–29" || r[1].Value != "3 values" {
		t.Fatalf("rows %+v", r)
	}
	// Stems, then leaves in order, then the key: duplicates keep both
	// leaves, since a stem-and-leaf plot must keep every value readable.
	want := []string{"1", "2", "5", "2", "1", "3", "3", "3", "8", "Key: 1 | 2 = 12"}
	if got := texts(s); !slices.Equal(got, want) {
		t.Fatalf("texts %q, want %q", got, want)
	}
}

func TestEmptyStemsBetweenValuesStillGetARow(t *testing.T) {
	// A gap in the data is information; skipping the row would hide it.
	r := stemRows(build(t, StemLeafChart{Values: []float64{11, 52}}))
	if len(r) != 5 || r[2].Value != "0 values" {
		t.Fatalf("rows %+v", r)
	}
}

func TestAutoUnitPicksTheFinestReadableSplit(t *testing.T) {
	// 0.12–0.89: hundredths as leaves, tenths as stems.
	s := build(t, StemLeafChart{Values: []float64{0.12, 0.29, 0.47, 0.89}})
	if l := stemRows(s)[0].Label; l != "0.1–0.19" {
		t.Fatalf("first row %q", l)
	}
	if !slices.Contains(texts(s), "Key: 1 | 2 = 0.12") {
		t.Fatalf("key missing: %q", texts(s))
	}
	// A coarser explicit unit rounds to it.
	unit := 10.0
	if l := stemRows(build(t, StemLeafChart{Values: []float64{120, 340, 355}, LeafUnit: &unit}))[0].Label; l != "100–190" {
		t.Fatalf("first row %q", l)
	}
}

func TestNegativeValuesAndOddLeafUnitsAreRejected(t *testing.T) {
	rejects(t, StemLeafChart{Values: []float64{-1}})
	half := 0.5
	rejects(t, StemLeafChart{Values: []float64{1}, LeafUnit: &half})
}
