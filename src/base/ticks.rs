//! Tick generation and domain "nicing", ported from d3-array `ticks` and
//! d3-scale `linear.nice` (ISC license, Mike Bostock).
//!
//! One deliberate deviation: d3 finds the power of ten with `log10` and `pow`,
//! whose last-ulp results differ between Rust's libm and Go's pure-Go math.
//! That can flip a `floor` at exact powers of ten and make the two ports pick
//! different steps, so powers come from a literal table instead.

const POW10_MIN: i32 = -20;
const POW10: [f64; 41] = [
    1e-20, 1e-19, 1e-18, 1e-17, 1e-16, 1e-15, 1e-14, 1e-13, 1e-12, 1e-11, 1e-10, 1e-9, 1e-8, 1e-7,
    1e-6, 1e-5, 1e-4, 1e-3, 1e-2, 1e-1, 1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10,
    1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19, 1e20,
];

pub(crate) fn pow10(p: i32) -> f64 {
    POW10[(p.clamp(POW10_MIN, -POW10_MIN) - POW10_MIN) as usize]
}

/// Largest `p` with `10^p <= x`, clamped to the table.
pub(crate) fn power_floor(x: f64) -> i32 {
    let mut p = POW10_MIN;
    while p < -POW10_MIN && pow10(p + 1) <= x {
        p += 1;
    }
    p
}

/// Returns `(i1, i2, inc)`: ticks are `i * inc` for `i` in `i1..=i2`, or
/// `i / -inc` when `inc` is negative (d3's trick to keep sub-1 steps exact).
fn tick_spec(start: f64, stop: f64, count: f64) -> (f64, f64, f64) {
    let step = (stop - start) / count.max(0.0);
    let power = power_floor(step);
    let error = step / pow10(power);
    let factor = if error >= 50f64.sqrt() {
        10.0
    } else if error >= 10f64.sqrt() {
        5.0
    } else if error >= 2f64.sqrt() {
        2.0
    } else {
        1.0
    };
    let (mut i1, mut i2, inc);
    if power < 0 {
        let inv = pow10(-power) / factor;
        i1 = (start * inv).round();
        i2 = (stop * inv).round();
        if i1 / inv < start {
            i1 += 1.0;
        }
        if i2 / inv > stop {
            i2 -= 1.0;
        }
        inc = -inv;
    } else {
        let s = pow10(power) * factor;
        i1 = (start / s).round();
        i2 = (stop / s).round();
        if i1 * s < start {
            i1 += 1.0;
        }
        if i2 * s > stop {
            i2 -= 1.0;
        }
        inc = s;
    }
    if i2 < i1 && (0.5..2.0).contains(&count) {
        return tick_spec(start, stop, count * 2.0);
    }
    (i1, i2, inc)
}

/// About `count` round-numbered ticks (multiples of 1, 2 or 5 × 10ⁿ) inside
/// `[start, stop]`. Requires `start <= stop`.
pub fn ticks(start: f64, stop: f64, count: usize) -> Vec<f64> {
    if count == 0 {
        return Vec::new();
    }
    if start == stop {
        return vec![start];
    }
    let (i1, i2, inc) = tick_spec(start, stop, count as f64);
    if i1.is_nan() || i2.is_nan() || i2 < i1 {
        return Vec::new();
    }
    let n = (i2 - i1 + 1.0) as usize;
    (0..n)
        .map(|i| {
            let k = i1 + i as f64;
            if inc < 0.0 { k / -inc } else { k * inc }
        })
        .collect()
}

/// Extends `[start, stop]` outward to the tick step, so the axis starts and
/// ends on a labelled tick. Requires `start <= stop`.
pub fn nice(start: f64, stop: f64, count: usize) -> (f64, f64) {
    let (mut start, mut stop) = (start, stop);
    let mut prestep: Option<f64> = None;
    for _ in 0..10 {
        let step = tick_spec(start, stop, count as f64).2;
        if prestep == Some(step) {
            break;
        }
        if step > 0.0 {
            start = (start / step).floor() * step;
            stop = (stop / step).ceil() * step;
        } else if step < 0.0 {
            start = (start * step).ceil() / step;
            stop = (stop * step).floor() / step;
        } else {
            break;
        }
        prestep = Some(step);
    }
    (start, stop)
}

#[cfg(test)]
mod tests {
    use super::*;

    /// A tick step is "round" when it is 1, 2 or 5 times a power of ten.
    fn is_round_step(step: f64) -> bool {
        let p = power_floor(step);
        let m = step / pow10(p);
        [1.0, 2.0, 5.0, 10.0].iter().any(|r| (m - r).abs() < 1e-9)
    }

    #[test]
    fn nice_domain_starts_and_ends_on_a_tick() {
        // An axis that stops short of its last label looks broken; nice() must
        // land both ends on the tick grid.
        for &(a, b, n) in &[
            (0.0, 87.0, 5),
            (-3.2, 41.7, 8),
            (0.0, 0.37, 6),
            (12.0, 13.0, 4),
        ] {
            let (lo, hi) = nice(a, b, n);
            assert!(lo <= a && hi >= b, "{lo}..{hi} must cover {a}..{b}");
            let t = ticks(lo, hi, n);
            assert_eq!(t.first().copied(), Some(lo), "{a}..{b}");
            assert_eq!(t.last().copied(), Some(hi), "{a}..{b}");
        }
    }

    #[test]
    fn ticks_are_evenly_spaced_round_numbers() {
        for &(a, b, n) in &[
            (0.0, 100.0, 5),
            (0.0, 0.3, 6),
            (-1000.0, 2500.0, 7),
            (0.0, 1e9, 4),
        ] {
            let t = ticks(a, b, n);
            assert!(t.len() >= 2, "{a}..{b} gave {t:?}");
            let step = t[1] - t[0];
            assert!(is_round_step(step), "step {step} for {a}..{b}");
            for w in t.windows(2) {
                assert!(((w[1] - w[0]) - step).abs() <= step * 1e-9, "{t:?}");
            }
        }
    }

    #[test]
    fn sub_unit_ticks_are_exact_decimals() {
        // Accumulating 0.1 steps yields 0.30000000000000004; d3's inverse
        // increment must give the literal decimals, or labels show noise.
        assert_eq!(ticks(0.0, 0.3, 3), vec![0.0, 0.1, 0.2, 0.3]);
    }

    #[test]
    fn exact_power_of_ten_steps() {
        assert_eq!(ticks(0.0, 1000.0, 1), vec![0.0, 1000.0]);
        assert_eq!(power_floor(1000.0), 3);
        assert_eq!(power_floor(999.0), 2);
    }
}
