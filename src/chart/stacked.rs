use super::bar::{BAR_MAX, PADDING, data_end};
use super::frame::{self, Key, category_frame};
use super::{Chart, MAX_SERIES, SURFACE_GAP, Series, check_series, check_size, count, describe};
use crate::base::format;
use crate::base::text::TextMeasure;
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Corners, Hit, Mark, Rect, Scene, Shape};

/// Series stacked into one bar per category. Positive values stack up from
/// the baseline, negative ones down, each direction on its own.
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::{Chart, StackedBarChart};
///
/// let chart = StackedBarChart::new(["Q1", "Q2", "Q3"])
///     .title("Tickets by channel")
///     .series("Email", [40.0, 35.0, 42.0])
///     .series("Chat", [22.0, 30.0, 33.0]);
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Tickets by channel. Stacked bar chart with 2 series and 3 categories.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct StackedBarChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub categories: Vec<String>,
    pub series: Vec<Series>,
}

impl StackedBarChart {
    pub fn new<L: Into<String>>(categories: impl IntoIterator<Item = L>) -> Self {
        StackedBarChart {
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

impl Chart for StackedBarChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        check_series(&self.categories, &self.series, MAX_SERIES)?;
        let names: Vec<String> = self.series.iter().map(|s| s.name.clone()).collect();
        let (mut lo, mut hi) = (0.0_f64, 0.0_f64);
        for i in 0..self.categories.len() {
            let (pos, neg) = self.series.iter().fold((0.0, 0.0), |(p, n), s| {
                let v = s.values[i];
                if v > 0.0 { (p + v, n) } else { (p, n + v) }
            });
            hi = hi.max(pos);
            lo = lo.min(neg);
        }

        let mut marks = Vec::new();
        let top = frame::title(&mut marks, self.title.as_deref());
        let top = frame::legend(&mut marks, measure, top, width, &names, Key::Swatch);
        let f = category_frame(
            measure,
            width,
            height,
            top,
            &self.categories,
            (lo, hi),
            PADDING,
            0.0,
        );
        f.grid(&mut marks);
        f.y_labels(&mut marks);

        let bw = f.band.bandwidth();
        let thick = BAR_MAX.min(bw);
        let half_gap = SURFACE_GAP / 2.0;
        for (i, cat) in self.categories.iter().enumerate() {
            let col = f.column(i);
            // The outermost segment in each direction gets the rounded end.
            let last_pos = self.series.iter().rposition(|s| s.values[i] > 0.0);
            let last_neg = self.series.iter().rposition(|s| s.values[i] < 0.0);
            let (mut pos, mut neg) = (0.0, 0.0);
            for (j, s) in self.series.iter().enumerate() {
                let v = s.values[i];
                if v == 0.0 {
                    continue;
                }
                let (from, to, outer) = if v > 0.0 {
                    pos += v;
                    (pos - v, pos, last_pos == Some(j))
                } else {
                    neg += v;
                    (neg - v, neg, last_neg == Some(j))
                };
                // `from` is the end nearer the baseline. Both ends shrink by
                // half the surface gap where they touch another segment.
                let (y_from, y_to) = (f.y.scale.map(from), f.y.scale.map(to));
                let inset_from = if from != 0.0 { half_gap } else { 0.0 };
                let inset_to = if outer { 0.0 } else { half_gap };
                let (top_y, bottom_y) = if v > 0.0 {
                    (y_to + inset_to, y_from - inset_from)
                } else {
                    (y_from + inset_from, y_to - inset_to)
                };
                let h = (bottom_y - top_y).max(0.0);
                let radius = if outer {
                    data_end(v, thick, h)
                } else {
                    Corners::default()
                };
                marks.push(
                    Mark::new(Shape::Rect {
                        rect: Rect {
                            x: f.band.position(i) + (bw - thick) / 2.0,
                            y: top_y,
                            w: thick,
                            h,
                        },
                        radius,
                    })
                    .fill(Color::Series(j as u8))
                    .series(s.name.as_str())
                    .hit(
                        Hit::new(
                            format!("{i}-{j}"),
                            format!("{} · {cat}", s.name),
                            format::value(v),
                        )
                        .area(Rect {
                            y: top_y - half_gap,
                            h: h + SURFACE_GAP,
                            ..col
                        }),
                    ),
                );
            }
        }
        f.baseline(&mut marks);
        f.x_labels(&mut marks, &self.categories);

        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Stacked bar chart with {} and {}.",
                    count(self.series.len(), "series", "series"),
                    count(self.categories.len(), "category", "categories")
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
    use crate::scene::HitKind;

    fn segments(s: &Scene) -> Vec<(Rect, Corners)> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Rect { rect, radius }, Some(h)) if h.kind == HitKind::Mark => {
                    Some((*rect, *radius))
                }
                _ => None,
            })
            .collect()
    }

    #[test]
    fn segments_stack_with_a_surface_gap_and_only_the_top_is_rounded() {
        let c = StackedBarChart::new(["a"])
            .series("x", [10.0])
            .series("y", [20.0]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let seg = segments(&s);
        // Series y sits on top of x.
        let (x, y) = (seg[0], seg[1]);
        let gap = x.0.y - (y.0.y + y.0.h);
        assert!((gap - SURFACE_GAP).abs() < 1e-9, "gap {gap}");
        assert_eq!(x.1, Corners::default());
        assert!(y.1.tl > 0.0 && y.1.bl == 0.0);
        // Heights stay proportional once the gap is added back.
        let per_unit = (x.0.h + SURFACE_GAP / 2.0) / 10.0;
        assert!((y.0.h + SURFACE_GAP / 2.0 - 20.0 * per_unit).abs() < 1e-9);
    }

    #[test]
    fn negatives_stack_downward_from_the_baseline() {
        let c = StackedBarChart::new(["a"])
            .series("x", [10.0])
            .series("y", [-5.0]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let seg = segments(&s);
        let baseline = seg[0].0.y + seg[0].0.h;
        assert!((seg[1].0.y - baseline).abs() < 1e-9);
        // Each direction's outermost segment is rounded at its data end.
        assert!(seg[0].1.tl > 0.0 && seg[1].1.bl > 0.0);
    }
}
