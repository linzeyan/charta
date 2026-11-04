use super::frame::{self, Key, text};
use super::polar::{self, around, at};
use super::{
    Chart, DOT_RADIUS, GAP, HAIRLINE, LINE_WIDTH, MIN_TARGET, PAD, SURFACE_GAP, Series, TICK_SIZE,
    check_series, check_size, count, describe,
};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Hit, Mark, PathOp, Rect, Scene, Shape, TipRow};
use std::f64::consts::TAU;

/// Overlapping translucent polygons turn to mud past three.
const MAX_SERIES: usize = 3;

/// Several measures on spokes sharing one scale from the center, each
/// series a closed polygon: profiles compared at a glance.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct RadarChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub axes: Vec<String>,
    pub series: Vec<Series>,
}

impl RadarChart {
    pub fn new<S: Into<String>>(axes: impl IntoIterator<Item = S>) -> Self {
        RadarChart {
            title: None,
            axes: axes.into_iter().map(Into::into).collect(),
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

fn polygon(points: &[(f64, f64)]) -> Vec<PathOp> {
    let mut ops: Vec<PathOp> = points
        .iter()
        .enumerate()
        .map(|(k, &(x, y))| {
            if k == 0 {
                PathOp::Move(x, y)
            } else {
                PathOp::Line(x, y)
            }
        })
        .collect();
    ops.push(PathOp::Close);
    ops
}

impl Chart for RadarChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        if self.axes.len() < 3 {
            return Err(Error::InvalidData(format!(
                "{} axes: a radar needs at least 3 (use a bar chart for fewer)",
                self.axes.len()
            )));
        }
        check_series(&self.axes, &self.series, MAX_SERIES)?;
        for s in &self.series {
            if s.values.iter().any(|&v| v < 0.0) {
                return Err(Error::InvalidData(format!(
                    "series {:?} has a negative value; spokes start at zero",
                    s.name
                )));
            }
        }
        let max = self
            .series
            .iter()
            .flat_map(|s| s.values.iter().copied())
            .fold(0.0, f64::max);
        let (top_v, ticks) = polar::radial_ticks(max);

        let names: Vec<String> = self.series.iter().map(|s| s.name.clone()).collect();
        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &names, Key::Line);
        let bottom = height - PAD;
        let (room_x, room_y) = polar::label_room(measure, &self.axes);
        let (cx, cy) = (width / 2.0, (top + bottom) / 2.0);
        let radius = ((width - 2.0 * PAD) / 2.0 - room_x)
            .min((bottom - top) / 2.0 - room_y)
            .max(1.0);
        let n = self.axes.len();
        let angle = |k: usize| TAU * k as f64 / n as f64;
        let point = |k: usize, v: f64| at(cx, cy, radius * v / top_v, angle(k));

        // Rings are polygons, not circles: values between spokes are not
        // defined, and a circle would suggest they are.
        for &t in &ticks {
            let ring: Vec<(f64, f64)> = (0..n).map(|k| point(k, t)).collect();
            marks.push(
                Mark::new(Shape::Path {
                    ops: polygon(&ring),
                })
                .stroke(Color::Grid, HAIRLINE),
            );
        }
        for k in 0..n {
            let (x, y) = point(k, top_v);
            marks.push(
                Mark::new(Shape::Line {
                    x1: cx,
                    y1: cy,
                    x2: x,
                    y2: y,
                })
                .stroke(Color::Grid, HAIRLINE),
            );
        }
        for (j, s) in self.series.iter().enumerate() {
            let pts: Vec<(f64, f64)> = (0..n).map(|k| point(k, s.values[k])).collect();
            marks.push(
                Mark::new(Shape::Path { ops: polygon(&pts) })
                    .fill(Color::Wash(j as u8))
                    .stroke(Color::Series(j as u8), LINE_WIDTH)
                    .series(s.name.as_str()),
            );
        }
        // Dots above every polygon, so each stays reachable.
        for (j, s) in self.series.iter().enumerate() {
            for k in 0..n {
                let (x, y) = point(k, s.values[k]);
                marks.push(
                    Mark::new(Shape::Circle {
                        cx: x,
                        cy: y,
                        r: DOT_RADIUS,
                    })
                    .fill(Color::Series(j as u8))
                    .stroke(Color::Surface, SURFACE_GAP)
                    .series(s.name.as_str())
                    .hit(
                        Hit::new(format!("{j}-{k}"), self.axes[k].as_str(), "")
                            .rows(vec![TipRow {
                                color: Some(Color::Series(j as u8)),
                                name: s.name.clone(),
                                value: format::value(s.values[k]),
                            }])
                            .area(Rect {
                                x: x - MIN_TARGET / 2.0,
                                y: y - MIN_TARGET / 2.0,
                                w: MIN_TARGET,
                                h: MIN_TARGET,
                            }),
                    ),
                );
            }
        }
        // The outermost ring's label would sit on the first axis's name;
        // the rings inside it give the step.
        for &t in ticks.iter().filter(|&&t| t < top_v) {
            let (_, y) = point(0, t);
            polar::tick_label(&mut marks, measure, (cx + GAP / 2.0, y), Anchor::Start, t);
        }
        for (k, a) in self.axes.iter().enumerate() {
            let (x, y, anchor, baseline) = around(cx, cy, radius + GAP / 2.0, angle(k));
            marks.push(text(
                x,
                y,
                a,
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
                    "Radar chart of {} on {}.",
                    count(self.series.len(), "series", "series"),
                    count(n, "axis", "axes")
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

    fn dots(s: &Scene) -> Vec<(f64, f64)> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Circle { cx, cy, .. }, Some(_)) => Some((*cx, *cy)),
                _ => None,
            })
            .collect()
    }

    #[test]
    fn distance_from_the_center_is_proportional_to_value() {
        let c = RadarChart::new(["a", "b", "c", "d"]).series("s", [10.0, 20.0, 40.0, 5.0]);
        let s = c.scene(400.0, 400.0, &InterMetrics).unwrap();
        let d = dots(&s);
        // Axis a points up, c points down: both sit on the vertical through
        // the center, c four times as far from it as a.
        let cy = (d[0].1 * 4.0 + d[2].1) / 5.0;
        assert!((d[0].0 - d[2].0).abs() < 1e-9);
        assert!(((d[2].1 - cy) / (cy - d[0].1) - 4.0).abs() < 1e-9);
        // b points right at twice a's distance.
        assert!(((d[1].0 - d[0].0) / (cy - d[0].1) - 2.0).abs() < 1e-9);
    }

    #[test]
    fn fewer_than_three_axes_or_too_many_series_are_rejected() {
        let m = &InterMetrics;
        let c = RadarChart::new(["a", "b"]).series("s", [1.0, 2.0]);
        assert!(c.scene(400.0, 400.0, m).is_err());
        let c = (0..4).fold(RadarChart::new(["a", "b", "c"]), |c, i| {
            c.series(i.to_string(), [1.0, 1.0, 1.0])
        });
        assert!(c.scene(400.0, 400.0, m).is_err());
    }

    #[test]
    fn every_vertex_names_its_axis_and_series() {
        let c = RadarChart::new(["Speed", "Range", "Price"])
            .series("A", [1.0, 2.0, 3.0])
            .series("B", [3.0, 2.0, 1.0]);
        let s = c.scene(400.0, 400.0, &InterMetrics).unwrap();
        let h = s
            .marks
            .iter()
            .filter_map(|m| m.hit.as_ref())
            .find(|h| h.key == "1-2")
            .unwrap();
        assert_eq!(
            (
                h.label.as_str(),
                h.rows[0].name.as_str(),
                h.rows[0].value.as_str()
            ),
            ("Price", "B", "1")
        );
    }
}
