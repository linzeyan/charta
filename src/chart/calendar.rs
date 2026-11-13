use super::frame::{self, text};
use super::{
    Chart, GAP, PAD, SURFACE_GAP, TICK_SIZE, TimeValue, check_size, count, describe, finite,
};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::base::time::{self, DAY, days_from_civil};
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Corners, Hit, Mark, Rect, Scene, Shape};

const CELL_MAX: f64 = 16.0;
const CELL_RADIUS: f64 = 2.0;
/// Weeks a year can touch (53 full, plus a partial one).
const WEEKS: f64 = 54.0;
const YEAR_SIZE: f64 = 12.0;

/// A year per block, a column per week and a row per weekday (Monday on
/// top): one cell per day, colored by its value on a sequential ramp.
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::base::time::{DAY, to_ms};
/// use charta::{CalendarChart, Chart, TimeValue};
///
/// let start = to_ms(2025, 1, 1, 0, 0, 0);
/// let commits = [3.0, 5.0, 2.0, 0.0, 1.0, 4.0, 6.0, 7.0, 2.0, 3.0, 0.0, 0.0, 5.0, 8.0];
/// let days = commits
///     .iter()
///     .enumerate()
///     .map(|(i, &v)| TimeValue::new(start + i as i64 * DAY, v))
///     .collect();
/// let chart = CalendarChart::new(days).title("Commits per day");
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Commits per day. Calendar heatmap of 14 days across 1 year.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
pub struct CalendarChart {
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
    /// One value per day; `time` is any instant in that (local) day.
    pub days: Vec<TimeValue>,
}

impl CalendarChart {
    pub fn new(days: Vec<TimeValue>) -> Self {
        CalendarChart {
            days,
            ..Self::default()
        }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }
}

impl Chart for CalendarChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let offset = self.utc_offset.unwrap_or(0);
        // Local day number → value.
        let mut by_day: Vec<(i64, f64, i64)> = Vec::with_capacity(self.days.len());
        for (i, d) in self.days.iter().enumerate() {
            finite(&format!("day {i}"), d.value)?;
            let c = time::civil(d.time, offset);
            by_day.push((days_from_civil(c.year, c.month, c.day), d.value, d.time));
        }
        by_day.sort_by_key(|e| e.0);
        if let Some(w) = by_day.windows(2).find(|w| w[0].0 == w[1].0) {
            return Err(Error::InvalidData(format!(
                "two values for {}",
                time::format_date(w[1].2, offset)
            )));
        }
        let (lo, hi) = by_day
            .iter()
            .fold((f64::INFINITY, f64::NEG_INFINITY), |(a, b), e| {
                (a.min(e.1), b.max(e.1))
            });
        let years: Vec<i64> = match (by_day.first(), by_day.last()) {
            (Some(a), Some(b)) => {
                let y = |d: i64| time::civil(d * DAY, 0).year;
                (y(a.0)..=y(b.0)).collect()
            }
            _ => Vec::new(),
        };

        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        if lo.is_finite() {
            top = frame::seq_legend(&mut marks, measure, top, lo, hi);
        }
        let wd_w = ["Mon", "Wed", "Fri"]
            .iter()
            .map(|d| measure.width(d, TICK_SIZE, Weight::Regular))
            .fold(0.0, f64::max);
        let year_w = years
            .iter()
            .map(|y| measure.width(&y.to_string(), YEAR_SIZE, Weight::Semibold))
            .fold(0.0, f64::max);
        let left = PAD + wd_w.max(year_w) + GAP;
        let header = TICK_SIZE + GAP;
        let block_gap = 2.0 * GAP;
        let n = years.len().max(1) as f64;
        let avail = height - PAD - top;
        let tall = (avail - n * (header + block_gap) + block_gap) / (7.0 * n);
        // Past this, year blocks would run off the bottom of the chart.
        if tall < 1.0 && !years.is_empty() {
            return Err(Error::InvalidData(format!(
                "{} not fit in {height}px: show fewer years or a taller chart",
                count(years.len(), "year does", "years do")
            )));
        }
        let cell = ((width - PAD - left) / WEEKS)
            .min(tall)
            .clamp(1.0, CELL_MAX);
        let half = SURFACE_GAP / 2.0;

        let mut idx = 0;
        for (yi, &year) in years.iter().enumerate() {
            let block_top = top + yi as f64 * (header + 7.0 * cell + block_gap);
            let grid_top = block_top + header;
            let jan1 = days_from_civil(year, 1, 1);
            let first_monday = jan1 - (jan1 + 3).rem_euclid(7);
            let end = days_from_civil(year + 1, 1, 1);
            marks.push(text(
                PAD,
                block_top,
                &year.to_string(),
                YEAR_SIZE,
                Weight::Semibold,
                Anchor::Start,
                Baseline::Top,
                Color::TextSecondary,
            ));
            for (row, name) in [(0, "Mon"), (2, "Wed"), (4, "Fri")] {
                marks.push(text(
                    left - GAP,
                    grid_top + (row as f64 + 0.5) * cell,
                    name,
                    TICK_SIZE,
                    Weight::Regular,
                    Anchor::End,
                    Baseline::Middle,
                    Color::TextMuted,
                ));
            }
            for m in 1..=12u32 {
                let d = days_from_civil(year, m, 1);
                marks.push(text(
                    left + ((d - first_monday) / 7) as f64 * cell,
                    block_top + (YEAR_SIZE - TICK_SIZE),
                    time::month_name(m),
                    TICK_SIZE,
                    Weight::Regular,
                    Anchor::Start,
                    Baseline::Top,
                    Color::TextMuted,
                ));
            }
            for d in jan1..end {
                let (col, row) = ((d - first_monday) / 7, (d - first_monday) % 7);
                let slot = Rect {
                    x: left + col as f64 * cell,
                    y: grid_top + row as f64 * cell,
                    w: cell,
                    h: cell,
                };
                let shape = Shape::Rect {
                    rect: Rect {
                        x: slot.x + half,
                        y: slot.y + half,
                        w: (cell - SURFACE_GAP).max(0.5),
                        h: (cell - SURFACE_GAP).max(0.5),
                    },
                    radius: Corners::all(CELL_RADIUS.min(cell / 4.0)),
                };
                while idx < by_day.len() && by_day[idx].0 < d {
                    idx += 1;
                }
                let m = match by_day.get(idx) {
                    Some(&(day, v, t)) if day == d => {
                        let c = time::civil(t, offset);
                        Mark::new(shape)
                            .fill(Color::Seq(frame::seq_class(v, lo, hi)))
                            .hit(
                                Hit::new(
                                    format!("d{d}"),
                                    format!(
                                        "{} {}",
                                        time::weekday_name(c.weekday),
                                        time::format_date(t, offset)
                                    ),
                                    format::value(v),
                                )
                                .area(slot),
                            )
                    }
                    _ => Mark::new(shape).fill(Color::Grid),
                };
                marks.push(m);
            }
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Calendar heatmap of {} across {}.",
                    count(by_day.len(), "day", "days"),
                    count(years.len(), "year", "years")
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
    use crate::base::time::to_ms;

    #[test]
    fn more_years_than_fit_are_rejected_rather_than_drawn_off_the_chart() {
        let days: Vec<TimeValue> = (0..30)
            .map(|y| TimeValue::new(crate::base::time::to_ms(1990 + y, 6, 1, 0, 0, 0), 1.0))
            .collect();
        let c = CalendarChart::new(days);
        assert!(c.scene(640.0, 400.0, &InterMetrics).is_err());
        assert!(c.scene(640.0, 4000.0, &InterMetrics).is_ok());
    }

    #[test]
    fn days_fall_in_their_weekday_row_and_week_column() {
        // 2026-01-01 is a Thursday (row 3); Monday 2026-01-05 opens week 1.
        let c = CalendarChart::new(vec![
            TimeValue::new(to_ms(2026, 1, 1, 9, 0, 0), 1.0),
            TimeValue::new(to_ms(2026, 1, 5, 9, 0, 0), 5.0),
        ]);
        let s = c.scene(900.0, 300.0, &InterMetrics).unwrap();
        let cells: Vec<(Rect, &str)> = s
            .marks
            .iter()
            .filter_map(|m| m.hit.as_ref().map(|h| (h.area.unwrap(), h.label.as_str())))
            .collect();
        assert_eq!(cells[0].1, "Thu Jan 1, 2026");
        assert_eq!(cells[1].1, "Mon Jan 5, 2026");
        let cell = cells[0].0.w;
        assert!((cells[1].0.x - cells[0].0.x - cell).abs() < 1e-9);
        assert!((cells[0].0.y - cells[1].0.y - 3.0 * cell).abs() < 1e-9);
        // Every day of the year gets a cell, with data or not.
        let all = s
            .marks
            .iter()
            .filter(|m| matches!(m.shape, Shape::Rect { .. }) && m.fill != Some(Color::Surface))
            .count();
        assert!(all >= 365);
    }

    #[test]
    fn the_offset_decides_which_day_a_value_belongs_to() {
        let late = to_ms(2026, 6, 1, 22, 0, 0);
        let s = CalendarChart {
            utc_offset: Some(180),
            ..CalendarChart::new(vec![TimeValue::new(late, 1.0)])
        }
        .scene(900.0, 300.0, &InterMetrics)
        .unwrap();
        let h = s.marks.iter().find_map(|m| m.hit.as_ref()).unwrap();
        assert_eq!(h.label, "Tue Jun 2, 2026");
    }

    #[test]
    fn two_values_on_one_day_are_rejected() {
        let d = to_ms(2026, 1, 1, 0, 0, 0);
        let c = CalendarChart::new(vec![
            TimeValue::new(d, 1.0),
            TimeValue::new(d + 3_600_000, 2.0),
        ]);
        assert!(c.scene(900.0, 300.0, &InterMetrics).is_err());
    }
}
