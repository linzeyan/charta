package charta

import (
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func TestStaticKeyEntriesSitSideBySideWithoutHits(t *testing.T) {
	var marks []scene.Mark
	m := base.InterMetrics{}
	below := staticKey(&marks, m, 0, []keyItem{
		{keySample{kind: sampleHollow, color: base.Series(0)}, "Rising"},
		{keySample{kind: sampleLine, color: base.Series(1), width: 3}, "Falling"},
	})
	if len(marks) != 4 || below != legendRow+gap {
		t.Fatalf("%d marks, content below at %v", len(marks), below)
	}
	// The key explains an encoding: nothing to toggle, nothing to hover.
	for _, mk := range marks {
		if mk.Hit != nil || mk.Series != "" {
			t.Fatalf("key mark %+v is interactive", mk)
		}
	}
	// The second sample starts after the first entry's name, not on it.
	first := marks[1].Shape.(scene.TextShape)
	second := marks[2].Shape.(scene.RectShape).Rect
	if end := first.X + m.Width("Rising", legendSize, base.Regular); !(second.X >= end+legendItemGap-1e-9) {
		t.Fatalf("second sample at %v overlaps the first name ending at %v", second.X, end)
	}
	// A line sample is as thick as the line it stands for.
	if second.H != 3 {
		t.Fatalf("line sample %v thick, want 3", second.H)
	}
	if _, ok := marks[0].Shape.(scene.PathShape); !ok || marks[0].Fill != base.Surface {
		t.Fatalf("hollow sample %+v", marks[0])
	}
}
