use super::frame::{self, Key, halo_label, text};
use super::graph::band;
use super::{Chart, GAP, MAX_SERIES, PAD, TICK_SIZE, check_size, count, describe};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Corners, Hit, Mark, Rect, Scene, Shape, TipRow};

const BAR_W: f64 = 8.0;
/// Room between categories on an axis: more than a label's height, so
/// labels of neighboring small categories cannot collide.
const CAT_GAP: f64 = 16.0;

/// How many items share one category on every dimension.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Combination {
    pub categories: Vec<String>,
    pub count: f64,
}

/// Categorical dimensions side by side, each split by category share, with
/// ribbons tracing how items in one dimension's categories divide across
/// the next. Ribbons keep the first dimension's color all the way along.
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::{Chart, ParallelSetsChart};
///
/// let chart = ParallelSetsChart::new(["Class", "Survived"])
///     .title("Titanic passengers")
///     .combination(["First", "Yes"], 203.0)
///     .combination(["First", "No"], 122.0)
///     .combination(["Third", "Yes"], 178.0)
///     .combination(["Third", "No"], 528.0);
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Titanic passengers. Parallel sets of 1,031 across 2 dimensions.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct ParallelSetsChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub dimensions: Vec<String>,
    pub combinations: Vec<Combination>,
}

impl ParallelSetsChart {
    pub fn new<S: Into<String>>(dimensions: impl IntoIterator<Item = S>) -> Self {
        ParallelSetsChart {
            title: None,
            dimensions: dimensions.into_iter().map(Into::into).collect(),
            combinations: Vec::new(),
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    pub fn combination<S: Into<String>>(
        mut self,
        categories: impl IntoIterator<Item = S>,
        count: f64,
    ) -> Self {
        self.combinations.push(Combination {
            categories: categories.into_iter().map(Into::into).collect(),
            count,
        });
        self
    }
}

/// Each dimension's categories in order of first appearance, and each
/// combination's category index per dimension.
type Indexed = (Vec<Vec<String>>, Vec<Vec<usize>>);

impl ParallelSetsChart {
    fn index(&self) -> Result<Indexed, Error> {
        let d = self.dimensions.len();
        if d < 2 {
            return Err(Error::InvalidData(format!(
                "{}, at least 2: there is nothing to trace across",
                count(d, "dimension", "dimensions")
            )));
        }
        if self.combinations.is_empty() {
            return Err(Error::InvalidData(
                "no combinations: there is nothing to trace".into(),
            ));
        }
        let mut cats: Vec<Vec<String>> = vec![Vec::new(); d];
        let mut idx = Vec::with_capacity(self.combinations.len());
        for (k, c) in self.combinations.iter().enumerate() {
            if c.categories.len() != d {
                return Err(Error::InvalidData(format!(
                    "combination {k} has {} categories for {d} dimensions",
                    c.categories.len()
                )));
            }
            if !(c.count.is_finite() && c.count > 0.0) {
                return Err(Error::InvalidData(format!(
                    "combination {k} needs a positive count"
                )));
            }
            if self.combinations[..k]
                .iter()
                .any(|o| o.categories == c.categories)
            {
                return Err(Error::InvalidData(format!(
                    "combination {:?} is given twice",
                    c.categories
                )));
            }
            let row: Vec<usize> = (0..d)
                .map(
                    |j| match cats[j].iter().position(|x| *x == c.categories[j]) {
                        Some(i) => i,
                        None => {
                            cats[j].push(c.categories[j].clone());
                            cats[j].len() - 1
                        }
                    },
                )
                .collect();
            idx.push(row);
        }
        // The first dimension's categories color every ribbon.
        if cats[0].len() > MAX_SERIES {
            return Err(Error::InvalidData(format!(
                "{} categories in {:?}, at most {MAX_SERIES}: it colors the ribbons; \
                 fold the rest into \"Other\" or put another dimension first",
                cats[0].len(),
                self.dimensions[0]
            )));
        }
        Ok((cats, idx))
    }
}

impl Chart for ParallelSetsChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let (cats, idx) = self.index()?;
        let d = self.dimensions.len();
        let total: f64 = self.combinations.iter().map(|c| c.count).sum();
        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &cats[0], Key::Swatch);

        let widest = |j: usize| {
            cats[j]
                .iter()
                .map(|c| measure.width(c, TICK_SIZE, Weight::Regular))
                .fold(0.0, f64::max)
        };
        // Dimension names above, then the axes; first-axis labels to its
        // left, the rest to each axis's right.
        let names_y = top;
        let plot_top = top + TICK_SIZE + GAP;
        let bottom = (height - PAD).max(plot_top + 1.0);
        let left = PAD + widest(0) + GAP / 2.0;
        let right = (width - PAD - widest(d - 1) - GAP / 2.0).max(left + BAR_W);
        let axis_x = |j: usize| left + j as f64 * (right - left - BAR_W) / (d - 1) as f64;
        let most = cats.iter().map(Vec::len).max().unwrap_or(1);
        let ky = if total > 0.0 {
            ((bottom - plot_top - (most - 1) as f64 * CAT_GAP) / total).max(0.0)
        } else {
            0.0
        };

        // Category extents; each axis centered vertically.
        let mut sums: Vec<Vec<f64>> = cats.iter().map(|c| vec![0.0; c.len()]).collect();
        for (c, row) in self.combinations.iter().zip(&idx) {
            for j in 0..d {
                sums[j][row[j]] += c.count;
            }
        }
        let mut cat_y: Vec<Vec<f64>> = Vec::with_capacity(d);
        for j in 0..d {
            let h = total * ky + (cats[j].len() - 1) as f64 * CAT_GAP;
            let mut y = plot_top + (bottom - plot_top - h) / 2.0;
            let mut ys = Vec::with_capacity(cats[j].len());
            for &s in &sums[j] {
                ys.push(y);
                y += s * ky + CAT_GAP;
            }
            cat_y.push(ys);
        }

        // Ribbons between neighboring axes, one per (first category, left
        // category, right category). On the left they stack by right
        // category, on the right by left category, so ribbons from one
        // category stay together and cross as little as the data allows;
        // the first dimension's category orders within that.
        for j in 0..d - 1 {
            let mut groups: Vec<((usize, usize, usize), f64)> = Vec::new();
            for (c, row) in self.combinations.iter().zip(&idx) {
                let key = (row[0], row[j], row[j + 1]);
                match groups.iter_mut().find(|g| g.0 == key) {
                    Some(g) => g.1 += c.count,
                    None => groups.push((key, c.count)),
                }
            }
            let mut by_left: Vec<usize> = (0..groups.len()).collect();
            by_left.sort_by_key(|&g| (groups[g].0.1, groups[g].0.2, groups[g].0.0));
            let mut by_right: Vec<usize> = (0..groups.len()).collect();
            by_right.sort_by_key(|&g| (groups[g].0.2, groups[g].0.1, groups[g].0.0));
            let mut y_left = vec![0.0; groups.len()];
            let mut y_right = vec![0.0; groups.len()];
            let mut fill = cat_y[j].clone();
            for &g in &by_left {
                let a = groups[g].0.1;
                y_left[g] = fill[a];
                fill[a] += groups[g].1 * ky;
            }
            let mut fill = cat_y[j + 1].clone();
            for &g in &by_right {
                let b = groups[g].0.2;
                y_right[g] = fill[b];
                fill[b] += groups[g].1 * ky;
            }
            for (g, &((c0, a, b), v)) in groups.iter().enumerate() {
                let w = (v * ky).max(1.0);
                marks.push(
                    Mark::new(Shape::Path {
                        ops: band(axis_x(j) + BAR_W, y_left[g], axis_x(j + 1), y_right[g], w),
                    })
                    .fill(Color::Link(c0 as u8))
                    .series(cats[0][c0].as_str())
                    .hit(
                        Hit::new(
                            format!("ribbon-{j}-{g}"),
                            format!("{} → {}", cats[j][a], cats[j + 1][b]),
                            format::value(v),
                        )
                        .rows(vec![
                            TipRow {
                                color: Some(Color::Series(c0 as u8)),
                                name: self.dimensions[0].clone(),
                                value: cats[0][c0].clone(),
                            },
                            TipRow {
                                color: None,
                                name: "of all".into(),
                                value: format!("{}%", format::approx(v / total * 100.0)),
                            },
                        ]),
                    ),
                );
            }
        }

        for j in 0..d {
            let x = axis_x(j);
            let (anchor, nx) = match j {
                0 => (Anchor::Start, x),
                _ if j == d - 1 => (Anchor::End, x + BAR_W),
                _ => (Anchor::Middle, x + BAR_W / 2.0),
            };
            marks.push(text(
                nx,
                names_y,
                &self.dimensions[j],
                TICK_SIZE,
                Weight::Semibold,
                anchor,
                Baseline::Top,
                Color::TextPrimary,
            ));
            for (i, cat) in cats[j].iter().enumerate() {
                let (y, h) = (cat_y[j][i], (sums[j][i] * ky).max(1.0));
                // The first axis wears the ribbon colors, so its bars are
                // the color key; the others stay neutral.
                let (fill, series) = if j == 0 {
                    (Color::Series(i as u8), Some(cat.as_str()))
                } else {
                    (Color::TextMuted, None)
                };
                let mut m = Mark::new(Shape::Rect {
                    rect: Rect { x, y, w: BAR_W, h },
                    radius: Corners::all(0.0),
                })
                .fill(fill)
                .hit(
                    Hit::new(
                        format!("cat-{j}-{i}"),
                        format!("{}: {cat}", self.dimensions[j]),
                        format::value(sums[j][i]),
                    )
                    .rows(vec![TipRow {
                        color: None,
                        name: "of all".into(),
                        value: format!("{}%", format::approx(sums[j][i] / total * 100.0)),
                    }]),
                );
                if let Some(s) = series {
                    m = m.series(s);
                }
                marks.push(m);
                let cy = y + h / 2.0;
                if j == 0 {
                    marks.push(text(
                        x - GAP / 2.0,
                        cy,
                        cat,
                        TICK_SIZE,
                        Weight::Regular,
                        Anchor::End,
                        Baseline::Middle,
                        Color::TextSecondary,
                    ));
                } else if j == d - 1 {
                    marks.push(text(
                        x + BAR_W + GAP / 2.0,
                        cy,
                        cat,
                        TICK_SIZE,
                        Weight::Regular,
                        Anchor::Start,
                        Baseline::Middle,
                        Color::TextSecondary,
                    ));
                } else {
                    halo_label(
                        &mut marks,
                        measure,
                        (x + BAR_W + GAP / 2.0, cy),
                        cat,
                        Anchor::Start,
                        Color::TextSecondary,
                    );
                }
            }
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Parallel sets of {} across {}.",
                    format::value(total),
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
    use crate::scene::{HitKind, PathOp};

    fn titanic() -> ParallelSetsChart {
        ParallelSetsChart::new(["Class", "Sex", "Survived"])
            .combination(["First", "Female", "Yes"], 141.0)
            .combination(["First", "Male", "No"], 118.0)
            .combination(["First", "Male", "Yes"], 62.0)
            .combination(["Crew", "Male", "No"], 670.0)
            .combination(["Crew", "Male", "Yes"], 192.0)
            .combination(["Crew", "Female", "Yes"], 20.0)
    }

    fn marks_with<'a>(s: &'a Scene, prefix: &str) -> Vec<&'a Mark> {
        s.marks
            .iter()
            .filter(|m| {
                m.hit
                    .as_ref()
                    .is_some_and(|h| h.kind == HitKind::Mark && h.key.starts_with(prefix))
            })
            .collect()
    }

    fn rect(m: &Mark) -> Rect {
        match m.shape {
            Shape::Rect { rect, .. } => rect,
            _ => panic!(),
        }
    }

    #[test]
    fn every_axis_divides_the_same_total() {
        let s = titanic().scene(640.0, 400.0, &InterMetrics).unwrap();
        let mut per_axis = [0.0; 3];
        for m in marks_with(&s, "cat-") {
            let j: usize = m.hit.as_ref().unwrap().key[4..5].parse().unwrap();
            per_axis[j] += rect(m).h;
        }
        assert!((per_axis[0] - per_axis[1]).abs() < 1e-9);
        assert!((per_axis[0] - per_axis[2]).abs() < 1e-9);
    }

    #[test]
    fn ribbons_tile_each_category_and_keep_the_first_color() {
        let s = titanic().scene(640.0, 400.0, &InterMetrics).unwrap();
        let male = marks_with(&s, "cat-1-")
            .into_iter()
            .find(|m| m.hit.as_ref().unwrap().label == "Sex: Male")
            .map(rect)
            .unwrap();
        // Ribbons leaving Male toward Survived stack from Male's top to
        // its bottom with no gap or overlap.
        let mut spans: Vec<(f64, f64, Option<Color>)> = marks_with(&s, "ribbon-1-")
            .into_iter()
            .filter(|m| m.hit.as_ref().unwrap().label.starts_with("Male"))
            .map(|m| {
                let Shape::Path { ops } = &m.shape else {
                    panic!()
                };
                let (PathOp::Move(_, y0), PathOp::Line(_, _)) = (ops[0], ops[2]) else {
                    panic!()
                };
                let PathOp::Cubic(.., y3) = ops[3] else {
                    panic!()
                };
                (y0, y3, m.fill)
            })
            .collect();
        spans.sort_by(|a, b| a.0.total_cmp(&b.0));
        assert!((spans[0].0 - male.y).abs() < 1e-9);
        for w in spans.windows(2) {
            assert!((w[0].1 - w[1].0).abs() < 1e-9);
        }
        assert!((spans.last().unwrap().1 - (male.y + male.h)).abs() < 1e-9);
        // Male → No ribbons carry First (slot 0) and Crew (slot 1) colors.
        assert!(spans.iter().any(|s| s.2 == Some(Color::Link(0))));
        assert!(spans.iter().any(|s| s.2 == Some(Color::Link(1))));
    }

    #[test]
    fn mismatched_and_repeated_combinations_are_rejected() {
        let m = &InterMetrics;
        assert!(
            ParallelSetsChart::new(["A", "B"])
                .combination(["x"], 1.0)
                .scene(640.0, 400.0, m)
                .is_err()
        );
        assert!(
            ParallelSetsChart::new(["A", "B"])
                .combination(["x", "y"], 1.0)
                .combination(["x", "y"], 2.0)
                .scene(640.0, 400.0, m)
                .is_err()
        );
        assert!(
            ParallelSetsChart::new(["A"])
                .combination(["x"], 1.0)
                .scene(640.0, 400.0, m)
                .is_err()
        );
    }

    #[test]
    fn no_combinations_is_an_error_not_a_panic() {
        let c = ParallelSetsChart {
            dimensions: vec!["a".into(), "b".into()],
            ..Default::default()
        };
        assert!(c.scene(640.0, 400.0, &InterMetrics).is_err());
    }
}
