use super::frame::{self, Key, category_frame, text};
use super::{
    Chart, DOT_RADIUS, GAP, LINE_WIDTH, MAX_SERIES, SURFACE_GAP, Series, TICK_SIZE, check_series,
    check_size, count, describe, snap,
};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::layout::curve::monotone;
use crate::scene::{Anchor, Baseline, Hit, HitKind, Mark, PathOp, Scene, Shape, TipRow};

/// Points sit at band centers with no padding, so the hover columns tile
/// the plot with no gaps between them.
const PADDING: (f64, f64) = (0.0, 0.0);
/// Direct end labels ride the lines for this many series at most; beyond
/// that the legend and tooltip carry identity.
const MAX_END_LABELS: usize = 4;

/// How a series runs from one point to the next.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
pub enum Curve {
    /// Straight segments: every bend is a data point.
    #[default]
    Linear,
    /// A smooth curve that never overshoots: between two points it stays
    /// within their values, so it shows no peak or dip the data lacks.
    Monotone,
}

impl Curve {
    #[cfg_attr(not(feature = "serde"), allow(dead_code))]
    fn is_linear(&self) -> bool {
        *self == Curve::Linear
    }
}

macro_rules! series_chart {
    ($(#[$doc:meta])* $name:ident) => {
        $(#[$doc])*
        #[derive(Clone, Debug, Default, PartialEq)]
        #[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
        pub struct $name {
            #[cfg_attr(
                feature = "serde",
                serde(default, skip_serializing_if = "Option::is_none")
            )]
            pub title: Option<String>,
            pub categories: Vec<String>,
            pub series: Vec<Series>,
            #[cfg_attr(
                feature = "serde",
                serde(default, skip_serializing_if = "Curve::is_linear")
            )]
            pub curve: Curve,
        }

        impl $name {
            pub fn new<L: Into<String>>(categories: impl IntoIterator<Item = L>) -> Self {
                $name {
                    title: None,
                    categories: categories.into_iter().map(Into::into).collect(),
                    series: Vec::new(),
                    curve: Curve::Linear,
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

            pub fn curve(mut self, curve: Curve) -> Self {
                self.curve = curve;
                self
            }

            /// The chart over categories `r` only: zoomed in, the y axis
            /// fits what is in view.
            fn windowed(&self, r: std::ops::Range<usize>) -> Self {
                let take = |v: &[f64]| v.iter().skip(r.start).take(r.len()).copied().collect();
                $name {
                    title: self.title.clone(),
                    categories: self.categories.iter().skip(r.start).take(r.len()).cloned().collect(),
                    series: self
                        .series
                        .iter()
                        .map(|s| Series {
                            name: s.name.clone(),
                            values: take(&s.values),
                        })
                        .collect(),
                    curve: self.curve,
                }
            }
        }
    };
}

series_chart!(
    /// One line per series across ordered categories.
    LineChart
);
series_chart!(
    /// Lines with a soft wash down to the zero baseline.
    AreaChart
);
series_chart!(
    /// Series stacked into bands; each band's top edge is drawn as a line.
    /// Values must be non-negative.
    StackedAreaChart
);

impl Chart for LineChart {
    fn category_count(&self) -> Option<usize> {
        Some(self.categories.len())
    }

    fn window(&self, r: std::ops::Range<usize>) -> Option<Box<dyn Chart>> {
        Some(Box::new(self.windowed(r)))
    }

    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        build(
            Kind::Line,
            self.curve,
            self.title.as_deref(),
            &self.categories,
            &self.series,
            width,
            height,
            measure,
        )
    }
}

impl Chart for AreaChart {
    fn category_count(&self) -> Option<usize> {
        Some(self.categories.len())
    }

    fn window(&self, r: std::ops::Range<usize>) -> Option<Box<dyn Chart>> {
        Some(Box::new(self.windowed(r)))
    }

    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        build(
            Kind::Area,
            self.curve,
            self.title.as_deref(),
            &self.categories,
            &self.series,
            width,
            height,
            measure,
        )
    }
}

impl Chart for StackedAreaChart {
    fn category_count(&self) -> Option<usize> {
        Some(self.categories.len())
    }

    fn window(&self, r: std::ops::Range<usize>) -> Option<Box<dyn Chart>> {
        Some(Box::new(self.windowed(r)))
    }

    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        build(
            Kind::Stacked,
            self.curve,
            self.title.as_deref(),
            &self.categories,
            &self.series,
            width,
            height,
            measure,
        )
    }
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum Kind {
    Line,
    Area,
    Stacked,
}

#[allow(clippy::too_many_arguments)]
fn build(
    kind: Kind,
    curve: Curve,
    title: Option<&str>,
    categories: &[String],
    series: &[Series],
    width: f64,
    height: f64,
    measure: &dyn TextMeasure,
) -> Result<Scene, Error> {
    check_size(width, height)?;
    check_series(categories, series, MAX_SERIES)?;
    if kind == Kind::Stacked {
        for s in series {
            if let Some(i) = s.values.iter().position(|&v| v < 0.0) {
                return Err(Error::InvalidData(format!(
                    "stacked area series {:?} is negative at {:?}",
                    s.name, categories[i]
                )));
            }
        }
    }
    let n = categories.len();
    // (lower, upper) value of each series at each category.
    let mut bands: Vec<Vec<(f64, f64)>> = Vec::with_capacity(series.len());
    let mut sums = vec![0.0; n];
    for s in series {
        bands.push(
            s.values
                .iter()
                .enumerate()
                .map(|(i, &v)| match kind {
                    Kind::Stacked => {
                        sums[i] += v;
                        (sums[i] - v, sums[i])
                    }
                    _ => (0.0, v),
                })
                .collect(),
        );
    }
    let uppers = bands.iter().flatten().map(|&(_, u)| u);
    let (lo, hi) = match kind {
        // Lines need not include zero; areas fill down to it.
        Kind::Line => uppers.fold((f64::INFINITY, f64::NEG_INFINITY), |(lo, hi), v| {
            (lo.min(v), hi.max(v))
        }),
        _ => uppers.fold((0.0_f64, 0.0_f64), |(lo, hi), v| (lo.min(v), hi.max(v))),
    };
    let (lo, hi) = if lo.is_finite() { (lo, hi) } else { (0.0, 0.0) };

    let names: Vec<String> = series.iter().map(|s| s.name.clone()).collect();
    let mut marks = Vec::new();
    let top = frame::title(&mut marks, title);
    let key = if kind == Kind::Line {
        Key::Line
    } else {
        Key::Swatch
    };
    let top = frame::legend(&mut marks, measure, top, width, &names, key);

    // Label line ends directly when there are few series and the labels
    // would not collide; otherwise fall back to legend and tooltip rather
    // than nudging labels away from their lines.
    let label_ends = n > 0 && (2..=MAX_END_LABELS).contains(&series.len());
    let reserve = if label_ends {
        names
            .iter()
            .map(|s| measure.width(s, TICK_SIZE, Weight::Regular))
            .fold(0.0, f64::max)
            + GAP
    } else {
        0.0
    };
    let layout = |reserve| {
        category_frame(
            measure,
            width,
            height,
            top,
            categories,
            (lo, hi),
            PADDING,
            reserve,
        )
    };
    let mut f = layout(reserve);
    let mut label_ends = label_ends;
    if label_ends {
        let mut ends: Vec<f64> = bands.iter().map(|b| f.y.scale.map(b[n - 1].1)).collect();
        ends.sort_by(f64::total_cmp);
        if ends
            .windows(2)
            .any(|w| w[1] - w[0] < TICK_SIZE + SURFACE_GAP)
        {
            label_ends = false;
            f = layout(0.0);
        }
    }

    f.grid(&mut marks);
    f.y_labels(&mut marks);
    let x: Vec<f64> = (0..n).map(|i| f.center(i)).collect();
    if kind != Kind::Line {
        for (j, b) in bands.iter().enumerate() {
            let shape = match curve {
                Curve::Linear => Shape::Area {
                    points: b
                        .iter()
                        .zip(&x)
                        .map(|(&(l, u), &x)| (x, f.y.scale.map(u), f.y.scale.map(l)))
                        .collect(),
                },
                // Out along the top edge and back along the bottom, both
                // curved alike, so a stacked band meets its neighbor's.
                Curve::Monotone => {
                    let edge = |v: fn(&(f64, f64)) -> f64| -> Vec<(f64, f64)> {
                        b.iter()
                            .zip(&x)
                            .map(|(p, &x)| (x, f.y.scale.map(v(p))))
                            .collect()
                    };
                    let mut lower = edge(|p| p.0);
                    lower.reverse();
                    let mut ops = monotone(&edge(|p| p.1), false);
                    ops.extend(monotone(&lower, true));
                    ops.push(PathOp::Close);
                    Shape::Path { ops }
                }
            };
            marks.push(
                Mark::new(shape)
                    .fill(Color::Wash(j as u8))
                    .series(names[j].as_str()),
            );
        }
    }
    for (j, b) in bands.iter().enumerate() {
        let points: Vec<(f64, f64)> = b
            .iter()
            .zip(&x)
            .map(|(&(_, u), &x)| (x, f.y.scale.map(u)))
            .collect();
        // A single point has no line to draw: show it as a dot.
        let shape = match points.as_slice() {
            [(cx, cy)] => Shape::Circle {
                cx: *cx,
                cy: *cy,
                r: DOT_RADIUS,
            },
            _ if curve == Curve::Linear => Shape::Polyline { points },
            _ => Shape::Path {
                ops: monotone(&points, false),
            },
        };
        let mark = match shape {
            Shape::Circle { .. } => Mark::new(shape).fill(Color::Series(j as u8)),
            _ => Mark::new(shape).stroke(Color::Series(j as u8), LINE_WIDTH),
        };
        marks.push(mark.series(names[j].as_str()));
    }
    f.baseline(&mut marks);

    // The crosshair finds the x: one column per category, listing every
    // series there, so the pointer never has to land on a 2px line.
    for (i, cat) in categories.iter().enumerate() {
        let rows = series
            .iter()
            .enumerate()
            .map(|(j, s)| TipRow {
                color: Some(Color::Series(j as u8)),
                name: s.name.clone(),
                value: format::value(s.values[i]),
            })
            .collect();
        marks.push(
            Mark::new(Shape::Line {
                x1: snap(x[i]),
                y1: f.top,
                x2: snap(x[i]),
                y2: f.bottom,
            })
            .stroke(Color::Axis, super::HAIRLINE)
            .hit(
                Hit::new(format!("x{i}"), cat.as_str(), "")
                    .kind(HitKind::Crosshair)
                    .rows(rows)
                    .area(f.column(i)),
            ),
        );
    }
    if label_ends {
        for (j, b) in bands.iter().enumerate() {
            marks.push(
                text(
                    x[n - 1] + GAP,
                    f.y.scale.map(b[n - 1].1),
                    &names[j],
                    TICK_SIZE,
                    Weight::Regular,
                    Anchor::Start,
                    Baseline::Middle,
                    Color::TextSecondary,
                )
                .series(names[j].as_str()),
            );
        }
    }
    f.x_labels(&mut marks, categories);

    let what = match kind {
        Kind::Line => "Line chart",
        Kind::Area => "Area chart",
        Kind::Stacked => "Stacked area chart",
    };
    Ok(Scene {
        width,
        height,
        label: describe(
            title,
            &format!(
                "{what} with {} and {}.",
                count(series.len(), "series", "series"),
                count(n, "point", "points")
            ),
        ),
        marks,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;

    fn build(c: &dyn Chart) -> Scene {
        c.scene(640.0, 400.0, &InterMetrics).unwrap()
    }

    fn crosshairs(s: &Scene) -> Vec<&Hit> {
        s.marks
            .iter()
            .filter_map(|m| m.hit.as_ref().filter(|h| h.kind == HitKind::Crosshair))
            .collect()
    }

    #[test]
    fn crosshair_tooltip_lists_every_series_at_that_x() {
        let c = LineChart::new(["Jan", "Feb", "Mar"])
            .series("North", [1.0, 2.0, 3.0])
            .series("South", [3.0, 2.5, 1250.0]);
        let s = build(&c);
        let x = crosshairs(&s);
        assert_eq!(x.len(), 3);
        assert_eq!(x[2].label, "Mar");
        let rows: Vec<(&str, &str)> = x[2]
            .rows
            .iter()
            .map(|r| (r.name.as_str(), r.value.as_str()))
            .collect();
        assert_eq!(rows, [("North", "3"), ("South", "1,250")]);
        // The columns tile the plot: hovering anywhere over Feb hits Feb.
        let feb = x[1].area.unwrap();
        let i = s.hit_test(feb.x + 1.0, feb.y + 1.0, &[]).unwrap();
        assert_eq!(s.marks[i].hit.as_ref().unwrap().label, "Feb");
    }

    #[test]
    fn few_separated_series_are_labelled_at_their_ends() {
        let c = LineChart::new(["a", "b"])
            .series("High", [10.0, 100.0])
            .series("Low", [5.0, 0.0]);
        let s = build(&c);
        let ends = s
            .marks
            .iter()
            .filter(
                |m| matches!(&m.shape, Shape::Text { text, .. } if text == "High" || text == "Low"),
            )
            .filter(|m| m.series.is_some())
            .count();
        assert_eq!(ends, 2);
    }

    #[test]
    fn colliding_end_labels_fall_back_to_the_legend() {
        let c = LineChart::new(["a", "b"])
            .series("One", [10.0, 50.0])
            .series("Two", [5.0, 50.5]);
        let s = build(&c);
        assert!(
            !s.marks
                .iter()
                .any(|m| matches!(m.shape, Shape::Text { .. }) && m.series.is_some()),
            "end labels must not be nudged apart"
        );
    }

    #[test]
    fn stacked_bands_sit_on_each_other_and_reject_negatives() {
        let c = StackedAreaChart::new(["a", "b"])
            .series("x", [1.0, 2.0])
            .series("y", [3.0, 4.0]);
        let s = build(&c);
        let areas: Vec<&Vec<(f64, f64, f64)>> = s
            .marks
            .iter()
            .filter_map(|m| match &m.shape {
                Shape::Area { points } => Some(points),
                _ => None,
            })
            .collect();
        // y's bottom edge is x's top edge.
        for (px, py) in areas[0].iter().zip(areas[1]) {
            assert!((px.1 - py.2).abs() < 1e-9);
        }
        let bad = StackedAreaChart::new(["a"]).series("x", [-1.0]);
        assert!(bad.scene(640.0, 400.0, &InterMetrics).is_err());
    }

    #[test]
    fn area_fills_with_a_wash_not_a_saturated_block() {
        let c = AreaChart::new(["a", "b"]).series("x", [1.0, 2.0]);
        let s = build(&c);
        assert!(s.marks.iter().any(|m| m.fill == Some(Color::Wash(0))));
        assert!(
            !s.marks
                .iter()
                .any(|m| matches!(m.shape, Shape::Area { .. }) && m.fill == Some(Color::Series(0)))
        );
    }

    fn paths(s: &Scene) -> Vec<&Vec<PathOp>> {
        s.marks
            .iter()
            .filter_map(|m| match &m.shape {
                Shape::Path { ops } => Some(ops),
                _ => None,
            })
            .collect()
    }

    #[test]
    fn a_monotone_line_passes_through_the_data_and_never_overshoots() {
        // A step: a naive spline would dip below the low and bulge above
        // the high, showing values the series never had.
        let c = LineChart::new(["a", "b", "c", "d", "e"]).series("x", [0.0, 0.0, 10.0, 10.0, 10.0]);
        let linear = build(&c);
        assert!(paths(&linear).is_empty(), "linear is the default");
        let data: Vec<f64> = linear
            .marks
            .iter()
            .find_map(|m| match &m.shape {
                Shape::Polyline { points } => Some(points.iter().map(|p| p.1).collect()),
                _ => None,
            })
            .unwrap();
        let s = build(&c.curve(Curve::Monotone));
        let p = paths(&s);
        assert_eq!(p.len(), 1);
        let ends: Vec<f64> = p[0]
            .iter()
            .map(|o| match *o {
                PathOp::Move(_, y) | PathOp::Cubic(.., y) => y,
                _ => panic!("expected a move and cubics, got {o:?}"),
            })
            .collect();
        assert_eq!(ends, data, "the curve runs through every value");
        let (low, high) = (data[0], data[4]);
        for &(_, y) in crate::scene::flatten(p[0]).iter().flatten() {
            assert!(y <= low + 1e-9 && y >= high - 1e-9, "overshoot to {y}");
        }
    }

    #[test]
    fn monotone_stacked_bands_still_meet_their_neighbors() {
        let c = StackedAreaChart::new(["a", "b", "c", "d"])
            .series("x", [1.0, 4.0, 2.0, 3.0])
            .series("y", [2.0, 1.0, 3.0, 1.0])
            .curve(Curve::Monotone);
        let s = build(&c);
        let p = paths(&s);
        // Two filled bands, then the two top-edge lines.
        assert_eq!(p.len(), 4);
        let cubics = |ops: &[PathOp]| -> Vec<[f64; 6]> {
            ops.iter()
                .filter_map(|o| match *o {
                    PathOp::Cubic(a, b, c, d, e, f) => Some([a, b, c, d, e, f]),
                    _ => None,
                })
                .collect()
        };
        // Band x's top edge comes first in its outline; band y's bottom edge
        // runs the same curve backward in the second half of its own.
        let top = cubics(p[0]);
        let mut bottom = cubics(p[1]);
        let bottom = bottom.split_off(3);
        assert_eq!((top.len() / 2, bottom.len()), (3, 3));
        for (k, t) in top[..3].iter().enumerate() {
            let b = bottom[2 - k];
            // Reversed: b's first control is t's second and vice versa.
            for (u, v) in [(t[2], b[0]), (t[3], b[1]), (t[0], b[2]), (t[1], b[3])] {
                assert!((u - v).abs() < 1e-9, "the bands part: {t:?} vs {b:?}");
            }
        }
        // The band's top line is the same curve as the fill's top edge.
        assert_eq!(cubics(p[2]), top[..3]);
    }
}
