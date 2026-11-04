//! Monotone cubic interpolation through points sorted by x, ported from
//! d3-shape `curveMonotoneX` (Steffen 1990; ISC license, Mike Bostock). The
//! curve never overshoots the data: between two points it stays within
//! their y range, so a smoothed series never shows a peak that is not there.

use crate::scene::PathOp;

/// Same sign rule as d3: zero counts as positive.
fn sign(x: f64) -> f64 {
    if x < 0.0 { -1.0 } else { 1.0 }
}

/// A segment's width as a divisor. A zero-width segment divides by a zero
/// signed like its neighbor, as in d3, so its slope is an infinity pointing
/// the right way instead of NaN.
fn divisor(h: f64, neighbor: f64) -> f64 {
    if h != 0.0 {
        h
    } else if neighbor < 0.0 {
        -0.0
    } else {
        0.0
    }
}

/// Tangent at the middle of three points.
fn slope3(p0: (f64, f64), p1: (f64, f64), p2: (f64, f64)) -> f64 {
    let (h0, h1) = (p1.0 - p0.0, p2.0 - p1.0);
    let s0 = (p1.1 - p0.1) / divisor(h0, h1);
    let s1 = (p2.1 - p1.1) / divisor(h1, h0);
    let p = (s0 * h1 + s1 * h0) / (h0 + h1);
    let t = (sign(s0) + sign(s1)) * s0.abs().min(s1.abs()).min(0.5 * p.abs());
    if t.is_nan() { 0.0 } else { t }
}

/// Tangent at an end point, from the segment and the neighbor's tangent.
fn slope2(p0: (f64, f64), p1: (f64, f64), t: f64) -> f64 {
    let h = p1.0 - p0.0;
    if h != 0.0 {
        (3.0 * (p1.1 - p0.1) / h - t) / 2.0
    } else {
        t
    }
}

/// Path commands from a move to the first point through the rest. With
/// `continue_path`, the first point is a line instead of a move, so a
/// curve can extend an outline already in progress.
pub fn monotone(points: &[(f64, f64)], continue_path: bool) -> Vec<PathOp> {
    let mut ops = Vec::with_capacity(points.len() + 1);
    let Some(&(x0, y0)) = points.first() else {
        return ops;
    };
    ops.push(if continue_path {
        PathOp::Line(x0, y0)
    } else {
        PathOp::Move(x0, y0)
    });
    let n = points.len();
    if n == 2 {
        ops.push(PathOp::Line(points[1].0, points[1].1));
    }
    if n <= 2 {
        return ops;
    }
    let mut t = vec![0.0; n];
    for i in 1..n - 1 {
        t[i] = slope3(points[i - 1], points[i], points[i + 1]);
    }
    t[0] = slope2(points[0], points[1], t[1]);
    t[n - 1] = slope2(points[n - 2], points[n - 1], t[n - 2]);
    for i in 0..n - 1 {
        let ((xa, ya), (xb, yb)) = (points[i], points[i + 1]);
        let dx = (xb - xa) / 3.0;
        ops.push(PathOp::Cubic(
            xa + dx,
            ya + dx * t[i],
            xb - dx,
            yb - dx * t[i + 1],
            xb,
            yb,
        ));
    }
    ops
}

type Pt = (f64, f64);

/// The cubic `p` between `t0` and `t1` (de Casteljau, twice).
fn sub_cubic(p: [Pt; 4], t0: f64, t1: f64) -> [Pt; 4] {
    let lerp = |a: Pt, b: Pt, t: f64| (a.0 + (b.0 - a.0) * t, a.1 + (b.1 - a.1) * t);
    // The part before t1: its points are the left edges of the triangle.
    let (a, b, c) = (
        lerp(p[0], p[1], t1),
        lerp(p[1], p[2], t1),
        lerp(p[2], p[3], t1),
    );
    let (d, e) = (lerp(a, b, t1), lerp(b, c, t1));
    let left = [p[0], a, d, lerp(d, e, t1)];
    if t0 <= 0.0 {
        return left;
    }
    // Of that, the part after t0, rescaled to it.
    let t = t0 / t1;
    let (a, b, c) = (
        lerp(left[0], left[1], t),
        lerp(left[1], left[2], t),
        lerp(left[2], left[3], t),
    );
    let (d, e) = (lerp(a, b, t), lerp(b, c, t));
    [lerp(d, e, t), e, c, left[3]]
}

/// The part of an x-monotone path (a move, then lines and cubics whose
/// controls sit at thirds of their x span, as [`monotone`] makes) between
/// `x0` and `x1`. Along such a cubic x moves linearly with t, so a cut at
/// an x is a split at a known t: the curve keeps its exact shape. With
/// `continue_path`, the path starts with a line instead of a move; the
/// input's first point may be either.
pub fn clip_x(ops: &[PathOp], x0: f64, x1: f64, continue_path: bool) -> Vec<PathOp> {
    let mut out = Vec::new();
    let Some((PathOp::Move(x, y) | PathOp::Line(x, y), rest)) = ops.split_first() else {
        return out;
    };
    let mut at = (*x, *y);
    for op in rest {
        let (seg, end) = match *op {
            PathOp::Move(x, y) => {
                at = (x, y);
                continue;
            }
            // A line as a cubic with its controls at thirds, so x moves
            // linearly with t along it too.
            PathOp::Line(x, y) => {
                let third = |k: f64| (at.0 + (x - at.0) * k / 3.0, at.1 + (y - at.1) * k / 3.0);
                ([at, third(1.0), third(2.0), (x, y)], (x, y))
            }
            PathOp::Cubic(a, b, c, d, x, y) => ([at, (a, b), (c, d), (x, y)], (x, y)),
            PathOp::Close => continue,
        };
        let (xa, xb) = (seg[0].0, seg[3].0);
        let (ta, tb) = if xb == xa {
            if xa >= x0 && xa <= x1 {
                (0.0, 1.0)
            } else {
                (1.0, 0.0)
            }
        } else {
            let (u, v) = ((x0 - xa) / (xb - xa), (x1 - xa) / (xb - xa));
            (u.min(v).max(0.0), u.max(v).min(1.0))
        };
        at = end;
        if ta >= tb {
            continue;
        }
        let part = if ta == 0.0 && tb == 1.0 {
            seg
        } else {
            sub_cubic(seg, ta, tb)
        };
        if out.is_empty() {
            let (x, y) = part[0];
            out.push(if continue_path {
                PathOp::Line(x, y)
            } else {
                PathOp::Move(x, y)
            });
        }
        out.push(match *op {
            PathOp::Line(..) => PathOp::Line(part[3].0, part[3].1),
            _ => PathOp::Cubic(
                part[1].0, part[1].1, part[2].0, part[2].1, part[3].0, part[3].1,
            ),
        });
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::scene::flatten;

    #[test]
    fn the_curve_passes_through_every_point() {
        let pts = [(0.0, 10.0), (10.0, 30.0), (20.0, 20.0), (35.0, 25.0)];
        let ops = monotone(&pts, false);
        assert_eq!(ops[0], PathOp::Move(0.0, 10.0));
        let ends: Vec<(f64, f64)> = ops[1..]
            .iter()
            .map(|o| match *o {
                PathOp::Cubic(.., x, y) => (x, y),
                _ => panic!("expected cubics"),
            })
            .collect();
        assert_eq!(ends, pts[1..]);
    }

    #[test]
    fn it_never_overshoots_between_points() {
        // A step: a naive spline would dip below 0 and rise above 10.
        let pts = [(0.0, 0.0), (10.0, 0.0), (20.0, 10.0), (30.0, 10.0)];
        let flat = flatten(&monotone(&pts, false));
        for &(_, y) in &flat[0] {
            assert!((-1e-9..=10.0 + 1e-9).contains(&y), "overshoot to {y}");
        }
    }

    #[test]
    fn two_points_are_a_straight_line() {
        let ops = monotone(&[(0.0, 0.0), (5.0, 5.0)], true);
        assert_eq!(ops, vec![PathOp::Line(0.0, 0.0), PathOp::Line(5.0, 5.0)]);
    }

    #[test]
    fn clipping_cuts_at_the_edges_and_keeps_the_shape() {
        let pts = [(0.0, 10.0), (10.0, 30.0), (20.0, 20.0), (35.0, 25.0)];
        let ops = monotone(&pts, false);
        let cut = clip_x(&ops, 4.0, 27.0, false);
        let flat = flatten(&cut);
        let (first, last) = (flat[0][0], *flat[0].last().unwrap());
        assert!((first.0 - 4.0).abs() < 1e-9 && (last.0 - 27.0).abs() < 1e-9);
        // Every kept point lies on the uncut curve: the cut pieces are the
        // same curve, not a new one through the edge points.
        let whole = flatten(&ops);
        let y_at = |x: f64| {
            let w = whole[0]
                .windows(2)
                .find(|w| w[0].0 <= x && x <= w[1].0)
                .unwrap();
            w[0].1 + (w[1].1 - w[0].1) * (x - w[0].0) / (w[1].0 - w[0].0)
        };
        for &(x, y) in &flat[0] {
            assert!((y - y_at(x)).abs() < 0.05, "({x}, {y}) is off the curve");
        }
        // Inside the edges, nothing is cut; outside, nothing is left.
        assert_eq!(clip_x(&ops, -5.0, 40.0, false), ops);
        assert!(clip_x(&ops, 50.0, 60.0, false).is_empty());
        // A straight piece is cut where it crosses the edge.
        let line = clip_x(
            &[PathOp::Move(0.0, 0.0), PathOp::Line(10.0, 20.0)],
            2.5,
            5.0,
            false,
        );
        assert_eq!(line, [PathOp::Move(2.5, 5.0), PathOp::Line(5.0, 10.0)]);
        // Backward (a band's lower edge) works the same way.
        let mut back = pts;
        back.reverse();
        let cut = clip_x(&monotone(&back, true), 4.0, 27.0, true);
        assert!(matches!(cut[0], PathOp::Line(x, _) if (x - 27.0).abs() < 1e-9));
    }
}
