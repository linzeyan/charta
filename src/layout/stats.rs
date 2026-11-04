//! Summary statistics, kernel density estimation and binning.
//!
//! Every function takes values already sorted ascending, so a chart sorts
//! once and reuses the order for quantiles, extents and the KDE.

use crate::base::ticks;

/// The `p`-quantile (0..=1) by linear interpolation between order
/// statistics (R type 7, d3's `quantileSorted`). Empty input gives NaN.
pub fn quantile(sorted: &[f64], p: f64) -> f64 {
    let n = sorted.len();
    if n == 0 {
        return f64::NAN;
    }
    if n == 1 || p <= 0.0 {
        return sorted[0];
    }
    if p >= 1.0 {
        return sorted[n - 1];
    }
    let h = (n - 1) as f64 * p;
    let i = h.floor() as usize;
    let (lo, hi) = (sorted[i], sorted[(i + 1).min(n - 1)]);
    lo + (h - i as f64) * (hi - lo)
}

pub fn mean(v: &[f64]) -> f64 {
    v.iter().sum::<f64>() / v.len() as f64
}

/// Sample standard deviation (n − 1); zero for fewer than two values.
pub fn std_dev(v: &[f64]) -> f64 {
    if v.len() < 2 {
        return 0.0;
    }
    let m = mean(v);
    let ss: f64 = v.iter().map(|x| (x - m) * (x - m)).sum();
    (ss / (v.len() - 1) as f64).sqrt()
}

/// Tukey's five-number summary with 1.5 × IQR fences: whiskers end at the
/// most extreme values inside the fences; the rest are outliers.
#[derive(Clone, Debug, PartialEq)]
pub struct BoxStats {
    pub q1: f64,
    pub median: f64,
    pub q3: f64,
    pub low: f64,
    pub high: f64,
    pub outliers: Vec<f64>,
}

pub fn box_stats(sorted: &[f64]) -> BoxStats {
    let (q1, median, q3) = (
        quantile(sorted, 0.25),
        quantile(sorted, 0.5),
        quantile(sorted, 0.75),
    );
    let reach = 1.5 * (q3 - q1);
    let (lo_fence, hi_fence) = (q1 - reach, q3 + reach);
    let inside = || {
        sorted
            .iter()
            .copied()
            .filter(|&v| v >= lo_fence && v <= hi_fence)
    };
    BoxStats {
        q1,
        median,
        q3,
        low: inside().fold(f64::INFINITY, f64::min),
        high: inside().fold(f64::NEG_INFINITY, f64::max),
        outliers: sorted
            .iter()
            .copied()
            .filter(|&v| v < lo_fence || v > hi_fence)
            .collect(),
    }
}

/// Silverman's rule-of-thumb bandwidth, 0.9 · min(σ, IQR / 1.34) · n^(−1/5).
/// Falls back to σ when the IQR is zero, and to 1 when every value is the
/// same, so a degenerate sample still draws a bump instead of dividing by 0.
pub fn silverman(sorted: &[f64]) -> f64 {
    let sd = std_dev(sorted);
    let iqr = (quantile(sorted, 0.75) - quantile(sorted, 0.25)) / 1.34;
    let spread = if iqr > 0.0 { sd.min(iqr) } else { sd };
    let h = 0.9 * spread * crate::dmath::powf(sorted.len() as f64, -0.2);
    if h > 0.0 { h } else { 1.0 }
}

/// 1 / √(2π). The Go port spells out the same literal.
const INV_SQRT_2PI: f64 = 0.398_942_280_401_432_7;

/// Gaussian kernel density of `sorted` at each of `xs`, bandwidth `h`.
pub fn kde(sorted: &[f64], h: f64, xs: &[f64]) -> Vec<f64> {
    let scale = INV_SQRT_2PI / (sorted.len() as f64 * h);
    xs.iter()
        .map(|&x| {
            let sum: f64 = sorted
                .iter()
                .map(|&v| {
                    let u = (x - v) / h;
                    crate::dmath::exp(-0.5 * u * u)
                })
                .sum();
            sum * scale
        })
        .collect()
}

/// `n` evenly spaced points from `lo` to `hi` inclusive.
pub fn grid(lo: f64, hi: f64, n: usize) -> Vec<f64> {
    if n < 2 {
        return vec![lo];
    }
    (0..n)
        .map(|i| lo + (hi - lo) * i as f64 / (n - 1) as f64)
        .collect()
}

/// Sturges' bin count, ⌈log₂ n⌉ + 1, computed on integers so both ports
/// agree exactly.
pub fn sturges(n: usize) -> usize {
    if n <= 1 {
        return 1;
    }
    (usize::BITS - (n - 1).leading_zeros()) as usize + 1
}

/// Histogram bins on round edges: about `count` bins whose edges are the
/// ticks of the niced extent (d3's `bin` with `nice`). The last bin
/// includes its upper edge so the maximum is counted. Identical values get
/// one bin one unit wide.
pub fn bins(sorted: &[f64], count: usize) -> (Vec<f64>, Vec<usize>) {
    let (Some(&lo), Some(&hi)) = (sorted.first(), sorted.last()) else {
        return (Vec::new(), Vec::new());
    };
    let edges = if lo == hi {
        vec![lo, lo + 1.0]
    } else {
        let (a, b) = ticks::nice(lo, hi, count);
        ticks::ticks(a, b, count)
    };
    let mut counts = vec![0; edges.len() - 1];
    let last = counts.len() - 1;
    let mut i = 0;
    for &v in sorted {
        while i < last && v >= edges[i + 1] {
            i += 1;
        }
        counts[i] += 1;
    }
    (edges, counts)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn quantiles_interpolate_between_order_statistics() {
        let v = [1.0, 2.0, 3.0, 4.0];
        assert_eq!(quantile(&v, 0.5), 2.5);
        assert_eq!(quantile(&v, 0.25), 1.75);
        assert_eq!(quantile(&v, 0.0), 1.0);
        assert_eq!(quantile(&v, 1.0), 4.0);
    }

    #[test]
    fn whiskers_stop_at_the_last_value_inside_the_fences() {
        // Q1 = 3, Q3 = 7, IQR 4: the upper fence is 13, so 40 is an outlier
        // and the whisker ends at 8, the last value inside, not at the fence.
        let v = [1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 40.0];
        let b = box_stats(&v);
        assert_eq!((b.q1, b.median, b.q3), (3.0, 5.0, 7.0));
        assert_eq!((b.low, b.high), (1.0, 8.0));
        assert_eq!(b.outliers, vec![40.0]);
    }

    #[test]
    fn kde_integrates_to_one() {
        // A density is a probability: the area under it must be ~1, or the
        // curves of two samples of different size are not comparable.
        let v = [1.0, 2.0, 2.5, 3.0, 7.0, 8.0, 8.5];
        let h = silverman(&v);
        let xs = grid(v[0] - 4.0 * h, v[6] + 4.0 * h, 2001);
        let ys = kde(&v, h, &xs);
        let dx = xs[1] - xs[0];
        let area: f64 = ys.windows(2).map(|w| (w[0] + w[1]) / 2.0 * dx).sum();
        assert!((area - 1.0).abs() < 1e-3, "area {area}");
    }

    #[test]
    fn degenerate_samples_still_get_a_bandwidth() {
        assert_eq!(silverman(&[5.0, 5.0, 5.0]), 1.0);
        assert!(silverman(&[1.0, 1.0, 1.0, 9.0]) > 0.0);
    }

    #[test]
    fn bins_count_every_value_once_on_round_edges() {
        let v: Vec<f64> = (0..37).map(|i| (i * i % 53) as f64 + 0.5).collect();
        let mut v = v;
        v.sort_by(f64::total_cmp);
        let (edges, counts) = bins(&v, sturges(v.len()));
        assert_eq!(counts.iter().sum::<usize>(), v.len());
        assert!(edges[0] <= v[0] && *edges.last().unwrap() >= v[v.len() - 1]);
        let step = edges[1] - edges[0];
        assert!([1.0, 2.0, 5.0, 10.0, 20.0].contains(&step), "step {step}");
        // The maximum lands in the last bin even when it sits on the edge.
        let (_, c) = bins(&[0.0, 5.0, 10.0], 2);
        assert_eq!(c, vec![1, 2]);
    }

    #[test]
    fn sturges_matches_the_formula() {
        assert_eq!(sturges(1), 1);
        assert_eq!(sturges(2), 2);
        assert_eq!(sturges(8), 4);
        assert_eq!(sturges(9), 5);
        assert_eq!(sturges(100), 8);
    }
}
