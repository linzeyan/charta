//! Sankey placement, ported from d3-sankey with its defaults: nodes in
//! columns by depth with sinks pushed to the last column ("justify"), node
//! heights proportional to throughput, then six rounds of relaxation that
//! slide each node toward the links feeding it while keeping a column's
//! nodes apart.
//!
//! One deliberate difference: link order at every node is re-sorted after
//! the last round, so links never cross where they meet a node.

/// Placed nodes and links, in input order. Vertical only: the caller turns
/// `layer` into x.
pub(crate) struct Sankey {
    pub columns: usize,
    pub layer: Vec<usize>,
    pub value: Vec<f64>,
    pub y0: Vec<f64>,
    pub y1: Vec<f64>,
    /// Per link: its thickness, and the y of its center where it leaves the
    /// source and where it enters the target.
    pub width: Vec<f64>,
    pub source_y: Vec<f64>,
    pub target_y: Vec<f64>,
}

const ITERATIONS: usize = 6;

struct Layout<'a> {
    edges: &'a [(usize, usize, f64)],
    layer: Vec<usize>,
    y0: Vec<f64>,
    y1: Vec<f64>,
    width: Vec<f64>,
    /// Link indices leaving / entering each node, kept sorted by the other
    /// end's position.
    out: Vec<Vec<usize>>,
    inc: Vec<Vec<usize>>,
    py: f64,
    top: f64,
    bottom: f64,
}

/// Lays `n` nodes and their `edges` out between `top` and `bottom`, columns
/// `padding` apart at most. Fails on a cycle, which has no left-to-right
/// order, naming a node on it.
pub(crate) fn sankey(
    n: usize,
    edges: &[(usize, usize, f64)],
    (top, bottom): (f64, f64),
    padding: f64,
) -> Result<Sankey, usize> {
    let mut out = vec![Vec::new(); n];
    let mut inc = vec![Vec::new(); n];
    for (k, &(s, t, _)) in edges.iter().enumerate() {
        out[s].push(k);
        inc[t].push(k);
    }
    let value: Vec<f64> = (0..n)
        .map(|i| {
            let o: f64 = out[i].iter().map(|&k| edges[k].2).sum();
            let n: f64 = inc[i].iter().map(|&k| edges[k].2).sum();
            o.max(n)
        })
        .collect();

    // Depth: the longest path from any source. A path longer than the node
    // count must revisit a node.
    let mut depth = vec![0; n];
    let mut current: Vec<usize> = (0..n).collect();
    let mut x = 0;
    while !current.is_empty() {
        let mut next = Vec::new();
        for &i in &current {
            depth[i] = x;
            for &k in &out[i] {
                if !next.contains(&edges[k].1) {
                    next.push(edges[k].1);
                }
            }
        }
        x += 1;
        if x > n {
            return Err(on_cycle(edges, &out));
        }
        current = next;
    }
    let columns_n = depth.iter().max().map_or(0, |d| d + 1);
    let layer: Vec<usize> = (0..n)
        .map(|i| {
            if out[i].is_empty() {
                columns_n - 1
            } else {
                depth[i]
            }
        })
        .collect();
    let mut columns: Vec<Vec<usize>> = vec![Vec::new(); columns_n];
    for i in 0..n {
        columns[layer[i]].push(i);
    }

    let tallest = columns.iter().map(Vec::len).max().unwrap_or(1);
    let py = if tallest > 1 {
        padding.min((bottom - top) / (tallest - 1) as f64)
    } else {
        padding
    };
    let ky = columns
        .iter()
        .map(|c| {
            let total: f64 = c.iter().map(|&i| value[i]).sum();
            (bottom - top - (c.len() - 1) as f64 * py) / total
        })
        .fold(f64::INFINITY, f64::min);

    let mut l = Layout {
        edges,
        layer,
        y0: vec![0.0; n],
        y1: vec![0.0; n],
        width: edges.iter().map(|e| e.2 * ky).collect(),
        out,
        inc,
        py,
        top,
        bottom,
    };
    // Stack each column from the top, then spread its slack evenly.
    for c in &columns {
        let mut y = top;
        for &i in c {
            l.y0[i] = y;
            l.y1[i] = y + value[i] * ky;
            y = l.y1[i] + py;
        }
        let slack = (bottom - y + py) / (c.len() + 1) as f64;
        for (k, &i) in c.iter().enumerate() {
            l.y0[i] += slack * (k + 1) as f64;
            l.y1[i] += slack * (k + 1) as f64;
        }
    }
    for i in 0..n {
        l.sort_links(i);
    }

    let mut alpha = 1.0;
    for it in 0..ITERATIONS {
        let beta = f64::max(1.0 - alpha, (it + 1) as f64 / ITERATIONS as f64);
        l.relax_right_to_left(&mut columns, alpha, beta);
        l.relax_left_to_right(&mut columns, alpha, beta);
        alpha *= 0.99;
    }
    for i in 0..n {
        l.sort_links(i);
    }

    let mut source_y = vec![0.0; edges.len()];
    let mut target_y = vec![0.0; edges.len()];
    for i in 0..n {
        let mut y = l.y0[i];
        for &k in &l.out[i] {
            source_y[k] = y + l.width[k] / 2.0;
            y += l.width[k];
        }
        let mut y = l.y0[i];
        for &k in &l.inc[i] {
            target_y[k] = y + l.width[k] / 2.0;
            y += l.width[k];
        }
    }
    Ok(Sankey {
        columns: columns_n,
        layer: l.layer,
        value,
        y0: l.y0,
        y1: l.y1,
        width: l.width,
        source_y,
        target_y,
    })
}

/// The first node (in input order) that can reach itself, for the error.
fn on_cycle(edges: &[(usize, usize, f64)], out: &[Vec<usize>]) -> usize {
    (0..out.len())
        .find(|&i| {
            let mut seen = vec![false; out.len()];
            let mut stack: Vec<usize> = out[i].iter().map(|&k| edges[k].1).collect();
            while let Some(j) = stack.pop() {
                if j == i {
                    return true;
                }
                if !std::mem::replace(&mut seen[j], true) {
                    stack.extend(out[j].iter().map(|&k| edges[k].1));
                }
            }
            false
        })
        .expect("a path longer than the node count revisits a node")
}

impl Layout<'_> {
    /// Orders a node's outgoing links by target position and incoming links
    /// by source position (input order breaks ties), so links fan out
    /// without crossing at the node.
    fn sort_links(&mut self, i: usize) {
        let (e, y0) = (self.edges, &self.y0);
        self.out[i].sort_by(|&a, &b| y0[e[a].1].total_cmp(&y0[e[b].1]).then(a.cmp(&b)));
        self.inc[i].sort_by(|&a, &b| y0[e[a].0].total_cmp(&y0[e[b].0]).then(a.cmp(&b)));
    }

    /// After node `i` moves, its neighbors' link orders may be stale.
    fn resort_neighbors(&mut self, i: usize) {
        for k in self.inc[i].clone() {
            let s = self.edges[k].0;
            let (e, y0) = (self.edges, &self.y0);
            self.out[s].sort_by(|&a, &b| y0[e[a].1].total_cmp(&y0[e[b].1]).then(a.cmp(&b)));
        }
        for k in self.out[i].clone() {
            let t = self.edges[k].1;
            let (e, y0) = (self.edges, &self.y0);
            self.inc[t].sort_by(|&a, &b| y0[e[a].0].total_cmp(&y0[e[b].0]).then(a.cmp(&b)));
        }
    }

    /// The target top that would make link source → target run straight.
    fn target_top(&self, source: usize, target: usize) -> f64 {
        let mut y = self.y0[source] - (self.out[source].len() - 1) as f64 * self.py / 2.0;
        for &k in &self.out[source] {
            if self.edges[k].1 == target {
                break;
            }
            y += self.width[k] + self.py;
        }
        for &k in &self.inc[target] {
            if self.edges[k].0 == source {
                break;
            }
            y -= self.width[k];
        }
        y
    }

    /// The source top that would make link source → target run straight.
    fn source_top(&self, source: usize, target: usize) -> f64 {
        let mut y = self.y0[target] - (self.inc[target].len() - 1) as f64 * self.py / 2.0;
        for &k in &self.inc[target] {
            if self.edges[k].0 == source {
                break;
            }
            y += self.width[k] + self.py;
        }
        for &k in &self.out[source] {
            if self.edges[k].1 == target {
                break;
            }
            y -= self.width[k];
        }
        y
    }

    fn relax_left_to_right(&mut self, columns: &mut [Vec<usize>], alpha: f64, beta: f64) {
        for column in columns.iter_mut().skip(1) {
            for &t in column.iter() {
                let (mut y, mut w) = (0.0, 0.0);
                for &k in &self.inc[t] {
                    let (s, _, value) = self.edges[k];
                    let v = value * (self.layer[t] - self.layer[s]) as f64;
                    y += self.target_top(s, t) * v;
                    w += v;
                }
                if w > 0.0 {
                    let dy = (y / w - self.y0[t]) * alpha;
                    self.y0[t] += dy;
                    self.y1[t] += dy;
                    self.resort_neighbors(t);
                }
            }
            self.settle(column, beta);
        }
    }

    fn relax_right_to_left(&mut self, columns: &mut [Vec<usize>], alpha: f64, beta: f64) {
        for column in columns.iter_mut().rev().skip(1) {
            for &s in column.iter() {
                let (mut y, mut w) = (0.0, 0.0);
                for &k in &self.out[s] {
                    let (_, t, value) = self.edges[k];
                    let v = value * (self.layer[t] - self.layer[s]) as f64;
                    y += self.source_top(s, t) * v;
                    w += v;
                }
                if w > 0.0 {
                    let dy = (y / w - self.y0[s]) * alpha;
                    self.y0[s] += dy;
                    self.y1[s] += dy;
                    self.resort_neighbors(s);
                }
            }
            self.settle(column, beta);
        }
    }

    /// Re-sorts a column by position, then pushes overlapping nodes apart
    /// outward from the middle one and back inside the extent.
    fn settle(&mut self, column: &mut [usize], alpha: f64) {
        let y0 = &self.y0;
        column.sort_by(|&a, &b| y0[a].total_cmp(&y0[b]));
        let mid = column.len() / 2;
        let subject = column[mid];
        self.push_up(column, self.y0[subject] - self.py, mid as isize - 1, alpha);
        self.push_down(column, self.y1[subject] + self.py, mid + 1, alpha);
        self.push_up(column, self.bottom, column.len() as isize - 1, alpha);
        self.push_down(column, self.top, 0, alpha);
    }

    fn push_down(&mut self, column: &[usize], mut y: f64, from: usize, alpha: f64) {
        for &i in &column[from.min(column.len())..] {
            let dy = (y - self.y0[i]) * alpha;
            if dy > 1e-6 {
                self.y0[i] += dy;
                self.y1[i] += dy;
            }
            y = self.y1[i] + self.py;
        }
    }

    fn push_up(&mut self, column: &[usize], mut y: f64, from: isize, alpha: f64) {
        let mut k = from;
        while k >= 0 {
            let i = column[k as usize];
            let dy = (self.y1[i] - y) * alpha;
            if dy > 1e-6 {
                self.y0[i] -= dy;
                self.y1[i] -= dy;
            }
            y = self.y0[i] - self.py;
            k -= 1;
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Energy-style flows: two sources merge, then split three ways, one
    /// branch skipping a column.
    fn sample() -> Vec<(usize, usize, f64)> {
        vec![
            (0, 2, 30.0),
            (1, 2, 20.0),
            (2, 3, 25.0),
            (2, 4, 15.0),
            (2, 5, 10.0),
            (3, 5, 5.0),
            (0, 6, 12.0),
        ]
    }

    #[test]
    fn links_run_left_to_right_and_sinks_line_up_on_the_right() {
        let e = sample();
        let s = sankey(7, &e, (0.0, 300.0), 8.0).unwrap();
        for &(a, b, _) in &e {
            assert!(s.layer[a] < s.layer[b]);
        }
        // 4, 5 and 6 have no outgoing links: all end in the last column,
        // where a reader looks for final destinations.
        for i in [4, 5, 6] {
            assert_eq!(s.layer[i], s.columns - 1);
        }
    }

    #[test]
    fn nodes_in_a_column_keep_apart_and_inside() {
        let s = sankey(7, &sample(), (0.0, 300.0), 8.0).unwrap();
        for c in 0..s.columns {
            let mut col: Vec<usize> = (0..7).filter(|&i| s.layer[i] == c).collect();
            col.sort_by(|&a, &b| s.y0[a].total_cmp(&s.y0[b]));
            for w in col.windows(2) {
                assert!(s.y0[w[1]] - s.y1[w[0]] >= 8.0 - 1e-6, "column {c}");
            }
            for &i in &col {
                assert!(s.y0[i] >= -1e-6 && s.y1[i] <= 300.0 + 1e-6);
            }
        }
    }

    #[test]
    fn heights_are_throughput_and_links_tile_each_node() {
        let e = sample();
        let s = sankey(7, &e, (0.0, 300.0), 8.0).unwrap();
        let ky = s.width[0] / e[0].2;
        for i in 0..7 {
            assert!(((s.y1[i] - s.y0[i]) - s.value[i] * ky).abs() < 1e-9);
            // Links leaving a node stack edge to edge from its top, in
            // the order their targets sit, so none cross at the node.
            let mut outs: Vec<usize> = (0..e.len()).filter(|&k| e[k].0 == i).collect();
            outs.sort_by(|&a, &b| s.source_y[a].total_cmp(&s.source_y[b]));
            let mut y = s.y0[i];
            for w in outs.windows(2) {
                assert!(s.y0[e[w[0]].1] <= s.y0[e[w[1]].1] + 1e-9);
            }
            for &k in &outs {
                assert!((s.source_y[k] - s.width[k] / 2.0 - y).abs() < 1e-9);
                y += s.width[k];
            }
        }
    }

    #[test]
    fn a_cycle_is_reported() {
        // 0 feeds the loop 1 → 2 → 1 but is not on it.
        let e = [(0, 1, 1.0), (1, 2, 1.0), (2, 1, 1.0)];
        assert_eq!(sankey(3, &e, (0.0, 100.0), 8.0).err(), Some(1));
    }
}
