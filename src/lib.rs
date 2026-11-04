//! Interactive charts rendered to HTML/SVG or drawn into native GUIs.
//!
//! Every chart builds a backend-agnostic [`scene::Scene`]; backends turn it
//! into HTML/SVG ([`backend::html`]) or paint it into a GUI toolkit
//! ([`backend::egui`], behind the `egui` feature). The Go module in this
//! repository is a line-for-line port and must produce identical scenes.
//!
//! ```
//! use charta::{BarChart, Chart, Options};
//!
//! let chart = BarChart::new([("Apples", 30.0), ("Pears", 80.0)]).title("Fruit sold");
//! let html = chart.render_html(&Options::default()).unwrap();
//! assert!(html.contains("Fruit sold"));
//! ```

pub mod backend;
pub mod base;
mod chart;
mod dmath;
mod error;
pub mod geo;
mod layout;
pub mod scene;
pub mod zoom;

pub use backend::html::{Format, Options};
pub use base::theme::Theme;
pub use chart::Chart;
#[cfg(feature = "serde")]
pub use chart::ChartSpec;
pub use chart::arc::ArcChart;
pub use chart::bar::{Bar, BarChart};
pub use chart::boxplot::{BoxPlotChart, ViolinChart};
pub use chart::bullet::{Bullet, BulletChart};
pub use chart::calendar::CalendarChart;
pub use chart::chord::{ChordChart, NonRibbonChordChart};
pub use chart::choropleth::{ChoroplethChart, Region};
pub use chart::density::DensityChart;
pub use chart::error_bars::{ErrorBar, ErrorBarChart};
pub use chart::finance::{Candle, CandlestickChart, OhlcChart};
pub use chart::flowchart::{Arrow, FlowChart, Step, StepKind};
pub use chart::gantt::{GanttChart, Task};
pub use chart::graph::Link;
pub use chart::grouped::GroupedBarChart;
pub use chart::heatmap::HeatmapChart;
pub use chart::hierarchy::Node;
pub use chart::histogram::HistogramChart;
pub use chart::icon::Icon;
pub use chart::kagi::KagiChart;
pub use chart::line::{AreaChart, Curve, LineChart, StackedAreaChart};
pub use chart::marimekko::MarimekkoChart;
pub use chart::network::NetworkChart;
pub use chart::packing::CirclePackingChart;
pub use chart::parallel::{ParallelChart, ParallelRow};
pub use chart::parallel_sets::{Combination, ParallelSetsChart};
pub use chart::pie::{DonutChart, PieChart};
pub use chart::point::{BubbleChart, Point, PointSeries, ScatterChart};
pub use chart::point_figure::PointFigureChart;
pub use chart::point_map::{BubbleMapChart, Dot, DotMapChart, Place};
pub use chart::pyramid::{PyramidChart, PyramidSide};
pub use chart::radar::RadarChart;
pub use chart::radial::{RadialBarChart, RadialColumnChart};
pub use chart::rose::RoseChart;
pub use chart::route_map::{ConnectionMapChart, FlowMapChart, Location};
pub use chart::sankey::SankeyChart;
pub use chart::span::{Span, SpanChart};
pub use chart::spiral::SpiralChart;
pub use chart::stacked::StackedBarChart;
pub use chart::stem_leaf::StemLeafChart;
pub use chart::stream::StreamChart;
pub use chart::sunburst::SunburstChart;
pub use chart::timeline::{Event, TimelineChart};
pub use chart::timetable::{Slot, TimetableChart};
pub use chart::tree::{BrainstormChart, TreeChart};
pub use chart::treemap::TreemapChart;
pub use chart::unit::{
    AreaShape, DotMatrixChart, PictogramChart, ProportionalAreaChart, TallyChart,
};
pub use chart::venn::{VennArea, VennChart};
pub use chart::word_cloud::{Word, WordCloudChart};
pub use chart::{Sample, Series, TimeValue};
pub use error::Error;
