use super::frame::{self, hline, text, vline};
use super::{Chart, GAP, PAD, SURFACE_GAP, TICK_SIZE, check_size, count, describe};
use crate::base::scale::LinearScale;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::base::time;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Hit, Mark, Rect, Scene, Shape};

const LABEL_SIZE: f64 = 12.0;
const DOT: f64 = 5.0;
/// A label block: date line, then the event's name.
const BLOCK: f64 = TICK_SIZE + 4.0 + LABEL_SIZE;
/// Distance from the axis to the nearest label block.
const STEM_MIN: f64 = 10.0;

/// Something that happened at an instant (UTC epoch ms).
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Event {
    pub time: i64,
    pub label: String,
}

impl Event {
    pub fn new(time: i64, label: impl Into<String>) -> Self {
        Event {
            time,
            label: label.into(),
        }
    }
}

/// Events in time order along one line, labels stacked above and below so
/// they never overlap.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
pub struct TimelineChart {
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
    pub events: Vec<Event>,
}

impl TimelineChart {
    pub fn new(events: Vec<Event>) -> Self {
        TimelineChart {
            events,
            ..Self::default()
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }
}

/// Picks a level for each label extent `(x0, x1)` in x order: the first of
/// above-1, below-1, above-2, below-2 … whose previous label ends at least
/// `GAP` before this one starts; when none is free, the level whose last
/// label ends earliest. Levels are ±1, ±2, … (positive is above).
pub(crate) fn levels(extents: &[(f64, f64)], per_side: usize) -> Vec<i32> {
    let order: Vec<i32> = (1..=per_side.max(1) as i32).flat_map(|n| [n, -n]).collect();
    let mut ends = vec![f64::NEG_INFINITY; order.len()];
    extents
        .iter()
        .map(|&(x0, x1)| {
            let k = order
                .iter()
                .enumerate()
                .position(|(k, _)| ends[k] + GAP <= x0)
                .unwrap_or_else(|| {
                    (0..ends.len())
                        .min_by(|&a, &b| ends[a].total_cmp(&ends[b]))
                        .expect("at least one level")
                });
            ends[k] = x1;
            order[k]
        })
        .collect()
}

impl Chart for TimelineChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let offset = self.utc_offset.unwrap_or(0);
        let mut order: Vec<usize> = (0..self.events.len()).collect();
        order.sort_by_key(|&i| self.events[i].time);

        let mut marks = Vec::new();
        let top = frame::title(&mut marks, self.title.as_deref());
        let bottom = height - PAD;
        let axis_y = (top + bottom) / 2.0;
        let (left, right) = (PAD, (width - PAD).max(PAD + 1.0));
        let (t0, t1) = match (order.first(), order.last()) {
            (Some(&a), Some(&b)) => (self.events[a].time, self.events[b].time),
            _ => (0, time::DAY),
        };
        // Pad the ends so the first and last dots are not on the edge.
        let pad = ((t1 - t0) / 20).max(time::HOUR);
        let x = LinearScale::new(((t0 - pad) as f64, (t1 + pad) as f64), (left, right));
        marks.push(hline(left, right, super::snap(axis_y), Color::Axis));

        let dates: Vec<String> = order
            .iter()
            .map(|&i| time::format_date(self.events[i].time, offset))
            .collect();
        // Each label starts at its dot and runs right, unless that would leave
        // the chart; then it ends at its dot.
        let extents: Vec<(f64, f64, Anchor)> = order
            .iter()
            .zip(&dates)
            .map(|(&i, d)| {
                let e = &self.events[i];
                let w = measure
                    .width(&e.label, LABEL_SIZE, Weight::Semibold)
                    .max(measure.width(d, TICK_SIZE, Weight::Regular));
                let cx = x.map(e.time as f64);
                if cx + w <= right {
                    (cx, cx + w, Anchor::Start)
                } else {
                    (cx - w, cx, Anchor::End)
                }
            })
            .collect();
        let step = BLOCK + GAP;
        let per_side = (((axis_y - top - STEM_MIN) / step).floor() as usize).max(1);
        let lv = levels(
            &extents.iter().map(|&(a, b, _)| (a, b)).collect::<Vec<_>>(),
            per_side,
        );

        for (k, &i) in order.iter().enumerate() {
            let e = &self.events[i];
            let cx = x.map(e.time as f64);
            let (x0, x1, anchor) = extents[k];
            let n = lv[k].unsigned_abs() as f64;
            let block_top = if lv[k] > 0 {
                axis_y - STEM_MIN - BLOCK - (n - 1.0) * step
            } else {
                axis_y + STEM_MIN + (n - 1.0) * step
            };
            let (stem_end, near) = if lv[k] > 0 {
                (block_top + BLOCK, block_top + BLOCK + 2.0)
            } else {
                (block_top, block_top - 2.0)
            };
            marks.push(vline(
                super::snap(cx),
                axis_y.min(near),
                axis_y.max(near),
                Color::Grid,
            ));
            marks.push(text(
                cx,
                block_top,
                &dates[k],
                TICK_SIZE,
                Weight::Regular,
                anchor,
                Baseline::Top,
                Color::TextMuted,
            ));
            marks.push(text(
                cx,
                block_top + TICK_SIZE + 4.0,
                &e.label,
                LABEL_SIZE,
                Weight::Semibold,
                anchor,
                Baseline::Top,
                Color::TextPrimary,
            ));
            let area = Rect {
                x: x0.min(cx - DOT) - 4.0,
                y: block_top.min(axis_y - DOT),
                w: x1.max(cx + DOT) - x0.min(cx - DOT) + 8.0,
                h: (stem_end.max(axis_y + DOT) - block_top.min(axis_y - DOT)).abs(),
            };
            marks.push(
                Mark::new(Shape::Circle {
                    cx,
                    cy: axis_y,
                    r: DOT,
                })
                .fill(Color::Series(0))
                .stroke(Color::Surface, SURFACE_GAP)
                .hit(
                    Hit::new(
                        i.to_string(),
                        time::format_datetime(e.time, offset),
                        e.label.as_str(),
                    )
                    .area(area),
                ),
            );
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Timeline of {}.",
                    count(self.events.len(), "event", "events")
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
    fn labels_alternate_and_climb_only_when_they_would_collide() {
        // Three close labels: above, below, then above again one level up.
        // A far one drops back to the first level.
        let l = levels(
            &[(0.0, 50.0), (10.0, 60.0), (20.0, 70.0), (200.0, 250.0)],
            3,
        );
        assert_eq!(l, vec![1, -1, 2, 1]);
    }

    #[test]
    fn when_every_level_is_taken_the_least_crowded_one_is_reused() {
        let l = levels(&[(0.0, 50.0), (10.0, 60.0), (20.0, 30.0)], 1);
        assert_eq!(l, vec![1, -1, 1]);
    }

    #[test]
    fn events_sit_on_the_axis_in_time_order() {
        let c = TimelineChart::new(vec![
            Event::new(3 * time::DAY, "C"),
            Event::new(0, "A"),
            Event::new(time::DAY, "B"),
        ]);
        let s = c
            .scene(640.0, 400.0, &crate::base::text::InterMetrics)
            .unwrap();
        let dots: Vec<(f64, &str)> = s
            .marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Circle { cx, .. }, Some(h)) => Some((*cx, h.value.as_str())),
                _ => None,
            })
            .collect();
        assert_eq!(
            dots.iter().map(|d| d.1).collect::<Vec<_>>(),
            ["A", "B", "C"]
        );
        // Spacing follows time: C is twice as far from B as B from A.
        assert!(((dots[2].0 - dots[1].0) / (dots[1].0 - dots[0].0) - 2.0).abs() < 1e-9);
    }
}
