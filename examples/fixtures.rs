//! Renders every fixture `<name>.json` in a directory to `<out>/<name>.scene.json`
//! (for the Rust/Go conformance diff) and `<out>/<name>.html` (for the gallery).
//!
//! Usage: cargo run --example fixtures --features serde -- <fixtures-dir> <out-dir>
//!
//! A fixture is `{"width": 640, "height": 400, "chart": {"type": "bar", ...}}`.

use std::{env, fs, path::Path, process::ExitCode};

use charta::base::text::InterMetrics;
use charta::{Chart, ChartSpec, Options};

fn main() -> ExitCode {
    let args: Vec<String> = env::args().collect();
    let [_, src, out] = args.as_slice() else {
        eprintln!("usage: fixtures <fixtures-dir> <out-dir>");
        return ExitCode::from(2);
    };
    fs::create_dir_all(out).expect("create out dir");
    let mut paths: Vec<_> = fs::read_dir(src)
        .expect("read fixtures dir")
        .map(|e| e.expect("dir entry").path())
        .filter(|p| p.extension().is_some_and(|x| x == "json"))
        .collect();
    paths.sort();
    let mut failed = false;
    for path in paths {
        if let Err(e) = render(&path, Path::new(out)) {
            eprintln!("{}: {e}", path.display());
            failed = true;
        }
    }
    if failed {
        ExitCode::FAILURE
    } else {
        ExitCode::SUCCESS
    }
}

fn render(path: &Path, out: &Path) -> Result<(), String> {
    let name = path
        .file_stem()
        .and_then(|s| s.to_str())
        .ok_or("bad file name")?;
    let text = fs::read_to_string(path).map_err(|e| e.to_string())?;
    let v: serde_json::Value = serde_json::from_str(&text).map_err(|e| e.to_string())?;
    let dim = |k: &str| v[k].as_f64().ok_or(format!("missing number {k:?}"));
    let (width, height) = (dim("width")?, dim("height")?);
    let chart: ChartSpec = serde_json::from_value(v["chart"].clone()).map_err(|e| e.to_string())?;
    let scene = chart
        .scene(width, height, &InterMetrics)
        .map_err(|e| e.to_string())?;
    let html = chart
        .render_html(&Options {
            width,
            height,
            ..Options::default()
        })
        .map_err(|e| e.to_string())?;
    fs::write(out.join(format!("{name}.scene.json")), scene.dump()).map_err(|e| e.to_string())?;
    fs::write(out.join(format!("{name}.html")), html).map_err(|e| e.to_string())
}
