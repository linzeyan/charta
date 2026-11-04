package charta

import (
	"math"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func orgSample() Node {
	return nbranch("CEO",
		nbranch("CTO", Node{Name: "Dev"}, Node{Name: "Ops"}),
		Node{Name: "CFO"},
		nbranch("COO", Node{Name: "Sales"}))
}

// nodeAt is every node's center by label, whether drawn as a dot or a pill.
func nodeAt(s *scene.Scene) map[string]scene.Point {
	out := map[string]scene.Point{}
	for _, m := range s.Marks {
		if m.Hit == nil || m.Hit.Kind != scene.HitMark {
			continue
		}
		switch sh := m.Shape.(type) {
		case scene.CircleShape:
			out[m.Hit.Label] = scene.Point{X: sh.CX, Y: sh.CY}
		case scene.RectShape:
			out[m.Hit.Label] = scene.Point{X: sh.Rect.X + sh.Rect.W/2, Y: sh.Rect.Y + sh.Rect.H/2}
		}
	}
	return out
}

func TestEachLevelIsAColumnAndParentsSitBesideTheirChildren(t *testing.T) {
	d := nodeAt(build(t, TreeChart{Root: orgSample()}))
	ceo, cto, cfo, dev, ops := d["CEO"], d["CTO"], d["CFO"], d["CTO / Dev"], d["CTO / Ops"]
	if !(ceo.X < cto.X && cto.X < dev.X) {
		t.Errorf("levels not left to right: %v %v %v", ceo, cto, dev)
	}
	if !near(cto.X, cfo.X) {
		t.Error("one column per level")
	}
	if !near(cto.Y, (dev.Y+ops.Y)/2) {
		t.Error("not centered on its children")
	}
	if !near(ceo.Y, (cto.Y+d["COO"].Y)/2) {
		t.Error("the root is not centered on its children")
	}
}

func TestTreeLeavesAreHollowAndLabelledOutward(t *testing.T) {
	s := build(t, TreeChart{Root: orgSample()})
	dev, cto := markNamed(t, s, "CTO / Dev"), markNamed(t, s, "CTO")
	if dev.Fill != base.Surface || dev.Stroke.Color != base.Series(0) || dev.Stroke.Width != treeLink {
		t.Errorf("leaf %+v", dev)
	}
	if cto.Fill != base.Series(0) || cto.Stroke.Color != base.Surface {
		t.Errorf("inner node %+v", cto)
	}
	if h := cto.Hit; h.Rows[0] != (scene.TipRow{Name: "children", Value: "2"}) || h.Area.W != minTarget {
		t.Errorf("tooltip %+v", h)
	}
	devAt, ctoAt := circleOf(dev), circleOf(cto)
	var devLabel, ctoLabel scene.TextShape
	var backing bool
	for _, m := range s.Marks {
		switch sh := m.Shape.(type) {
		case scene.TextShape:
			switch sh.Text {
			case "Dev":
				devLabel = sh
			case "CTO":
				ctoLabel = sh
			}
		case scene.RectShape:
			if m.Fill == base.Surface && sh.Rect.X+sh.Rect.W <= ctoAt.CX && sh.Rect.Y < ctoAt.CY && sh.Rect.Y+sh.Rect.H > ctoAt.CY {
				backing = true
			}
		}
	}
	// Leaves label to the right; inner nodes to the left on a backing, over
	// the link coming in.
	if devLabel.Anchor != scene.Start || devLabel.X != devAt.CX+dotRadius+gap/2 || devLabel.Y != devAt.CY {
		t.Errorf("leaf label %+v", devLabel)
	}
	if ctoLabel.Anchor != scene.End || ctoLabel.X != ctoAt.CX-dotRadius-gap/2 || !backing {
		t.Errorf("inner label %+v, backing %v", ctoLabel, backing)
	}
}

func TestTreeLinksAreCurvesFromParentToChild(t *testing.T) {
	s := build(t, TreeChart{Root: orgSample()})
	d := nodeAt(s)
	n := 0
	for _, m := range s.Marks {
		p, ok := m.Shape.(scene.PathShape)
		if !ok {
			continue
		}
		n++
		from, to := p.Ops[0], p.Ops[1]
		if from.X != d["CEO"].X && from.X != d["CTO"].X && from.X != d["COO"].X {
			t.Errorf("link starts at %v, not a parent", from)
		}
		if to.X1 != (from.X+to.X)/2 || to.Y1 != from.Y || to.Y2 != to.Y {
			t.Errorf("link %v is not a horizontal S-curve", p.Ops)
		}
	}
	if n != 6 {
		t.Fatalf("%d links for 7 nodes", n)
	}
}

func TestMindMapBranchesSpreadAroundTheCenterByDepth(t *testing.T) {
	d := nodeAt(build(t, BrainstormChart{Root: orgSample()}))
	c := d["CEO"]
	r := func(l string) float64 { return math.Hypot(d[l].X-c.X, d[l].Y-c.Y) }
	if math.Abs(r("CTO")-r("CFO")) >= 1e-6 {
		t.Error("one ring per level")
	}
	if math.Abs(r("CTO / Dev")-2*r("CTO")) >= 1e-6 {
		t.Errorf("second ring at %v, first at %v", r("CTO / Dev"), r("CTO"))
	}
}

func TestEachMainBranchGetsItsOwnHueDownToItsLeaves(t *testing.T) {
	s := build(t, BrainstormChart{Root: orgSample()})
	// The branch's pill wears its hue as an outline; its ideas as dots.
	if m := markNamed(t, s, "CTO"); m.Stroke.Color != base.Series(0) || m.Fill != base.Surface {
		t.Errorf("CTO pill %+v", m)
	}
	if m := markNamed(t, s, "CTO / Ops"); m.Fill != base.Series(0) || m.Series != "CTO" {
		t.Errorf("Ops %+v", m)
	}
	if m := markNamed(t, s, "COO / Sales"); m.Fill != base.Series(2) {
		t.Errorf("Sales %+v", m)
	}
	// Links from the center are the heavier trunks, in the branch hue.
	var trunks, twigs int
	for _, m := range s.Marks {
		if _, ok := m.Shape.(scene.PathShape); ok {
			switch m.Stroke.Width {
			case treeTrunk:
				trunks++
			case treeLink:
				twigs++
			}
		}
	}
	if trunks != 3 || twigs != 3 {
		t.Errorf("%d trunks, %d twigs", trunks, twigs)
	}
}

func TestBranchNamesSitInsideTheirPills(t *testing.T) {
	s := build(t, BrainstormChart{Root: orgSample()})
	cto := markNamed(t, s, "CTO")
	r := rectOf(cto)
	if *cto.Hit.Area != r || r.H != tickSize+2*treeBranchPad {
		t.Errorf("pill %+v, hit area %+v", r, cto.Hit.Area)
	}
	c := nodeAt(s)["CTO"]
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Text == "CTO" && tx.Anchor == scene.Middle && near(tx.X, c.X) && near(tx.Y, c.Y) {
			return
		}
	}
	t.Fatal("CTO is not centered in its pill")
}

func TestTheCentralTopicIsTheBiggestPillOverEverything(t *testing.T) {
	s := build(t, BrainstormChart{Root: orgSample()})
	last := s.Marks[len(s.Marks)-1]
	if tx, ok := last.Shape.(scene.TextShape); !ok || tx.Text != "CEO" || tx.Size != titleSize {
		t.Fatalf("last mark %+v", last)
	}
	root := markNamed(t, s, "CEO")
	if root.Stroke.Color != base.Axis || rectOf(root).H != titleSize+2*treeRootPad {
		t.Errorf("root pill %+v", root)
	}
	// Ideas below the first level are dots labelled away from the center.
	ops := circleOf(markNamed(t, s, "CTO / Ops"))
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && tx.Text == "Ops" {
			ceo := nodeAt(s)["CEO"]
			if math.Hypot(tx.X-ceo.X, tx.Y-ceo.Y) <= math.Hypot(ops.CX-ceo.X, ops.CY-ceo.Y) {
				t.Errorf("label %+v is not outside its dot %+v", tx, ops)
			}
		}
	}
	if s.Label != `Mind map of 6 ideas around "CEO".` {
		t.Errorf("%q", s.Label)
	}
}

func TestDeeperMindMapNeighboursNeedLessAngle(t *testing.T) {
	// Siblings one ring out are spaced half the angle of siblings on the
	// first ring: the outer ring is twice as long.
	s := build(t, BrainstormChart{Root: nbranch("root", Node{Name: "A"}, Node{Name: "B"}, nbranch("C", Node{Name: "c1"}, Node{Name: "c2"}))})
	d := nodeAt(s)
	c := d["root"]
	angle := func(l string) float64 { return math.Atan2(d[l].X-c.X, c.Y-d[l].Y) }
	first, second := math.Abs(angle("B")-angle("A")), math.Abs(angle("C / c2")-angle("C / c1"))
	if math.Abs(second/first-0.5) >= 1e-9 {
		t.Fatalf("angles %v on the first ring, %v on the second", first, second)
	}
}
