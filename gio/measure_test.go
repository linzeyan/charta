package gio

import (
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/linzeyan/charta/base"
)

// Layout (axis gutter, label rotation, tooltip width) trusts these widths, so
// they must track the real text: longer strings measure wider, and a Latin
// label lands near the Inter table the HTML backend uses.
func TestMeasureTracksText(t *testing.T) {
	m := Measure{Metric: unit.Metric{PxPerDp: 2, PxPerSp: 2}, Shaper: text.NewShaper(text.WithCollection(gofont.Collection()))}

	short, long := m.Width("18", 12, base.Regular), m.Width("Pears", 12, base.Regular)
	if long <= short {
		t.Fatalf("Width(Pears)=%.2f not wider than Width(18)=%.2f", long, short)
	}
	inter := base.InterMetrics{}.Width("Cherries", 12, base.Regular)
	if got := m.Width("Cherries", 12, base.Regular); got < inter*0.75 || got > inter*1.25 {
		t.Errorf("Width(Cherries)=%.2f dp, Inter says %.2f dp", got, inter)
	}
}
