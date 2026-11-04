use super::frame::{self, CategoryFrame, category_frame};
use super::{
    Chart, DOT_RADIUS, LINE_WIDTH, MIN_TARGET, Sample, check_samples, check_size, count, describe,
};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::layout::stats::{self, BoxStats};
use crate::scene::{Anchor, Baseline, Corners, Hit, Mark, PathOp, Rect, Scene, Shape, TipRow};

const PADDING: (f64, f64) = (0.3, 0.15);
const BOX_MAX: f64 = 48.0;
const VIOLIN_MAX: f64 = 96.0;
/// Points along each violin's outline (per side).
const VIOLIN_SAMPLES: usize = 48;
/// Width of the interquartile bar inside a violin.
const INNER: f64 = 6.0;
const OUTLINE: f64 = 1.5;

macro_rules! sample_chart {
    ($(#[$doc:meta])* $name:ident) => {
        $(#[$doc])*
        #[derive(Clone, Debug, Default, PartialEq)]
        #[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
        #[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
        pub struct $name {
            #[cfg_attr(feature = "serde", serde(default, skip_serializing_if = "Option::is_none"))]
            pub title: Option<String>,
            /// Name of the measured quantity, shown above the value axis.
            #[cfg_attr(feature = "serde", serde(default, skip_serializing_if = "Option::is_none"))]
            pub y_label: Option<String>,
            pub groups: Vec<Sample>,
        }

        impl $name {
            pub fn new() -> Self {
                Self::default()
            }

            pub fn title(mut self, title: impl Into<String>) -> Self {
                self.title = Some(title.into());
                self
            }

            pub fn y_label(mut self, label: impl Into<String>) -> Self {
                self.y_label = Some(label.into());
                self
            }

            pub fn group(mut self, name: impl Into<String>, values: impl IntoIterator<Item = f64>) -> Self {
                self.groups.push(Sample::new(name, values));
                self
            }
        }
    };
}

sample_chart!(
    /// Each group's quartiles as a box, whiskers to the last values inside
    /// 1.5 × IQR, and the values beyond as outliers.
    BoxPlotChart
);
sample_chart!(
    /// Each group's distribution as a mirrored density, with its quartiles
    /// inside.
    ViolinChart
);

/// What both charts share: validated groups, their summaries and the frame.
struct Layout {
    sorted: Vec<Vec<f64>>,
    stats: Vec<BoxStats>,
    frame: CategoryFrame,
    marks: Vec<Mark>,
}

fn layout(
    title: Option<&str>,
    y_label: Option<&str>,
    groups: &[Sample],
    width: f64,
    height: f64,
    measure: &dyn TextMeasure,
) -> Result<Layout, Error> {
    check_size(width, height)?;
    check_samples(groups, usize::MAX, "group")?;
    let sorted: Vec<Vec<f64>> = groups.iter().map(Sample::sorted).collect();
    let stats: Vec<BoxStats> = sorted.iter().map(|v| stats::box_stats(v)).collect();
    let lo = sorted.iter().map(|v| v[0]).fold(f64::INFINITY, f64::min);
    let hi = sorted
        .iter()
        .map(|v| v[v.len() - 1])
        .fold(f64::NEG_INFINITY, f64::max);
    let (lo, hi) = if lo.is_finite() { (lo, hi) } else { (0.0, 1.0) };

    let mut marks = Vec::new();
    let mut top = frame::title(&mut marks, title);
    if let Some(l) = y_label.filter(|l| !l.is_empty()) {
        marks.push(frame::text(
            super::PAD,
            top,
            l,
            super::TICK_SIZE,
            Weight::Regular,
            Anchor::Start,
            Baseline::Top,
            Color::TextMuted,
        ));
        top += super::TICK_SIZE + super::GAP;
    }
    let labels: Vec<String> = groups.iter().map(|g| g.name.clone()).collect();
    let frame = category_frame(measure, width, height, top, &labels, (lo, hi), PADDING, 0.0);
    frame.grid(&mut marks);
    frame.y_labels(&mut marks);
    Ok(Layout {
        sorted,
        stats,
        frame,
        marks,
    })
}

/// The tooltip of a group: its summary, largest first, as on the plot.
fn summary(b: &BoxStats, n: usize) -> Vec<TipRow> {
    let row = |name: &str, value: String| TipRow {
        color: None,
        name: name.into(),
        value,
    };
    let mut rows = vec![
        row("upper whisker", format::value(b.high)),
        row("Q3", format::approx(b.q3)),
        row("median", format::approx(b.median)),
        row("Q1", format::approx(b.q1)),
        row("lower whisker", format::value(b.low)),
    ];
    if !b.outliers.is_empty() {
        rows.push(row("outliers", format::value(b.outliers.len() as f64)));
    }
    rows.push(row("values", format::value(n as f64)));
    rows
}

impl Chart for BoxPlotChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        let Layout {
            sorted,
            stats,
            frame: f,
            mut marks,
        } = layout(
            self.title.as_deref(),
            self.y_label.as_deref(),
            &self.groups,
            width,
            height,
            measure,
        )?;
        let w = BOX_MAX.min(f.band.bandwidth());
        let y = |v: f64| f.y.scale.map(v);
        for (i, (g, b)) in self.groups.iter().zip(&stats).enumerate() {
            let cx = f.center(i);
            let line = |x1: f64, y1: f64, x2: f64, y2: f64| {
                Mark::new(Shape::Line { x1, y1, x2, y2 }).stroke(Color::Series(0), LINE_WIDTH)
            };
            marks.push(line(cx, y(b.low), cx, y(b.high)));
            for v in [b.low, b.high] {
                marks.push(line(cx - w / 4.0, y(v), cx + w / 4.0, y(v)));
            }
            marks.push(
                Mark::new(Shape::Rect {
                    rect: Rect {
                        x: cx - w / 2.0,
                        y: y(b.q3),
                        w,
                        h: y(b.q1) - y(b.q3),
                    },
                    radius: Corners::all(2.0_f64.min((y(b.q1) - y(b.q3)) / 2.0)),
                })
                .fill(Color::Series(0))
                .hit(
                    Hit::new(i.to_string(), g.name.as_str(), "")
                        .rows(summary(b, sorted[i].len()))
                        .area(f.column(i)),
                ),
            );
            // The median cuts the box in the surface color, legible on the fill.
            marks.push(
                Mark::new(Shape::Line {
                    x1: cx - w / 2.0,
                    y1: y(b.median),
                    x2: cx + w / 2.0,
                    y2: y(b.median),
                })
                .stroke(Color::Surface, LINE_WIDTH),
            );
            for (k, &v) in b.outliers.iter().enumerate() {
                marks.push(
                    Mark::new(Shape::Circle {
                        cx,
                        cy: y(v),
                        r: DOT_RADIUS - OUTLINE / 2.0,
                    })
                    .fill(Color::Surface)
                    .stroke(Color::Series(0), OUTLINE)
                    .hit(
                        Hit::new(
                            format!("{i}-o{k}"),
                            format!("{} · outlier", g.name),
                            format::value(v),
                        )
                        .area(Rect {
                            x: cx - MIN_TARGET / 2.0,
                            y: y(v) - MIN_TARGET / 2.0,
                            w: MIN_TARGET,
                            h: MIN_TARGET,
                        }),
                    ),
                );
            }
        }
        f.baseline(&mut marks);
        let labels: Vec<String> = self.groups.iter().map(|g| g.name.clone()).collect();
        f.x_labels(&mut marks, &labels);
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Box plot of {}.",
                    count(self.groups.len(), "group", "groups")
                ),
            ),
            marks,
        })
    }
}

impl Chart for ViolinChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        let Layout {
            sorted,
            stats,
            frame: f,
            mut marks,
        } = layout(
            self.title.as_deref(),
            self.y_label.as_deref(),
            &self.groups,
            width,
            height,
            measure,
        )?;
        let w = VIOLIN_MAX.min(f.band.bandwidth());
        let y = |v: f64| f.y.scale.map(v);
        // Each violin is trimmed to its own data range; one density scale
        // for all keeps their areas equal, so widths compare.
        let curves: Vec<(Vec<f64>, Vec<f64>)> = sorted
            .iter()
            .map(|v| {
                let xs = stats::grid(v[0], v[v.len() - 1], VIOLIN_SAMPLES);
                let d = stats::kde(v, stats::silverman(v), &xs);
                (xs, d)
            })
            .collect();
        let peak = curves
            .iter()
            .flat_map(|(_, d)| d)
            .fold(0.0_f64, |m, &d| m.max(d));
        for (i, (g, b)) in self.groups.iter().zip(&stats).enumerate() {
            let cx = f.center(i);
            let hit = Hit::new(i.to_string(), g.name.as_str(), "")
                .rows(summary(b, sorted[i].len()))
                .area(f.column(i));
            let (xs, d) = &curves[i];
            let spread = xs[0] < xs[xs.len() - 1];
            let mut inner_hit = Some(hit);
            if spread {
                let half = |k: usize| d[k] / peak * w / 2.0;
                let mut ops = vec![PathOp::Move(cx + half(0), y(xs[0]))];
                ops.extend((1..xs.len()).map(|k| PathOp::Line(cx + half(k), y(xs[k]))));
                ops.extend(
                    (0..xs.len())
                        .rev()
                        .map(|k| PathOp::Line(cx - half(k), y(xs[k]))),
                );
                ops.push(PathOp::Close);
                marks.push(
                    Mark::new(Shape::Path { ops })
                        .fill(Color::Wash(0))
                        .stroke(Color::Series(0), OUTLINE)
                        .hit(inner_hit.take().expect("hit not used yet")),
                );
            }
            let mut iqr = Mark::new(Shape::Rect {
                rect: Rect {
                    x: cx - INNER / 2.0,
                    y: y(b.q3),
                    w: INNER,
                    h: y(b.q1) - y(b.q3),
                },
                radius: Corners::all((INNER / 2.0).min((y(b.q1) - y(b.q3)) / 2.0)),
            })
            .fill(Color::Series(0));
            if let Some(h) = inner_hit {
                iqr = iqr.hit(h);
            }
            marks.push(iqr);
            marks.push(
                Mark::new(Shape::Circle {
                    cx,
                    cy: y(b.median),
                    r: INNER / 2.0 - 0.5,
                })
                .fill(Color::Surface),
            );
        }
        f.baseline(&mut marks);
        let labels: Vec<String> = self.groups.iter().map(|g| g.name.clone()).collect();
        f.x_labels(&mut marks, &labels);
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Violin plot of {}.",
                    count(self.groups.len(), "group", "groups")
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

    fn boxes(s: &Scene) -> Vec<(Rect, &Hit)> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Rect { rect, .. }, Some(h)) => Some((*rect, h)),
                _ => None,
            })
            .collect()
    }

    #[test]
    fn box_spans_the_quartiles_and_outliers_sit_beyond_the_whiskers() {
        let c = BoxPlotChart::new()
            .group("a", [1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 40.0])
            .group("b", [10.0, 11.0, 12.0]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let (r, h) = boxes(&s)[0];
        let row = |n: &str| h.rows.iter().find(|r| r.name == n).unwrap().value.clone();
        assert_eq!(
            (row("Q1"), row("median"), row("Q3")),
            ("3".into(), "5".into(), "7".into())
        );
        assert_eq!(row("upper whisker"), "8");
        assert_eq!(row("outliers"), "1");
        // The box's height is the IQR on the value scale; the outlier is
        // drawn far above the box.
        let outlier = s
            .marks
            .iter()
            .find_map(|m| match (&m.shape, &m.hit) {
                (Shape::Circle { cy, .. }, Some(h)) if h.label == "a · outlier" => Some(*cy),
                _ => None,
            })
            .unwrap();
        assert!(outlier < r.y);
    }

    #[test]
    fn hovering_an_outlier_wins_over_its_column() {
        let c = BoxPlotChart::new().group("a", [1.0, 2.0, 3.0, 4.0, 5.0, 40.0]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let (cx, cy) = s
            .marks
            .iter()
            .find_map(|m| match m.shape {
                Shape::Circle { cx, cy, .. } if m.hit.is_some() => Some((cx, cy)),
                _ => None,
            })
            .unwrap();
        let i = s.hit_test(cx + 6.0, cy, &[]).unwrap();
        assert_eq!(s.marks[i].hit.as_ref().unwrap().value, "40");
    }

    #[test]
    fn violins_mirror_and_share_one_density_scale() {
        let wide = (0..40).map(|i| (i % 7) as f64);
        let c = ViolinChart::new()
            .group("a", wide)
            .group("b", [3.0, 3.1, 2.9, 3.0, 3.05]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let outlines: Vec<&Vec<PathOp>> = s
            .marks
            .iter()
            .filter_map(|m| match &m.shape {
                Shape::Path { ops } => Some(ops),
                _ => None,
            })
            .collect();
        assert_eq!(outlines.len(), 2);
        let width = |ops: &Vec<PathOp>| {
            let xs: Vec<f64> = ops
                .iter()
                .filter_map(|o| match *o {
                    PathOp::Move(x, _) | PathOp::Line(x, _) => Some(x),
                    _ => None,
                })
                .collect();
            xs.iter().cloned().fold(f64::MIN, f64::max)
                - xs.iter().cloned().fold(f64::MAX, f64::min)
        };
        // The tight sample is denser, so it is the wider violin.
        assert!(width(outlines[1]) > width(outlines[0]));
        // Mirrored: the first point (right side) and last point (left side)
        // are equidistant from the center at the same height.
        if let (PathOp::Move(x0, y0), PathOp::Line(x1, y1)) =
            (outlines[0][0], outlines[0][outlines[0].len() - 2])
        {
            assert_eq!(y0, y1);
            let cx = (x0 + x1) / 2.0;
            assert!((x0 - cx - (cx - x1)).abs() < 1e-9);
        } else {
            panic!("unexpected outline");
        }
    }

    #[test]
    fn a_single_value_group_still_has_a_tooltip() {
        let c = ViolinChart::new().group("a", [5.0]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        assert_eq!(boxes(&s).len(), 1);
        assert!(
            BoxPlotChart::new()
                .group("a", [])
                .scene(640.0, 400.0, &InterMetrics)
                .is_err()
        );
    }
}
