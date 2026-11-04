//! Writes every gallery demo's scene and HTML to `<out>/<NN>-<name>.{scene.json,html}`
//! for `make test-conformance` to diff against the Go port's
//! `internal/cmd/demos`, and for a chart with continuous axes also the
//! middle half of it, zoomed, to `<NN>-<name>.zoomed.scene.json`. The demos
//! live in the repo (unlike the fixtures in docs/), so CI can hold the two
//! ports to identical output.
//!
//! Usage: cargo run -p charta-demos --example dump -- <out-dir>

use std::{env, fs, path::Path, process::ExitCode};

use charta::Options;
use charta::base::text::InterMetrics;
use charta_demos::demos;

fn main() -> ExitCode {
    let args: Vec<String> = env::args().collect();
    let [_, out] = args.as_slice() else {
        eprintln!("usage: dump <out-dir>");
        return ExitCode::from(2);
    };
    let out = Path::new(out);
    fs::create_dir_all(out).expect("create out dir");
    let mut failed = false;
    for (i, (name, chart)) in demos().into_iter().enumerate() {
        let stem = out.join(format!("{i:02}-{}", slug(name)));
        let html = Options::default();
        let result = chart
            .scene(html.width, html.height, &InterMetrics)
            .map_err(|e| e.to_string())
            .and_then(|s| {
                let page = chart.render_html(&html).map_err(|e| e.to_string())?;
                fs::write(stem.with_extension("scene.json"), s.dump())
                    .map_err(|e| e.to_string())?;
                fs::write(stem.with_extension("html"), page).map_err(|e| e.to_string())?;
                let Some(plot) = chart
                    .plot_scene(html.width, html.height, &InterMetrics)
                    .map_err(|e| e.to_string())?
                    .1
                else {
                    return Ok(());
                };
                let mid = |(a, b): (f64, f64)| (a + (b - a) / 4.0, b - (b - a) / 4.0);
                let zoomed = chart
                    .view(mid(plot.x), plot.y.map(mid))
                    .ok_or("a chart with a plot has no view")?
                    .scene(html.width, html.height, &InterMetrics)
                    .map_err(|e| e.to_string())?;
                fs::write(stem.with_extension("zoomed.scene.json"), zoomed.dump())
                    .map_err(|e| e.to_string())
            });
        if let Err(e) = result {
            eprintln!("{name}: {e}");
            failed = true;
        }
    }
    if failed {
        ExitCode::FAILURE
    } else {
        ExitCode::SUCCESS
    }
}

/// Lowercase ASCII letters and digits, anything else a dash: the same file
/// names as the Go runner's.
fn slug(name: &str) -> String {
    name.bytes()
        .map(|b| {
            if b.is_ascii_alphanumeric() {
                b.to_ascii_lowercase() as char
            } else {
                '-'
            }
        })
        .collect()
}
