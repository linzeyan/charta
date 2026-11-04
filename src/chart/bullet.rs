use super::bar::{BAR_MAX, BAR_RADIUS};
use super::frame::{self, ValueAxis, X_TICK_SPACING, text};
use super::{Chart, GAP, LINE_WIDTH, PAD, TICK_SIZE, check_size, count, describe, finite};
use crate::base::format;
use crate::base::scale::BandScale;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::{BAND_CLASSES, Color};
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Corners, Hit, Mark, Rect, Scene, Shape};

const PADDING: (f64, f64) = (0.3, 0.1);
/// The measure bar is a third of the range band's thickness.
const MEASURE_RATIO: f64 = 1.0 / 3.0;
/// The target marker spans this share of the range band.
const TARGET_RATIO: f64 = 0.75;
/// Space between a gauge and its tick labels.
const TICK_GAP: f64 = 4.0;

/// One measure against its target and qualitative ranges.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Bullet {
    pub label: String,
    pub value: f64,
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub target: Option<f64>,
    /// Upper bounds of up to three qualitative ranges (e.g. poor, fair,
    /// good), each starting at zero.
    #[cfg_attr(feature = "serde", serde(default))]
    pub ranges: Vec<f64>,
}

/// Bullet graphs: compact horizontal gauges, one per measure. Measures
/// rarely share units, so each gauge has its own scale and tick labels.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct BulletChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub data: Vec<Bullet>,
}

impl BulletChart {
    pub fn new(data: Vec<Bullet>) -> Self {
        BulletChart { title: None, data }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }
}

impl Bullet {
    pub fn new(label: impl Into<String>, value: f64) -> Self {
        Bullet {
            label: label.into(),
            value,
            target: None,
            ranges: Vec::new(),
        }
    }

    pub fn target(mut self, t: f64) -> Self {
        self.target = Some(t);
        self
    }

    pub fn ranges(mut self, r: impl IntoIterator<Item = f64>) -> Self {
        self.ranges = r.into_iter().collect();
        self
    }
}

impl Chart for BulletChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let mut domains = Vec::with_capacity(self.data.len());
        for d in &self.data {
            finite(&format!("{:?} value", d.label), d.value)?;
            let (mut lo, mut hi) = (d.value.min(0.0), d.value.max(0.0));
            if let Some(t) = d.target {
                finite(&format!("{:?} target", d.label), t)?;
                (lo, hi) = (lo.min(t), hi.max(t));
            }
            if d.ranges.len() > BAND_CLASSES as usize {
                return Err(Error::InvalidData(format!(
                    "{:?} has {} ranges, at most {BAND_CLASSES}",
                    d.label,
                    d.ranges.len()
                )));
            }
            for &r in &d.ranges {
                if !(r.is_finite() && r >= 0.0) {
                    return Err(Error::InvalidData(format!(
                        "{:?} ranges must be non-negative numbers",
                        d.label
                    )));
                }
                hi = hi.max(r);
            }
            domains.push((lo, hi));
        }

        let mut marks = Vec::new();
        let top = frame::title(&mut marks, self.title.as_deref());
        let label_w = self
            .data
            .iter()
            .map(|d| measure.width(&d.label, TICK_SIZE, Weight::Regular))
            .fold(0.0, f64::max);
        let left = PAD + label_w + GAP;
        let right = (width - PAD).max(left + 1.0);
        let bottom = (height - PAD).max(top + 1.0);
        let band = BandScale::new(self.data.len(), (top, bottom), PADDING.0, PADDING.1);
        let bw = band.bandwidth();
        let thick = BAR_MAX.min(bw - TICK_GAP - TICK_SIZE).max(1.0);
        let ticks = (((right - left) / X_TICK_SPACING).round() as usize).max(2);
        let mut axes: Vec<ValueAxis> = domains
            .iter()
            .map(|&(lo, hi)| ValueAxis::new(lo, hi, (left, right), ticks))
            .collect();
        // Every gauge ends at the same x: pull them all in by the widest
        // last tick label, which is centered on the end.
        let last = axes
            .iter()
            .filter_map(|a| {
                a.ticks
                    .last()
                    .map(|&t| measure.width(&a.fmt.format(t), TICK_SIZE, Weight::Regular))
            })
            .fold(0.0, f64::max);
        let right = (right - last / 2.0).max(left + 1.0);
        for a in &mut axes {
            a.scale.range.1 = right;
        }

        for (i, (d, axis)) in self.data.iter().zip(&axes).enumerate() {
            let y = band.position(i);
            let cy = y + thick / 2.0;
            let x0 = axis.scale.map(0.0);
            let mut ranges = d.ranges.clone();
            ranges.sort_by(f64::total_cmp);
            // Widest range first and lightest; narrower ones darker on top.
            let offset = BAND_CLASSES as usize - ranges.len();
            for (k, &r) in ranges.iter().enumerate().rev() {
                marks.push(
                    Mark::new(Shape::Rect {
                        rect: Rect {
                            x: x0,
                            y,
                            w: axis.scale.map(r) - x0,
                            h: thick,
                        },
                        radius: Corners::default(),
                    })
                    .fill(Color::Band((offset + k) as u8)),
                );
            }
            let m = thick * MEASURE_RATIO;
            let x1 = axis.scale.map(d.value);
            let len = (x1 - x0).abs();
            let r = BAR_RADIUS.min(m / 2.0).min(len);
            let radius = if d.value >= 0.0 {
                Corners {
                    tl: 0.0,
                    tr: r,
                    br: r,
                    bl: 0.0,
                }
            } else {
                Corners {
                    tl: r,
                    tr: 0.0,
                    br: 0.0,
                    bl: r,
                }
            };
            let label = match d.target {
                Some(t) => format!("{} · target {}", d.label, format::value(t)),
                None => d.label.clone(),
            };
            marks.push(
                Mark::new(Shape::Rect {
                    rect: Rect {
                        x: x0.min(x1),
                        y: cy - m / 2.0,
                        w: len,
                        h: m,
                    },
                    radius,
                })
                .fill(Color::Series(0))
                .hit(
                    Hit::new(i.to_string(), label, format::value(d.value)).area(Rect {
                        x: left,
                        y: y - (band.step() - bw) / 2.0,
                        w: right - left,
                        h: band.step(),
                    }),
                ),
            );
            if let Some(t) = d.target {
                let x = axis.scale.map(t);
                let h = thick * TARGET_RATIO;
                marks.push(
                    Mark::new(Shape::Line {
                        x1: x,
                        y1: cy - h / 2.0,
                        x2: x,
                        y2: cy + h / 2.0,
                    })
                    .stroke(Color::TextPrimary, LINE_WIDTH),
                );
            }
            marks.push(text(
                left - GAP,
                cy,
                &d.label,
                TICK_SIZE,
                Weight::Regular,
                Anchor::End,
                Baseline::Middle,
                Color::TextMuted,
            ));
            for &t in &axis.ticks {
                marks.push(text(
                    axis.scale.map(t),
                    y + thick + TICK_GAP,
                    &axis.fmt.format(t),
                    TICK_SIZE,
                    Weight::Regular,
                    Anchor::Middle,
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
                    "Bullet graph with {}.",
                    count(self.data.len(), "measure", "measures")
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

    #[test]
    fn ranges_nest_from_zero_with_the_widest_lightest() {
        let c = BulletChart::new(vec![
            Bullet::new("Revenue", 270.0)
                .target(250.0)
                .ranges([150.0, 225.0, 300.0]),
        ]);
        let s = c.scene(640.0, 300.0, &InterMetrics).unwrap();
        let bands: Vec<(Rect, Color)> = s
            .marks
            .iter()
            .filter_map(|m| match (&m.shape, m.fill) {
                (Shape::Rect { rect, .. }, Some(c @ Color::Band(_))) => Some((*rect, c)),
                _ => None,
            })
            .collect();
        assert_eq!(bands.len(), 3);
        assert_eq!(bands[0].1, Color::Band(2));
        assert!(bands[0].0.w > bands[1].0.w && bands[1].0.w > bands[2].0.w);
        assert!(bands.iter().all(|(r, _)| (r.x - bands[0].0.x).abs() < 1e-9));
        let hit = s.marks.iter().find_map(|m| m.hit.as_ref()).unwrap();
        assert_eq!(hit.label, "Revenue · target 250");
    }

    #[test]
    fn each_gauge_has_its_own_scale() {
        // A 0–5 rating next to a 0–2,500 count must still fill its gauge.
        let c = BulletChart::new(vec![
            Bullet::new("Customers", 1500.0).ranges([2500.0]),
            Bullet::new("Satisfaction", 4.4).ranges([5.0]),
        ]);
        let s = c.scene(640.0, 300.0, &InterMetrics).unwrap();
        let widths: Vec<f64> = s
            .marks
            .iter()
            .filter_map(|m| match (&m.shape, m.fill) {
                (Shape::Rect { rect, .. }, Some(Color::Band(_))) => Some(rect.w),
                _ => None,
            })
            .collect();
        assert!((widths[0] - widths[1]).abs() < 1e-9, "{widths:?}");
    }

    #[test]
    fn four_ranges_are_rejected() {
        let c = BulletChart::new(vec![Bullet::new("x", 1.0).ranges([1.0, 2.0, 3.0, 4.0])]);
        assert!(c.scene(640.0, 300.0, &InterMetrics).is_err());
    }
}
