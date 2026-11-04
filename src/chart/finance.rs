//! Price bars over time: candlesticks and open-high-low-close bars.

use super::bar::BAR_MAX;
use super::frame::{self, KeySample, rect_ops, time_frame};
use super::{Chart, Ranges, Viewable, check_size, count, describe, finite, viewable};
use crate::base::format;
use crate::base::text::TextMeasure;
use crate::base::theme::Color;
use crate::base::time::{self, DAY};
use crate::error::Error;
use crate::scene::{Hit, Mark, Plot, Rect, Scene, Shape, TipRow};

/// Rising and falling periods: the palette's first two slots (a CVD-safe
/// pair), never the reserved status red/green. Shape repeats the cue:
/// rising candles are hollow, falling ones solid.
const RISE: Color = Color::Series(0);
const FALL: Color = Color::Series(1);
const OUTLINE: f64 = 1.5;
/// A candle takes this share of the time between periods.
const FILL: f64 = 0.6;

/// One period's prices. `time` is the period's start, UTC epoch ms.
#[derive(Clone, Copy, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Candle {
    pub time: i64,
    pub open: f64,
    pub high: f64,
    pub low: f64,
    pub close: f64,
}

impl Candle {
    pub fn new(time: i64, open: f64, high: f64, low: f64, close: f64) -> Self {
        Candle {
            time,
            open,
            high,
            low,
            close,
        }
    }

    fn rising(&self) -> bool {
        self.close >= self.open
    }
}

macro_rules! price_chart {
    ($(#[$doc:meta])* $name:ident) => {
        $(#[$doc])*
        #[derive(Clone, Debug, Default, PartialEq)]
        #[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
        #[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
        pub struct $name {
            #[cfg_attr(feature = "serde", serde(default, skip_serializing_if = "Option::is_none"))]
            pub title: Option<String>,
            /// Minutes the displayed clock is ahead of UTC (dates and ticks only).
            #[cfg_attr(feature = "serde", serde(default, skip_serializing_if = "Option::is_none"))]
            pub utc_offset: Option<i32>,
            pub data: Vec<Candle>,
        }

        impl $name {
            pub fn new(data: Vec<Candle>) -> Self {
                $name { data, ..Self::default() }
            }

            pub fn title(mut self, title: impl Into<String>) -> Self {
                self.title = Some(title.into());
                self
            }

            pub fn utc_offset(mut self, minutes: i32) -> Self {
                self.utc_offset = Some(minutes);
                self
            }
        }
    };
}

price_chart!(
    /// Candlesticks: a body from open to close and wicks to the high and low.
    CandlestickChart
);
price_chart!(
    /// Open-high-low-close bars: the range as a line, open and close as
    /// ticks to the left and right.
    OhlcChart
);

impl Viewable for CandlestickChart {
    fn laid_out(
        &self,
        width: f64,
        height: f64,
        measure: &dyn TextMeasure,
        view: Option<Ranges>,
    ) -> Result<(Scene, Plot), Error> {
        build(
            true,
            self.title.as_deref(),
            self.utc_offset.unwrap_or(0),
            &self.data,
            (width, height),
            measure,
            view.map(|v| v.0),
        )
    }
}
viewable!(CandlestickChart);

impl Viewable for OhlcChart {
    fn laid_out(
        &self,
        width: f64,
        height: f64,
        measure: &dyn TextMeasure,
        view: Option<Ranges>,
    ) -> Result<(Scene, Plot), Error> {
        build(
            false,
            self.title.as_deref(),
            self.utc_offset.unwrap_or(0),
            &self.data,
            (width, height),
            measure,
            view.map(|v| v.0),
        )
    }
}
viewable!(OhlcChart);

fn check(data: &[Candle]) -> Result<(), Error> {
    for (i, c) in data.iter().enumerate() {
        for (what, v) in [
            ("open", c.open),
            ("high", c.high),
            ("low", c.low),
            ("close", c.close),
        ] {
            finite(&format!("candle {i} {what}"), v)?;
        }
        if c.low > c.open.min(c.close) || c.high < c.open.max(c.close) {
            return Err(Error::InvalidData(format!(
                "candle {i}: low and high must bracket open and close"
            )));
        }
        if i > 0 && c.time <= data[i - 1].time {
            return Err(Error::InvalidData(format!(
                "candle {i}: times must increase"
            )));
        }
    }
    Ok(())
}

/// Lays the bars out over all of `data`, or over the times in `view`
/// (epoch ms) showing the periods whose whole bar fits.
fn build(
    candles: bool,
    title: Option<&str>,
    offset: i32,
    data: &[Candle],
    (width, height): (f64, f64),
    measure: &dyn TextMeasure,
    view: Option<(f64, f64)>,
) -> Result<(Scene, Plot), Error> {
    check_size(width, height)?;
    check(data)?;
    // Each period owns the time up to halfway to its neighbours; the domain
    // pads half a period at both ends so the first and last bars fit.
    let step = data
        .windows(2)
        .map(|w| w[1].time - w[0].time)
        .min()
        .unwrap_or(DAY);
    let (t0, t1) = match (view, data.first(), data.last()) {
        (Some((a, b)), ..) => (a.round() as i64, b.round() as i64),
        (None, Some(a), Some(b)) => (a.time - step / 2, b.time + step / 2),
        _ => (0, DAY),
    };
    let reach = step as f64 * FILL / 2.0;
    let shown: Vec<(usize, &Candle)> = data
        .iter()
        .enumerate()
        .filter(|(_, c)| {
            view.is_none()
                || (c.time as f64 - reach >= t0 as f64 && c.time as f64 + reach <= t1 as f64)
        })
        .collect();
    let (lo, hi) = shown
        .iter()
        .map(|&(_, c)| c)
        .fold((f64::INFINITY, f64::NEG_INFINITY), |(lo, hi), c| {
            (lo.min(c.low), hi.max(c.high))
        });
    let (lo, hi) = if lo.is_finite() { (lo, hi) } else { (0.0, 1.0) };

    let mut marks = Vec::new();
    let mut top = frame::title(&mut marks, title);
    let key = if candles {
        [
            (KeySample::Hollow(RISE), "Rising"),
            (KeySample::Solid(FALL), "Falling"),
        ]
    } else {
        [
            (KeySample::Line(RISE, 2.0), "Rising"),
            (KeySample::Line(FALL, 2.0), "Falling"),
        ]
    };
    if !data.is_empty() {
        top = frame::static_key(&mut marks, measure, top, &key);
    }
    let f = time_frame(measure, width, height, top, (t0, t1), (lo, hi), offset);
    f.grid(&mut marks);
    let slot = f.x.map(t0 + step) - f.x.map(t0);
    let w = (slot * FILL).clamp(1.0, BAR_MAX);
    let y = |v: f64| f.y.scale.map(v);
    for &(i, c) in &shown {
        let cx = f.x.map(c.time);
        let color = if c.rising() { RISE } else { FALL };
        let hit = Hit::new(i.to_string(), time::format_datetime(c.time, offset), "")
            .rows(
                [
                    ("open", c.open),
                    ("high", c.high),
                    ("low", c.low),
                    ("close", c.close),
                ]
                .into_iter()
                .map(|(n, v)| TipRow {
                    color: None,
                    name: n.into(),
                    value: format::value(v),
                })
                .collect(),
            )
            .area(Rect {
                x: cx - slot / 2.0,
                y: f.top,
                w: slot,
                h: f.bottom - f.top,
            });
        let line = |x1: f64, y1: f64, x2: f64, y2: f64| {
            Mark::new(Shape::Line { x1, y1, x2, y2 }).stroke(color, OUTLINE)
        };
        if candles {
            marks.push(line(cx, y(c.high), cx, y(c.low)));
            let body = Rect {
                x: cx - w / 2.0,
                y: y(c.open.max(c.close)),
                w,
                h: y(c.open.min(c.close)) - y(c.open.max(c.close)),
            };
            // The surface fill hides the wick inside a hollow body.
            let m = if c.rising() {
                Mark::new(Shape::Path {
                    ops: rect_ops(body),
                })
                .fill(Color::Surface)
                .stroke(color, OUTLINE)
            } else {
                Mark::new(Shape::Rect {
                    rect: body,
                    radius: Default::default(),
                })
                .fill(color)
            };
            marks.push(m.hit(hit));
        } else {
            marks.push(line(cx - w / 2.0, y(c.open), cx, y(c.open)));
            marks.push(line(cx, y(c.close), cx + w / 2.0, y(c.close)));
            marks.push(line(cx, y(c.high), cx, y(c.low)).hit(hit));
        }
    }
    f.baseline(&mut marks);
    f.labels(&mut marks);
    let what = if candles {
        "Candlestick chart"
    } else {
        "OHLC chart"
    };
    let plot = Plot {
        rect: Rect {
            x: f.left,
            y: f.top,
            w: f.right - f.left,
            h: f.bottom - f.top,
        },
        x: (t0 as f64, t1 as f64),
        y: None,
    };
    let scene = Scene {
        width,
        height,
        label: describe(
            title,
            &format!("{what} of {}.", count(shown.len(), "period", "periods")),
        ),
        marks,
    };
    Ok((scene, plot))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;
    use crate::base::time::to_ms;
    use crate::scene::PathOp;

    fn week() -> Vec<Candle> {
        let d = |n: u32| to_ms(2026, 3, n, 0, 0, 0);
        vec![
            Candle::new(d(2), 100.0, 104.0, 98.0, 103.0),
            Candle::new(d(3), 103.0, 105.0, 99.0, 100.0),
            Candle::new(d(4), 100.0, 101.0, 95.0, 96.0),
            Candle::new(d(5), 96.0, 102.0, 96.0, 101.0),
        ]
    }

    #[test]
    fn rising_candles_are_hollow_and_falling_ones_solid() {
        let s = CandlestickChart::new(week())
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let bodies: Vec<&Mark> = s.marks.iter().filter(|m| m.hit.is_some()).collect();
        assert_eq!(bodies.len(), 4);
        assert!(matches!(bodies[0].shape, Shape::Path { .. }));
        assert_eq!(bodies[0].fill, Some(Color::Surface));
        assert!(matches!(bodies[1].shape, Shape::Rect { .. }));
        assert_eq!(bodies[1].fill, Some(FALL));
        // The body spans open to close on the price scale.
        let Shape::Path { ops } = &bodies[0].shape else {
            unreachable!()
        };
        let ys: Vec<f64> = ops
            .iter()
            .filter_map(|o| match *o {
                PathOp::Move(_, y) | PathOp::Line(_, y) => Some(y),
                _ => None,
            })
            .collect();
        let span = ys.iter().cloned().fold(f64::MIN, f64::max)
            - ys.iter().cloned().fold(f64::MAX, f64::min);
        let Shape::Rect { rect, .. } = bodies[1].shape else {
            unreachable!()
        };
        // Both bodies are 3 price units tall.
        assert!((span - rect.h).abs() < 1e-9);
    }

    #[test]
    fn tooltips_list_open_high_low_close_with_the_date() {
        let s = OhlcChart::new(week())
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let h = s.marks.iter().find_map(|m| m.hit.as_ref()).unwrap();
        assert_eq!(h.label, "Mar 2, 2026");
        let r: Vec<(&str, &str)> = h
            .rows
            .iter()
            .map(|r| (r.name.as_str(), r.value.as_str()))
            .collect();
        assert_eq!(
            r,
            [
                ("open", "100"),
                ("high", "104"),
                ("low", "98"),
                ("close", "103")
            ]
        );
    }

    #[test]
    fn inconsistent_prices_and_unordered_times_are_rejected() {
        let mut bad = week();
        bad[1].low = 101.0;
        assert!(
            CandlestickChart::new(bad)
                .scene(640.0, 400.0, &InterMetrics)
                .is_err()
        );
        let mut back = week();
        back.swap(0, 1);
        assert!(
            OhlcChart::new(back)
                .scene(640.0, 400.0, &InterMetrics)
                .is_err()
        );
    }

    #[test]
    fn a_view_shows_the_periods_that_fit_and_refits_prices() {
        let d = |n: u32| to_ms(2026, 3, n, 0, 0, 0) as f64;
        let c = CandlestickChart::new(week());
        let full = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        // Mar 4 and 5 whole; the edge cuts Mar 3's body, so it is left out.
        let x = (d(3) + 0.2 * DAY as f64, d(5) + 0.5 * DAY as f64);
        let (s, p) = c
            .view(x, None)
            .unwrap()
            .plot_scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let p = p.unwrap();
        assert_eq!(p.x, (x.0.round(), x.1.round()));
        let bodies: Vec<&Mark> = s.marks.iter().filter(|m| m.hit.is_some()).collect();
        assert_eq!(bodies.len(), 2);
        assert!(s.label.contains("2 periods"), "{}", s.label);
        // The y axis fits Mar 4 and 5 (95 to 102): 104 was a tick before.
        let tick = |s: &Scene, t: &str| {
            s.marks
                .iter()
                .any(|m| matches!(&m.shape, Shape::Text { text, .. } if text == t))
        };
        assert!(tick(&full, "104") && !tick(&s, "104"));
    }
}
