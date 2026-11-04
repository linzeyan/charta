use super::frame::{self, KeySample, hline, text};
use super::{Chart, GAP, PAD, TICK_SIZE, TimeValue, check_size, check_times, count, describe};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::base::{ticks, time};
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Hit, Mark, PathOp, Rect, Scene, Shape, TipRow};

const RISE: Color = Color::Series(0);
const FALL: Color = Color::Series(1);
const CELL_MAX: f64 = 18.0;
/// Smaller boxes than this stop reading as X and O.
const CELL_MIN: f64 = 8.0;
const STROKE: f64 = 1.5;
const DEFAULT_REVERSAL: u32 = 3;
/// Box counts across the price range tried when no box size is given.
const AUTO_BOXES: [usize; 6] = [10, 15, 20, 30, 40, 60];
/// Prices exactly on a box edge must count as reaching it even after the
/// division leaves them a hair short.
const EPS: f64 = 1e-9;

/// A point-and-figure chart: columns of X (rising) and O (falling) boxes;
/// a new column starts only when the price reverses by several boxes.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
pub struct PointFigureChart {
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
    /// Prices in time order.
    pub data: Vec<TimeValue>,
    /// Price per box; defaults to a round step giving about 20 boxes.
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub box_size: Option<f64>,
    /// Boxes the price must reverse to start a new column; default 3.
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub reversal: Option<u32>,
}

impl PointFigureChart {
    pub fn new(data: Vec<TimeValue>) -> Self {
        PointFigureChart {
            data,
            ..Self::default()
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    pub fn box_size(mut self, size: f64) -> Self {
        self.box_size = Some(size);
        self
    }

    pub fn reversal(mut self, boxes: u32) -> Self {
        self.reversal = Some(boxes);
        self
    }
}

/// A column of boxes `low..=high` (box `k` spans `k·size .. (k+1)·size`),
/// begun at data index `first`.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub(crate) struct Column {
    pub rising: bool,
    pub low: i64,
    pub high: i64,
    pub first: usize,
}

/// An X box `k` fills once the price reaches its top edge; an O box once
/// it reaches its bottom edge.
pub(crate) fn columns(prices: &[f64], size: f64, reversal: u32) -> Vec<Column> {
    let top_box = |p: f64| (p / size + EPS).floor() as i64 - 1;
    let bottom_box = |p: f64| (p / size - EPS).ceil() as i64;
    let r = reversal as i64;
    let mut out: Vec<Column> = Vec::new();
    let Some(&p0) = prices.first() else {
        return out;
    };
    let start = (p0 / size + EPS).floor() as i64;
    for (i, &p) in prices.iter().enumerate().skip(1) {
        let Some(c) = out.last_mut() else {
            if top_box(p) >= start {
                out.push(Column {
                    rising: true,
                    low: start,
                    high: top_box(p),
                    first: i,
                });
            } else if bottom_box(p) < start {
                out.push(Column {
                    rising: false,
                    low: bottom_box(p),
                    high: start - 1,
                    first: i,
                });
            }
            continue;
        };
        if c.rising {
            if top_box(p) > c.high {
                c.high = top_box(p);
            } else if bottom_box(p) <= c.high - r {
                let high = c.high - 1;
                out.push(Column {
                    rising: false,
                    low: bottom_box(p),
                    high,
                    first: i,
                });
            }
        } else if bottom_box(p) < c.low {
            c.low = bottom_box(p);
        } else if top_box(p) >= c.low + r {
            let low = c.low + 1;
            out.push(Column {
                rising: true,
                low,
                high: top_box(p),
                first: i,
            });
        }
    }
    out
}

/// Box `k`'s lower edge as an exact decimal: 3 × 0.1 must print as 0.3.
fn box_price(size: f64, k: i64) -> f64 {
    let inv = (1.0 / size).round();
    if size < 1.0 && (1.0 / inv - size).abs() <= size * 1e-12 {
        k as f64 / inv
    } else {
        k as f64 * size
    }
}

/// The grid for one box size: columns, their box range and square cells.
struct Fit {
    size: f64,
    cols: Vec<Column>,
    b0: i64,
    b1: i64,
    left: f64,
    cell: f64,
}

fn fit(
    prices: &[f64],
    size: f64,
    reversal: u32,
    measure: &dyn TextMeasure,
    width: f64,
    plot_h: f64,
) -> Fit {
    let cols = columns(prices, size, reversal);
    let (b0, b1) = cols.iter().fold((i64::MAX, i64::MIN), |(a, b), c| {
        (a.min(c.low), b.max(c.high))
    });
    if cols.is_empty() {
        return Fit {
            size,
            cols,
            b0: 0,
            b1: 0,
            left: PAD,
            cell: 0.0,
        };
    }
    let label_w = (b0..=b1 + 1)
        .map(|k| {
            measure.width(
                &format::value(box_price(size, k)),
                TICK_SIZE,
                Weight::Regular,
            )
        })
        .fold(0.0, f64::max);
    let left = PAD + label_w + GAP;
    let cell = ((width - PAD - left) / cols.len() as f64)
        .min(plot_h / (b1 - b0 + 1) as f64)
        .min(CELL_MAX);
    Fit {
        size,
        cols,
        b0,
        b1,
        left,
        cell,
    }
}

impl Chart for PointFigureChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        check_times(&self.data)?;
        let offset = self.utc_offset.unwrap_or(0);
        let reversal = self.reversal.unwrap_or(DEFAULT_REVERSAL);
        if reversal == 0 {
            return Err(Error::InvalidData("reversal must be at least 1 box".into()));
        }
        let prices: Vec<f64> = self.data.iter().map(|d| d.value).collect();
        let (lo, hi) = prices
            .iter()
            .fold((f64::INFINITY, f64::NEG_INFINITY), |(a, b), &p| {
                (a.min(p), b.max(p))
            });

        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::static_key(
            &mut marks,
            measure,
            top,
            &[
                (KeySample::Line(RISE, 2.0), "X: rising"),
                (KeySample::Line(FALL, 2.0), "O: falling"),
            ],
        );
        let bottom = height - PAD - TICK_SIZE - GAP;
        let plot_h = bottom - top - TICK_SIZE / 2.0;
        let Fit {
            size,
            cols,
            b0,
            b1,
            left,
            cell,
        } = match self.box_size {
            Some(b) if b.is_finite() && b > 0.0 => {
                fit(&prices, b, reversal, measure, width, plot_h)
            }
            Some(_) => return Err(Error::InvalidData("box size must be positive".into())),
            // The box size sets both how much noise is filtered and the
            // grid's shape; with cells square, pick the nice size whose grid
            // fills the plot best, among those whose cells stay legible.
            None if lo < hi => {
                let mut sizes: Vec<f64> = AUTO_BOXES
                    .iter()
                    .filter_map(|&n| match ticks::ticks(lo, hi, n)[..] {
                        [a, b, ..] => Some(b - a),
                        _ => None,
                    })
                    .collect();
                sizes.dedup();
                let area = (width - PAD) * plot_h;
                let fill = |f: &Fit| {
                    let used = f.cell * f.cols.len() as f64 * f.cell * (f.b1 - f.b0 + 1) as f64;
                    (f.cell >= CELL_MIN, used / area)
                };
                sizes
                    .into_iter()
                    .map(|b| fit(&prices, b, reversal, measure, width, plot_h))
                    .reduce(|best, f| if fill(&f) > fill(&best) { f } else { best })
                    .unwrap_or_else(|| fit(&prices, 1.0, reversal, measure, width, plot_h))
            }
            None => fit(&prices, 1.0, reversal, measure, width, plot_h),
        };
        let price = |k: i64| box_price(size, k);
        if cols.is_empty() {
            return Ok(Scene {
                width,
                height,
                label: describe(
                    self.title.as_deref(),
                    "Point and figure chart with no columns.",
                ),
                marks,
            });
        }
        let rows = (b1 - b0 + 1) as f64;
        let grid_bottom = bottom;
        let grid_top = grid_bottom - rows * cell;
        let right = left + cols.len() as f64 * cell;
        let y_of = |k: i64| grid_bottom - (k - b0) as f64 * cell;
        // Label every n-th box edge so labels stay apart.
        let every = ((TICK_SIZE * 2.0) / cell).ceil().max(1.0) as i64;
        for k in b0..=b1 + 1 {
            if (k - b0) % every == 0 {
                marks.push(hline(left, right, super::snap(y_of(k)), Color::Grid));
                marks.push(text(
                    left - GAP,
                    y_of(k),
                    &format::value(price(k)),
                    TICK_SIZE,
                    Weight::Regular,
                    Anchor::End,
                    Baseline::Middle,
                    Color::TextMuted,
                ));
            }
        }
        let inset = cell * 0.2;
        for (c, col) in cols.iter().enumerate() {
            let x0 = left + c as f64 * cell;
            for k in col.low..=col.high {
                let (y1, y0) = (y_of(k + 1), y_of(k));
                let m = if col.rising {
                    Mark::new(Shape::Path {
                        ops: vec![
                            PathOp::Move(x0 + inset, y1 + inset),
                            PathOp::Line(x0 + cell - inset, y0 - inset),
                            PathOp::Move(x0 + cell - inset, y1 + inset),
                            PathOp::Line(x0 + inset, y0 - inset),
                        ],
                    })
                    .stroke(RISE, STROKE)
                } else {
                    Mark::new(Shape::Circle {
                        cx: x0 + cell / 2.0,
                        cy: (y0 + y1) / 2.0,
                        r: cell / 2.0 - inset,
                    })
                    .stroke(FALL, STROKE)
                };
                marks.push(m);
            }
            let boxes = col.high - col.low + 1;
            marks.push(
                Mark::new(Shape::Rect {
                    rect: Rect {
                        x: x0,
                        y: grid_top,
                        w: cell,
                        h: grid_bottom - grid_top,
                    },
                    radius: Default::default(),
                })
                .hit(
                    Hit::new(
                        c.to_string(),
                        format!(
                            "{} from {}",
                            if col.rising { "Rising" } else { "Falling" },
                            time::format_date(self.data[col.first].time, offset)
                        ),
                        format!(
                            "{}–{}",
                            format::value(price(col.low)),
                            format::value(price(col.high + 1))
                        ),
                    )
                    .rows(vec![TipRow {
                        color: None,
                        name: "boxes".into(),
                        value: boxes.to_string(),
                    }]),
                ),
            );
        }
        if let (Some(a), Some(b)) = (self.data.first(), self.data.last()) {
            let (first, last) = (
                time::format_date(a.time, offset),
                time::format_date(b.time, offset),
            );
            let both = measure.width(&first, TICK_SIZE, Weight::Regular)
                + measure.width(&last, TICK_SIZE, Weight::Regular)
                + GAP;
            let mut ends = vec![(first, left, Anchor::Start)];
            // A narrow grid keeps only the start date rather than overprint.
            if both <= right - left {
                ends.push((last, right, Anchor::End));
            }
            for (t, x, anchor) in ends {
                marks.push(text(
                    x,
                    grid_bottom + GAP,
                    &t,
                    TICK_SIZE,
                    Weight::Regular,
                    anchor,
                    Baseline::Top,
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
                    "Point and figure chart with {}, box size {}, {}-box reversal.",
                    count(cols.len(), "column", "columns"),
                    format::value(size),
                    reversal
                ),
            ),
            marks,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn columns_extend_with_the_trend_and_turn_on_a_full_reversal() {
        // Box 1, reversal 3: up to 15, a 2-box dip (ignored), on to 17,
        // then down to 12 (a reversal) and back to 13 (not one).
        let c = columns(&[10.0, 15.0, 13.0, 17.0, 12.0, 13.0], 1.0, 3);
        assert_eq!(
            c,
            vec![
                Column {
                    rising: true,
                    low: 10,
                    high: 16,
                    first: 1
                },
                Column {
                    rising: false,
                    low: 12,
                    high: 15,
                    first: 4
                },
            ]
        );
    }

    #[test]
    fn edges_count_despite_float_division() {
        // 0.3 / 0.1 is 2.9999999999999996; the price still fills box 2.
        let c = columns(&[0.0, 0.3], 0.1, 3);
        assert_eq!(c[0].high, 2);
    }

    #[test]
    fn rising_columns_are_xs_and_falling_ones_os() {
        let data: Vec<TimeValue> = [10.0, 15.0, 11.0]
            .iter()
            .enumerate()
            .map(|(i, &v)| TimeValue::new(i as i64 * time::DAY, v))
            .collect();
        let s = PointFigureChart::new(data)
            .box_size(1.0)
            .scene(640.0, 400.0, &crate::base::text::InterMetrics)
            .unwrap();
        let xs = s
            .marks
            .iter()
            .filter(|m| matches!(m.shape, Shape::Path { .. }) && m.hit.is_none())
            .count();
        let os = s
            .marks
            .iter()
            .filter(|m| matches!(m.shape, Shape::Circle { .. }))
            .count();
        // 10 → 15 fills boxes 10..14; the O column runs from 13 down to 11.
        assert_eq!((xs, os), (5, 3));
        let h: Vec<&str> = s
            .marks
            .iter()
            .filter_map(|m| m.hit.as_ref())
            .map(|h| h.value.as_str())
            .collect();
        assert_eq!(h, ["10–15", "11–14"]);
    }

    fn daily(prices: &[f64]) -> Vec<TimeValue> {
        prices
            .iter()
            .enumerate()
            .map(|(i, &v)| TimeValue::new(i as i64 * time::DAY, v))
            .collect()
    }

    #[test]
    fn the_automatic_box_size_keeps_every_cell_legible() {
        // A wide price range with many small swings: the finest nice box
        // would fill the plot with specks.
        let prices: Vec<f64> = (0..200)
            .map(|i| i as f64 * 0.5 + if i % 2 == 0 { 0.0 } else { 12.0 })
            .collect();
        let s = PointFigureChart::new(daily(&prices))
            .scene(640.0, 400.0, &crate::base::text::InterMetrics)
            .unwrap();
        for m in &s.marks {
            if let (Shape::Rect { rect, .. }, Some(_)) = (&m.shape, &m.hit) {
                assert!(rect.w >= CELL_MIN, "cell {}", rect.w);
            }
        }
    }

    #[test]
    fn a_grid_too_narrow_for_both_dates_shows_only_the_first() {
        // One rising column: the grid is a single cell wide.
        let s = PointFigureChart::new(daily(&[10.0, 40.0]))
            .box_size(1.0)
            .scene(640.0, 400.0, &crate::base::text::InterMetrics)
            .unwrap();
        let dates: Vec<&str> = s
            .marks
            .iter()
            .filter_map(|m| match &m.shape {
                Shape::Text { text, .. } if text.contains("1970") => Some(text.as_str()),
                _ => None,
            })
            .collect();
        assert_eq!(dates, ["Jan 1, 1970"]);
    }
}
