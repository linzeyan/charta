package charta

import (
	"fmt"
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/geo"
	"github.com/linzeyan/charta/scene"
)

func tiles() []geo.Feature {
	var out []geo.Feature
	for i := range 3 {
		x := float64(i) * 10
		out = append(out, geo.Feature{
			ID:       fmt.Sprintf("T%d", i),
			Name:     fmt.Sprintf("Tile %d", i),
			Geometry: geo.Geometry{Polygons: []geo.Polygon{{{{x, 0}, {x + 10, 0}, {x + 10, 10}, {x, 10}}}}},
		})
	}
	return out
}

func regionNamed(t *testing.T, s *scene.Scene, name string) scene.Mark {
	t.Helper()
	for _, m := range s.Marks {
		if m.Hit != nil && m.Hit.Label == name {
			return m
		}
	}
	t.Fatalf("no region %q", name)
	return scene.Mark{}
}

func TestValuesPickRampClassesAndMissingRegionsStayNeutral(t *testing.T) {
	s := build(t, ChoroplethChart{Features: tiles(), Values: []Region{{"T0", 1}, {"T2", 8}}})
	if m := regionNamed(t, s, "Tile 0"); m.Fill != base.Seq(0) || m.Hit.Value != "1" {
		t.Errorf("the low value takes the first class: %v %q", m.Fill.Token(), m.Hit.Value)
	}
	if m := regionNamed(t, s, "Tile 2"); m.Fill != base.Seq(6) {
		t.Errorf("the high value takes the last class: %v", m.Fill.Token())
	}
	if m := regionNamed(t, s, "Tile 1"); m.Fill != base.Grid || m.Hit.Value != "no data" {
		t.Errorf("a region without a value: %v %q", m.Fill.Token(), m.Hit.Value)
	}
	// The ramp's key sits under the title only when there are values.
	none := build(t, ChoroplethChart{Features: tiles()})
	if len(none.Marks) != 3 || !strings.Contains(none.Label, "3 regions, 0 with data") {
		t.Errorf("no values: %d marks, %q", len(none.Marks), none.Label)
	}
}

func TestAnUnknownIDIsAnErrorNotASilentGap(t *testing.T) {
	_, err := ChoroplethChart{Features: tiles(), Values: []Region{{"T9", 1}}}.Scene(640, 400, base.InterMetrics{})
	if err == nil || !strings.Contains(err.Error(), `"T9"`) {
		t.Fatalf("%v", err)
	}
	rejects(t, ChoroplethChart{Features: tiles(), Values: []Region{{"T0", 1}, {"T0", 2}}})
}
