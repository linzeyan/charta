use super::frame::{self, Key, ValueAxis, text, vline};
use super::{Chart, GAP, PAD, TICK_SIZE, TICK_SPACING, check_size, count, describe, finite};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Corners, Hit, Mark, Rect, Scene, Shape, TipRow};

/// Lines cross every other line, so the all-pairs palette limit applies.
const MAX_GROUPS: usize = 3;
const LINE: f64 = 1.5;
const HALO: f64 = 1.0;

/// One observation: a value on every dimension, optionally in a group.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct ParallelRow {
    pub label: String,
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub group: Option<String>,
    pub values: Vec<f64>,
}

impl ParallelRow {
    pub fn new(label: impl Into<String>, values: impl IntoIterator<Item = f64>) -> Self {
        ParallelRow {
            label: label.into(),
            group: None,
            values: values.into_iter().collect(),
        }
    }

    pub fn group(mut self, group: impl Into<String>) -> Self {
        self.group = Some(group.into());
        self
    }
}

/// Many numeric dimensions side by side, each on its own vertical axis;
/// every observation is a line through its values.
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::{Chart, ParallelChart, ParallelRow};
///
/// let chart = ParallelChart::new(["Price ($)", "Weight (kg)", "Battery (h)"])
///     .title("Laptops")
///     .row(ParallelRow::new("Air 13", [1099.0, 1.24, 18.0]).group("Ultrabook"))
///     .row(ParallelRow::new("Legion 5", [1299.0, 2.4, 5.0]).group("Gaming"))
///     .row(ParallelRow::new("Chromebook", [299.0, 1.5, 10.0]).group("Budget"));
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Laptops. Parallel coordinates plot of 3 rows across 3 dimensions.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct ParallelChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub dimensions: Vec<String>,
    pub rows: Vec<ParallelRow>,
}

impl ParallelChart {
    pub fn new<S: Into<String>>(dimensions: impl IntoIterator<Item = S>) -> Self {
        ParallelChart {
            title: None,
            dimensions: dimensions.into_iter().map(Into::into).collect(),
            rows: Vec::new(),
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    pub fn row(mut self, row: ParallelRow) -> Self {
        self.rows.push(row);
        self
    }
}

impl Chart for ParallelChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let d = self.dimensions.len();
        if d < 2 {
            return Err(Error::InvalidData(
                "a parallel coordinates plot needs at least 2 dimensions".into(),
            ));
        }
        let mut groups: Vec<String> = Vec::new();
        for (r, row) in self.rows.iter().enumerate() {
            if row.values.len() != d {
                return Err(Error::InvalidData(format!(
                    "row {r} ({:?}) has {} values for {d} dimensions",
                    row.label,
                    row.values.len()
                )));
            }
            for (k, &v) in row.values.iter().enumerate() {
                finite(&format!("row {r} {:?}", self.dimensions[k]), v)?;
            }
            if let Some(g) = &row.group
                && !groups.contains(g)
            {
                groups.push(g.clone());
            }
        }
        if groups.len() > MAX_GROUPS {
            return Err(Error::InvalidData(format!(
                "{} groups, at most {MAX_GROUPS}: lines cross, so every pair of colors must stay distinguishable",
                groups.len()
            )));
        }

        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &groups, Key::Line);
        let names_y = top;
        let plot_top = top + TICK_SIZE + GAP + TICK_SIZE / 2.0;
        let bottom = (height - PAD - TICK_SIZE / 2.0).max(plot_top + 1.0);
        let ticks = (((bottom - plot_top) / TICK_SPACING).round() as usize).max(2);
        let axes: Vec<ValueAxis> = (0..d)
            .map(|k| {
                let (lo, hi) = self
                    .rows
                    .iter()
                    .fold((f64::INFINITY, f64::NEG_INFINITY), |(lo, hi), r| {
                        (lo.min(r.values[k]), hi.max(r.values[k]))
                    });
                let (lo, hi) = if lo.is_finite() { (lo, hi) } else { (0.0, 1.0) };
                ValueAxis::new(lo, hi, (bottom, plot_top), ticks)
            })
            .collect();
        let name_w = |k: usize| measure.width(&self.dimensions[k], TICK_SIZE, Weight::Semibold);
        let tick_w = axes[0]
            .ticks
            .iter()
            .map(|&t| measure.width(&axes[0].fmt.format(t), TICK_SIZE, Weight::Regular))
            .fold(0.0, f64::max);
        let left = PAD + (tick_w + GAP / 2.0).max(name_w(0) / 2.0);
        let right = (width - PAD - name_w(d - 1) / 2.0).max(left + 1.0);
        let x = |k: usize| left + (right - left) * k as f64 / (d - 1) as f64;

        for k in 0..d {
            marks.push(vline(super::snap(x(k)), plot_top, bottom, Color::Axis));
        }
        let multi = groups.len() > 1;
        for (r, row) in self.rows.iter().enumerate() {
            let points: Vec<(f64, f64)> = (0..d)
                .map(|k| (x(k), axes[k].scale.map(row.values[k])))
                .collect();
            let g = row
                .group
                .as_ref()
                .and_then(|g| groups.iter().position(|o| o == g));
            let rows = self
                .dimensions
                .iter()
                .zip(&row.values)
                .map(|(n, &v)| TipRow {
                    color: None,
                    name: n.clone(),
                    value: format::value(v),
                })
                .collect();
            let label = match (&row.group, multi) {
                (Some(g), true) => format!("{} · {g}", row.label),
                _ => row.label.clone(),
            };
            let mut m = Mark::new(Shape::Polyline { points })
                .stroke(Color::Series(g.unwrap_or(0) as u8), LINE)
                .hit(Hit::new(format!("r{r}"), label, "").rows(rows));
            if let Some(gr) = &row.group {
                m = m.series(gr.as_str());
            }
            marks.push(m);
        }
        // Labels go on top of the lines they would otherwise hide under.
        for (k, axis) in axes.iter().enumerate() {
            marks.push(text(
                x(k),
                names_y,
                &self.dimensions[k],
                TICK_SIZE,
                Weight::Semibold,
                Anchor::Middle,
                Baseline::Top,
                Color::TextSecondary,
            ));
            for &t in &axis.ticks {
                let label = axis.fmt.format(t);
                // A surface-colored backing keeps the number legible where
                // lines run under it.
                let w = measure.width(&label, TICK_SIZE, Weight::Regular);
                marks.push(
                    Mark::new(Shape::Rect {
                        rect: Rect {
                            x: x(k) - GAP / 2.0 - w - HALO,
                            y: axis.scale.map(t) - TICK_SIZE / 2.0 - HALO,
                            w: w + 2.0 * HALO,
                            h: TICK_SIZE + 2.0 * HALO,
                        },
                        radius: Corners::all(2.0),
                    })
                    .fill(Color::Surface),
                );
                marks.push(text(
                    x(k) - GAP / 2.0,
                    axis.scale.map(t),
                    &label,
                    TICK_SIZE,
                    Weight::Regular,
                    Anchor::End,
                    Baseline::Middle,
                    Color::TextMuted,
                ));
            }
        }

        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Parallel coordinates plot of {} across {}.",
                    count(self.rows.len(), "row", "rows"),
                    count(d, "dimension", "dimensions")
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

    fn chart() -> ParallelChart {
        ParallelChart::new(["Price", "Rating", "Weight"])
            .row(ParallelRow::new("A", [10.0, 4.5, 1.2]).group("Budget"))
            .row(ParallelRow::new("B", [40.0, 3.0, 2.5]).group("Premium"))
            .row(ParallelRow::new("C", [25.0, 5.0, 0.8]).group("Budget"))
    }

    fn lines(s: &Scene) -> Vec<(&Vec<(f64, f64)>, &Mark)> {
        s.marks
            .iter()
            .filter_map(|m| match &m.shape {
                Shape::Polyline { points } => Some((points, m)),
                _ => None,
            })
            .collect()
    }

    #[test]
    fn each_dimension_has_its_own_scale() {
        let s = chart().scene(640.0, 400.0, &InterMetrics).unwrap();
        let l = lines(&s);
        // B has the highest price but the lowest rating: its line is the
        // highest on axis 0 and the lowest on axis 1.
        let y = |i: usize, k: usize| l[i].0[k].1;
        assert!(y(1, 0) < y(0, 0) && y(1, 0) < y(2, 0));
        assert!(y(1, 1) > y(0, 1) && y(1, 1) > y(2, 1));
        // Every line visits every axis at the same x.
        assert!(l.iter().all(|(p, _)| p.len() == 3 && p[1].0 == l[0].0[1].0));
    }

    #[test]
    fn groups_color_lines_and_the_tooltip_lists_every_dimension() {
        let s = chart().scene(640.0, 400.0, &InterMetrics).unwrap();
        let l = lines(&s);
        assert_eq!(l[0].1.series.as_deref(), Some("Budget"));
        assert_eq!(l[1].1.stroke.unwrap().color, Color::Series(1));
        let h = l[1].1.hit.as_ref().unwrap();
        assert_eq!(h.label, "B · Premium");
        assert_eq!(h.rows.len(), 3);
        assert_eq!(h.rows[0].value, "40");
    }

    #[test]
    fn mismatched_rows_and_too_many_groups_are_rejected() {
        let m = &InterMetrics;
        let short = ParallelChart::new(["a", "b"]).row(ParallelRow::new("x", [1.0]));
        assert!(short.scene(640.0, 400.0, m).is_err());
        let mut many = ParallelChart::new(["a", "b"]);
        for g in ["p", "q", "r", "s"] {
            many = many.row(ParallelRow::new(g, [1.0, 2.0]).group(g));
        }
        assert!(many.scene(640.0, 400.0, m).is_err());
    }
}
