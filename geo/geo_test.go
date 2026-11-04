package geo

import (
	"encoding/json"
	"math"
	"testing"
)

func TestProjectionsMatchD3(t *testing.T) {
	// Reference values from d3-geo 3 (y negated: screen y grows down).
	x, y := Projection{}.Project(10, 50)
	if math.Abs(x-0.1331534461932999) > 1e-12 || math.Abs(y+0.8790066371341401) > 1e-12 {
		t.Errorf("natural earth (%v, %v)", x, y)
	}
	x, y = Projection{Kind: Mercator}.Project(0, 60)
	if x != 0 || math.Abs(y+1.316957896924816) > 1e-12 {
		t.Errorf("mercator (%v, %v)", x, y)
	}
	albers := Projection{Kind: Albers, Lat1: 29.5, Lat2: 45.5, Lon0: -96}
	x, y = albers.Project(-80, 40)
	if math.Abs(x-0.2109801886836941) > 1e-12 || math.Abs(y+0.686842147796396) > 1e-12 {
		t.Errorf("albers (%v, %v)", x, y)
	}
	// Equirectangular is degrees as radians, north up.
	x, y = Projection{Kind: Equirectangular}.Project(90, 45)
	if x != math.Pi/2 || y != -math.Pi/4 {
		t.Errorf("equirectangular (%v, %v)", x, y)
	}
}

func TestMercatorKeepsShapesAndStopsShortOfThePole(t *testing.T) {
	// Conformal: at 60° a degree of latitude is stretched exactly as much as
	// a degree of longitude, 1 / cos 60° = 2.
	m := Projection{Kind: Mercator}
	_, a := m.Project(0, 60-1e-4)
	_, b := m.Project(0, 60+1e-4)
	l, _ := m.Project(-1e-4, 60)
	r, _ := m.Project(1e-4, 60)
	if math.Abs((a-b)/(2e-4*deg)-2) > 1e-6 || math.Abs((r-l)/(2e-4*deg)-1) > 1e-12 {
		t.Errorf("dy %v dx %v", (a-b)/(2e-4*deg), (r-l)/(2e-4*deg))
	}
	// Past the cutoff everything lands on the square map's edge.
	_, edge := m.Project(0, mercatorLat)
	for _, lat := range []float64{86, 90} {
		if _, y := m.Project(0, lat); y != edge {
			t.Errorf("latitude %v projects to %v, the edge is %v", lat, y, edge)
		}
	}
	if _, y := m.Project(0, -90); math.Abs(y+edge) > 1e-12 {
		t.Errorf("the south pole projects to %v", y)
	}
}

func TestAlbersKeepsAreas(t *testing.T) {
	// 1°×1° cells near each standard parallel, at the equator and at 60°:
	// an equal-area projection draws each as large as its true spherical
	// area (cos φ shrinks the true area toward the pole).
	p := Projection{Kind: Albers, Lat1: 29.5, Lat2: 45.5, Lon0: -96}
	cell := func(lon, lat float64) float64 {
		var c [4][2]float64
		for i, ll := range [4][2]float64{{lon, lat}, {lon + 1, lat}, {lon + 1, lat + 1}, {lon, lat + 1}} {
			c[i][0], c[i][1] = p.Project(ll[0], ll[1])
		}
		s := 0.0
		for i := range 4 {
			j := (i + 1) % 4
			s += c[i][0]*c[j][1] - c[j][0]*c[i][1]
		}
		return math.Abs(s) / 2
	}
	trueArea := func(lat float64) float64 { return deg*math.Sin((lat+1)*deg) - deg*math.Sin(lat*deg) }
	for _, ll := range [][2]float64{{-96, 30}, {-80, 45}, {-120, 0}, {-100, 60}} {
		if r := cell(ll[0], ll[1]) / trueArea(ll[1]); math.Abs(r-1) > 1e-3 {
			t.Errorf("%v: area ratio %v", ll, r)
		}
	}
}

func TestAlbersWrapsLongitudeAroundItsMeridian(t *testing.T) {
	// 190° east of the central meridian is 170° west of it: the same point.
	p := Projection{Kind: Albers, Lat1: 29.5, Lat2: 45.5, Lon0: -96}
	for _, d := range []float64{190, -190} {
		x1, y1 := p.Project(-96+d, 40)
		x2, y2 := p.Project(-96+d-math.Copysign(360, d), 40)
		if math.Abs(x1-x2) > 1e-12 || math.Abs(y1-y2) > 1e-12 {
			t.Errorf("%v°: (%v, %v) vs (%v, %v)", d, x1, y1, x2, y2)
		}
	}
}

func TestAGreatCircleFromNewYorkToLondonBowsNorth(t *testing.T) {
	path := GreatCircle(Position{-74, 40.7}, Position{-0.1, 51.5}, 16)
	// Both ends go through the sphere and back, so within an ulp or so.
	if len(path) != 17 || math.Abs(path[0][0]+74) > 1e-9 || math.Abs(path[0][1]-40.7) > 1e-9 {
		t.Fatalf("%d points, first %v", len(path), path[0])
	}
	if math.Abs(path[16][0]+0.1) > 1e-9 || math.Abs(path[16][1]-51.5) > 1e-9 {
		t.Errorf("ends at %v", path[16])
	}
	// The shortest route arcs above the straight line on the map; midpoint
	// per d3-geo's geoInterpolate.
	if math.Abs(path[8][0]+41.275706935569964) > 1e-9 || math.Abs(path[8][1]-52.36254604648877) > 1e-9 {
		t.Errorf("midpoint %v", path[8])
	}
	// A route of no length stays put.
	for _, p := range GreatCircle(Position{3, 4}, Position{3, 4}, 4) {
		if p != (Position{3, 4}) {
			t.Fatalf("%v", p)
		}
	}
}

func TestTheBasemapFormatParses(t *testing.T) {
	f, err := ParseBasemap("# c\nAAA\tA land\t0,0 1,0 1,1|5,5 6,5 6,6;5.2,5.2 5.8,5.2 5.8,5.8\n\nBBB\tB\t0,0 1,0 1,1\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 2 || f[0].ID != "AAA" || f[0].Name != "A land" {
		t.Fatalf("%+v", f)
	}
	g := f[0].Geometry
	if !g.Multi || len(g.Polygons) != 2 {
		t.Fatalf("two polygons make a MultiPolygon: %+v", g)
	}
	if len(g.Polygons[1]) != 2 || g.Polygons[1][1][0] != (Position{5.2, 5.2}) {
		t.Fatalf("exterior plus one hole: %v", g.Polygons[1])
	}
	if f[1].Geometry.Multi || f[1].Geometry.Polygons[0][0][2] != (Position{1, 1}) {
		t.Fatalf("one polygon is a Polygon: %+v", f[1].Geometry)
	}
	for _, bad := range []string{"AAA\tA\n", "AAA\tA\t0,0 1\n", "AAA\tA\t0,x\n"} {
		if _, err := ParseBasemap(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestGeometryAndProjectionReadTheSharedJSON(t *testing.T) {
	// The Rust crate's serde forms; an unknown name must fail rather than
	// quietly fall back to the default projection.
	var f Feature
	if err := json.Unmarshal([]byte(`{"id":"X","name":"Y","geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1]]]}}`), &f); err != nil {
		t.Fatal(err)
	}
	if f.Geometry.Multi || f.Geometry.Polygons[0][0][1] != (Position{1, 0}) {
		t.Fatalf("%+v", f)
	}
	var g Geometry
	if err := json.Unmarshal([]byte(`{"type":"MultiPolygon","coordinates":[[[[0,0],[1,0],[1,1]]],[[[5,5],[6,5],[6,6]]]]}`), &g); err != nil || !g.Multi || len(g.Polygons) != 2 {
		t.Fatalf("%+v %v", g, err)
	}
	for _, bad := range []string{`{"type":"Point","coordinates":[0,0]}`, `{"type":"Polygon"}`, `{"type":"Polygon","coordinates":[[[0,0,5]]]}`} {
		if err := json.Unmarshal([]byte(bad), &g); err == nil {
			t.Errorf("%s decoded", bad)
		}
	}
	for in, want := range map[string]Projection{
		`"naturalEarth"`:    {},
		`"mercator"`:        {Kind: Mercator},
		`"equirectangular"`: {Kind: Equirectangular},
		`{"albers":{"lat1":29.5,"lat2":45.5,"lon0":-96}}`: {Kind: Albers, Lat1: 29.5, Lat2: 45.5, Lon0: -96},
	} {
		var p Projection
		if err := json.Unmarshal([]byte(in), &p); err != nil || p != want {
			t.Errorf("%s: %+v %v", in, p, err)
		}
		out, err := json.Marshal(want)
		var back Projection
		if err != nil || json.Unmarshal(out, &back) != nil || back != want {
			t.Errorf("%+v round-trips as %s", want, out)
		}
	}
	for _, bad := range []string{`"albers"`, `"Mercator"`, `{"albers":{"lat1":1}}`, `{"albers":{"lat2":1,"lon0":2}}`, `{"mercator":null}`, `null`} {
		var p Projection
		if err := json.Unmarshal([]byte(bad), &p); err == nil {
			t.Errorf("%s decoded as %+v", bad, p)
		}
	}
	out, err := json.Marshal(f.Geometry)
	if err != nil || string(out) != `{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1]]]}` {
		t.Fatalf("%s %v", out, err)
	}
}

func TestBasemapsAreFoundByName(t *testing.T) {
	RegisterBasemap("test-tiles", func() []Feature { return []Feature{{ID: "T"}} })
	if f, ok := Basemap("test-tiles"); !ok || f[0].ID != "T" {
		t.Fatalf("%v %v", f, ok)
	}
	if _, ok := Basemap("mars"); ok {
		t.Fatal("an unregistered basemap was found")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("a second registration must panic, not replace the first")
		}
	}()
	RegisterBasemap("test-tiles", nil)
}
