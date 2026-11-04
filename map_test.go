package charta

import (
	"math"
	"strings"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/geo"
	"github.com/linzeyan/charta/scene"
)

func square(x, y, s float64, clockwise bool) []geo.Position {
	r := []geo.Position{{x, y}, {x + s, y}, {x + s, y + s}, {x, y + s}}
	if clockwise {
		r[1], r[3] = r[3], r[1]
	}
	return r
}

// signed is a closed polyline's signed area (twice it, unhalved).
func signed(p []scene.Point) float64 {
	s := 0.0
	for i := 0; i+1 < len(p); i++ {
		s += p[i].X*p[i+1].Y - p[i+1].X*p[i].Y
	}
	return s
}

func TestHolesWindAgainstTheirExteriorWhateverTheInput(t *testing.T) {
	// Both rings given the same way round, as sloppy GeoJSON does: the hole
	// must still come out opposite so nonzero filling skips it.
	fit := newMapFit(geo.Projection{Kind: geo.Equirectangular}, nil, []geo.Position{{0, 0}, {10, 10}}, 0, 0, 100, 100)
	for _, cw := range []bool{false, true} {
		g := geo.Geometry{Polygons: []geo.Polygon{{square(0, 0, 10, cw), square(4, 4, 2, cw)}}}
		subs := scene.Flatten(fit.region(g))
		if len(subs) != 2 || signed(subs[0])*signed(subs[1]) >= 0 {
			t.Fatalf("clockwise=%v: rings wind %v and %v", cw, signed(subs[0]), signed(subs[1]))
		}
		// The exterior always comes out the same way, so both inputs fill
		// alike.
		if signed(subs[0]) < 0 {
			t.Errorf("clockwise=%v: exterior winds %v", cw, signed(subs[0]))
		}
	}
	// A ring of fewer than three points encloses nothing and is dropped.
	if ops := fit.region(geo.Geometry{Polygons: []geo.Polygon{{{{0, 0}, {1, 1}}}}}); len(ops) != 0 {
		t.Fatalf("a degenerate ring drew %v", ops)
	}
}

func TestTheFitFillsOneSideAndCentersTheOther(t *testing.T) {
	// A 20°×10° box into a 200×200 plot: width-bound, centered vertically.
	fit := newMapFit(geo.Projection{Kind: geo.Equirectangular}, nil, []geo.Position{{0, 0}, {20, 10}}, 0, 0, 200, 200)
	ax, ay := fit.xy(0, 10)
	bx, by := fit.xy(20, 0)
	if !near(ax, 0) || !near(bx, 200) {
		t.Errorf("x spans %v to %v", ax, bx)
	}
	if !near((ay+by)/2, 100) || ay >= by {
		t.Errorf("y %v (north) and %v (south) not centered with north up", ay, by)
	}
	// Region exteriors count toward the bounds, holes and all.
	tile := []geo.Feature{{Geometry: geo.Geometry{Polygons: []geo.Polygon{{square(0, 0, 10, false)}}}}}
	fit = newMapFit(geo.Projection{Kind: geo.Equirectangular}, tile, nil, 0, 0, 100, 50)
	if x, _ := fit.xy(10, 0); !near(x, 75) {
		t.Errorf("a 10° square in a 100×50 plot reaches x %v, want 75", x)
	}
	// Holes never shrink the bounds: the exterior alone sets them.
	holed := []geo.Feature{{Geometry: geo.Geometry{Polygons: []geo.Polygon{{square(0, 0, 10, false), square(4, 4, 2, false)}}}}}
	fit = newMapFit(geo.Projection{Kind: geo.Equirectangular}, holed, nil, 0, 0, 100, 50)
	if x, _ := fit.xy(10, 0); !near(x, 75) {
		t.Errorf("with a hole the square reaches x %v, want 75", x)
	}
	// Nothing at all still fits without dividing by zero.
	fit = newMapFit(geo.Projection{}, nil, []geo.Position{{5, 5}}, 0, 0, 100, 100)
	if x, y := fit.xy(5, 5); !near(x, 50) || !near(y, 50) {
		t.Errorf("a lone point lands at (%v, %v), want the center", x, y)
	}
}

func TestABasemapAndOwnFeaturesCannotMix(t *testing.T) {
	own := []geo.Feature{{ID: "X", Name: "X", Geometry: geo.Geometry{Polygons: []geo.Polygon{{square(0, 0, 1, false)}}}}}
	if _, err := regions(own, "world", geo.Projection{}); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("features and a basemap: %v", err)
	}
	if _, err := regions(nil, "mars", geo.Projection{}); err == nil || !strings.Contains(err.Error(), `"mars"`) {
		t.Fatalf("unknown basemap: %v", err)
	}
	if f, err := regions(own, "", geo.Projection{}); err != nil || len(f) != 1 {
		t.Fatalf("own features: %v %v", f, err)
	}
	if _, err := regions(own, "", geo.Projection{Kind: geo.Albers + 1}); err == nil {
		t.Fatal("an unknown projection kind must fail, not panic")
	}
	for _, ll := range [][2]float64{{181, 0}, {-181, 0}, {0, 91}, {0, -91}, {math.NaN(), 0}, {0, math.Inf(1)}} {
		if checkLonLat("x", ll[0], ll[1]) == nil {
			t.Errorf("%v is on the globe", ll)
		}
	}
	for _, ll := range [][2]float64{{180, 90}, {-180, -90}, {0, 0}} {
		if err := checkLonLat("x", ll[0], ll[1]); err != nil {
			t.Errorf("%v: %v", ll, err)
		}
	}
}

func TestLandIsAQuietBackdrop(t *testing.T) {
	// Map marks sit on regions drawn in the grid color with surface-colored
	// hairline borders and no hover, so only the data reads.
	s := build(t, BubbleMapChart{
		Features: []geo.Feature{{ID: "A", Name: "A", Geometry: geo.Geometry{Polygons: []geo.Polygon{{square(0, 0, 10, false)}}}}},
		Places:   []Place{{Name: "p", Lon: 5, Lat: 5, Value: 1}},
	})
	m := s.Marks[0]
	if _, ok := m.Shape.(scene.PathShape); !ok || m.Fill != base.Grid || m.Stroke == nil || m.Stroke.Color != base.Surface || m.Stroke.Width != mapBorder || m.Hit != nil {
		t.Fatalf("%+v", m)
	}
}
