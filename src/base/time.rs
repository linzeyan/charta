//! Time: civil dates from UTC epoch milliseconds, round tick intervals and
//! their labels.
//!
//! Inputs are always UTC epoch milliseconds; a chart's `utc_offset` (minutes)
//! shifts only what is displayed, never the data. Everything is integer
//! arithmetic (Howard Hinnant's `civil_from_days`), so the Go port lands on
//! the same dates and ticks bit for bit, with no time zone database.

pub const SECOND: i64 = 1000;
pub const MINUTE: i64 = 60 * SECOND;
pub const HOUR: i64 = 60 * MINUTE;
pub const DAY: i64 = 24 * HOUR;
pub const WEEK: i64 = 7 * DAY;

const MONTHS: [&str; 12] = [
    "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
];
const WEEKDAYS: [&str; 7] = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

pub fn month_name(m: u32) -> &'static str {
    MONTHS[(m - 1) as usize]
}

pub fn weekday_name(w: u32) -> &'static str {
    WEEKDAYS[w as usize]
}

/// A local calendar time.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Civil {
    pub year: i64,
    /// 1–12.
    pub month: u32,
    /// 1–31.
    pub day: u32,
    pub hour: u32,
    pub minute: u32,
    pub second: u32,
    /// 0 = Monday … 6 = Sunday.
    pub weekday: u32,
}

/// Days since 1970-01-01 of a proleptic Gregorian date.
pub fn days_from_civil(y: i64, m: u32, d: u32) -> i64 {
    let y = if m <= 2 { y - 1 } else { y };
    let era = y.div_euclid(400);
    let yoe = y - era * 400;
    let mp = (m as i64 + 9) % 12;
    let doy = (153 * mp + 2) / 5 + d as i64 - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    era * 146_097 + doe - 719_468
}

fn civil_from_days(z: i64) -> (i64, u32, u32) {
    let z = z + 719_468;
    let era = z.div_euclid(146_097);
    let doe = z - era * 146_097;
    let yoe = (doe - doe / 1460 + doe / 36_524 - doe / 146_096) / 365;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let d = (doy - (153 * mp + 2) / 5 + 1) as u32;
    let m = if mp < 10 { mp + 3 } else { mp - 9 } as u32;
    let y = yoe + era * 400 + if m <= 2 { 1 } else { 0 };
    (y, m, d)
}

/// Local time at `ms` with the clock `offset` minutes ahead of UTC.
pub fn civil(ms: i64, offset: i32) -> Civil {
    let local = ms + offset as i64 * MINUTE;
    let days = local.div_euclid(DAY);
    let rem = local.rem_euclid(DAY);
    let (year, month, day) = civil_from_days(days);
    Civil {
        year,
        month,
        day,
        hour: (rem / HOUR) as u32,
        minute: (rem % HOUR / MINUTE) as u32,
        second: (rem % MINUTE / SECOND) as u32,
        // 1970-01-01 was a Thursday.
        weekday: (days + 3).rem_euclid(7) as u32,
    }
}

/// UTC milliseconds of a local date and time.
pub fn to_ms(y: i64, m: u32, d: u32, h: u32, min: u32, offset: i32) -> i64 {
    days_from_civil(y, m, d) * DAY + h as i64 * HOUR + min as i64 * MINUTE - offset as i64 * MINUTE
}

/// "Jan 5, 2026".
pub fn format_date(ms: i64, offset: i32) -> String {
    let c = civil(ms, offset);
    format!("{} {}, {}", month_name(c.month), c.day, c.year)
}

/// "Jan 5, 2026 14:30", or just the date at local midnight.
pub fn format_datetime(ms: i64, offset: i32) -> String {
    let c = civil(ms, offset);
    if c.hour == 0 && c.minute == 0 && c.second == 0 {
        format_date(ms, offset)
    } else {
        format!("{} {:02}:{:02}", format_date(ms, offset), c.hour, c.minute)
    }
}

/// A span of time in words: "14 days", "3 h 20 min", "45 min".
pub fn format_duration(ms: i64) -> String {
    let ms = ms.abs();
    if ms >= DAY && ms % DAY == 0 {
        let d = ms / DAY;
        return format!("{d} day{}", if d == 1 { "" } else { "s" });
    }
    let (h, m) = (ms / HOUR, ms % HOUR / MINUTE);
    match (h, m) {
        (0, m) => format!("{m} min"),
        (h, 0) => format!("{h} h"),
        (h, m) => format!("{h} h {m} min"),
    }
}

/// The tick spacing an axis settled on; it also decides the label style.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Interval {
    /// A fixed number of milliseconds (seconds up to weeks).
    Fixed(i64),
    Months(u32),
    Years(i64),
}

/// d3's tick intervals: fixed durations, then calendar months and years.
const FIXED: [i64; 15] = [
    SECOND,
    5 * SECOND,
    15 * SECOND,
    30 * SECOND,
    MINUTE,
    5 * MINUTE,
    15 * MINUTE,
    30 * MINUTE,
    HOUR,
    3 * HOUR,
    6 * HOUR,
    12 * HOUR,
    DAY,
    2 * DAY,
    WEEK,
];
/// Average lengths used only to choose an interval, not to place ticks.
const MONTH_APPROX: i64 = 30 * DAY;
const YEAR_APPROX: i64 = 365 * DAY;

impl Interval {
    fn approx(self) -> i64 {
        match self {
            Interval::Fixed(d) => d,
            Interval::Months(n) => n as i64 * MONTH_APPROX,
            Interval::Years(n) => n * YEAR_APPROX,
        }
    }
}

/// The interval whose length is closest (by ratio) to a `count`th of the
/// span, as d3's `ticks` picks it. Past a year, years step by 1, 2 or 5 ×
/// 10ⁿ.
pub fn interval(lo: i64, hi: i64, count: usize) -> Interval {
    let target = (hi - lo) / count.max(1) as i64;
    let mut all: Vec<Interval> = FIXED.iter().map(|&d| Interval::Fixed(d)).collect();
    all.extend([Interval::Months(1), Interval::Months(3), Interval::Years(1)]);
    if target > YEAR_APPROX {
        let years = target as f64 / YEAR_APPROX as f64;
        let ticks = super::ticks::ticks(0.0, years * count as f64, count);
        let step = if ticks.len() >= 2 {
            ticks[1] - ticks[0]
        } else {
            1.0
        };
        return Interval::Years((step.round() as i64).max(1));
    }
    let i = all
        .iter()
        .position(|v| v.approx() >= target)
        .unwrap_or(all.len() - 1);
    if i == 0 {
        return all[0];
    }
    let (a, b) = (all[i - 1], all[i]);
    // Compare target/a with b/target without division: target² vs a·b.
    if (target as i128) * (target as i128) < (a.approx() as i128) * (b.approx() as i128) {
        a
    } else {
        b
    }
}

/// Tick times in `[lo, hi]` on local round boundaries of `iv`.
pub fn ticks(lo: i64, hi: i64, iv: Interval, offset: i32) -> Vec<i64> {
    let off = offset as i64 * MINUTE;
    let mut out = Vec::new();
    match iv {
        Interval::Fixed(d) => {
            // Weeks start on Monday; 1970-01-05 was one.
            let anchor = if d == WEEK { 4 * DAY } else { 0 };
            let local = lo + off - anchor;
            let mut t = local.div_euclid(d) * d + anchor - off;
            if t < lo {
                t += d;
            }
            while t <= hi {
                out.push(t);
                t += d;
            }
        }
        Interval::Months(n) => {
            let c = civil(lo, offset);
            let mut idx = c.year * 12 + c.month as i64 - 1;
            idx -= idx.rem_euclid(n as i64);
            loop {
                let (y, m) = (idx.div_euclid(12), (idx.rem_euclid(12) + 1) as u32);
                let t = to_ms(y, m, 1, 0, 0, offset);
                if t > hi {
                    break;
                }
                if t >= lo {
                    out.push(t);
                }
                idx += n as i64;
            }
        }
        Interval::Years(n) => {
            let mut y = civil(lo, offset).year;
            y -= y.rem_euclid(n);
            loop {
                let t = to_ms(y, 1, 1, 0, 0, offset);
                if t > hi {
                    break;
                }
                if t >= lo {
                    out.push(t);
                }
                y += n;
            }
        }
    }
    out
}

/// A tick label: the coarsest unit that changes at that tick, so the year
/// shows where it turns over and the date where a day starts.
pub fn tick_label(ms: i64, iv: Interval, offset: i32) -> String {
    let c = civil(ms, offset);
    let midnight = c.hour == 0 && c.minute == 0 && c.second == 0;
    let new_year = midnight && c.month == 1 && c.day == 1;
    match iv {
        Interval::Years(_) => c.year.to_string(),
        Interval::Months(_) if c.month == 1 => c.year.to_string(),
        Interval::Months(_) => month_name(c.month).to_string(),
        Interval::Fixed(d) if d >= DAY => {
            if new_year {
                c.year.to_string()
            } else {
                format!("{} {}", month_name(c.month), c.day)
            }
        }
        Interval::Fixed(d) => {
            if midnight {
                format!("{} {}", month_name(c.month), c.day)
            } else if d < MINUTE {
                format!("{:02}:{:02}:{:02}", c.hour, c.minute, c.second)
            } else {
                format!("{:02}:{:02}", c.hour, c.minute)
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn civil_round_trips_and_knows_leap_years_and_weekdays() {
        for &(y, m, d) in &[
            (1970, 1, 1),
            (2000, 2, 29),
            (2024, 12, 31),
            (1969, 12, 31),
            (2100, 3, 1),
        ] {
            let ms = to_ms(y, m, d, 13, 45, 0);
            let c = civil(ms, 0);
            assert_eq!(
                (c.year, c.month, c.day, c.hour, c.minute),
                (y, m, d, 13, 45)
            );
        }
        // 2026-01-05 is a Monday; 2000-02-29 a Tuesday.
        assert_eq!(civil(to_ms(2026, 1, 5, 0, 0, 0), 0).weekday, 0);
        assert_eq!(civil(to_ms(2000, 2, 29, 0, 0, 0), 0).weekday, 1);
        assert_eq!(days_from_civil(1970, 1, 1), 0);
    }

    #[test]
    fn the_offset_moves_the_display_not_the_instant() {
        // 23:30 UTC is already the next day at UTC+8.
        let ms = to_ms(2026, 3, 1, 23, 30, 0);
        assert_eq!(civil(ms, 480).day, 2);
        assert_eq!(civil(ms, 480).hour, 7);
        assert_eq!(to_ms(2026, 3, 2, 7, 30, 480), ms);
    }

    #[test]
    fn intervals_aim_for_the_requested_count() {
        let day0 = to_ms(2026, 1, 1, 0, 0, 0);
        assert_eq!(interval(day0, day0 + 10 * DAY, 10), Interval::Fixed(DAY));
        assert_eq!(interval(day0, day0 + 6 * HOUR, 6), Interval::Fixed(HOUR));
        assert_eq!(interval(day0, day0 + 365 * DAY, 12), Interval::Months(1));
        assert_eq!(interval(day0, day0 + 365 * DAY, 4), Interval::Months(3));
        assert_eq!(
            interval(day0, day0 + 40 * 365 * DAY, 5),
            Interval::Years(10)
        );
    }

    #[test]
    fn ticks_land_on_local_boundaries() {
        let lo = to_ms(2026, 1, 3, 10, 0, 0);
        let hi = to_ms(2026, 4, 20, 0, 0, 0);
        let t = ticks(lo, hi, Interval::Months(1), 0);
        let labels: Vec<String> = t
            .iter()
            .map(|&x| tick_label(x, Interval::Months(1), 0))
            .collect();
        assert_eq!(labels, ["Feb", "Mar", "Apr"]);
        // Weeks start on Monday.
        for x in ticks(lo, hi, Interval::Fixed(WEEK), 0) {
            assert_eq!(civil(x, 0).weekday, 0);
        }
        // Local midnight at UTC−5 is 05:00 UTC.
        let d = ticks(lo, lo + 3 * DAY, Interval::Fixed(DAY), -300);
        assert!(
            d.iter()
                .all(|&x| civil(x, -300).hour == 0 && civil(x, 0).hour == 5)
        );
    }

    #[test]
    fn labels_name_the_unit_that_turns_over() {
        let ny = to_ms(2027, 1, 1, 0, 0, 0);
        assert_eq!(tick_label(ny, Interval::Months(1), 0), "2027");
        assert_eq!(tick_label(ny, Interval::Fixed(DAY), 0), "2027");
        assert_eq!(tick_label(ny + DAY, Interval::Fixed(DAY), 0), "Jan 2");
        assert_eq!(
            tick_label(ny + 6 * HOUR, Interval::Fixed(6 * HOUR), 0),
            "06:00"
        );
        assert_eq!(tick_label(ny + DAY, Interval::Fixed(6 * HOUR), 0), "Jan 2");
        assert_eq!(
            format_datetime(ny + 14 * HOUR + 30 * MINUTE, 0),
            "Jan 1, 2027 14:30"
        );
        assert_eq!(format_duration(14 * DAY), "14 days");
        assert_eq!(format_duration(3 * HOUR + 20 * MINUTE), "3 h 20 min");
    }
}
