use super::bar::{BAR_MAX, BAR_RADIUS, PADDING};
use super::frame::{self, row_frame};
use super::{Chart, check_size, count, describe, finite};
use crate::base::format;
use crate::base::text::TextMeasure;
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Corners, Hit, Mark, Rect, Scene, Shape};

/// A range from low to high.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Span {
    pub label: String,
    pub low: f64,
    pub high: f64,
}

/// Horizontal bars spanning each category's range. A span has no baseline,
/// so both ends are data ends and both are rounded.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct SpanChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub data: Vec<Span>,
}

impl SpanChart {
    pub fn new<L: Into<String>>(data: impl IntoIterator<Item = (L, f64, f64)>) -> Self {
        SpanChart {
            title: None,
            data: data
                .into_iter()
                .map(|(label, low, high)| Span {
                    label: label.into(),
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

impl Chart for SpanChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        for d in &self.data {
            finite(&format!("{:?} low", d.label), d.low)?;
            finite(&format!("{:?} high", d.label), d.high)?;
            if d.low > d.high {
                return Err(Error::InvalidData(format!(
                    "{:?} has low above high",
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
        let f = row_frame(measure, width, height, top, &labels, (lo, hi), PADDING);
        f.grid(&mut marks);
        let bw = f.band.bandwidth();
        let thick = BAR_MAX.min(bw);
        for (i, d) in self.data.iter().enumerate() {
            let (x0, x1) = (f.x.scale.map(d.low), f.x.scale.map(d.high));
            let len = x1 - x0;
            marks.push(
                Mark::new(Shape::Rect {
                    rect: Rect {
                        x: x0,
                        y: f.band.position(i) + (bw - thick) / 2.0,
                        w: len,
                        h: thick,
                    },
                    radius: Corners::all(BAR_RADIUS.min(thick / 2.0).min(len / 2.0)),
                })
                .fill(Color::Series(0))
                .hit(
                    Hit::new(
                        i.to_string(),
                        d.label.as_str(),
                        format!("{} – {}", format::value(d.low), format::value(d.high)),
                    )
                    .area(f.row(i)),
                ),
            );
        }
        f.row_labels(&mut marks, &labels);
        f.x_labels(&mut marks);

        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Span chart with {}.",
                    count(self.data.len(), "range", "ranges")
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
    fn span_runs_from_low_to_high_with_both_ends_rounded() {
        let c = SpanChart::new([("Mon", 12.0, 21.0), ("Tue", 9.0, 18.0)]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let (r, rad, hit) = s
            .marks
            .iter()
            .find_map(|m| match (&m.shape, &m.hit) {
                (Shape::Rect { rect, radius }, Some(h)) => Some((*rect, *radius, h)),
                _ => None,
            })
            .unwrap();
        assert!(rad.tl > 0.0 && rad.br > 0.0);
        assert!(r.h <= BAR_MAX);
        assert_eq!(hit.value, "12 – 21");
    }
}
