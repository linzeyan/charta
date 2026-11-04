use super::frame::{self, Key, TimeAxis, X_TICK_SPACING, text};
use super::{
    Chart, GAP, MAX_SERIES, PAD, Ranges, TICK_SIZE, Viewable, check_size, count, describe, viewable,
};
use crate::base::scale::BandScale;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::base::time;
use crate::error::Error;
use crate::scene::{
    Anchor, Baseline, Corners, Hit, Mark, PathOp, Plot, Rect, Scene, Shape, TipRow,
};

const PADDING: (f64, f64) = (0.3, 0.15);
const BAR_MAX: f64 = 20.0;
const RADIUS: f64 = 4.0;

/// One task: a span of time, or a milestone when `start == end`.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Task {
    pub name: String,
    pub start: i64,
    pub end: i64,
    /// Tasks in the same group share a color; groups get a legend.
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub group: Option<String>,
}

impl Task {
    pub fn new(name: impl Into<String>, start: i64, end: i64) -> Self {
        Task {
            name: name.into(),
            start,
            end,
            group: None,
        }
    }

    pub fn group(mut self, group: impl Into<String>) -> Self {
        self.group = Some(group.into());
        self
    }
}

/// Tasks as bars on a shared time axis, one row each, in the given order.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
pub struct GanttChart {
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
    pub tasks: Vec<Task>,
}

impl GanttChart {
    pub fn new(tasks: Vec<Task>) -> Self {
        GanttChart {
            tasks,
            ..Self::default()
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }
}

viewable!(GanttChart);

/// Every task keeps its row when zoomed; bars are cut at the view's edges,
/// squarely, so a cut does not read as the task's start or end.
impl Viewable for GanttChart {
    fn laid_out(
        &self,
        width: f64,
        height: f64,
        measure: &dyn TextMeasure,
        view: Option<Ranges>,
    ) -> Result<(Scene, Plot), Error> {
        check_size(width, height)?;
        let offset = self.utc_offset.unwrap_or(0);
        let mut groups: Vec<String> = Vec::new();
        for (i, t) in self.tasks.iter().enumerate() {
            if t.end < t.start {
                return Err(Error::InvalidData(format!(
                    "task {i} ({:?}) ends before it starts",
                    t.name
                )));
            }
            if let Some(g) = &t.group
                && !groups.contains(g)
            {
                groups.push(g.clone());
            }
        }
        if groups.len() > MAX_SERIES {
            return Err(Error::InvalidData(format!(
                "{} groups, at most {MAX_SERIES}: merge some",
                groups.len()
            )));
        }
        let (t0, t1) = self.tasks.iter().fold((i64::MAX, i64::MIN), |(a, b), t| {
            (a.min(t.start), b.max(t.end))
        });
        let (t0, t1) = match view {
            Some(((a, b), _)) => (a.round() as i64, b.round() as i64),
            None if t0 <= t1 => (t0, t1),
            None => (0, time::DAY),
        };

        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &groups, Key::Swatch);
        let names: Vec<String> = self.tasks.iter().map(|t| t.name.clone()).collect();
        let label_w = names
            .iter()
            .map(|n| measure.width(n, TICK_SIZE, Weight::Regular))
            .fold(0.0, f64::max);
        let left = PAD + label_w + GAP;
        let right = (width - PAD).max(left + 1.0);
        let bottom = (height - PAD - TICK_SIZE - GAP).max(top + 1.0);
        let count_x = (((right - left) / X_TICK_SPACING).round() as usize).max(2);
        let mut x = TimeAxis::new(t0, t1, (left, right), count_x, offset);
        x.fit(measure, width - PAD);
        let right = x.scale.range.1;
        let rows = BandScale::new(self.tasks.len(), (top, bottom), PADDING.0, PADDING.1);
        x.grid(&mut marks, top, bottom);
        let thick = rows.bandwidth().min(BAR_MAX);
        for (i, t) in self.tasks.iter().enumerate() {
            let cy = rows.position(i) + rows.bandwidth() / 2.0;
            let g = t
                .group
                .as_ref()
                .and_then(|g| groups.iter().position(|o| o == g));
            let color = Color::Series(g.unwrap_or(0) as u8);
            let (start, end) = (t.start.max(t0), t.end.min(t1));
            let shown = if t.start == t.end {
                t.start >= t0 && t.start <= t1
            } else {
                start < end
            };
            let (xa, xb) = (x.map(start), x.map(end));
            let shape = if t.start == t.end {
                // A milestone is a diamond on its date.
                let r = thick / 2.0;
                Shape::Path {
                    ops: vec![
                        PathOp::Move(xa, cy - r),
                        PathOp::Line(xa + r, cy),
                        PathOp::Line(xa, cy + r),
                        PathOp::Line(xa - r, cy),
                        PathOp::Close,
                    ],
                }
            } else {
                Shape::Rect {
                    rect: Rect {
                        x: xa,
                        y: cy - thick / 2.0,
                        w: xb - xa,
                        h: thick,
                    },
                    radius: {
                        let r = RADIUS.min((xb - xa) / 2.0).min(thick / 2.0);
                        let (rs, re) = (
                            if start > t.start { 0.0 } else { r },
                            if end < t.end { 0.0 } else { r },
                        );
                        Corners {
                            tl: rs,
                            tr: re,
                            br: re,
                            bl: rs,
                        }
                    },
                }
            };
            let label = match (&t.group, groups.len() > 1) {
                (Some(g), true) => format!("{} · {g}", t.name),
                _ => t.name.clone(),
            };
            let value = if t.start == t.end {
                "milestone".to_string()
            } else {
                time::format_duration(t.end - t.start)
            };
            let mut rows_tip = vec![TipRow {
                color: None,
                name: "start".into(),
                value: time::format_datetime(t.start, offset),
            }];
            if t.end != t.start {
                rows_tip.push(TipRow {
                    color: None,
                    name: "end".into(),
                    value: time::format_datetime(t.end, offset),
                });
            }
            let band = Rect {
                x: left,
                y: rows.position(i) - (rows.step() - rows.bandwidth()) / 2.0,
                w: right - left,
                h: rows.step(),
            };
            let mut m = Mark::new(shape).fill(color).hit(
                Hit::new(i.to_string(), label, value)
                    .rows(rows_tip)
                    .area(band),
            );
            if let Some(g) = &t.group {
                m = m.series(g.as_str());
            }
            if shown {
                marks.push(m);
            }
            marks.push(text(
                left - GAP,
                cy,
                &t.name,
                TICK_SIZE,
                Weight::Regular,
                Anchor::End,
                Baseline::Middle,
                Color::TextMuted,
            ));
        }
        x.labels(&mut marks, bottom + GAP);
        let plot = Plot {
            rect: Rect {
                x: left,
                y: top,
                w: right - left,
                h: bottom - top,
            },
            x: (t0 as f64, t1 as f64),
            y: None,
        };
        let scene = Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Gantt chart of {}.",
                    count(self.tasks.len(), "task", "tasks")
                ),
            ),
            marks,
        };
        Ok((scene, plot))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;
    use crate::base::time::to_ms;

    fn plan() -> GanttChart {
        let d = |n: u32| to_ms(2026, 4, n, 0, 0, 0);
        GanttChart::new(vec![
            Task::new("Design", d(1), d(8)).group("Product"),
            Task::new("Build", d(6), d(20)).group("Engineering"),
            Task::new("Launch", d(20), d(20)).group("Product"),
        ])
    }

    #[test]
    fn bar_length_is_duration_and_milestones_are_diamonds() {
        let s = plan().scene(640.0, 400.0, &InterMetrics).unwrap();
        let tasks: Vec<&Mark> = s
            .marks
            .iter()
            .filter(|m| {
                m.hit
                    .as_ref()
                    .is_some_and(|h| h.kind == crate::scene::HitKind::Mark)
            })
            .collect();
        let w = |m: &Mark| match m.shape {
            Shape::Rect { rect, .. } => rect.w,
            _ => 0.0,
        };
        // Build lasts 14 days, Design 7: twice as long.
        assert!((w(tasks[1]) / w(tasks[0]) - 2.0).abs() < 1e-9);
        assert!(matches!(tasks[2].shape, Shape::Path { .. }));
        let h = tasks[1].hit.as_ref().unwrap();
        assert_eq!(h.value, "14 days");
        assert_eq!(h.rows[0].value, "Apr 6, 2026");
        // Groups color the bars and name the series for the legend.
        assert_eq!(tasks[1].fill, Some(Color::Series(1)));
        assert_eq!(tasks[2].series.as_deref(), Some("Product"));
    }

    #[test]
    fn a_task_cannot_end_before_it_starts() {
        let c = GanttChart::new(vec![Task::new("x", 10, 5)]);
        assert!(c.scene(640.0, 400.0, &InterMetrics).is_err());
    }

    #[test]
    fn a_view_cuts_bars_squarely_and_keeps_every_row() {
        let d = |n: u32| to_ms(2026, 4, n, 0, 0, 0) as f64;
        let (s, p) = plan()
            .view((d(4), d(15)), None)
            .unwrap()
            .plot_scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let p = p.unwrap();
        let bars: Vec<(Rect, Corners)> = s
            .marks
            .iter()
            .filter(|m| {
                m.hit
                    .as_ref()
                    .is_some_and(|h| h.kind == crate::scene::HitKind::Mark)
            })
            .filter_map(|m| match m.shape {
                Shape::Rect { rect, radius } => Some((rect, radius)),
                _ => None,
            })
            .collect();
        // Design (1–8) is cut on the left, Build (6–20) on the right; the
        // launch milestone (20) is out of view.
        assert_eq!(bars.len(), 2);
        let (design, build) = (bars[0], bars[1]);
        assert!((design.0.x - p.rect.x).abs() < 1e-9);
        assert_eq!((design.1.tl, design.1.bl), (0.0, 0.0));
        assert!(design.1.tr > 0.0);
        assert!((build.0.x + build.0.w - (p.rect.x + p.rect.w)).abs() < 1e-9);
        assert_eq!((build.1.tr, build.1.br), (0.0, 0.0));
        assert!(
            !s.marks
                .iter()
                .any(|m| matches!(m.shape, Shape::Path { .. }))
        );
        for name in ["Design", "Build", "Launch"] {
            assert!(
                s.marks
                    .iter()
                    .any(|m| matches!(&m.shape, Shape::Text { text, .. } if text == name))
            );
        }
    }
}
