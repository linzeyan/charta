package charta

import (
	"slices"
	"testing"

	"github.com/linzeyan/charta/scene"
)

func TestOverlappingSlotsShareTheColumnSideBySide(t *testing.T) {
	// Day 0: 9–11 and 10–12 overlap (two lanes); 11–12 reuses lane 0.
	lane, used := timetableLanes([]placedSlot{{0, 540, 660}, {0, 600, 720}, {0, 660, 720}, {1, 540, 600}}, 2)
	if !slices.Equal(lane, []int{0, 1, 0, 0}) || !slices.Equal(used, []int{2, 1}) {
		t.Fatalf("lanes %v used %v", lane, used)
	}
}

func TestTimetableBlocksSpanTheirMinutes(t *testing.T) {
	s := build(t, TimetableChart{Days: []string{"Mon", "Tue"}, Slots: []Slot{
		{Day: "Mon", Start: 9 * 60, End: 10*60 + 30, Label: "Maths"},
		{Day: "Tue", Start: 13 * 60, End: 14 * 60, Label: "Art"},
	}})
	b := dataRects(s)
	if b[0].Hit.Value != "09:00–10:30" {
		t.Fatalf("value %q", b[0].Hit.Value)
	}
	// 90 minutes against 60: one and a half times as tall (the 2px gap
	// aside).
	if r := (rectOf(b[0]).H + 2) / (rectOf(b[1]).H + 2); !near(r, 1.5) {
		t.Fatalf("height ratio %v", r)
	}
	// Each block sits in its own day's column.
	if !(rectOf(b[1]).X > rectOf(b[0]).X+rectOf(b[0]).W) {
		t.Fatal("Tuesday's block must be right of Monday's")
	}
}

func TestOverlappingBlocksSplitTheirColumn(t *testing.T) {
	s := build(t, TimetableChart{Days: []string{"Mon"}, Slots: []Slot{
		{Day: "Mon", Start: 540, End: 660, Label: "a"},
		{Day: "Mon", Start: 600, End: 720, Label: "b"},
	}})
	b := dataRects(s)
	a, c := rectOf(b[0]), rectOf(b[1])
	if !near(a.W, c.W) || !(c.X >= a.X+a.W) {
		t.Fatalf("overlapping blocks %+v %+v must sit side by side", a, c)
	}
}

func TestUnknownDaysAndInvertedSlotsAreRejected(t *testing.T) {
	rejects(t, TimetableChart{Days: []string{"Mon"}, Slots: []Slot{{Day: "Sun", Start: 0, End: 60, Label: "x"}}})
	rejects(t, TimetableChart{Days: []string{"Mon"}, Slots: []Slot{{Day: "Mon", Start: 60, End: 60, Label: "x"}}})
	rejects(t, TimetableChart{Days: []string{"Mon"}, Slots: []Slot{{Day: "Mon", Start: 60, End: 24*60 + 1, Label: "x"}}})
	rejects(t, TimetableChart{Days: []string{"Mon"}, Slots: []Slot{{Day: "Mon", Start: -30, End: 60, Label: "x"}}})
}

func TestTimetableLabelsOnlyWhereTheyFit(t *testing.T) {
	s := build(t, TimetableChart{Days: []string{"Mon"}, Slots: []Slot{
		{Day: "Mon", Start: 540, End: 600, Label: "Maths"},
		{Day: "Mon", Start: 600, End: 605, Label: "Brief"},
	}})
	var texts []string
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && (tx.Text == "Maths" || tx.Text == "Brief") {
			texts = append(texts, tx.Text)
		}
	}
	// A five-minute block is too short for its label; the tooltip names it.
	if !slices.Equal(texts, []string{"Maths"}) {
		t.Fatalf("labels %v", texts)
	}
}
