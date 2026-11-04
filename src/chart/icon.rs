//! Pictogram icons: built-in shapes or SVG path data, placed into a box and
//! cut at a vertical line for the fractional last icon of a row.

use crate::error::Error;
use crate::scene::{PathOp, flatten};

/// 4/3·(√2 − 1): cubic control distance for a quarter circle.
const KAPPA: f64 = 0.552_284_749_830_793_6;
/// Built-in icons are drawn in a 24×24 box, the common icon-set grid.
const BUILTIN_BOX: f64 = 24.0;

/// What each unit of a pictogram looks like.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
pub enum Icon {
    #[default]
    Person,
    Circle,
    Square,
    /// SVG path data (`M L H V C Q Z`, absolute or relative) drawn in a
    /// `size`×`size` box, e.g. 24 for most icon sets.
    Path {
        d: String,
        size: f64,
    },
}

impl Icon {
    /// The icon as path ops filling the square at (`x`, `y`) with side `s`.
    pub(crate) fn place(&self, x: f64, y: f64, s: f64) -> Result<Vec<PathOp>, Error> {
        let (ops, size) = match self {
            Icon::Person => (person(), BUILTIN_BOX),
            Icon::Circle => (circle(12.0, 12.0, 10.0), BUILTIN_BOX),
            Icon::Square => (
                vec![
                    PathOp::Move(2.0, 2.0),
                    PathOp::Line(22.0, 2.0),
                    PathOp::Line(22.0, 22.0),
                    PathOp::Line(2.0, 22.0),
                    PathOp::Close,
                ],
                BUILTIN_BOX,
            ),
            Icon::Path { d, size } => {
                if !(size.is_finite() && *size > 0.0) {
                    return Err(Error::InvalidData(
                        "an icon path's size must be a positive number".into(),
                    ));
                }
                (parse_path(d)?, *size)
            }
        };
        let k = s / size;
        let at = |px: f64, py: f64| (x + px * k, y + py * k);
        Ok(ops
            .into_iter()
            .map(|op| match op {
                PathOp::Move(px, py) => {
                    let (a, b) = at(px, py);
                    PathOp::Move(a, b)
                }
                PathOp::Line(px, py) => {
                    let (a, b) = at(px, py);
                    PathOp::Line(a, b)
                }
                PathOp::Cubic(x1, y1, x2, y2, px, py) => {
                    let (a1, b1) = at(x1, y1);
                    let (a2, b2) = at(x2, y2);
                    let (a, b) = at(px, py);
                    PathOp::Cubic(a1, b1, a2, b2, a, b)
                }
                PathOp::Close => PathOp::Close,
            })
            .collect())
    }
}

/// A circle as four cubics, clockwise from the top.
pub(crate) fn circle(cx: f64, cy: f64, r: f64) -> Vec<PathOp> {
    let k = KAPPA * r;
    vec![
        PathOp::Move(cx, cy - r),
        PathOp::Cubic(cx + k, cy - r, cx + r, cy - k, cx + r, cy),
        PathOp::Cubic(cx + r, cy + k, cx + k, cy + r, cx, cy + r),
        PathOp::Cubic(cx - k, cy + r, cx - r, cy + k, cx - r, cy),
        PathOp::Cubic(cx - r, cy - k, cx - k, cy - r, cx, cy - r),
        PathOp::Close,
    ]
}

/// Head and rounded shoulders, in the 24 box.
fn person() -> Vec<PathOp> {
    let mut ops = circle(12.0, 6.5, 4.5);
    ops.extend([
        PathOp::Move(4.0, 22.0),
        PathOp::Line(4.0, 17.5),
        PathOp::Cubic(4.0, 14.5, 7.5, 12.5, 12.0, 12.5),
        PathOp::Cubic(16.5, 12.5, 20.0, 14.5, 20.0, 17.5),
        PathOp::Line(20.0, 22.0),
        PathOp::Close,
    ]);
    ops
}

/// The subset of SVG path data icon sets use once arcs are converted:
/// moves, lines, cubics and quadratics, absolute or relative.
pub(crate) fn parse_path(d: &str) -> Result<Vec<PathOp>, Error> {
    let bad = |why: String| Error::InvalidData(format!("icon path: {why}"));
    let b = d.as_bytes();
    let mut i = 0;
    let skip = |i: &mut usize| {
        while *i < b.len() && (b[*i].is_ascii_whitespace() || b[*i] == b',') {
            *i += 1;
        }
    };
    let number = |i: &mut usize| -> Option<f64> {
        skip(i);
        let start = *i;
        if *i < b.len() && (b[*i] == b'+' || b[*i] == b'-') {
            *i += 1;
        }
        let mut dot = false;
        let mut digits = false;
        while *i < b.len() && (b[*i].is_ascii_digit() || (b[*i] == b'.' && !dot)) {
            dot |= b[*i] == b'.';
            digits |= b[*i].is_ascii_digit();
            *i += 1;
        }
        if digits && *i < b.len() && (b[*i] == b'e' || b[*i] == b'E') {
            let mark = *i;
            *i += 1;
            if *i < b.len() && (b[*i] == b'+' || b[*i] == b'-') {
                *i += 1;
            }
            let exp = *i;
            while *i < b.len() && b[*i].is_ascii_digit() {
                *i += 1;
            }
            if *i == exp {
                *i = mark;
            }
        }
        if !digits {
            *i = start;
            return None;
        }
        d[start..*i].parse().ok()
    };

    let mut ops = Vec::new();
    let (mut cx, mut cy) = (0.0, 0.0);
    let (mut sx, mut sy) = (0.0, 0.0);
    let mut cmd = 0u8;
    loop {
        skip(&mut i);
        if i >= b.len() {
            break;
        }
        if b[i].is_ascii_alphabetic() {
            cmd = b[i];
            i += 1;
        } else if cmd == 0 {
            return Err(bad("numbers without a command".into()));
        }
        if ops.is_empty() && !cmd.eq_ignore_ascii_case(&b'M') {
            return Err(bad("must start with M".into()));
        }
        let rel = cmd.is_ascii_lowercase();
        let (ox, oy) = if rel { (cx, cy) } else { (0.0, 0.0) };
        let nums = |n: usize, i: &mut usize| -> Result<Vec<f64>, Error> {
            (0..n)
                .map(|_| number(i).ok_or_else(|| bad(format!("{} needs {n} numbers", cmd as char))))
                .collect()
        };
        match cmd.to_ascii_uppercase() {
            b'M' => {
                let v = nums(2, &mut i)?;
                (cx, cy) = (ox + v[0], oy + v[1]);
                (sx, sy) = (cx, cy);
                ops.push(PathOp::Move(cx, cy));
                // Further pairs after a move are lines.
                cmd = if rel { b'l' } else { b'L' };
            }
            b'L' => {
                let v = nums(2, &mut i)?;
                (cx, cy) = (ox + v[0], oy + v[1]);
                ops.push(PathOp::Line(cx, cy));
            }
            b'H' => {
                let v = nums(1, &mut i)?;
                cx = ox + v[0];
                ops.push(PathOp::Line(cx, cy));
            }
            b'V' => {
                let v = nums(1, &mut i)?;
                cy = oy + v[0];
                ops.push(PathOp::Line(cx, cy));
            }
            b'C' => {
                let v = nums(6, &mut i)?;
                let c = (
                    ox + v[0],
                    oy + v[1],
                    ox + v[2],
                    oy + v[3],
                    ox + v[4],
                    oy + v[5],
                );
                (cx, cy) = (c.4, c.5);
                ops.push(PathOp::Cubic(c.0, c.1, c.2, c.3, c.4, c.5));
            }
            b'Q' => {
                // A quadratic is the cubic with controls two thirds of the
                // way from each end to its one control point.
                let v = nums(4, &mut i)?;
                let (qx, qy, x, y) = (ox + v[0], oy + v[1], ox + v[2], oy + v[3]);
                ops.push(PathOp::Cubic(
                    cx + 2.0 / 3.0 * (qx - cx),
                    cy + 2.0 / 3.0 * (qy - cy),
                    x + 2.0 / 3.0 * (qx - x),
                    y + 2.0 / 3.0 * (qy - y),
                    x,
                    y,
                ));
                (cx, cy) = (x, y);
            }
            b'Z' => {
                ops.push(PathOp::Close);
                (cx, cy) = (sx, sy);
                cmd = 0;
                skip(&mut i);
                if i < b.len() && !b[i].is_ascii_alphabetic() {
                    return Err(bad("numbers after Z".into()));
                }
            }
            other => {
                return Err(bad(format!(
                    "command {:?} is not supported; use M, L, H, V, C, Q and Z (convert arcs to curves)",
                    other as char
                )));
            }
        }
    }
    if ops.is_empty() {
        return Err(bad("empty".into()));
    }
    Ok(ops)
}

/// The part of the filled path left of `x`: every subpath flattened and
/// clipped to the half-plane. Clipping against one straight edge keeps any
/// polygon, concave or not, a single correct polygon, so nonzero filling of
/// the result matches the original up to the cut.
pub(crate) fn clip_left(ops: &[PathOp], x: f64) -> Vec<PathOp> {
    let mut out = Vec::new();
    for mut pts in flatten(ops) {
        if pts.len() > 1 && pts.first() == pts.last() {
            pts.pop();
        }
        let n = pts.len();
        let mut poly: Vec<(f64, f64)> = Vec::new();
        for k in 0..n {
            let (p, c) = (pts[(k + n - 1) % n], pts[k]);
            let cross = || {
                let t = (x - p.0) / (c.0 - p.0);
                (x, p.1 + t * (c.1 - p.1))
            };
            match (p.0 <= x, c.0 <= x) {
                (true, true) => poly.push(c),
                (true, false) => poly.push(cross()),
                (false, true) => {
                    poly.push(cross());
                    poly.push(c);
                }
                (false, false) => {}
            }
        }
        if poly.len() >= 3 {
            out.push(PathOp::Move(poly[0].0, poly[0].1));
            out.extend(poly[1..].iter().map(|&(a, b)| PathOp::Line(a, b)));
            out.push(PathOp::Close);
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn relative_and_absolute_commands_agree() {
        let abs = parse_path("M2 2 L10 2 H12 V8 C12 9 11 10 10 10 Q6 12 2 10 Z").unwrap();
        let rel = parse_path("m2 2 l8 0 h2 v6 c0 1-1 2-2 2 q-4 2-8 0z").unwrap();
        assert_eq!(abs, rel);
        // Numbers run together the way minified icon sets write them.
        assert_eq!(
            parse_path("M1.5.5L-2e1,3").unwrap(),
            vec![PathOp::Move(1.5, 0.5), PathOp::Line(-20.0, 3.0)]
        );
    }

    #[test]
    fn implicit_lines_follow_a_move_and_z_returns_to_the_start() {
        let ops = parse_path("m1 1 2 0 0 2z l1 1").unwrap();
        assert_eq!(
            ops,
            vec![
                PathOp::Move(1.0, 1.0),
                PathOp::Line(3.0, 1.0),
                PathOp::Line(3.0, 3.0),
                PathOp::Close,
                PathOp::Line(2.0, 2.0),
            ]
        );
    }

    #[test]
    fn unsupported_commands_say_what_to_do() {
        let err = parse_path("M0 0 A5 5 0 0 1 10 10").unwrap_err().to_string();
        assert!(err.contains("'A'") && err.contains("arcs"), "{err}");
        assert!(parse_path("L1 1").is_err());
        assert!(parse_path("M1").is_err());
    }

    #[test]
    fn a_clip_keeps_the_left_part_of_a_concave_shape() {
        // A U shape cut through both arms keeps both arms' left parts as
        // one polygon whose area is exactly the left part's.
        let u = vec![
            PathOp::Move(0.0, 0.0),
            PathOp::Line(2.0, 0.0),
            PathOp::Line(2.0, 4.0),
            PathOp::Line(4.0, 4.0),
            PathOp::Line(4.0, 0.0),
            PathOp::Line(6.0, 0.0),
            PathOp::Line(6.0, 6.0),
            PathOp::Line(0.0, 6.0),
            PathOp::Close,
        ];
        let area = |ops: &[PathOp]| -> f64 {
            flatten(ops)
                .iter()
                .map(|p| {
                    p.windows(2)
                        .map(|w| w[0].0 * w[1].1 - w[1].0 * w[0].1)
                        .sum::<f64>()
                        / 2.0
                })
                .sum::<f64>()
                .abs()
        };
        // Left of x = 5: the left arm (2×6), the bridge (2×2) and a 1×6
        // strip of the right arm.
        assert!((area(&clip_left(&u, 5.0)) - (12.0 + 4.0 + 6.0)).abs() < 1e-9);
        assert!(clip_left(&u, -1.0).is_empty());
        assert!((area(&clip_left(&u, 7.0)) - area(&u)).abs() < 1e-9);
    }
}
