//! Geometry shared by charts laid out around a center. Angles are radians
//! running clockwise from twelve o'clock, the way people read a clock face
//! and the way pie charts are conventionally drawn.

use super::frame::text;
use super::{GAP, TICK_SIZE};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::base::ticks;
use crate::dmath;
use crate::scene::{Anchor, Baseline, Corners, Mark, PathOp, Rect, Shape};
use std::f64::consts::{FRAC_PI_2, TAU};

const HALO: f64 = 1.0;
/// Labels around a circle snap to an edge-aligned anchor once they are this
/// far (as the sine/cosine of their angle) from the vertical/horizontal.
const SIDE: f64 = 0.25;

pub(crate) fn at(cx: f64, cy: f64, r: f64, a: f64) -> (f64, f64) {
    let (s, c) = dmath::sin_cos(a);
    (cx + r * s, cy - r * c)
}

/// Appends cubic Béziers tracing the circle of radius `r` from angle `a0`
/// to `a1` (either direction), starting from the current point. Each piece
/// spans at most a quarter turn, where the cubic stays within 0.03% of the
/// true arc.
pub(crate) fn arc(ops: &mut Vec<PathOp>, cx: f64, cy: f64, r: f64, a0: f64, a1: f64) {
    let n = ((a1 - a0).abs() / FRAC_PI_2).ceil().max(1.0) as usize;
    let step = (a1 - a0) / n as f64;
    let k = 4.0 / 3.0 * dmath::tan(step / 4.0) * r;
    for i in 0..n {
        let (s, e) = (a0 + step * i as f64, a0 + step * (i + 1) as f64);
        let (x0, y0) = at(cx, cy, r, s);
        let (x3, y3) = at(cx, cy, r, e);
        let ((ss, sc), (es, ec)) = (dmath::sin_cos(s), dmath::sin_cos(e));
        // The tangent at angle a (clockwise from twelve) is (cos a, sin a).
        ops.push(PathOp::Cubic(
            x0 + k * sc,
            y0 + k * ss,
            x3 - k * ec,
            y3 - k * es,
            x3,
            y3,
        ));
    }
}

/// The ring segment between radii `r0` < `r1` and angles `a0` < `a1`; with
/// `r0` zero, a pie slice.
pub(crate) fn sector(cx: f64, cy: f64, r0: f64, r1: f64, a0: f64, a1: f64) -> Vec<PathOp> {
    let (x, y) = at(cx, cy, r1, a0);
    let mut ops = vec![PathOp::Move(x, y)];
    arc(&mut ops, cx, cy, r1, a0, a1);
    if r0 > 0.0 {
        let (x, y) = at(cx, cy, r0, a1);
        ops.push(PathOp::Line(x, y));
        arc(&mut ops, cx, cy, r0, a1, a0);
    } else {
        ops.push(PathOp::Line(cx, cy));
    }
    ops.push(PathOp::Close);
    ops
}

pub(crate) fn circle(cx: f64, cy: f64, r: f64) -> Vec<PathOp> {
    let mut ops = vec![PathOp::Move(cx, cy - r)];
    arc(&mut ops, cx, cy, r, 0.0, TAU);
    ops.push(PathOp::Close);
    ops
}

/// Where a label for angle `a` goes just outside radius `r`: anchored on
/// the side facing away from the center, so it never runs back over the
/// chart.
pub(crate) fn around(cx: f64, cy: f64, r: f64, a: f64) -> (f64, f64, Anchor, Baseline) {
    let (x, y) = at(cx, cy, r, a);
    let (s, c) = dmath::sin_cos(a);
    let anchor = if s > SIDE {
        Anchor::Start
    } else if s < -SIDE {
        Anchor::End
    } else {
        Anchor::Middle
    };
    let baseline = if c > SIDE {
        Baseline::Bottom
    } else if c < -SIDE {
        Baseline::Top
    } else {
        Baseline::Middle
    };
    (x, y, anchor, baseline)
}

/// Room labels placed with [`around`] need beyond the radius: horizontally
/// the widest label, vertically one line.
pub(crate) fn label_room(measure: &dyn TextMeasure, labels: &[String]) -> (f64, f64) {
    let w = labels
        .iter()
        .map(|l| measure.width(l, TICK_SIZE, Weight::Regular))
        .fold(0.0, f64::max);
    (w + GAP, TICK_SIZE + GAP)
}

/// The radial value scale's top (a round number at or above `max`) and its
/// ticks, zero excluded: the center already reads as zero.
pub(crate) fn radial_ticks(max: f64) -> (f64, Vec<f64>) {
    let top = if max > 0.0 {
        ticks::nice(0.0, max, 4).1
    } else {
        1.0
    };
    let t = ticks::ticks(0.0, top, 4)
        .into_iter()
        .filter(|&t| t > 0.0)
        .collect();
    (top, t)
}

/// A value label over the plot, on a surface-colored backing so the data
/// drawn beneath cannot swallow it. `anchor` is Start or Middle.
pub(crate) fn tick_label(
    marks: &mut Vec<Mark>,
    measure: &dyn TextMeasure,
    (x, y): (f64, f64),
    anchor: Anchor,
    v: f64,
) {
    let label = format::value(v);
    let w = measure.width(&label, TICK_SIZE, Weight::Regular);
    let left = if anchor == Anchor::Middle {
        x - w / 2.0
    } else {
        x
    };
    marks.push(
        Mark::new(Shape::Rect {
            rect: Rect {
                x: left - HALO,
                y: y - TICK_SIZE / 2.0 - HALO,
                w: w + 2.0 * HALO,
                h: TICK_SIZE + 2.0 * HALO,
            },
            radius: Corners::all(2.0),
        })
        .fill(Color::Surface),
    );
    marks.push(text(
        x,
        y,
        &label,
        TICK_SIZE,
        Weight::Regular,
        anchor,
        Baseline::Middle,
        Color::TextMuted,
    ));
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::scene::flatten;

    #[test]
    fn arcs_stay_on_the_circle() {
        // A cubic per quarter turn deviates from the true circle by under
        // 0.03% of the radius; anything worse shows as lumpy pies.
        let ops = circle(0.0, 0.0, 100.0);
        for &(x, y) in &flatten(&ops)[0] {
            let r = (x * x + y * y).sqrt();
            assert!((r - 100.0).abs() < 0.03, "{r}");
        }
    }

    #[test]
    fn a_sector_covers_its_wedge_and_nothing_else() {
        use crate::scene::{Hit, Scene};
        // Quarter ring from three to six o'clock, radii 50..100.
        let s = Scene {
            width: 1.0,
            height: 1.0,
            label: String::new(),
            marks: vec![
                Mark::new(Shape::Path {
                    ops: sector(0.0, 0.0, 50.0, 100.0, FRAC_PI_2, 2.0 * FRAC_PI_2),
                })
                .fill(Color::Series(0))
                .hit(Hit::new("0", "wedge", "")),
            ],
        };
        let inside = |x: f64, y: f64| s.hit_test(x, y, &[]).is_some();
        assert!(inside(53.0, 53.0), "middle of the wedge");
        assert!(!inside(20.0, 20.0), "the hole");
        assert!(!inside(-53.0, 53.0), "the next quarter");
        assert!(!inside(53.0, -53.0), "the quarter before");
    }

    #[test]
    fn labels_face_away_from_the_center() {
        assert!(matches!(
            around(0.0, 0.0, 10.0, FRAC_PI_2),
            (_, _, Anchor::Start, Baseline::Middle)
        ));
        assert!(matches!(
            around(0.0, 0.0, 10.0, 0.0),
            (_, _, Anchor::Middle, Baseline::Bottom)
        ));
        assert!(matches!(
            around(0.0, 0.0, 10.0, 1.5 * std::f64::consts::PI),
            (_, _, Anchor::End, Baseline::Middle)
        ));
    }
}
