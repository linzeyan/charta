//! Writes a page with one interactive chart to stdout: tooltips, hover
//! highlight and legend toggles run in the page, without a server.
//!
//! Usage: cargo run --example html > chart.html

use charta::{BarChart, Chart, Options};

fn main() {
    let chart = BarChart::new([("Apples", 42.0), ("Pears", 18.0), ("Figs", 33.0)])
        .title("Fruit sold this week");
    // The default Options is 640×400 and follows the viewer's light or dark
    // scheme. The result is a <div> to embed: it carries its own styles and
    // script.
    let div = chart.render_html(&Options::default()).expect("valid chart");
    println!(
        "<!doctype html>\n<meta charset=\"utf-8\">\n<title>Fruit sold this week</title>\n{div}"
    );
}
