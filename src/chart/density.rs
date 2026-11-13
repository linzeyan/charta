use super::frame::{self, Key, xy_frame};
use super::{Chart, LINE_WIDTH, Sample, check_samples, check_size, count, describe};
use crate::base::format;
use crate::base::text::TextMeasure;
use crate::base::theme::Color;
use crate::error::Error;
use crate::layout::stats;
use crate::scene::{Hit, Mark, Scene, Shape, TipRow};

/// Overlapping curves are compared all against all, so only the palette's
/// first three slots stay distinguishable (as on a scatterplot).
const MAX_DENSITY_SERIES: usize = 3;
/// Points each curve is sampled at; the curves share one grid so their
/// areas line up.
const SAMPLES: usize = 96;
/// The support extends this many bandwidths past the data, where the
/// Gaussian tails have all but vanished.
const TAIL: f64 = 3.0;

/// Smoothed distributions (Gaussian kernel density estimates) of one or
/// more samples on a shared value axis.
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::{Chart, DensityChart};
///
/// let chart = DensityChart::new()
///     .title("Response time by region")
///     .x_label("ms")
///     .series("EU", [138.0, 117.0, 85.0, 145.0, 118.0, 109.0, 127.0, 147.0, 102.0, 140.0])
///     .series("US", [145.0, 119.0, 136.0, 177.0, 109.0, 175.0, 170.0, 150.0, 154.0, 121.0]);
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Response time by region. Density plot of 2 series.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
pub struct DensityChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub x_label: Option<String>,
    pub series: Vec<Sample>,
}

impl DensityChart {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    pub fn x_label(mut self, label: impl Into<String>) -> Self {
        self.x_label = Some(label.into());
        self
    }

    pub fn series(
        mut self,
        name: impl Into<String>,
        values: impl IntoIterator<Item = f64>,
    ) -> Self {
        self.series.push(Sample::new(name, values));
        self
    }
}

impl Chart for DensityChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        check_samples(&self.series, MAX_DENSITY_SERIES, "series")?;
        let sorted: Vec<Vec<f64>> = self.series.iter().map(Sample::sorted).collect();
        let bandwidths: Vec<f64> = sorted.iter().map(|v| stats::silverman(v)).collect();
        let (mut lo, mut hi) = (f64::INFINITY, f64::NEG_INFINITY);
        for (v, &h) in sorted.iter().zip(&bandwidths) {
            lo = lo.min(v[0] - TAIL * h);
            hi = hi.max(v[v.len() - 1] + TAIL * h);
        }
        if !lo.is_finite() {
            (lo, hi) = (0.0, 1.0);
        }
        let xs = stats::grid(lo, hi, SAMPLES);
        let curves: Vec<Vec<f64>> = sorted
            .iter()
            .zip(&bandwidths)
            .map(|(v, &h)| stats::kde(v, h, &xs))
            .collect();
        let peak = curves.iter().flatten().fold(0.0_f64, |m, &d| m.max(d));

        let names: Vec<String> = self.series.iter().map(|s| s.name.clone()).collect();
        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &names, Key::Line);
        let f = xy_frame(
            measure,
            width,
            height,
            top,
            (lo, hi),
            true,
            (0.0, peak),
            usize::MAX,
            self.x_label.is_some(),
        );
        f.grid(&mut marks);
        let y0 = f.y.scale.map(0.0);
        for (j, (s, ys)) in self.series.iter().zip(&curves).enumerate() {
            let pts: Vec<(f64, f64)> = xs
                .iter()
                .zip(ys)
                .map(|(&x, &d)| (f.x.scale.map(x), f.y.scale.map(d)))
                .collect();
            marks.push(
                Mark::new(Shape::Area {
                    points: pts.iter().map(|&(x, y)| (x, y, y0)).collect(),
                })
                .fill(Color::Wash(j as u8))
                .series(s.name.as_str()),
            );
            let v = &sorted[j];
            let rows = vec![
                TipRow {
                    color: None,
                    name: "values".into(),
                    value: format::value(v.len() as f64),
                },
                TipRow {
                    color: None,
                    name: "median".into(),
                    value: format::approx(stats::quantile(v, 0.5)),
                },
                TipRow {
                    color: None,
                    name: "bandwidth".into(),
                    value: format::approx(bandwidths[j]),
                },
            ];
            marks.push(
                Mark::new(Shape::Polyline { points: pts })
                    .stroke(Color::Series(j as u8), LINE_WIDTH)
                    .series(s.name.as_str())
                    .hit(Hit::new(format!("s{j}"), s.name.as_str(), "").rows(rows)),
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
                    "Density plot of {}.",
                    count(self.series.len(), "series", "series")
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

    fn curves(s: &Scene) -> Vec<&Vec<(f64, f64)>> {
        s.marks
            .iter()
            .filter_map(|m| match &m.shape {
                Shape::Polyline { points } => Some(points),
                _ => None,
            })
            .collect()
    }

    #[test]
    fn curves_share_one_grid_and_peak_where_the_data_clusters() {
        let c = DensityChart::new()
            .series("a", [1.0, 1.2, 1.1, 0.9, 5.0])
            .series("b", [4.0, 4.1, 3.9, 4.2]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let k = curves(&s);
        assert_eq!(k.len(), 2);
        let xs = |c: &Vec<(f64, f64)>| c.iter().map(|p| p.0).collect::<Vec<_>>();
        assert_eq!(xs(k[0]), xs(k[1]));
        // Higher density is higher up the screen (smaller y).
        let peak_x = |c: &Vec<(f64, f64)>| c.iter().min_by(|a, b| a.1.total_cmp(&b.1)).unwrap().0;
        assert!(peak_x(k[0]) < peak_x(k[1]));
    }

    #[test]
    fn hovering_a_curve_summarises_its_sample() {
        let c = DensityChart::new().series("a", [1.0, 2.0, 3.0]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let h = s.marks.iter().find_map(|m| m.hit.as_ref()).unwrap();
        assert_eq!(h.rows[0].value, "3");
        assert_eq!(h.rows[1].value, "2");
    }

    #[test]
    fn empty_samples_and_a_fourth_series_are_rejected() {
        let one = DensityChart::new().series("a", []);
        assert!(one.scene(640.0, 400.0, &InterMetrics).is_err());
        let mut four = DensityChart::new();
        for n in ["a", "b", "c", "d"] {
            four = four.series(n, [1.0]);
        }
        assert!(four.scene(640.0, 400.0, &InterMetrics).is_err());
    }
}
