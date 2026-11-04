package charta

import (
	"fmt"
	"math"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// markKeyed is the data mark whose hit key is key.
func markKeyed(t *testing.T, s *scene.Scene, key string) scene.Mark {
	t.Helper()
	for _, m := range s.Marks {
		if m.Hit != nil && m.Hit.Kind == scene.HitMark && m.Hit.Key == key {
			return m
		}
	}
	t.Fatalf("no mark keyed %q", key)
	return scene.Mark{}
}

type nodeDot struct {
	scene.CircleShape
	name string
}

func nodeDots(s *scene.Scene) []nodeDot {
	var out []nodeDot
	for _, m := range s.Marks {
		if c, ok := m.Shape.(scene.CircleShape); ok && m.Hit != nil && m.Hit.Kind == scene.HitMark {
			out = append(out, nodeDot{c, m.Hit.Label})
		}
	}
	return out
}

func club() NetworkChart {
	var c NetworkChart
	for _, p := range [][2]string{{"Ann", "Bo"}, {"Ann", "Cy"}, {"Bo", "Cy"}, {"Cy", "Di"}, {"Di", "Ed"}, {"Di", "Flo"}, {"Ed", "Flo"}} {
		c.Links = append(c.Links, Link{p[0], p[1], 1})
	}
	return c
}

func TestEveryNetworkDotAndLabelStaysInsideTheChart(t *testing.T) {
	s := buildAt(t, club(), 480, 320)
	for _, d := range nodeDots(s) {
		if d.CX-d.R < pad-1e-6 || d.CY+d.R > 320-pad+1e-6 {
			t.Errorf("%s pokes out", d.name)
		}
		w := base.InterMetrics{}.Width(d.name, tickSize, base.Regular)
		if d.CX+d.R+gap/2+w > 480-pad+1e-6 {
			t.Errorf("%s's label runs off the right", d.name)
		}
	}
}

func TestLongNetworkLabelsStillFitOnTheRight(t *testing.T) {
	// Room for the longest label is held back on the right, so a crowded
	// plot shrinks the layout instead of pushing names off the chart.
	var c NetworkChart
	for _, l := range club().Links {
		c.Links = append(c.Links, Link{l.Source + " Longname", l.Target + " Longname", 1})
	}
	s := buildAt(t, c, 300, 400)
	for _, d := range nodeDots(s) {
		w := base.InterMetrics{}.Width(d.name, tickSize, base.Regular)
		if d.CX+d.R+gap/2+w > 300-pad+1e-6 {
			t.Errorf("%s's label runs off the right", d.name)
		}
	}
}

func TestTheBestConnectedNodeIsTheBiggest(t *testing.T) {
	d := nodeDots(build(t, club()))
	r := func(n string) float64 {
		for _, x := range d {
			if x.name == n {
				return x.R
			}
		}
		t.Fatalf("no %s", n)
		return 0
	}
	// Cy and Di have three links each; Ann has two.
	if r("Cy") != networkMaxRadius || r("Di") != networkMaxRadius {
		t.Fatalf("Cy %v, Di %v; want %v", r("Cy"), r("Di"), networkMaxRadius)
	}
	// Radius grows with the square root of strength, so the dot's area
	// (what the eye compares) is proportional to it.
	if !near(r("Ann"), dotRadius+(networkMaxRadius-dotRadius)*math.Sqrt(2.0/3)) {
		t.Fatalf("Ann %v", r("Ann"))
	}
	if h := markNamed(t, build(t, club()), "Cy").Hit; h.Value != "3" || h.Rows[0].Value != "3" {
		t.Fatalf("Cy's tooltip %+v", h)
	}
}

func TestAPairIsNotStretchedAcrossTheWholeChart(t *testing.T) {
	d := nodeDots(build(t, NetworkChart{Links: []Link{{"a", "b", 1}}}))
	if gap := math.Hypot(d[0].CX-d[1].CX, d[0].CY-d[1].CY); gap >= 200 {
		t.Fatalf("%v apart", gap)
	}
}

func TestBigNetworksDropLabels(t *testing.T) {
	var c NetworkChart
	for i := range networkMaxLabels {
		c.Links = append(c.Links, Link{"hub", fmt.Sprintf("n%d", i), 1})
	}
	for _, mk := range build(t, c).Marks {
		if _, ok := mk.Shape.(scene.TextShape); ok {
			t.Fatal("31 nodes: labels would pile up")
		}
	}
	// One fewer and every node is named.
	c.Links = c.Links[:networkMaxLabels-1]
	if n := len(texts(build(t, c))); n != networkMaxLabels {
		t.Fatalf("%d labels for %d nodes", n, networkMaxLabels)
	}
}

func TestHeavierNetworkLinksDrawThicker(t *testing.T) {
	s := build(t, NetworkChart{Links: []Link{{"a", "b", 1}, {"b", "c", 4}}})
	thin, thick := markKeyed(t, s, "link-0").Stroke, markKeyed(t, s, "link-1").Stroke
	if thick.Width != hairline+2 || thin.Width != hairline+0.5 || thick.Color != base.Axis {
		t.Fatalf("widths %v %v", thin.Width, thick.Width)
	}
}

func TestTooManyNodesAreRefused(t *testing.T) {
	var c NetworkChart
	for i := range networkMaxNodes {
		c.Links = append(c.Links, Link{"hub", fmt.Sprintf("n%d", i), 1})
	}
	rejects(t, c)
}

func TestNetworksUpToTheCapAreLaidOut(t *testing.T) {
	// A tree of fan-out three with cross links, at the cap: the quadtree
	// goes many levels deep and every node still lands inside the chart.
	var c NetworkChart
	for i := 1; i < networkMaxNodes; i++ {
		c.Links = append(c.Links, Link{fmt.Sprint("n", (i-1)/3), fmt.Sprint("n", i), 1})
		if i%7 == 0 {
			c.Links = append(c.Links, Link{fmt.Sprint("n", i), fmt.Sprint("n", i*13%i), 1})
		}
	}
	s := buildAt(t, c, 960, 640)
	dots := 0
	for _, m := range s.Marks {
		if d, ok := m.Shape.(scene.CircleShape); ok {
			dots++
			if !(d.CX-d.R >= pad-1e-6 && d.CX+d.R <= 960-pad+1e-6 && d.CY+d.R <= 640-pad+1e-6) {
				t.Fatalf("%s at (%v, %v) is outside", m.Hit.Label, d.CX, d.CY)
			}
		}
	}
	if dots != networkMaxNodes {
		t.Fatalf("%d dots", dots)
	}
}
