pub mod arc;
pub mod bar;
pub mod boxplot;
pub mod bullet;
pub mod calendar;
pub mod chord;
pub mod choropleth;
pub mod density;
pub mod error_bars;
pub mod finance;
pub mod flowchart;
pub(crate) mod frame;
pub mod gantt;
pub mod graph;
pub mod grouped;
pub mod heatmap;
pub(crate) mod hierarchy;
pub mod histogram;
pub mod icon;
pub mod kagi;
pub mod line;
pub(crate) mod map;
pub mod marimekko;
pub mod network;
pub mod packing;
pub mod parallel;
pub mod parallel_sets;
pub mod pie;
pub mod point;
pub mod point_figure;
pub mod point_map;
pub(crate) mod polar;
pub mod pyramid;
pub mod radar;
pub mod radial;
pub mod rose;
pub mod route_map;
pub mod sankey;
pub mod span;
pub mod spiral;
pub mod stacked;
pub mod stem_leaf;
pub mod stream;
pub mod sunburst;
pub mod timeline;
pub mod timetable;
pub mod tree;
pub mod treemap;
pub mod unit;
pub mod venn;
pub mod word_cloud;

use crate::backend::html::{self, Options};
use crate::base::text::{InterMetrics, TextMeasure};
use crate::base::theme::SERIES_SLOTS;
use crate::error::Error;
use crate::scene::{Plot, Scene};

/// A chart: data plus the layout that turns it into a [`Scene`].
pub trait Chart {
    /// Lays the chart out at `width` × `height` pixels, sizing text with `measure`.
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error>;

    /// How many categories the x axis runs through, for charts that can
    /// show a window of them (see [`crate::zoom`]); `None` for the rest.
    fn category_count(&self) -> Option<usize> {
        None
    }

    /// This chart narrowed to the categories in `range`.
    fn window(&self, _range: std::ops::Range<usize>) -> Option<Box<dyn Chart>> {
        None
    }

    /// The scene and, for a chart whose x axis is continuous (time or
    /// numbers) and can zoom, where its plot sits (see [`crate::zoom::View`]);
    /// `None` for the rest.
    fn plot_scene(
        &self,
        width: f64,
        height: f64,
        measure: &dyn TextMeasure,
    ) -> Result<(Scene, Option<Plot>), Error> {
        Ok((self.scene(width, height, measure)?, None))
    }

    /// This chart showing x in `x` and, for a chart that zooms y too, y in
    /// `y`: its axes run exactly over them, and what falls outside is left
    /// out.
    fn view(&self, _x: (f64, f64), _y: Option<(f64, f64)>) -> Option<Box<dyn Chart>> {
        None
    }

    /// Renders interactive HTML (or SVG, per `opts.format`) using Inter metrics.
    fn render_html(&self, opts: &Options) -> Result<String, Error> {
        let scene = self.scene(opts.width, opts.height, &InterMetrics)?;
        Ok(html::render(&scene, opts))
    }
}

/// Any chart, tagged by `"type"` in JSON. The Go port reads the same JSON.
#[cfg(feature = "serde")]
#[derive(Clone, Debug, PartialEq, serde::Serialize, serde::Deserialize)]
#[serde(tag = "type", rename_all = "camelCase")]
pub enum ChartSpec {
    Bar(bar::BarChart),
    GroupedBar(grouped::GroupedBarChart),
    StackedBar(stacked::StackedBarChart),
    Line(line::LineChart),
    Area(line::AreaChart),
    StackedArea(line::StackedAreaChart),
    Scatter(point::ScatterChart),
    Bubble(point::BubbleChart),
    Heatmap(heatmap::HeatmapChart),
    Span(span::SpanChart),
    Pyramid(pyramid::PyramidChart),
    Bullet(bullet::BulletChart),
    ErrorBars(error_bars::ErrorBarChart),
    Histogram(histogram::HistogramChart),
    Density(density::DensityChart),
    BoxPlot(boxplot::BoxPlotChart),
    Violin(boxplot::ViolinChart),
    StemLeaf(stem_leaf::StemLeafChart),
    Parallel(parallel::ParallelChart),
    Candlestick(finance::CandlestickChart),
    Ohlc(finance::OhlcChart),
    Kagi(kagi::KagiChart),
    PointFigure(point_figure::PointFigureChart),
    Gantt(gantt::GanttChart),
    Timeline(timeline::TimelineChart),
    Stream(stream::StreamChart),
    Calendar(calendar::CalendarChart),
    Timetable(timetable::TimetableChart),
    Spiral(spiral::SpiralChart),
    Pie(pie::PieChart),
    Donut(pie::DonutChart),
    Rose(rose::RoseChart),
    Radar(radar::RadarChart),
    RadialBar(radial::RadialBarChart),
    RadialColumn(radial::RadialColumnChart),
    Treemap(treemap::TreemapChart),
    CirclePacking(packing::CirclePackingChart),
    Sunburst(sunburst::SunburstChart),
    Tree(tree::TreeChart),
    Marimekko(marimekko::MarimekkoChart),
    Brainstorm(tree::BrainstormChart),
    Network(network::NetworkChart),
    Arc(arc::ArcChart),
    Chord(chord::ChordChart),
    NonRibbonChord(chord::NonRibbonChordChart),
    Sankey(sankey::SankeyChart),
    ParallelSets(parallel_sets::ParallelSetsChart),
    FlowChart(flowchart::FlowChart),
    Choropleth(choropleth::ChoroplethChart),
    BubbleMap(point_map::BubbleMapChart),
    DotMap(point_map::DotMapChart),
    ConnectionMap(route_map::ConnectionMapChart),
    FlowMap(route_map::FlowMapChart),
    Pictogram(unit::PictogramChart),
    ProportionalArea(unit::ProportionalAreaChart),
    Tally(unit::TallyChart),
    DotMatrix(unit::DotMatrixChart),
    WordCloud(word_cloud::WordCloudChart),
    Venn(venn::VennChart),
}

#[cfg(feature = "serde")]
impl ChartSpec {
    fn inner(&self) -> &dyn Chart {
        match self {
            ChartSpec::Bar(c) => c,
            ChartSpec::GroupedBar(c) => c,
            ChartSpec::StackedBar(c) => c,
            ChartSpec::Line(c) => c,
            ChartSpec::Area(c) => c,
            ChartSpec::StackedArea(c) => c,
            ChartSpec::Scatter(c) => c,
            ChartSpec::Bubble(c) => c,
            ChartSpec::Heatmap(c) => c,
            ChartSpec::Span(c) => c,
            ChartSpec::Pyramid(c) => c,
            ChartSpec::Bullet(c) => c,
            ChartSpec::ErrorBars(c) => c,
            ChartSpec::Histogram(c) => c,
            ChartSpec::Density(c) => c,
            ChartSpec::BoxPlot(c) => c,
            ChartSpec::Violin(c) => c,
            ChartSpec::StemLeaf(c) => c,
            ChartSpec::Parallel(c) => c,
            ChartSpec::Candlestick(c) => c,
            ChartSpec::Ohlc(c) => c,
            ChartSpec::Kagi(c) => c,
            ChartSpec::PointFigure(c) => c,
            ChartSpec::Gantt(c) => c,
            ChartSpec::Timeline(c) => c,
            ChartSpec::Stream(c) => c,
            ChartSpec::Calendar(c) => c,
            ChartSpec::Timetable(c) => c,
            ChartSpec::Spiral(c) => c,
            ChartSpec::Pie(c) => c,
            ChartSpec::Donut(c) => c,
            ChartSpec::Rose(c) => c,
            ChartSpec::Radar(c) => c,
            ChartSpec::RadialBar(c) => c,
            ChartSpec::RadialColumn(c) => c,
            ChartSpec::Treemap(c) => c,
            ChartSpec::CirclePacking(c) => c,
            ChartSpec::Sunburst(c) => c,
            ChartSpec::Tree(c) => c,
            ChartSpec::Marimekko(c) => c,
            ChartSpec::Brainstorm(c) => c,
            ChartSpec::Network(c) => c,
            ChartSpec::Arc(c) => c,
            ChartSpec::Chord(c) => c,
            ChartSpec::NonRibbonChord(c) => c,
            ChartSpec::Sankey(c) => c,
            ChartSpec::ParallelSets(c) => c,
            ChartSpec::FlowChart(c) => c,
            ChartSpec::Choropleth(c) => c,
            ChartSpec::BubbleMap(c) => c,
            ChartSpec::DotMap(c) => c,
            ChartSpec::ConnectionMap(c) => c,
            ChartSpec::FlowMap(c) => c,
            ChartSpec::Pictogram(c) => c,
            ChartSpec::ProportionalArea(c) => c,
            ChartSpec::Tally(c) => c,
            ChartSpec::DotMatrix(c) => c,
            ChartSpec::WordCloud(c) => c,
            ChartSpec::Venn(c) => c,
        }
    }
}

#[cfg(feature = "serde")]
impl Chart for ChartSpec {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        self.inner().scene(width, height, measure)
    }

    fn category_count(&self) -> Option<usize> {
        self.inner().category_count()
    }

    fn window(&self, range: std::ops::Range<usize>) -> Option<Box<dyn Chart>> {
        self.inner().window(range)
    }

    fn plot_scene(
        &self,
        width: f64,
        height: f64,
        measure: &dyn TextMeasure,
    ) -> Result<(Scene, Option<Plot>), Error> {
        self.inner().plot_scene(width, height, measure)
    }

    fn view(&self, x: (f64, f64), y: Option<(f64, f64)>) -> Option<Box<dyn Chart>> {
        self.inner().view(x, y)
    }
}

/// The ranges a zoomed chart shows: x, and y for charts that zoom it.
pub(crate) type Ranges = ((f64, f64), Option<(f64, f64)>);

/// A chart with continuous axes that lays itself out over given ranges.
/// [`viewable!`] turns it into a [`Chart`] that zooms.
pub(crate) trait Viewable: Clone + 'static {
    /// The scene and its plot, over `view` or, when `None`, the full data.
    fn laid_out(
        &self,
        width: f64,
        height: f64,
        measure: &dyn TextMeasure,
        view: Option<Ranges>,
    ) -> Result<(Scene, Plot), Error>;
}

/// A chart shown over fixed ranges, what [`Chart::view`] returns.
struct InView<C> {
    chart: C,
    view: Ranges,
}

impl<C: Viewable> Chart for InView<C> {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        Ok(self
            .chart
            .laid_out(width, height, measure, Some(self.view))?
            .0)
    }

    fn plot_scene(
        &self,
        width: f64,
        height: f64,
        measure: &dyn TextMeasure,
    ) -> Result<(Scene, Option<Plot>), Error> {
        let (s, p) = self
            .chart
            .laid_out(width, height, measure, Some(self.view))?;
        Ok((s, Some(p)))
    }

    fn view(&self, x: (f64, f64), y: Option<(f64, f64)>) -> Option<Box<dyn Chart>> {
        Some(Box::new(InView {
            chart: self.chart.clone(),
            view: (x, y),
        }))
    }
}

/// `impl Chart` for a [`Viewable`] chart.
macro_rules! viewable {
    ($name:ty) => {
        impl Chart for $name {
            fn scene(
                &self,
                width: f64,
                height: f64,
                measure: &dyn TextMeasure,
            ) -> Result<Scene, Error> {
                Ok(self.laid_out(width, height, measure, None)?.0)
            }

            fn plot_scene(
                &self,
                width: f64,
                height: f64,
                measure: &dyn TextMeasure,
            ) -> Result<(Scene, Option<crate::scene::Plot>), Error> {
                let (s, p) = self.laid_out(width, height, measure, None)?;
                Ok((s, Some(p)))
            }

            fn view(&self, x: (f64, f64), y: Option<(f64, f64)>) -> Option<Box<dyn Chart>> {
                Some(Box::new(super::InView {
                    chart: self.clone(),
                    view: (x, y),
                }))
            }
        }
    };
}
pub(crate) use viewable;

/// One named series of values, one per category.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Series {
    pub name: String,
    pub values: Vec<f64>,
}

impl Series {
    pub fn new(name: impl Into<String>, values: impl IntoIterator<Item = f64>) -> Self {
        Series {
            name: name.into(),
            values: values.into_iter().collect(),
        }
    }
}

/// A value at an instant (UTC epoch ms).
#[derive(Clone, Copy, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct TimeValue {
    pub time: i64,
    pub value: f64,
}

impl TimeValue {
    pub fn new(time: i64, value: f64) -> Self {
        TimeValue { time, value }
    }
}

/// Values must be finite and times strictly increasing.
pub(crate) fn check_times(data: &[TimeValue]) -> Result<(), Error> {
    for (i, d) in data.iter().enumerate() {
        finite(&format!("value {i}"), d.value)?;
        if i > 0 && d.time <= data[i - 1].time {
            return Err(Error::InvalidData(format!(
                "value {i}: times must increase"
            )));
        }
    }
    Ok(())
}

/// A named sample of raw observations, summarised by a statistical chart.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Sample {
    pub name: String,
    pub values: Vec<f64>,
}

impl Sample {
    pub fn new(name: impl Into<String>, values: impl IntoIterator<Item = f64>) -> Self {
        Sample {
            name: name.into(),
            values: values.into_iter().collect(),
        }
    }

    pub(crate) fn sorted(&self) -> Vec<f64> {
        let mut v = self.values.clone();
        v.sort_by(f64::total_cmp);
        v
    }
}

/// Samples need distinct names, at least one value each, all finite. `what`
/// names them in errors ("series", "group").
pub(crate) fn check_samples(samples: &[Sample], max: usize, what: &str) -> Result<(), Error> {
    if samples.len() > max {
        return Err(Error::InvalidData(format!(
            "{} {what}, at most {max}: fold the rest into \"Other\" or use small multiples",
            samples.len()
        )));
    }
    for (j, s) in samples.iter().enumerate() {
        if samples[..j].iter().any(|o| o.name == s.name) {
            return Err(Error::InvalidData(format!(
                "{what} name {:?} is used twice",
                s.name
            )));
        }
        if s.values.is_empty() {
            return Err(Error::InvalidData(format!(
                "{what} {:?} has no values",
                s.name
            )));
        }
        if s.values.iter().any(|v| !v.is_finite()) {
            return Err(Error::InvalidData(format!(
                "{what} {:?} has a non-finite value",
                s.name
            )));
        }
    }
    Ok(())
}

/// Series must line up with the categories, be finite, have distinct names
/// (a name is the series' identity for highlight and hide) and fit the
/// palette: a ninth series is never a generated hue.
pub(crate) fn check_series(
    categories: &[String],
    series: &[Series],
    max: usize,
) -> Result<(), Error> {
    if series.len() > max {
        return Err(Error::InvalidData(format!(
            "{} series, at most {max}: fold the rest into \"Other\" or use small multiples",
            series.len()
        )));
    }
    for (j, s) in series.iter().enumerate() {
        if series[..j].iter().any(|o| o.name == s.name) {
            return Err(Error::InvalidData(format!(
                "series name {:?} is used twice",
                s.name
            )));
        }
        if s.values.len() != categories.len() {
            return Err(Error::InvalidData(format!(
                "series {:?} has {} values for {} categories",
                s.name,
                s.values.len(),
                categories.len()
            )));
        }
        if let Some(i) = s.values.iter().position(|v| !v.is_finite()) {
            return Err(Error::InvalidData(format!(
                "series {:?} has a non-finite value at {:?}",
                s.name, categories[i]
            )));
        }
    }
    Ok(())
}

pub(crate) const MAX_SERIES: usize = SERIES_SLOTS as usize;

pub(crate) fn finite(what: &str, v: f64) -> Result<(), Error> {
    if v.is_finite() {
        Ok(())
    } else {
        Err(Error::InvalidData(format!("{what} is not finite")))
    }
}

// Layout constants shared by charts (px). Mark sizes follow the dataviz
// skill's mark specs; the Go port uses the same values.
pub(crate) const PAD: f64 = 16.0;
pub(crate) const GAP: f64 = 8.0;
pub(crate) const TITLE_SIZE: f64 = 14.0;
pub(crate) const TICK_SIZE: f64 = 11.0;
/// Target vertical distance between value-axis ticks.
pub(crate) const TICK_SPACING: f64 = 50.0;
pub(crate) const HAIRLINE: f64 = 1.0;
/// Series lines and whiskers.
pub(crate) const LINE_WIDTH: f64 = 2.0;
/// Markers are at least 8px across.
pub(crate) const DOT_RADIUS: f64 = 4.0;
/// The surface-colored gap between touching marks, and the ring around dots.
pub(crate) const SURFACE_GAP: f64 = 2.0;
/// Small marks still get a hover target at least this wide.
pub(crate) const MIN_TARGET: f64 = 24.0;

pub(crate) fn check_size(width: f64, height: f64) -> Result<(), Error> {
    if width.is_finite() && height.is_finite() && width > 0.0 && height > 0.0 {
        Ok(())
    } else {
        Err(Error::InvalidSize)
    }
}

/// The accessible name: the title, if any, then what kind of chart it is.
pub(crate) fn describe(title: Option<&str>, what: &str) -> String {
    match title {
        Some(t) if !t.is_empty() => format!("{t}. {what}"),
        _ => what.to_string(),
    }
}

/// "1 category", "3 categories". The plural is spelled out because English
/// plurals are irregular ("series", "categories").
pub(crate) fn count(n: usize, one: &str, many: &str) -> String {
    format!("{n} {}", if n == 1 { one } else { many })
}

/// Centers a 1px line on a pixel so it renders crisp instead of as a 2px blur.
pub(crate) fn snap(v: f64) -> f64 {
    v.floor() + 0.5
}
