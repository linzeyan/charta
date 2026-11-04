//! charta's layout engine as WebAssembly, so a chart rendered to HTML (by
//! either port; the markup is the same) can zoom in the browser: zooming
//! lays the chart out again, which the page's small runtime cannot. The
//! gestures live in `zoom.js`; this crate keeps the chart, its zoom and the
//! scene on screen, and hands back the HTML to show.
//!
//! Positions are scene coordinates, the SVG's viewBox units. Gesture
//! methods return what changed: 0 nothing, 1 the brush shading (redraw it
//! from [`ZoomChart::brush`]), 2 the zoom (show [`ZoomChart::html`]).

use charta::backend::gestures::{Change, Gestures};
use charta::backend::html;
use charta::base::text::InterMetrics;
use charta::scene::{HitKind, Scene};
use charta::{ChartSpec, Format, Options, Theme};
use wasm_bindgen::prelude::*;

/// A chart as rendered to HTML, zoomable.
#[wasm_bindgen]
pub struct ZoomChart {
    chart: ChartSpec,
    opts: Options,
    gestures: Gestures,
    scene: Scene,
}

#[wasm_bindgen]
impl ZoomChart {
    /// `spec` is the JSON the chart was rendered from; `width`, `height`
    /// and `theme` (`"auto"`, `"light"` or `"dark"`) the options it was
    /// rendered with.
    #[wasm_bindgen(constructor)]
    pub fn new(spec: &str, width: f64, height: f64, theme: &str) -> Result<ZoomChart, JsError> {
        Self::build(spec, width, height, theme).map_err(|e| JsError::new(&e))
    }

    /// Whether the chart zooms: it has a category axis to narrow or a
    /// continuous one. When not, leave the wheel to the page.
    pub fn zooms(&self) -> bool {
        self.gestures.zooms()
    }

    /// The chart's HTML as zoomed. Unzoomed, it is what charta rendered,
    /// byte for byte.
    pub fn html(&self) -> String {
        html::render(&self.scene, &self.opts)
    }

    /// The primary button went down; `shift` makes a drag from here a
    /// brush.
    pub fn pressed(&mut self, x: f64, y: f64, shift: bool) {
        self.gestures.pressed((x, y), shift);
    }

    pub fn moved(&mut self, x: f64, y: f64) -> Result<u8, JsError> {
        let c = self.gestures.moved(&self.scene, (x, y));
        self.apply(c).map_err(|e| JsError::new(&e))
    }

    pub fn released(&mut self, x: f64, y: f64) -> Result<u8, JsError> {
        let c = self.gestures.released(&self.scene, (x, y));
        self.apply(c).map_err(|e| JsError::new(&e))
    }

    /// The press ended without a release; a brush is dropped.
    pub fn cancelled(&mut self) -> u8 {
        self.gestures.cancelled() as u8
    }

    /// The wheel turned by `notches` (positive zooms in) at (x, y).
    pub fn wheel(&mut self, x: f64, y: f64, notches: f64) -> Result<u8, JsError> {
        let c = self.gestures.wheel(&self.scene, (x, y), notches);
        self.apply(c).map_err(|e| JsError::new(&e))
    }

    /// A double click at (x, y): shows everything again, unless it was on
    /// a legend entry, which toggled its series twice.
    pub fn reset(&mut self, x: f64, y: f64) -> Result<u8, JsError> {
        self.double_click(x, y).map_err(|e| JsError::new(&e))
    }

    /// Whether the last press was a drag: the click the browser fires for
    /// its release is none.
    pub fn dragged(&self) -> bool {
        self.gestures.dragged()
    }

    /// The brush under way as `[x, y, width, height]`; empty when none.
    pub fn brush(&self) -> Vec<f64> {
        self.gestures
            .brush(&self.scene)
            .map_or(Vec::new(), |r| vec![r.x, r.y, r.w, r.h])
    }
}

impl ZoomChart {
    /// [`ZoomChart::new`] for Rust callers: the error as text, since a
    /// `JsError` can only be made inside a JS host.
    pub fn build(spec: &str, width: f64, height: f64, theme: &str) -> Result<Self, String> {
        let chart: ChartSpec = serde_json::from_str(spec).map_err(|e| e.to_string())?;
        let theme = match theme {
            "auto" => Theme::Auto,
            "light" => Theme::Light,
            "dark" => Theme::Dark,
            t => return Err(format!("theme {t:?}: auto, light or dark")),
        };
        let opts = Options {
            width,
            height,
            theme,
            format: Format::Html,
        };
        let mut gestures = Gestures::new();
        // The measure charta's HTML is laid out with, so the zoomed chart
        // continues the one on the page.
        let scene = gestures
            .lay_out(&chart, width, height, &InterMetrics)
            .map_err(|e| e.to_string())?;
        Ok(ZoomChart {
            chart,
            opts,
            gestures,
            scene,
        })
    }

    fn double_click(&mut self, x: f64, y: f64) -> Result<u8, String> {
        let on_legend = self
            .scene
            .hit_test(x, y, &[])
            .and_then(|i| self.scene.marks[i].hit.as_ref())
            .is_some_and(|h| h.kind == HitKind::Legend);
        if on_legend {
            return Ok(Change::None as u8);
        }
        let c = self.gestures.reset();
        self.apply(c)
    }

    fn apply(&mut self, c: Change) -> Result<u8, String> {
        if c == Change::Zoom {
            self.scene = self
                .gestures
                .lay_out(
                    &self.chart,
                    self.opts.width,
                    self.opts.height,
                    &InterMetrics,
                )
                .map_err(|e| e.to_string())?;
        }
        Ok(c as u8)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use charta::Chart;

    const LINE: &str = r#"{"type": "line", "categories": ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug"],
        "series": [{"name": "Visits", "values": [10, 20, 15, 30, 25, 40, 35, 50]},
                   {"name": "Signups", "values": [4, 8, 6, 12, 10, 16, 14, 20]}]}"#;
    const SCATTER: &str = r#"{"type": "scatter", "series": [{"name": "Trials", "points": [
        {"x": 1, "y": 3}, {"x": 2, "y": 8}, {"x": 4, "y": 5}, {"x": 6, "y": 9}, {"x": 8, "y": 2}, {"x": 9, "y": 7}]}]}"#;
    const BAR: &str =
        r#"{"type": "bar", "data": [{"label": "A", "value": 3}, {"label": "B", "value": 5}]}"#;

    fn ok(r: Result<u8, JsError>) -> u8 {
        r.expect("the gesture failed")
    }

    fn page(spec: &str, theme: Theme) -> String {
        let chart: ChartSpec = serde_json::from_str(spec).unwrap();
        chart
            .render_html(&Options {
                theme,
                ..Options::default()
            })
            .unwrap()
    }

    #[test]
    fn unzoomed_it_is_the_page_charta_rendered() {
        // zoom.js swaps this HTML in for the page's: unzoomed, a swap must
        // change nothing, whichever chart and theme.
        for spec in [LINE, SCATTER, BAR] {
            for (name, theme) in [("auto", Theme::Auto), ("dark", Theme::Dark)] {
                let z = ZoomChart::build(spec, 640.0, 400.0, name).unwrap();
                assert!(z.html() == page(spec, theme), "{spec} in {name}");
            }
        }
        assert!(ZoomChart::build(LINE, 640.0, 400.0, "sepia").is_err());
        assert!(ZoomChart::build("{\"type\": \"nope\"}", 640.0, 400.0, "auto").is_err());
    }

    #[test]
    fn the_wheel_zooms_categories_and_a_double_click_shows_them_all() {
        let mut z = ZoomChart::build(LINE, 640.0, 400.0, "auto").unwrap();
        assert!(z.zooms());
        let full = z.html();
        assert_eq!(z.double_click(320.0, 200.0).unwrap(), 0, "nothing to reset");
        // Half a notch is not a category yet; with two more it is two.
        assert_eq!(ok(z.wheel(320.0, 200.0, 0.5)), 0);
        assert_eq!(ok(z.wheel(320.0, 200.0, 2.0)), 2);
        let zoomed = z.html();
        assert_ne!(zoomed, full);
        assert!(!zoomed.contains(">Jan<"), "Jan is still shown");
        // On a legend entry a double click is two toggles, not a reset.
        let legend = z
            .scene
            .marks
            .iter()
            .find_map(|m| m.hit.as_ref().filter(|h| h.kind == HitKind::Legend)?.area)
            .unwrap();
        let (lx, ly) = (legend.x + legend.w / 2.0, legend.y + legend.h / 2.0);
        assert_eq!(z.double_click(lx, ly).unwrap(), 0);
        assert_eq!(z.html(), zoomed);
        assert_eq!(z.double_click(320.0, 200.0).unwrap(), 2);
        assert_eq!(z.html(), full);
    }

    #[test]
    fn a_shift_drag_brushes_a_box_on_a_scatterplot() {
        let mut z = ZoomChart::build(SCATTER, 640.0, 400.0, "auto").unwrap();
        let full = z.html();
        z.pressed(200.0, 120.0, true);
        assert_eq!(ok(z.moved(203.0, 121.0)), 0, "inside the drag threshold");
        assert_eq!(ok(z.moved(320.0, 260.0)), 1);
        let b = z.brush();
        assert_eq!(b, [200.0, 120.0, 120.0, 140.0]);
        assert_eq!(ok(z.released(330.0, 280.0)), 2);
        assert!(z.dragged() && z.brush().is_empty());
        assert_ne!(z.html(), full);
        // A plain drag pans; a click is no drag.
        z.pressed(300.0, 200.0, false);
        assert_eq!(ok(z.moved(200.0, 200.0)), 2);
        assert_eq!(ok(z.released(200.0, 200.0)), 0);
        z.pressed(300.0, 200.0, false);
        assert_eq!(ok(z.released(300.0, 200.0)), 0);
        assert!(!z.dragged());
    }

    #[test]
    fn a_chart_without_a_zoomable_axis_takes_no_gestures() {
        let mut z = ZoomChart::build(BAR, 640.0, 400.0, "auto").unwrap();
        assert!(!z.zooms());
        assert_eq!(ok(z.wheel(320.0, 200.0, 3.0)), 0);
        z.pressed(300.0, 200.0, true);
        assert_eq!(ok(z.moved(100.0, 100.0)), 0);
        assert!(z.brush().is_empty());
    }
}
