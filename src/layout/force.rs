//! Force-directed placement after d3-force's defaults: links pull toward a
//! rest length, every pair of nodes repels, weak x/y forces keep separate
//! components from drifting apart, and the system cools over 300 ticks.
//!
//! The many-body force is Barnes–Hut, as in d3: a far enough cell of a
//! quadtree pushes as one body at its centroid. Both ports build the tree
//! the same way (nodes inserted in index order, a fixed quadrant order,
//! cells summed children first) and walk it in the same order, so the
//! approximation, and every jiggle drawn on the way, comes out bit-identical.

use super::hierarchy::Lcg;

const TICKS: usize = 300;
/// 1 − 0.001^(1/300), d3's default: alpha falls from 1 to 0.001 over the
/// ticks. A literal so neither port depends on its `pow`.
const ALPHA_DECAY: f64 = 0.022_762_779_044_189_33;
/// d3's velocity decay of 0.4: each tick keeps 60% of the velocity.
const VELOCITY_KEEP: f64 = 0.6;
const DISTANCE: f64 = 30.0;
const CHARGE: f64 = -30.0;
const GRAVITY: f64 = 0.05;
/// d3's Barnes–Hut θ of 0.9, squared: a cell narrower than θ times its
/// distance pushes as one body.
const THETA2: f64 = 0.81;
/// Cells stop splitting this deep; nodes closer than that share a leaf.
const MAX_DEPTH: u32 = 32;

/// A quadtree cell over the square at (`x`, `y`) of side `size`: how many
/// nodes it holds and their centroid, and either four children or, as a
/// leaf, the nodes themselves.
struct Cell {
    x: f64,
    y: f64,
    size: f64,
    depth: u32,
    count: usize,
    cx: f64,
    cy: f64,
    kids: Option<[usize; 4]>,
    nodes: Vec<usize>,
}

impl Cell {
    fn new(x: f64, y: f64, size: f64, depth: u32) -> Self {
        Cell {
            x,
            y,
            size,
            depth,
            count: 0,
            cx: 0.0,
            cy: 0.0,
            kids: None,
            nodes: Vec::new(),
        }
    }

    /// Which child holds (`px`, `py`): right adds 1, lower adds 2.
    fn quadrant(&self, px: f64, py: f64) -> usize {
        let half = self.size / 2.0;
        usize::from(px >= self.x + half) + 2 * usize::from(py >= self.y + half)
    }
}

/// The quadtree over `p`, root first; a child always comes after its parent.
fn quadtree(p: &[(f64, f64)]) -> Vec<Cell> {
    let (mut x0, mut y0, mut x1, mut y1) = (p[0].0, p[0].1, p[0].0, p[0].1);
    for &(x, y) in p {
        x0 = x0.min(x);
        y0 = y0.min(y);
        x1 = x1.max(x);
        y1 = y1.max(y);
    }
    let size = (x1 - x0).max(y1 - y0);
    let mut cells = vec![Cell::new(x0, y0, if size > 0.0 { size } else { 1.0 }, 0)];
    for (i, &(px, py)) in p.iter().enumerate() {
        let mut c = 0;
        loop {
            if let Some(kids) = cells[c].kids {
                c = kids[cells[c].quadrant(px, py)];
                continue;
            }
            let cell = &cells[c];
            let shared = cell.nodes.first().is_some_and(|&j| p[j] == (px, py));
            if cell.nodes.is_empty() || shared || cell.depth == MAX_DEPTH {
                cells[c].nodes.push(i);
                break;
            }
            // Split: the nodes here (all at one spot) move down a level,
            // and the new one tries again.
            let (x, y, half, depth) = (cell.x, cell.y, cell.size / 2.0, cell.depth + 1);
            let first = cells.len();
            for k in 0..4 {
                let (dx, dy) = ((k % 2) as f64 * half, (k / 2) as f64 * half);
                cells.push(Cell::new(x + dx, y + dy, half, depth));
            }
            let moved = std::mem::take(&mut cells[c].nodes);
            let j = moved[0];
            let q = cells[c].quadrant(p[j].0, p[j].1);
            cells[first + q].nodes = moved;
            cells[c].kids = Some([first, first + 1, first + 2, first + 3]);
        }
    }
    // Children come after parents, so backward is children first.
    for c in (0..cells.len()).rev() {
        let (count, sx, sy) = match cells[c].kids {
            Some(kids) => kids.iter().fold((0, 0.0, 0.0), |(n, sx, sy), &k| {
                let w = cells[k].count as f64;
                (
                    n + cells[k].count,
                    sx + w * cells[k].cx,
                    sy + w * cells[k].cy,
                )
            }),
            None => cells[c]
                .nodes
                .iter()
                .fold((0, 0.0, 0.0), |(n, sx, sy), &j| {
                    (n + 1, sx + p[j].0, sy + p[j].1)
                }),
        };
        let cell = &mut cells[c];
        cell.count = count;
        if count > 0 {
            cell.cx = sx / count as f64;
            cell.cy = sy / count as f64;
        }
    }
    cells
}

/// Adds every node's push from all the others to `v`, Barnes–Hut style.
fn many_body(p: &[(f64, f64)], v: &mut [(f64, f64)], alpha: f64, rng: &mut Lcg) {
    let cells = quadtree(p);
    let mut stack = Vec::new();
    for i in 0..p.len() {
        stack.push(0);
        while let Some(c) = stack.pop() {
            let cell = &cells[c];
            if cell.count == 0 {
                continue;
            }
            let mut x = cell.cx - p[i].0;
            let mut y = cell.cy - p[i].1;
            let mut l = x * x + y * y;
            let far = cell.size * cell.size / THETA2 < l;
            if !far {
                if let Some(kids) = cell.kids {
                    // Pushed backward, so the children are visited in order.
                    stack.extend(kids.iter().rev());
                    continue;
                }
                if cell.nodes == [i] {
                    continue;
                }
            }
            if x == 0.0 {
                x = jiggle(rng);
                l += x * x;
            }
            if y == 0.0 {
                y = jiggle(rng);
                l += y * y;
            }
            // d3's minimum distance of 1 keeps near-coincident nodes from
            // flinging each other off to infinity.
            if l < 1.0 {
                l = l.sqrt();
            }
            let others = if far {
                cell.count
            } else {
                cell.nodes.iter().filter(|&&j| j != i).count()
            };
            let w = CHARGE * others as f64 * alpha / l;
            v[i].0 += x * w;
            v[i].1 += y * w;
        }
    }
}

fn jiggle(rng: &mut Lcg) -> f64 {
    (rng.next() - 0.5) * 1e-6
}

/// Positions for `n` nodes joined by `edges`, in layout units centered on
/// the origin; the caller scales them to the plot.
pub(crate) fn force(n: usize, edges: &[(usize, usize)]) -> Vec<(f64, f64)> {
    let mut rng = Lcg(1);
    // Random starts over d3's phyllotaxis radius; d3's own spiral needs
    // sin and cos, whose last bit differs between the ports' math libraries.
    let spread = 10.0 * (n as f64).sqrt();
    let mut p: Vec<(f64, f64)> = (0..n)
        .map(|_| {
            let x = (rng.next() * 2.0 - 1.0) * spread;
            let y = (rng.next() * 2.0 - 1.0) * spread;
            (x, y)
        })
        .collect();
    let mut v = vec![(0.0, 0.0); n];

    // d3's link defaults: a link pulls less on a node with many links, and
    // moves the less-connected end more.
    let mut degree = vec![0.0; n];
    for &(s, t) in edges {
        degree[s] += 1.0;
        degree[t] += 1.0;
    }
    let strength: Vec<f64> = edges
        .iter()
        .map(|&(s, t)| 1.0 / f64::min(degree[s], degree[t]))
        .collect();
    let bias: Vec<f64> = edges
        .iter()
        .map(|&(s, t)| degree[s] / (degree[s] + degree[t]))
        .collect();

    let mut alpha = 1.0;
    for _ in 0..TICKS {
        alpha -= alpha * ALPHA_DECAY;
        for (k, &(s, t)) in edges.iter().enumerate() {
            let mut x = p[t].0 + v[t].0 - p[s].0 - v[s].0;
            let mut y = p[t].1 + v[t].1 - p[s].1 - v[s].1;
            if x == 0.0 {
                x = jiggle(&mut rng);
            }
            if y == 0.0 {
                y = jiggle(&mut rng);
            }
            let d = (x * x + y * y).sqrt();
            let l = (d - DISTANCE) / d * alpha * strength[k];
            x *= l;
            y *= l;
            let b = bias[k];
            v[t].0 -= x * b;
            v[t].1 -= y * b;
            v[s].0 += x * (1.0 - b);
            v[s].1 += y * (1.0 - b);
        }
        many_body(&p, &mut v, alpha, &mut rng);
        for i in 0..n {
            v[i].0 -= p[i].0 * GRAVITY * alpha;
            v[i].1 -= p[i].1 * GRAVITY * alpha;
            v[i].0 *= VELOCITY_KEEP;
            v[i].1 *= VELOCITY_KEEP;
            p[i].0 += v[i].0;
            p[i].1 += v[i].1;
        }
    }
    p
}

#[cfg(test)]
mod tests {
    use super::*;

    fn dist(a: (f64, f64), b: (f64, f64)) -> f64 {
        ((a.0 - b.0).powi(2) + (a.1 - b.1).powi(2)).sqrt()
    }

    #[test]
    fn clusters_stay_together_and_apart() {
        // Two triangles joined by one bridge (2–3): the reason to draw a
        // network at all is that this structure shows.
        let e = [(0, 1), (1, 2), (2, 0), (3, 4), (4, 5), (5, 3), (2, 3)];
        let p = force(6, &e);
        let within = (dist(p[0], p[1]) + dist(p[3], p[4])) / 2.0;
        let across = (dist(p[0], p[4]) + dist(p[1], p[5])) / 2.0;
        assert!(across > 1.5 * within, "{within} {across}");
    }

    #[test]
    fn no_two_nodes_land_on_each_other() {
        // A star: every leaf is pulled to the same hub, only repulsion keeps
        // them apart.
        let e: Vec<(usize, usize)> = (1..12).map(|i| (0, i)).collect();
        let p = force(12, &e);
        for i in 0..12 {
            for j in i + 1..12 {
                assert!(dist(p[i], p[j]) > 5.0, "{i} {j}");
            }
        }
    }

    #[test]
    fn separate_components_do_not_drift_away() {
        let p = force(6, &[(0, 1), (2, 3), (4, 5)]);
        for q in p {
            assert!(q.0.abs() < 150.0 && q.1.abs() < 150.0, "{q:?}");
        }
    }

    #[test]
    fn linked_nodes_settle_near_the_rest_length() {
        let p = force(2, &[(0, 1)]);
        let d = dist(p[0], p[1]);
        assert!(d > 20.0 && d < 60.0, "{d}");
    }

    /// Every pair summed exactly, as the layout did before Barnes–Hut.
    fn exact(p: &[(f64, f64)]) -> Vec<(f64, f64)> {
        let mut v = vec![(0.0, 0.0); p.len()];
        for i in 0..p.len() {
            for j in 0..p.len() {
                if i != j {
                    let (x, y) = (p[j].0 - p[i].0, p[j].1 - p[i].1);
                    let mut l = x * x + y * y;
                    if l < 1.0 {
                        l = l.sqrt();
                    }
                    v[i].0 += x * CHARGE / l;
                    v[i].1 += y * CHARGE / l;
                }
            }
        }
        v
    }

    #[test]
    fn the_tree_pushes_about_as_hard_as_every_pair() {
        // Clustered like a laid-out graph: a few dense blobs and a sparse
        // field, where far cells stand in for many nodes at once.
        let mut rng = Lcg(7);
        let p: Vec<(f64, f64)> = (0..600)
            .map(|i| {
                let (cx, cy, s) =
                    [(0.0, 0.0, 40.0), (150.0, 30.0, 20.0), (-90.0, 120.0, 10.0)][i % 3];
                let spread = if i % 5 == 0 { 300.0 } else { s };
                (
                    cx + (rng.next() - 0.5) * spread,
                    cy + (rng.next() - 0.5) * spread,
                )
            })
            .collect();
        let want = exact(&p);
        let mut got = vec![(0.0, 0.0); p.len()];
        many_body(&p, &mut got, 1.0, &mut Lcg(1));
        let norm = |a: (f64, f64)| (a.0 * a.0 + a.1 * a.1).sqrt();
        let mut errs: Vec<f64> = want
            .iter()
            .zip(&got)
            .map(|(&w, &g)| norm((g.0 - w.0, g.1 - w.1)) / norm(w))
            .collect();
        errs.sort_by(f64::total_cmp);
        let (median, p95) = (errs[errs.len() / 2], errs[errs.len() * 95 / 100]);
        // θ = 0.9 measured about 1.4% and 6.9%.
        assert!(median < 0.03 && p95 < 0.12, "median {median}, 95th {p95}");
    }
}
