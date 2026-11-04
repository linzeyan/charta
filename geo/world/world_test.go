package world

import (
	"testing"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/geo"
	"github.com/linzeyan/charta/scene"
)

func TestTheWorldHasEveryCountryOnce(t *testing.T) {
	w := Features()
	if len(w) != 177 {
		t.Fatalf("%d countries, want 177", len(w))
	}
	seen := map[string]bool{}
	france := false
	for _, f := range w {
		if seen[f.ID] {
			t.Errorf("%q twice", f.ID)
		}
		seen[f.ID] = true
		france = france || (f.ID == "FRA" && f.Name == "France")
	}
	if !france {
		t.Error("no France")
	}
}

func TestImportingRegistersTheWorldBasemap(t *testing.T) {
	// The map charts find "world" only through the registry; a missing
	// registration would make every basemap spec fail.
	f, ok := geo.Basemap("world")
	if !ok || len(f) != len(Features()) {
		t.Fatalf("registered %v, %d features", ok, len(f))
	}
}

func TestTheWorldFitsInsideThePlot(t *testing.T) {
	// The fit scales the whole basemap into the plot box: no country may
	// spill into the padding, whichever projection.
	for _, p := range []geo.Projection{{}, {Kind: geo.Mercator}, {Kind: geo.Equirectangular}} {
		s, err := charta.ChoroplethChart{Basemap: "world", Projection: p, Values: []charta.Region{{ID: "BRA", Value: 1}}}.Scene(640, 400, base.InterMetrics{})
		if err != nil {
			t.Fatal(err)
		}
		paths := 0
		for _, m := range s.Marks {
			ps, ok := m.Shape.(scene.PathShape)
			if !ok {
				continue
			}
			paths++
			for _, sub := range scene.Flatten(ps.Ops) {
				for _, q := range sub {
					if q.X < 16-1e-6 || q.X > 640-16+1e-6 || q.Y < 16-1e-6 || q.Y > 400-16+1e-6 {
						t.Fatalf("%+v: (%v, %v) is outside the plot", p, q.X, q.Y)
					}
				}
			}
		}
		if paths != 177 {
			t.Fatalf("%d regions drawn", paths)
		}
	}
}
