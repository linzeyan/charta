use super::frame::{self, text};
use super::{Chart, GAP, PAD, TICK_SIZE, check_size, count, describe, finite};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::dmath;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Hit, Mark, Scene, Shape, TipRow};
use std::f64::consts::PI;

/// Past five sets the overlaps stop being readable as circles.
const MAX_SETS: usize = 5;
/// Disjoint sets sit this much further apart than touching, so they do not
/// look like they share a point.
const DISJOINT: f64 = 1.1;
const OUTLINE: f64 = 1.5;
/// Slack for a crossing point to count as inside a circle it lies on.
const ON_EDGE: f64 = 1e-10;
/// cos 45°. The Go port spells out the same literal, 0.7071067811865476.
const SQRT_HALF: f64 = std::f64::consts::FRAC_1_SQRT_2;
/// The axis and diagonal directions, for placing circles and labels.
const DIRS: [(f64, f64); 8] = [
    (1.0, 0.0),
    (0.0, 1.0),
    (-1.0, 0.0),
    (0.0, -1.0),
    (SQRT_HALF, SQRT_HALF),
    (-SQRT_HALF, SQRT_HALF),
    (-SQRT_HALF, -SQRT_HALF),
    (SQRT_HALF, -SQRT_HALF),
];

/// A set (one name) or the overlap of several, and its size.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct VennArea {
    pub sets: Vec<String>,
    pub size: f64,
}

/// Sets as circles, area proportional to size, placed so the overlaps
/// match the data. Pairs alone match exactly for two or three sets and as
/// near as possible for four or five; an overlap of three or more sets
/// given too brings every given area into the fit. A pair left out shares
/// nothing.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct VennChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub areas: Vec<VennArea>,
}

impl VennChart {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    /// One name for a set, several for their overlap.
    pub fn area<S: Into<String>>(mut self, sets: impl IntoIterator<Item = S>, size: f64) -> Self {
        self.areas.push(VennArea {
            sets: sets.into_iter().map(Into::into).collect(),
            size,
        });
        self
    }
}

/// How a pair's target distance constrains the layout.
#[derive(Clone, Copy, PartialEq)]
enum Pair {
    /// Exactly this far apart.
    Exact,
    /// At least this far: any further still has no overlap.
    AtLeast,
    /// At most this far: any closer still has the smaller one inside.
    AtMost,
}

/// The area two circles share at center distance `d`.
fn lens(r1: f64, r2: f64, d: f64) -> f64 {
    if d >= r1 + r2 {
        return 0.0;
    }
    if d <= (r1 - r2).abs() {
        let r = r1.min(r2);
        return PI * r * r;
    }
    let a1 = ((d * d + r1 * r1 - r2 * r2) / (2.0 * d * r1)).clamp(-1.0, 1.0);
    let a2 = ((d * d + r2 * r2 - r1 * r1) / (2.0 * d * r2)).clamp(-1.0, 1.0);
    let k = (-d + r1 + r2) * (d + r1 - r2) * (d - r1 + r2) * (d + r1 + r2);
    r1 * r1 * dmath::acos(a1) + r2 * r2 * dmath::acos(a2) - 0.5 * k.max(0.0).sqrt()
}

/// The distance at which two circles share `overlap`, and how strictly.
fn distance(r1: f64, r2: f64, overlap: f64) -> (f64, Pair) {
    let smaller = PI * r1.min(r2) * r1.min(r2);
    if overlap <= 0.0 {
        return (DISJOINT * (r1 + r2), Pair::AtLeast);
    }
    if overlap >= smaller {
        // Inside, centered halfway between concentric and touching.
        return ((r1 - r2).abs() / 2.0, Pair::AtMost);
    }
    // The shared area shrinks as the circles part: bisect.
    let (mut lo, mut hi) = ((r1 - r2).abs(), r1 + r2);
    for _ in 0..60 {
        let mid = (lo + hi) / 2.0;
        if lens(r1, r2, mid) > overlap {
            lo = mid;
        } else {
            hi = mid;
        }
    }
    ((lo + hi) / 2.0, Pair::Exact)
}

/// How far the layout is from every pair's target.
fn stress(p: &[(f64, f64)], target: &[Vec<(f64, Pair)>]) -> f64 {
    let mut s = 0.0;
    for i in 0..p.len() {
        for j in i + 1..p.len() {
            let (dx, dy) = (p[i].0 - p[j].0, p[i].1 - p[j].1);
            let d = (dx * dx + dy * dy).sqrt();
            let (t, kind) = target[i][j];
            let e = match kind {
                Pair::AtLeast if d >= t => 0.0,
                Pair::AtMost if d <= t => 0.0,
                _ => d - t,
            };
            s += e * e;
        }
    }
    s
}

/// Nelder–Mead with the standard coefficients and a fixed iteration budget:
/// no randomness and no early exit on timing, so both ports take the same
/// steps.
fn nelder_mead(f: impl Fn(&[f64]) -> f64, start: Vec<f64>, step: f64) -> Vec<f64> {
    let n = start.len();
    let mut simplex: Vec<(Vec<f64>, f64)> = (0..=n)
        .map(|k| {
            let mut x = start.clone();
            if k > 0 {
                x[k - 1] += step;
            }
            let v = f(&x);
            (x, v)
        })
        .collect();
    for _ in 0..200 * n {
        simplex.sort_by(|a, b| a.1.total_cmp(&b.1));
        if simplex[n].1 - simplex[0].1 < 1e-12 {
            break;
        }
        let centroid: Vec<f64> = (0..n)
            .map(|d| simplex[..n].iter().map(|s| s.0[d]).sum::<f64>() / n as f64)
            .collect();
        let toward = |t: f64| -> Vec<f64> {
            (0..n)
                .map(|d| centroid[d] + t * (simplex[n].0[d] - centroid[d]))
                .collect()
        };
        let r = toward(-1.0);
        let fr = f(&r);
        if fr < simplex[0].1 {
            let e = toward(-2.0);
            let fe = f(&e);
            simplex[n] = if fe < fr { (e, fe) } else { (r, fr) };
        } else if fr < simplex[n - 1].1 {
            simplex[n] = (r, fr);
        } else {
            let (c, fc) = if fr < simplex[n].1 {
                let c = toward(-0.5);
                let fc = f(&c);
                (c, fc)
            } else {
                let c = toward(0.5);
                let fc = f(&c);
                (c, fc)
            };
            if fc < fr.min(simplex[n].1) {
                simplex[n] = (c, fc);
            } else {
                // Shrink everything toward the best point.
                let best = simplex[0].0.clone();
                for s in simplex.iter_mut().skip(1) {
                    for (v, b) in s.0.iter_mut().zip(&best) {
                        *v = b + 0.5 * (*v - b);
                    }
                    s.1 = f(&s.0);
                }
            }
        }
    }
    simplex.sort_by(|a, b| a.1.total_cmp(&b.1));
    simplex.swap_remove(0).0
}

/// Circle centers meeting the pairwise targets.
fn layout(r: &[f64], target: &[Vec<(f64, Pair)>]) -> Vec<(f64, f64)> {
    let n = r.len();
    match n {
        1 => vec![(0.0, 0.0)],
        2 => vec![(0.0, 0.0), (target[0][1].0, 0.0)],
        3 => {
            // The triangle with the three target distances as sides, the
            // third set below the first two.
            let (a, b, c) = (target[0][1].0, target[0][2].0, target[1][2].0);
            let (x, y) = if a > 0.0 {
                let x = (a * a + b * b - c * c) / (2.0 * a);
                (x, (b * b - x * x).max(0.0).sqrt())
            } else {
                (b, 0.0)
            };
            vec![(0.0, 0.0), (a, 0.0), (x, y)]
        }
        _ => {
            // Greedy start: each set at the best of eight spots at its
            // target distance from each placed set, then refine together.
            let mut p = vec![(0.0, 0.0)];
            for i in 1..n {
                let mut best = (f64::INFINITY, (0.0, 0.0));
                for (j, &q) in p.clone().iter().enumerate() {
                    for &(ux, uy) in &DIRS {
                        let t = target[j][i].0;
                        let c = (q.0 + t * ux, q.1 + t * uy);
                        p.push(c);
                        let sub: Vec<Vec<(f64, Pair)>> =
                            target[..=i].iter().map(|row| row[..=i].to_vec()).collect();
                        let s = stress(&p, &sub);
                        p.pop();
                        if s < best.0 {
                            best = (s, c);
                        }
                    }
                }
                p.push(best.1);
            }
            let flat: Vec<f64> = p.iter().flat_map(|&(x, y)| [x, y]).collect();
            let mean_r = r.iter().sum::<f64>() / n as f64;
            let best = nelder_mead(
                |x| {
                    let q: Vec<(f64, f64)> = x.chunks(2).map(|c| (c[0], c[1])).collect();
                    stress(&q, target)
                },
                flat,
                mean_r / 10.0,
            );
            best.chunks(2).map(|c| (c[0], c[1])).collect()
        }
    }
}

/// A circle as center x, center y and radius.
type Circle = (f64, f64, f64);

fn apart(x: f64, y: f64, c: Circle) -> f64 {
    let (dx, dy) = (x - c.0, y - c.1);
    (dx * dx + dy * dy).sqrt()
}

/// The two points where the circles' edges cross, if they do.
fn crossings(a: Circle, b: Circle) -> Option<[(f64, f64); 2]> {
    let (dx, dy) = (b.0 - a.0, b.1 - a.1);
    let d = (dx * dx + dy * dy).sqrt();
    if d >= a.2 + b.2 || d <= (a.2 - b.2).abs() {
        return None;
    }
    let along = (a.2 * a.2 - b.2 * b.2 + d * d) / (2.0 * d);
    let h = (a.2 * a.2 - along * along).max(0.0).sqrt();
    let (x0, y0) = (a.0 + along * dx / d, a.1 + along * dy / d);
    let (rx, ry) = (-dy * (h / d), -dx * (h / d));
    Some([(x0 + rx, y0 - ry), (x0 - rx, y0 + ry)])
}

/// The area a chord cuts off a circle of radius `r`, `w` deep.
fn segment(r: f64, w: f64) -> f64 {
    r * r * dmath::acos(1.0 - w / r) - (r - w) * (w * (2.0 * r - w)).sqrt()
}

/// The area every circle covers, ported from venn.js `intersectionArea`
/// (MIT license, Ben Frederickson): the polygon through the crossing points
/// inside all circles, plus the circular segment beyond each of its edges.
fn common_area(c: &[Circle]) -> f64 {
    // Each corner with the two circles it lies on.
    let mut corners: Vec<(f64, f64, [usize; 2])> = Vec::new();
    for i in 0..c.len() {
        for j in i + 1..c.len() {
            for (x, y) in crossings(c[i], c[j]).into_iter().flatten() {
                if c.iter().all(|&k| apart(x, y, k) <= k.2 + ON_EDGE) {
                    corners.push((x, y, [i, j]));
                }
            }
        }
    }
    if corners.len() < 2 {
        // No region with corners: the smallest circle lies inside all the
        // others, or they have nothing in common.
        let s = c.iter().fold(c[0], |m, &k| if k.2 < m.2 { k } else { m });
        let inside = c.iter().all(|&k| apart(s.0, s.1, k) <= (s.2 - k.2).abs());
        return if inside { PI * s.2 * s.2 } else { 0.0 };
    }
    let m = corners.len() as f64;
    let mx = corners.iter().map(|p| p.0).sum::<f64>() / m;
    let my = corners.iter().map(|p| p.1).sum::<f64>() / m;
    // Around the centroid, so consecutive corners are the polygon's edges.
    let mut around: Vec<(f64, usize)> = corners
        .iter()
        .enumerate()
        .map(|(k, p)| (dmath::atan2(p.0 - mx, p.1 - my), k))
        .collect();
    around.sort_by(|a, b| b.0.total_cmp(&a.0));
    let (mut polygon, mut arcs) = (0.0, 0.0);
    let mut p2 = corners[around[around.len() - 1].1];
    for &(_, k) in &around {
        let p1 = corners[k];
        polygon += (p2.0 + p1.0) * (p1.1 - p2.1);
        let mid = ((p1.0 + p2.0) / 2.0, (p1.1 + p2.1) / 2.0);
        // Of the circles both corners lie on, the edge follows the one
        // whose arc between them bulges least.
        let mut arc: Option<(f64, f64)> = None;
        for &ci in p1.2.iter().filter(|ci| p2.2.contains(ci)) {
            let k = c[ci];
            let a1 = dmath::atan2(p1.0 - k.0, p1.1 - k.1);
            let a2 = dmath::atan2(p2.0 - k.0, p2.1 - k.1);
            let mut turn = a2 - a1;
            if turn < 0.0 {
                turn += 2.0 * PI;
            }
            let (sin, cos) = dmath::sin_cos(a2 - turn / 2.0);
            let w = apart(mid.0, mid.1, (k.0 + k.2 * sin, k.1 + k.2 * cos, 0.0)).min(2.0 * k.2);
            if arc.is_none_or(|(aw, _)| aw > w) {
                arc = Some((w, k.2));
            }
        }
        if let Some((w, r)) = arc {
            arcs += segment(r, w);
            p2 = p1;
        }
    }
    arcs + polygon / 2.0
}

/// Moves the circles so every given overlap comes as close as it can to
/// its size, starting from the pairwise layout. `given` is each overlap's
/// sets and size; pairs that share nothing keep their gap instead.
fn refine(
    at: Vec<(f64, f64)>,
    r: &[f64],
    target: &[Vec<(f64, Pair)>],
    given: &[(Vec<usize>, f64)],
) -> Vec<(f64, f64)> {
    let n = r.len();
    // Both errors as shares, of the mean set's area and radius, so neither
    // outweighs the other by units alone.
    let mean_r = r.iter().sum::<f64>() / n as f64;
    let mean_area = r.iter().map(|&r| PI * r * r).sum::<f64>() / n as f64;
    let loss = |x: &[f64]| -> f64 {
        let c: Vec<Circle> = (0..n).map(|i| (x[2 * i], x[2 * i + 1], r[i])).collect();
        let mut e = 0.0;
        for i in 0..n {
            for j in i + 1..n {
                let (t, kind) = target[i][j];
                if kind == Pair::AtLeast {
                    let gap = (t - apart(c[i].0, c[i].1, c[j])).max(0.0) / mean_r;
                    e += gap * gap;
                }
            }
        }
        for (ids, size) in given {
            let have = match *ids.as_slice() {
                [i, j] if target[i][j].1 == Pair::AtLeast => continue,
                [i, j] => lens(r[i], r[j], apart(c[i].0, c[i].1, c[j])),
                _ => common_area(&ids.iter().map(|&i| c[i]).collect::<Vec<_>>()),
            };
            let d = (have - size) / mean_area;
            e += d * d;
        }
        e
    };
    let flat: Vec<f64> = at.iter().flat_map(|&(x, y)| [x, y]).collect();
    nelder_mead(loss, flat, mean_r / 10.0)
        .chunks(2)
        .map(|c| (c[0], c[1]))
        .collect()
}

impl Chart for VennChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let names: Vec<&str> = self
            .areas
            .iter()
            .filter(|a| a.sets.len() == 1)
            .map(|a| a.sets[0].as_str())
            .collect();
        if names.is_empty() || names.len() > MAX_SETS {
            return Err(Error::InvalidData(format!(
                "{} sets: give 1 to {MAX_SETS}, each as an area with one name",
                names.len()
            )));
        }
        let index = |s: &str| names.iter().position(|n| *n == s);
        let mut size = vec![0.0; names.len()];
        let mut overlap = vec![vec![0.0; names.len()]; names.len()];
        let mut keys: Vec<Vec<usize>> = Vec::new();
        for a in &self.areas {
            let what = a.sets.join(" ∩ ");
            finite(&format!("the size of {what}"), a.size)?;
            let mut ids = Vec::with_capacity(a.sets.len());
            for s in &a.sets {
                let i = index(s).ok_or_else(|| {
                    Error::InvalidData(format!("{what} names {s:?}, which has no area of its own"))
                })?;
                if ids.contains(&i) {
                    return Err(Error::InvalidData(format!("{what} names {s:?} twice")));
                }
                ids.push(i);
            }
            ids.sort_unstable();
            if keys.contains(&ids) {
                return Err(Error::InvalidData(format!("{what} is given twice")));
            }
            keys.push(ids.clone());
            if ids.len() == 1 {
                if a.size <= 0.0 {
                    return Err(Error::InvalidData(format!(
                        "set {what} needs a positive size"
                    )));
                }
                size[ids[0]] = a.size;
            } else if a.size < 0.0 {
                return Err(Error::InvalidData(format!("{what} has a negative size")));
            } else if ids.len() == 2 {
                overlap[ids[0]][ids[1]] = a.size;
                overlap[ids[1]][ids[0]] = a.size;
            }
        }
        // An overlap cannot hold more than any set in it.
        for (a, ids) in self.areas.iter().zip(&keys) {
            if let Some(&i) = ids.iter().find(|&&i| a.size > size[i]) {
                return Err(Error::InvalidData(format!(
                    "{} ({}) is bigger than {:?} ({})",
                    a.sets.join(" ∩ "),
                    format::value(a.size),
                    names[i],
                    format::value(size[i])
                )));
            }
        }
        // Nor more than an overlap of fewer of its sets; and a pair left out
        // shares nothing, so neither can anything more that holds it.
        for (a, ids) in self.areas.iter().zip(&keys) {
            if ids.len() < 3 || a.size == 0.0 {
                continue;
            }
            let what = a.sets.join(" ∩ ");
            for (b, sub) in self.areas.iter().zip(&keys) {
                if sub.len() > 1
                    && sub.len() < ids.len()
                    && sub.iter().all(|i| ids.contains(i))
                    && a.size > b.size
                {
                    return Err(Error::InvalidData(format!(
                        "{what} ({}) is bigger than {} ({})",
                        format::value(a.size),
                        b.sets.join(" ∩ "),
                        format::value(b.size)
                    )));
                }
            }
            for (x, &i) in ids.iter().enumerate() {
                for &j in &ids[x + 1..] {
                    if !keys.contains(&vec![i, j]) {
                        return Err(Error::InvalidData(format!(
                            "{what} is {} but {} ∩ {} is not given, so they share nothing",
                            format::value(a.size),
                            names[i],
                            names[j]
                        )));
                    }
                }
            }
        }

        let r: Vec<f64> = size.iter().map(|&s| (s / PI).sqrt()).collect();
        let n = names.len();
        let target: Vec<Vec<(f64, Pair)>> = (0..n)
            .map(|i| {
                (0..n)
                    .map(|j| {
                        if i == j {
                            (0.0, Pair::Exact)
                        } else {
                            distance(r[i], r[j], overlap[i][j])
                        }
                    })
                    .collect()
            })
            .collect();
        let mut at = layout(&r, &target);
        if keys.iter().any(|ids| ids.len() > 2) {
            let given: Vec<(Vec<usize>, f64)> = keys
                .iter()
                .zip(&self.areas)
                .filter(|(ids, _)| ids.len() > 1)
                .map(|(ids, a)| (ids.clone(), a.size))
                .collect();
            at = refine(at, &r, &target, &given);
        }

        let mut marks = Vec::new();
        let top = frame::title(&mut marks, self.title.as_deref());
        // Room for labels outside the circles on every side.
        let label_w = names
            .iter()
            .map(|s| measure.width(s, TICK_SIZE, Weight::Semibold))
            .fold(0.0, f64::max);
        let (mx, my) = (label_w + GAP, TICK_SIZE + GAP);
        let (left, right, up, down) = (PAD + mx, width - PAD - mx, top + my, height - PAD - my);
        let (mut x0, mut y0, mut x1, mut y1) = (
            f64::INFINITY,
            f64::INFINITY,
            f64::NEG_INFINITY,
            f64::NEG_INFINITY,
        );
        for (&(x, y), &ri) in at.iter().zip(&r) {
            x0 = x0.min(x - ri);
            y0 = y0.min(y - ri);
            x1 = x1.max(x + ri);
            y1 = y1.max(y + ri);
        }
        let k = ((right - left).max(1.0) / (x1 - x0)).min((down - up).max(1.0) / (y1 - y0));
        let ox = (left + right) / 2.0 - (x0 + x1) / 2.0 * k;
        let oy = (up + down) / 2.0 - (y0 + y1) / 2.0 * k;
        let xy: Vec<(f64, f64)> = at.iter().map(|&(x, y)| (ox + x * k, oy + y * k)).collect();
        let (gx, gy) = (
            xy.iter().map(|p| p.0).sum::<f64>() / n as f64,
            xy.iter().map(|p| p.1).sum::<f64>() / n as f64,
        );

        for i in 0..n {
            let (cx, cy) = xy[i];
            let rows: Vec<TipRow> = self
                .areas
                .iter()
                .zip(&keys)
                .filter(|(_, ids)| ids.len() > 1 && ids.contains(&i))
                .map(|(a, ids)| TipRow {
                    color: None,
                    name: format!(
                        "∩ {}",
                        ids.iter()
                            .filter(|&&j| j != i)
                            .map(|&j| names[j])
                            .collect::<Vec<_>>()
                            .join(" ∩ ")
                    ),
                    value: format::value(a.size),
                })
                .collect();
            marks.push(
                Mark::new(Shape::Circle {
                    cx,
                    cy,
                    r: r[i] * k,
                })
                .fill(Color::Link(i as u8))
                .stroke(Color::Series(i as u8), OUTLINE)
                .hit(Hit::new(format!("set-{i}"), names[i], format::value(size[i])).rows(rows)),
            );
        }
        // Each name just outside its circle: straight away from the others
        // if that spot is clear, else whichever of eight directions puts
        // the name over the fewest other circles, inside the chart.
        let inside_other = |i: usize, (px, py): (f64, f64)| {
            (0..n).any(|j| {
                let (dx, dy) = (px - xy[j].0, py - xy[j].1);
                j != i && (dx * dx + dy * dy).sqrt() < r[j] * k
            })
        };
        for i in 0..n {
            let (cx, cy) = xy[i];
            let (dx, dy) = (cx - gx, cy - gy);
            let len = (dx * dx + dy * dy).sqrt();
            let away = if len > 1e-9 {
                (dx / len, dy / len)
            } else {
                (0.0, -1.0)
            };
            let w = measure.width(names[i], TICK_SIZE, Weight::Semibold);
            let reach = r[i] * k + GAP / 2.0;
            let mut best = (f64::INFINITY, (0.0, 0.0), Anchor::Middle, Baseline::Middle);
            for (ux, uy) in std::iter::once(away).chain(DIRS) {
                let (x, y) = (cx + ux * reach, cy + uy * reach);
                let (anchor, baseline, bx, by) = if ux.abs() >= uy.abs() {
                    if ux > 0.0 {
                        (Anchor::Start, Baseline::Middle, x, y - TICK_SIZE / 2.0)
                    } else {
                        (Anchor::End, Baseline::Middle, x - w, y - TICK_SIZE / 2.0)
                    }
                } else if uy > 0.0 {
                    (Anchor::Middle, Baseline::Top, x - w / 2.0, y)
                } else {
                    (Anchor::Middle, Baseline::Bottom, x - w / 2.0, y - TICK_SIZE)
                };
                let covered = [
                    (bx, by),
                    (bx + w, by),
                    (bx, by + TICK_SIZE),
                    (bx + w, by + TICK_SIZE),
                    (bx + w / 2.0, by + TICK_SIZE / 2.0),
                ]
                .into_iter()
                .filter(|&p| inside_other(i, p))
                .count();
                let out =
                    bx < PAD || by < top || bx + w > width - PAD || by + TICK_SIZE > height - PAD;
                let score = 10.0 * covered as f64 + if out { 100.0 } else { 0.0 }
                    - (ux * away.0 + uy * away.1);
                if score < best.0 {
                    best = (score, (x, y), anchor, baseline);
                }
            }
            let (_, (x, y), anchor, baseline) = best;
            marks.push(text(
                x,
                y,
                names[i],
                TICK_SIZE,
                Weight::Semibold,
                anchor,
                baseline,
                Color::TextSecondary,
            ));
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!("Venn diagram of {}.", count(n, "set", "sets")),
            ),
            marks,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;

    fn circles(s: &Scene) -> Vec<(f64, f64, f64)> {
        s.marks
            .iter()
            .filter_map(|m| match m.shape {
                Shape::Circle { cx, cy, r } => Some((cx, cy, r)),
                _ => None,
            })
            .collect()
    }

    /// Overlap as a share of the first circle's area, which survives the
    /// fit's scaling.
    fn shared(c: &[(f64, f64, f64)], i: usize, j: usize) -> f64 {
        let d = ((c[i].0 - c[j].0).powi(2) + (c[i].1 - c[j].1).powi(2)).sqrt();
        lens(c[i].2, c[j].2, d) / (PI * c[i].2 * c[i].2)
    }

    #[test]
    fn two_and_three_sets_match_every_pairwise_overlap() {
        let s = VennChart::new()
            .area(["A"], 100.0)
            .area(["B"], 60.0)
            .area(["C"], 40.0)
            .area(["A", "B"], 20.0)
            .area(["A", "C"], 10.0)
            .area(["B", "C"], 8.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let c = circles(&s);
        assert!((shared(&c, 0, 1) - 0.20).abs() < 1e-6);
        assert!((shared(&c, 0, 2) - 0.10).abs() < 1e-6);
        assert!((shared(&c, 1, 2) - 8.0 / 60.0).abs() < 1e-6);
        // Areas track sizes.
        assert!(((c[1].2 / c[0].2).powi(2) - 0.6).abs() < 1e-9);
    }

    #[test]
    fn four_sets_come_close_and_disjoint_ones_stay_apart() {
        let s = VennChart::new()
            .area(["A"], 100.0)
            .area(["B"], 100.0)
            .area(["C"], 100.0)
            .area(["D"], 30.0)
            .area(["A", "B"], 30.0)
            .area(["B", "C"], 30.0)
            .area(["A", "C"], 30.0)
            .area(["A", "D"], 10.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let c = circles(&s);
        for (i, j, want) in [(0, 1, 0.3), (1, 2, 0.3), (0, 2, 0.3), (0, 3, 0.1)] {
            assert!((shared(&c, i, j) - want).abs() < 0.02, "{i}{j}");
        }
        // D overlaps nothing but A.
        assert_eq!(shared(&c, 1, 3), 0.0);
        assert_eq!(shared(&c, 2, 3), 0.0);
    }

    #[test]
    fn names_avoid_the_other_circles() {
        // Frontend sits near the middle, so "away from the others" points
        // into Design's circle; the name has to find a clear side.
        let s = VennChart::new()
            .area(["Design"], 30.0)
            .area(["Frontend"], 45.0)
            .area(["Backend"], 50.0)
            .area(["Data"], 25.0)
            .area(["Design", "Frontend"], 12.0)
            .area(["Frontend", "Backend"], 15.0)
            .area(["Backend", "Data"], 10.0)
            .area(["Frontend", "Data"], 4.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let c = circles(&s);
        let names: Vec<(f64, f64)> = s
            .marks
            .iter()
            .filter_map(|m| match &m.shape {
                Shape::Text {
                    x, y, size, weight, ..
                } if *size == TICK_SIZE && *weight == Weight::Semibold => Some((*x, *y)),
                _ => None,
            })
            .collect();
        assert_eq!(names.len(), 4);
        for &(x, y) in &names {
            for &(cx, cy, r) in &c {
                assert!(
                    ((x - cx).powi(2) + (y - cy).powi(2)).sqrt() >= r,
                    "({x}, {y})"
                );
            }
        }
    }

    #[test]
    fn a_subset_sits_inside() {
        let s = VennChart::new()
            .area(["All"], 100.0)
            .area(["Some"], 20.0)
            .area(["All", "Some"], 20.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let c = circles(&s);
        assert!((shared(&c, 1, 0) - 1.0).abs() < 1e-9);
    }

    #[test]
    fn impossible_overlaps_are_named() {
        let err = VennChart::new()
            .area(["A"], 10.0)
            .area(["B"], 5.0)
            .area(["A", "B"], 6.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap_err();
        assert!(err.to_string().contains("bigger than \"B\""), "{err}");
        assert!(
            VennChart::new()
                .area(["A"], 10.0)
                .area(["A", "Z"], 1.0)
                .scene(640.0, 400.0, &InterMetrics)
                .is_err()
        );
    }

    /// The area inside every circle, counted on a fine grid.
    fn counted(c: &[Circle]) -> f64 {
        let x0 = c.iter().map(|k| k.0 - k.2).fold(f64::INFINITY, f64::min);
        let x1 = c
            .iter()
            .map(|k| k.0 + k.2)
            .fold(f64::NEG_INFINITY, f64::max);
        let y0 = c.iter().map(|k| k.1 - k.2).fold(f64::INFINITY, f64::min);
        let y1 = c
            .iter()
            .map(|k| k.1 + k.2)
            .fold(f64::NEG_INFINITY, f64::max);
        let steps = 1200;
        let (sx, sy) = ((x1 - x0) / steps as f64, (y1 - y0) / steps as f64);
        let mut hits = 0;
        for a in 0..steps {
            for b in 0..steps {
                let (x, y) = (x0 + (a as f64 + 0.5) * sx, y0 + (b as f64 + 0.5) * sy);
                hits += c.iter().all(|&k| apart(x, y, k) <= k.2) as usize;
            }
        }
        hits as f64 * sx * sy
    }

    #[test]
    fn the_common_area_of_any_circles_matches_a_count() {
        for c in [
            // Two: the lens.
            vec![(0.0, 0.0, 3.0), (4.0, 0.0, 2.0)],
            // Three overlapping unevenly.
            vec![(0.0, 0.0, 3.0), (3.0, 0.5, 2.5), (1.2, 2.4, 2.0)],
            // Four, one corner region cut by every circle.
            vec![
                (0.0, 0.0, 3.0),
                (2.5, 0.0, 3.0),
                (1.2, 2.0, 2.6),
                (1.0, 0.6, 1.8),
            ],
            // A small circle inside two larger ones.
            vec![(0.0, 0.0, 5.0), (1.0, 0.0, 5.0), (0.5, 0.5, 1.0)],
            // Pairwise overlapping, nothing common to all three.
            vec![(0.0, 0.0, 2.0), (3.0, 0.0, 2.0), (1.5, 2.8, 2.0)],
        ] {
            let (want, got) = (counted(&c), common_area(&c));
            assert!(
                (got - want).abs() <= 0.005 * want.max(0.05),
                "{c:?}: {got} vs counted {want}"
            );
        }
        let two = [(0.0, 0.0, 3.0), (4.0, 0.0, 2.0)];
        assert!((common_area(&two) - lens(3.0, 2.0, 4.0)).abs() < 1e-9);
    }

    #[test]
    fn a_three_way_overlap_given_shapes_the_layout() {
        // Placed by pairs alone, three equal sets each sharing 30% have
        // about 14% in common to all three; the data says 25%.
        let pairs = VennChart::new()
            .area(["A"], 100.0)
            .area(["B"], 100.0)
            .area(["C"], 100.0)
            .area(["A", "B"], 30.0)
            .area(["A", "C"], 30.0)
            .area(["B", "C"], 30.0);
        let all = pairs.clone().area(["A", "B", "C"], 25.0);
        // Squared error of the pairs, and the error of the three-way share.
        let fit = |c: &VennChart| -> (f64, f64) {
            let c = circles(&c.scene(640.0, 400.0, &InterMetrics).unwrap());
            let pairs = [(0, 1), (0, 2), (1, 2)]
                .iter()
                .map(|&(i, j)| (shared(&c, i, j) - 0.3).powi(2))
                .sum::<f64>();
            (pairs, common_area(&c) / (PI * c[0].2 * c[0].2) - 0.25)
        };
        let (p0, t0) = fit(&pairs);
        let (p1, t1) = fit(&all);
        assert!(p0 < 1e-9 && t0 < -0.1, "pairs alone: {p0}, {t0}");
        assert!(t1.abs() < t0.abs() - 0.02, "{t1} is no nearer than {t0}");
        assert!(p1 + t1 * t1 < t0 * t0, "worse overall: {p1} + {t1}²");
    }

    #[test]
    fn overlaps_must_nest() {
        let base = || {
            VennChart::new()
                .area(["A"], 10.0)
                .area(["B"], 10.0)
                .area(["C"], 10.0)
                .area(["A", "B"], 3.0)
                .area(["A", "C"], 3.0)
        };
        let err = |c: VennChart| {
            c.scene(640.0, 400.0, &InterMetrics)
                .unwrap_err()
                .to_string()
        };
        let e = err(base().area(["B", "C"], 3.0).area(["A", "B", "C"], 4.0));
        assert!(e.contains("is bigger than A ∩ B (3)"), "{e}");
        let e = err(base().area(["A", "B", "C"], 1.0));
        assert!(e.contains("B ∩ C is not given"), "{e}");
        // A three-way overlap of nothing needs no pairs.
        assert!(
            base()
                .area(["A", "B", "C"], 0.0)
                .scene(640.0, 400.0, &InterMetrics)
                .is_ok()
        );
    }
}
