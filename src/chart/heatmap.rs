use super::frame::{self, text};
use super::{Chart, GAP, PAD, SURFACE_GAP, TICK_SIZE, check_size, count, describe};
use crate::base::format;
use crate::base::scale::BandScale;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Corners, Hit, Mark, Rect, Scene, Shape};

const CELL_RADIUS: f64 = 2.0;
/// cos 45°, as in the category frame.
const SQRT_HALF: f64 = std::f64::consts::FRAC_1_SQRT_2;

/// A grid of cells colored by magnitude on a one-hue sequential ramp.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct HeatmapChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    /// Column labels.
    pub x: Vec<String>,
    /// Row labels, top to bottom.
    pub y: Vec<String>,
    /// One row per `y` label, one value per `x` label.
    pub values: Vec<Vec<f64>>,
}

impl HeatmapChart {
    pub fn new<L: Into<String>, M: Into<String>>(
        x: impl IntoIterator<Item = L>,
        y: impl IntoIterator<Item = M>,
        values: Vec<Vec<f64>>,
    ) -> Self {
        HeatmapChart {
            title: None,
            x: x.into_iter().map(Into::into).collect(),
            y: y.into_iter().map(Into::into).collect(),
            values,
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }
}

impl Chart for HeatmapChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        if self.values.len() != self.y.len() {
            return Err(Error::InvalidData(format!(
                "{} value rows for {} y labels",
                self.values.len(),
                self.y.len()
            )));
        }
        for (r, row) in self.values.iter().enumerate() {
            if row.len() != self.x.len() {
                return Err(Error::InvalidData(format!(
                    "row {:?} has {} values for {} x labels",
                    self.y[r],
                    row.len(),
                    self.x.len()
                )));
            }
            if let Some(c) = row.iter().position(|v| !v.is_finite()) {
                return Err(Error::InvalidData(format!(
                    "cell ({:?}, {:?}) is not finite",
                    self.x[c], self.y[r]
                )));
            }
        }
        let (lo, hi) = self
            .values
            .iter()
            .flatten()
            .fold((f64::INFINITY, f64::NEG_INFINITY), |(lo, hi), &v| {
                (lo.min(v), hi.max(v))
            });

        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        if lo.is_finite() {
            top = frame::seq_legend(&mut marks, measure, top, lo, hi);
        }

        let label_w = |ls: &[String]| {
            ls.iter()
                .map(|l| measure.width(l, TICK_SIZE, Weight::Regular))
                .fold(0.0, f64::max)
        };
        let left = PAD + label_w(&self.y) + GAP;
        let right = (width - PAD).max(left + 1.0);
        let cols = BandScale::new(self.x.len(), (left, right), 0.0, 0.0);
        let max_x = label_w(&self.x);
        let rotated = !self.x.is_empty() && max_x > cols.step();
        let x_area = if rotated {
            (max_x + TICK_SIZE) * SQRT_HALF + GAP
        } else {
            TICK_SIZE + GAP
        };
        let bottom = (height - PAD - x_area).max(top + 1.0);
        let rows = BandScale::new(self.y.len(), (top, bottom), 0.0, 0.0);

        let half_gap = SURFACE_GAP / 2.0;
        for (r, row) in self.values.iter().enumerate() {
            for (c, &v) in row.iter().enumerate() {
                let cell = Rect {
                    x: cols.position(c),
                    y: rows.position(r),
                    w: cols.step(),
                    h: rows.step(),
                };
                marks.push(
                    Mark::new(Shape::Rect {
                        // Touching cells are separated by the surface gap.
                        rect: Rect {
                            x: cell.x + half_gap,
                            y: cell.y + half_gap,
                            w: (cell.w - SURFACE_GAP).max(0.0),
                            h: (cell.h - SURFACE_GAP).max(0.0),
                        },
                        radius: Corners::all(CELL_RADIUS),
                    })
                    .fill(Color::Seq(frame::seq_class(v, lo, hi)))
                    .hit(
                        Hit::new(
                            format!("{r}-{c}"),
                            format!("{} · {}", self.x[c], self.y[r]),
                            format::value(v),
                        )
                        .area(cell),
                    ),
                );
            }
        }
        for (r, l) in self.y.iter().enumerate() {
            marks.push(text(
                left - GAP,
                rows.position(r) + rows.step() / 2.0,
                l,
                TICK_SIZE,
                Weight::Regular,
                Anchor::End,
                Baseline::Middle,
                Color::TextMuted,
            ));
        }
        for (c, l) in self.x.iter().enumerate() {
            let mut m = text(
                cols.position(c) + cols.step() / 2.0,
                bottom + GAP,
                l,
                TICK_SIZE,
                Weight::Regular,
                Anchor::Middle,
                Baseline::Top,
                Color::TextMuted,
            );
            if rotated && let Shape::Text { anchor, angle, .. } = &mut m.shape {
                *anchor = Anchor::End;
                *angle = -45.0;
            }
            marks.push(m);
        }

        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Heatmap with {} and {}.",
                    count(self.y.len(), "row", "rows"),
                    count(self.x.len(), "column", "columns")
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
    use crate::base::theme::SEQ_CLASSES;

    #[test]
    fn classes_span_the_ramp_from_low_to_high() {
        assert_eq!(frame::seq_class(0.0, 0.0, 70.0), 0);
        assert_eq!(frame::seq_class(70.0, 0.0, 70.0), SEQ_CLASSES - 1);
        assert_eq!(frame::seq_class(35.0, 0.0, 70.0), 3);
        assert_eq!(frame::seq_class(5.0, 5.0, 5.0), 3);
    }

    #[test]
    fn cells_are_separated_and_hover_covers_the_gap() {
        let c = HeatmapChart::new(["a", "b"], ["r"], vec![vec![1.0, 2.0]]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let cells: Vec<(Rect, Rect)> = s
            .marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Rect { rect, .. }, Some(h)) => Some((*rect, h.area.unwrap())),
                _ => None,
            })
            .collect();
        let gap = cells[1].0.x - (cells[0].0.x + cells[0].0.w);
        assert!((gap - SURFACE_GAP).abs() < 1e-9);
        // The two hover targets touch: no dead strip between cells.
        assert!((cells[0].1.x + cells[0].1.w - cells[1].1.x).abs() < 1e-9);
    }

    #[test]
    fn ragged_rows_are_rejected() {
        let c = HeatmapChart::new(["a", "b"], ["r"], vec![vec![1.0]]);
        assert!(c.scene(640.0, 400.0, &InterMetrics).is_err());
    }
}
