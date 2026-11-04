//! Hierarchy layouts, ported from d3-hierarchy so the results match what
//! people expect from the reference implementation: squarified treemaps,
//! circle packing (front-chain placement plus Welzl's enclosing circle) and
//! Buchheim's tidy tree. Nodes live in an arena indexed in preorder.

/// A tree in preorder: node 0 is the root; every parent precedes its
/// children, and children are listed in order.
#[derive(Clone, Debug, Default)]
pub(crate) struct Tree {
    pub parent: Vec<Option<usize>>,
    pub children: Vec<Vec<usize>>,
    pub depth: Vec<usize>,
    /// Leaf values, and for inner nodes the sum of their leaves.
    pub value: Vec<f64>,
}

impl Tree {
    pub fn len(&self) -> usize {
        self.parent.len()
    }

    /// Adds a node under `parent` (None for the root) and returns its index.
    pub fn push(&mut self, parent: Option<usize>, value: f64) -> usize {
        let i = self.len();
        self.parent.push(parent);
        self.children.push(Vec::new());
        self.depth.push(parent.map_or(0, |p| self.depth[p] + 1));
        self.value.push(value);
        if let Some(p) = parent {
            self.children[p].push(i);
        }
        i
    }

    /// Sets every inner node's value to the sum of its children's, in
    /// reverse preorder so children are summed before their parents.
    pub fn sum(&mut self) {
        for i in (0..self.len()).rev() {
            if !self.children[i].is_empty() {
                self.value[i] = self.children[i].iter().map(|&c| self.value[c]).sum();
            }
        }
    }

    /// Orders every node's children by value, largest first (stable), as
    /// treemaps and packs lay out best that way.
    pub fn sort_by_value(&mut self) {
        for i in 0..self.len() {
            let mut c = std::mem::take(&mut self.children[i]);
            c.sort_by(|&a, &b| self.value[b].total_cmp(&self.value[a]));
            self.children[i] = c;
        }
    }

    /// Nodes in post-order, children left to right before their parent.
    fn post_order(&self) -> Vec<usize> {
        let mut out = Vec::with_capacity(self.len());
        let mut stack = vec![(0usize, 0usize)];
        while let Some(&mut (n, ref mut k)) = stack.last_mut() {
            if *k < self.children[n].len() {
                let c = self.children[n][*k];
                *k += 1;
                stack.push((c, 0));
            } else {
                out.push(n);
                stack.pop();
            }
        }
        out
    }

    /// Nodes in preorder by traversal (which differs from index order once
    /// children have been re-sorted).
    fn pre_order(&self) -> Vec<usize> {
        let mut out = Vec::with_capacity(self.len());
        let mut stack = vec![0usize];
        while let Some(n) = stack.pop() {
            out.push(n);
            stack.extend(self.children[n].iter().rev());
        }
        out
    }
}

/// The golden ratio d3 targets for treemap cell aspect ratios.
const PHI: f64 = 1.618_033_988_749_895;

/// Squarified treemap of one level: rectangles `(x0, y0, x1, y1)` for
/// `values` (in order; zeros get empty rectangles) tiling the given one.
pub(crate) fn squarify(
    values: &[f64],
    (mut x0, mut y0, x1, y1): (f64, f64, f64, f64),
) -> Vec<(f64, f64, f64, f64)> {
    let n = values.len();
    let mut out = vec![(x0, y0, x0, y0); n];
    let mut value: f64 = values.iter().sum();
    let (mut i0, mut i1) = (0, 0);
    while i0 < n {
        let (dx, dy) = (x1 - x0, y1 - y0);
        // Find the next non-empty node.
        let mut sum_value;
        loop {
            sum_value = values[i1];
            i1 += 1;
            if sum_value != 0.0 || i1 >= n {
                break;
            }
        }
        let (mut min_value, mut max_value) = (sum_value, sum_value);
        let alpha = (dy / dx).max(dx / dy) / (value * PHI);
        let mut beta = sum_value * sum_value * alpha;
        let mut min_ratio = (max_value / beta).max(beta / min_value);
        // Keep adding nodes while the aspect ratio maintains or improves.
        while i1 < n {
            let node_value = values[i1];
            sum_value += node_value;
            if node_value < min_value {
                min_value = node_value;
            }
            if node_value > max_value {
                max_value = node_value;
            }
            beta = sum_value * sum_value * alpha;
            let new_ratio = (max_value / beta).max(beta / min_value);
            if new_ratio > min_ratio {
                sum_value -= node_value;
                break;
            }
            min_ratio = new_ratio;
            i1 += 1;
        }
        let row = &values[i0..i1];
        if dx < dy {
            // Dice: the row runs across the top.
            let y = if value != 0.0 {
                y0 + dy * sum_value / value
            } else {
                y1
            };
            let k = if sum_value != 0.0 {
                (x1 - x0) / sum_value
            } else {
                0.0
            };
            let mut x = x0;
            for (j, &v) in row.iter().enumerate() {
                let nx = x + v * k;
                out[i0 + j] = (x, y0, nx, y);
                x = nx;
            }
            y0 = y;
        } else {
            // Slice: the row runs down the left side.
            let x = if value != 0.0 {
                x0 + dx * sum_value / value
            } else {
                x1
            };
            let k = if sum_value != 0.0 {
                (y1 - y0) / sum_value
            } else {
                0.0
            };
            let mut y = y0;
            for (j, &v) in row.iter().enumerate() {
                let ny = y + v * k;
                out[i0 + j] = (x0, y, x, ny);
                y = ny;
            }
            x0 = x;
        }
        value -= sum_value;
        i0 = i1;
    }
    out
}

#[derive(Clone, Copy, Debug, Default, PartialEq)]
pub(crate) struct Circle {
    pub x: f64,
    pub y: f64,
    pub r: f64,
}

/// d3's linear congruential generator for the enclosing circle's shuffle
/// (and the force layout's start positions):
/// the shuffle only guards against worst-case input order, so a fixed
/// sequence keeps layouts reproducible across runs and ports.
pub(crate) struct Lcg(pub(crate) u64);

impl Lcg {
    pub(crate) fn next(&mut self) -> f64 {
        self.0 = (1_664_525 * self.0 + 1_013_904_223) % 4_294_967_296;
        self.0 as f64 / 4_294_967_296.0
    }
}

fn place(b: Circle, a: Circle, c: &mut Circle) {
    let (dx, dy) = (b.x - a.x, b.y - a.y);
    let d2 = dx * dx + dy * dy;
    if d2 != 0.0 {
        let a2 = (a.r + c.r) * (a.r + c.r);
        let b2 = (b.r + c.r) * (b.r + c.r);
        if a2 > b2 {
            let x = (d2 + b2 - a2) / (2.0 * d2);
            let y = (b2 / d2 - x * x).max(0.0).sqrt();
            c.x = b.x - x * dx - y * dy;
            c.y = b.y - x * dy + y * dx;
        } else {
            let x = (d2 + a2 - b2) / (2.0 * d2);
            let y = (a2 / d2 - x * x).max(0.0).sqrt();
            c.x = a.x + x * dx - y * dy;
            c.y = a.y + x * dy + y * dx;
        }
    } else {
        c.x = a.x + c.r;
        c.y = a.y;
    }
}

fn intersects(a: Circle, b: Circle) -> bool {
    let dr = a.r + b.r - 1e-6;
    let (dx, dy) = (b.x - a.x, b.y - a.y);
    dr > 0.0 && dr * dr > dx * dx + dy * dy
}

/// Packs `circles` (radii given) side by side around the origin; returns
/// the radius of the circle enclosing them all, centered on the origin.
fn pack_siblings(circles: &mut [Circle], rng: &mut Lcg) -> f64 {
    let n = circles.len();
    if n == 0 {
        return 0.0;
    }
    circles[0].x = 0.0;
    circles[0].y = 0.0;
    if n == 1 {
        return circles[0].r;
    }
    circles[0].x = -circles[1].r;
    circles[1].x = circles[0].r;
    circles[1].y = 0.0;
    if n == 2 {
        return circles[0].r + circles[1].r;
    }
    let (c0, c1) = (circles[0], circles[1]);
    place(c1, c0, &mut circles[2]);
    // The front chain as a circular doubly linked list over indices.
    let mut next = vec![0usize; n];
    let mut prev = vec![0usize; n];
    let (mut a, mut b) = (0usize, 1usize);
    next[0] = 1;
    prev[1] = 0;
    next[1] = 2;
    prev[2] = 1;
    next[2] = 0;
    prev[0] = 2;
    let score = |circles: &[Circle], next: &[usize], i: usize| {
        let (p, q) = (circles[i], circles[next[i]]);
        let ab = p.r + q.r;
        let dx = (p.x * q.r + q.x * p.r) / ab;
        let dy = (p.y * q.r + q.y * p.r) / ab;
        dx * dx + dy * dy
    };
    let mut i = 3;
    'pack: while i < n {
        let (ca, cb) = (circles[a], circles[b]);
        place(ca, cb, &mut circles[i]);
        let c = i;
        // Find the closest intersecting circle on the front chain, if any.
        let (mut j, mut k) = (next[b], prev[a]);
        let (mut sj, mut sk) = (circles[b].r, circles[a].r);
        loop {
            if sj <= sk {
                if intersects(circles[j], circles[c]) {
                    b = j;
                    next[a] = b;
                    prev[b] = a;
                    continue 'pack;
                }
                sj += circles[j].r;
                j = next[j];
            } else {
                if intersects(circles[k], circles[c]) {
                    a = k;
                    next[a] = b;
                    prev[b] = a;
                    continue 'pack;
                }
                sk += circles[k].r;
                k = prev[k];
            }
            if j == next[k] {
                break;
            }
        }
        // Insert c between a and b.
        prev[c] = a;
        next[c] = b;
        next[a] = c;
        prev[b] = c;
        b = c;
        // The new closest pair to the centroid.
        let mut aa = score(circles, &next, a);
        let mut d = next[c];
        while d != b {
            let ca = score(circles, &next, d);
            if ca < aa {
                a = d;
                aa = ca;
            }
            d = next[d];
        }
        b = next[a];
        i += 1;
    }
    // Enclose the front chain and center it on the origin.
    let mut chain = vec![circles[b]];
    let mut c = next[b];
    while c != b {
        chain.push(circles[c]);
        c = next[c];
    }
    let e = enclose(chain, rng);
    for c in circles.iter_mut() {
        c.x -= e.x;
        c.y -= e.y;
    }
    e.r
}

fn encloses_not(a: Circle, b: Circle) -> bool {
    let dr = a.r - b.r;
    let (dx, dy) = (b.x - a.x, b.y - a.y);
    dr < 0.0 || dr * dr < dx * dx + dy * dy
}

fn encloses_weak(a: Circle, b: Circle) -> bool {
    let dr = a.r - b.r + a.r.max(b.r).max(1.0) * 1e-9;
    let (dx, dy) = (b.x - a.x, b.y - a.y);
    dr > 0.0 && dr * dr > dx * dx + dy * dy
}

fn encloses_weak_all(a: Circle, basis: &[Circle]) -> bool {
    basis.iter().all(|&b| encloses_weak(a, b))
}

fn basis2(a: Circle, b: Circle) -> Circle {
    let (x21, y21, r21) = (b.x - a.x, b.y - a.y, b.r - a.r);
    let l = (x21 * x21 + y21 * y21).sqrt();
    Circle {
        x: (a.x + b.x + x21 / l * r21) / 2.0,
        y: (a.y + b.y + y21 / l * r21) / 2.0,
        r: (l + a.r + b.r) / 2.0,
    }
}

fn basis3(a: Circle, b: Circle, c: Circle) -> Circle {
    let (x1, y1, r1) = (a.x, a.y, a.r);
    let (x2, y2, r2) = (b.x, b.y, b.r);
    let (x3, y3, r3) = (c.x, c.y, c.r);
    let (a2, a3) = (x1 - x2, x1 - x3);
    let (b2, b3) = (y1 - y2, y1 - y3);
    let (c2, c3) = (r2 - r1, r3 - r1);
    let d1 = x1 * x1 + y1 * y1 - r1 * r1;
    let d2 = d1 - x2 * x2 - y2 * y2 + r2 * r2;
    let d3 = d1 - x3 * x3 - y3 * y3 + r3 * r3;
    let ab = a3 * b2 - a2 * b3;
    let xa = (b2 * d3 - b3 * d2) / (ab * 2.0) - x1;
    let xb = (b3 * c2 - b2 * c3) / ab;
    let ya = (a3 * d2 - a2 * d3) / (ab * 2.0) - y1;
    let yb = (a2 * c3 - a3 * c2) / ab;
    let qa = xb * xb + yb * yb - 1.0;
    let qb = 2.0 * (r1 + xa * xb + ya * yb);
    let qc = xa * xa + ya * ya - r1 * r1;
    let r = -(if qa.abs() > 1e-6 {
        (qb + (qb * qb - 4.0 * qa * qc).sqrt()) / (2.0 * qa)
    } else {
        qc / qb
    });
    Circle {
        x: x1 + xa + xb * r,
        y: y1 + ya + yb * r,
        r,
    }
}

fn basis(b: &[Circle]) -> Circle {
    match *b {
        [a] => a,
        [a, c] => basis2(a, c),
        [a, c, d] => basis3(a, c, d),
        _ => unreachable!("a basis has one to three circles"),
    }
}

fn extend_basis(b: &[Circle], p: Circle) -> Vec<Circle> {
    if encloses_weak_all(p, b) {
        return vec![p];
    }
    for &bi in b {
        if encloses_not(p, bi) && encloses_weak_all(basis2(bi, p), b) {
            return vec![bi, p];
        }
    }
    for i in 0..b.len().saturating_sub(1) {
        for j in i + 1..b.len() {
            if encloses_not(basis2(b[i], b[j]), p)
                && encloses_not(basis2(b[i], p), b[j])
                && encloses_not(basis2(b[j], p), b[i])
                && encloses_weak_all(basis3(b[i], b[j], p), b)
            {
                return vec![b[i], b[j], p];
            }
        }
    }
    unreachable!("Welzl's basis always extends")
}

/// The smallest circle enclosing `circles` (Welzl's algorithm, after a
/// shuffle).
fn enclose(mut circles: Vec<Circle>, rng: &mut Lcg) -> Circle {
    let mut m = circles.len();
    while m > 0 {
        let i = (rng.next() * m as f64) as usize;
        m -= 1;
        circles.swap(m, i);
    }
    let mut b: Vec<Circle> = Vec::new();
    let mut e: Option<Circle> = None;
    let mut i = 0;
    while i < circles.len() {
        let p = circles[i];
        match e {
            Some(c) if encloses_weak(c, p) => i += 1,
            _ => {
                b = extend_basis(&b, p);
                e = Some(basis(&b));
                i = 0;
            }
        }
    }
    e.unwrap_or_default()
}

/// Circle packing of the whole tree into a `w` × `h` box: leaf areas
/// proportional to value, about `padding` px between siblings and around
/// them inside their parent. Values must be summed first. Only about: as
/// in d3, the padding is converted to layout units with the scale of an
/// unpadded first pass, and padding changes that scale (10px comes out
/// near 9px).
pub(crate) fn pack(tree: &Tree, w: f64, h: f64, padding: f64) -> Vec<Circle> {
    // Leaf radii start as √value, so areas follow values.
    let mut c: Vec<Circle> = (0..tree.len())
        .map(|i| Circle {
            r: if tree.children[i].is_empty() {
                tree.value[i].max(0.0).sqrt()
            } else {
                0.0
            },
            ..Circle::default()
        })
        .collect();
    let mut rng = Lcg(1);
    let post = tree.post_order();
    let mut pack_children = |c: &mut Vec<Circle>, k: f64, pad: f64| {
        for &i in &post {
            let kids = &tree.children[i];
            if kids.is_empty() {
                continue;
            }
            let r = pad * k;
            let mut sib: Vec<Circle> = kids.iter().map(|&k| c[k]).collect();
            if r != 0.0 {
                sib.iter_mut().for_each(|s| s.r += r);
            }
            let e = pack_siblings(&mut sib, &mut rng);
            if r != 0.0 {
                sib.iter_mut().for_each(|s| s.r -= r);
            }
            for (&k, s) in kids.iter().zip(sib) {
                c[k] = s;
            }
            c[i].r = e + r;
        }
    };
    pack_children(&mut c, 1.0, 0.0);
    let k = c[0].r / w.min(h);
    pack_children(&mut c, k, padding);
    let k = w.min(h) / (2.0 * c[0].r);
    c[0].x = w / 2.0;
    c[0].y = h / 2.0;
    for i in tree.pre_order() {
        c[i].r *= k;
        if let Some(p) = tree.parent[i] {
            c[i].x = c[p].x + k * c[i].x;
            c[i].y = c[p].y + k * c[i].y;
        }
    }
    c
}

/// Buchheim's linear-time tidy tree (Reingold–Tilford): each node's
/// breadth position in `[0, 1]`, subtrees never overlapping, parents
/// centered over their children. `separation(same_parent, depth)` is the
/// minimum gap between neighbours, in units of the tightest gap.
pub(crate) fn tidy(tree: &Tree, separation: impl Fn(bool, usize) -> f64) -> Vec<f64> {
    let n = tree.len();
    if n == 0 {
        return Vec::new();
    }
    // Node n is a pseudo-root above the real one, as in d3.
    let parent: Vec<usize> = (0..n).map(|i| tree.parent[i].unwrap_or(n)).collect();
    let kids = |v: usize| -> &[usize] {
        if v == n {
            std::slice::from_ref(&0)
        } else {
            &tree.children[v]
        }
    };
    let mut index = vec![0usize; n];
    for v in 0..n {
        for (k, &c) in tree.children[v].iter().enumerate() {
            index[c] = k;
        }
    }
    let mut ancestor_of: Vec<usize> = (0..=n).collect(); // `a`
    let mut big_a: Vec<Option<usize>> = vec![None; n + 1]; // `A`
    let mut thread: Vec<Option<usize>> = vec![None; n + 1]; // `t`
    let (mut z, mut m, mut c, mut s) = (
        vec![0.0; n + 1],
        vec![0.0; n + 1],
        vec![0.0; n + 1],
        vec![0.0; n + 1],
    );
    let sep = |a: usize, b: usize| separation(parent[a] == parent[b], tree.depth[a]);
    let next_left = |v: usize, thread: &[Option<usize>]| -> Option<usize> {
        kids(v).first().copied().or(thread[v])
    };
    let next_right = |v: usize, thread: &[Option<usize>]| -> Option<usize> {
        kids(v).last().copied().or(thread[v])
    };

    for v in tree.post_order() {
        let siblings = kids(parent[v]);
        let w = if index[v] > 0 {
            Some(siblings[index[v] - 1])
        } else {
            None
        };
        let ch = kids(v);
        if !ch.is_empty() {
            // Execute shifts.
            let (mut shift, mut change) = (0.0, 0.0);
            for &x in ch.iter().rev() {
                z[x] += shift;
                m[x] += shift;
                change += c[x];
                shift += s[x] + change;
            }
            let midpoint = (z[ch[0]] + z[ch[ch.len() - 1]]) / 2.0;
            if let Some(w) = w {
                z[v] = z[w] + sep(v, w);
                m[v] = z[v] - midpoint;
            } else {
                z[v] = midpoint;
            }
        } else if let Some(w) = w {
            z[v] = z[w] + sep(v, w);
        }
        // Apportion.
        let mut ancestor = big_a[parent[v]].unwrap_or(siblings[0]);
        if let Some(w) = w {
            let (mut vip, mut vop, mut vim) = (v, v, w);
            let mut vom = siblings[0];
            let (mut sip, mut sop, mut sim, mut som) = (m[vip], m[vop], m[vim], m[vom]);
            let mut nim = next_right(vim, &thread);
            let mut nip = next_left(vip, &thread);
            while let (Some(a), Some(b)) = (nim, nip) {
                vim = a;
                vip = b;
                vom = next_left(vom, &thread).expect("left contour");
                vop = next_right(vop, &thread).expect("right contour");
                ancestor_of[vop] = v;
                let shift = z[vim] + sim - z[vip] - sip + sep(vim, vip);
                if shift > 0.0 {
                    let wm = if parent[ancestor_of[vim]] == parent[v] {
                        ancestor_of[vim]
                    } else {
                        ancestor
                    };
                    // Move subtree.
                    let change = shift / (index[v] as f64 - index[wm] as f64);
                    c[v] -= change;
                    s[v] += shift;
                    c[wm] += change;
                    z[v] += shift;
                    m[v] += shift;
                    sip += shift;
                    sop += shift;
                }
                sim += m[vim];
                sip += m[vip];
                som += m[vom];
                sop += m[vop];
                nim = next_right(vim, &thread);
                nip = next_left(vip, &thread);
            }
            if nim.is_some() && next_right(vop, &thread).is_none() {
                thread[vop] = nim;
                m[vop] += sim - sop;
            }
            if nip.is_some() && next_left(vom, &thread).is_none() {
                thread[vom] = nip;
                m[vom] += sip - som;
                ancestor = v;
            }
        }
        big_a[parent[v]] = Some(ancestor);
    }
    m[n] = -z[0];
    let mut x = vec![0.0; n];
    for v in tree.pre_order() {
        x[v] = z[v] + m[parent[v]];
        m[v] += m[parent[v]];
    }
    // Normalize to [0, 1], leaving half a separation at either end.
    let (mut left, mut right) = (0, 0);
    for v in tree.pre_order() {
        if x[v] < x[left] {
            left = v;
        }
        if x[v] > x[right] {
            right = v;
        }
    }
    let sp = if left == right {
        1.0
    } else {
        sep(left, right) / 2.0
    };
    let tx = sp - x[left];
    let kx = 1.0 / (x[right] + sp + tx);
    x.iter().map(|&v| (v + tx) * kx).collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn tree(spec: &[(Option<usize>, f64)]) -> Tree {
        let mut t = Tree::default();
        for &(p, v) in spec {
            t.push(p, v);
        }
        t.sum();
        t
    }

    #[test]
    fn squarified_cells_tile_the_rect_with_areas_proportional_to_value() {
        let v = [6.0, 6.0, 4.0, 3.0, 2.0, 2.0, 1.0];
        let r = squarify(&v, (0.0, 0.0, 600.0, 400.0));
        let total: f64 = v.iter().sum();
        let mut area = 0.0;
        for (k, &(x0, y0, x1, y1)) in r.iter().enumerate() {
            let a = (x1 - x0) * (y1 - y0);
            assert!((a / (600.0 * 400.0) - v[k] / total).abs() < 1e-9, "{k}");
            assert!(x0 >= -1e-9 && y0 >= -1e-9 && x1 <= 600.0 + 1e-9 && y1 <= 400.0 + 1e-9);
            area += a;
            for &(u0, v0, u1, v1) in &r[..k] {
                let overlap =
                    (x1.min(u1) - x0.max(u0)).max(0.0) * (y1.min(v1) - y0.max(v0)).max(0.0);
                assert!(overlap < 1e-6, "cells {k} overlap");
            }
        }
        assert!((area - 600.0 * 400.0).abs() < 1e-6, "fills the rect");
    }

    #[test]
    fn squarified_cells_stay_close_to_square() {
        // Bruls et al.'s example: the worst aspect ratio stays small, where
        // slicing in one direction would give slivers.
        let r = squarify(&[6.0, 6.0, 4.0, 3.0, 2.0, 2.0, 1.0], (0.0, 0.0, 6.0, 4.0));
        let worst = r
            .iter()
            .map(|&(x0, y0, x1, y1)| {
                let (w, h) = (x1 - x0, y1 - y0);
                w.max(h) / w.min(h)
            })
            .fold(0.0, f64::max);
        assert!(worst < 3.0, "{worst}");
    }

    #[test]
    fn packed_circles_never_overlap_and_stay_inside_their_parent() {
        let mut spec = vec![(None, 0.0)];
        for g in 0..3 {
            spec.push((Some(0), 0.0));
            let p = spec.len() - 1;
            for k in 0..(4 + g * 3) {
                spec.push((Some(p), 1.0 + (k * 7 % 5) as f64));
            }
        }
        let mut t = tree(&spec);
        t.sort_by_value();
        let c = pack(&t, 500.0, 400.0, 3.0);
        assert!((c[0].r - 200.0).abs() < 1e-9 && (c[0].x - 250.0).abs() < 1e-9);
        for i in 1..t.len() {
            let p = c[t.parent[i].unwrap()];
            let d = ((c[i].x - p.x).powi(2) + (c[i].y - p.y).powi(2)).sqrt();
            assert!(d + c[i].r <= p.r + 1e-6, "{i} pokes out of its parent");
            for &j in &t.children[t.parent[i].unwrap()] {
                if j < i {
                    let d = ((c[i].x - c[j].x).powi(2) + (c[i].y - c[j].y).powi(2)).sqrt();
                    assert!(d >= c[i].r + c[j].r - 1e-6, "{i} overlaps {j}");
                }
            }
        }
    }

    #[test]
    fn packed_leaf_areas_are_proportional_to_value() {
        let t = tree(&[(None, 0.0), (Some(0), 1.0), (Some(0), 4.0), (Some(0), 9.0)]);
        let c = pack(&t, 300.0, 300.0, 0.0);
        assert!((c[2].r / c[1].r - 2.0).abs() < 1e-9);
        assert!((c[3].r / c[1].r - 3.0).abs() < 1e-9);
    }

    #[test]
    fn the_enclosing_circle_is_the_smallest_one() {
        // Two equal circles side by side: the enclosure spans exactly both.
        let e = enclose(
            vec![
                Circle {
                    x: -1.0,
                    y: 0.0,
                    r: 1.0,
                },
                Circle {
                    x: 1.0,
                    y: 0.0,
                    r: 1.0,
                },
                Circle {
                    x: 0.0,
                    y: 0.0,
                    r: 0.5,
                },
            ],
            &mut Lcg(1),
        );
        assert!((e.r - 2.0).abs() < 1e-9 && e.x.abs() < 1e-9 && e.y.abs() < 1e-9);
    }

    #[test]
    fn tidy_trees_center_parents_and_keep_neighbours_apart() {
        //        0
        //     /  |  \
        //    1   4   5
        //   / \      |
        //  2   3     6
        let t = tree(&[
            (None, 0.0),
            (Some(0), 0.0),
            (Some(1), 1.0),
            (Some(1), 1.0),
            (Some(0), 1.0),
            (Some(0), 0.0),
            (Some(5), 1.0),
        ]);
        let x = tidy(&t, |same, _| if same { 1.0 } else { 2.0 });
        assert!(
            (x[1] - (x[2] + x[3]) / 2.0).abs() < 1e-9,
            "parent over children"
        );
        assert!((x[0] - (x[1] + x[5]) / 2.0).abs() < 1e-9);
        let unit = x[3] - x[2];
        // Same depth, different parents: exactly two units apart, as close
        // as allowed (a tidy tree is compact, not merely non-overlapping);
        // siblings at least one.
        assert!((x[6] - x[3] - 2.0 * unit).abs() < 1e-9, "{x:?}");
        assert!(x[4] - x[1] >= unit - 1e-9);
        assert!(x.iter().all(|&v| (0.0..=1.0).contains(&v)));
    }

    #[test]
    fn tidy_spreads_a_small_subtree_between_big_ones() {
        // Root with three children; the outer two have wide subtrees. The
        // middle leaf is placed evenly between them, not crammed left.
        let mut spec = vec![(None, 0.0), (Some(0), 0.0)];
        for _ in 0..4 {
            spec.push((Some(1), 1.0));
        }
        spec.push((Some(0), 1.0)); // 6: the middle leaf
        spec.push((Some(0), 0.0)); // 7
        for _ in 0..4 {
            spec.push((Some(7), 1.0));
        }
        let t = tree(&spec);
        let x = tidy(&t, |same, _| if same { 1.0 } else { 2.0 });
        assert!(((x[6] - x[1]) - (x[7] - x[6])).abs() < 1e-9, "{x:?}");
    }

    #[test]
    fn tidy_trees_are_compact() {
        // Reingold–Tilford places each subtree as close to its elder
        // siblings as separation allows, so the last child's subtree
        // touches them at some depth. Mistakes in the contour threads only
        // ever leave extra room, which no overlap check notices.
        let mut rng = Lcg(7);
        for _ in 0..50 {
            let n = 2 + (rng.next() * 30.0) as usize;
            let mut spec = vec![(None, 1.0)];
            for i in 1..n {
                spec.push((Some((rng.next() * i as f64) as usize), 1.0));
            }
            let t = tree(&spec);
            let sep = |a: usize, b: usize| if t.parent[a] == t.parent[b] { 1.0 } else { 2.0 };
            let x = tidy(&t, |same, _| if same { 1.0 } else { 2.0 });
            let mut depth = vec![0; n];
            for i in t.pre_order() {
                if let Some(p) = t.parent[i] {
                    depth[i] = depth[p] + 1;
                }
            }
            let under = |a: usize, top: usize| {
                let mut v = Some(a);
                while let Some(u) = v {
                    if u == top {
                        return true;
                    }
                    v = t.parent[u];
                }
                false
            };
            // The tightest gap, in separation units, is the layout's unit.
            let mut unit = f64::INFINITY;
            for a in 0..n {
                for b in 0..n {
                    if a != b && depth[a] == depth[b] && x[b] > x[a] {
                        unit = unit.min((x[b] - x[a]) / sep(a, b));
                    }
                }
            }
            if unit.is_infinite() {
                continue;
            }
            assert!(unit > 0.0);
            for p in 0..n {
                let kids = &t.children[p];
                let Some(&last) = kids.last().filter(|_| kids.len() > 1) else {
                    continue;
                };
                let mut closest = f64::INFINITY;
                for a in (0..n).filter(|&a| kids[..kids.len() - 1].iter().any(|&k| under(a, k))) {
                    for b in (0..n).filter(|&b| under(b, last) && depth[b] == depth[a]) {
                        closest = closest.min((x[b] - x[a]) / sep(a, b));
                    }
                }
                assert!((closest - unit).abs() < 1e-9, "{spec:?}: node {p}");
            }
        }
    }

    #[test]
    fn tidy_shares_the_push_among_every_sibling_in_between() {
        // Two leaves between two wide subtrees: pushing the right subtree
        // away must space all four children evenly. With three children the
        // single middle one would hide an uneven split.
        let mut spec = vec![(None, 0.0), (Some(0), 0.0)];
        for _ in 0..4 {
            spec.push((Some(1), 1.0));
        }
        spec.push((Some(0), 1.0)); // 6
        spec.push((Some(0), 1.0)); // 7
        spec.push((Some(0), 0.0)); // 8
        for _ in 0..4 {
            spec.push((Some(8), 1.0));
        }
        let t = tree(&spec);
        let x = tidy(&t, |same, _| if same { 1.0 } else { 2.0 });
        let gaps = [x[6] - x[1], x[7] - x[6], x[8] - x[7]];
        assert!(
            (gaps[0] - gaps[1]).abs() < 1e-9 && (gaps[1] - gaps[2]).abs() < 1e-9,
            "{gaps:?}"
        );
    }
}
