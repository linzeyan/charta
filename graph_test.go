package charta

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/linzeyan/charta/internal/layout"
	"github.com/linzeyan/charta/scene"
)

func TestGraphNodesFollowFirstMention(t *testing.T) {
	g, err := graph([]Link{{"b", "a", 1}, {"c", "b", 2}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(g.names, []string{"b", "a", "c"}) {
		t.Fatalf("names %v", g.names)
	}
	if !slices.Equal(g.edges, []layout.Flow{{S: 0, T: 1, V: 1}, {S: 2, T: 0, V: 2}}) {
		t.Fatalf("edges %v", g.edges)
	}
	if !slices.Equal(g.strength(), []float64{3, 1, 2}) {
		t.Fatalf("strength %v", g.strength())
	}
}

func TestAReversedPairIsADuplicateOnlyWhenDirectionIsIgnored(t *testing.T) {
	l := []Link{{"a", "b", 1}, {"b", "a", 1}}
	if _, err := graph(l, true); err != nil {
		t.Fatalf("directed: %v", err)
	}
	if _, err := graph(l, false); err == nil {
		t.Fatal("undirected: a → b and b → a would draw over each other")
	}
	if _, err := graph([]Link{{"a", "b", 1}, {"a", "b", 2}}, true); err == nil {
		t.Fatal("the same directed link twice must be refused")
	}
}

func TestBadLinksAreRejected(t *testing.T) {
	for _, l := range []Link{{"a", "b", 0}, {"a", "b", -1}, {"a", "b", math.NaN()}, {"a", "b", math.Inf(1)}, {"a", "a", 1}} {
		if _, err := graph([]Link{l}, true); err == nil {
			t.Errorf("%+v accepted", l)
		}
	}
}

func TestANinthNodeTurnsColorsOff(t *testing.T) {
	var eight []Link
	for i := 1; i < 8; i++ {
		eight = append(eight, Link{"hub", fmt.Sprintf("n%d", i), 1})
	}
	g, _ := graph(eight, false)
	if !g.colored() || g.color(7) != 7 || len(g.legend()) != 8 || g.series(7) != "n7" {
		t.Fatal("eight nodes each get a palette slot and a legend entry")
	}
	g, _ = graph(append(eight, Link{"hub", "n8", 1}), false)
	if g.colored() || g.color(8) != 0 || len(g.legend()) != 0 || g.series(8) != "" {
		t.Fatal("a ninth node must fold every node into one hue, not a generated ninth")
	}
}

func TestQuadTracesTheQuadraticExactly(t *testing.T) {
	// The cubic's control points sit 2/3 of the way to the quadratic's: its
	// midpoint is then the quadratic's, (from + 2c + to) / 4.
	op := quad(nil, 0, 0, 6, 9, 12, 0)[0]
	mx := (0 + 3*op.X1 + 3*op.X2 + op.X) / 8
	my := (0 + 3*op.Y1 + 3*op.Y2 + op.Y) / 8
	if !near(mx, 6) || !near(my, 4.5) || op.X != 12 || op.Y != 0 {
		t.Fatalf("%+v", op)
	}
}

func TestAFlowBandHasItsThicknessAtBothEnds(t *testing.T) {
	ops := flowBand(10, 20, 110, 60, 5)
	if ops[0] != scene.Move(10, 20) || ops[2] != scene.Line(110, 65) || ops[3].Y != 25 || ops[4].Verb != scene.ClosePath {
		t.Fatalf("%+v", ops)
	}
	// Both curves leave and arrive horizontally: the controls share the
	// ends' y.
	if ops[1].Y1 != 20 || ops[1].Y2 != 60 || ops[3].Y1 != 65 || ops[3].Y2 != 25 {
		t.Fatalf("%+v", ops)
	}
}
