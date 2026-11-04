package charta

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func TestAFlatTreemapGivesEachLeafAreaInProportion(t *testing.T) {
	s := build(t, TreemapChart{Root: nbranch("r", nleaf("a", 1), nleaf("b", 3), nleaf("c", 6))})
	// The 1px inset on each side is the gap, not data: add it back.
	area := map[string]float64{}
	for _, m := range dataRects(s) {
		r := rectOf(m)
		area[m.Hit.Label] = (r.W + 2) * (r.H + 2)
	}
	if math.Abs(area["b"]/area["a"]-3) >= 1e-6 || math.Abs(area["c"]/area["a"]-6) >= 1e-6 {
		t.Fatalf("%v", area)
	}
	// The cells fill the plot: everything below the legend.
	sum := area["a"] + area["b"] + area["c"]
	if math.Abs(sum-(640-2*pad)*(400-pad-(pad+legendRow+gap))) >= 1e-6 {
		t.Fatalf("cells cover %v", sum)
	}
}

func TestGroupsNestTheirCellsAndColorThemByBranch(t *testing.T) {
	s := build(t, TreemapChart{Root: nbranch("r",
		nbranch("A", nleaf("a1", 2), nleaf("a2", 2)),
		nbranch("B", nleaf("b1", 4)))})
	if m := markNamed(t, s, "A / a1"); m.Fill != base.Wash(0) || m.Stroke.Color != base.Series(0) || m.Series != "A" {
		t.Errorf("a1: %+v", m)
	}
	if m := markNamed(t, s, "B / b1"); m.Fill != base.Wash(1) || m.Series != "B" {
		t.Errorf("b1: %+v", m)
	}
	// The header names each group above its cells, inside the group.
	var header scene.TextShape
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Text == "A" && m.Hit == nil {
			header = tx
		}
	}
	a1, a2 := rectOf(markNamed(t, s, "A / a1")), rectOf(markNamed(t, s, "A / a2"))
	if header.Text != "A" || header.Y >= math.Min(a1.Y, a2.Y) {
		t.Fatalf("header %+v not above cells %+v %+v", header, a1, a2)
	}
	// Cells sit inside their group's inset: the header strip on top.
	if math.Min(a1.Y, a2.Y)-1 != header.Y-treemapHeader/2+treemapHeader {
		t.Errorf("cells start at %v, header strip ends at %v", math.Min(a1.Y, a2.Y)-1, header.Y+treemapHeader/2)
	}
}

func TestZeroLeavesGetNoCell(t *testing.T) {
	s := build(t, TreemapChart{Root: nbranch("r", nleaf("a", 1), nleaf("z", 0))})
	if n := len(dataRects(s)); n != 1 {
		t.Fatalf("%d cells", n)
	}
}

func TestTreemapCellsLabelThemselvesWhenTheyFit(t *testing.T) {
	// A big cell shows its name and value; a sliver shows neither, so no
	// text runs past its edges.
	// One top-level group: no legend, whose entries would repeat the names.
	s := build(t, TreemapChart{Root: nbranch("r", nbranch("g", nleaf("Big", 1000), nleaf("Tiny one", 1)))})
	got := texts(s)
	if !slices.Contains(got, "Big") || !slices.Contains(got, "1,000") {
		t.Errorf("big cell labels %v", got)
	}
	if slices.Contains(got, "Tiny one") || slices.Contains(got, "1") {
		t.Errorf("a sliver got a label: %v", got)
	}
	big := rectOf(markNamed(t, s, "g / Big"))
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Text == "1,000" {
			if tx.X != big.X+treemapLabelPad || tx.Y != big.Y+treemapLabelPad+tickSize+treemapLabelPad {
				t.Errorf("value at %v, %v: not the second line in the cell %+v", tx.X, tx.Y, big)
			}
		}
	}
	h := markNamed(t, s, "g / Big").Hit
	if h.Value != "1,000" || h.Rows[0].Value != "99.9%" {
		t.Errorf("tooltip %+v", h)
	}
}

func TestLongGroupNamesAreEllipsizedToTheirHeader(t *testing.T) {
	long := strings.TrimSpace(strings.Repeat("Wide group name ", 6))
	s := buildAt(t, TreemapChart{Root: nbranch("r",
		nbranch(long, nleaf("x", 1), nleaf("y", 1)),
		nbranch("Other", nleaf("z", 9)))}, 640, 400)
	for _, m := range s.Marks {
		tx, ok := m.Shape.(scene.TextShape)
		// The legend names the group in full; the header is semibold.
		if !ok || tx.Weight != base.Semibold || !strings.HasPrefix(tx.Text, "Wide") {
			continue
		}
		cell := rectOf(markNamed(t, s, long+" / x"))
		w := base.InterMetrics{}.Width(tx.Text, tickSize, base.Semibold)
		if !strings.HasSuffix(tx.Text, "…") || tx.X+w > cell.X+cell.W+treemapInset+1 {
			t.Fatalf("header %q (%vpx) overruns its group", tx.Text, w)
		}
		return
	}
	t.Fatal("no header for the long group")
}

func TestEllipsizeKeepsAsMuchAsFits(t *testing.T) {
	m := base.InterMetrics{}
	got := ellipsize(m, "Networking", m.Width("Netw…", tickSize, base.Semibold))
	if got != "Netw…" {
		t.Errorf("got %q", got)
	}
	// With room for all but the last character, only that one goes.
	if got := ellipsize(m, "Networking", m.Width("Networkin…", tickSize, base.Semibold)); got != "Networkin…" {
		t.Errorf("got %q", got)
	}
	if got := ellipsize(m, "Networking", 1); got != "" {
		t.Errorf("nothing fits, got %q", got)
	}
}

func TestTheLargestCellTakesTheTopLeftCorner(t *testing.T) {
	// Children are laid out largest first, whatever the data order: the
	// biggest cell leads the reading order and squarifying works best.
	s := build(t, TreemapChart{Root: nbranch("r", nbranch("g", nleaf("a", 1), nleaf("b", 3), nleaf("c", 6)))})
	c, a := rectOf(markNamed(t, s, "g / c")), rectOf(markNamed(t, s, "g / a"))
	if c.X > a.X || c.Y > a.Y {
		t.Fatalf("c %+v does not lead a %+v", c, a)
	}
}
