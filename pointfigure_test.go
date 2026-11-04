package charta

import (
	"slices"
	"strings"
	"testing"

	"github.com/linzeyan/charta/scene"
)

func TestColumnsExtendWithTheTrendAndTurnOnAFullReversal(t *testing.T) {
	// Box 1, reversal 3: up to 15, a 2-box dip (ignored), on to 17, then
	// down to 12 (a reversal) and back to 13 (not one).
	c := pfColumns([]float64{10, 15, 13, 17, 12, 13}, 1, 3)
	want := []pfColumn{{rising: true, low: 10, high: 16, first: 1}, {rising: false, low: 12, high: 15, first: 4}}
	if !slices.Equal(c, want) {
		t.Fatalf("columns %+v, want %+v", c, want)
	}
}

func TestBoxEdgesCountDespiteFloatDivision(t *testing.T) {
	// 0.3 / 0.1 is 2.9999999999999996; the price still fills box 2.
	if c := pfColumns([]float64{0, 0.3}, 0.1, 3); len(c) != 1 || c[0].high != 2 {
		t.Fatalf("%+v", c)
	}
	// And the edge prints as 0.3, not 0.30000000000000004.
	if p := pfBoxPrice(0.1, 3); p != 0.3 {
		t.Fatalf("box 3 of 0.1 at %v", p)
	}
}

func TestRisingColumnsAreXsAndFallingOnesOs(t *testing.T) {
	one := 1.0
	s := build(t, PointFigureChart{Data: daily(10, 15, 11), BoxSize: &one})
	xs, os := 0, 0
	var values []string
	for _, m := range s.Marks {
		switch m.Shape.(type) {
		case scene.PathShape:
			if m.Hit == nil {
				xs++
			}
		case scene.CircleShape:
			os++
		}
		if m.Hit != nil {
			values = append(values, m.Hit.Value)
		}
	}
	// 10 → 15 fills boxes 10..14; the O column runs from 13 down to 11.
	if xs != 5 || os != 3 {
		t.Fatalf("%d X and %d O, want 5 and 3", xs, os)
	}
	if !slices.Equal(values, []string{"10–15", "11–14"}) {
		t.Fatalf("column ranges %v", values)
	}
}

func TestTheAutomaticBoxSizeKeepsEveryCellLegible(t *testing.T) {
	// A wide price range with many small swings: the finest nice box would
	// fill the plot with specks.
	prices := make([]float64, 200)
	for i := range prices {
		prices[i] = float64(i) * 0.5
		if i%2 == 1 {
			prices[i] += 12
		}
	}
	cols := hits(build(t, PointFigureChart{Data: daily(prices...)}))
	if len(cols) == 0 {
		t.Fatal("no columns")
	}
	for _, m := range cols {
		if w := rectOf(m).W; w < pfCellMin {
			t.Fatalf("cell %v", w)
		}
	}
}

func TestAGridTooNarrowForBothDatesShowsOnlyTheFirst(t *testing.T) {
	// One rising column: the grid is a single cell wide.
	one := 1.0
	s := build(t, PointFigureChart{Data: daily(10, 40), BoxSize: &one})
	var dates []string
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && strings.Contains(tx.Text, "1970") {
			dates = append(dates, tx.Text)
		}
	}
	if !slices.Equal(dates, []string{"Jan 1, 1970"}) {
		t.Fatalf("dates %v", dates)
	}
}

func TestPointFigureRejectsBadBoxesAndReversals(t *testing.T) {
	zero, neg := 0, -2.0
	rejects(t, PointFigureChart{Data: daily(1, 2), Reversal: &zero})
	rejects(t, PointFigureChart{Data: daily(1, 2), BoxSize: &neg})
}
