use super::frame::{self, Key, hline, text};
use super::{Chart, GAP, MAX_SERIES, PAD, TICK_SIZE, check_size, count, describe};
use crate::base::scale::{BandScale, LinearScale};
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Corners, Hit, Mark, Rect, Scene, Shape, TipRow};

const MINUTES_PER_DAY: u32 = 24 * 60;
const ACCENT: f64 = 3.0;
const RADIUS: f64 = 4.0;
const LABEL_SIZE: f64 = 11.0;
/// Hour rows closer than this label every other hour.
const HOUR_MIN: f64 = 20.0;

/// One booking: `start`/`end` in minutes after midnight on `day`.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Slot {
    pub day: String,
    pub start: u32,
    pub end: u32,
    pub label: String,
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub group: Option<String>,
}

impl Slot {
    pub fn new(day: impl Into<String>, start: u32, end: u32, label: impl Into<String>) -> Self {
        Slot {
            day: day.into(),
            start,
            end,
            label: label.into(),
            group: None,
        }
    }

    pub fn group(mut self, group: impl Into<String>) -> Self {
        self.group = Some(group.into());
        self
    }
}

/// A weekly schedule: a column per day, time of day running down, each
/// booking a block; overlapping bookings share their column side by side.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct TimetableChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub days: Vec<String>,
    pub slots: Vec<Slot>,
}

impl TimetableChart {
    pub fn new<S: Into<String>>(days: impl IntoIterator<Item = S>) -> Self {
        TimetableChart {
            title: None,
            days: days.into_iter().map(Into::into).collect(),
            slots: Vec::new(),
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    pub fn slot(mut self, slot: Slot) -> Self {
        self.slots.push(slot);
        self
    }
}

fn clock(minutes: u32) -> String {
    format!("{:02}:{:02}", minutes / 60, minutes % 60)
}

/// Lane per slot (index into `slots`) and lanes used per day: each slot
/// takes the first lane free at its start, in start order.
pub(crate) fn lanes(slots: &[(usize, u32, u32)], days: usize) -> (Vec<usize>, Vec<usize>) {
    let mut lane = vec![0; slots.len()];
    let mut used = vec![0; days];
    let mut order: Vec<usize> = (0..slots.len()).collect();
    order.sort_by_key(|&i| (slots[i].0, slots[i].1, slots[i].2));
    let mut ends: Vec<Vec<u32>> = vec![Vec::new(); days];
    for i in order {
        let (d, start, end) = slots[i];
        let k = ends[d]
            .iter()
            .position(|&e| e <= start)
            .unwrap_or(ends[d].len());
        if k == ends[d].len() {
            ends[d].push(end);
        } else {
            ends[d][k] = end;
        }
        lane[i] = k;
        used[d] = used[d].max(k + 1);
    }
    (lane, used)
}

impl Chart for TimetableChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let mut groups: Vec<String> = Vec::new();
        let mut placed: Vec<(usize, u32, u32)> = Vec::new();
        for (i, s) in self.slots.iter().enumerate() {
            let d = self.days.iter().position(|d| *d == s.day).ok_or_else(|| {
                Error::InvalidData(format!(
                    "slot {i} ({:?}) is on unknown day {:?}",
                    s.label, s.day
                ))
            })?;
            if s.start >= s.end || s.end > MINUTES_PER_DAY {
                return Err(Error::InvalidData(format!(
                    "slot {i} ({:?}) must start before it ends, within the day",
                    s.label
                )));
            }
            if let Some(g) = &s.group
                && !groups.contains(g)
            {
                groups.push(g.clone());
            }
            placed.push((d, s.start, s.end));
        }
        if groups.len() > MAX_SERIES {
            return Err(Error::InvalidData(format!(
                "{} groups, at most {MAX_SERIES}: merge some",
                groups.len()
            )));
        }
        let (first, last) = self
            .slots
            .iter()
            .fold((u32::MAX, 0), |(a, b), s| (a.min(s.start), b.max(s.end)));
        let (h0, h1) = if first < last {
            (first / 60, last.div_ceil(60))
        } else {
            (8, 18)
        };
        let (lane, used) = lanes(&placed, self.days.len());

        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &groups, Key::Swatch);
        let header = top;
        let grid_top = top + TICK_SIZE + GAP + TICK_SIZE / 2.0;
        let bottom = (height - PAD - TICK_SIZE / 2.0).max(grid_top + 1.0);
        let left = PAD + measure.width("00:00", TICK_SIZE, Weight::Regular) + GAP;
        let right = (width - PAD).max(left + 1.0);
        let y = LinearScale::new(((h0 * 60) as f64, (h1 * 60) as f64), (grid_top, bottom));
        let cols = BandScale::new(self.days.len(), (left, right), 0.08, 0.0);
        let hour_px = (bottom - grid_top) / (h1 - h0).max(1) as f64;
        let every = if hour_px < HOUR_MIN { 2 } else { 1 };
        for h in (h0..=h1).filter(|h| (h - h0) % every == 0) {
            let py = y.map((h * 60) as f64);
            marks.push(hline(left, right, super::snap(py), Color::Grid));
            marks.push(text(
                left - GAP,
                py,
                &clock(h * 60),
                TICK_SIZE,
                Weight::Regular,
                Anchor::End,
                Baseline::Middle,
                Color::TextMuted,
            ));
        }
        for (d, name) in self.days.iter().enumerate() {
            marks.push(text(
                cols.position(d) + cols.bandwidth() / 2.0,
                header,
                name,
                TICK_SIZE,
                Weight::Semibold,
                Anchor::Middle,
                Baseline::Top,
                Color::TextSecondary,
            ));
        }
        for (i, s) in self.slots.iter().enumerate() {
            let (d, ..) = placed[i];
            let lane_w = cols.bandwidth() / used[d].max(1) as f64;
            let g = s
                .group
                .as_ref()
                .and_then(|g| groups.iter().position(|o| o == g));
            let c = g.unwrap_or(0) as u8;
            let r = Rect {
                x: cols.position(d) + lane[i] as f64 * lane_w + 1.0,
                y: y.map(s.start as f64) + 1.0,
                w: (lane_w - 2.0).max(1.0),
                h: (y.map(s.end as f64) - y.map(s.start as f64) - 2.0).max(1.0),
            };
            let label = format!("{} · {}", s.label, s.day);
            let mut rows = Vec::new();
            if let Some(gr) = &s.group {
                rows.push(TipRow {
                    color: None,
                    name: "group".into(),
                    value: gr.clone(),
                });
            }
            // A light wash with an accent edge: the label stays dark text on
            // every palette hue, which a solid fill would not allow.
            let mut block = Mark::new(Shape::Rect {
                rect: r,
                radius: Corners::all(RADIUS.min(r.w / 2.0).min(r.h / 2.0)),
            })
            .fill(Color::Wash(c))
            .hit(
                Hit::new(
                    i.to_string(),
                    label,
                    format!("{}–{}", clock(s.start), clock(s.end)),
                )
                .rows(rows),
            );
            let mut accent = Mark::new(Shape::Rect {
                rect: Rect {
                    w: ACCENT.min(r.w),
                    ..r
                },
                radius: Corners {
                    tl: RADIUS.min(r.h / 2.0).min(ACCENT),
                    tr: 0.0,
                    br: 0.0,
                    bl: RADIUS.min(r.h / 2.0).min(ACCENT),
                },
            })
            .fill(Color::Series(c));
            if let Some(gr) = &s.group {
                block = block.series(gr.as_str());
                accent = accent.series(gr.as_str());
            }
            marks.push(block);
            marks.push(accent);
            let tw = measure.width(&s.label, LABEL_SIZE, Weight::Semibold);
            if r.h >= LABEL_SIZE + 8.0 && tw <= r.w - ACCENT - 8.0 {
                let mut t = text(
                    r.x + ACCENT + 4.0,
                    r.y + 4.0,
                    &s.label,
                    LABEL_SIZE,
                    Weight::Semibold,
                    Anchor::Start,
                    Baseline::Top,
                    Color::TextPrimary,
                );
                if let Some(gr) = &s.group {
                    t = t.series(gr.as_str());
                }
                marks.push(t);
            }
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Timetable of {} across {}.",
                    count(self.slots.len(), "slot", "slots"),
                    count(self.days.len(), "day", "days")
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
    fn overlapping_slots_share_the_column_side_by_side() {
        // Day 0: 9–11 and 10–12 overlap (two lanes); 11–12 reuses lane 0.
        let (lane, used) = lanes(
            &[(0, 540, 660), (0, 600, 720), (0, 660, 720), (1, 540, 600)],
            2,
        );
        assert_eq!(lane, vec![0, 1, 0, 0]);
        assert_eq!(used, vec![2, 1]);
    }

    #[test]
    fn blocks_span_their_minutes() {
        let c = TimetableChart::new(["Mon", "Tue"])
            .slot(Slot::new("Mon", 9 * 60, 10 * 60 + 30, "Maths"))
            .slot(Slot::new("Tue", 13 * 60, 14 * 60, "Art"));
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let blocks: Vec<(Rect, &Hit)> = s
            .marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Rect { rect, .. }, Some(h)) => Some((*rect, h)),
                _ => None,
            })
            .collect();
        assert_eq!(blocks[0].1.value, "09:00–10:30");
        // 90 minutes against 60: one and a half times as tall (the 2px gap
        // aside).
        let (a, b) = (blocks[0].0.h + 2.0, blocks[1].0.h + 2.0);
        assert!((a / b - 1.5).abs() < 1e-9);
    }

    #[test]
    fn unknown_days_and_inverted_slots_are_rejected() {
        let m = &InterMetrics;
        let c = TimetableChart::new(["Mon"]).slot(Slot::new("Sun", 0, 60, "x"));
        assert!(c.scene(640.0, 400.0, m).is_err());
        let c = TimetableChart::new(["Mon"]).slot(Slot::new("Mon", 60, 60, "x"));
        assert!(c.scene(640.0, 400.0, m).is_err());
    }
}
