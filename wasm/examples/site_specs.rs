//! Checks the Pages gallery's zoom specs against the wasm build's code path:
//! each `<stem>.json` that internal/cmd/site wrote must lay out, through
//! `ZoomChart`, to the `<stem>.html` the page shows. The Go port wrote the
//! JSON from its own structs, so this is where a field the Rust serde shape
//! reads differently would surface, before a visitor's first zoom swaps in
//! another chart.
//!
//! Usage: cargo run -p charta-wasm --example site_specs -- <specs-dir>

use std::path::PathBuf;
use std::process::ExitCode;

use charta_wasm::ZoomChart;

fn main() -> ExitCode {
    let Some(dir) = std::env::args().nth(1).map(PathBuf::from) else {
        eprintln!("usage: site_specs <specs-dir>");
        return ExitCode::from(2);
    };
    let mut specs: Vec<_> = match std::fs::read_dir(&dir) {
        Ok(entries) => entries
            .filter_map(|e| Some(e.ok()?.path()))
            .filter(|p| p.extension().is_some_and(|x| x == "json"))
            .collect(),
        Err(e) => {
            eprintln!("{}: {e}", dir.display());
            return ExitCode::from(2);
        }
    };
    specs.sort();
    if specs.is_empty() {
        eprintln!("no specs in {}", dir.display());
        return ExitCode::FAILURE;
    }
    let mut failed = 0;
    for spec in &specs {
        let page = std::fs::read_to_string(spec.with_extension("html")).unwrap_or_default();
        let json = std::fs::read_to_string(spec).unwrap_or_default();
        let verdict = match ZoomChart::build(&json, 640.0, 400.0, "auto") {
            Ok(z) if !z.zooms() => "does not zoom in Rust".to_string(),
            Ok(z) if z.html() != page => "lays out to other HTML than the page".to_string(),
            Ok(_) => "same".to_string(),
            Err(e) => format!("does not parse in Rust: {e}"),
        };
        if verdict != "same" {
            failed += 1;
        }
        println!(
            "{:<28} {verdict}",
            spec.file_name().unwrap().to_string_lossy()
        );
    }
    if failed > 0 {
        eprintln!("{failed} of {} specs differ", specs.len());
        return ExitCode::FAILURE;
    }
    ExitCode::SUCCESS
}
