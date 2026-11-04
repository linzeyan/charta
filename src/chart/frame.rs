//! Layout pieces shared by charts: title, legend, value axes and the two
//! plot frames (categories along x, or categories as rows).

use super::{GAP, HAIRLINE, PAD, TICK_SIZE, TICK_SPACING, TITLE_SIZE, snap};
use crate::base::format::TickFormat;
use crate::base::scale::{BandScale, LinearScale};
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::base::theme::SEQ_CLASSES;
use crate::base::time::{self, DAY, Interval};
use crate::scene::{Anchor, Baseline, Corners, Hit, HitKind, Mark, PathOp, Rect, Shape};

pub(crate) const LEGEND_SIZE: f64 = 12.0;
const HALO: f64 = 1.0;
pub(crate) const LEGEND_ROW: f64 = 16.0;
const LEGEND_SWATCH: f64 = 10.0;
const LEGEND_LINE: f64 = 16.0;
const LEGEND_KEY_GAP: f64 = 6.0;
const LEGEND_ITEM_GAP: f64 = 16.0;
/// Target horizontal distance between value-axis ticks: labels sit side by
/// side, so they need more room than stacked ones.
pub(crate) const X_TICK_SPACING: f64 = 80.0;
/// cos 45°. The Go port spells out the same literal, 0.7071067811865476.
const SQRT_HALF: f64 = std::f64::consts::FRAC_1_SQRT_2;

#[allow(clippy::too_many_arguments)]
pub(crate) fn text(
    x: f64,
    y: f64,
    s: &str,
    size: f64,
    weight: Weight,
    anchor: Anchor,
    baseline: Baseline,
    color: Color,
) -> Mark {
    Mark::new(Shape::Text {
        x,
        y,
        text: s.to_string(),
        size,
        weight,
        anchor,
        baseline,
        angle: 0.0,
    })
    .fill(color)
}

pub(crate) fn hline(x1: f64, x2: f64, y: f64, color: Color) -> Mark {
    Mark::new(Shape::Line {
        x1,
        y1: y,
        x2,
        y2: y,
    })
    .stroke(color, HAIRLINE)
}

pub(crate) fn vline(x: f64, y1: f64, y2: f64, color: Color) -> Mark {
    Mark::new(Shape::Line {
        x1: x,
        y1,
        x2: x,
        y2,
    })
    .stroke(color, HAIRLINE)
}

/// The backing box `halo_label` draws for this text.
pub(crate) fn halo_rect(
    measure: &dyn TextMeasure,
    (x, y): (f64, f64),
    s: &str,
    anchor: Anchor,
) -> Rect {
    let w = measure.width(s, TICK_SIZE, Weight::Regular);
    let left = match anchor {
        Anchor::Start => x,
        Anchor::Middle => x - w / 2.0,
        Anchor::End => x - w,
    };
    Rect {
        x: left - HALO,
        y: y - TICK_SIZE / 2.0 - HALO,
        w: w + 2.0 * HALO,
        h: TICK_SIZE + 2.0 * HALO,
    }
}

/// A label on a surface-colored backing, so links passing under it cannot
/// cross through the text.
pub(crate) fn halo_label(
    marks: &mut Vec<Mark>,
    measure: &dyn TextMeasure,
    (x, y): (f64, f64),
    s: &str,
    anchor: Anchor,
    color: Color,
) {
    marks.push(
        Mark::new(Shape::Rect {
            rect: halo_rect(measure, (x, y), s, anchor),
            radius: Corners::all(2.0),
        })
        .fill(Color::Surface),
    );
    marks.push(text(
        x,
        y,
        s,
        TICK_SIZE,
        Weight::Regular,
        anchor,
        Baseline::Middle,
        color,
    ));
}

/// Draws the title, if any; returns the y where the content below starts.
pub(crate) fn title(marks: &mut Vec<Mark>, title: Option<&str>) -> f64 {
    let mut top = PAD;
    // An empty title reserves no space, as in the Go port where "" means none.
    if let Some(t) = title.filter(|t| !t.is_empty()) {
        marks.push(text(
            PAD,
            PAD,
            t,
            TITLE_SIZE,
            Weight::Semibold,
            Anchor::Start,
            Baseline::Top,
            Color::TextPrimary,
        ));
        top += TITLE_SIZE + 2.0 * GAP;
    }
    top
}

/// Legend keys mirror the mark: a swatch for bars and areas, a short line
/// for lines.
#[derive(Clone, Copy, PartialEq, Eq)]
pub(crate) enum Key {
    Swatch,
    Line,
}

/// One row of legend entries under the title, wrapping when full. Drawn only
/// for two or more series: with one, the title already says what is plotted.
/// Returns the y where the content below starts.
pub(crate) fn legend(
    marks: &mut Vec<Mark>,
    measure: &dyn TextMeasure,
    top: f64,
    width: f64,
    names: &[String],
    key: Key,
) -> f64 {
    if names.len() < 2 {
        return top;
    }
    let (key_w, key_h, radius) = match key {
        Key::Swatch => (LEGEND_SWATCH, LEGEND_SWATCH, 2.0),
        Key::Line => (LEGEND_LINE, 2.0, 1.0),
    };
    let (mut x, mut y) = (PAD, top);
    for (i, name) in names.iter().enumerate() {
        let w = key_w + LEGEND_KEY_GAP + measure.width(name, LEGEND_SIZE, Weight::Regular);
        if x > PAD && x + w > width - PAD {
            x = PAD;
            y += LEGEND_ROW;
        }
        let cy = y + LEGEND_ROW / 2.0;
        marks.push(
            Mark::new(Shape::Rect {
                rect: Rect {
                    x,
                    y: cy - key_h / 2.0,
                    w: key_w,
                    h: key_h,
                },
                radius: Corners::all(radius),
            })
            .fill(Color::Series(i as u8))
            .series(name.as_str())
            .hit(
                Hit::new(format!("legend-{i}"), name.as_str(), "")
                    .kind(HitKind::Legend)
                    .area(Rect {
                        x: x - LEGEND_KEY_GAP / 2.0,
                        y,
                        w: w + LEGEND_KEY_GAP,
                        h: LEGEND_ROW,
                    }),
            ),
        );
        marks.push(text(
            x + key_w + LEGEND_KEY_GAP,
            cy,
            name,
            LEGEND_SIZE,
            Weight::Regular,
            Anchor::Start,
            Baseline::Middle,
            Color::TextSecondary,
        ));
        x += w + LEGEND_ITEM_GAP;
    }
    y + LEGEND_ROW + GAP
}

const SWATCH_W: f64 = 16.0;
const SWATCH_H: f64 = 8.0;
const SCALE_ROW: f64 = 16.0;

/// The sequential scale's legend under the title: low value, the ramp's
/// classes, high value. Returns the y where the content below starts.
pub(crate) fn seq_legend(
    marks: &mut Vec<Mark>,
    measure: &dyn TextMeasure,
    top: f64,
    lo: f64,
    hi: f64,
) -> f64 {
    let fmt = TickFormat::for_ticks(&[lo, hi]);
    let (lo_s, hi_s) = (fmt.format(lo), fmt.format(hi));
    let cy = top + SCALE_ROW / 2.0;
    marks.push(text(
        PAD,
        cy,
        &lo_s,
        TICK_SIZE,
        Weight::Regular,
        Anchor::Start,
        Baseline::Middle,
        Color::TextMuted,
    ));
    let mut x = PAD + measure.width(&lo_s, TICK_SIZE, Weight::Regular) + GAP;
    for k in 0..SEQ_CLASSES {
        marks.push(
            Mark::new(Shape::Rect {
                rect: Rect {
                    x,
                    y: cy - SWATCH_H / 2.0,
                    w: SWATCH_W,
                    h: SWATCH_H,
                },
                radius: Corners::default(),
            })
            .fill(Color::Seq(k)),
        );
        x += SWATCH_W;
    }
    marks.push(text(
        x + GAP,
        cy,
        &hi_s,
        TICK_SIZE,
        Weight::Regular,
        Anchor::Start,
        Baseline::Middle,
        Color::TextMuted,
    ));
    top + SCALE_ROW + GAP
}

/// Equal-width classes over [lo, hi]; the top value lands in the last class.
/// A flat field sits in the middle class.
pub(crate) fn seq_class(v: f64, lo: f64, hi: f64) -> u8 {
    if hi == lo {
        return SEQ_CLASSES / 2;
    }
    let k = ((v - lo) / (hi - lo) * SEQ_CLASSES as f64).floor() as u8;
    k.min(SEQ_CLASSES - 1)
}

/// How a static key entry draws its sample.
#[derive(Clone, Copy)]
pub(crate) enum KeySample {
    /// An outlined, surface-filled box.
    Hollow(Color),
    Solid(Color),
    /// A short stroke of the given width.
    Line(Color, f64),
}

/// A legend row that explains an encoding rather than naming series:
/// nothing to toggle, so no hit targets. Returns the y below it.
pub(crate) fn static_key(
    marks: &mut Vec<Mark>,
    measure: &dyn TextMeasure,
    top: f64,
    items: &[(KeySample, &str)],
) -> f64 {
    let cy = top + LEGEND_ROW / 2.0;
    let mut x = PAD;
    for &(sample, name) in items {
        let (key_w, key_h) = match sample {
            KeySample::Line(_, w) => (LEGEND_LINE, w),
            _ => (LEGEND_SWATCH, LEGEND_SWATCH),
        };
        let r = Rect {
            x,
            y: cy - key_h / 2.0,
            w: key_w,
            h: key_h,
        };
        marks.push(match sample {
            KeySample::Hollow(c) => Mark::new(Shape::Path { ops: rect_ops(r) })
                .fill(Color::Surface)
                .stroke(c, 1.5),
            KeySample::Solid(c) | KeySample::Line(c, _) => Mark::new(Shape::Rect {
                rect: r,
                radius: Corners::all(1.0),
            })
            .fill(c),
        });
        marks.push(text(
            x + key_w + LEGEND_KEY_GAP,
            cy,
            name,
            LEGEND_SIZE,
            Weight::Regular,
            Anchor::Start,
            Baseline::Middle,
            Color::TextSecondary,
        ));
        x += key_w
            + LEGEND_KEY_GAP
            + measure.width(name, LEGEND_SIZE, Weight::Regular)
            + LEGEND_ITEM_GAP;
    }
    top + LEGEND_ROW + GAP
}

/// A rectangle as a closed path, for outlines (rect marks only fill).
pub(crate) fn rect_ops(r: Rect) -> Vec<PathOp> {
    vec![
        PathOp::Move(r.x, r.y),
        PathOp::Line(r.x + r.w, r.y),
        PathOp::Line(r.x + r.w, r.y + r.h),
        PathOp::Line(r.x, r.y + r.h),
        PathOp::Close,
    ]
}

/// A nice linear value axis and its tick labels.
pub(crate) struct ValueAxis {
    pub scale: LinearScale,
    pub ticks: Vec<f64>,
    pub fmt: TickFormat,
}

impl ValueAxis {
    /// A degenerate domain (all values equal) widens upward by one unit so
    /// the axis still has two ticks.
    pub fn new(lo: f64, hi: f64, range: (f64, f64), count: usize) -> Self {
        let hi = if lo == hi { lo + 1.0 } else { hi };
        let scale = LinearScale::new((lo, hi), range).nice(count);
        let ticks = scale.ticks(count);
        let fmt = TickFormat::for_ticks(&ticks);
        ValueAxis { scale, ticks, fmt }
    }

    /// An axis over exactly `[lo, hi]` with round ticks inside it, for
    /// domains that are already meaningful (histogram edges, a density's
    /// support): niceing would leave empty margins past the data.
    pub fn exact(lo: f64, hi: f64, range: (f64, f64), count: usize) -> Self {
        let hi = if lo == hi { lo + 1.0 } else { hi };
        let scale = LinearScale::new((lo, hi), range);
        let ticks = scale.ticks(count);
        let fmt = TickFormat::for_ticks(&ticks);
        ValueAxis { scale, ticks, fmt }
    }

    /// Where the baseline sits: zero when the domain spans it, otherwise the
    /// domain's low end.
    pub fn base(&self) -> f64 {
        let (d0, d1) = self.scale.domain;
        if d0 <= 0.0 && d1 >= 0.0 { 0.0 } else { d0 }
    }

    fn label(&self, t: f64) -> String {
        self.fmt.format(t)
    }

    fn label_width(&self, measure: &dyn TextMeasure) -> f64 {
        self.ticks
            .iter()
            .map(|&t| measure.width(&self.label(t), TICK_SIZE, Weight::Regular))
            .fold(0.0, f64::max)
    }
}

fn max_width(measure: &dyn TextMeasure, labels: &[String]) -> f64 {
    labels
        .iter()
        .map(|l| measure.width(l, TICK_SIZE, Weight::Regular))
        .fold(0.0, f64::max)
}

/// Categories along x (bands), values up y.
pub(crate) struct CategoryFrame {
    pub y: ValueAxis,
    pub band: BandScale,
    pub left: f64,
    pub right: f64,
    pub top: f64,
    pub bottom: f64,
    pub rotated: bool,
}

/// Lays out a category frame below `top`. `reserve_right` keeps room for
/// direct labels at the right edge.
#[allow(clippy::too_many_arguments)]
pub(crate) fn category_frame(
    measure: &dyn TextMeasure,
    width: f64,
    height: f64,
    top: f64,
    labels: &[String],
    (lo, hi): (f64, f64),
    (padding_inner, padding_outer): (f64, f64),
    reserve_right: f64,
) -> CategoryFrame {
    // Room for the top tick label, which is centered on its gridline.
    let plot_top = top + TICK_SIZE / 2.0;
    let max_label = max_width(measure, labels);
    // The x-label height depends on whether labels fit their band, which
    // depends on the plot width, which depends on the y-label width:
    // lay out once with horizontal labels, and again if they must rotate.
    let mut rotated = false;
    let mut x_area = TICK_SIZE + GAP;
    loop {
        let bottom = height - PAD - x_area;
        let plot_h = (bottom - plot_top).max(1.0);
        let count = ((plot_h / TICK_SPACING).round() as usize).max(2);
        let y = ValueAxis::new(lo, hi, (bottom, plot_top), count);
        let left = PAD + y.label_width(measure) + GAP;
        let right = (width - PAD - reserve_right).max(left + 1.0);
        let band = BandScale::new(labels.len(), (left, right), padding_inner, padding_outer);
        if !rotated && !labels.is_empty() && max_label > band.step() {
            rotated = true;
            x_area = (max_label + TICK_SIZE) * SQRT_HALF + GAP;
            continue;
        }
        return CategoryFrame {
            y,
            band,
            left,
            right,
            top: plot_top,
            bottom,
            rotated,
        };
    }
}

impl CategoryFrame {
    pub fn grid(&self, marks: &mut Vec<Mark>) {
        let base = self.y.base();
        for &t in &self.y.ticks {
            if t != base {
                marks.push(hline(
                    self.left,
                    self.right,
                    snap(self.y.scale.map(t)),
                    Color::Grid,
                ));
            }
        }
    }

    pub fn y_labels(&self, marks: &mut Vec<Mark>) {
        for &t in &self.y.ticks {
            marks.push(text(
                self.left - GAP,
                self.y.scale.map(t),
                &self.y.label(t),
                TICK_SIZE,
                Weight::Regular,
                Anchor::End,
                Baseline::Middle,
                Color::TextMuted,
            ));
        }
    }

    pub fn baseline(&self, marks: &mut Vec<Mark>) {
        marks.push(hline(
            self.left,
            self.right,
            snap(self.y.scale.map(self.y.base())),
            Color::Axis,
        ));
    }

    pub fn x_labels(&self, marks: &mut Vec<Mark>, labels: &[String]) {
        for (i, l) in labels.iter().enumerate() {
            let mut m = text(
                self.center(i),
                self.bottom + GAP,
                l,
                TICK_SIZE,
                Weight::Regular,
                Anchor::Middle,
                Baseline::Top,
                Color::TextMuted,
            );
            if self.rotated
                && let Shape::Text { anchor, angle, .. } = &mut m.shape
            {
                *anchor = Anchor::End;
                *angle = -45.0;
            }
            marks.push(m);
        }
    }

    pub fn center(&self, i: usize) -> f64 {
        self.band.position(i) + self.band.bandwidth() / 2.0
    }

    /// Band `i`'s whole column, padding included: the hover target, so short
    /// marks are as easy to hit as tall ones.
    pub fn column(&self, i: usize) -> Rect {
        let b = &self.band;
        Rect {
            x: b.position(i) - (b.step() - b.bandwidth()) / 2.0,
            y: self.top,
            w: b.step(),
            h: self.bottom - self.top,
        }
    }
}

/// Two value axes: x continuous along the bottom, y up the left.
pub(crate) struct XyFrame {
    pub x: ValueAxis,
    pub y: ValueAxis,
    pub left: f64,
    pub right: f64,
    pub top: f64,
    pub bottom: f64,
}

/// Lays out an x/y frame below `top`. `x_exact` keeps the x domain as given
/// (see [`ValueAxis::exact`]); y is always niced. `y_ticks` caps the y tick
/// count (counts on a histogram never need fractional ticks).
#[allow(clippy::too_many_arguments)]
pub(crate) fn xy_frame(
    measure: &dyn TextMeasure,
    width: f64,
    height: f64,
    top: f64,
    (xlo, xhi): (f64, f64),
    x_exact: bool,
    (ylo, yhi): (f64, f64),
    y_ticks: usize,
    x_label: bool,
) -> XyFrame {
    let plot_top = top + TICK_SIZE / 2.0;
    let x_area = TICK_SIZE + GAP + if x_label { TICK_SIZE + GAP } else { 0.0 };
    let bottom = (height - PAD - x_area).max(plot_top + 1.0);
    let count_y = ((((bottom - plot_top) / TICK_SPACING).round() as usize).max(2)).min(y_ticks);
    let y = ValueAxis::new(ylo, yhi, (bottom, plot_top), count_y);
    let left = PAD + y.label_width(measure) + GAP;
    let right = (width - PAD).max(left + 1.0);
    let count_x = (((right - left) / X_TICK_SPACING).round() as usize).max(2);
    let mut x = if x_exact {
        ValueAxis::exact(xlo, xhi, (left, right), count_x)
    } else {
        ValueAxis::new(xlo, xhi, (left, right), count_x)
    };
    // Keep the last tick label inside the padding when it sits at the edge.
    let overhang = x.ticks.last().map_or(0.0, |&t| {
        let w = measure.width(&x.label(t), TICK_SIZE, Weight::Regular) / 2.0;
        (x.scale.map(t) + w - (width - PAD)).max(0.0)
    });
    x.scale.range.1 = (right - overhang).max(left + 1.0);
    XyFrame {
        right: x.scale.range.1,
        x,
        y,
        left,
        top: plot_top,
        bottom,
    }
}

impl XyFrame {
    /// Horizontal gridlines at every y tick but the baseline's.
    pub fn grid(&self, marks: &mut Vec<Mark>) {
        let base = self.y.base();
        for &t in &self.y.ticks {
            if t != base {
                marks.push(hline(
                    self.left,
                    self.right,
                    snap(self.y.scale.map(t)),
                    Color::Grid,
                ));
            }
        }
    }

    pub fn labels(&self, marks: &mut Vec<Mark>, x_label: Option<&str>) {
        for &t in &self.y.ticks {
            marks.push(text(
                self.left - GAP,
                self.y.scale.map(t),
                &self.y.label(t),
                TICK_SIZE,
                Weight::Regular,
                Anchor::End,
                Baseline::Middle,
                Color::TextMuted,
            ));
        }
        for &t in &self.x.ticks {
            marks.push(text(
                self.x.scale.map(t),
                self.bottom + GAP,
                &self.x.label(t),
                TICK_SIZE,
                Weight::Regular,
                Anchor::Middle,
                Baseline::Top,
                Color::TextMuted,
            ));
        }
        if let Some(l) = x_label.filter(|l| !l.is_empty()) {
            marks.push(text(
                self.right,
                self.bottom + GAP + TICK_SIZE + GAP,
                l,
                TICK_SIZE,
                Weight::Regular,
                Anchor::End,
                Baseline::Top,
                Color::TextMuted,
            ));
        }
    }

    pub fn baseline(&self, marks: &mut Vec<Mark>) {
        marks.push(hline(
            self.left,
            self.right,
            snap(self.y.scale.map(self.y.base())),
            Color::Axis,
        ));
    }
}

/// A time axis: UTC epoch ms mapped linearly, ticks on local round
/// boundaries (`offset` minutes ahead of UTC).
pub(crate) struct TimeAxis {
    pub scale: LinearScale,
    pub ticks: Vec<i64>,
    pub interval: Interval,
    pub offset: i32,
}

impl TimeAxis {
    /// A zero-length span widens to one day so a single instant still has
    /// an axis around it.
    pub fn new(lo: i64, hi: i64, range: (f64, f64), count: usize, offset: i32) -> Self {
        let hi = if lo == hi { lo + DAY } else { hi };
        let interval = time::interval(lo, hi, count);
        TimeAxis {
            scale: LinearScale::new((lo as f64, hi as f64), range),
            ticks: time::ticks(lo, hi, interval, offset),
            interval,
            offset,
        }
    }

    pub fn map(&self, t: i64) -> f64 {
        self.scale.map(t as f64)
    }

    pub fn label(&self, t: i64) -> String {
        time::tick_label(t, self.interval, self.offset)
    }

    /// Tick labels centered under each tick at `y`.
    pub fn labels(&self, marks: &mut Vec<Mark>, y: f64) {
        for &t in &self.ticks {
            marks.push(text(
                self.map(t),
                y,
                &self.label(t),
                TICK_SIZE,
                Weight::Regular,
                Anchor::Middle,
                Baseline::Top,
                Color::TextMuted,
            ));
        }
    }

    /// Vertical gridlines at every tick between `top` and `bottom`.
    pub fn grid(&self, marks: &mut Vec<Mark>, top: f64, bottom: f64) {
        for &t in &self.ticks {
            marks.push(vline(snap(self.map(t)), top, bottom, Color::Grid));
        }
    }

    /// Pulls the range's end in so the last tick label, centered on its
    /// tick, stays inside `limit`.
    pub(crate) fn fit(&mut self, measure: &dyn TextMeasure, limit: f64) {
        let Some(&t) = self.ticks.last() else { return };
        let w = measure.width(&self.label(t), TICK_SIZE, Weight::Regular) / 2.0;
        let over = (self.map(t) + w - limit).max(0.0);
        let (r0, r1) = self.scale.range;
        self.scale.range.1 = (r1 - over).max(r0 + 1.0);
    }
}

/// Time along x, values up y.
pub(crate) struct TimeFrame {
    pub x: TimeAxis,
    pub y: ValueAxis,
    pub left: f64,
    pub right: f64,
    pub top: f64,
    pub bottom: f64,
}

/// Lays out a time frame below `top` over `[t0, t1]` (already padded by the
/// caller) and the value range `[lo, hi]`, niced.
#[allow(clippy::too_many_arguments)]
pub(crate) fn time_frame(
    measure: &dyn TextMeasure,
    width: f64,
    height: f64,
    top: f64,
    (t0, t1): (i64, i64),
    (lo, hi): (f64, f64),
    offset: i32,
) -> TimeFrame {
    let plot_top = top + TICK_SIZE / 2.0;
    let bottom = (height - PAD - TICK_SIZE - GAP).max(plot_top + 1.0);
    let count_y = (((bottom - plot_top) / TICK_SPACING).round() as usize).max(2);
    let y = ValueAxis::new(lo, hi, (bottom, plot_top), count_y);
    let left = PAD + y.label_width(measure) + GAP;
    let right = (width - PAD).max(left + 1.0);
    let count_x = (((right - left) / X_TICK_SPACING).round() as usize).max(2);
    let mut x = TimeAxis::new(t0, t1, (left, right), count_x, offset);
    x.fit(measure, width - PAD);
    TimeFrame {
        right: x.scale.range.1,
        x,
        y,
        left,
        top: plot_top,
        bottom,
    }
}

impl TimeFrame {
    /// Horizontal gridlines at every y tick but the baseline's.
    pub fn grid(&self, marks: &mut Vec<Mark>) {
        let base = self.y.base();
        for &t in &self.y.ticks {
            if t != base {
                marks.push(hline(
                    self.left,
                    self.right,
                    snap(self.y.scale.map(t)),
                    Color::Grid,
                ));
            }
        }
    }

    pub fn labels(&self, marks: &mut Vec<Mark>) {
        for &t in &self.y.ticks {
            marks.push(text(
                self.left - GAP,
                self.y.scale.map(t),
                &self.y.label(t),
                TICK_SIZE,
                Weight::Regular,
                Anchor::End,
                Baseline::Middle,
                Color::TextMuted,
            ));
        }
        self.x.labels(marks, self.bottom + GAP);
    }

    pub fn baseline(&self, marks: &mut Vec<Mark>) {
        marks.push(hline(
            self.left,
            self.right,
            snap(self.y.scale.map(self.y.base())),
            Color::Axis,
        ));
    }
}

/// Categories as rows (bands down y), values along x.
pub(crate) struct RowFrame {
    pub x: ValueAxis,
    pub band: BandScale,
    pub left: f64,
    pub right: f64,
    pub top: f64,
    pub bottom: f64,
}

pub(crate) fn row_frame(
    measure: &dyn TextMeasure,
    width: f64,
    height: f64,
    top: f64,
    labels: &[String],
    (lo, hi): (f64, f64),
    (padding_inner, padding_outer): (f64, f64),
) -> RowFrame {
    let left = PAD + max_width(measure, labels) + GAP;
    let bottom = (height - PAD - TICK_SIZE - GAP).max(top + 1.0);
    let right = (width - PAD).max(left + 1.0);
    let count = (((right - left) / X_TICK_SPACING).round() as usize).max(2);
    let mut x = ValueAxis::new(lo, hi, (left, right), count);
    // The last tick label is centered on the right edge; pull the plot in
    // so it does not spill past the padding.
    let last = x.ticks.last().map_or(0.0, |&t| {
        measure.width(&x.label(t), TICK_SIZE, Weight::Regular)
    });
    x.scale.range.1 = (right - last / 2.0).max(left + 1.0);
    let band = BandScale::new(labels.len(), (top, bottom), padding_inner, padding_outer);
    RowFrame {
        right: x.scale.range.1,
        x,
        band,
        left,
        top,
        bottom,
    }
}

impl RowFrame {
    /// Gridlines at every tick, zero included: a row frame has no baseline
    /// (spans float; nothing grows from zero).
    pub fn grid(&self, marks: &mut Vec<Mark>) {
        for &t in &self.x.ticks {
            marks.push(vline(
                snap(self.x.scale.map(t)),
                self.top,
                self.bottom,
                Color::Grid,
            ));
        }
    }

    pub fn x_labels(&self, marks: &mut Vec<Mark>) {
        for &t in &self.x.ticks {
            marks.push(text(
                self.x.scale.map(t),
                self.bottom + GAP,
                &self.x.label(t),
                TICK_SIZE,
                Weight::Regular,
                Anchor::Middle,
                Baseline::Top,
                Color::TextMuted,
            ));
        }
    }

    pub fn row_labels(&self, marks: &mut Vec<Mark>, labels: &[String]) {
        for (i, l) in labels.iter().enumerate() {
            marks.push(text(
                self.left - GAP,
                self.center(i),
                l,
                TICK_SIZE,
                Weight::Regular,
                Anchor::End,
                Baseline::Middle,
                Color::TextMuted,
            ));
        }
    }

    pub fn center(&self, i: usize) -> f64 {
        self.band.position(i) + self.band.bandwidth() / 2.0
    }

    /// Row `i`'s full width and step: the hover target.
    pub fn row(&self, i: usize) -> Rect {
        let b = &self.band;
        Rect {
            x: self.left,
            y: b.position(i) - (b.step() - b.bandwidth()) / 2.0,
            w: self.right - self.left,
            h: b.step(),
        }
    }
}
