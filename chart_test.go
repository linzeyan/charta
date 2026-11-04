package charta

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/geo"
	"github.com/linzeyan/charta/scene"
)

func build(t *testing.T, c Chart) *scene.Scene {
	t.Helper()
	return buildAt(t, c, 640, 400)
}

func buildAt(t *testing.T, c Chart, w, h float64) *scene.Scene {
	t.Helper()
	s, err := c.Scene(w, h, base.InterMetrics{})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// rejects asserts the chart refuses its data with an InvalidDataError
// instead of drawing something misleading.
func rejects(t *testing.T, c Chart) {
	t.Helper()
	_, err := c.Scene(640, 400, base.InterMetrics{})
	var de *InvalidDataError
	if !errors.As(err, &de) {
		t.Fatalf("want InvalidDataError, got %v", err)
	}
}

// dataRects are the rects of data marks (not legend entries), in paint order.
func dataRects(s *scene.Scene) []scene.Mark {
	var out []scene.Mark
	for _, m := range s.Marks {
		if _, ok := m.Shape.(scene.RectShape); ok && m.Hit != nil && m.Hit.Kind == scene.HitMark {
			out = append(out, m)
		}
	}
	return out
}

func rectOf(m scene.Mark) scene.Rect      { return m.Shape.(scene.RectShape).Rect }
func radiusOf(m scene.Mark) scene.Corners { return m.Shape.(scene.RectShape).Radius }

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestDecodeSpecReadsTheSharedJSON(t *testing.T) {
	c, err := DecodeSpec([]byte(`{"type":"bar","title":"T","data":[{"label":"A","value":2}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if bc := c.(BarChart); bc.Title != "T" || bc.Data[0].Value != 2 {
		t.Fatalf("%+v", bc)
	}
	// Field names follow the Rust serde names, camelCase included.
	c, err = DecodeSpec([]byte(`{"type":"scatter","xLabel":"X","yLabel":"Y","series":[{"name":"a","points":[{"x":1,"y":2}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if sc := c.(ScatterChart); sc.XLabel != "X" || sc.YLabel != "Y" || sc.Series[0].Points[0].Size != nil {
		t.Fatalf("%+v", sc)
	}
	for tag, want := range map[string]Chart{
		"bar": BarChart{}, "groupedBar": GroupedBarChart{}, "stackedBar": StackedBarChart{},
		"line": LineChart{}, "area": AreaChart{}, "stackedArea": StackedAreaChart{},
		"scatter": ScatterChart{}, "bubble": BubbleChart{}, "heatmap": HeatmapChart{},
		"span": SpanChart{}, "pyramid": PyramidChart{}, "bullet": BulletChart{}, "errorBars": ErrorBarChart{},
		"histogram": HistogramChart{}, "density": DensityChart{}, "boxPlot": BoxPlotChart{}, "violin": ViolinChart{},
		"stemLeaf": StemLeafChart{}, "parallel": ParallelChart{},
		"candlestick": CandlestickChart{}, "ohlc": OHLCChart{}, "kagi": KagiChart{}, "pointFigure": PointFigureChart{},
		"gantt": GanttChart{}, "timeline": TimelineChart{}, "stream": StreamChart{}, "calendar": CalendarChart{},
		"timetable": TimetableChart{}, "spiral": SpiralChart{},
		"pie": PieChart{}, "donut": DonutChart{}, "rose": RoseChart{}, "radar": RadarChart{},
		"radialBar": RadialBarChart{}, "radialColumn": RadialColumnChart{},
		"treemap": TreemapChart{}, "circlePacking": CirclePackingChart{}, "sunburst": SunburstChart{},
		"tree": TreeChart{}, "marimekko": MarimekkoChart{}, "brainstorm": BrainstormChart{},
		"network": NetworkChart{}, "arc": ArcChart{}, "chord": ChordChart{}, "nonRibbonChord": NonRibbonChordChart{},
		"sankey": SankeyChart{}, "parallelSets": ParallelSetsChart{}, "flowChart": FlowChart{},
		"choropleth": ChoroplethChart{}, "bubbleMap": BubbleMapChart{}, "dotMap": DotMapChart{},
		"connectionMap": ConnectionMapChart{}, "flowMap": FlowMapChart{},
		"pictogram": PictogramChart{}, "proportionalArea": ProportionalAreaChart{}, "tally": TallyChart{},
		"dotMatrix": DotMatrixChart{}, "wordCloud": WordCloudChart{}, "venn": VennChart{},
	} {
		got, err := DecodeSpec([]byte(`{"type":"` + tag + `"}`))
		if err != nil {
			t.Fatalf("%s: %v", tag, err)
		}
		if reflect.TypeOf(got) != reflect.TypeOf(want) {
			t.Errorf("%s decoded as %T, want %T", tag, got, want)
		}
	}
	c, err = DecodeSpec([]byte(`{"type":"stemLeaf","values":[1],"leafUnit":0.1}`))
	if err != nil {
		t.Fatal(err)
	}
	if sl := c.(StemLeafChart); sl.LeafUnit == nil || *sl.LeafUnit != 0.1 {
		t.Fatalf("%+v", sl)
	}
	if _, err := DecodeSpec([]byte(`{"type":"nope"}`)); err == nil {
		t.Fatal("unknown types must fail")
	}
}

func TestDecodeSpecReadsTheTimeChartFields(t *testing.T) {
	// The fixtures spell these the Rust serde way; a misnamed tag would
	// silently fall back to the default and draw a different chart.
	c, err := DecodeSpec([]byte(`{"type":"pointFigure","utcOffset":540,"boxSize":0.5,"reversal":2,"data":[{"time":1,"value":2}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if pf := c.(PointFigureChart); pf.UTCOffset != 540 || *pf.BoxSize != 0.5 || *pf.Reversal != 2 || pf.Data[0] != (TimeValue{1, 2}) {
		t.Fatalf("%+v", pf)
	}
	c, err = DecodeSpec([]byte(`{"type":"spiral","period":12,"angleLabels":["Jan"],"data":[{"label":"a","value":1}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if sp := c.(SpiralChart); sp.Period != 12 || sp.AngleLabels[0] != "Jan" || sp.Data[0].Label != "a" {
		t.Fatalf("%+v", sp)
	}
	c, err = DecodeSpec([]byte(`{"type":"gantt","tasks":[{"name":"a","start":1,"end":2,"group":"g"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if g := c.(GanttChart); g.Tasks[0] != (Task{"a", 1, 2, "g"}) {
		t.Fatalf("%+v", g)
	}
}

func TestDecodeSpecReadsTheRelationshipFields(t *testing.T) {
	// Step kinds are lowercase and default to process; an arrow's label may
	// be left out. A misspelled kind must fail, not quietly draw a box.
	c, err := DecodeSpec([]byte(`{"type":"flowChart","steps":[{"id":"a","label":"A","kind":"decision"},{"id":"b","label":"B"},{"id":"c","label":"C","kind":"terminal"}],` +
		`"arrows":[{"from":"a","to":"b","label":"Yes"},{"from":"b","to":"c"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	f := c.(FlowChart)
	if f.Steps[0].Kind != Decision || f.Steps[1].Kind != Process || f.Steps[2].Kind != Terminal || f.Arrows[0].Label != "Yes" || f.Arrows[1] != (Arrow{"b", "c", ""}) {
		t.Fatalf("%+v", f)
	}
	if _, err := DecodeSpec([]byte(`{"type":"flowChart","steps":[{"id":"a","label":"A","kind":"Decision"}]}`)); err == nil {
		t.Fatal("kinds are lowercase, as serde spells them")
	}
	out, err := json.Marshal(f.Steps[0])
	if err != nil || string(out) != `{"id":"a","label":"A","kind":"decision"}` {
		t.Fatalf("%s %v", out, err)
	}
	c, err = DecodeSpec([]byte(`{"type":"parallelSets","dimensions":["A","B"],"combinations":[{"categories":["x","y"],"count":3}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if p := c.(ParallelSetsChart); p.Dimensions[1] != "B" || p.Combinations[0].Categories[1] != "y" || p.Combinations[0].Count != 3 {
		t.Fatalf("%+v", p)
	}
	c, err = DecodeSpec([]byte(`{"type":"sankey","links":[{"source":"a","target":"b","value":2}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if l := c.(SankeyChart).Links[0]; l != (Link{"a", "b", 2}) {
		t.Fatalf("%+v", l)
	}
}

func TestDecodeSpecReadsTheMapAndUnitFields(t *testing.T) {
	// The fixtures spell these the Rust serde way. A misspelled projection,
	// icon or shape must fail rather than draw the default.
	c, err := DecodeSpec([]byte(`{"type":"choropleth","basemap":"world","projection":{"albers":{"lat1":29.5,"lat2":45.5,"lon0":-96}},"values":[{"id":"USA","value":3}],` +
		`"features":[{"id":"X","name":"Y","geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1]]]}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if ch := c.(ChoroplethChart); ch.Basemap != "world" || ch.Projection != (geo.Projection{Kind: geo.Albers, Lat1: 29.5, Lat2: 45.5, Lon0: -96}) ||
		ch.Values[0] != (Region{"USA", 3}) || ch.Features[0].Name != "Y" {
		t.Fatalf("%+v", ch)
	}
	c, err = DecodeSpec([]byte(`{"type":"dotMap","projection":"equirectangular","dots":[{"lon":1,"lat":2,"group":"g"},{"lon":3,"lat":4}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if d := c.(DotMapChart); d.Projection.Kind != geo.Equirectangular || d.Dots[0] != (Dot{1, 2, "g"}) || d.Dots[1].Group != "" {
		t.Fatalf("%+v", d)
	}
	c, err = DecodeSpec([]byte(`{"type":"bubbleMap","places":[{"name":"a","lon":1,"lat":2,"value":3}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if b := c.(BubbleMapChart); b.Places[0] != (Place{"a", 1, 2, 3}) || b.Projection.Kind != geo.NaturalEarth {
		t.Fatalf("%+v", b)
	}
	c, err = DecodeSpec([]byte(`{"type":"flowMap","projection":"mercator","places":[{"name":"a","lon":1,"lat":2}],"links":[{"source":"a","target":"b","value":1}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if f := c.(FlowMapChart); f.Projection.Kind != geo.Mercator || f.Places[0] != (Location{"a", 1, 2}) || f.Links[0].Target != "b" {
		t.Fatalf("%+v", f)
	}
	c, err = DecodeSpec([]byte(`{"type":"pictogram","unit":10,"icon":{"path":{"d":"M0 0Z","size":24}},"data":[{"label":"a","value":5}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if p := c.(PictogramChart); p.Unit != 10 || p.Icon != (Icon{IconPath, "M0 0Z", 24}) || p.Data[0] != (Bar{"a", 5}) {
		t.Fatalf("%+v", p)
	}
	c, err = DecodeSpec([]byte(`{"type":"proportionalArea","shape":"square","data":[]}`))
	if err != nil || c.(ProportionalAreaChart).Shape != AreaSquare {
		t.Fatalf("%+v %v", c, err)
	}
	c, err = DecodeSpec([]byte(`{"type":"wordCloud","words":[{"text":"a","weight":2}]}`))
	if err != nil || c.(WordCloudChart).Words[0] != (Word{"a", 2}) {
		t.Fatalf("%+v %v", c, err)
	}
	c, err = DecodeSpec([]byte(`{"type":"venn","areas":[{"sets":["a","b"],"size":2}]}`))
	if err != nil || c.(VennChart).Areas[0].Sets[1] != "b" || c.(VennChart).Areas[0].Size != 2 {
		t.Fatalf("%+v %v", c, err)
	}
	for _, bad := range []string{
		`{"type":"choropleth","projection":"robinson"}`,
		`{"type":"pictogram","icon":"star"}`,
		`{"type":"proportionalArea","shape":"Square"}`,
		`{"type":"choropleth","features":[{"id":"X","name":"Y","geometry":{"type":"LineString","coordinates":[]}}]}`,
	} {
		if _, err := DecodeSpec([]byte(bad)); err == nil {
			t.Errorf("%s decoded", bad)
		}
	}
}
