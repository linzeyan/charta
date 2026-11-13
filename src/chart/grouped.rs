use super::bar::{BAR_MAX, PADDING, data_end};
use super::frame::{self, Key, category_frame};
use super::{Chart, MAX_SERIES, SURFACE_GAP, Series, check_series, check_size, count, describe};
use crate::base::format;
use crate::base::text::TextMeasure;
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Hit, Mark, Rect, Scene, Shape};

/// Several series side by side in each category (multi-set bar chart).
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::{Chart, GroupedBarChart};
///
/// let chart = GroupedBarChart::new(["Q1", "Q2", "Q3"])
///     .title("Revenue by region")
///     .series("North", [12.0, 15.0, 14.0])
///     .series("South", [9.0, 11.0, 16.0]);
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Revenue by region. Grouped bar chart with 2 series and 3 categories.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct GroupedBarChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub categories: Vec<String>,
    pub series: Vec<Series>,
}

impl GroupedBarChart {
    pub fn new<L: Into<String>>(categories: impl IntoIterator<Item = L>) -> Self {
        GroupedBarChart {
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

impl Chart for GroupedBarChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        check_series(&self.categories, &self.series, MAX_SERIES)?;
        let names: Vec<String> = self.series.iter().map(|s| s.name.clone()).collect();
        let values = self.series.iter().flat_map(|s| s.values.iter().copied());
        let (lo, hi) = values.fold((0.0_f64, 0.0_f64), |(lo, hi), v| (lo.min(v), hi.max(v)));

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

        let k = self.series.len();
        let bw = f.band.bandwidth();
        // Bars in a group touch, separated only by the surface gap.
        let thick = if k == 0 {
            0.0
        } else {
            BAR_MAX
                .min((bw - SURFACE_GAP * (k as f64 - 1.0)) / k as f64)
                .max(1.0)
        };
        let group = k as f64 * thick + (k as f64 - 1.0).max(0.0) * SURFACE_GAP;
        let y0 = f.y.scale.map(0.0);
        for (i, cat) in self.categories.iter().enumerate() {
            let col = f.column(i);
            let slice = col.w / k.max(1) as f64;
            for (j, s) in self.series.iter().enumerate() {
                let v = s.values[i];
                let y1 = f.y.scale.map(v);
                let len = (y1 - y0).abs();
                marks.push(
                    Mark::new(Shape::Rect {
                        rect: Rect {
                            x: f.band.position(i)
                                + (bw - group) / 2.0
                                + j as f64 * (thick + SURFACE_GAP),
                            y: y0.min(y1),
                            w: thick,
                            h: len,
                        },
                        radius: data_end(v, thick, len),
                    })
                    .fill(Color::Series(j as u8))
                    .series(s.name.as_str())
                    .hit(
                        Hit::new(
                            format!("{i}-{j}"),
                            format!("{} · {cat}", s.name),
                            format::value(v),
                        )
                        // The group's column, split evenly: no dead zones
                        // between bars.
                        .area(Rect {
                            x: col.x + j as f64 * slice,
                            w: slice,
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
                    "Grouped bar chart with {} and {}.",
                    count(k, "series", "series"),
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

    fn bars(s: &Scene) -> Vec<(Rect, &Mark)> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Rect { rect, .. }, Some(h)) if h.kind == crate::scene::HitKind::Mark => {
                    Some((*rect, m))
                }
                _ => None,
            })
            .collect()
    }

    #[test]
    fn bars_in_a_group_are_separated_by_the_surface_gap() {
        let c = GroupedBarChart::new(["Q1", "Q2"])
            .series("2024", [10.0, 20.0])
            .series("2025", [15.0, 5.0]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let b = bars(&s);
        assert_eq!(b.len(), 4);
        let gap = b[1].0.x - (b[0].0.x + b[0].0.w);
        assert!((gap - SURFACE_GAP).abs() < 1e-9, "gap {gap}");
        // Color follows the series, not the position.
        assert_eq!(b[0].1.fill, Some(Color::Series(0)));
        assert_eq!(b[1].1.fill, Some(Color::Series(1)));
        assert_eq!(b[3].1.series.as_deref(), Some("2025"));
    }

    #[test]
    fn two_series_get_a_legend_with_one_entry_each() {
        let c = GroupedBarChart::new(["a"])
            .series("x", [1.0])
            .series("y", [2.0]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let legend = s
            .marks
            .iter()
            .filter(|m| {
                m.hit
                    .as_ref()
                    .is_some_and(|h| h.kind == crate::scene::HitKind::Legend)
            })
            .count();
        assert_eq!(legend, 2);
    }

    #[test]
    fn mismatched_lengths_and_duplicate_names_are_rejected() {
        let short = GroupedBarChart::new(["a", "b"]).series("x", [1.0]);
        assert!(short.scene(640.0, 400.0, &InterMetrics).is_err());
        let dup = GroupedBarChart::new(["a"])
            .series("x", [1.0])
            .series("x", [2.0]);
        assert!(dup.scene(640.0, 400.0, &InterMetrics).is_err());
    }

    #[test]
    fn label_reads_as_english_and_ignores_an_empty_title() {
        // Screen readers speak this label; "seriess" or ". Grouped…" is noise.
        let c = GroupedBarChart::new(["Q1", "Q2"])
            .title("")
            .series("North", [1.0, 2.0])
            .series("South", [3.0, 4.0]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        assert_eq!(s.label, "Grouped bar chart with 2 series and 2 categories.");
        assert!(
            !s.marks
                .iter()
                .any(|m| matches!(&m.shape, Shape::Text { text, .. } if text.is_empty()))
        );
    }
}
