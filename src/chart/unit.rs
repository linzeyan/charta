//! Unit charts: counts shown as repeated marks (icons, tally strokes, dots)
//! or as shapes sized by area.

use super::bar::Bar;
use super::frame::{self, Key, LEGEND_ROW, LEGEND_SIZE, text};
use super::icon::{Icon, circle, clip_left};
use super::{
    Chart, GAP, LINE_WIDTH, MAX_SERIES, PAD, TICK_SIZE, check_size, count, describe, finite,
};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Hit, Mark, PathOp, Rect, Scene, Shape};

/// Icons are at most this big, and below `ICON_MIN` they stop reading as
/// pictures.
const ICON_MAX: f64 = 24.0;
const ICON_MIN: f64 = 8.0;
const ICON_GAP: f64 = 4.0;
/// The icon drawn in the "each icon = …" key.
const KEY_ICON: f64 = 12.0;
/// Tally strokes: spacing, height, and how far the fifth (diagonal) stroke
/// reaches past the first and fourth.
const STROKE_GAP: f64 = 6.0;
const TALLY_H: f64 = 20.0;
const OVERHANG: f64 = 3.0;
const GROUP_GAP: f64 = 8.0;
const GROUP_STEP: f64 = 3.0 * STROKE_GAP + 2.0 * OVERHANG + GROUP_GAP;
/// Past this many marks a tally stops being countable at a glance.
const MAX_TALLY: f64 = 500.0;
/// Dot matrix cells: at most 20px, at least 4px; the dot fills 80% of its
/// cell so neighbors stay apart.
const CELL_MAX: f64 = 20.0;
const CELL_MIN: f64 = 4.0;
const DOT_FILL: f64 = 0.8;
const MAX_DOTS: usize = 2500;
/// Space between proportional shapes, and between a value and its name.
const SLOT_GAP: f64 = 16.0;
const LINE_GAP: f64 = 4.0;

macro_rules! unit_chart {
    ($(#[$doc:meta])* $name:ident { $($(#[$fdoc:meta])* $field:ident: $ty:ty),* }) => {
        $(#[$doc])*
        #[derive(Clone, Debug, Default, PartialEq)]
        #[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
        pub struct $name {
            #[cfg_attr(
                feature = "serde",
                serde(default, skip_serializing_if = "Option::is_none")
            )]
            pub title: Option<String>,
            $($(#[$fdoc])* pub $field: $ty,)*
            pub data: Vec<Bar>,
        }

        impl $name {
            pub fn title(mut self, title: impl Into<String>) -> Self {
                self.title = Some(title.into());
                self
            }
        }
    };
}

unit_chart!(
    /// Rows of icons, each standing for `unit`; the last one cut to the
    /// fraction left over.
    ///
    /// # Example
    ///
    /// ```
    /// use charta::base::text::InterMetrics;
    /// use charta::{Chart, PictogramChart};
    ///
    /// // Each icon stands for 10 volunteers.
    /// let chart = PictogramChart::new([("Lisbon", 46.0), ("Porto", 32.0), ("Faro", 7.0)], 10.0)
    ///     .title("Volunteers by city");
    /// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
    /// // The scene's label is what a screen reader announces.
    /// assert_eq!(scene.label, "Volunteers by city. Pictogram of 3 rows, each icon 10.");
    /// ```
    PictogramChart {
        /// What one icon is worth.
        unit: f64,
        #[cfg_attr(feature = "serde", serde(default))]
        icon: Icon
    }
);

unit_chart!(
    /// Counts as tally marks in groups of five.
    ///
    /// # Example
    ///
    /// ```
    /// use charta::base::text::InterMetrics;
    /// use charta::{Chart, TallyChart};
    ///
    /// let chart = TallyChart::new([("Robin", 12.0), ("Sparrow", 23.0), ("Wren", 4.0)])
    ///     .title("Birds spotted this morning");
    /// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
    /// // The scene's label is what a screen reader announces.
    /// assert_eq!(scene.label, "Birds spotted this morning. Tally chart of 3 rows.");
    /// ```
    TallyChart {}
);

unit_chart!(
    /// One dot per `unit`, colored by category and filled in order.
    ///
    /// # Example
    ///
    /// ```
    /// use charta::base::text::InterMetrics;
    /// use charta::{Chart, DotMatrixChart};
    ///
    /// // One dot per seat.
    /// let chart = DotMatrixChart::new([("Greens", 42.0), ("Reds", 31.0), ("Blues", 15.0)], 1.0)
    ///     .title("Seats won");
    /// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
    /// // The scene's label is what a screen reader announces.
    /// assert_eq!(scene.label, "Seats won. Dot matrix of 88 dots in 3 categories, each dot 1.");
    /// ```
    DotMatrixChart {
        /// What one dot is worth.
        unit: f64
    }
);

/// The shape a proportional area chart sizes.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "lowercase"))]
pub enum AreaShape {
    #[default]
    Circle,
    Square,
}

unit_chart!(
    /// Shapes side by side, area proportional to value: for values too far
    /// apart for bars, at the cost of precise comparison.
    ///
    /// # Example
    ///
    /// ```
    /// use charta::base::text::InterMetrics;
    /// use charta::{Chart, ProportionalAreaChart};
    ///
    /// let chart = ProportionalAreaChart::new([("Russia", 17.1), ("India", 3.29), ("Japan", 0.378)])
    ///     .title("Land area (M km²)");
    /// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
    /// // The scene's label is what a screen reader announces.
    /// assert_eq!(scene.label, "Land area (M km²). Proportional area chart of 3 values.");
    /// ```
    ProportionalAreaChart {
        #[cfg_attr(feature = "serde", serde(default))]
        shape: AreaShape
    }
);

fn bars<L: Into<String>>(data: impl IntoIterator<Item = (L, f64)>) -> Vec<Bar> {
    data.into_iter()
        .map(|(label, value)| Bar {
            label: label.into(),
            value,
        })
        .collect()
}

impl PictogramChart {
    pub fn new<L: Into<String>>(data: impl IntoIterator<Item = (L, f64)>, unit: f64) -> Self {
        PictogramChart {
            data: bars(data),
            unit,
            ..Default::default()
        }
    }

    pub fn icon(mut self, icon: Icon) -> Self {
        self.icon = icon;
        self
    }
}

impl TallyChart {
    pub fn new<L: Into<String>>(data: impl IntoIterator<Item = (L, f64)>) -> Self {
        TallyChart {
            data: bars(data),
            ..Default::default()
        }
    }
}

impl DotMatrixChart {
    pub fn new<L: Into<String>>(data: impl IntoIterator<Item = (L, f64)>, unit: f64) -> Self {
        DotMatrixChart {
            data: bars(data),
            unit,
            ..Default::default()
        }
    }
}

impl ProportionalAreaChart {
    pub fn new<L: Into<String>>(data: impl IntoIterator<Item = (L, f64)>) -> Self {
        ProportionalAreaChart {
            data: bars(data),
            ..Default::default()
        }
    }

    pub fn shape(mut self, shape: AreaShape) -> Self {
        self.shape = shape;
        self
    }
}

/// Values must be finite and not negative: a count of marks cannot be.
fn check_counts(data: &[Bar]) -> Result<(), Error> {
    for b in data {
        finite(&format!("{:?}", b.label), b.value)?;
        if b.value < 0.0 {
            return Err(Error::InvalidData(format!(
                "{:?} is negative; a count of marks cannot be",
                b.label
            )));
        }
    }
    Ok(())
}

fn check_unit(unit: f64) -> Result<(), Error> {
    if unit.is_finite() && unit > 0.0 {
        Ok(())
    } else {
        Err(Error::InvalidData(
            "the unit must be a positive number".into(),
        ))
    }
}

/// Labelled rows, top-aligned: the label column, where marks may run to
/// (leaving room for the value after them), and the most each row can take.
struct Rows {
    left: f64,
    right: f64,
    room: f64,
}

fn rows(measure: &dyn TextMeasure, width: f64, height: f64, top: f64, data: &[Bar]) -> Rows {
    let widest = |f: &dyn Fn(&Bar) -> String| {
        data.iter()
            .map(|b| measure.width(&f(b), TICK_SIZE, Weight::Regular))
            .fold(0.0, f64::max)
    };
    let left = PAD + widest(&|b| b.label.clone()) + GAP;
    let right = (width - PAD - widest(&|b| format::value(b.value)) - GAP).max(left + 1.0);
    let room = (height - PAD - top).max(1.0) / data.len().max(1) as f64;
    Rows { left, right, room }
}

/// A row's label, its value after the marks, and the hover target: the
/// whole row, so a zero-mark row can be hovered too.
fn row_text(marks: &mut Vec<Mark>, r: &Rows, i: usize, b: &Bar, (y, h): (f64, f64), end: f64) {
    let cy = y + h / 2.0;
    marks.push(text(
        r.left - GAP,
        cy,
        &b.label,
        TICK_SIZE,
        Weight::Regular,
        Anchor::End,
        Baseline::Middle,
        Color::TextMuted,
    ));
    marks.push(
        text(
            end,
            cy,
            &format::value(b.value),
            TICK_SIZE,
            Weight::Regular,
            Anchor::Start,
            Baseline::Middle,
            Color::TextSecondary,
        )
        .hit(
            Hit::new(i.to_string(), b.label.as_str(), format::value(b.value)).area(Rect {
                x: PAD,
                y,
                w: (r.right - PAD).max(0.0) + GAP,
                h,
            }),
        ),
    );
}

impl Chart for PictogramChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        check_unit(self.unit)?;
        check_counts(&self.data)?;
        // Whole icons and the fraction of the last; a hair over a whole
        // number is float noise, not a sliver of an icon.
        let split: Vec<(usize, f64)> = self
            .data
            .iter()
            .map(|b| {
                let q = b.value / self.unit;
                let full = q.floor();
                let frac = q - full;
                if frac < 1e-9 {
                    (full as usize, 0.0)
                } else if frac > 1.0 - 1e-9 {
                    (full as usize + 1, 0.0)
                } else {
                    (full as usize, frac)
                }
            })
            .collect();
        let most = split
            .iter()
            .map(|&(n, f)| n + usize::from(f > 0.0))
            .max()
            .unwrap_or(0);

        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        let ky = top + (LEGEND_ROW - KEY_ICON) / 2.0;
        marks.push(
            Mark::new(Shape::Path {
                ops: self.icon.place(PAD, ky, KEY_ICON)?,
            })
            .fill(Color::Series(0)),
        );
        marks.push(text(
            PAD + KEY_ICON + LINE_GAP,
            top + LEGEND_ROW / 2.0,
            &format!("= {}", format::value(self.unit)),
            LEGEND_SIZE,
            Weight::Regular,
            Anchor::Start,
            Baseline::Middle,
            Color::TextSecondary,
        ));
        top += LEGEND_ROW + GAP;

        let r = rows(measure, width, height, top, &self.data);
        let fit = if most > 0 {
            (r.right - r.left + ICON_GAP) / most as f64 - ICON_GAP
        } else {
            ICON_MAX
        };
        let s = ICON_MAX.min(fit).min(r.room - GAP);
        if s < ICON_MIN {
            return Err(Error::InvalidData(format!(
                "{most} icons in a row do not fit at {width}×{height}; raise the unit"
            )));
        }
        let pitch = r.room.min(s + 2.0 * GAP);
        for (i, (b, &(full, frac))) in self.data.iter().zip(&split).enumerate() {
            let y = top + i as f64 * pitch;
            let iy = y + (pitch - s) / 2.0;
            let n = full + usize::from(frac > 0.0);
            for k in 0..n {
                let x = r.left + k as f64 * (s + ICON_GAP);
                let ops = self.icon.place(x, iy, s)?;
                if k < full {
                    marks.push(Mark::new(Shape::Path { ops }).fill(Color::Series(0)));
                } else {
                    // The whole icon as a faint track shows what the cut
                    // one is a fraction of.
                    let part = clip_left(&ops, x + frac * s);
                    marks.push(Mark::new(Shape::Path { ops }).fill(Color::Grid));
                    marks.push(Mark::new(Shape::Path { ops: part }).fill(Color::Series(0)));
                }
            }
            let end = if n > 0 {
                r.left + n as f64 * (s + ICON_GAP) - ICON_GAP + GAP
            } else {
                r.left
            };
            row_text(&mut marks, &r, i, b, (y, pitch), end);
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Pictogram of {}, each icon {}.",
                    count(self.data.len(), "row", "rows"),
                    format::value(self.unit)
                ),
            ),
            marks,
        })
    }
}

impl Chart for TallyChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        check_counts(&self.data)?;
        for b in &self.data {
            if b.value.fract() != 0.0 || b.value > MAX_TALLY {
                return Err(Error::InvalidData(format!(
                    "{:?} is {}: tallies count whole things, up to {MAX_TALLY}",
                    b.label,
                    format::value(b.value)
                )));
            }
        }
        let mut marks = Vec::new();
        let top = frame::title(&mut marks, self.title.as_deref());
        let r = rows(measure, width, height, top, &self.data);
        let h = TALLY_H.min(r.room - GAP);
        if h < ICON_MIN {
            return Err(Error::InvalidData(format!(
                "{} do not fit at {width}×{height}",
                count(self.data.len(), "row", "rows")
            )));
        }
        let pitch = r.room.min(h + 2.0 * GAP);
        for (i, b) in self.data.iter().enumerate() {
            let n = b.value as usize;
            let (groups, rest) = (n / 5, n % 5);
            // Where the marks end: the last group's diagonal, or the last
            // loose stroke.
            let first = r.left + OVERHANG;
            let end = if rest > 0 {
                first + groups as f64 * GROUP_STEP + (rest - 1) as f64 * STROKE_GAP
            } else if groups > 0 {
                first + (groups - 1) as f64 * GROUP_STEP + 3.0 * STROKE_GAP + OVERHANG
            } else {
                r.left
            };
            if end > r.right {
                return Err(Error::InvalidData(format!(
                    "{:?}: {n} tally marks do not fit at {width}×{height}; a bar chart reads better",
                    b.label
                )));
            }
            let y = top + i as f64 * pitch;
            let (y0, y1) = (y + (pitch - h) / 2.0, y + (pitch + h) / 2.0);
            let mut ops = Vec::new();
            for g in 0..=groups {
                let gx = first + g as f64 * GROUP_STEP;
                let strokes = if g < groups { 4 } else { rest };
                for k in 0..strokes {
                    let x = gx + k as f64 * STROKE_GAP;
                    ops.push(PathOp::Move(x, y0));
                    ops.push(PathOp::Line(x, y1));
                }
                if g < groups {
                    ops.push(PathOp::Move(gx - OVERHANG, y1 - OVERHANG));
                    ops.push(PathOp::Line(
                        gx + 3.0 * STROKE_GAP + OVERHANG,
                        y0 + OVERHANG,
                    ));
                }
            }
            if !ops.is_empty() {
                marks.push(Mark::new(Shape::Path { ops }).stroke(Color::Series(0), LINE_WIDTH));
            }
            let end = if n > 0 { end + GAP } else { r.left };
            row_text(&mut marks, &r, i, b, (y, pitch), end);
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!("Tally chart of {}.", count(self.data.len(), "row", "rows")),
            ),
            marks,
        })
    }
}

impl Chart for DotMatrixChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        check_unit(self.unit)?;
        check_counts(&self.data)?;
        if self.data.len() > MAX_SERIES {
            return Err(Error::InvalidData(format!(
                "{} categories, at most {MAX_SERIES}: fold the rest into \"Other\"",
                self.data.len()
            )));
        }
        for (i, b) in self.data.iter().enumerate() {
            if self.data[..i].iter().any(|o| o.label == b.label) {
                return Err(Error::InvalidData(format!(
                    "category {:?} is listed twice",
                    b.label
                )));
            }
        }
        // Each category rounds on its own: a dot always means one unit of
        // that category, never a share borrowed from a neighbor.
        let counts: Vec<f64> = self
            .data
            .iter()
            .map(|b| (b.value / self.unit).round())
            .collect();
        // Summed as floats: a huge value must be rejected, not overflow.
        let sum: f64 = counts.iter().sum();
        if sum > MAX_DOTS as f64 {
            return Err(Error::InvalidData(format!(
                "{} dots, at most {MAX_DOTS}: raise the unit",
                format::value(sum)
            )));
        }
        let dots: Vec<usize> = counts.iter().map(|&q| q as usize).collect();
        let total: usize = dots.iter().sum();

        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        let names: Vec<String> = self.data.iter().map(|b| b.label.clone()).collect();
        top = frame::legend(&mut marks, measure, top, width, &names, Key::Swatch);
        marks.push(text(
            PAD,
            top + LEGEND_ROW / 2.0,
            &format!("Each dot = {}", format::value(self.unit)),
            LEGEND_SIZE,
            Weight::Regular,
            Anchor::Start,
            Baseline::Middle,
            Color::TextMuted,
        ));
        top += LEGEND_ROW + GAP;

        let (w, h) = ((width - 2.0 * PAD).max(1.0), (height - PAD - top).max(1.0));
        // The column count that makes the cells biggest.
        let (mut cols, mut best) = (1, 0.0);
        for c in 1..=total.max(1) {
            let rows = total.div_ceil(c).max(1);
            let cell = (w / c as f64).min(h / rows as f64);
            if cell > best {
                (cols, best) = (c, cell);
            }
        }
        let cell = best.min(CELL_MAX);
        if cell < CELL_MIN {
            return Err(Error::InvalidData(format!(
                "{total} dots do not fit at {width}×{height}; raise the unit"
            )));
        }
        let mut k = 0;
        for (i, (b, &n)) in self.data.iter().zip(&dots).enumerate() {
            for _ in 0..n {
                let (col, row) = (k % cols, k / cols);
                let mut m = Mark::new(Shape::Circle {
                    cx: PAD + (col as f64 + 0.5) * cell,
                    cy: top + (row as f64 + 0.5) * cell,
                    r: cell * DOT_FILL / 2.0,
                })
                .fill(Color::Series(i as u8))
                .hit(Hit::new(
                    format!("dot-{k}"),
                    b.label.as_str(),
                    format::value(b.value),
                ));
                if names.len() > 1 {
                    m = m.series(b.label.as_str());
                }
                marks.push(m);
                k += 1;
            }
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Dot matrix of {} in {}, each dot {}.",
                    count(total, "dot", "dots"),
                    count(self.data.len(), "category", "categories"),
                    format::value(self.unit)
                ),
            ),
            marks,
        })
    }
}

impl Chart for ProportionalAreaChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        check_counts(&self.data)?;
        let mut marks = Vec::new();
        let top = frame::title(&mut marks, self.title.as_deref());
        let roots: Vec<f64> = self.data.iter().map(|b| b.value.sqrt()).collect();
        let label_w: Vec<f64> = self
            .data
            .iter()
            .map(|b| {
                measure
                    .width(&b.label, TICK_SIZE, Weight::Regular)
                    .max(measure.width(&format::value(b.value), TICK_SIZE, Weight::Semibold))
            })
            .collect();
        let labels_h = GAP + 2.0 * TICK_SIZE + LINE_GAP;
        let (w, h) = (
            (width - 2.0 * PAD).max(1.0),
            (height - PAD - top - labels_h).max(1.0),
        );
        // A shape's side is k·√value; each slot is as wide as its shape or
        // its label. Find the largest k whose slots fit across, capped by
        // the biggest shape's height. Total width grows with k, so bisect.
        let used = |k: f64| -> f64 {
            roots
                .iter()
                .zip(&label_w)
                .map(|(&q, &l)| (k * q).max(l))
                .sum::<f64>()
                + SLOT_GAP * self.data.len().saturating_sub(1) as f64
        };
        if used(0.0) > w {
            return Err(Error::InvalidData(format!(
                "{} labels do not fit side by side at {width}",
                self.data.len()
            )));
        }
        let max_root = roots.iter().copied().fold(0.0, f64::max);
        let mut k = if max_root > 0.0 { h / max_root } else { 0.0 };
        if used(k) > w {
            let (mut lo, mut hi) = (0.0, k);
            for _ in 0..60 {
                let mid = (lo + hi) / 2.0;
                if used(mid) <= w {
                    lo = mid;
                } else {
                    hi = mid;
                }
            }
            k = lo;
        }
        let base = top + k * max_root;
        let mut x = PAD + (w - used(k)) / 2.0;
        for (i, (b, (&q, &l))) in self.data.iter().zip(roots.iter().zip(&label_w)).enumerate() {
            let side = k * q;
            let slot = side.max(l);
            let cx = x + slot / 2.0;
            let ops = match self.shape {
                AreaShape::Circle => circle(cx, base - side / 2.0, side / 2.0),
                AreaShape::Square => frame::rect_ops(Rect {
                    x: cx - side / 2.0,
                    y: base - side,
                    w: side,
                    h: side,
                }),
            };
            if side > 0.0 {
                marks.push(
                    Mark::new(Shape::Path { ops })
                        .fill(Color::Series(0))
                        .hit(Hit::new(
                            i.to_string(),
                            b.label.as_str(),
                            format::value(b.value),
                        )),
                );
            }
            marks.push(text(
                cx,
                base + GAP,
                &format::value(b.value),
                TICK_SIZE,
                Weight::Semibold,
                Anchor::Middle,
                Baseline::Top,
                Color::TextPrimary,
            ));
            marks.push(text(
                cx,
                base + GAP + TICK_SIZE + LINE_GAP,
                &b.label,
                TICK_SIZE,
                Weight::Regular,
                Anchor::Middle,
                Baseline::Top,
                Color::TextSecondary,
            ));
            x += slot + SLOT_GAP;
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Proportional area chart of {}.",
                    count(self.data.len(), "value", "values")
                ),
            ),
            marks,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;
    use crate::scene::flatten;

    fn area(ops: &[PathOp]) -> f64 {
        flatten(ops)
            .iter()
            .map(|p| {
                p.windows(2)
                    .map(|w| w[0].0 * w[1].1 - w[1].0 * w[0].1)
                    .sum::<f64>()
                    / 2.0
            })
            .sum::<f64>()
            .abs()
    }

    fn filled(s: &Scene, c: Color) -> Vec<&Vec<PathOp>> {
        s.marks
            .iter()
            .filter_map(|m| match &m.shape {
                Shape::Path { ops } if m.fill == Some(c) => Some(ops),
                _ => None,
            })
            .collect()
    }

    #[test]
    fn the_last_icon_is_cut_to_the_fraction_left_over() {
        let s = PictogramChart::new([("A", 2500.0)], 1000.0)
            .icon(Icon::Square)
            .scene(640.0, 200.0, &InterMetrics)
            .unwrap();
        // The key icon, two whole icons and the half one, over one track.
        let solid = filled(&s, Color::Series(0));
        let track = filled(&s, Color::Grid);
        assert_eq!((solid.len(), track.len()), (4, 1));
        let ratio = area(solid[3]) / area(track[0]);
        assert!((ratio - 0.5).abs() < 1e-9, "{ratio}");
    }

    #[test]
    fn whole_numbers_get_no_sliver_from_float_noise() {
        // 0.3 / 0.1 is 2.9999999999999996 in floating point.
        let s = PictogramChart::new([("A", 0.3)], 0.1)
            .scene(640.0, 200.0, &InterMetrics)
            .unwrap();
        assert!(filled(&s, Color::Grid).is_empty());
        assert_eq!(filled(&s, Color::Series(0)).len(), 1 + 3);
    }

    #[test]
    fn too_many_icons_ask_for_a_bigger_unit() {
        let err = PictogramChart::new([("A", 500.0)], 1.0)
            .scene(640.0, 200.0, &InterMetrics)
            .unwrap_err();
        assert!(err.to_string().contains("raise the unit"), "{err}");
    }

    #[test]
    fn tallies_group_in_fives_with_a_crossing_stroke() {
        let s = TallyChart::new([("A", 7.0)])
            .scene(640.0, 200.0, &InterMetrics)
            .unwrap();
        let Shape::Path { ops } = &s.marks.iter().find(|m| m.stroke.is_some()).unwrap().shape
        else {
            panic!()
        };
        let segs: Vec<((f64, f64), (f64, f64))> = ops
            .chunks(2)
            .map(|c| match (c[0], c[1]) {
                (PathOp::Move(a, b), PathOp::Line(x, y)) => ((a, b), (x, y)),
                _ => panic!(),
            })
            .collect();
        // 4 upright + 1 diagonal, then 2 upright.
        assert_eq!(segs.len(), 7);
        let upright = |s: &((f64, f64), (f64, f64))| s.0.0 == s.1.0;
        assert_eq!(
            segs.iter().map(upright).collect::<Vec<_>>(),
            [true, true, true, true, false, true, true]
        );
        // The diagonal spans all four uprights.
        let d = segs[4];
        assert!(d.0.0 < segs[0].0.0 && d.1.0 > segs[3].0.0);
        assert!(
            TallyChart::new([("A", 2.5)])
                .scene(640.0, 200.0, &InterMetrics)
                .is_err()
        );
    }

    #[test]
    fn dots_round_per_category_and_fill_in_order() {
        let s = DotMatrixChart::new([("A", 26.0), ("B", 14.0)], 10.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let fills: Vec<Color> = s
            .marks
            .iter()
            .filter(|m| matches!(m.shape, Shape::Circle { .. }))
            .map(|m| m.fill.unwrap())
            .collect();
        assert_eq!(
            fills,
            [
                Color::Series(0),
                Color::Series(0),
                Color::Series(0),
                Color::Series(1)
            ]
        );
    }

    #[test]
    fn the_dot_grid_takes_the_column_count_with_the_biggest_cells() {
        // 6 dots in a wide plot: one row of six beats two rows of three.
        let s = DotMatrixChart::new([("A", 6.0)], 1.0)
            .scene(640.0, 200.0, &InterMetrics)
            .unwrap();
        let ys: Vec<f64> = s
            .marks
            .iter()
            .filter_map(|m| match m.shape {
                Shape::Circle { cy, .. } => Some(cy),
                _ => None,
            })
            .collect();
        assert!(ys.iter().all(|&y| y == ys[0]), "{ys:?}");
    }

    #[test]
    fn shape_area_tracks_value() {
        for shape in [AreaShape::Circle, AreaShape::Square] {
            let s = ProportionalAreaChart::new([("A", 100.0), ("B", 25.0), ("C", 1.0)])
                .shape(shape)
                .scene(640.0, 400.0, &InterMetrics)
                .unwrap();
            let a = filled(&s, Color::Series(0));
            let ratio = area(a[1]) / area(a[0]);
            assert!((ratio - 0.25).abs() < 1e-3, "{shape:?} {ratio}");
            // The tiny one's label still gets a slot as wide as itself, so
            // the shapes stay apart.
            let boxes: Vec<(f64, f64)> = a
                .iter()
                .map(|ops| {
                    let p = &flatten(ops)[0];
                    let xs = p.iter().map(|q| q.0);
                    (
                        xs.clone().fold(f64::INFINITY, f64::min),
                        xs.fold(f64::NEG_INFINITY, f64::max),
                    )
                })
                .collect();
            assert!(boxes.windows(2).all(|w| w[0].1 + SLOT_GAP <= w[1].0 + 1e-9));
        }
    }

    #[test]
    fn a_huge_count_is_rejected_not_overflowed() {
        let err = DotMatrixChart::new([("A", 1e300), ("B", 1e300)], 1.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap_err();
        assert!(err.to_string().contains("raise the unit"), "{err}");
    }
}
