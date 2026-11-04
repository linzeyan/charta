use super::frame::{self, category_frame};
use super::{Chart, check_size, describe};
use crate::base::format;
use crate::base::text::TextMeasure;
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Corners, Hit, Mark, Rect, Scene, Shape};

/// Bars never fill their slot: thickness is capped and the rest is air.
pub(crate) const BAR_MAX: f64 = 24.0;
/// Rounded data end; the baseline end stays square.
pub(crate) const BAR_RADIUS: f64 = 4.0;
pub(crate) const PADDING: (f64, f64) = (0.2, 0.1);

/// One category and its value.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Bar {
    pub label: String,
    pub value: f64,
}

/// Vertical bars comparing one value across categories.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct BarChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub data: Vec<Bar>,
}

impl BarChart {
    pub fn new<L: Into<String>>(data: impl IntoIterator<Item = (L, f64)>) -> Self {
        BarChart {
            title: None,
            data: data
                .into_iter()
                .map(|(label, value)| Bar {
                    label: label.into(),
                    value,
                })
                .collect(),
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }
}

impl Chart for BarChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        if let Some((i, b)) = self
            .data
            .iter()
            .enumerate()
            .find(|(_, b)| !b.value.is_finite())
        {
            return Err(Error::InvalidData(format!(
                "bar {i} ({:?}) has a non-finite value",
                b.label
            )));
        }

        let mut marks = Vec::new();
        let top = frame::title(&mut marks, self.title.as_deref());
        let labels: Vec<String> = self.data.iter().map(|b| b.label.clone()).collect();
        let lo = self.data.iter().fold(0.0_f64, |m, b| m.min(b.value));
        let hi = self.data.iter().fold(0.0_f64, |m, b| m.max(b.value));
        let f = category_frame(measure, width, height, top, &labels, (lo, hi), PADDING, 0.0);

        f.grid(&mut marks);
        f.y_labels(&mut marks);
        let y0 = f.y.scale.map(0.0);
        let bw = f.band.bandwidth();
        let thick = BAR_MAX.min(bw);
        for (i, b) in self.data.iter().enumerate() {
            let y1 = f.y.scale.map(b.value);
            marks.push(
                Mark::new(Shape::Rect {
                    rect: Rect {
                        x: f.band.position(i) + (bw - thick) / 2.0,
                        y: y0.min(y1),
                        w: thick,
                        h: (y1 - y0).abs(),
                    },
                    radius: data_end(b.value, thick, (y1 - y0).abs()),
                })
                .fill(Color::Series(0))
                .hit(
                    Hit::new(i.to_string(), b.label.as_str(), format::value(b.value))
                        .area(f.column(i)),
                ),
            );
        }
        f.baseline(&mut marks);
        f.x_labels(&mut marks, &labels);

        let n = self.data.len();
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!("Bar chart with {n} bar{}.", if n == 1 { "" } else { "s" }),
            ),
            marks,
        })
    }
}

/// Rounds the data end of a vertical bar: the top for positive values, the
/// bottom for negative ones. The radius never exceeds half the thickness or
/// the bar's own length.
pub(crate) fn data_end(value: f64, thick: f64, len: f64) -> Corners {
    let r = BAR_RADIUS.min(thick / 2.0).min(len);
    if value >= 0.0 {
        Corners {
            tl: r,
            tr: r,
            br: 0.0,
            bl: 0.0,
        }
    } else {
        Corners {
            tl: 0.0,
            tr: 0.0,
            br: r,
            bl: r,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;

    fn bars(s: &Scene) -> Vec<(Rect, &Hit)> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Rect { rect, .. }, Some(h)) => Some((*rect, h)),
                _ => None,
            })
            .collect()
    }

    fn build(c: &BarChart) -> Scene {
        c.scene(640.0, 400.0, &InterMetrics).unwrap()
    }

    #[test]
    fn bar_height_is_proportional_to_value() {
        // The whole point of a bar chart: length encodes magnitude.
        let s = build(&BarChart::new([("a", 10.0), ("b", 20.0), ("c", 35.0)]));
        let b = bars(&s);
        let per_unit = b[0].0.h / 10.0;
        assert!((b[1].0.h - 20.0 * per_unit).abs() < 1e-9);
        assert!((b[2].0.h - 35.0 * per_unit).abs() < 1e-9);
    }

    #[test]
    fn bars_share_one_baseline_and_negatives_hang_below_it() {
        let s = build(&BarChart::new([("up", 5.0), ("down", -3.0)]));
        let b = bars(&s);
        let base = b[0].0.y + b[0].0.h;
        assert!(
            (b[1].0.y - base).abs() < 1e-9,
            "negative bar starts at the baseline"
        );
        // The rounded end is the data end: top for positive, bottom for negative.
        let radii: Vec<Corners> = s
            .marks
            .iter()
            .filter_map(|m| match m.shape {
                Shape::Rect { radius, .. } if m.hit.is_some() => Some(radius),
                _ => None,
            })
            .collect();
        assert!(radii[0].tl > 0.0 && radii[0].bl == 0.0);
        assert!(radii[1].bl > 0.0 && radii[1].tl == 0.0);
    }

    #[test]
    fn bars_are_capped_in_thickness_and_stay_inside_the_chart() {
        let s = build(&BarChart::new([("only", 1.0)]));
        let (r, _) = bars(&s)[0];
        assert!(r.w <= BAR_MAX);
        assert!(r.x >= 0.0 && r.x + r.w <= 640.0 && r.y >= 0.0 && r.y + r.h <= 400.0);
    }

    #[test]
    fn hovering_anywhere_in_a_band_hits_that_bar() {
        let s = build(&BarChart::new([("a", 100.0), ("b", 1.0)]));
        let (r, _) = bars(&s)[1];
        // Far above the short bar, still inside its band column.
        let i = s.hit_test(r.x + r.w / 2.0, 60.0, &[]).expect("hit");
        assert_eq!(s.marks[i].hit.as_ref().unwrap().label, "b");
    }

    #[test]
    fn crowded_labels_rotate_instead_of_overlapping() {
        let data: Vec<(String, f64)> = (0..30)
            .map(|i| (format!("Category {i}"), i as f64))
            .collect();
        let s = build(&BarChart::new(data));
        let rotated = s
            .marks
            .iter()
            .filter(|m| matches!(m.shape, Shape::Text { angle, .. } if angle == -45.0))
            .count();
        assert_eq!(rotated, 30);
    }

    #[test]
    fn tooltip_carries_label_and_formatted_value() {
        let s = build(&BarChart::new([("Revenue", 1234567.0)]));
        let (_, h) = bars(&s)[0];
        assert_eq!(
            (h.label.as_str(), h.value.as_str()),
            ("Revenue", "1,234,567")
        );
    }

    #[test]
    fn non_finite_values_are_rejected_with_the_culprit_named() {
        let err = BarChart::new([("ok", 1.0), ("bad", f64::NAN)])
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap_err();
        assert!(
            matches!(&err, Error::InvalidData(m) if m.contains("bad")),
            "{err}"
        );
        assert_eq!(
            BarChart::default().scene(0.0, 10.0, &InterMetrics),
            Err(Error::InvalidSize)
        );
    }

    #[test]
    fn empty_data_still_draws_axes() {
        let s = build(&BarChart::default());
        assert!(bars(&s).is_empty());
        assert!(
            s.marks
                .iter()
                .any(|m| matches!(m.shape, Shape::Line { .. }))
        );
    }
}
