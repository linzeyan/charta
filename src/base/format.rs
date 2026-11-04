//! Number formatting for coordinates, values and axis ticks.
//!
//! The Go port must print identical strings, so this sticks to operations
//! both languages define the same way: correctly rounded fixed-point
//! formatting, shortest round-trip digits, and round-half-away-from-zero.

/// A coordinate for SVG and scene dumps: two decimals, trailing zeros trimmed.
pub fn coord(v: f64) -> String {
    let r = (v * 100.0).round() / 100.0;
    trim_zeros(format!("{:.2}", positive_zero(r)))
}

/// A data value for tooltips and tables: every significant digit the input
/// had, with thousands grouped.
pub fn value(v: f64) -> String {
    group(&format!("{}", positive_zero(v)))
}

/// A computed statistic (quartile, mean, bandwidth) for a tooltip: three
/// significant digits, never fewer than the integer part, thousands grouped.
/// Data values use [`value`] instead, which keeps every digit given.
pub fn approx(v: f64) -> String {
    let a = v.abs();
    let decimals = if a == 0.0 || a >= 100.0 {
        0
    } else {
        (2 - super::ticks::power_floor(a)).clamp(0, 6) as usize
    };
    // Round half away from zero first, as `coord` does: formatting alone
    // rounds exact ties to even (41.25 → "41.2").
    let m = super::ticks::pow10(decimals as i32);
    let r = (v * m).round() / m;
    let s = trim_zeros(format!("{:.*}", decimals, positive_zero(r)));
    group(if s == "-0" { "0" } else { &s })
}

/// One label style for a whole axis: in full below 10,000, compact (10K,
/// 2.5M) once any tick reaches it, with just enough decimals for the step.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct TickFormat {
    divisor: f64,
    suffix: &'static str,
    decimals: usize,
}

impl TickFormat {
    pub fn for_ticks(ticks: &[f64]) -> Self {
        let max = ticks.iter().fold(0.0_f64, |m, t| m.max(t.abs()));
        let (divisor, suffix) = if max >= 1e12 {
            (1e12, "T")
        } else if max >= 1e9 {
            (1e9, "B")
        } else if max >= 1e6 {
            (1e6, "M")
        } else if max >= 1e4 {
            (1e3, "K")
        } else {
            (1.0, "")
        };
        let step = if ticks.len() >= 2 {
            (ticks[1] - ticks[0]).abs()
        } else {
            0.0
        };
        TickFormat {
            divisor,
            suffix,
            decimals: decimals_for(step / divisor),
        }
    }

    /// Zero is always "0". Full labels keep a fixed number of decimals so a
    /// column of them aligns; compact ones drop trailing zeros ("1M", not
    /// "1.0M"), since the suffix already breaks alignment.
    pub fn format(&self, v: f64) -> String {
        let x = positive_zero(v / self.divisor);
        if x == 0.0 {
            return "0".to_string();
        }
        let fixed = format!("{:.*}", self.decimals, x);
        let digits = if self.suffix.is_empty() {
            fixed
        } else {
            trim_zeros(fixed)
        };
        let mut s = group(&digits);
        s.push_str(self.suffix);
        s
    }
}

/// Fewest decimals (at most 6) that show `step` exactly: 0.25 → 2, 5 → 0.
fn decimals_for(step: f64) -> usize {
    if step.is_nan() || step <= 0.0 {
        return 0;
    }
    let mut scaled = step;
    for d in 0..6 {
        if (scaled - scaled.round()).abs() <= 1e-9 * scaled.max(1.0) {
            return d;
        }
        scaled *= 10.0;
    }
    6
}

/// `-0.0` prints as "-0" in both languages; nobody wants that on an axis.
fn positive_zero(v: f64) -> f64 {
    if v == 0.0 { 0.0 } else { v }
}

fn trim_zeros(s: String) -> String {
    if s.contains('.') {
        s.trim_end_matches('0').trim_end_matches('.').to_string()
    } else {
        s
    }
}

fn group(s: &str) -> String {
    let (sign, rest) = match s.strip_prefix('-') {
        Some(r) => ("-", r),
        None => ("", s),
    };
    let (int, frac) = match rest.split_once('.') {
        Some((i, f)) => (i, Some(f)),
        None => (rest, None),
    };
    let mut out = String::with_capacity(s.len() + int.len() / 3);
    out.push_str(sign);
    for (i, c) in int.chars().enumerate() {
        if i > 0 && (int.len() - i) % 3 == 0 {
            out.push(',');
        }
        out.push(c);
    }
    if let Some(f) = frac {
        out.push('.');
        out.push_str(f);
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn coord_trims_and_never_prints_negative_zero() {
        assert_eq!(coord(12.0), "12");
        assert_eq!(coord(12.345), "12.35");
        assert_eq!(coord(-0.001), "0");
        assert_eq!(coord(0.1 + 0.2), "0.3");
    }

    #[test]
    fn value_keeps_input_digits_and_groups_thousands() {
        assert_eq!(value(1234567.5), "1,234,567.5");
        assert_eq!(value(-1200.0), "-1,200");
        assert_eq!(value(0.125), "0.125");
    }

    #[test]
    fn axis_switches_to_compact_once_a_tick_reaches_ten_thousand() {
        // dataviz rule: full numbers under 10,000, compact on an axis that
        // reaches it, one style for the whole axis.
        let full = TickFormat::for_ticks(&[0.0, 2000.0, 4000.0, 6000.0, 8000.0]);
        assert_eq!(full.format(8000.0), "8,000");
        let compact = TickFormat::for_ticks(&[0.0, 5000.0, 10000.0]);
        assert_eq!(compact.format(5000.0), "5K");
        assert_eq!(compact.format(10000.0), "10K");
        let half = TickFormat::for_ticks(&[0.0, 2.5e6, 5e6]);
        assert_eq!(half.format(2.5e6), "2.5M");
    }

    #[test]
    fn compact_labels_drop_trailing_zeros_and_zero_is_plain() {
        let f = TickFormat::for_ticks(&[0.0, 2e5, 4e5, 6e5, 8e5, 1e6, 1.2e6]);
        assert_eq!(f.format(0.0), "0");
        assert_eq!(f.format(1e6), "1M");
        assert_eq!(f.format(1.2e6), "1.2M");
        assert_eq!(f.format(4e5), "0.4M");
    }

    #[test]
    fn approx_keeps_three_significant_digits() {
        assert_eq!(approx(2.313_452), "2.31");
        assert_eq!(approx(0.012_345), "0.0123");
        assert_eq!(approx(41.25), "41.3");
        assert_eq!(approx(1234.5), "1,235");
        assert_eq!(approx(3.0), "3");
        assert_eq!(approx(-0.0001), "-0.0001");
        assert_eq!(approx(-0.000_000_1), "0");
    }

    #[test]
    fn tick_decimals_follow_the_step() {
        let f = TickFormat::for_ticks(&[0.0, 0.25, 0.5]);
        assert_eq!(f.format(0.5), "0.50");
        assert_eq!(TickFormat::for_ticks(&[0.0, 0.1, 0.2]).format(0.2), "0.2");
    }
}
