//! Zoom gestures from raw pointer events, for backends whose toolkit hands
//! over presses, moves and wheel turns rather than recognized drags: Slint,
//! and the web through charta-wasm. The wheel zooms around the pointer,
//! dragging pans, shift-dragging brushes a range (a box, on a chart that
//! zooms y too) to zoom to, and a reset shows everything again. The
//! arithmetic is [`crate::zoom`]'s, shared with every backend and the Go
//! port; egui and Gio recognize drags themselves and call it directly.
//!
//! Positions are scene coordinates. After a gesture reports
//! [`Change::Zoom`], lay the chart out again with [`Gestures::lay_out`]
//! before the next one: gestures map through the scene it returned.

use crate::Chart;
use crate::base::text::TextMeasure;
use crate::error::Error;
use crate::scene::{Plot, Rect, Scene};
use crate::zoom::{self, View, Window};

/// A press that moves less than this before release is a click, not a drag
/// (egui's max_click_dist, as Gio uses).
const DRAG_SLOP: f64 = 6.0;

/// What a gesture changed: nothing, only the brush shading, or the zoom (lay
/// the chart out again).
#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord)]
pub enum Change {
    None,
    Brush,
    Zoom,
}

/// What the chart zooms, as of its last layout.
#[derive(Clone, Copy, Debug, PartialEq)]
pub(crate) enum Zoom {
    Off,
    Categories(Window),
    /// A continuous axis: `view` within `full`, the whole chart's plot;
    /// `plot` is the plot on screen, which gestures map through.
    Range {
        view: View,
        full: Plot,
        plot: Plot,
    },
}

/// The primary button is down: where it went down, whether shift was held
/// then (which makes a drag a brush), whether it has moved far enough to be
/// a drag, and where the pointer was last.
#[derive(Clone, Copy)]
struct Press {
    at: (f64, f64),
    shift: bool,
    dragging: bool,
    last: (f64, f64),
}

/// The zoom of one chart and the gesture under way.
#[derive(Clone)]
pub struct Gestures {
    pub(crate) zoom: Zoom,
    /// Wheel notches and drag distance not yet a whole category; they carry
    /// over so trackpad fractions and slow drags still move.
    wheel: f64,
    pub(crate) pan: f64,
    press: Option<Press>,
    dragged: bool,
}

impl Default for Gestures {
    fn default() -> Self {
        Gestures {
            zoom: Zoom::Off,
            wheel: 0.0,
            pan: 0.0,
            press: None,
            dragged: false,
        }
    }
}

impl Gestures {
    pub fn new() -> Self {
        Self::default()
    }

    /// Whether the wheel and drags zoom the chart, as of the last layout.
    /// When not, leave the wheel to whatever encloses the chart.
    pub fn zooms(&self) -> bool {
        self.zoom != Zoom::Off
    }

    /// Whether the last press was a drag: the click a toolkit reports for
    /// its release is none.
    pub fn dragged(&self) -> bool {
        self.dragged
    }

    /// Lays `chart` out as zoomed, and records what it zooms. A zoom kept
    /// from other data (another category count, another full extent) starts
    /// over.
    pub fn lay_out(
        &mut self,
        chart: &dyn Chart,
        w: f64,
        h: f64,
        m: &dyn TextMeasure,
    ) -> Result<Scene, Error> {
        if let Some(n) = chart.category_count().filter(|&n| n > zoom::MIN_SPAN) {
            let win = match self.zoom {
                Zoom::Categories(win) if win.len == n => win,
                _ => Window::full(n),
            };
            self.zoom = Zoom::Categories(win);
            return match (!win.is_full())
                .then(|| chart.window(win.start..win.end))
                .flatten()
            {
                Some(c) => c.scene(w, h, m),
                None => chart.scene(w, h, m),
            };
        }
        let (scene, full) = chart.plot_scene(w, h, m)?;
        let Some(full) = full else {
            self.zoom = Zoom::Off;
            return Ok(scene);
        };
        let view = match self.zoom {
            Zoom::Range { view, .. } if view.fits(&full) => view,
            _ => View::full(&full),
        };
        let (x, y) = view.ranges();
        let (scene, plot) = match (!view.is_full()).then(|| chart.view(x, y)).flatten() {
            Some(c) => {
                let (s, p) = c.plot_scene(w, h, m)?;
                (s, p.unwrap_or(full))
            }
            None => (scene, full),
        };
        self.zoom = Zoom::Range { view, full, plot };
        Ok(scene)
    }

    /// The primary button went down at `at`; `shift` makes a drag from here
    /// a brush.
    pub fn pressed(&mut self, at: (f64, f64), shift: bool) {
        self.press = Some(Press {
            at,
            shift,
            dragging: false,
            last: at,
        });
        self.dragged = false;
    }

    /// The pointer moved to `to`: while the button is down, a drag pans, or
    /// a brush follows it.
    pub fn moved(&mut self, scene: &Scene, to: (f64, f64)) -> Change {
        let Some(mut press) = self.press else {
            return Change::None;
        };
        if self.zoom == Zoom::Off {
            return Change::None;
        }
        if !press.dragging {
            if (to.0 - press.at.0).hypot(to.1 - press.at.1) < DRAG_SLOP {
                return Change::None;
            }
            // The movement before the threshold counts too.
            press.dragging = true;
        }
        let (dx, dy) = (to.0 - press.last.0, to.1 - press.last.1);
        press.last = to;
        self.press = Some(press);
        if press.shift {
            return Change::Brush;
        }
        let next = match self.zoom {
            Zoom::Categories(w) => match zoom::column_width(scene).filter(|&c| c > 0.0) {
                Some(cw) => {
                    self.pan += dx;
                    let cols = (self.pan / cw).trunc();
                    self.pan -= cols * cw;
                    Zoom::Categories(w.pan(-cols as i64))
                }
                None => self.zoom,
            },
            Zoom::Range { view, full, plot } => Zoom::Range {
                view: view.pan(&plot, (dx, dy)),
                full,
                plot,
            },
            Zoom::Off => Zoom::Off,
        };
        self.rezoom(next)
    }

    /// The primary button came up at `to`: a brush zooms to what it
    /// covered.
    pub fn released(&mut self, scene: &Scene, to: (f64, f64)) -> Change {
        let Some(press) = self.press.take() else {
            return Change::None;
        };
        self.pan = 0.0;
        self.dragged = press.dragging;
        if !(press.dragging && press.shift) {
            return Change::None;
        }
        let next = match self.zoom {
            Zoom::Categories(w) => {
                let last = (w.end - w.start).saturating_sub(1);
                match (
                    zoom::column_at(scene, press.at.0),
                    zoom::column_at(scene, to.0),
                ) {
                    (Some(a), Some(b)) => {
                        Zoom::Categories(w.brush((a as usize).min(last), (b as usize).min(last)))
                    }
                    _ => self.zoom,
                }
            }
            Zoom::Range { view, full, plot } => Zoom::Range {
                view: view.brush(&plot, press.at, to),
                full,
                plot,
            },
            Zoom::Off => Zoom::Off,
        };
        // The shading goes, zoomed or not.
        self.rezoom(next).max(Change::Brush)
    }

    /// The press ended without a release (the pointer was grabbed away): a
    /// drag stops where it is, and a brush is dropped.
    pub fn cancelled(&mut self) -> Change {
        let brushing = self.press.is_some_and(|p| p.dragging && p.shift);
        self.press = None;
        self.pan = 0.0;
        if brushing {
            Change::Brush
        } else {
            Change::None
        }
    }

    /// The wheel turned by `notches` (positive zooms in; fractions add up)
    /// with the pointer at `at`.
    pub fn wheel(&mut self, scene: &Scene, at: (f64, f64), notches: f64) -> Change {
        let next = match self.zoom {
            Zoom::Categories(w) => match zoom::column_at(scene, at.0) {
                Some(col) => {
                    self.wheel += notches;
                    let whole = self.wheel.trunc();
                    self.wheel -= whole;
                    if whole != 0.0 {
                        Zoom::Categories(w.wheel(col, whole))
                    } else {
                        self.zoom
                    }
                }
                None => self.zoom,
            },
            Zoom::Range { view, full, plot } => Zoom::Range {
                view: view.wheel(&plot, at, notches),
                full,
                plot,
            },
            Zoom::Off => Zoom::Off,
        };
        self.rezoom(next)
    }

    /// Shows everything again (a double click; not one on a legend entry,
    /// which toggled its series twice).
    pub fn reset(&mut self) -> Change {
        let next = match self.zoom {
            Zoom::Categories(w) => Zoom::Categories(Window::full(w.len)),
            Zoom::Range { full, plot, .. } => Zoom::Range {
                view: View::full(&full),
                full,
                plot,
            },
            Zoom::Off => Zoom::Off,
        };
        self.rezoom(next)
    }

    /// What a shift-drag under way covers, to shade: the columns it touched
    /// (the scene's height), or a band or box of the plot.
    pub fn brush(&self, scene: &Scene) -> Option<Rect> {
        let press = self.press.filter(|p| p.dragging && p.shift)?;
        match self.zoom {
            Zoom::Categories(_) => {
                let (a, b) = (
                    zoom::column_at(scene, press.at.0)?,
                    zoom::column_at(scene, press.last.0)?,
                );
                let (lo, hi) = zoom::brush_edges(a, b);
                let (x0, x1) = (zoom::column_x(scene, lo)?, zoom::column_x(scene, hi)?);
                Some(Rect {
                    x: x0,
                    y: 0.0,
                    w: x1 - x0,
                    h: scene.height,
                })
            }
            Zoom::Range { view, plot, .. } => Some(view.brush_rect(&plot, press.at, press.last)),
            Zoom::Off => None,
        }
    }

    fn rezoom(&mut self, next: Zoom) -> Change {
        let changed = next != self.zoom;
        self.zoom = next;
        if changed { Change::Zoom } else { Change::None }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::LineChart;
    use crate::base::text::InterMetrics;

    const MONTHS: [&str; 12] = [
        "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
    ];
    const MID: (f64, f64) = (320.0, 200.0);

    fn months(n: usize) -> LineChart {
        LineChart::new(MONTHS[..n].iter().copied()).series("Visits", (1..=n).map(|i| i as f64))
    }

    fn lay_out(g: &mut Gestures, n: usize) -> Scene {
        g.lay_out(&months(n), 640.0, 400.0, &InterMetrics).unwrap()
    }

    fn window(g: &Gestures) -> Window {
        match g.zoom {
            Zoom::Categories(w) => w,
            z => panic!("{z:?}"),
        }
    }

    #[test]
    fn a_whole_step_spends_the_fractions_it_was_made_of() {
        // Trackpads send fractions of a notch: each whole step must spend
        // what it took, or every later fraction zooms (or pans) a step more.
        let mut g = Gestures::new();
        let mut scene = lay_out(&mut g, 12);
        let col = zoom::column_at(&scene, MID.0).unwrap();
        assert_eq!(g.wheel(&scene, MID, 0.5), Change::None);
        assert_eq!(g.wheel(&scene, MID, 0.5), Change::Zoom);
        assert_eq!(window(&g), Window::full(12).wheel(col, 1.0));
        scene = lay_out(&mut g, 12);
        assert_eq!(g.wheel(&scene, MID, 0.5), Change::None);

        // Dragging two and a half columns left, a quarter at a time, shows
        // two later months.
        assert_eq!(g.wheel(&scene, MID, 3.5), Change::Zoom);
        scene = lay_out(&mut g, 12);
        let zoomed = window(&g);
        assert_ne!(zoomed.pan(2), zoomed.pan(3), "no room to overshoot");
        let cw = zoom::column_width(&scene).unwrap();
        g.pressed(MID, false);
        for k in 1..=10 {
            if g.moved(&scene, (MID.0 - cw * 0.25 * k as f64, MID.1)) == Change::Zoom {
                scene = lay_out(&mut g, 12);
            }
        }
        assert_eq!(window(&g), zoomed.pan(2));
    }

    #[test]
    fn a_brush_that_ends_without_zooming_still_takes_its_shading() {
        let mut g = Gestures::new();
        let scene = lay_out(&mut g, 12);
        let x = |col: f64| zoom::column_x(&scene, col).unwrap();
        let full = g.zoom;

        // Within one column: narrower than a zoom, but it shaded.
        g.pressed((x(3.2), MID.1), true);
        assert_eq!(g.moved(&scene, (x(3.8), MID.1)), Change::Brush);
        assert!(g.brush(&scene).is_some());
        assert_eq!(g.released(&scene, (x(3.8), MID.1)), Change::Brush);
        assert_eq!(g.zoom, full);
        assert!(g.brush(&scene).is_none());

        // The pointer grabbed away mid-brush: the shading goes, the zoom
        // stays. A cancelled pan has no shading to take.
        g.pressed((x(2.5), MID.1), true);
        assert_eq!(g.moved(&scene, (x(6.5), MID.1)), Change::Brush);
        assert_eq!(g.cancelled(), Change::Brush);
        assert_eq!(g.zoom, full);
        assert!(g.brush(&scene).is_none());
        g.pressed(MID, false);
        g.moved(&scene, (MID.0 - 60.0, MID.1));
        assert_eq!(g.cancelled(), Change::None);
    }

    #[test]
    fn a_window_kept_from_another_category_count_starts_over() {
        // New data with fewer categories: the old window would reach past
        // the last one.
        let mut g = Gestures::new();
        let scene = lay_out(&mut g, 12);
        assert_eq!(g.wheel(&scene, MID, 3.0), Change::Zoom);
        assert!(!window(&g).is_full());
        lay_out(&mut g, 8);
        assert_eq!(window(&g), Window::full(8));
    }
}
