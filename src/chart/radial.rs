use super::bar::{BAR_MAX, Bar};
use super::frame::{self, text};
use super::polar::{self, around, at, circle, sector};
use super::{Chart, GAP, HAIRLINE, PAD, TICK_SIZE, check_size, count, describe, finite};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Hit, Mark, Scene, Shape};
use std::f64::consts::{PI, TAU};

/// The largest value sweeps three quarters of a turn: beyond that, bars on
/// different rings stop being comparable at a glance (an outer ring's arc is
/// far longer than an inner one's for the same angle), and the free quarter
/// holds the labels.
const SWEEP: f64 = 1.5 * PI;
/// The empty middle, as a share of the radius.
const HOLE: f64 = 0.3;
/// Share of each ring a bar fills.
const RING_FILL: f64 = 0.7;
/// Share of each angular slot a column fills.
const COLUMN_FILL: f64 = 0.8;

macro_rules! radial_chart {
    ($(#[$doc:meta])* $name:ident, $build:ident) => {
        $(#[$doc])*
        #[derive(Clone, Debug, Default, PartialEq)]
        #[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
        pub struct $name {
            #[cfg_attr(
                feature = "serde",
                serde(default, skip_serializing_if = "Option::is_none")
            )]
            pub title: Option<String>,
            pub data: Vec<Bar>,
        }

        impl $name {
            pub fn new<L: Into<String>>(data: impl IntoIterator<Item = (L, f64)>) -> Self {
                $name {
                    title: None,
                    data: data
                        .into_iter()
                        .map(|(label, value)| Bar {
                            label: label.into(),
                            value,
                        })
                        .collect(),
                }
            }

            pub fn title(mut self, title: impl Into<String>) -> Self {
                self.title = Some(title.into());
                self
            }
        }

        impl Chart for $name {
            fn scene(
                &self,
                width: f64,
                height: f64,
                measure: &dyn TextMeasure,
            ) -> Result<Scene, Error> {
                check_size(width, height)?;
                for (i, b) in self.data.iter().enumerate() {
                    finite(&format!("bar {i} ({:?})", b.label), b.value)?;
                    if b.value < 0.0 {
                        return Err(Error::InvalidData(format!(
                            "bar {i} ({:?}) is negative; radial bars start at zero",
                            b.label
                        )));
                    }
                }
                $build(self.title.as_deref(), &self.data, width, height, measure)
            }
        }
    };
}

radial_chart!(
    /// Bars bent into concentric rings, length as the angle swept clockwise
    /// from twelve o'clock; the first bar is the outermost ring.
    RadialBarChart,
    radial_bar
);
radial_chart!(
    /// Columns standing outward from a circle, one per category around it,
    /// length from the inner circle proportional to value.
    RadialColumnChart,
    radial_column
);

fn max_value(data: &[Bar]) -> f64 {
    data.iter().map(|b| b.value).fold(0.0, f64::max)
}

fn radial_bar(
    title: Option<&str>,
    data: &[Bar],
    width: f64,
    height: f64,
    measure: &dyn TextMeasure,
) -> Result<Scene, Error> {
    let (top_v, ticks) = polar::radial_ticks(max_value(data));
    let mut marks = Vec::new();
    let top = frame::title(&mut marks, title);
    let bottom = height - PAD;
    let tick_labels: Vec<String> = ticks.iter().map(|&t| format::value(t)).collect();
    let (room_x, room_y) = polar::label_room(measure, &tick_labels);
    let (cx, cy) = (width / 2.0, (top + bottom) / 2.0);
    let radius = ((width - 2.0 * PAD) / 2.0 - room_x)
        .min((bottom - top) / 2.0 - room_y)
        .max(1.0);
    let inner = radius * HOLE;
    let ring = (radius - inner) / data.len().max(1) as f64;
    let thick = (ring * RING_FILL).min(BAR_MAX);
    let angle = |v: f64| SWEEP * v / top_v;

    // Spokes at each tick, with "0" at the start: the scale is the angle.
    for t in std::iter::once(0.0).chain(ticks.iter().copied()) {
        let a = angle(t);
        let (x1, y1) = at(cx, cy, inner - GAP / 2.0, a);
        let (x2, y2) = at(cx, cy, radius + GAP / 2.0, a);
        marks.push(Mark::new(Shape::Line { x1, y1, x2, y2 }).stroke(Color::Grid, HAIRLINE));
        let (x, y, anchor, baseline) = around(cx, cy, radius + GAP, a);
        marks.push(text(
            x,
            y,
            &format::value(t),
            TICK_SIZE,
            Weight::Regular,
            anchor,
            baseline,
            Color::TextMuted,
        ));
    }
    for (i, b) in data.iter().enumerate() {
        let mid = radius - (i as f64 + 0.5) * ring;
        let (r0, r1) = (mid - thick / 2.0, mid + thick / 2.0);
        // The full-length track shows how far the bar could go; a wash of
        // the bar's hue reads as its background, not as more data.
        marks.push(
            Mark::new(Shape::Path {
                ops: sector(cx, cy, r0, r1, 0.0, SWEEP),
            })
            .fill(Color::Wash(0)),
        );
        if b.value > 0.0 {
            marks.push(
                Mark::new(Shape::Path {
                    ops: sector(cx, cy, r0, r1, 0.0, angle(b.value)),
                })
                .fill(Color::Series(0))
                .hit(Hit::new(
                    i.to_string(),
                    b.label.as_str(),
                    format::value(b.value),
                )),
            );
        }
        // Labels sit in the free quarter, right-aligned to where bars start.
        marks.push(text(
            cx - GAP / 2.0,
            cy - mid,
            &b.label,
            TICK_SIZE,
            Weight::Regular,
            Anchor::End,
            Baseline::Middle,
            Color::TextSecondary,
        ));
    }
    Ok(Scene {
        width,
        height,
        label: describe(
            title,
            &format!(
                "Radial bar chart with {}.",
                count(data.len(), "bar", "bars")
            ),
        ),
        marks,
    })
}

fn radial_column(
    title: Option<&str>,
    data: &[Bar],
    width: f64,
    height: f64,
    measure: &dyn TextMeasure,
) -> Result<Scene, Error> {
    let (top_v, ticks) = polar::radial_ticks(max_value(data));
    let mut marks = Vec::new();
    let top = frame::title(&mut marks, title);
    let bottom = height - PAD;
    let labels: Vec<String> = data.iter().map(|b| b.label.clone()).collect();
    let (room_x, room_y) = polar::label_room(measure, &labels);
    let (cx, cy) = (width / 2.0, (top + bottom) / 2.0);
    let radius = ((width - 2.0 * PAD) / 2.0 - room_x)
        .min((bottom - top) / 2.0 - room_y)
        .max(1.0);
    let inner = radius * HOLE;
    let rad = |v: f64| inner + (radius - inner) * v / top_v;
    // One slot more than there are columns: the slot at twelve o'clock
    // stays empty and carries the scale.
    let step = TAU / (data.len() + 1) as f64;
    let pad = step * (1.0 - COLUMN_FILL) / 2.0;

    marks.push(
        Mark::new(Shape::Path {
            ops: circle(cx, cy, inner),
        })
        .stroke(Color::Axis, HAIRLINE),
    );
    for &t in &ticks {
        marks.push(
            Mark::new(Shape::Path {
                ops: circle(cx, cy, rad(t)),
            })
            .stroke(Color::Grid, HAIRLINE),
        );
    }
    for (i, b) in data.iter().enumerate() {
        let (a0, a1) = ((i as f64 + 0.5) * step + pad, (i as f64 + 1.5) * step - pad);
        if b.value > 0.0 {
            marks.push(
                Mark::new(Shape::Path {
                    ops: sector(cx, cy, inner, rad(b.value), a0, a1),
                })
                .fill(Color::Series(0))
                .hit(Hit::new(
                    i.to_string(),
                    b.label.as_str(),
                    format::value(b.value),
                )),
            );
        }
        let (x, y, anchor, baseline) = around(cx, cy, radius + GAP / 2.0, (a0 + a1) / 2.0);
        marks.push(text(
            x,
            y,
            &b.label,
            TICK_SIZE,
            Weight::Regular,
            anchor,
            baseline,
            Color::TextSecondary,
        ));
    }
    for &t in &ticks {
        polar::tick_label(&mut marks, measure, (cx, cy - rad(t)), Anchor::Middle, t);
    }
    Ok(Scene {
        width,
        height,
        label: describe(
            title,
            &format!(
                "Radial column chart with {}.",
                count(data.len(), "column", "columns")
            ),
        ),
        marks,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;
    use crate::scene::PathOp;

    fn hit_paths(s: &Scene) -> Vec<&Vec<PathOp>> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Path { ops }, Some(_)) => Some(ops),
                _ => None,
            })
            .collect()
    }

    /// The angle swept by a bar's outer edge, from its start (straight up)
    /// to the end point of its last outer cubic.
    fn swept(ops: &[PathOp], cx: f64, cy: f64) -> f64 {
        let end = ops
            .iter()
            .take_while(|o| !matches!(o, PathOp::Line(..)))
            .filter_map(|o| match *o {
                PathOp::Cubic(.., x, y) => Some((x, y)),
                _ => None,
            })
            .last()
            .unwrap();
        let a = (end.0 - cx).atan2(cy - end.1);
        if a < 0.0 { a + TAU } else { a }
    }

    #[test]
    fn radial_bar_angle_is_proportional_to_value() {
        let s = RadialBarChart::new([("a", 20.0), ("b", 40.0)])
            .scene(400.0, 400.0, &InterMetrics)
            .unwrap();
        let p = hit_paths(&s);
        let cy = (PAD + 400.0 - PAD) / 2.0;
        let (a, b) = (swept(p[0], 200.0, cy), swept(p[1], 200.0, cy));
        assert!((b / a - 2.0).abs() < 1e-9, "{a} {b}");
        // The scale tops out at a round number, never past three quarters.
        assert!(b <= SWEEP + 1e-9);
    }

    #[test]
    fn the_first_radial_bar_is_the_outermost_ring() {
        let s = RadialBarChart::new([("outer", 1.0), ("inner", 1.0)])
            .scene(400.0, 400.0, &InterMetrics)
            .unwrap();
        let p = hit_paths(&s);
        let cy = (PAD + 400.0 - PAD) / 2.0;
        let r = |ops: &Vec<PathOp>| match ops[0] {
            PathOp::Move(_, y) => cy - y,
            _ => panic!(),
        };
        assert!(r(p[0]) > r(p[1]));
    }

    #[test]
    fn radial_column_length_from_the_inner_circle_is_proportional_to_value() {
        let s = RadialColumnChart::new([("a", 10.0), ("b", 20.0), ("c", 0.0), ("d", 5.0)])
            .scene(400.0, 400.0, &InterMetrics)
            .unwrap();
        let p = hit_paths(&s);
        assert_eq!(p.len(), 3, "a zero column draws nothing");
        let cy = (PAD + 400.0 - PAD) / 2.0;
        let dist = |x: f64, y: f64| ((x - 200.0).powi(2) + (y - cy).powi(2)).sqrt();
        // Outer radius from the Move; inner radius from the Line onto the
        // inner arc.
        let radii = |ops: &Vec<PathOp>| {
            let (PathOp::Move(x, y), Some(&PathOp::Line(u, v))) =
                (ops[0], ops.iter().find(|o| matches!(o, PathOp::Line(..))))
            else {
                panic!()
            };
            (dist(u, v), dist(x, y))
        };
        let (i0, o0) = radii(p[0]);
        let (i1, o1) = radii(p[1]);
        assert!((i0 - i1).abs() < 1e-9, "one shared inner circle");
        assert!(((o1 - i1) / (o0 - i0) - 2.0).abs() < 1e-9);
    }

    #[test]
    fn negative_values_are_rejected() {
        let m = &InterMetrics;
        assert!(
            RadialBarChart::new([("a", -1.0)])
                .scene(400.0, 400.0, m)
                .is_err()
        );
        assert!(
            RadialColumnChart::new([("a", -1.0)])
                .scene(400.0, 400.0, m)
                .is_err()
        );
    }
}
