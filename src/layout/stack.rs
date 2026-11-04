//! The stream graph's stack: d3-shape's `stackOrderInsideOut` and
//! `stackOffsetWiggle` (Byron & Wattenberg 2008; ISC license, Mike Bostock).

/// Layer order with the earliest-peaking series in the middle and later
/// ones alternating outward, balancing the total weight on each side. The
/// result lists series bottom to top.
pub fn inside_out(series: &[Vec<f64>]) -> Vec<usize> {
    let peak = |v: &Vec<f64>| {
        let mut best = 0;
        for (j, &x) in v.iter().enumerate() {
            if x > v[best] {
                best = j;
            }
        }
        best
    };
    let mut by_peak: Vec<usize> = (0..series.len()).collect();
    by_peak.sort_by_key(|&i| peak(&series[i]));
    let (mut top, mut bottom) = (0.0, 0.0);
    let (mut tops, mut bottoms) = (Vec::new(), Vec::new());
    for i in by_peak {
        let sum: f64 = series[i].iter().sum();
        if top < bottom {
            top += sum;
            tops.push(i);
        } else {
            bottom += sum;
            bottoms.push(i);
        }
    }
    bottoms.reverse();
    bottoms.extend(tops);
    bottoms
}

/// Lower and upper edge of each series at each x, stacked in `order` on a
/// baseline that minimizes the layers' weighted wiggle (slope). Indexed by
/// series, then x.
pub fn wiggle(series: &[Vec<f64>], order: &[usize]) -> Vec<Vec<(f64, f64)>> {
    let m = series.first().map_or(0, Vec::len);
    let mut base = vec![0.0; m];
    let mut y = 0.0;
    for j in 1..m {
        let (mut s1, mut s2) = (0.0, 0.0);
        for (i, &si) in order.iter().enumerate() {
            let v = series[si][j];
            let mut s3 = (v - series[si][j - 1]) / 2.0;
            for &sk in &order[..i] {
                s3 += series[sk][j] - series[sk][j - 1];
            }
            s1 += v;
            s2 += s3 * v;
        }
        if s1 != 0.0 {
            y -= s2 / s1;
        }
        base[j] = y;
    }
    let mut out = vec![Vec::with_capacity(m); series.len()];
    for (j, &b) in base.iter().enumerate() {
        let mut lo = b;
        for &si in order {
            let hi = lo + series[si][j];
            out[si].push((lo, hi));
            lo = hi;
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn layers_touch_and_keep_their_thickness() {
        let s = vec![
            vec![1.0, 2.0, 3.0],
            vec![4.0, 1.0, 2.0],
            vec![2.0, 2.0, 5.0],
        ];
        let order = inside_out(&s);
        let e = wiggle(&s, &order);
        for j in 0..3 {
            for w in order.windows(2) {
                assert_eq!(e[w[0]][j].1, e[w[1]][j].0, "gap between layers at {j}");
            }
            for (i, v) in s.iter().enumerate() {
                assert!((e[i][j].1 - e[i][j].0 - v[j]).abs() < 1e-12);
            }
        }
    }

    #[test]
    fn a_growing_layer_does_not_tilt_the_whole_stream() {
        // One steady series and one that grows: wiggle keeps the steady one
        // flatter than a zero baseline would (where it rides up the growth).
        let s = vec![vec![5.0, 5.0, 5.0, 5.0], vec![1.0, 3.0, 6.0, 10.0]];
        let e = wiggle(&s, &[1, 0]);
        let mid = |j: usize| (e[0][j].0 + e[0][j].1) / 2.0;
        // On a zero baseline its midline would climb from 3.5 to 12.5.
        assert!((mid(3) - mid(0)).abs() < 9.0, "drift {}", mid(3) - mid(0));
    }

    #[test]
    fn inside_out_puts_early_peaks_in_the_middle() {
        let s = vec![
            vec![0.0, 0.0, 9.0],
            vec![9.0, 0.0, 0.0],
            vec![0.0, 9.0, 0.0],
        ];
        // Peaks: series 1 first, then 2, then 0. Series 1 goes to the
        // bottom list first, 2 on top of it, 0 below.
        assert_eq!(inside_out(&s), vec![0, 1, 2]);
    }
}
