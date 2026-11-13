//! Renders one chart to a PNG file with the system fonts, for reports, e-mail
//! and anywhere a picture goes. It needs no GUI toolkit.
//!
//! Usage: cargo run --example png --features png [-- chart.png]

use charta::BarChart;
use charta::backend::raster::render_png;

fn main() {
    let out = std::env::args()
        .nth(1)
        .unwrap_or_else(|| "chart.png".into());
    let chart = BarChart::new([("Apples", 42.0), ("Pears", 18.0), ("Figs", 33.0)])
        .title("Fruit sold this week");
    // 640×400 logical pixels at scale 2: sharp on a Retina display. The last
    // argument picks the dark palette.
    let png = render_png(&chart, 640.0, 400.0, 2.0, false).expect("valid chart");
    std::fs::write(&out, png).expect("write the PNG");
}
