use super::frame::{self, Key, text};
use super::{
    Chart, GAP, MAX_SERIES, PAD, Series, TICK_SIZE, check_series, check_size, count, describe,
};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Corners, Hit, Mark, Rect, Scene, Shape, TipRow};

/// Percent ticks up the side: quarters read at a glance.
const QUARTERS: usize = 4;

/// Columns as wide as each category's total and split by series share: a
/// 100% stacked bar chart whose bar widths also carry a value, so every
/// cell's area is its share of the grand total.
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::{Chart, MarimekkoChart};
///
/// let chart = MarimekkoChart::new(["North America", "Europe", "Asia"])
///     .title("Sales by region and vendor (M$)")
///     .series("Vendor A", [120.0, 80.0, 60.0])
///     .series("Vendor B", [60.0, 90.0, 110.0]);
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Sales by region and vendor (M$). Marimekko chart of 2 series across 3 categories.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct MarimekkoChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub categories: Vec<String>,
    pub series: Vec<Series>,
}

impl MarimekkoChart {
    pub fn new<S: Into<String>>(categories: impl IntoIterator<Item = S>) -> Self {
        MarimekkoChart {
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

impl Chart for MarimekkoChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        check_series(&self.categories, &self.series, MAX_SERIES)?;
        for s in &self.series {
            if s.values.iter().any(|&v| v < 0.0) {
                return Err(Error::InvalidData(format!(
                    "series {:?} has a negative value; shares cannot be",
                    s.name
                )));
            }
        }
        let totals: Vec<f64> = (0..self.categories.len())
            .map(|i| self.series.iter().map(|s| s.values[i]).sum())
            .collect();
        let grand: f64 = totals.iter().sum();
        if !self.series.is_empty() && !self.categories.is_empty() && grand <= 0.0 {
            return Err(Error::InvalidData(
                "every value is zero: there is no whole to divide".into(),
            ));
        }

        let names: Vec<String> = self.series.iter().map(|s| s.name.clone()).collect();
        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &names, Key::Swatch);
        let plot_top = top + TICK_SIZE / 2.0;
        let bottom = (height - PAD - TICK_SIZE - GAP).max(plot_top + 1.0);
        let left = PAD + measure.width("100%", TICK_SIZE, Weight::Regular) + GAP;
        let right = (width - PAD).max(left + 1.0);
        let plot_h = bottom - plot_top;
        for k in 0..=QUARTERS {
            let frac = k as f64 / QUARTERS as f64;
            marks.push(text(
                left - GAP,
                bottom - frac * plot_h,
                &format!("{}%", 100 * k / QUARTERS),
                TICK_SIZE,
                Weight::Regular,
                Anchor::End,
                Baseline::Middle,
                Color::TextMuted,
            ));
        }
        let mut x = left;
        for (i, cat) in self.categories.iter().enumerate() {
            let col_w = if grand > 0.0 {
                (right - left) * totals[i] / grand
            } else {
                0.0
            };
            if totals[i] > 0.0 {
                let mut y = bottom;
                for (j, s) in self.series.iter().enumerate() {
                    let v = s.values[i];
                    let h = plot_h * v / totals[i];
                    if v > 0.0 {
                        // Half the 2px surface gap on every side.
                        let r = Rect {
                            x: x + 1.0,
                            y: y - h + 1.0,
                            w: (col_w - 2.0).max(0.0),
                            h: (h - 2.0).max(0.0),
                        };
                        marks.push(
                            Mark::new(Shape::Rect {
                                rect: r,
                                radius: Corners::all(0.0),
                            })
                            .fill(Color::Series(j as u8))
                            .series(s.name.as_str())
                            .hit(
                                Hit::new(format!("{i}-{j}"), cat.as_str(), "").rows(vec![
                                    TipRow {
                                        color: Some(Color::Series(j as u8)),
                                        name: s.name.clone(),
                                        value: format::value(v),
                                    },
                                    TipRow {
                                        color: None,
                                        name: format!("share of {cat}"),
                                        value: format!(
                                            "{}%",
                                            format::approx(v / totals[i] * 100.0)
                                        ),
                                    },
                                    TipRow {
                                        color: None,
                                        name: "share of total".into(),
                                        value: format!("{}%", format::approx(v / grand * 100.0)),
                                    },
                                ]),
                            ),
                        );
                    }
                    y -= h;
                }
                // A label too wide for its column is left to the tooltip.
                if measure.width(cat, TICK_SIZE, Weight::Regular) <= col_w - 4.0 {
                    marks.push(text(
                        x + col_w / 2.0,
                        bottom + GAP,
                        cat,
                        TICK_SIZE,
                        Weight::Regular,
                        Anchor::Middle,
                        Baseline::Top,
                        Color::TextSecondary,
                    ));
                }
            }
            x += col_w;
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Marimekko chart of {} across {}.",
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

    fn cells(s: &Scene) -> Vec<(Rect, String, String)> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Rect { rect, .. }, Some(h)) if h.kind == crate::scene::HitKind::Mark => {
                    Some((*rect, h.label.clone(), h.rows[0].name.clone()))
                }
                _ => None,
            })
            .collect()
    }

    #[test]
    fn every_cell_s_area_is_its_share_of_the_grand_total() {
        let c = MarimekkoChart::new(["A", "B"])
            .series("x", [30.0, 10.0])
            .series("y", [10.0, 50.0]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let cs = cells(&s);
        // Add back the 1px gap on each side, which is not data.
        let area = |r: Rect| (r.w + 2.0) * (r.h + 2.0);
        let total: f64 = cs.iter().map(|c| area(c.0)).sum();
        let share = |cat: &str, ser: &str| {
            area(cs.iter().find(|c| c.1 == cat && c.2 == ser).unwrap().0) / total
        };
        assert!((share("A", "x") - 0.3).abs() < 1e-9);
        assert!((share("B", "y") - 0.5).abs() < 1e-9);
    }

    #[test]
    fn columns_are_as_wide_as_their_totals_and_fill_the_height() {
        let c = MarimekkoChart::new(["A", "B"])
            .series("x", [10.0, 20.0])
            .series("y", [10.0, 40.0]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let cs = cells(&s);
        let (a, b) = (cs[0].0, cs[2].0);
        assert!(
            ((b.w + 2.0) / (a.w + 2.0) - 3.0).abs() < 1e-9,
            "B totals three times A"
        );
        let col_h: f64 = cs[..2].iter().map(|c| c.0.h + 2.0).sum();
        let col_h2: f64 = cs[2..].iter().map(|c| c.0.h + 2.0).sum();
        assert!((col_h - col_h2).abs() < 1e-9, "every column is 100%");
    }

    #[test]
    fn negative_and_all_zero_data_are_rejected() {
        let m = &InterMetrics;
        assert!(
            MarimekkoChart::new(["A"])
                .series("x", [-1.0])
                .scene(640.0, 400.0, m)
                .is_err()
        );
        assert!(
            MarimekkoChart::new(["A"])
                .series("x", [0.0])
                .scene(640.0, 400.0, m)
                .is_err()
        );
    }
}
