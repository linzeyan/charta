package charta

import (
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func plan() GanttChart {
	d := func(n int) int64 { return base.ToMS(2026, 4, n, 0, 0, 0) }
	return GanttChart{Tasks: []Task{
		{Name: "Design", Start: d(1), End: d(8), Group: "Product"},
		{Name: "Build", Start: d(6), End: d(20), Group: "Engineering"},
		{Name: "Launch", Start: d(20), End: d(20), Group: "Product"},
	}}
}

func TestBarLengthIsDurationAndMilestonesAreDiamonds(t *testing.T) {
	var tasks []scene.Mark
	for _, m := range build(t, plan()).Marks {
		if m.Hit != nil && m.Hit.Kind == scene.HitMark {
			tasks = append(tasks, m)
		}
	}
	// Build lasts 14 days, Design 7: twice as long.
	if r := rectOf(tasks[1]).W / rectOf(tasks[0]).W; !near(r, 2) {
		t.Fatalf("Build/Design length ratio %v, want 2", r)
	}
	if _, ok := tasks[2].Shape.(scene.PathShape); !ok {
		t.Fatalf("milestone drawn as %T", tasks[2].Shape)
	}
	h := tasks[1].Hit
	if h.Value != "14 days" || h.Rows[0].Value != "Apr 6, 2026" || h.Rows[1].Value != "Apr 20, 2026" {
		t.Fatalf("Build tooltip %q %+v", h.Value, h.Rows)
	}
	if tasks[2].Hit.Value != "milestone" || len(tasks[2].Hit.Rows) != 1 {
		t.Fatalf("milestone tooltip %+v", tasks[2].Hit)
	}
	// Groups color the bars and name the series for the legend.
	if tasks[1].Fill != base.Series(1) || tasks[2].Series != "Product" {
		t.Fatalf("Build fill %v, Launch series %q", tasks[1].Fill, tasks[2].Series)
	}
	if tasks[0].Hit.Label != "Design · Product" {
		t.Fatalf("label %q", tasks[0].Hit.Label)
	}
}

func TestATaskCannotEndBeforeItStarts(t *testing.T) {
	rejects(t, GanttChart{Tasks: []Task{{Name: "x", Start: 10, End: 5}}})
}

func TestGanttTasksStartWhereTheTimeAxisSaysTheyDo(t *testing.T) {
	// The bar's left edge and the tick of its start date must coincide.
	s := build(t, plan())
	start := rectOf(dataRects(s)[0]).X
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Text == "Apr 1" {
			if !near(tx.X, start) {
				t.Fatalf("Apr 1 tick at %v, Design starts at %v", tx.X, start)
			}
			return
		}
	}
	t.Fatal("no Apr 1 tick")
}

func TestGanttTickLabelsStayInsideThePadding(t *testing.T) {
	// Ending on Apr 21 puts a tick (every 2 days from Apr 1) at the plot's
	// right edge, where its centered label would spill past the padding.
	c := plan()
	c.Tasks[1].End = base.ToMS(2026, 4, 21, 0, 0, 0)
	tickLabelsInside(t, build(t, c), 640)
}
