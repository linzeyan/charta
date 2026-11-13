use super::bar::BAR_RADIUS;
use super::frame::{self, xy_frame};
use super::{Chart, SURFACE_GAP, check_size, count, describe, finite};
use crate::base::format;
use crate::base::text::TextMeasure;
use crate::base::theme::Color;
use crate::error::Error;
use crate::layout::stats;
use crate::scene::{Corners, Hit, Mark, Rect, Scene, Shape};

/// The distribution of one variable as counts in equal-width bins.
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::{Chart, HistogramChart};
///
/// let waits = [
///     30.0, 37.0, 30.0, 29.0, 24.0, 30.0, 42.0, 36.0, 41.0, 34.0, 17.0, 40.0, 26.0, 47.0, 25.0,
/// ];
/// let chart = HistogramChart::new(waits)
///     .title("Support wait times")
///     .x_label("Minutes");
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Support wait times. Histogram of 15 values in 7 bins.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
pub struct HistogramChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    /// Name of the measured quantity, shown under the x axis.
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub x_label: Option<String>,
    pub values: Vec<f64>,
    /// About how many bins; edges snap to round numbers, so the result may
    /// differ by one or two. Defaults to Sturges' rule.
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub bins: Option<usize>,
}

impl HistogramChart {
    pub fn new(values: impl IntoIterator<Item = f64>) -> Self {
        HistogramChart {
            values: values.into_iter().collect(),
            ..Self::default()
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    pub fn x_label(mut self, label: impl Into<String>) -> Self {
        self.x_label = Some(label.into());
        self
    }

    pub fn bins(mut self, n: usize) -> Self {
        self.bins = Some(n);
        self
    }
}

impl Chart for HistogramChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        for (i, &v) in self.values.iter().enumerate() {
            finite(&format!("value {i}"), v)?;
        }
        if self.bins == Some(0) {
            return Err(Error::InvalidData("bins must be at least 1".into()));
        }
        let mut sorted = self.values.clone();
        sorted.sort_by(f64::total_cmp);
        let target = self.bins.unwrap_or_else(|| stats::sturges(sorted.len()));
        let (edges, counts) = stats::bins(&sorted, target);
        let (xlo, xhi) = match (edges.first(), edges.last()) {
            (Some(&a), Some(&b)) => (a, b),
            _ => (0.0, 1.0),
        };
        let max = counts.iter().copied().max().unwrap_or(0);

        let mut marks = Vec::new();
        let top = frame::title(&mut marks, self.title.as_deref());
        // Counts are whole numbers: never more ticks than the tallest count.
        let f = xy_frame(
            measure,
            width,
            height,
            top,
            (xlo, xhi),
            true,
            (0.0, max as f64),
            max.max(1),
            self.x_label.is_some(),
        );
        f.grid(&mut marks);
        let y0 = f.y.scale.map(0.0);
        for (i, &n) in counts.iter().enumerate() {
            let (x0, x1) = (f.x.scale.map(edges[i]), f.x.scale.map(edges[i + 1]));
            let y1 = f.y.scale.map(n as f64);
            // Adjacent bins keep a surface-colored gap so each reads as a bar.
            let w = (x1 - x0 - SURFACE_GAP).max(1.0);
            let h = y0 - y1;
            let r = BAR_RADIUS.min(w / 2.0).min(h);
            marks.push(
                Mark::new(Shape::Rect {
                    rect: Rect {
                        x: x0 + SURFACE_GAP / 2.0,
                        y: y1,
                        w,
                        h,
                    },
                    radius: Corners {
                        tl: r,
                        tr: r,
                        br: 0.0,
                        bl: 0.0,
                    },
                })
                .fill(Color::Series(0))
                .hit(
                    Hit::new(
                        i.to_string(),
                        format!(
                            "{}–{}",
                            format::value(edges[i]),
                            format::value(edges[i + 1])
                        ),
                        format::value(n as f64),
                    )
                    .area(Rect {
                        x: x0,
                        y: f.top,
                        w: x1 - x0,
                        h: f.bottom - f.top,
                    }),
                ),
            );
        }
        f.baseline(&mut marks);
        f.labels(&mut marks, self.x_label.as_deref());

        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Histogram of {} in {}.",
                    count(sorted.len(), "value", "values"),
                    count(counts.len(), "bin", "bins")
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

    fn bins(s: &Scene) -> Vec<(Rect, &Hit)> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Rect { rect, .. }, Some(h)) => Some((*rect, h)),
                _ => None,
            })
            .collect()
    }

    #[test]
    fn bar_heights_are_counts_and_every_value_is_counted() {
        let c = HistogramChart::new([1.0, 2.0, 2.5, 3.0, 3.5, 3.7, 8.0]).bins(4);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let b = bins(&s);
        let total: f64 = b.iter().map(|(_, h)| h.value.parse::<f64>().unwrap()).sum();
        assert_eq!(total, 7.0);
        // Height is proportional to the count.
        let per = b
            .iter()
            .find(|(_, h)| h.value == "1")
            .map(|(r, _)| r.h)
            .unwrap();
        for (r, h) in &b {
            let n: f64 = h.value.parse().unwrap();
            assert!((r.h - n * per).abs() < 1e-9, "{} vs {}", r.h, n * per);
        }
        // Labels name round edges.
        assert_eq!(b[0].1.label, "0–2");
    }

    #[test]
    fn bins_touch_but_keep_a_surface_gap() {
        let c = HistogramChart::new([0.0, 1.0, 2.0, 3.0, 4.0, 5.0]).bins(5);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let b = bins(&s);
        let gap = b[1].0.x - (b[0].0.x + b[0].0.w);
        assert!((gap - SURFACE_GAP).abs() < 1e-9, "gap {gap}");
    }

    #[test]
    fn empty_data_draws_an_empty_frame() {
        let s = HistogramChart::new([])
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        assert!(bins(&s).is_empty());
        assert!(
            HistogramChart::new([1.0])
                .bins(0)
                .scene(640.0, 400.0, &InterMetrics)
                .is_err()
        );
    }
}
