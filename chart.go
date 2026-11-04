// Package charta renders interactive charts in pure Go: HTML/SVG output for
// the web, plus native GUI backends in sibling packages (gio).
//
// Every chart builds a backend-agnostic scene.Scene. This module is a port
// of the Rust crate in the same repository; both must produce identical
// scenes for the same chart, which the conformance fixtures check.
//
//	chart := charta.BarChart{Title: "Fruit sold", Data: []charta.Bar{{Label: "Apples", Value: 30}}}
//	page, err := charta.RenderHTML(chart, charta.Options{})
package charta

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/html"
	"github.com/linzeyan/charta/scene"
)

// Chart is data plus the layout that turns it into a scene.
type Chart interface {
	// Scene lays the chart out at width × height pixels, sizing text with m.
	Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error)
}

// Zoomable is a chart whose x axis runs through categories and that can
// show a window of them (package zoom): Line, Area and Stacked area.
// Backends check for it with a type assertion; a separate interface keeps
// every other chart free of methods that would only say "no".
type Zoomable interface {
	Chart
	// CategoryCount is how many categories the x axis runs through.
	CategoryCount() int
	// Window is the chart narrowed to categories start..end; its y axis
	// fits the data in view.
	Window(start, end int) Chart
}

var (
	_ Zoomable = LineChart{}
	_ Zoomable = AreaChart{}
	_ Zoomable = StackedAreaChart{}
)

// Viewable is a chart whose x axis is continuous (time or numbers) and that
// can show a range of it (package zoom's View): Candlestick, OHLC, Gantt and
// Stream, and Scatter and Bubble, which zoom y as well.
type Viewable interface {
	Chart
	// PlotScene is Scene plus where its plot sits and the data at its edges.
	PlotScene(width, height float64, m base.TextMeasure) (*scene.Scene, scene.Plot, error)
	// View is the chart showing x in x and, for a chart that zooms y, y in
	// *y: its axes run exactly over them, and what falls outside is left out.
	View(x [2]float64, y *[2]float64) Viewable
}

var (
	_ Viewable = CandlestickChart{}
	_ Viewable = OHLCChart{}
	_ Viewable = GanttChart{}
	_ Viewable = StreamChart{}
	_ Viewable = ScatterChart{}
	_ Viewable = BubbleChart{}
)

// ranges is what a zoomed chart shows: x, and y for charts that zoom it.
type ranges struct {
	x [2]float64
	y *[2]float64
}

// laidOuter is a chart with continuous axes that lays itself out over given
// ranges, or over all its data when v is nil.
type laidOuter interface {
	laidOut(width, height float64, m base.TextMeasure, v *ranges) (*scene.Scene, scene.Plot, error)
}

// inView is a chart shown over fixed ranges, what View returns.
type inView[C laidOuter] struct {
	chart C
	v     ranges
}

func (c inView[C]) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	s, _, err := c.chart.laidOut(width, height, m, &c.v)
	return s, err
}

func (c inView[C]) PlotScene(width, height float64, m base.TextMeasure) (*scene.Scene, scene.Plot, error) {
	return c.chart.laidOut(width, height, m, &c.v)
}

func (c inView[C]) View(x [2]float64, y *[2]float64) Viewable {
	return inView[C]{c.chart, ranges{x, y}}
}

// Options control RenderHTML. The zero value renders a 640×400 HTML chart
// that follows the viewer's light/dark setting.
type Options struct {
	Width, Height float64
	Theme         base.Theme
	Format        html.Format
}

// RenderHTML lays the chart out with Inter metrics and renders HTML or SVG.
func RenderHTML(c Chart, opts Options) (string, error) {
	if opts.Width == 0 {
		opts.Width = 640
	}
	if opts.Height == 0 {
		opts.Height = 400
	}
	s, err := c.Scene(opts.Width, opts.Height, base.InterMetrics{})
	if err != nil {
		return "", err
	}
	return html.Render(s, opts.Theme, opts.Format), nil
}

// ErrInvalidSize reports a width or height that is not positive and finite.
var ErrInvalidSize = errors.New("chart width and height must be positive and finite")

// InvalidDataError reports chart data that cannot be drawn; the message
// names the offending item.
type InvalidDataError struct{ Msg string }

func (e *InvalidDataError) Error() string { return "invalid chart data: " + e.Msg }

func invalid(format string, a ...any) error {
	return &InvalidDataError{fmt.Sprintf(format, a...)}
}

// DecodeSpec reads a chart from JSON tagged by "type", the same format the
// Rust crate's ChartSpec reads.
func DecodeSpec(data []byte) (Chart, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, err
	}
	switch head.Type {
	case "bar":
		return decode[BarChart](data)
	case "groupedBar":
		return decode[GroupedBarChart](data)
	case "stackedBar":
		return decode[StackedBarChart](data)
	case "line":
		return decode[LineChart](data)
	case "area":
		return decode[AreaChart](data)
	case "stackedArea":
		return decode[StackedAreaChart](data)
	case "scatter":
		return decode[ScatterChart](data)
	case "bubble":
		return decode[BubbleChart](data)
	case "heatmap":
		return decode[HeatmapChart](data)
	case "span":
		return decode[SpanChart](data)
	case "pyramid":
		return decode[PyramidChart](data)
	case "bullet":
		return decode[BulletChart](data)
	case "errorBars":
		return decode[ErrorBarChart](data)
	case "histogram":
		return decode[HistogramChart](data)
	case "density":
		return decode[DensityChart](data)
	case "boxPlot":
		return decode[BoxPlotChart](data)
	case "violin":
		return decode[ViolinChart](data)
	case "stemLeaf":
		return decode[StemLeafChart](data)
	case "parallel":
		return decode[ParallelChart](data)
	case "candlestick":
		return decode[CandlestickChart](data)
	case "ohlc":
		return decode[OHLCChart](data)
	case "kagi":
		return decode[KagiChart](data)
	case "pointFigure":
		return decode[PointFigureChart](data)
	case "gantt":
		return decode[GanttChart](data)
	case "timeline":
		return decode[TimelineChart](data)
	case "stream":
		return decode[StreamChart](data)
	case "calendar":
		return decode[CalendarChart](data)
	case "timetable":
		return decode[TimetableChart](data)
	case "spiral":
		return decode[SpiralChart](data)
	case "pie":
		return decode[PieChart](data)
	case "donut":
		return decode[DonutChart](data)
	case "rose":
		return decode[RoseChart](data)
	case "radar":
		return decode[RadarChart](data)
	case "radialBar":
		return decode[RadialBarChart](data)
	case "radialColumn":
		return decode[RadialColumnChart](data)
	case "treemap":
		return decode[TreemapChart](data)
	case "circlePacking":
		return decode[CirclePackingChart](data)
	case "sunburst":
		return decode[SunburstChart](data)
	case "tree":
		return decode[TreeChart](data)
	case "marimekko":
		return decode[MarimekkoChart](data)
	case "brainstorm":
		return decode[BrainstormChart](data)
	case "network":
		return decode[NetworkChart](data)
	case "arc":
		return decode[ArcChart](data)
	case "chord":
		return decode[ChordChart](data)
	case "nonRibbonChord":
		return decode[NonRibbonChordChart](data)
	case "sankey":
		return decode[SankeyChart](data)
	case "parallelSets":
		return decode[ParallelSetsChart](data)
	case "flowChart":
		return decode[FlowChart](data)
	case "choropleth":
		return decode[ChoroplethChart](data)
	case "bubbleMap":
		return decode[BubbleMapChart](data)
	case "dotMap":
		return decode[DotMapChart](data)
	case "connectionMap":
		return decode[ConnectionMapChart](data)
	case "flowMap":
		return decode[FlowMapChart](data)
	case "pictogram":
		return decode[PictogramChart](data)
	case "proportionalArea":
		return decode[ProportionalAreaChart](data)
	case "tally":
		return decode[TallyChart](data)
	case "dotMatrix":
		return decode[DotMatrixChart](data)
	case "wordCloud":
		return decode[WordCloudChart](data)
	case "venn":
		return decode[VennChart](data)
	}
	return nil, fmt.Errorf("unknown chart type %q", head.Type)
}

func decode[T Chart](data []byte) (Chart, error) {
	var c T
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return c, nil
}

// Series is one named series of values, one per category.
type Series struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
}

// maxSeries: a ninth series is never a generated hue.
const maxSeries = base.SeriesSlots

// checkSeries: series must line up with the categories, be finite, have
// distinct names (a name is the series' identity for highlight and hide)
// and fit the palette.
func checkSeries(categories []string, series []Series, limit int) error {
	if len(series) > limit {
		return invalid(`%d series, at most %d: fold the rest into "Other" or use small multiples`, len(series), limit)
	}
	for j, s := range series {
		for _, o := range series[:j] {
			if o.Name == s.Name {
				return invalid("series name %q is used twice", s.Name)
			}
		}
		if len(s.Values) != len(categories) {
			return invalid("series %q has %d values for %d categories", s.Name, len(s.Values), len(categories))
		}
		for i, v := range s.Values {
			if !isFinite(v) {
				return invalid("series %q has a non-finite value at %q", s.Name, categories[i])
			}
		}
	}
	return nil
}

// TimeValue is a value at an instant (UTC epoch ms).
type TimeValue struct {
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
}

// checkTimes: values must be finite and times strictly increasing.
func checkTimes(data []TimeValue) error {
	for i, d := range data {
		if err := finite(fmt.Sprintf("value %d", i), d.Value); err != nil {
			return err
		}
		if i > 0 && d.Time <= data[i-1].Time {
			return invalid("value %d: times must increase", i)
		}
	}
	return nil
}

// Sample is a named sample of raw observations, summarised by a
// statistical chart.
type Sample struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
}

func (s Sample) sorted() []float64 {
	v := slices.Clone(s.Values)
	slices.Sort(v)
	return v
}

// checkSamples: samples need distinct names, at least one value each, all
// finite. what names them in errors ("series", "group").
func checkSamples(samples []Sample, limit int, what string) error {
	if len(samples) > limit {
		return invalid(`%d %s, at most %d: fold the rest into "Other" or use small multiples`, len(samples), what, limit)
	}
	for j, s := range samples {
		for _, o := range samples[:j] {
			if o.Name == s.Name {
				return invalid("%s name %q is used twice", what, s.Name)
			}
		}
		if len(s.Values) == 0 {
			return invalid("%s %q has no values", what, s.Name)
		}
		for _, v := range s.Values {
			if !isFinite(v) {
				return invalid("%s %q has a non-finite value", what, s.Name)
			}
		}
	}
	return nil
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func finite(what string, v float64) error {
	if isFinite(v) {
		return nil
	}
	return invalid("%s is not finite", what)
}

// Layout constants shared by charts (px), identical to the Rust crate's.
// Mark sizes follow the dataviz skill's mark specs.
const (
	pad         = 16.0
	gap         = 8.0
	titleSize   = 14.0
	tickSize    = 11.0
	tickSpacing = 50.0
	hairline    = 1.0
	// lineWidth is for series lines and whiskers.
	lineWidth = 2.0
	// dotRadius keeps markers at least 8px across.
	dotRadius = 4.0
	// surfaceGap is the surface-colored gap between touching marks, and the
	// ring around dots.
	surfaceGap = 2.0
	// minTarget is the smallest hover target a small mark gets.
	minTarget = 24.0
)

func checkSize(width, height float64) error {
	if !isFinite(width) || !isFinite(height) || !(width > 0) || !(height > 0) {
		return ErrInvalidSize
	}
	return nil
}

// describe is the accessible name: the title, if any, then what kind of
// chart it is.
func describe(title, what string) string {
	if title != "" {
		return title + ". " + what
	}
	return what
}

// count is "1 category", "3 categories". The plural is spelled out because
// English plurals are irregular ("series", "categories").
func count(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// snap centers a 1px line on a pixel so it renders crisp instead of blurred.
func snap(v float64) float64 { return math.Floor(v) + 0.5 }
