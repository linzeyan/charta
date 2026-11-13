use super::frame::{self, Key, TimeAxis, X_TICK_SPACING};
use super::{
    Chart, GAP, MAX_SERIES, PAD, Ranges, Series, TICK_SIZE, Viewable, check_size, count, describe,
    viewable,
};
use crate::base::format;
use crate::base::scale::LinearScale;
use crate::base::text::TextMeasure;
use crate::base::theme::Color;
use crate::base::time;
use crate::error::Error;
use crate::layout::{curve, stack};
use crate::scene::{Hit, HitKind, Mark, PathOp, Plot, Rect, Scene, Shape, TipRow};

/// A thin surface outline separates touching layers.
const SEAM: f64 = 1.0;

/// Stacked layers flowing around a moving center line: totals and their
/// mix over time, with the wiggle of each layer minimized.
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::base::time::to_ms;
/// use charta::{Chart, StreamChart};
///
/// let months = (1..=6).map(|m| to_ms(2025, m, 1, 0, 0, 0));
/// let chart = StreamChart::new(months)
///     .title("Listening hours by genre (k)")
///     .series("Pop", [22.0, 24.0, 23.0, 27.0, 30.0, 36.0])
///     .series("Rock", [18.0, 17.0, 19.0, 18.0, 16.0, 15.0])
///     .series("Jazz", [6.0, 6.0, 5.0, 5.0, 4.0, 4.0]);
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Listening hours by genre (k). Stream graph of 3 series over 6 times.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
pub struct StreamChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub utc_offset: Option<i32>,
    /// Sample instants (UTC epoch ms), increasing.
    pub times: Vec<i64>,
    /// One value per time, each ≥ 0.
    pub series: Vec<Series>,
}

impl StreamChart {
    pub fn new(times: impl IntoIterator<Item = i64>) -> Self {
        StreamChart {
            times: times.into_iter().collect(),
            ..Self::default()
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

viewable!(StreamChart);

/// Zoomed, the layers keep the shape the whole stream gives them: they are
/// laid out over all the data and cut at the view's edges, and only the
/// height refits to what is in view.
impl Viewable for StreamChart {
    fn laid_out(
        &self,
        width: f64,
        height: f64,
        measure: &dyn TextMeasure,
        view: Option<Ranges>,
    ) -> Result<(Scene, Plot), Error> {
        check_size(width, height)?;
        let offset = self.utc_offset.unwrap_or(0);
        // Dates, not epoch milliseconds, name the time in errors.
        let labels: Vec<String> = self
            .times
            .iter()
            .map(|&t| time::format_datetime(t, offset))
            .collect();
        super::check_series(&labels, &self.series, MAX_SERIES)?;
        for (i, w) in self.times.windows(2).enumerate() {
            if w[1] <= w[0] {
                return Err(Error::InvalidData(format!(
                    "time {}: times must increase",
                    i + 1
                )));
            }
        }
        for s in &self.series {
            if s.values.iter().any(|&v| v < 0.0) {
                return Err(Error::InvalidData(format!(
                    "series {:?} has a negative value; layers cannot be negative",
                    s.name
                )));
            }
        }
        let values: Vec<Vec<f64>> = self.series.iter().map(|s| s.values.clone()).collect();
        let order = stack::inside_out(&values);
        let edges = stack::wiggle(&values, &order);
        let (t0, t1) = match (view, self.times.first(), self.times.last()) {
            (Some(((a, b), _)), ..) => (a.round() as i64, b.round() as i64),
            (None, Some(&a), Some(&b)) => (a, b),
            _ => (0, time::DAY),
        };
        // The times in view, and for the height one more each side: a
        // layer's curve between two samples stays within their values.
        let inside = |t: i64| t >= t0 && t <= t1;
        let first = self
            .times
            .iter()
            .position(|&t| t >= t0)
            .unwrap_or(self.times.len());
        let last = self.times.iter().rposition(|&t| t <= t1);
        let reach = first.saturating_sub(1)
            ..=last.map_or(0, |k| (k + 1).min(self.times.len().saturating_sub(1)));
        let (lo, hi) = edges
            .iter()
            .flat_map(|e| e.get(reach.clone()).unwrap_or_default())
            .fold((f64::INFINITY, f64::NEG_INFINITY), |(a, b), &(l, h)| {
                (a.min(l), b.max(h))
            });
        let (lo, hi) = if lo < hi { (lo, hi) } else { (0.0, 1.0) };

        let names: Vec<String> = self.series.iter().map(|s| s.name.clone()).collect();
        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &names, Key::Swatch);
        let bottom = (height - PAD - TICK_SIZE - GAP).max(top + 1.0);
        let (left, right) = (PAD, (width - PAD).max(PAD + 1.0));
        let count_x = (((right - left) / X_TICK_SPACING).round() as usize).max(2);
        let mut x = TimeAxis::new(t0, t1, (left, right), count_x, offset);
        x.fit(measure, width - PAD);
        // No value axis: the stream's offset is arbitrary, only thickness
        // carries value. The crosshair reads exact values instead.
        let y = LinearScale::new((lo, hi), (bottom, top));
        x.grid(&mut marks, top, bottom);
        for &si in &order {
            let pts = |f: fn(&(f64, f64)) -> f64| -> Vec<(f64, f64)> {
                self.times
                    .iter()
                    .zip(&edges[si])
                    .map(|(&t, e)| (x.map(t), y.map(f(e))))
                    .collect()
            };
            let mut lower = pts(|e| e.0);
            lower.reverse();
            let (upper, lower) = (
                curve::monotone(&pts(|e| e.1), false),
                curve::monotone(&lower, true),
            );
            let mut ops = match view {
                None => [upper, lower].concat(),
                Some(_) => {
                    let (a, b) = x.scale.range;
                    [
                        curve::clip_x(&upper, a, b, false),
                        curve::clip_x(&lower, a, b, true),
                    ]
                    .concat()
                }
            };
            ops.push(PathOp::Close);
            marks.push(
                Mark::new(Shape::Path { ops })
                    .fill(Color::Series(si as u8))
                    .stroke(Color::Surface, SEAM)
                    .series(self.series[si].name.as_str()),
            );
        }
        let (px0, px1) = x.scale.range;
        for (i, &t) in self.times.iter().enumerate().filter(|&(_, &t)| inside(t)) {
            let cx = x.map(t);
            let half_left = if i > 0 {
                (cx - x.map(self.times[i - 1])) / 2.0
            } else {
                0.0
            };
            let half_right = if i + 1 < self.times.len() {
                (x.map(self.times[i + 1]) - cx) / 2.0
            } else {
                0.0
            };
            let rows = self
                .series
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
                    x1: super::snap(cx),
                    y1: top,
                    x2: super::snap(cx),
                    y2: bottom,
                })
                .stroke(Color::Axis, super::HAIRLINE)
                .hit(
                    Hit::new(format!("x{i}"), time::format_datetime(t, offset), "")
                        .kind(HitKind::Crosshair)
                        .rows(rows)
                        .area(match view {
                            None => Rect {
                                x: cx - half_left,
                                y: top,
                                w: half_left + half_right,
                                h: bottom - top,
                            },
                            // A column reaching past the view's edge stops at it.
                            Some(_) => {
                                let (a, b) =
                                    ((cx - half_left).max(px0), (cx + half_right).min(px1));
                                Rect {
                                    x: a,
                                    y: top,
                                    w: b - a,
                                    h: bottom - top,
                                }
                            }
                        }),
                ),
            );
        }
        x.labels(&mut marks, bottom + GAP);
        let plot = Plot {
            rect: Rect {
                x: px0,
                y: top,
                w: px1 - px0,
                h: bottom - top,
            },
            x: (t0 as f64, t1 as f64),
            y: None,
        };
        let shown = self.times.iter().filter(|&&t| inside(t)).count();
        let scene = Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Stream graph of {} over {}.",
                    count(self.series.len(), "series", "series"),
                    count(shown, "time", "times")
                ),
            ),
            marks,
        };
        Ok((scene, plot))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;

    fn chart() -> StreamChart {
        StreamChart::new((0..5).map(|i| i * time::WEEK))
            .series("Rock", [5.0, 6.0, 4.0, 3.0, 2.0])
            .series("Pop", [2.0, 4.0, 7.0, 9.0, 8.0])
            .series("Jazz", [1.0, 1.0, 2.0, 1.0, 1.0])
    }

    #[test]
    fn every_layer_is_a_closed_smooth_band() {
        let s = chart().scene(640.0, 400.0, &InterMetrics).unwrap();
        let layers: Vec<&Vec<PathOp>> = s
            .marks
            .iter()
            .filter_map(|m| match &m.shape {
                Shape::Path { ops } => Some(ops),
                _ => None,
            })
            .collect();
        assert_eq!(layers.len(), 3);
        for ops in layers {
            assert!(matches!(ops[0], PathOp::Move(..)));
            assert_eq!(*ops.last().unwrap(), PathOp::Close);
            assert!(ops.iter().any(|o| matches!(o, PathOp::Cubic(..))));
        }
    }

    #[test]
    fn the_crosshair_lists_every_series_at_that_time() {
        let s = chart().scene(640.0, 400.0, &InterMetrics).unwrap();
        let h = s
            .marks
            .iter()
            .filter_map(|m| m.hit.as_ref())
            .find(|h| h.key == "x2")
            .unwrap();
        let r: Vec<&str> = h.rows.iter().map(|r| r.value.as_str()).collect();
        assert_eq!(r, ["4", "7", "2"]);
    }

    #[test]
    fn negative_layers_are_rejected() {
        let c = StreamChart::new([0, 1]).series("a", [1.0, -1.0]);
        assert!(c.scene(640.0, 400.0, &InterMetrics).is_err());
    }

    #[test]
    fn a_view_cuts_the_layers_at_its_edges() {
        let x = (0.5 * time::WEEK as f64, 2.5 * time::WEEK as f64);
        let (s, p) = chart()
            .view(x, None)
            .unwrap()
            .plot_scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let p = p.unwrap();
        let (l, r) = (p.rect.x, p.rect.x + p.rect.w);
        let layers: Vec<&Vec<PathOp>> = s
            .marks
            .iter()
            .filter_map(|m| match &m.shape {
                Shape::Path { ops } => Some(ops),
                _ => None,
            })
            .collect();
        assert_eq!(layers.len(), 3);
        for ops in layers {
            let xs: Vec<f64> = crate::scene::flatten(ops)[0].iter().map(|q| q.0).collect();
            let (lo, hi) = xs
                .iter()
                .fold((f64::MAX, f64::MIN), |(a, b), &x| (a.min(x), b.max(x)));
            assert!(
                (lo - l).abs() < 1e-6 && (hi - r).abs() < 1e-6,
                "{lo}..{hi} vs {l}..{r}"
            );
        }
        // Weeks 1 and 2 are in view: two hover columns.
        assert!(s.label.contains("2 times"), "{}", s.label);
    }
}
