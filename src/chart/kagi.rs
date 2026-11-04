use super::frame::{self, KeySample, ValueAxis, hline, text};
use super::{
    Chart, GAP, PAD, TICK_SIZE, TICK_SPACING, TimeValue, check_size, check_times, count, describe,
};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::base::time;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Hit, Mark, Rect, Scene, Shape};

/// Yang lines (price broke above the last shoulder) are thick and in the
/// first slot; yin lines (below the last waist) thin and in the second.
/// Thickness is the traditional cue; color repeats it.
const YANG: (Color, f64) = (Color::Series(0), 3.0);
const YIN: (Color, f64) = (Color::Series(1), 1.5);
const DEFAULT_REVERSAL: f64 = 4.0;

/// A Kagi chart: a line that only turns when the price reverses by a set
/// percentage, ignoring time; it thickens on new highs and thins on new lows.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
pub struct KagiChart {
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
    /// Percent move against the line that turns it; default 4.
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub reversal: Option<f64>,
}

impl KagiChart {
    pub fn new(data: Vec<TimeValue>) -> Self {
        KagiChart {
            data,
            ..Self::default()
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    pub fn reversal(mut self, percent: f64) -> Self {
        self.reversal = Some(percent);
        self
    }
}

/// One vertical stroke of the line: from `from` to `to`, spanning the
/// prices between data indices `first` and `last`.
#[derive(Clone, Copy, Debug, PartialEq)]
pub(crate) struct Stroke {
    pub from: f64,
    pub to: f64,
    pub first: usize,
    pub last: usize,
}

/// The vertical strokes: a stroke extends while the price keeps going its
/// way and ends when the price retraces `pct` percent from its extreme.
pub(crate) fn strokes(prices: &[f64], pct: f64) -> Vec<Stroke> {
    let mut out = Vec::new();
    let Some(&p0) = prices.first() else {
        return out;
    };
    let mut cur = Stroke {
        from: p0,
        to: p0,
        first: 0,
        last: 0,
    };
    let mut dir = 0.0_f64;
    for (i, &p) in prices.iter().enumerate().skip(1) {
        let turn = cur.to.abs() * pct / 100.0;
        if dir == 0.0 {
            if (p - cur.from).abs() >= turn && p != cur.from {
                dir = if p > cur.from { 1.0 } else { -1.0 };
                cur.to = p;
                cur.last = i;
            }
        } else if (p - cur.to) * dir > 0.0 {
            cur.to = p;
            cur.last = i;
        } else if (cur.to - p) * dir >= turn {
            out.push(cur);
            cur = Stroke {
                from: cur.to,
                to: p,
                first: cur.last,
                last: i,
            };
            dir = -dir;
        }
    }
    if dir != 0.0 {
        out.push(cur);
    }
    out
}

impl Chart for KagiChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        check_times(&self.data)?;
        let pct = self.reversal.unwrap_or(DEFAULT_REVERSAL);
        if !(pct.is_finite() && pct > 0.0) {
            return Err(Error::InvalidData(
                "reversal must be a positive percentage".into(),
            ));
        }
        let offset = self.utc_offset.unwrap_or(0);
        let prices: Vec<f64> = self.data.iter().map(|d| d.value).collect();
        let st = strokes(&prices, pct);
        let (lo, hi) = prices
            .iter()
            .fold((f64::INFINITY, f64::NEG_INFINITY), |(a, b), &p| {
                (a.min(p), b.max(p))
            });
        let (lo, hi) = if lo.is_finite() { (lo, hi) } else { (0.0, 1.0) };

        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::static_key(
            &mut marks,
            measure,
            top,
            &[
                (KeySample::Line(YANG.0, YANG.1), "Yang: above the last high"),
                (KeySample::Line(YIN.0, YIN.1), "Yin: below the last low"),
            ],
        );
        let plot_top = top + TICK_SIZE / 2.0;
        let bottom = (height - PAD - TICK_SIZE - GAP).max(plot_top + 1.0);
        let count_y = (((bottom - plot_top) / TICK_SPACING).round() as usize).max(2);
        let y = ValueAxis::new(lo, hi, (bottom, plot_top), count_y);
        let y_w = y
            .ticks
            .iter()
            .map(|&t| measure.width(&y.fmt.format(t), TICK_SIZE, Weight::Regular))
            .fold(0.0, f64::max);
        let left = PAD + y_w + GAP;
        let right = (width - PAD).max(left + 1.0);
        // Time is not an axis here: each stroke gets an equal column.
        let col = (right - left) / st.len().max(1) as f64;
        let x = |k: usize| left + col * (k as f64 + 0.5);
        let py = |v: f64| y.scale.map(v);

        for &t in &y.ticks {
            marks.push(hline(left, right, super::snap(py(t)), Color::Grid));
            marks.push(text(
                left - GAP,
                py(t),
                &y.fmt.format(t),
                TICK_SIZE,
                Weight::Regular,
                Anchor::End,
                Baseline::Middle,
                Color::TextMuted,
            ));
        }

        // Walk the line, switching yin/yang where a stroke crosses the last
        // shoulder (top of a rise) or waist (bottom of a fall).
        // The starting price is the first waist (or shoulder) to beat.
        let mut yang = st.first().is_some_and(|s| s.to > s.from);
        let (mut shoulder, mut waist) = match st.first() {
            Some(s) if yang => (f64::INFINITY, s.from),
            Some(s) => (s.from, f64::NEG_INFINITY),
            None => (f64::INFINITY, f64::NEG_INFINITY),
        };
        let mut pieces: Vec<(bool, Vec<(f64, f64)>)> = Vec::new();
        let mut push = |yang: bool, a: (f64, f64), b: (f64, f64)| match pieces.last_mut() {
            Some((y, pts)) if *y == yang => pts.push(b),
            _ => pieces.push((yang, vec![a, b])),
        };
        for (k, s) in st.iter().enumerate() {
            if k > 0 {
                push(yang, (x(k - 1), py(s.from)), (x(k), py(s.from)));
            }
            let up = s.to > s.from;
            // A stroke starts on the far side of the level it may break, so
            // the switch happens exactly where it crosses that level.
            let level = if up { shoulder } else { waist };
            let breaks = if up {
                !yang && s.to > level
            } else {
                yang && s.to < level
            };
            if breaks {
                push(yang, (x(k), py(s.from)), (x(k), py(level)));
                yang = up;
                push(yang, (x(k), py(level)), (x(k), py(s.to)));
            } else {
                push(yang, (x(k), py(s.from)), (x(k), py(s.to)));
            }
            if up {
                shoulder = s.to;
            } else {
                waist = s.to;
            }
        }
        for (yang, points) in pieces {
            let (c, w) = if yang { YANG } else { YIN };
            marks.push(Mark::new(Shape::Polyline { points }).stroke(c, w));
        }
        for (k, s) in st.iter().enumerate() {
            let (a, b) = (&self.data[s.first], &self.data[s.last]);
            marks.push(
                Mark::new(Shape::Rect {
                    rect: Rect {
                        x: x(k) - col / 2.0,
                        y: plot_top,
                        w: col,
                        h: bottom - plot_top,
                    },
                    radius: Default::default(),
                })
                .hit(Hit::new(
                    k.to_string(),
                    format!(
                        "{} – {}",
                        time::format_date(a.time, offset),
                        time::format_date(b.time, offset)
                    ),
                    format!("{} → {}", format::value(s.from), format::value(s.to)),
                )),
            );
        }
        if let (Some(a), Some(b)) = (self.data.first(), self.data.last()) {
            for (t, x, anchor) in [(a.time, left, Anchor::Start), (b.time, right, Anchor::End)] {
                marks.push(text(
                    x,
                    bottom + GAP,
                    &time::format_date(t, offset),
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
                    "Kagi chart of {} with a {}% reversal.",
                    count(self.data.len(), "price", "prices"),
                    format::value(pct)
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
    fn small_moves_against_the_line_are_ignored() {
        // 100 → 110, a 2% dip (ignored), 115, then a 10% drop turns it.
        let s = strokes(&[100.0, 110.0, 107.8, 115.0, 103.0, 101.0], 4.0);
        assert_eq!(s.len(), 2);
        assert_eq!((s[0].from, s[0].to), (100.0, 115.0));
        assert_eq!((s[1].from, s[1].to), (115.0, 101.0));
        // Strokes join: each starts where the last ended.
        assert_eq!(s[1].first, s[0].last);
    }

    #[test]
    fn the_line_thickens_only_above_the_last_shoulder() {
        let data: Vec<TimeValue> = [100.0, 120.0, 105.0, 125.0, 90.0]
            .iter()
            .enumerate()
            .map(|(i, &v)| TimeValue::new(i as i64 * time::DAY, v))
            .collect();
        let s = KagiChart::new(data)
            .scene(640.0, 400.0, &crate::base::text::InterMetrics)
            .unwrap();
        let widths: Vec<f64> = s
            .marks
            .iter()
            .filter_map(|m| match m.shape {
                Shape::Polyline { .. } => m.stroke.map(|s| s.width),
                _ => None,
            })
            .collect();
        // Up (yang), down to 105 stays yang (above the 100 waist), up past
        // 120 stays yang, down past 105 flips to yin: yang then yin.
        assert_eq!(widths, vec![YANG.1, YIN.1]);
    }
}
