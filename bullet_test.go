package charta

import (
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func bandRects(s *scene.Scene) []scene.Mark {
	var out []scene.Mark
	for _, m := range s.Marks {
		if _, ok := m.Shape.(scene.RectShape); ok && m.Fill >= base.Band(0) && m.Fill < base.Band(base.BandClasses) {
			out = append(out, m)
		}
	}
	return out
}

func TestBulletRangesNestFromZeroWithTheWidestLightest(t *testing.T) {
	target := 250.0
	s := buildAt(t, BulletChart{Data: []Bullet{{Label: "Revenue", Value: 270, Target: &target, Ranges: []float64{150, 225, 300}}}}, 640, 300)
	b := bandRects(s)
	if len(b) != 3 || b[0].Fill != base.Band(2) {
		t.Fatalf("%d bands, first %v", len(b), b)
	}
	if !(rectOf(b[0]).W > rectOf(b[1]).W && rectOf(b[1]).W > rectOf(b[2]).W) {
		t.Fatal("wider ranges must paint first")
	}
	for _, m := range b {
		if !near(rectOf(m).X, rectOf(b[0]).X) {
			t.Fatal("every range starts at zero")
		}
	}
	if h := dataRects(s)[0].Hit; h.Label != "Revenue · target 250" {
		t.Fatal(h.Label)
	}
}

func TestEachBulletGaugeHasItsOwnScale(t *testing.T) {
	// A 0–5 rating next to a 0–2,500 count must still fill its gauge.
	s := buildAt(t, BulletChart{Data: []Bullet{
		{Label: "Customers", Value: 1500, Ranges: []float64{2500}},
		{Label: "Satisfaction", Value: 4.4, Ranges: []float64{5}},
	}}, 640, 300)
	b := bandRects(s)
	if !near(rectOf(b[0]).W, rectOf(b[1]).W) {
		t.Fatalf("%v vs %v", rectOf(b[0]).W, rectOf(b[1]).W)
	}
}

func TestFourBulletRangesAreRejected(t *testing.T) {
	rejects(t, BulletChart{Data: []Bullet{{Label: "x", Value: 1, Ranges: []float64{1, 2, 3, 4}}}})
}
