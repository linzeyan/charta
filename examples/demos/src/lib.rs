//! The demo charts every native gallery shows, shared by the apps and their
//! headless tests so the tests exercise exactly what a person sees.

use charta::base::time::{DAY, to_ms};
use charta::geo::{Projection, world};
use charta::{
    ArcChart, AreaChart, BarChart, BoxPlotChart, BrainstormChart, BubbleChart, BubbleMapChart,
    Bullet, BulletChart, CalendarChart, Candle, CandlestickChart, Chart, ChordChart,
    ChoroplethChart, CirclePackingChart, ConnectionMapChart, Curve, DensityChart, DonutChart,
    DotMapChart, DotMatrixChart, ErrorBarChart, Event, FlowChart, FlowMapChart, GanttChart,
    GroupedBarChart, HeatmapChart, HistogramChart, KagiChart, LineChart, MarimekkoChart,
    NetworkChart, Node, NonRibbonChordChart, OhlcChart, ParallelChart, ParallelRow,
    ParallelSetsChart, PictogramChart, PieChart, Point, PointFigureChart, ProportionalAreaChart,
    PyramidChart, RadarChart, RadialBarChart, RadialColumnChart, RoseChart, SankeyChart,
    ScatterChart, Slot, SpanChart, SpiralChart, StackedAreaChart, StackedBarChart, StemLeafChart,
    StepKind, StreamChart, SunburstChart, TallyChart, Task, TimeValue, TimelineChart,
    TimetableChart, TreeChart, TreemapChart, VennChart, ViolinChart, WordCloudChart,
};

const QUARTERS: [&str; 4] = ["Q1", "Q2", "Q3", "Q4"];
const MONTHS: [&str; 6] = ["Jan", "Feb", "Mar", "Apr", "May", "Jun"];
const WAITS: [f64; 60] = [
    30.0, 37.0, 30.0, 29.0, 24.0, 30.0, 42.0, 36.0, 41.0, 34.0, 36.0, 34.0, 17.0, 40.0, 37.0, 36.0,
    17.0, 16.0, 24.0, 28.0, 35.0, 32.0, 37.0, 26.0, 35.0, 36.0, 26.0, 47.0, 37.0, 43.0, 26.0, 25.0,
    29.0, 31.0, 38.0, 34.0, 28.0, 23.0, 27.0, 43.0, 25.0, 34.0, 36.0, 19.0, 32.0, 44.0, 14.0, 29.0,
    31.0, 25.0, 36.0, 31.0, 19.0, 39.0, 38.0, 41.0, 45.0, 35.0, 33.0, 20.0,
];
const EU_MS: [f64; 50] = [
    138.0, 117.0, 85.0, 145.0, 118.0, 109.0, 127.0, 127.0, 147.0, 102.0, 140.0, 147.0, 146.0,
    117.0, 107.0, 138.0, 122.0, 122.0, 146.0, 115.0, 79.0, 113.0, 87.0, 135.0, 126.0, 109.0, 120.0,
    135.0, 121.0, 144.0, 119.0, 139.0, 147.0, 149.0, 108.0, 136.0, 86.0, 100.0, 85.0, 139.0, 98.0,
    120.0, 117.0, 119.0, 109.0, 124.0, 152.0, 121.0, 130.0, 138.0,
];
const US_MS: [f64; 50] = [
    145.0, 119.0, 136.0, 177.0, 109.0, 135.0, 175.0, 170.0, 150.0, 170.0, 154.0, 121.0, 111.0,
    134.0, 173.0, 136.0, 127.0, 131.0, 112.0, 147.0, 121.0, 159.0, 91.0, 158.0, 134.0, 101.0,
    168.0, 143.0, 94.0, 128.0, 157.0, 139.0, 169.0, 169.0, 167.0, 158.0, 183.0, 166.0, 161.0, 98.0,
    172.0, 183.0, 143.0, 138.0, 199.0, 106.0, 162.0, 211.0, 127.0, 167.0,
];
const YIELD_NONE: [f64; 40] = [
    5.7, 4.3, 4.5, 3.6, 3.9, 4.4, 6.4, 2.8, 3.4, 5.3, 6.6, 5.6, 2.9, 2.2, 5.4, 4.2, 3.8, 6.1, 6.2,
    5.2, 5.3, 5.5, 6.8, 5.7, 5.6, 5.6, 3.3, 6.4, 6.1, 5.6, 2.8, 4.3, 5.9, 3.0, 4.8, 6.1, 3.6, 6.8,
    5.6, 4.8,
];
const YIELD_ORGANIC: [f64; 40] = [
    6.7, 6.9, 6.5, 7.3, 5.9, 6.1, 7.2, 6.4, 5.7, 7.2, 7.6, 6.0, 5.3, 6.3, 6.3, 6.2, 7.5, 5.6, 7.4,
    5.4, 5.8, 6.9, 7.3, 7.1, 6.7, 6.5, 6.5, 6.9, 6.3, 6.6, 6.9, 6.4, 7.0, 6.9, 8.0, 6.7, 6.1, 6.1,
    6.4, 7.1,
];
const YIELD_MINERAL: [f64; 41] = [
    3.6, 4.7, 7.0, -0.0, 2.3, 4.5, 4.7, 4.5, 3.4, 5.1, 4.6, 3.3, 8.0, 4.7, 3.2, 3.9, 3.7, 4.0,
    -0.3, 3.3, 5.7, 2.2, 4.0, 5.6, 5.5, 6.5, 1.4, 3.5, 3.6, 5.1, 5.8, -0.2, 5.8, 1.8, 5.2, 1.7,
    4.4, 6.0, 3.9, 4.4, 11.5,
];
const SCORES: [f64; 30] = [
    53.0, 53.0, 53.0, 57.0, 57.0, 59.0, 59.0, 60.0, 60.0, 67.0, 68.0, 70.0, 71.0, 73.0, 73.0, 74.0,
    78.0, 79.0, 79.0, 80.0, 80.0, 81.0, 82.0, 84.0, 86.0, 89.0, 89.0, 90.0, 90.0, 99.0,
];

pub fn demos() -> Vec<(&'static str, Box<dyn Chart>)> {
    vec![
        (
            "Bar",
            Box::new(
                BarChart::new([
                    ("Apples", 42.0),
                    ("Pears", 18.0),
                    ("Cherries", 27.5),
                    ("Plums", -6.0),
                    ("Figs", 33.0),
                ])
                .title("Fruit sold this week"),
            ),
        ),
        (
            "Grouped bar",
            Box::new(
                GroupedBarChart::new(QUARTERS)
                    .title("Revenue by region")
                    .series("North", [12.0, 15.0, 14.0, 19.0])
                    .series("South", [9.0, 11.0, 16.0, 13.0])
                    .series("West", [6.0, 8.0, 7.0, 11.0]),
            ),
        ),
        (
            "Stacked bar",
            Box::new(
                StackedBarChart::new(QUARTERS)
                    .title("Tickets by channel")
                    .series("Email", [40.0, 35.0, 42.0, 38.0])
                    .series("Chat", [22.0, 30.0, 33.0, 41.0])
                    .series("Phone", [15.0, 12.0, 10.0, 8.0]),
            ),
        ),
        ("Line", Box::new(line())),
        (
            "Area",
            Box::new(
                AreaChart::new(MONTHS)
                    .title("Active users (k)")
                    .series("Users", [3.2, 4.1, 4.0, 5.6, 6.3, 7.1])
                    .curve(Curve::Monotone),
            ),
        ),
        (
            "Stacked area",
            Box::new(
                StackedAreaChart::new(MONTHS)
                    .title("Traffic by source (k)")
                    .series("Search", [12.0, 14.0, 13.0, 16.0, 18.0, 21.0])
                    .series("Social", [5.0, 7.0, 9.0, 8.0, 10.0, 12.0])
                    .series("Direct", [4.0, 4.0, 5.0, 6.0, 6.0, 7.0]),
            ),
        ),
        (
            "Scatter",
            Box::new(
                ScatterChart::new()
                    .title("Height vs weight")
                    .axes("Height (cm)", "Weight (kg)")
                    .series(
                        "Adults",
                        vec![
                            Point::new(160.0, 55.0),
                            Point::new(168.0, 62.0),
                            Point::new(175.0, 70.0),
                            Point::new(182.0, 81.0),
                            Point::new(171.0, 66.0),
                        ],
                    )
                    .series(
                        "Teens",
                        vec![
                            Point::new(150.0, 42.0),
                            Point::new(158.0, 49.0),
                            Point::new(165.0, 54.0),
                        ],
                    ),
            ),
        ),
        (
            "Bubble",
            Box::new(
                BubbleChart::new()
                    .title("Markets")
                    .axes("Growth (%)", "Margin (%)")
                    .series(
                        "Markets",
                        vec![
                            Point::new(2.0, 12.0).size(40.0).label("EU"),
                            Point::new(6.5, 9.0).size(90.0).label("Asia"),
                            Point::new(4.0, 18.0).size(25.0).label("US"),
                            Point::new(8.0, 6.0).size(10.0).label("LatAm"),
                        ],
                    ),
            ),
        ),
        (
            "Heatmap",
            Box::new(
                HeatmapChart::new(
                    ["Mon", "Tue", "Wed", "Thu", "Fri"],
                    ["Morning", "Noon", "Evening"],
                    vec![
                        vec![12.0, 18.0, 15.0, 20.0, 9.0],
                        vec![30.0, 34.0, 28.0, 41.0, 25.0],
                        vec![22.0, 19.0, 26.0, 31.0, 38.0],
                    ],
                )
                .title("Orders by time of day"),
            ),
        ),
        (
            "Span",
            Box::new(
                SpanChart::new([
                    ("Mon", 12.0, 21.0),
                    ("Tue", 9.0, 18.0),
                    ("Wed", 14.0, 25.0),
                    ("Thu", 11.0, 19.0),
                ])
                .title("Daily temperature range (°C)"),
            ),
        ),
        (
            "Population pyramid",
            Box::new(
                PyramidChart::new(["0–19", "20–39", "40–59", "60–79", "80+"])
                    .title("Population by age (k)")
                    .sides(
                        ("Male", vec![210.0, 260.0, 240.0, 150.0, 40.0]),
                        ("Female", vec![200.0, 255.0, 245.0, 170.0, 65.0]),
                    ),
            ),
        ),
        (
            "Bullet",
            Box::new(
                BulletChart::new(vec![
                    Bullet::new("Revenue (k$)", 270.0)
                        .target(250.0)
                        .ranges([150.0, 225.0, 300.0]),
                    Bullet::new("Profit (%)", 22.5)
                        .target(26.0)
                        .ranges([20.0, 25.0, 30.0]),
                    Bullet::new("New customers", 1650.0)
                        .target(2100.0)
                        .ranges([1400.0, 2000.0, 2500.0]),
                ])
                .title("Q3 against target"),
            ),
        ),
        (
            "Error bars",
            Box::new(
                ErrorBarChart::new([
                    ("Control", 4.2, 3.6, 4.9),
                    ("Dose A", 5.8, 5.1, 6.4),
                    ("Dose B", 7.1, 6.0, 8.3),
                ])
                .title("Response by treatment"),
            ),
        ),
        (
            "Histogram",
            Box::new(
                HistogramChart::new(WAITS)
                    .title("Support wait times")
                    .x_label("Minutes"),
            ),
        ),
        (
            "Density",
            Box::new(
                DensityChart::new()
                    .title("Response time by region")
                    .x_label("ms")
                    .series("EU", EU_MS)
                    .series("US", US_MS),
            ),
        ),
        (
            "Box plot",
            Box::new(
                BoxPlotChart::new()
                    .title("Yield by fertilizer")
                    .y_label("t/ha")
                    .group("None", YIELD_NONE)
                    .group("Organic", YIELD_ORGANIC)
                    .group("Mineral", YIELD_MINERAL),
            ),
        ),
        (
            "Violin",
            Box::new(
                ViolinChart::new()
                    .title("Yield by fertilizer")
                    .y_label("t/ha")
                    .group("None", YIELD_NONE)
                    .group("Organic", YIELD_ORGANIC)
                    .group("Mineral", YIELD_MINERAL),
            ),
        ),
        (
            "Stem and leaf",
            Box::new(StemLeafChart::new(SCORES).title("Exam scores")),
        ),
        ("Parallel coordinates", Box::new(laptops())),
        (
            "Candlestick",
            Box::new(CandlestickChart::new(candles()).title("ACME daily")),
        ),
        (
            "OHLC",
            Box::new(OhlcChart::new(candles()).title("ACME daily")),
        ),
        (
            "Kagi",
            Box::new(KagiChart::new(closes()).title("ACME close")),
        ),
        (
            "Point and figure",
            Box::new(PointFigureChart::new(closes()).title("ACME close")),
        ),
        ("Gantt", Box::new(gantt())),
        ("Timeline", Box::new(timeline())),
        ("Stream graph", Box::new(stream())),
        ("Calendar", Box::new(calendar())),
        ("Timetable", Box::new(timetable())),
        ("Spiral", Box::new(spiral())),
        (
            "Pie",
            Box::new(
                PieChart::new([
                    ("Chrome", 64.7),
                    ("Safari", 18.6),
                    ("Edge", 5.2),
                    ("Firefox", 2.8),
                    ("Other", 8.7),
                ])
                .title("Browser share (%)"),
            ),
        ),
        (
            "Donut",
            Box::new(
                DonutChart::new([
                    ("Engineering", 420.0),
                    ("Sales", 260.0),
                    ("Marketing", 180.0),
                    ("Support", 90.0),
                    ("Admin", 50.0),
                ])
                .title("Budget by department (k$)"),
            ),
        ),
        (
            "Nightingale rose",
            Box::new(
                RoseChart::new(MONTH_NAMES)
                    .title("Causes of mortality")
                    .series(
                        "Disease",
                        [
                            120.0, 150.0, 180.0, 210.0, 260.0, 320.0, 380.0, 410.0, 350.0, 280.0,
                            200.0, 150.0,
                        ],
                    )
                    .series(
                        "Wounds",
                        [
                            20.0, 25.0, 30.0, 40.0, 60.0, 80.0, 90.0, 85.0, 70.0, 50.0, 35.0, 25.0,
                        ],
                    )
                    .series(
                        "Other",
                        [
                            30.0, 28.0, 32.0, 35.0, 40.0, 45.0, 50.0, 48.0, 42.0, 38.0, 34.0, 30.0,
                        ],
                    ),
            ),
        ),
        (
            "Radar",
            Box::new(
                RadarChart::new(["Speed", "Range", "Comfort", "Safety", "Price", "Efficiency"])
                    .title("Model comparison")
                    .series("Model A", [8.0, 6.0, 7.0, 9.0, 5.0, 7.0])
                    .series("Model B", [6.0, 9.0, 6.0, 7.0, 8.0, 8.0]),
            ),
        ),
        (
            "Radial bar",
            Box::new(
                RadialBarChart::new([
                    ("Running", 82.0),
                    ("Cycling", 64.0),
                    ("Swimming", 45.0),
                    ("Yoga", 30.0),
                ])
                .title("Goal progress (%)"),
            ),
        ),
        (
            "Radial column",
            Box::new(
                RadialColumnChart::new(MONTH_NAMES.into_iter().zip([
                    78.0, 62.0, 55.0, 48.0, 61.0, 84.0, 102.0, 96.0, 73.0, 88.0, 95.0, 86.0,
                ]))
                .title("Rainfall by month (mm)"),
            ),
        ),
        (
            "Treemap",
            Box::new(TreemapChart::new(revenue()).title("Revenue by product line (M$)")),
        ),
        (
            "Circle packing",
            Box::new(CirclePackingChart::new(teams()).title("Team sizes")),
        ),
        (
            "Sunburst",
            Box::new(SunburstChart::new(disk()).title("Disk usage (GB)")),
        ),
        (
            "Tree diagram",
            Box::new(TreeChart::new(org()).title("Organization")),
        ),
        (
            "Marimekko",
            Box::new(
                MarimekkoChart::new(["North America", "Europe", "Asia", "Other"])
                    .title("Sales by region and vendor (M$)")
                    .series("Vendor A", [120.0, 80.0, 60.0, 15.0])
                    .series("Vendor B", [60.0, 90.0, 110.0, 10.0])
                    .series("Vendor C", [40.0, 30.0, 90.0, 25.0]),
            ),
        ),
        (
            "Brainstorm",
            Box::new(BrainstormChart::new(launch_plan()).title("Launch plan")),
        ),
        (
            "Network",
            Box::new(
                NETWORK
                    .iter()
                    .fold(NetworkChart::new(), |c, &(s, t, v)| c.link(s, t, v))
                    .title("Who works with whom"),
            ),
        ),
        (
            "Arc",
            Box::new(
                PAPERS
                    .iter()
                    .fold(ArcChart::new(), |c, &(s, t, v)| c.link(s, t, v))
                    .title("Co-authored papers"),
            ),
        ),
        (
            "Chord",
            Box::new(
                MIGRATION
                    .iter()
                    .fold(ChordChart::new(), |c, &(s, t, v)| c.link(s, t, v))
                    .title("Migration between regions (k people)"),
            ),
        ),
        (
            "Non-ribbon chord",
            Box::new(
                DEPS.iter()
                    .fold(NonRibbonChordChart::new(), |c, &(s, t, v)| c.link(s, t, v))
                    .title("Package dependencies"),
            ),
        ),
        (
            "Sankey",
            Box::new(
                ENERGY
                    .iter()
                    .fold(SankeyChart::new(), |c, &(s, t, v)| c.link(s, t, v))
                    .title("Energy flows (TWh)"),
            ),
        ),
        ("Parallel sets", Box::new(titanic())),
        ("Flow chart", Box::new(order_flow())),
        (
            "Choropleth",
            Box::new(
                LIFE.iter()
                    .fold(ChoroplethChart::new(world()), |c, &(id, v)| c.value(id, v))
                    .title("Life expectancy at birth (years)"),
            ),
        ),
        (
            "Bubble map",
            Box::new(
                CITIES
                    .iter()
                    .fold(BubbleMapChart::new(world()), |c, &(n, lon, lat, v)| {
                        c.place(n, lon, lat, v)
                    })
                    .title("Largest urban areas (M people)"),
            ),
        ),
        (
            "Dot map",
            Box::new(
                QUAKES
                    .iter()
                    .fold(DotMapChart::new(world()), |c, &(lon, lat, g)| {
                        c.dot(lon, lat, g)
                    })
                    .projection(Projection::Equirectangular)
                    .title("Strong earthquakes (sample)"),
            ),
        ),
        (
            "Connection map",
            Box::new(
                FLIGHTS
                    .iter()
                    .fold(
                        AIRPORTS
                            .iter()
                            .fold(ConnectionMapChart::new(world()), |c, &(n, lon, lat)| {
                                c.place(n, lon, lat)
                            }),
                        |c, &(s, t, v)| c.link(s, t, v),
                    )
                    .projection(Projection::Mercator)
                    .title("Weekly nonstop flights from Singapore"),
            ),
        ),
        (
            "Flow map",
            Box::new(
                COFFEE
                    .iter()
                    .fold(
                        COFFEE_PLACES
                            .iter()
                            .fold(FlowMapChart::new(world()), |c, &(n, lon, lat)| {
                                c.place(n, lon, lat)
                            }),
                        |c, &(s, t, v)| c.link(s, t, v),
                    )
                    .title("Green coffee exports (kt)"),
            ),
        ),
        (
            "Pictogram",
            Box::new(
                PictogramChart::new(
                    [
                        ("Lisbon", 46.0),
                        ("Porto", 32.0),
                        ("Braga", 18.0),
                        ("Faro", 7.0),
                    ],
                    10.0,
                )
                .title("Volunteers by city"),
            ),
        ),
        (
            "Proportional area",
            Box::new(
                ProportionalAreaChart::new([
                    ("Russia", 17.1),
                    ("Canada", 9.98),
                    ("China", 9.6),
                    ("India", 3.29),
                    ("Japan", 0.378),
                ])
                .title("Land area (M km²)"),
            ),
        ),
        (
            "Tally",
            Box::new(
                TallyChart::new([
                    ("Robin", 12.0),
                    ("Sparrow", 23.0),
                    ("Blue tit", 7.0),
                    ("Magpie", 4.0),
                    ("Wren", 0.0),
                ])
                .title("Birds spotted this morning"),
            ),
        ),
        (
            "Dot matrix",
            Box::new(
                DotMatrixChart::new(
                    [
                        ("Greens", 42.0),
                        ("Reds", 31.0),
                        ("Blues", 15.0),
                        ("Yellows", 8.0),
                        ("Independents", 4.0),
                    ],
                    1.0,
                )
                .title("Seats won"),
            ),
        ),
        (
            "Word cloud",
            Box::new(
                WORDS
                    .iter()
                    .fold(WordCloudChart::new(), |c, &(w, v)| c.word(w, v))
                    .title("Words in chart reviews"),
            ),
        ),
        (
            "Venn",
            Box::new(
                VennChart::new()
                    .area(["English"], 120.0)
                    .area(["Spanish"], 70.0)
                    .area(["French"], 45.0)
                    .area(["English", "Spanish"], 30.0)
                    .area(["English", "French"], 20.0)
                    .area(["Spanish", "French"], 8.0)
                    .area(["English", "Spanish", "French"], 5.0)
                    .title("Languages spoken (people)"),
            ),
        ),
    ]
}

const NETWORK: [(&str, &str, f64); 20] = [
    ("Ana", "Ben", 3.0),
    ("Ana", "Cleo", 2.0),
    ("Ben", "Cleo", 2.0),
    ("Ana", "Dev", 1.0),
    ("Cleo", "Dev", 1.0),
    ("Eli", "Fay", 3.0),
    ("Eli", "Gus", 2.0),
    ("Fay", "Gus", 1.0),
    ("Fay", "Hal", 2.0),
    ("Gus", "Hal", 1.0),
    ("Ivy", "Jon", 2.0),
    ("Ivy", "Kai", 2.0),
    ("Jon", "Kai", 1.0),
    ("Kai", "Lea", 3.0),
    ("Jon", "Lea", 1.0),
    ("Lea", "Max", 1.0),
    ("Max", "Ivy", 1.0),
    ("Dev", "Eli", 1.0),
    ("Hal", "Ivy", 1.0),
    ("Cleo", "Kai", 1.0),
];

const PAPERS: [(&str, &str, f64); 11] = [
    ("Kim", "Lee", 4.0),
    ("Kim", "Park", 2.0),
    ("Lee", "Park", 3.0),
    ("Park", "Choi", 1.0),
    ("Choi", "Jung", 2.0),
    ("Jung", "Kang", 3.0),
    ("Kang", "Yoon", 1.0),
    ("Kim", "Yoon", 2.0),
    ("Lee", "Kang", 1.0),
    ("Choi", "Yoon", 1.0),
    ("Jung", "Han", 2.0),
];

const MIGRATION: [(&str, &str, f64); 14] = [
    ("North", "South", 30.0),
    ("South", "North", 12.0),
    ("North", "East", 18.0),
    ("East", "North", 9.0),
    ("East", "West", 15.0),
    ("West", "East", 22.0),
    ("West", "South", 25.0),
    ("South", "West", 8.0),
    ("North", "West", 10.0),
    ("South", "East", 14.0),
    ("Central", "North", 16.0),
    ("North", "Central", 6.0),
    ("Central", "South", 11.0),
    ("East", "Central", 7.0),
];

const DEPS: [(&str, &str, f64); 16] = [
    ("core", "utils", 2.0),
    ("core", "io", 2.0),
    ("core", "log", 1.0),
    ("io", "net", 1.0),
    ("io", "fs", 2.0),
    ("net", "http", 2.0),
    ("http", "json", 1.0),
    ("http", "auth", 1.0),
    ("auth", "crypto", 2.0),
    ("fs", "path", 1.0),
    ("log", "format", 1.0),
    ("json", "format", 1.0),
    ("cli", "core", 2.0),
    ("cli", "format", 1.0),
    ("test", "core", 1.0),
    ("test", "utils", 1.0),
];

const ENERGY: [(&str, &str, f64); 9] = [
    ("Coal", "Electricity", 120.0),
    ("Gas", "Electricity", 90.0),
    ("Gas", "Homes", 60.0),
    ("Gas", "Industry", 50.0),
    ("Nuclear", "Electricity", 80.0),
    ("Renewables", "Electricity", 70.0),
    ("Electricity", "Homes", 110.0),
    ("Electricity", "Industry", 130.0),
    ("Electricity", "Losses", 120.0),
];

const LIFE: [(&str, f64); 50] = [
    ("USA", 78.5),
    ("CAN", 82.2),
    ("MEX", 76.0),
    ("BRA", 75.9),
    ("ARG", 76.6),
    ("CHL", 80.7),
    ("COL", 79.3),
    ("PER", 79.9),
    ("VEN", 73.9),
    ("BOL", 73.1),
    ("GBR", 81.4),
    ("FRA", 82.5),
    ("DEU", 81.7),
    ("ESP", 83.2),
    ("ITA", 83.0),
    ("PRT", 81.6),
    ("NOR", 82.6),
    ("SWE", 82.4),
    ("FIN", 81.6),
    ("POL", 78.3),
    ("UKR", 73.0),
    ("RUS", 73.2),
    ("TUR", 78.6),
    ("EGY", 71.8),
    ("NGA", 62.6),
    ("ETH", 68.7),
    ("KEN", 66.1),
    ("ZAF", 65.3),
    ("COD", 62.4),
    ("AGO", 63.1),
    ("TZA", 66.2),
    ("DZA", 77.1),
    ("MAR", 73.0),
    ("SAU", 75.1),
    ("IRN", 77.3),
    ("PAK", 65.6),
    ("IND", 70.8),
    ("CHN", 77.4),
    ("JPN", 84.3),
    ("KOR", 83.3),
    ("IDN", 71.3),
    ("PHL", 70.4),
    ("VNM", 73.7),
    ("THA", 77.7),
    ("AUS", 83.0),
    ("NZL", 82.0),
    ("MNG", 68.1),
    ("KAZ", 73.2),
    ("AFG", 63.2),
    ("BGD", 74.3),
];

const CITIES: [(&str, f64, f64, f64); 24] = [
    ("Tokyo", 139.69, 35.69, 37.4),
    ("Delhi", 77.21, 28.61, 31.0),
    ("Shanghai", 121.47, 31.23, 27.1),
    ("São Paulo", -46.63, -23.55, 22.0),
    ("Mexico City", -99.13, 19.43, 21.8),
    ("Cairo", 31.24, 30.04, 21.3),
    ("Dhaka", 90.41, 23.81, 21.0),
    ("Mumbai", 72.88, 19.08, 20.7),
    ("Beijing", 116.41, 39.9, 20.5),
    ("Osaka", 135.5, 34.69, 19.2),
    ("New York", -74.01, 40.71, 18.8),
    ("Karachi", 67.01, 24.86, 16.1),
    ("Buenos Aires", -58.38, -34.6, 15.2),
    ("Istanbul", 28.98, 41.01, 15.2),
    ("Lagos", 3.38, 6.52, 14.4),
    ("Kinshasa", 15.27, -4.44, 14.3),
    ("Manila", 120.98, 14.6, 13.9),
    ("Rio de Janeiro", -43.17, -22.91, 13.5),
    ("Moscow", 37.62, 55.76, 12.5),
    ("Los Angeles", -118.24, 34.05, 12.5),
    ("Paris", 2.35, 48.86, 11.0),
    ("Jakarta", 106.85, -6.21, 10.8),
    ("Lima", -77.04, -12.05, 10.7),
    ("London", -0.13, 51.51, 9.5),
];

const QUAKES: [(f64, f64, &str); 80] = [
    (142.8, 36.4, "Shallow"),
    (143.5, 41.9, "Shallow"),
    (145.8, 41.8, "Shallow"),
    (140.7, 36.7, "Deep"),
    (145.2, 40.3, "Shallow"),
    (138.1, 37.5, "Deep"),
    (142.7, 35.8, "Shallow"),
    (142.5, 35.2, "Deep"),
    (120.4, -5.4, "Deep"),
    (120.5, -2.5, "Shallow"),
    (119.4, -5.8, "Deep"),
    (117.1, -2.3, "Deep"),
    (121.1, -8.2, "Shallow"),
    (123.3, -4.1, "Shallow"),
    (116.2, -2.0, "Shallow"),
    (119.8, -5.7, "Shallow"),
    (-73.7, -33.2, "Shallow"),
    (-68.5, -32.1, "Shallow"),
    (-70.2, -27.0, "Shallow"),
    (-72.4, -33.9, "Shallow"),
    (-72.8, -26.2, "Shallow"),
    (-69.8, -29.2, "Shallow"),
    (-70.7, -30.8, "Deep"),
    (-74.0, -31.9, "Deep"),
    (-149.8, 56.0, "Shallow"),
    (-151.9, 58.1, "Deep"),
    (-148.7, 62.8, "Shallow"),
    (-154.2, 56.6, "Shallow"),
    (-148.1, 63.5, "Deep"),
    (-153.5, 59.0, "Deep"),
    (-149.9, 56.5, "Shallow"),
    (-154.5, 56.3, "Deep"),
    (-96.0, 16.1, "Shallow"),
    (-98.6, 14.0, "Shallow"),
    (-96.6, 15.2, "Shallow"),
    (-98.6, 18.4, "Shallow"),
    (-102.7, 13.7, "Shallow"),
    (-101.5, 14.0, "Deep"),
    (-102.8, 18.9, "Deep"),
    (-102.2, 20.3, "Deep"),
    (122.4, 12.0, "Shallow"),
    (124.9, 8.5, "Shallow"),
    (124.7, 11.5, "Shallow"),
    (124.6, 6.4, "Deep"),
    (124.8, 11.2, "Deep"),
    (122.0, 13.1, "Shallow"),
    (122.9, 8.4, "Shallow"),
    (128.5, 8.6, "Deep"),
    (173.2, -40.6, "Deep"),
    (175.8, -37.1, "Shallow"),
    (173.1, -42.8, "Deep"),
    (175.2, -38.5, "Shallow"),
    (175.1, -39.5, "Deep"),
    (177.8, -39.2, "Shallow"),
    (174.9, -43.2, "Shallow"),
    (174.4, -38.4, "Shallow"),
    (54.3, 33.8, "Shallow"),
    (54.4, 27.1, "Deep"),
    (52.9, 26.2, "Shallow"),
    (55.5, 27.3, "Shallow"),
    (51.0, 32.8, "Shallow"),
    (52.2, 30.0, "Shallow"),
    (56.9, 27.2, "Shallow"),
    (55.6, 30.4, "Shallow"),
    (36.2, 38.8, "Deep"),
    (33.6, 41.1, "Shallow"),
    (38.5, 39.2, "Deep"),
    (31.8, 38.3, "Shallow"),
    (33.0, 41.0, "Shallow"),
    (36.4, 41.3, "Shallow"),
    (38.1, 35.6, "Deep"),
    (32.9, 34.4, "Deep"),
    (-75.0, -13.3, "Shallow"),
    (-79.8, -10.0, "Deep"),
    (-72.1, -8.6, "Deep"),
    (-78.7, -8.1, "Shallow"),
    (-75.9, -11.9, "Shallow"),
    (-79.3, -12.6, "Shallow"),
    (-76.5, -9.6, "Shallow"),
    (-74.3, -9.0, "Shallow"),
];

const AIRPORTS: [(&str, f64, f64); 11] = [
    ("Singapore", 103.82, 1.35),
    ("London", -0.13, 51.51),
    ("Sydney", 151.21, -33.87),
    ("Tokyo", 139.69, 35.69),
    ("Dubai", 55.27, 25.2),
    ("Mumbai", 72.88, 19.08),
    ("Beijing", 116.41, 39.9),
    ("San Francisco", -122.42, 37.77),
    ("Johannesburg", 28.05, -26.2),
    ("Frankfurt", 8.68, 50.11),
    ("Auckland", 174.76, -36.85),
];

const FLIGHTS: [(&str, &str, f64); 10] = [
    ("Singapore", "London", 28.0),
    ("Singapore", "Sydney", 35.0),
    ("Singapore", "Tokyo", 42.0),
    ("Singapore", "Dubai", 21.0),
    ("Singapore", "Mumbai", 18.0),
    ("Singapore", "Beijing", 24.0),
    ("Singapore", "San Francisco", 7.0),
    ("Singapore", "Johannesburg", 5.0),
    ("Singapore", "Frankfurt", 14.0),
    ("Singapore", "Auckland", 9.0),
];

const COFFEE_PLACES: [(&str, f64, f64); 7] = [
    ("Brazil", -51.9, -14.2),
    ("Vietnam", 108.3, 14.1),
    ("Colombia", -74.3, 4.6),
    ("Ethiopia", 40.5, 9.1),
    ("USA", -98.6, 39.8),
    ("Germany", 10.5, 51.2),
    ("Japan", 138.3, 36.2),
];

const COFFEE: [(&str, &str, f64); 8] = [
    ("Brazil", "USA", 400.0),
    ("Brazil", "Germany", 300.0),
    ("Vietnam", "Germany", 200.0),
    ("Vietnam", "USA", 150.0),
    ("Colombia", "USA", 250.0),
    ("Ethiopia", "Germany", 80.0),
    ("Brazil", "Japan", 120.0),
    ("Vietnam", "Japan", 100.0),
];

const WORDS: [(&str, f64); 35] = [
    ("data", 90.0),
    ("chart", 72.0),
    ("color", 60.0),
    ("scale", 55.0),
    ("axis", 50.0),
    ("legend", 44.0),
    ("tooltip", 40.0),
    ("label", 38.0),
    ("grid", 33.0),
    ("series", 31.0),
    ("bar", 29.0),
    ("line", 27.0),
    ("area", 25.0),
    ("hover", 23.0),
    ("dark mode", 22.0),
    ("contrast", 20.0),
    ("palette", 19.0),
    ("marker", 17.0),
    ("baseline", 16.0),
    ("tick", 15.0),
    ("zoom", 14.0),
    ("brush", 13.0),
    ("small multiples", 12.0),
    ("annotation", 11.0),
    ("density", 10.0),
    ("outlier", 9.0),
    ("trend", 9.0),
    ("ramp", 8.0),
    ("hue", 8.0),
    ("pixel", 7.0),
    ("layout", 7.0),
    ("glyph", 6.0),
    ("texture", 6.0),
    ("focus", 5.0),
    ("story", 5.0),
];

fn titanic() -> ParallelSetsChart {
    [
        ("First", "Male", "No", 118.0),
        ("First", "Male", "Yes", 62.0),
        ("First", "Female", "No", 4.0),
        ("First", "Female", "Yes", 141.0),
        ("Second", "Male", "No", 154.0),
        ("Second", "Male", "Yes", 25.0),
        ("Second", "Female", "No", 13.0),
        ("Second", "Female", "Yes", 93.0),
        ("Third", "Male", "No", 422.0),
        ("Third", "Male", "Yes", 88.0),
        ("Third", "Female", "No", 106.0),
        ("Third", "Female", "Yes", 90.0),
        ("Crew", "Male", "No", 670.0),
        ("Crew", "Male", "Yes", 192.0),
        ("Crew", "Female", "No", 3.0),
        ("Crew", "Female", "Yes", 20.0),
    ]
    .iter()
    .fold(
        ParallelSetsChart::new(["Class", "Sex", "Survived"]).title("Titanic passengers"),
        |c, &(class, sex, survived, n)| c.combination([class, sex, survived], n),
    )
}

fn order_flow() -> FlowChart {
    FlowChart::new()
        .title("Order handling")
        .step("order", StepKind::Terminal, "Order placed")
        .step("stock", StepKind::Decision, "In stock?")
        .step("pack", StepKind::Process, "Pack")
        .step("reorder", StepKind::Process, "Reorder")
        .step("ship", StepKind::Process, "Ship")
        .step("wait", StepKind::Process, "Wait for stock")
        .step("done", StepKind::Terminal, "Delivered")
        .arrow("order", "stock", "")
        .arrow("stock", "pack", "Yes")
        .arrow("stock", "reorder", "No")
        .arrow("pack", "ship", "")
        .arrow("reorder", "wait", "")
        .arrow("wait", "stock", "")
        .arrow("ship", "done", "")
}

fn leaves(pairs: &[(&str, f64)]) -> Vec<Node> {
    pairs.iter().map(|&(n, v)| Node::leaf(n, v)).collect()
}

fn revenue() -> Node {
    Node::branch(
        "Revenue",
        vec![
            Node::branch(
                "Software",
                leaves(&[
                    ("Cloud", 42.0),
                    ("Licenses", 18.0),
                    ("Support", 12.0),
                    ("Training", 4.0),
                ]),
            ),
            Node::branch(
                "Hardware",
                leaves(&[("Servers", 28.0), ("Storage", 16.0), ("Networking", 9.0)]),
            ),
            Node::branch(
                "Services",
                leaves(&[("Consulting", 22.0), ("Managed", 14.0)]),
            ),
        ],
    )
}

fn teams() -> Node {
    Node::branch(
        "Company",
        vec![
            Node::branch(
                "Engineering",
                leaves(&[
                    ("Frontend", 12.0),
                    ("Backend", 18.0),
                    ("Mobile", 8.0),
                    ("Data", 10.0),
                    ("Platform", 6.0),
                ]),
            ),
            Node::branch(
                "Product",
                leaves(&[("PM", 6.0), ("Design", 9.0), ("Research", 3.0)]),
            ),
            Node::branch(
                "Go-to-market",
                leaves(&[("Sales", 15.0), ("Marketing", 8.0), ("Support", 11.0)]),
            ),
        ],
    )
}

fn disk() -> Node {
    Node::branch(
        "Disk",
        vec![
            Node::branch(
                "Documents",
                vec![
                    Node::branch("Work", leaves(&[("Reports", 12.0), ("Slides", 8.0)])),
                    Node::leaf("Personal", 6.0),
                ],
            ),
            Node::branch(
                "Media",
                leaves(&[("Photos", 48.0), ("Videos", 72.0), ("Music", 20.0)]),
            ),
            Node::branch(
                "Apps",
                leaves(&[("System", 35.0), ("Games", 40.0), ("Tools", 10.0)]),
            ),
        ],
    )
}

/// A node with children named by `kids`, each a bare leaf.
fn topic(name: &str, kids: &[&str]) -> Node {
    Node::branch(name, kids.iter().map(|&k| Node::new(k)).collect())
}

fn org() -> Node {
    Node::branch(
        "CEO",
        vec![
            Node::branch(
                "CTO",
                vec![
                    Node::new("Platform"),
                    topic("Product eng", &["Web", "Mobile"]),
                    Node::new("Data"),
                ],
            ),
            topic("CFO", &["Finance", "Legal"]),
            topic("COO", &["Sales", "Support", "Operations"]),
        ],
    )
}

fn launch_plan() -> Node {
    Node::branch(
        "Launch plan",
        vec![
            topic("Marketing", &["Social", "Email", "Ads"]),
            topic("Product", &["Features", "Pricing", "Docs"]),
            topic("Sales", &["Training", "Partners"]),
            topic("Support", &["FAQ", "Onboarding"]),
        ],
    )
}

const MONTH_NAMES: [&str; 12] = [
    "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
];

/// A fixed pseudo-random sequence (the classic C `rand` LCG): demo series
/// look organic yet stay identical run to run, and the fixture generator
/// reproduces them exactly because it is all integer arithmetic.
struct Lcg(u64);

impl Lcg {
    fn below(&mut self, n: u64) -> i64 {
        self.0 = (self.0 * 1_103_515_245 + 12_345) % (1 << 31);
        (self.0 % n) as i64
    }
}

/// Six weeks of trading days from Monday 2 March 2026, prices in cents.
fn candles() -> Vec<Candle> {
    let mut r = Lcg(7);
    let mut price = 10_000;
    let start = to_ms(2026, 3, 2, 0, 0, 0);
    (0..42)
        .filter(|d| d % 7 < 5)
        .map(|d| {
            let open = price;
            let close = open + r.below(401) - 200;
            let high = open.max(close) + r.below(150);
            let low = open.min(close) - r.below(150);
            price = close;
            let c = |v: i64| v as f64 / 100.0;
            Candle::new(start + d * DAY, c(open), c(high), c(low), c(close))
        })
        .collect()
}

/// 150 daily closes from 1 January 2026, trending up and down by month.
fn closes() -> Vec<TimeValue> {
    let mut r = Lcg(11);
    let mut price = 5_000;
    let start = to_ms(2026, 1, 1, 0, 0, 0);
    (0..150)
        .map(|d| {
            let drift = if (d / 30) % 2 == 0 { 8 } else { -8 };
            price += r.below(301) - 150 + drift;
            TimeValue::new(start + d * DAY, price as f64 / 100.0)
        })
        .collect()
}

fn gantt() -> GanttChart {
    let day = |m, d| to_ms(2026, m, d, 0, 0, 0);
    GanttChart::new(vec![
        Task::new("Research", day(1, 5), day(1, 17)).group("Design"),
        Task::new("Wireframes", day(1, 12), day(1, 31)).group("Design"),
        Task::new("Visual design", day(1, 26), day(2, 21)).group("Design"),
        Task::new("Backend", day(2, 2), day(3, 14)).group("Build"),
        Task::new("Frontend", day(2, 16), day(3, 21)).group("Build"),
        Task::new("QA", day(3, 16), day(4, 1)).group("Launch"),
        Task::new("Go live", day(4, 1), day(4, 1)).group("Launch"),
    ])
    .title("Website relaunch")
}

fn timeline() -> TimelineChart {
    let at = |y, m, d| to_ms(y, m, d, 0, 0, 0);
    TimelineChart::new(vec![
        Event::new(at(2019, 3, 14), "Founded"),
        Event::new(at(2019, 11, 2), "First prototype"),
        Event::new(at(2020, 6, 20), "Seed round"),
        Event::new(at(2021, 2, 10), "Beta launch"),
        Event::new(at(2021, 9, 1), "1.0 release"),
        Event::new(at(2022, 5, 17), "10k users"),
        Event::new(at(2023, 8, 8), "Series A"),
        Event::new(at(2024, 4, 22), "Mobile app"),
        Event::new(at(2025, 10, 30), "1M users"),
    ])
    .title("Company milestones")
}

fn stream() -> StreamChart {
    StreamChart::new((1..=12).map(|m| to_ms(2025, m, 1, 0, 0, 0)))
        .title("Listening hours by genre (k)")
        .series(
            "Pop",
            [
                22.0, 24.0, 23.0, 27.0, 30.0, 36.0, 41.0, 39.0, 32.0, 28.0, 26.0, 31.0,
            ],
        )
        .series(
            "Rock",
            [
                18.0, 17.0, 19.0, 18.0, 16.0, 15.0, 17.0, 18.0, 19.0, 21.0, 20.0, 19.0,
            ],
        )
        .series(
            "Hip-hop",
            [
                9.0, 11.0, 13.0, 16.0, 18.0, 21.0, 24.0, 25.0, 22.0, 20.0, 19.0, 21.0,
            ],
        )
        .series(
            "Jazz",
            [6.0, 6.0, 5.0, 5.0, 4.0, 4.0, 4.0, 5.0, 6.0, 7.0, 8.0, 10.0],
        )
}

/// Commits per day through 2025, quieter at weekends.
fn calendar() -> CalendarChart {
    let mut r = Lcg(3);
    let start = to_ms(2025, 1, 1, 0, 0, 0);
    CalendarChart::new(
        (0..365)
            .map(|d| {
                // 1 January 2025 was a Wednesday (2 days after Monday).
                let v = r.below(10);
                let v = if (d + 2) % 7 >= 5 { v / 3 } else { v };
                TimeValue::new(start + d * DAY, v as f64)
            })
            .collect(),
    )
    .title("Commits per day, 2025")
}

fn timetable() -> TimetableChart {
    let slot = |day, (h0, m0): (u32, u32), (h1, m1): (u32, u32), label, group| {
        Slot::new(day, h0 * 60 + m0, h1 * 60 + m1, label).group(group)
    };
    [
        slot("Mon", (9, 0), (10, 30), "Maths", "Lecture"),
        slot("Mon", (11, 0), (12, 0), "Physics", "Lecture"),
        slot("Mon", (14, 0), (16, 0), "Chemistry", "Lab"),
        slot("Tue", (9, 0), (10, 0), "English", "Seminar"),
        slot("Tue", (10, 30), (12, 0), "Maths", "Lecture"),
        slot("Wed", (13, 0), (15, 0), "Physics", "Lab"),
        slot("Wed", (14, 0), (15, 0), "Tutorial", "Seminar"),
        slot("Thu", (9, 0), (10, 30), "Maths", "Lecture"),
        slot("Thu", (11, 0), (12, 30), "History", "Seminar"),
        slot("Fri", (10, 0), (12, 0), "Biology", "Lab"),
        slot("Fri", (13, 0), (14, 0), "Physics", "Lecture"),
    ]
    .into_iter()
    .fold(
        TimetableChart::new(["Mon", "Tue", "Wed", "Thu", "Fri"]).title("Class timetable"),
        TimetableChart::slot,
    )
}

/// Monthly mean temperature, 2023–2025: a seasonal cycle in tenths of a
/// degree, warming a little each year.
fn spiral() -> SpiralChart {
    const BASE: [i64; 12] = [40, 50, 85, 120, 160, 195, 220, 215, 180, 130, 80, 50];
    const NAMES: [&str; 12] = MONTH_NAMES;
    let mut r = Lcg(5);
    let data: Vec<(String, f64)> = (0..36)
        .map(|i| {
            let (y, m) = (i / 12, i % 12);
            let t = BASE[m] + y as i64 * 4 + r.below(21) - 10;
            (format!("{} {}", NAMES[m], 2023 + y), t as f64 / 10.0)
        })
        .collect();
    SpiralChart::new(12, data)
        .title("Monthly mean temperature (°C)")
        .angle_labels(NAMES)
}

fn laptops() -> ParallelChart {
    let rows = [
        ("Air 13", "Ultrabook", [1099.0, 1.24, 18.0, 4.7]),
        ("Zen 14", "Ultrabook", [899.0, 1.39, 14.0, 4.3]),
        ("XPS 13", "Ultrabook", [1199.0, 1.17, 12.0, 4.5]),
        ("Legion 5", "Gaming", [1299.0, 2.4, 5.0, 4.4]),
        ("ROG 16", "Gaming", [1799.0, 2.5, 6.0, 4.6]),
        ("Nitro 5", "Gaming", [849.0, 2.6, 4.0, 4.0]),
        ("Chromebook", "Budget", [299.0, 1.5, 10.0, 3.8]),
        ("IdeaPad 3", "Budget", [449.0, 1.65, 7.0, 3.9]),
    ];
    rows.into_iter().fold(
        ParallelChart::new(["Price ($)", "Weight (kg)", "Battery (h)", "Rating"]).title("Laptops"),
        |c, (label, group, values)| c.row(ParallelRow::new(label, values).group(group)),
    )
}

/// The multi-series line demo, exposed on its own because the tests drive
/// its crosshair and legend.
pub fn line() -> LineChart {
    LineChart::new(MONTHS)
        .title("Weekly signups")
        .series("Web", [120.0, 135.0, 128.0, 160.0, 172.0, 190.0])
        .series("iOS", [80.0, 95.0, 110.0, 108.0, 125.0, 140.0])
        .series("Android", [60.0, 70.0, 66.0, 85.0, 90.0, 104.0])
}
