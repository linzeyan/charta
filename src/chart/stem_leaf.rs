use super::frame::{self, text, vline};
use super::{Chart, GAP, PAD, TICK_SIZE, TITLE_SIZE, check_size, count, describe, finite};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::base::ticks::pow10;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Corners, Hit, HitKind, Mark, Rect, Scene, Shape};

/// Auto leaf units stop at the smallest power of ten giving this many stems
/// or fewer: past it the display is a column of near-empty rows.
const MAX_STEMS: i64 = 15;
/// Rows grow up to this height so a short display still fills its space
/// legibly; digits scale with them up to the title size.
const ROW_MAX: f64 = 28.0;
const LEAF_GAP: f64 = 6.0;
/// Smallest and largest auto leaf unit, as powers of ten.
const UNIT_RANGE: (i32, i32) = (-6, 12);

/// Values split into a stem (leading digits) and a leaf (the next digit),
/// one row per stem: a histogram that keeps every value readable.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
pub struct StemLeafChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    /// Non-negative values.
    pub values: Vec<f64>,
    /// What one leaf digit is worth, a power of ten (0.1, 1, 10, …).
    /// Defaults to the finest unit that needs at most 15 stems.
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub leaf_unit: Option<f64>,
}

impl StemLeafChart {
    pub fn new(values: impl IntoIterator<Item = f64>) -> Self {
        StemLeafChart {
            values: values.into_iter().collect(),
            ..Self::default()
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    pub fn leaf_unit(mut self, unit: f64) -> Self {
        self.leaf_unit = Some(unit);
        self
    }
}

/// A leaf unit as a power of ten: `n` units is `n * 10^p`, computed as a
/// division for negative `p` so 29 × 0.01 prints as 0.29.
#[derive(Clone, Copy)]
struct Unit(i32);

impl Unit {
    fn of(self, n: i64) -> f64 {
        if self.0 < 0 {
            n as f64 / pow10(-self.0)
        } else {
            n as f64 * pow10(self.0)
        }
    }

    fn count(self, v: f64) -> i64 {
        if self.0 < 0 {
            (v * pow10(-self.0)).round() as i64
        } else {
            (v / pow10(self.0)).round() as i64
        }
    }
}

fn unit_power(u: f64) -> Option<i32> {
    (UNIT_RANGE.0..=UNIT_RANGE.1).find(|&p| pow10(p) == u)
}

impl Chart for StemLeafChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        for (i, &v) in self.values.iter().enumerate() {
            finite(&format!("value {i}"), v)?;
            if v < 0.0 {
                return Err(Error::InvalidData(format!(
                    "value {i} is negative; a stem-and-leaf plot takes values ≥ 0"
                )));
            }
        }
        let mut sorted = self.values.clone();
        sorted.sort_by(f64::total_cmp);
        let unit = match self.leaf_unit {
            Some(u) => Unit(unit_power(u).ok_or_else(|| {
                Error::InvalidData(format!("leaf unit {u} is not a power of ten"))
            })?),
            None => {
                let (lo, hi) = (
                    sorted.first().copied().unwrap_or(0.0),
                    sorted.last().copied().unwrap_or(0.0),
                );
                Unit(
                    (UNIT_RANGE.0..=UNIT_RANGE.1)
                        .find(|&p| {
                            let u = Unit(p);
                            u.count(hi).div_euclid(10) - u.count(lo).div_euclid(10) < MAX_STEMS
                        })
                        .unwrap_or(UNIT_RANGE.1),
                )
            }
        };
        let leaves: Vec<i64> = sorted.iter().map(|&v| unit.count(v)).collect();
        let (first, last) = match (leaves.first(), leaves.last()) {
            (Some(a), Some(b)) => (a.div_euclid(10), b.div_euclid(10)),
            _ => (0, -1),
        };
        let stems: Vec<i64> = (first..=last).collect();
        let rows: Vec<Vec<i64>> = stems
            .iter()
            .map(|&s| {
                leaves
                    .iter()
                    .filter(|l| l.div_euclid(10) == s)
                    .map(|l| l.rem_euclid(10))
                    .collect()
            })
            .collect();

        let mut marks = Vec::new();
        let top = frame::title(&mut marks, self.title.as_deref());
        let key_h = TICK_SIZE + GAP;
        let avail = (height - PAD - key_h - top).max(1.0);
        let row_h = ROW_MAX.min(avail / stems.len().max(1) as f64);
        let size = TITLE_SIZE.min(row_h * 0.6);
        let stem_w = stems
            .iter()
            .map(|s| measure.width(&s.to_string(), size, Weight::Regular))
            .fold(0.0, f64::max);
        let digit_w = (0..10)
            .map(|d| measure.width(&d.to_string(), size, Weight::Regular))
            .fold(0.0, f64::max);
        let x_stem = PAD + stem_w;
        let x_sep = x_stem + GAP;
        let x_leaf = x_sep + GAP;
        let most = rows.iter().map(Vec::len).max().unwrap_or(0).max(1);
        let step = (digit_w + LEAF_GAP).min((width - PAD - x_leaf) / most as f64);

        for (k, (&s, row)) in stems.iter().zip(&rows).enumerate() {
            let y = top + k as f64 * row_h;
            let mid = y + row_h / 2.0;
            // A row band that shows only under the pointer: the "crosshair"
            // of this chart, with the row's range and count.
            marks.push(
                Mark::new(Shape::Rect {
                    rect: Rect {
                        x: PAD / 2.0,
                        y,
                        w: width - PAD,
                        h: row_h,
                    },
                    radius: Corners::all(2.0),
                })
                .fill(Color::Grid)
                .hit(
                    Hit::new(
                        format!("r{k}"),
                        format!(
                            "{}–{}",
                            format::value(unit.of(s * 10)),
                            format::value(unit.of(s * 10 + 9))
                        ),
                        count(row.len(), "value", "values"),
                    )
                    .kind(HitKind::Crosshair),
                ),
            );
            marks.push(text(
                x_stem,
                mid,
                &s.to_string(),
                size,
                Weight::Regular,
                Anchor::End,
                Baseline::Middle,
                Color::TextSecondary,
            ));
            // Each leaf sits in a fixed slot, so columns align whatever the
            // font's digit widths.
            for (i, l) in row.iter().enumerate() {
                marks.push(text(
                    x_leaf + (i as f64 + 0.5) * step,
                    mid,
                    &l.to_string(),
                    size,
                    Weight::Regular,
                    Anchor::Middle,
                    Baseline::Middle,
                    Color::TextPrimary,
                ));
            }
        }
        let bottom = top + stems.len() as f64 * row_h;
        if !stems.is_empty() {
            marks.push(vline(super::snap(x_sep), top, bottom, Color::Axis));
        }
        // The key is what makes the digits readable as values.
        if let Some(&l) = leaves.first() {
            let (s, d) = (l.div_euclid(10), l.rem_euclid(10));
            marks.push(text(
                PAD,
                bottom + GAP,
                &format!("Key: {s} | {d} = {}", format::value(unit.of(l))),
                TICK_SIZE,
                Weight::Regular,
                Anchor::Start,
                Baseline::Top,
                Color::TextMuted,
            ));
        }

        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Stem-and-leaf plot of {}.",
                    count(sorted.len(), "value", "values")
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

    fn texts(s: &Scene) -> Vec<String> {
        s.marks
            .iter()
            .filter_map(|m| match &m.shape {
                Shape::Text { text, .. } => Some(text.clone()),
                _ => None,
            })
            .collect()
    }

    fn rows(s: &Scene) -> Vec<&crate::scene::Hit> {
        s.marks.iter().filter_map(|m| m.hit.as_ref()).collect()
    }

    #[test]
    fn every_value_becomes_one_leaf_under_its_stem() {
        let c = StemLeafChart::new([12.0, 15.0, 21.0, 23.0, 23.0, 38.0]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let r = rows(&s);
        assert_eq!(r.len(), 3);
        assert_eq!(r[1].label, "20–29");
        assert_eq!(r[1].value, "3 values");
        // Stems, then leaves in order, then the key.
        let t = texts(&s);
        assert_eq!(
            &t[..],
            [
                "1",
                "2",
                "5",
                "2",
                "1",
                "3",
                "3",
                "3",
                "8",
                "Key: 1 | 2 = 12"
            ]
        );
    }

    #[test]
    fn empty_stems_between_values_still_get_a_row() {
        // A gap in the data is information; skipping the row would hide it.
        let s = StemLeafChart::new([11.0, 52.0])
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        assert_eq!(rows(&s).len(), 5);
        assert_eq!(rows(&s)[2].value, "0 values");
    }

    #[test]
    fn auto_unit_picks_the_finest_readable_split() {
        // 0.12–0.89: hundredths as leaves, tenths as stems.
        let s = StemLeafChart::new([0.12, 0.29, 0.47, 0.89])
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        assert_eq!(rows(&s)[0].label, "0.1–0.19");
        assert!(texts(&s).contains(&"Key: 1 | 2 = 0.12".to_string()));
        // A coarser explicit unit rounds to it.
        let s = StemLeafChart::new([120.0, 340.0, 355.0])
            .leaf_unit(10.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        assert_eq!(rows(&s)[0].label, "100–190");
    }

    #[test]
    fn negative_values_and_odd_units_are_rejected() {
        let m = &InterMetrics;
        assert!(StemLeafChart::new([-1.0]).scene(640.0, 400.0, m).is_err());
        assert!(
            StemLeafChart::new([1.0])
                .leaf_unit(0.5)
                .scene(640.0, 400.0, m)
                .is_err()
        );
    }
}
