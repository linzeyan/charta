use super::frame::{self, Key, text};
use super::polar::{self, around, circle, sector};
use super::{
    Chart, GAP, HAIRLINE, MAX_SERIES, PAD, SURFACE_GAP, Series, TICK_SIZE, check_series,
    check_size, count, describe,
};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Hit, Mark, Scene, Shape, TipRow};
use std::f64::consts::TAU;

/// Categories as equal wedges around a circle (a Nightingale rose or
/// coxcomb), each wedge's area proportional to its value; several series
/// stack outward.
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::{Chart, RoseChart};
///
/// let chart = RoseChart::new(["Jan", "Feb", "Mar", "Apr", "May", "Jun"])
///     .title("Causes of mortality")
///     .series("Disease", [120.0, 150.0, 180.0, 210.0, 260.0, 320.0])
///     .series("Wounds", [20.0, 25.0, 30.0, 40.0, 60.0, 80.0]);
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Causes of mortality. Nightingale rose chart of 2 series across 6 categories.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct RoseChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub categories: Vec<String>,
    pub series: Vec<Series>,
}

impl RoseChart {
    pub fn new<S: Into<String>>(categories: impl IntoIterator<Item = S>) -> Self {
        RoseChart {
            title: None,
            categories: categories.into_iter().map(Into::into).collect(),
            series: Vec::new(),
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    pub fn series(
        mut self,
        name: impl Into<String>,
        values: impl IntoIterator<Item = f64>,
    ) -> Self {
        self.series.push(Series::new(name, values));
        self
    }
}

impl Chart for RoseChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        check_series(&self.categories, &self.series, MAX_SERIES)?;
        for s in &self.series {
            if s.values.iter().any(|&v| v < 0.0) {
                return Err(Error::InvalidData(format!(
                    "series {:?} has a negative value; wedges cannot be",
                    s.name
                )));
            }
        }
        let n = self.categories.len();
        let max = (0..n)
            .map(|i| self.series.iter().map(|s| s.values[i]).sum::<f64>())
            .fold(0.0, f64::max);
        let (top_v, ticks) = polar::radial_ticks(max);

        let names: Vec<String> = self.series.iter().map(|s| s.name.clone()).collect();
        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &names, Key::Swatch);
        let bottom = height - PAD;
        let (room_x, room_y) = polar::label_room(measure, &self.categories);
        let (cx, cy) = (width / 2.0, (top + bottom) / 2.0);
        let radius = ((width - 2.0 * PAD) / 2.0 - room_x)
            .min((bottom - top) / 2.0 - room_y)
            .max(1.0);
        // Area, not radius, carries the value: a wedge twice the value is
        // √2 times as long, as Nightingale drew them.
        let rad = |v: f64| radius * (v / top_v).sqrt();

        for &t in &ticks {
            marks.push(
                Mark::new(Shape::Path {
                    ops: circle(cx, cy, rad(t)),
                })
                .stroke(Color::Grid, HAIRLINE),
            );
        }
        let step = TAU / n.max(1) as f64;
        let stacked = self.series.len() > 1;
        for i in 0..n {
            let mut cum = 0.0;
            for (j, s) in self.series.iter().enumerate() {
                let v = s.values[i];
                let (r0, r1) = (rad(cum), rad(cum + v));
                cum += v;
                if v == 0.0 {
                    continue;
                }
                let cat = self.categories[i].as_str();
                let hit = if stacked {
                    Hit::new(format!("{i}-{j}"), cat, "").rows(vec![TipRow {
                        color: Some(Color::Series(j as u8)),
                        name: s.name.clone(),
                        value: format::value(v),
                    }])
                } else {
                    Hit::new(format!("{i}-{j}"), cat, format::value(v))
                };
                marks.push(
                    Mark::new(Shape::Path {
                        ops: sector(cx, cy, r0, r1, i as f64 * step, (i + 1) as f64 * step),
                    })
                    .fill(Color::Series(j as u8))
                    .stroke(Color::Surface, SURFACE_GAP)
                    .series(s.name.as_str())
                    .hit(hit),
                );
            }
        }
        for &t in &ticks {
            polar::tick_label(
                &mut marks,
                measure,
                (cx + GAP / 2.0, cy - rad(t)),
                Anchor::Start,
                t,
            );
        }
        for (i, c) in self.categories.iter().enumerate() {
            let (x, y, anchor, baseline) =
                around(cx, cy, radius + GAP / 2.0, (i as f64 + 0.5) * step);
            marks.push(text(
                x,
                y,
                c,
                TICK_SIZE,
                Weight::Regular,
                anchor,
                baseline,
                Color::TextSecondary,
            ));
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Nightingale rose chart of {} across {}.",
                    count(self.series.len(), "series", "series"),
                    count(n, "category", "categories")
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
    use crate::scene::PathOp;

    /// Outer radius of each wedge: distance from the center to its first
    /// point (the outer arc's start).
    fn outer(s: &Scene, cx: f64, cy: f64) -> Vec<f64> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Path { ops }, Some(_)) => match ops[0] {
                    PathOp::Move(x, y) => Some(((x - cx).powi(2) + (y - cy).powi(2)).sqrt()),
                    _ => None,
                },
                _ => None,
            })
            .collect()
    }

    #[test]
    fn wedge_area_not_length_is_proportional_to_value() {
        let c = RoseChart::new(["a", "b"]).series("v", [25.0, 100.0]);
        let s = c.scene(400.0, 400.0, &InterMetrics).unwrap();
        let r = outer(&s, 200.0, 200.0);
        // Four times the value, twice the radius (equal angles).
        assert!((r[1] / r[0] - 2.0).abs() < 1e-9, "{r:?}");
    }

    #[test]
    fn series_stack_outward_from_the_center() {
        let c = RoseChart::new(["a"]).series("x", [1.0]).series("y", [3.0]);
        let s = c.scene(400.0, 400.0, &InterMetrics).unwrap();
        let wedges: Vec<&Vec<PathOp>> = s
            .marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Path { ops }, Some(_)) => Some(ops),
                _ => None,
            })
            .collect();
        // The first series is a pie slice from the center; the second
        // starts where it ends (at the radius of 1 out of the total 4).
        let Some(&PathOp::Line(cx, cy)) = wedges[0].iter().find(|o| matches!(o, PathOp::Line(..)))
        else {
            panic!("{:?}", wedges[0])
        };
        assert!((cx - 200.0).abs() < 1e-9);
        let r = outer(&s, cx, cy);
        assert!((r[0] / r[1] - 0.5).abs() < 1e-9, "{r:?}");
    }

    #[test]
    fn negative_values_are_rejected() {
        let c = RoseChart::new(["a"]).series("v", [-1.0]);
        assert!(c.scene(400.0, 400.0, &InterMetrics).is_err());
    }
}
