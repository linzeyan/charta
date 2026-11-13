use super::bar::{BAR_MAX, BAR_RADIUS, PADDING};
use super::frame::{self, Key, ValueAxis, X_TICK_SPACING, text, vline};
use super::{Chart, GAP, PAD, TICK_SIZE, check_size, count, describe, snap};
use crate::base::format;
use crate::base::scale::BandScale;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Corners, Hit, Mark, Rect, Scene, Shape};

/// One side of a population pyramid: a name and a value per group.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct PyramidSide {
    pub name: String,
    pub values: Vec<f64>,
}

/// Two distributions back to back: groups as rows (top to bottom), one side
/// growing left and the other right from a shared center gutter that holds
/// the group labels.
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::{Chart, PyramidChart};
///
/// let chart = PyramidChart::new(["0–29", "30–59", "60+"])
///     .title("Population by age (k)")
///     .sides(
///         ("Male", vec![310.0, 280.0, 150.0]),
///         ("Female", vec![300.0, 285.0, 190.0]),
///     );
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Population by age (k). Population pyramid of Male and Female across 3 groups.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct PyramidChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub categories: Vec<String>,
    pub left: PyramidSide,
    pub right: PyramidSide,
}

impl PyramidChart {
    pub fn new<L: Into<String>>(categories: impl IntoIterator<Item = L>) -> Self {
        PyramidChart {
            categories: categories.into_iter().map(Into::into).collect(),
            ..Self::default()
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    pub fn sides(
        mut self,
        left: (impl Into<String>, Vec<f64>),
        right: (impl Into<String>, Vec<f64>),
    ) -> Self {
        self.left = PyramidSide {
            name: left.0.into(),
            values: left.1,
        };
        self.right = PyramidSide {
            name: right.0.into(),
            values: right.1,
        };
        self
    }
}

impl Chart for PyramidChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        if self.left.name == self.right.name {
            return Err(Error::InvalidData(
                "the two sides need different names".into(),
            ));
        }
        for side in [&self.left, &self.right] {
            if side.values.len() != self.categories.len() {
                return Err(Error::InvalidData(format!(
                    "side {:?} has {} values for {} groups",
                    side.name,
                    side.values.len(),
                    self.categories.len()
                )));
            }
            if let Some(i) = side
                .values
                .iter()
                .position(|v| !(v.is_finite() && *v >= 0.0))
            {
                return Err(Error::InvalidData(format!(
                    "side {:?} at {:?} must be a non-negative number",
                    side.name, self.categories[i]
                )));
            }
        }
        let max = self
            .left
            .values
            .iter()
            .chain(&self.right.values)
            .fold(0.0_f64, |m, &v| m.max(v));

        let mut marks = Vec::new();
        let top = frame::title(&mut marks, self.title.as_deref());
        let names = [self.left.name.clone(), self.right.name.clone()];
        let top = frame::legend(&mut marks, measure, top, width, &names, Key::Swatch);
        let label_w = self
            .categories
            .iter()
            .map(|l| measure.width(l, TICK_SIZE, Weight::Regular))
            .fold(0.0, f64::max);
        let mid = width / 2.0;
        let gutter = label_w + 2.0 * GAP;
        let (zero_l, zero_r) = (mid - gutter / 2.0, mid + gutter / 2.0);
        let half = (zero_l - PAD).max(1.0);
        let count_x = ((half / X_TICK_SPACING).round() as usize).max(2);
        let mut axis = ValueAxis::new(0.0, max, (0.0, half), count_x);
        // The outermost tick labels are centered on the plot edges.
        let last = axis.ticks.last().map_or(0.0, |&t| {
            measure.width(&axis.fmt.format(t), TICK_SIZE, Weight::Regular)
        });
        axis.scale.range.1 = (half - last / 2.0).max(1.0);
        let reach = axis.scale.range.1;
        let bottom = (height - PAD - TICK_SIZE - GAP).max(top + 1.0);
        let band = BandScale::new(self.categories.len(), (top, bottom), PADDING.0, PADDING.1);

        for &t in &axis.ticks {
            if t != 0.0 {
                let d = axis.scale.map(t);
                marks.push(vline(snap(zero_l - d), top, bottom, Color::Grid));
                marks.push(vline(snap(zero_r + d), top, bottom, Color::Grid));
            }
        }
        let bw = band.bandwidth();
        let thick = BAR_MAX.min(bw);
        for (i, cat) in self.categories.iter().enumerate() {
            let y = band.position(i) + (bw - thick) / 2.0;
            let row_y = band.position(i) - (band.step() - bw) / 2.0;
            for (j, side) in [&self.left, &self.right].into_iter().enumerate() {
                let v = side.values[i];
                let len = axis.scale.map(v);
                let r = BAR_RADIUS.min(thick / 2.0).min(len);
                // Rounded at the outer (data) end; square at the gutter.
                let (x, radius, area_x) = if j == 0 {
                    (
                        zero_l - len,
                        Corners {
                            tl: r,
                            tr: 0.0,
                            br: 0.0,
                            bl: r,
                        },
                        zero_l - reach,
                    )
                } else {
                    (
                        zero_r,
                        Corners {
                            tl: 0.0,
                            tr: r,
                            br: r,
                            bl: 0.0,
                        },
                        zero_r,
                    )
                };
                marks.push(
                    Mark::new(Shape::Rect {
                        rect: Rect {
                            x,
                            y,
                            w: len,
                            h: thick,
                        },
                        radius,
                    })
                    .fill(Color::Series(j as u8))
                    .series(side.name.as_str())
                    .hit(
                        Hit::new(
                            format!("{i}-{j}"),
                            format!("{} · {cat}", side.name),
                            format::value(v),
                        )
                        .area(Rect {
                            x: area_x,
                            y: row_y,
                            w: reach,
                            h: band.step(),
                        }),
                    ),
                );
            }
        }
        marks.push(vline(snap(zero_l), top, bottom, Color::Axis));
        marks.push(vline(snap(zero_r), top, bottom, Color::Axis));
        for (i, cat) in self.categories.iter().enumerate() {
            marks.push(text(
                mid,
                band.position(i) + bw / 2.0,
                cat,
                TICK_SIZE,
                Weight::Regular,
                Anchor::Middle,
                Baseline::Middle,
                Color::TextMuted,
            ));
        }
        for &t in &axis.ticks {
            let d = axis.scale.map(t);
            for x in [zero_l - d, zero_r + d] {
                marks.push(text(
                    x,
                    bottom + GAP,
                    &axis.fmt.format(t),
                    TICK_SIZE,
                    Weight::Regular,
                    Anchor::Middle,
                    Baseline::Top,
                    Color::TextMuted,
                ));
            }
        }

        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Population pyramid of {} and {} across {}.",
                    self.left.name,
                    self.right.name,
                    count(self.categories.len(), "group", "groups")
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

    #[test]
    fn sides_grow_outward_from_the_gutter_on_one_shared_scale() {
        let c = PyramidChart::new(["0-9", "10-19"])
            .sides(("Male", vec![10.0, 20.0]), ("Female", vec![20.0, 5.0]));
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let bars: Vec<Rect> = s
            .marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Rect { rect, .. }, Some(h)) if h.kind == crate::scene::HitKind::Mark => {
                    Some(*rect)
                }
                _ => None,
            })
            .collect();
        let (male0, female0) = (bars[0], bars[1]);
        assert!(male0.x + male0.w <= 320.0 && female0.x >= 320.0);
        // Same value, same length, on either side.
        assert!((male0.w * 2.0 - female0.w).abs() < 1e-9);
        assert!((bars[2].w - female0.w).abs() < 1e-9);
    }

    #[test]
    fn negative_counts_are_rejected() {
        let c = PyramidChart::new(["a"]).sides(("L", vec![-1.0]), ("R", vec![1.0]));
        assert!(c.scene(640.0, 400.0, &InterMetrics).is_err());
    }
}
