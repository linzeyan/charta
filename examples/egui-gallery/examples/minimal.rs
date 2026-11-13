//! One interactive chart in an egui window: hover for tooltips, click legend
//! entries to hide a series, and zoom a chart with an axis by wheel, drag,
//! shift-drag and double click. The widget follows egui's light or dark
//! theme.
//!
//! Usage: cargo run -p charta-egui-gallery --example minimal

use charta::LineChart;
use charta::backend::egui::ChartView;
use eframe::egui;

fn main() -> eframe::Result {
    let chart = LineChart::new(["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"])
        .title("Weekly active users (k)")
        .series("Web", [120.0, 132.0, 101.0, 134.0, 90.0, 230.0, 210.0])
        .series("iOS", [220.0, 182.0, 191.0, 234.0, 290.0, 330.0, 310.0]);
    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default().with_inner_size([720.0, 440.0]),
        ..Default::default()
    };
    // The view keeps its hover, hidden series and zoom in egui's memory, so
    // building it anew each frame is fine.
    eframe::run_ui_native("charta in egui", options, move |ui, _frame| {
        egui::CentralPanel::default().show(ui, |ui| {
            ui.add(ChartView::new(&chart).size(ui.available_size()));
        });
    })
}
