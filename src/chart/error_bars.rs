use super::bar::PADDING;
use super::frame::{self, category_frame};
use super::{Chart, DOT_RADIUS, LINE_WIDTH, SURFACE_GAP, check_size, count, describe, finite};
use crate::base::format;
use crate::base::text::TextMeasure;
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Hit, Mark, Scene, Shape};

/// Half the width of a whisker's end cap.
const CAP: f64 = 4.0;

/// A value and the uncertainty around it.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct ErrorBar {
    pub label: String,
    pub value: f64,
    pub low: f64,
    pub high: f64,
}

/// One estimate per category with its error range as a whisker.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct ErrorBarChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub data: Vec<ErrorBar>,
}

impl ErrorBarChart {
    pub fn new<L: Into<String>>(data: impl IntoIterator<Item = (L, f64, f64, f64)>) -> Self {
        ErrorBarChart {
            title: None,
            data: data
                .into_iter()
                .map(|(label, value, low, high)| ErrorBar {
                    label: label.into(),
                    value,
                    low,
                    high,
                })
                .collect(),
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }
}

impl Chart for ErrorBarChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        for d in &self.data {
            for (what, v) in [("value", d.value), ("low", d.low), ("high", d.high)] {
                finite(&format!("{:?} {what}", d.label), v)?;
            }
            if !(d.low <= d.value && d.value <= d.high) {
                return Err(Error::InvalidData(format!(
                    "{:?} needs low <= value <= high",
                    d.label
                )));
            }
        }
        let (lo, hi) = self
            .data
            .iter()
            .fold((f64::INFINITY, f64::NEG_INFINITY), |(lo, hi), d| {
                (lo.min(d.low), hi.max(d.high))
            });
        let (lo, hi) = if lo.is_finite() { (lo, hi) } else { (0.0, 0.0) };
        let labels: Vec<String> = self.data.iter().map(|d| d.label.clone()).collect();

        let mut marks = Vec::new();
        let top = frame::title(&mut marks, self.title.as_deref());
        let f = category_frame(measure, width, height, top, &labels, (lo, hi), PADDING, 0.0);
        f.grid(&mut marks);
        f.y_labels(&mut marks);
        f.baseline(&mut marks);
        for (i, d) in self.data.iter().enumerate() {
            let cx = f.center(i);
            let (y_low, y_high) = (f.y.scale.map(d.low), f.y.scale.map(d.high));
            let line = |x1, y1, x2, y2| {
                Mark::new(Shape::Line { x1, y1, x2, y2 }).stroke(Color::Series(0), LINE_WIDTH)
            };
            marks.push(line(cx, y_high, cx, y_low));
            marks.push(line(cx - CAP, y_high, cx + CAP, y_high));
            marks.push(line(cx - CAP, y_low, cx + CAP, y_low));
            marks.push(
                Mark::new(Shape::Circle {
                    cx,
                    cy: f.y.scale.map(d.value),
                    r: DOT_RADIUS,
                })
                .fill(Color::Series(0))
                .stroke(Color::Surface, SURFACE_GAP)
                .hit(
                    Hit::new(
                        i.to_string(),
                        format!(
                            "{} · {} – {}",
                            d.label,
                            format::value(d.low),
                            format::value(d.high)
                        ),
                        format::value(d.value),
                    )
                    .area(f.column(i)),
                ),
            );
        }
        f.x_labels(&mut marks, &labels);

        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Error bar chart with {}.",
                    count(self.data.len(), "estimate", "estimates")
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
    fn whisker_spans_low_to_high_through_the_estimate() {
        let c = ErrorBarChart::new([("A", 10.0, 8.0, 13.0), ("B", 20.0, 19.0, 22.0)]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let (whisker, dot) = (&s.marks, s.marks.iter().find(|m| m.hit.is_some()).unwrap());
        let Shape::Circle { cy, .. } = dot.shape else {
            panic!()
        };
        let span = whisker
            .iter()
            .find_map(|m| match m.shape {
                Shape::Line { x1, y1, x2, y2 }
                    if x1 == x2 && m.stroke.unwrap().width == LINE_WIDTH =>
                {
                    Some((y1, y2))
                }
                _ => None,
            })
            .unwrap();
        assert!(span.0 < cy && cy < span.1);
        assert_eq!(dot.hit.as_ref().unwrap().label, "A · 8 – 13");
    }

    #[test]
    fn inverted_ranges_are_rejected() {
        let c = ErrorBarChart::new([("A", 10.0, 12.0, 13.0)]);
        assert!(c.scene(640.0, 400.0, &InterMetrics).is_err());
    }
}
