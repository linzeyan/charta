//! Layered (Sugiyama) placement for directed graphs drawn top to bottom:
//! break cycles, assign layers, order each layer to reduce crossings, then
//! give every node an x that keeps it near its neighbors.
//!
//! Each stage is the simple, well-known choice rather than the strongest:
//! DFS back-edge reversal, longest-path layering, barycenter sweeps, and
//! least-squares x placement per layer. Flowcharts are small; the simple
//! versions are readable, and both ports can follow them step by step.

/// Positions for the real nodes, plus the route of every edge.
pub(crate) struct Layered {
    pub layers: usize,
    pub layer: Vec<usize>,
    /// Center x of each node, the leftmost edge at 0.
    pub x: Vec<f64>,
    /// Total width.
    pub width: f64,
    /// Per input edge: x of each intermediate layer it passes through, in
    /// order from its upper end to its lower end.
    pub bends: Vec<Vec<f64>>,
    /// The edge points up (it closes a loop): its source is the lower end.
    pub reversed: Vec<bool>,
}

const SWEEPS: usize = 4;
const ROUNDS: usize = 6;

/// Lays out nodes of the given `widths` joined by `edges` (no self-loops,
/// no duplicates), keeping `gap` between neighbors in a layer.
pub(crate) fn layered(widths: &[f64], edges: &[(usize, usize)], gap: f64) -> Layered {
    let n = widths.len();
    let reversed = back_edges(n, edges);
    let down: Vec<(usize, usize)> = edges
        .iter()
        .zip(&reversed)
        .map(|(&(s, t), &r)| if r { (t, s) } else { (s, t) })
        .collect();
    let layer = layers(n, &down);
    let layers_n = layer.iter().max().map_or(0, |l| l + 1);

    // Split long edges with zero-width dummies, one per layer crossed.
    let mut w = widths.to_vec();
    let mut lay = layer.clone();
    let mut chains: Vec<Vec<usize>> = Vec::with_capacity(down.len());
    let mut succ: Vec<Vec<usize>> = vec![Vec::new(); n];
    let mut pred: Vec<Vec<usize>> = vec![Vec::new(); n];
    for &(s, t) in &down {
        let mut chain = vec![s];
        for l in layer[s] + 1..layer[t] {
            w.push(0.0);
            lay.push(l);
            succ.push(Vec::new());
            pred.push(Vec::new());
            chain.push(w.len() - 1);
        }
        chain.push(t);
        for p in chain.windows(2) {
            succ[p[0]].push(p[1]);
            pred[p[1]].push(p[0]);
        }
        chains.push(chain);
    }
    let total = w.len();

    // Initial order: the order a depth-first walk from the sources meets
    // each node, which already keeps most subtrees together.
    let mut rows: Vec<Vec<usize>> = vec![Vec::new(); layers_n];
    let mut seen = vec![false; total];
    for root in 0..total {
        if seen[root] || !pred[root].is_empty() {
            continue;
        }
        let mut stack = vec![root];
        while let Some(v) = stack.pop() {
            if std::mem::replace(&mut seen[v], true) {
                continue;
            }
            rows[lay[v]].push(v);
            stack.extend(succ[v].iter().rev().filter(|&&u| !seen[u]));
        }
    }
    order(&mut rows, &pred, &succ);

    let sep = |a: usize, b: usize| {
        let g = if a >= n && b >= n { gap / 2.0 } else { gap };
        (w[a] + w[b]) / 2.0 + g
    };
    let mut x = vec![0.0; total];
    for row in &rows {
        let mut at = 0.0;
        for (k, &v) in row.iter().enumerate() {
            at = if k == 0 {
                w[v] / 2.0
            } else {
                at + sep(row[k - 1], v)
            };
            x[v] = at;
        }
    }
    let mean = |x: &[f64], of: &[usize], v: usize| -> f64 {
        if of.is_empty() {
            x[v]
        } else {
            of.iter().map(|&u| x[u]).sum::<f64>() / of.len() as f64
        }
    };
    for round in 0..=ROUNDS {
        for row in &rows {
            let want: Vec<f64> = row
                .iter()
                .map(|&v| {
                    if round == ROUNDS {
                        let both: Vec<usize> = pred[v].iter().chain(&succ[v]).copied().collect();
                        mean(&x, &both, v)
                    } else {
                        mean(&x, &pred[v], v)
                    }
                })
                .collect();
            place(row, &want, &sep, &mut x);
        }
        if round == ROUNDS {
            break;
        }
        for row in rows.iter().rev() {
            let want: Vec<f64> = row.iter().map(|&v| mean(&x, &succ[v], v)).collect();
            place(row, &want, &sep, &mut x);
        }
    }
    let left = (0..total)
        .map(|v| x[v] - w[v] / 2.0)
        .fold(f64::INFINITY, f64::min);
    let left = if left.is_finite() { left } else { 0.0 };
    for v in &mut x {
        *v -= left;
    }
    let width = (0..total).map(|v| x[v] + w[v] / 2.0).fold(0.0, f64::max);
    Layered {
        layers: layers_n,
        layer,
        width,
        bends: chains
            .iter()
            .map(|c| c[1..c.len() - 1].iter().map(|&d| x[d]).collect())
            .collect(),
        x: x[..n].to_vec(),
        reversed,
    }
}

/// Marks the edges a depth-first walk (nodes and edges in input order)
/// finds pointing back to a node still on its path. Reversing exactly those
/// leaves a graph without cycles.
fn back_edges(n: usize, edges: &[(usize, usize)]) -> Vec<bool> {
    let mut out: Vec<Vec<usize>> = vec![Vec::new(); n];
    for (k, &(s, _)) in edges.iter().enumerate() {
        out[s].push(k);
    }
    // 0 unvisited, 1 on the current path, 2 done.
    let mut state = vec![0u8; n];
    let mut back = vec![false; edges.len()];
    for root in 0..n {
        if state[root] != 0 {
            continue;
        }
        state[root] = 1;
        let mut stack = vec![(root, 0)];
        while let Some(&mut (v, ref mut next)) = stack.last_mut() {
            if *next == out[v].len() {
                state[v] = 2;
                stack.pop();
                continue;
            }
            let k = out[v][*next];
            *next += 1;
            let t = edges[k].1;
            match state[t] {
                0 => {
                    state[t] = 1;
                    stack.push((t, 0));
                }
                1 => back[k] = true,
                _ => {}
            }
        }
    }
    back
}

/// Longest path from the sources, then each source moved down to sit just
/// above its highest child: a side entry joins where it is needed instead
/// of trailing a long edge from the top row.
fn layers(n: usize, edges: &[(usize, usize)]) -> Vec<usize> {
    let mut indeg = vec![0; n];
    let mut out: Vec<Vec<usize>> = vec![Vec::new(); n];
    for &(s, t) in edges {
        indeg[t] += 1;
        out[s].push(t);
    }
    let mut layer = vec![0; n];
    let mut ready: Vec<usize> = (0..n).filter(|&v| indeg[v] == 0).collect();
    let mut k = 0;
    while k < ready.len() {
        let v = ready[k];
        k += 1;
        for &t in &out[v] {
            layer[t] = layer[t].max(layer[v] + 1);
            indeg[t] -= 1;
            if indeg[t] == 0 {
                ready.push(t);
            }
        }
    }
    for v in 0..n {
        let is_source = !edges.iter().any(|&(_, t)| t == v);
        if is_source && let Some(m) = out[v].iter().map(|&t| layer[t]).min() {
            layer[v] = m - 1;
        }
    }
    layer
}

fn crossings(rows: &[Vec<usize>], succ: &[Vec<usize>]) -> usize {
    let mut pos = vec![0; succ.len()];
    for row in rows {
        for (i, &v) in row.iter().enumerate() {
            pos[v] = i;
        }
    }
    let mut count = 0;
    for row in rows {
        let segs: Vec<(usize, usize)> = row
            .iter()
            .flat_map(|&v| succ[v].iter().map(move |&u| (v, u)))
            .map(|(v, u)| (pos[v], pos[u]))
            .collect();
        for (i, a) in segs.iter().enumerate() {
            for b in &segs[i + 1..] {
                if (a.0 < b.0 && a.1 > b.1) || (a.0 > b.0 && a.1 < b.1) {
                    count += 1;
                }
            }
        }
    }
    count
}

/// Barycenter sweeps, down then up, keeping the best order seen. A node
/// with no neighbors on the fixed side keeps its current slot.
fn order(rows: &mut [Vec<usize>], pred: &[Vec<usize>], succ: &[Vec<usize>]) {
    let mut best = rows.to_vec();
    let mut fewest = crossings(rows, succ);
    let mut pos = vec![0.0; succ.len()];
    let sync = |rows: &[Vec<usize>], pos: &mut [f64]| {
        for row in rows {
            for (i, &v) in row.iter().enumerate() {
                pos[v] = i as f64;
            }
        }
    };
    for _ in 0..SWEEPS {
        for pass in 0..2 {
            let ls: Vec<usize> = if pass == 0 {
                (1..rows.len()).collect()
            } else {
                (0..rows.len().saturating_sub(1)).rev().collect()
            };
            for l in ls {
                sync(rows, &mut pos);
                let nb = if pass == 0 { pred } else { succ };
                let key: Vec<f64> = rows[l]
                    .iter()
                    .map(|&v| {
                        if nb[v].is_empty() {
                            pos[v]
                        } else {
                            nb[v].iter().map(|&u| pos[u]).sum::<f64>() / nb[v].len() as f64
                        }
                    })
                    .collect();
                let mut idx: Vec<usize> = (0..rows[l].len()).collect();
                idx.sort_by(|&a, &b| key[a].total_cmp(&key[b]));
                rows[l] = idx.iter().map(|&i| rows[l][i]).collect();
            }
            let c = crossings(rows, succ);
            if c < fewest {
                fewest = c;
                best = rows.to_vec();
            }
        }
    }
    rows.clone_from_slice(&best);
}

/// The x for each node of `row` closest (least squares) to `want` that
/// keeps the row's order and separations: pool-adjacent-violators on the
/// positions with the separations subtracted out.
fn place(row: &[usize], want: &[f64], sep: &dyn Fn(usize, usize) -> f64, x: &mut [f64]) {
    let mut offset = Vec::with_capacity(row.len());
    let mut acc = 0.0;
    for (k, &v) in row.iter().enumerate() {
        if k > 0 {
            acc += sep(row[k - 1], v);
        }
        offset.push(acc);
    }
    // Blocks of (sum, count): each block's members share one shifted value.
    let mut blocks: Vec<(f64, usize)> = Vec::new();
    for k in 0..row.len() {
        blocks.push((want[k] - offset[k], 1));
        while blocks.len() > 1 {
            let (s1, c1) = blocks[blocks.len() - 1];
            let (s0, c0) = blocks[blocks.len() - 2];
            if s0 / c0 as f64 <= s1 / c1 as f64 {
                break;
            }
            blocks.pop();
            *blocks.last_mut().unwrap() = (s0 + s1, c0 + c1);
        }
    }
    let mut k = 0;
    for (s, c) in blocks {
        for _ in 0..c {
            x[row[k]] = s / c as f64 + offset[k];
            k += 1;
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn edges_point_down_and_only_loops_are_reversed() {
        // start → check → do → check (loop back), check → end.
        let e = [(0, 1), (1, 2), (2, 1), (1, 3)];
        let l = layered(&[40.0; 4], &e, 20.0);
        assert_eq!(l.reversed, [false, false, true, false]);
        for (&(s, t), &r) in e.iter().zip(&l.reversed) {
            let (hi, lo) = if r { (t, s) } else { (s, t) };
            assert!(l.layer[hi] < l.layer[lo], "{s}→{t}");
        }
        let dag = layered(&[40.0; 4], &[(0, 1), (0, 2), (1, 3), (2, 3)], 20.0);
        assert!(dag.reversed.iter().all(|r| !r));
    }

    #[test]
    fn edges_passing_through_a_layer_bundle_closer_than_nodes() {
        // Three long edges from 0 skip layer 1 beside node 1. Their bends
        // carry no box, so half a gap keeps them apart: a full one would
        // spread parallel lines as if they were steps.
        let e = [(0, 1), (1, 2), (1, 3), (1, 4), (0, 2), (0, 3), (0, 4)];
        let l = layered(&[20.0; 5], &e, 20.0);
        let mut xs: Vec<f64> = l.bends[4..].iter().map(|b| b[0]).collect();
        xs.sort_by(f64::total_cmp);
        let closest = xs
            .windows(2)
            .map(|w| w[1] - w[0])
            .fold(f64::INFINITY, f64::min);
        assert!((closest - 10.0).abs() < 1e-9, "{xs:?}");
    }

    #[test]
    fn a_layer_keeps_its_gaps() {
        let widths = [60.0, 30.0, 50.0, 40.0, 20.0];
        let e = [(0, 1), (0, 2), (0, 3), (0, 4)];
        let l = layered(&widths, &e, 20.0);
        let mut row: Vec<usize> = (1..5).collect();
        row.sort_by(|&a, &b| l.x[a].total_cmp(&l.x[b]));
        for p in row.windows(2) {
            let gap = (l.x[p[1]] - widths[p[1]] / 2.0) - (l.x[p[0]] + widths[p[0]] / 2.0);
            assert!(gap >= 20.0 - 1e-9, "{gap}");
        }
        // The parent sits over its children's mean: the least-squares
        // spot for the edges it sends them.
        let mean = row.iter().map(|&v| l.x[v]).sum::<f64>() / 4.0;
        assert!((l.x[0] - mean).abs() < 1e-9);
    }

    #[test]
    fn ordering_removes_an_avoidable_crossing() {
        // The depth-first start puts 0's children (2, then 3) left of
        // nothing of 1's, so 0 → 3 crosses 1 → 2; swapping 2 and 3 clears
        // it, and a reader should never have to untangle that.
        let e = [(0, 2), (0, 3), (1, 2)];
        let l = layered(&[20.0; 4], &e, 20.0);
        for (i, &(a, b)) in e.iter().enumerate() {
            for &(c, d) in &e[i + 1..] {
                if a != c && b != d {
                    assert_eq!(l.x[a] < l.x[c], l.x[b] < l.x[d], "{a}→{b} × {c}→{d}");
                }
            }
        }
    }

    #[test]
    fn a_long_edge_bends_through_each_layer_it_crosses() {
        // 0 → 1 → 2 → 3 and the shortcut 0 → 3.
        let l = layered(&[40.0; 4], &[(0, 1), (1, 2), (2, 3), (0, 3)], 20.0);
        assert_eq!(l.bends[3].len(), 2);
        assert!(l.bends[..3].iter().all(Vec::is_empty));
        // The shortcut routes beside the chain, not through its boxes.
        for (k, &bx) in l.bends[3].iter().enumerate() {
            assert!((bx - l.x[k + 1]).abs() >= 20.0 + 20.0 - 1e-9);
        }
    }

    #[test]
    fn a_decision_diamond_is_symmetric() {
        let l = layered(&[40.0; 4], &[(0, 1), (0, 2), (1, 3), (2, 3)], 20.0);
        assert!((l.x[0] - l.x[3]).abs() < 1e-9);
        assert!(((l.x[1] + l.x[2]) / 2.0 - l.x[0]).abs() < 1e-9);
    }
}
