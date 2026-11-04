//! Zooming: the part of its data a chart shows, and how wheel, drag and
//! brush gestures move it. Backends own the gestures; the arithmetic lives
//! here so every backend, and the Go port, zooms alike.
//!
//! A category axis zooms by [`Window`]: a chart reports its category count
//! through [`Chart::category_count`](crate::Chart::category_count) and
//! narrows itself with [`Chart::window`](crate::Chart::window). A continuous
//! axis (time or numbers) zooms by [`View`]: a chart reports where its plot
//! sits through [`Chart::plot_scene`](crate::Chart::plot_scene) and shows a
//! range with [`Chart::view`](crate::Chart::view); scatter and bubble charts
//! zoom y as well. Either way the axes then fit what is in view, which is
//! the point of zooming in.

use crate::scene::{HitKind, Plot, Rect, Scene};

/// Narrower than two categories there is no line to read.
pub const MIN_SPAN: usize = 2;
/// Each wheel notch shows this share of the categories: in by 0.8, out by
/// 1/0.8.
pub const WHEEL_STEP: f64 = 0.8;

/// The visible categories, `start..end`, out of `len`.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Window {
    pub start: usize,
    pub end: usize,
    pub len: usize,
}

impl Window {
    pub fn full(len: usize) -> Self {
        Window {
            start: 0,
            end: len,
            len,
        }
    }

    pub fn is_full(&self) -> bool {
        self.start == 0 && self.end == self.len
    }

    fn span(&self) -> usize {
        self.end - self.start
    }

    /// Zooms by `notches` wheel steps (positive is in) keeping the category
    /// under the pointer, `at` (a fractional index into the window, from
    /// [`column_at`]), where it is.
    pub fn wheel(&self, at: f64, notches: f64) -> Self {
        let want = (self.span() as f64 * crate::dmath::powf(WHEEL_STEP, notches)).round() as usize;
        self.resized(at, want.clamp(MIN_SPAN.min(self.len), self.len))
    }

    fn resized(&self, at: f64, span: usize) -> Self {
        let share = (at.clamp(0.0, self.span() as f64)) / self.span().max(1) as f64;
        let anchor = self.start as f64 + at.clamp(0.0, self.span() as f64);
        let start = (anchor - share * span as f64).round().max(0.0) as usize;
        self.placed(start, span)
    }

    /// Shifts the window by whole categories (positive shows later ones),
    /// stopping at either end.
    pub fn pan(&self, by: i64) -> Self {
        let start = (self.start as i64 + by).max(0) as usize;
        self.placed(start, self.span())
    }

    /// Zooms to the categories between two brushed columns, both included.
    /// A brush narrower than [`MIN_SPAN`] leaves the window as it is.
    pub fn brush(&self, a: usize, b: usize) -> Self {
        let (lo, hi) = (a.min(b), a.max(b));
        if hi - lo + 1 < MIN_SPAN {
            return *self;
        }
        self.placed(self.start + lo, hi - lo + 1)
    }

    fn placed(&self, start: usize, span: usize) -> Self {
        let span = span.min(self.len);
        let start = start.min(self.len - span);
        Window {
            start,
            end: start + span,
            len: self.len,
        }
    }
}

/// Where `x` falls along a scene's category columns (its crosshair hit
/// areas), as a fractional index from the first column's left edge;
/// clamped to the columns. `None` when the scene has none.
pub fn column_at(scene: &Scene, x: f64) -> Option<f64> {
    let cols: Vec<_> = scene
        .marks
        .iter()
        .filter_map(|m| m.hit.as_ref())
        .filter(|h| h.kind == HitKind::Crosshair)
        .filter_map(|h| h.area)
        .collect();
    let (first, last) = (cols.first()?, cols.last()?);
    let right = last.x + last.w;
    let x = x.clamp(first.x, right);
    let k = cols
        .iter()
        .position(|c| x < c.x + c.w)
        .unwrap_or(cols.len() - 1);
    let c = cols[k];
    Some(k as f64 + if c.w > 0.0 { (x - c.x) / c.w } else { 0.0 })
}

/// The x of a fractional column index, the inverse of [`column_at`]: where
/// to draw a brush edge.
pub fn column_x(scene: &Scene, at: f64) -> Option<f64> {
    let first = scene
        .marks
        .iter()
        .filter_map(|m| m.hit.as_ref())
        .find(|h| h.kind == HitKind::Crosshair)
        .and_then(|h| h.area)?;
    Some(first.x + at * first.w)
}

/// The column edges a brush from column `from` to `to` (fractional, from
/// [`column_at`]) covers, whichever way it was dragged: both end columns
/// whole, as [`Window::brush`] will zoom to them.
pub fn brush_edges(from: f64, to: f64) -> (f64, f64) {
    let (a, b) = (from.floor(), to.floor());
    (a.min(b), a.max(b) + 1.0)
}

/// The narrowest continuous view: this share of the full extent.
pub const MIN_SHARE: f64 = 1e-3;
/// A brush shorter than this (px) along a zoomed axis is a slip of the hand,
/// not a range.
pub const MIN_BRUSH: f64 = 4.0;

/// The shown part `lo..hi` of a continuous axis whose whole extent is
/// `full`.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Span {
    pub lo: f64,
    pub hi: f64,
    pub full: (f64, f64),
}

impl Span {
    pub fn full(lo: f64, hi: f64) -> Self {
        Span {
            lo,
            hi,
            full: (lo, hi),
        }
    }

    pub fn is_full(&self) -> bool {
        (self.lo, self.hi) == self.full
    }

    fn width(&self) -> f64 {
        self.hi - self.lo
    }

    /// Zooms by `notches` wheel steps (positive is in) keeping the value
    /// `at` where it is on screen.
    pub fn wheel(&self, at: f64, notches: f64) -> Self {
        let w = self.width() * crate::dmath::powf(WHEEL_STEP, notches);
        let share = ((at - self.lo) / self.width()).clamp(0.0, 1.0);
        self.placed(at - share * w, w)
    }

    /// Shifts the span by `by` (positive shows later values), stopping at
    /// either end.
    pub fn pan(&self, by: f64) -> Self {
        self.placed(self.lo + by, self.width())
    }

    /// Zooms to between `a` and `b`, either way round.
    pub fn brush(&self, a: f64, b: f64) -> Self {
        let (lo, hi) = (a.min(b), a.max(b));
        self.placed(lo, hi - lo)
    }

    /// `lo..lo + w`, the width kept between [`MIN_SHARE`] and all of it,
    /// slid back inside the full extent.
    fn placed(&self, lo: f64, w: f64) -> Self {
        let (f0, f1) = self.full;
        let w = w.max((f1 - f0) * MIN_SHARE);
        if w >= f1 - f0 {
            return Span::full(f0, f1);
        }
        let lo = lo.clamp(f0, f1 - w);
        Span {
            lo,
            hi: lo + w,
            full: self.full,
        }
    }
}

/// What a continuous zoom shows: an x span and, for charts that zoom both
/// axes, a y span. Gestures arrive as scene coordinates and map to data
/// through the [`Plot`] of the scene they happened on.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct View {
    pub x: Span,
    pub y: Option<Span>,
}

impl View {
    /// Everything: the view of a chart whose full scene has `plot`.
    pub fn full(plot: &Plot) -> Self {
        View {
            x: Span::full(plot.x.0, plot.x.1),
            y: plot.y.map(|(lo, hi)| Span::full(lo, hi)),
        }
    }

    pub fn is_full(&self) -> bool {
        self.x.is_full() && self.y.is_none_or(|y| y.is_full())
    }

    /// Whether this view belongs to the chart whose full scene has `plot`;
    /// a view kept from before the data changed does not.
    pub fn fits(&self, plot: &Plot) -> bool {
        self.x.full == plot.x && self.y.map(|y| y.full) == plot.y
    }

    /// What to pass to [`Chart::view`](crate::Chart::view).
    pub fn ranges(&self) -> ((f64, f64), Option<(f64, f64)>) {
        ((self.x.lo, self.x.hi), self.y.map(|y| (y.lo, y.hi)))
    }

    /// Zooms by `notches` wheel steps around the pointer at `at`.
    pub fn wheel(&self, plot: &Plot, at: (f64, f64), notches: f64) -> Self {
        let (x, y) = data_at(plot, at);
        View {
            x: self.x.wheel(x, notches),
            y: self.y.zip(y).map(|(s, y)| s.wheel(y, notches)),
        }
    }

    /// Pans so the data follows a pointer that moved by `by` (px).
    pub fn pan(&self, plot: &Plot, by: (f64, f64)) -> Self {
        let r = plot.rect;
        View {
            x: self.x.pan(-by.0 * self.x.width() / r.w.max(1.0)),
            y: self.y.map(|s| s.pan(by.1 * s.width() / r.h.max(1.0))),
        }
    }

    /// Zooms to the range brushed from `from` to `to`: along x, and for a
    /// chart that zooms y, along y too. A brush shorter than [`MIN_BRUSH`]
    /// on a zoomed axis leaves the view as it is.
    pub fn brush(&self, plot: &Plot, from: (f64, f64), to: (f64, f64)) -> Self {
        let short = (to.0 - from.0).abs() < MIN_BRUSH
            || (self.y.is_some() && (to.1 - from.1).abs() < MIN_BRUSH);
        if short {
            return *self;
        }
        let ((x0, y0), (x1, y1)) = (data_at(plot, from), data_at(plot, to));
        View {
            x: self.x.brush(x0, x1),
            y: self.y.zip(y0.zip(y1)).map(|(s, (a, b))| s.brush(a, b)),
        }
    }

    /// What to draw while brushing from `from` to `to`: a band across the
    /// plot's height, or for a chart that zooms y, a box; inside the plot.
    pub fn brush_rect(&self, plot: &Plot, from: (f64, f64), to: (f64, f64)) -> Rect {
        let r = plot.rect;
        let clamp_x = |x: f64| x.clamp(r.x, r.x + r.w);
        let clamp_y = |y: f64| y.clamp(r.y, r.y + r.h);
        let (x0, x1) = (clamp_x(from.0.min(to.0)), clamp_x(from.0.max(to.0)));
        let (y0, y1) = if self.y.is_some() {
            (clamp_y(from.1.min(to.1)), clamp_y(from.1.max(to.1)))
        } else {
            (r.y, r.y + r.h)
        };
        Rect {
            x: x0,
            y: y0,
            w: x1 - x0,
            h: y1 - y0,
        }
    }
}

/// The data under the scene point `at`, clamped to the plot: x, and y for
/// a plot that has it (screen y grows downward, data y upward).
fn data_at(plot: &Plot, at: (f64, f64)) -> (f64, Option<f64>) {
    let r = plot.rect;
    let tx = if r.w > 0.0 {
        ((at.0 - r.x) / r.w).clamp(0.0, 1.0)
    } else {
        0.0
    };
    let x = plot.x.0 + tx * (plot.x.1 - plot.x.0);
    let y = plot.y.map(|(lo, hi)| {
        let ty = if r.h > 0.0 {
            ((r.y + r.h - at.1) / r.h).clamp(0.0, 1.0)
        } else {
            0.0
        };
        lo + ty * (hi - lo)
    });
    (x, y)
}

/// The width of one category column, for turning drag distance into a pan.
pub fn column_width(scene: &Scene) -> Option<f64> {
    scene
        .marks
        .iter()
        .filter_map(|m| m.hit.as_ref())
        .find(|h| h.kind == HitKind::Crosshair)
        .and_then(|h| h.area)
        .map(|a| a.w)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;
    use crate::{Chart, LineChart};

    #[test]
    fn the_wheel_zooms_around_the_pointer() {
        let w = Window::full(100);
        // Pointer at category 75: it stays at three quarters across.
        let z = w.wheel(75.0, 1.0);
        assert_eq!((z.start, z.end), (15, 95));
        let back = z.wheel(60.0, -1.0);
        assert_eq!((back.start, back.end), (0, 100));
    }

    #[test]
    fn zoom_stops_at_two_categories_and_the_whole_axis() {
        let mut w = Window::full(10);
        for _ in 0..20 {
            w = w.wheel(5.0, 1.0);
        }
        assert_eq!(w.end - w.start, MIN_SPAN);
        for _ in 0..20 {
            w = w.wheel(0.0, -1.0);
        }
        assert!(w.is_full());
    }

    #[test]
    fn pan_and_brush_stay_inside_the_axis() {
        let w = Window {
            start: 10,
            end: 20,
            len: 30,
        };
        assert_eq!(
            w.pan(15),
            Window {
                start: 20,
                end: 30,
                len: 30
            }
        );
        assert_eq!(
            w.pan(-15),
            Window {
                start: 0,
                end: 10,
                len: 30
            }
        );
        // Columns 2 and 6 of the window are categories 12 and 16.
        assert_eq!(
            w.brush(6, 2),
            Window {
                start: 12,
                end: 17,
                len: 30
            }
        );
        assert_eq!(w.brush(3, 3), w, "one column is not a range");
    }

    #[test]
    fn a_brush_covers_its_end_columns_either_way() {
        // Dragged right from inside column 2 to inside column 5, or left
        // from 5 to 2: columns 2 through 5, edges 2 and 6.
        assert_eq!(brush_edges(2.4, 5.7), (2.0, 6.0));
        assert_eq!(brush_edges(5.7, 2.4), (2.0, 6.0));
    }

    #[test]
    fn columns_come_from_the_crosshair_areas() {
        let c = LineChart::new(["a", "b", "c", "d"]).series("s", [1.0, 2.0, 3.0, 4.0]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let w = column_width(&s).unwrap();
        let first = s
            .marks
            .iter()
            .find_map(|m| m.hit.as_ref().filter(|h| h.kind == HitKind::Crosshair))
            .unwrap()
            .area
            .unwrap();
        assert_eq!(column_at(&s, first.x + 2.5 * w), Some(2.5));
        assert_eq!(column_x(&s, 2.5), Some(first.x + 2.5 * w));
        assert_eq!(column_at(&s, -100.0), Some(0.0));
        assert_eq!(column_at(&s, 1e6), Some(4.0));
    }

    #[test]
    fn a_window_narrows_the_chart_and_refits_y() {
        let c = LineChart::new(["a", "b", "c", "d"]).series("s", [1.0, 2.0, 3.0, 400.0]);
        assert_eq!(c.category_count(), Some(4));
        let z = c.window(0..3).unwrap();
        let s = z.scene(640.0, 400.0, &InterMetrics).unwrap();
        let cols = s
            .marks
            .iter()
            .filter(|m| m.hit.as_ref().is_some_and(|h| h.kind == HitKind::Crosshair))
            .count();
        assert_eq!(cols, 3);
        // The 400 is out of view, so no tick reaches 100.
        assert!(!s.marks.iter().any(|m| matches!(
            &m.shape,
            crate::scene::Shape::Text { text, .. } if text == "100" || text == "400"
        )));
    }

    fn plot_2d() -> Plot {
        Plot {
            rect: Rect {
                x: 100.0,
                y: 50.0,
                w: 400.0,
                h: 200.0,
            },
            x: (0.0, 100.0),
            y: Some((-10.0, 10.0)),
        }
    }

    #[test]
    fn a_span_zooms_around_a_value_and_stays_inside_its_extent() {
        let s = Span::full(0.0, 100.0);
        // The value under the pointer, 75, stays three quarters across.
        let z = s.wheel(75.0, 1.0);
        assert!(
            (z.lo - 15.0).abs() < 1e-9 && (z.hi - 95.0).abs() < 1e-9,
            "{z:?}"
        );
        assert_eq!(z.wheel(60.0, -1.0), s, "zooming back out is everything");
        let mut deep = s;
        for _ in 0..100 {
            deep = deep.wheel(50.0, 1.0);
        }
        assert!((deep.hi - deep.lo - 100.0 * MIN_SHARE).abs() < 1e-9);
        let mid = Span {
            lo: 40.0,
            hi: 60.0,
            full: (0.0, 100.0),
        };
        assert_eq!((mid.pan(50.0).lo, mid.pan(50.0).hi), (80.0, 100.0));
        assert_eq!((mid.pan(-50.0).lo, mid.pan(-50.0).hi), (0.0, 20.0));
        assert_eq!(
            (s.brush(70.0, 30.0).lo, s.brush(70.0, 30.0).hi),
            (30.0, 70.0)
        );
    }

    #[test]
    fn a_view_maps_gestures_through_the_plot() {
        let p = plot_2d();
        let v = View::full(&p);
        // A quarter across and three quarters up: x 25, y 5.
        let at = (200.0, 100.0);
        let z = v.wheel(&p, at, 2.0);
        let zp = Plot {
            x: (z.x.lo, z.x.hi),
            y: z.y.map(|y| (y.lo, y.hi)),
            ..p
        };
        let (x, y) = data_at(&zp, at);
        assert!((x - 25.0).abs() < 1e-9 && (y.unwrap() - 5.0).abs() < 1e-9);
        // Dragging right and down shows earlier x and higher y: the data
        // follows the pointer.
        let moved = z.pan(&zp, (40.0, 20.0));
        assert!(moved.x.lo < z.x.lo && moved.y.unwrap().lo > z.y.unwrap().lo);
        // A brush is a box on a 2D plot; too short on either axis, nothing.
        let b = v.brush(&p, (100.0, 250.0), (300.0, 150.0));
        assert_eq!((b.x.lo, b.x.hi), (0.0, 50.0));
        assert_eq!((b.y.unwrap().lo, b.y.unwrap().hi), (-10.0, 0.0));
        assert_eq!(v.brush(&p, (100.0, 250.0), (300.0, 248.0)), v);
        let r = v.brush_rect(&p, (300.0, 150.0), (900.0, 120.0));
        assert_eq!((r.x, r.y, r.w, r.h), (300.0, 120.0, 200.0, 30.0));
        // Along x only, the band spans the plot's height.
        let p1 = Plot { y: None, ..p };
        let r = View::full(&p1).brush_rect(&p1, (300.0, 150.0), (200.0, 160.0));
        assert_eq!((r.y, r.h), (50.0, 200.0));
    }

    #[test]
    fn a_view_of_new_data_does_not_fit() {
        let p = plot_2d();
        let v = View::full(&p).wheel(&p, (300.0, 150.0), 1.0);
        assert!(v.fits(&p));
        assert!(!v.fits(&Plot {
            x: (0.0, 200.0),
            ..p
        }));
    }
}
