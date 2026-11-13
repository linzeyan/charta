use super::bar::Bar;
use super::frame::{self, text};
use super::{Chart, GAP, PAD, TICK_SIZE, check_size, count, describe, finite};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Hit, Mark, PathOp, Scene, Shape};

/// The hole in the middle, as a share of the radius: the first turn would
/// otherwise crush into a point.
const HOLE: f64 = 0.18;
/// Share of each turn's band a segment fills; the rest separates turns.
const THICK: f64 = 0.85;
/// Share of each step a segment spans; the rest separates neighbours.
const SPAN: f64 = 0.94;
/// Line segments per step along each curved edge.
const EDGE_STEPS: usize = 6;

/// Values in sequence around an Archimedean spiral, one turn per period
/// (a year of months, a week of days), so cycles line up radially.
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::{Chart, SpiralChart};
///
/// // Two years of quarterly sales, one turn of the spiral per year.
/// let chart = SpiralChart::new(
///     4,
///     [
///         ("Q1 2024", 12.0),
///         ("Q2 2024", 15.0),
///         ("Q3 2024", 19.0),
///         ("Q4 2024", 24.0),
///         ("Q1 2025", 14.0),
///         ("Q2 2025", 17.0),
///         ("Q3 2025", 22.0),
///         ("Q4 2025", 28.0),
///     ],
/// )
/// .title("Sales by quarter (M$)")
/// .angle_labels(["Q1", "Q2", "Q3", "Q4"]);
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Sales by quarter (M$). Spiral plot of 8 values, 4 per turn.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
pub struct SpiralChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    /// Values per turn.
    pub period: usize,
    /// Names of the positions within a turn (e.g. month names), shown
    /// around the outside; empty, or one per position.
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Vec::is_empty")
    )]
    pub angle_labels: Vec<String>,
    pub data: Vec<Bar>,
}

impl SpiralChart {
    pub fn new<L: Into<String>>(period: usize, data: impl IntoIterator<Item = (L, f64)>) -> Self {
        SpiralChart {
            period,
            data: data
                .into_iter()
                .map(|(label, value)| Bar {
                    label: label.into(),
                    value,
                })
                .collect(),
            ..Self::default()
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    pub fn angle_labels<S: Into<String>>(mut self, labels: impl IntoIterator<Item = S>) -> Self {
        self.angle_labels = labels.into_iter().map(Into::into).collect();
        self
    }
}

impl Chart for SpiralChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        if self.period == 0 {
            return Err(Error::InvalidData("period must be at least 1".into()));
        }
        if !self.angle_labels.is_empty() && self.angle_labels.len() != self.period {
            return Err(Error::InvalidData(format!(
                "{} angle labels for a period of {}",
                self.angle_labels.len(),
                self.period
            )));
        }
        for (i, b) in self.data.iter().enumerate() {
            finite(&format!("value {i} ({:?})", b.label), b.value)?;
        }
        let (lo, hi) = self
            .data
            .iter()
            .fold((f64::INFINITY, f64::NEG_INFINITY), |(a, b), d| {
                (a.min(d.value), b.max(d.value))
            });

        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        if lo.is_finite() {
            top = frame::seq_legend(&mut marks, measure, top, lo, hi);
        }
        let label_room = if self.angle_labels.is_empty() {
            0.0
        } else {
            TICK_SIZE
                + GAP
                + self
                    .angle_labels
                    .iter()
                    .map(|l| measure.width(l, TICK_SIZE, Weight::Regular))
                    .fold(0.0, f64::max)
                    / 2.0
        };
        let (cx, cy) = (width / 2.0, (top + height - PAD) / 2.0);
        let radius = ((width - 2.0 * PAD).min(height - PAD - top) / 2.0 - label_room).max(1.0);
        let turns = self.data.len().div_ceil(self.period).max(1) as f64;
        let r0 = radius * HOLE;
        // The last turn's outer edge sits one segment thickness beyond its
        // inner edge, and must still end at `radius`.
        let band = (radius - r0) / (turns + THICK);
        let p = self.period as f64;
        // Angle 0 is at twelve o'clock, increasing clockwise.
        let at = |u: f64, r: f64| {
            let (s, c) = crate::dmath::sin_cos(std::f64::consts::TAU * u / p);
            (cx + r * s, cy - r * c)
        };
        let inner = |u: f64| r0 + band * u / p;
        for (i, d) in self.data.iter().enumerate() {
            let (u0, u1) = (i as f64 + (1.0 - SPAN) / 2.0, i as f64 + (1.0 + SPAN) / 2.0);
            let us: Vec<f64> = (0..=EDGE_STEPS)
                .map(|k| u0 + (u1 - u0) * k as f64 / EDGE_STEPS as f64)
                .collect();
            let mut ops = Vec::with_capacity(2 * us.len() + 1);
            for (k, &u) in us.iter().enumerate() {
                let (x, y) = at(u, inner(u) + band * THICK);
                ops.push(if k == 0 {
                    PathOp::Move(x, y)
                } else {
                    PathOp::Line(x, y)
                });
            }
            for &u in us.iter().rev() {
                let (x, y) = at(u, inner(u));
                ops.push(PathOp::Line(x, y));
            }
            ops.push(PathOp::Close);
            marks.push(
                Mark::new(Shape::Path { ops })
                    .fill(Color::Seq(frame::seq_class(d.value, lo, hi)))
                    .hit(Hit::new(
                        i.to_string(),
                        d.label.as_str(),
                        format::value(d.value),
                    )),
            );
        }
        for (k, l) in self.angle_labels.iter().enumerate() {
            let (x, y) = at(k as f64 + 0.5, radius + GAP + TICK_SIZE / 2.0);
            marks.push(text(
                x,
                y,
                l,
                TICK_SIZE,
                Weight::Regular,
                Anchor::Middle,
                Baseline::Middle,
                Color::TextMuted,
            ));
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Spiral plot of {}, {} per turn.",
                    count(self.data.len(), "value", "values"),
                    self.period
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

    fn centroid(ops: &[PathOp]) -> (f64, f64) {
        let pts = &flatten(ops)[0];
        let n = pts.len() as f64;
        (
            pts.iter().map(|p| p.0).sum::<f64>() / n,
            pts.iter().map(|p| p.1).sum::<f64>() / n,
        )
    }

    #[test]
    fn the_same_position_in_each_turn_lines_up_and_moves_outward() {
        let c = SpiralChart::new(4, (0..8).map(|i| (format!("{i}"), i as f64)));
        let s = c.scene(600.0, 600.0, &InterMetrics).unwrap();
        let segs: Vec<(f64, f64)> = s
            .marks
            .iter()
            .filter_map(|m| match &m.shape {
                Shape::Path { ops } => Some(centroid(ops)),
                _ => None,
            })
            .collect();
        let (c0, c4) = (segs[0], segs[4]);
        // Position 0 of each turn sits at 45° (half a step past twelve
        // o'clock, clockwise): the second turn's segment lies farther out
        // along that same bearing, up and to the right in equal measure.
        let (dx, dy) = (c4.0 - c0.0, c4.1 - c0.1);
        assert!(dx > 0.0 && dy < 0.0, "{c0:?} {c4:?}");
        // Centroids of segments on a widening spiral drift a little off the
        // exact bearing.
        assert!((dx + dy).abs() < 0.2 * dx, "bearing drifts: {dx} {dy}");
    }

    #[test]
    fn the_outermost_turn_stays_clear_of_the_angle_labels() {
        let names = ["N", "E", "S", "W"];
        let c =
            SpiralChart::new(4, (0..10).map(|i| (format!("{i}"), i as f64))).angle_labels(names);
        let s = c.scene(600.0, 600.0, &InterMetrics).unwrap();
        let (mut cx, mut cy) = (0.0, 0.0);
        let mut labels = Vec::new();
        for m in &s.marks {
            if let Shape::Text { x, y, text, .. } = &m.shape
                && names.contains(&text.as_str())
            {
                labels.push((*x, *y));
                (cx, cy) = (cx + x / 4.0, cy + y / 4.0);
            }
        }
        // The labels' ring, less half a label height, bounds the segments.
        let ring = ((labels[0].0 - cx).powi(2) + (labels[0].1 - cy).powi(2)).sqrt();
        for m in &s.marks {
            if let Shape::Path { ops } = &m.shape {
                for &(x, y) in &flatten(ops)[0] {
                    let r = ((x - cx).powi(2) + (y - cy).powi(2)).sqrt();
                    assert!(
                        r < ring - TICK_SIZE / 2.0,
                        "{r} reaches the labels at {ring}"
                    );
                }
            }
        }
    }

    #[test]
    fn angle_labels_must_match_the_period() {
        let c = SpiralChart::new(12, [("a", 1.0)]).angle_labels(["Jan"]);
        assert!(c.scene(600.0, 600.0, &InterMetrics).is_err());
    }
}
