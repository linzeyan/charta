//! Every chart charta can draw, as egui widgets. Run with `make run-egui`.

use charta::backend::egui::ChartView;
use charta_demos::demos;
use eframe::egui;

fn main() -> eframe::Result {
    let demos = demos();
    let mut pick = 0;

    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default().with_inner_size([920.0, 540.0]),
        ..Default::default()
    };
    eframe::run_ui_native("charta egui gallery", options, move |ui, _frame| {
        egui::Panel::left("charts").show(ui, |ui| {
            egui::widgets::global_theme_preference_buttons(ui);
            ui.separator();
            egui::ScrollArea::vertical().show(ui, |ui| {
                for (i, (name, _)) in demos.iter().enumerate() {
                    ui.selectable_value(&mut pick, i, *name);
                }
            });
        });
        egui::CentralPanel::default().show(ui, |ui| {
            ui.add(ChartView::new(demos[pick].1.as_ref()).size(egui::vec2(720.0, 440.0)));
        });
    })
}
