use super::frame::{self, Key, halo_label};
use super::graph::{Graph, Link, graph, links_chart};
use super::{
    Chart, DOT_RADIUS, GAP, HAIRLINE, PAD, SURFACE_GAP, TICK_SIZE, check_size, count, describe,
};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::layout::force::force;
use crate::scene::{Anchor, Hit, Mark, Scene, Shape, TipRow};

/// The most connected node is drawn this large; the least, `DOT_RADIUS`.
const MAX_RADIUS: f64 = 10.0;
/// Past this many nodes labels would pile up; tooltips name them instead.
const MAX_LABELS: usize = 30;
/// Barnes–Hut keeps the force layout near n log n per tick, but past this a
/// network takes long enough to lay out to notice, and is a hairball
/// anyway: filter or aggregate first.
const MAX_NODES: usize = 2000;
/// Small graphs are not blown up to fill the plot: two nodes at opposite
/// corners read as unrelated.
const MAX_ZOOM: f64 = 3.0;

links_chart!(
    /// Nodes placed by a force simulation: linked nodes pull together,
    /// all nodes push apart, so clusters and bridges show.
    NetworkChart,
    false,
    network
);

fn network(
    title: Option<&str>,
    g: &Graph,
    width: f64,
    height: f64,
    measure: &dyn TextMeasure,
) -> Result<Scene, Error> {
    let n = g.names.len();
    if n > MAX_NODES {
        return Err(Error::InvalidData(format!(
            "{n} nodes, at most {MAX_NODES}: filter or aggregate the network first"
        )));
    }
    let mut marks = Vec::new();
    let mut top = frame::title(&mut marks, title);
    top = frame::legend(&mut marks, measure, top, width, &g.legend(), Key::Swatch);

    let edges: Vec<(usize, usize)> = g.edges.iter().map(|e| (e.0, e.1)).collect();
    let p = force(n, &edges);
    let strength = g.strength();
    let max_s = strength.iter().copied().fold(0.0, f64::max);
    let radius: Vec<f64> = strength
        .iter()
        .map(|&s| DOT_RADIUS + (MAX_RADIUS - DOT_RADIUS) * (s / max_s).sqrt())
        .collect();
    let labeled = n <= MAX_LABELS;
    let label_w = if labeled {
        g.names
            .iter()
            .map(|s| measure.width(s, TICK_SIZE, Weight::Regular))
            .fold(0.0, f64::max)
            + GAP / 2.0
    } else {
        0.0
    };

    // Fit the layout's bounding box into the plot, uniformly, leaving room
    // for the biggest dot all round and for labels on the right.
    let (left, right) = (PAD + MAX_RADIUS, width - PAD - MAX_RADIUS - label_w);
    let (upper, lower) = (top + MAX_RADIUS, height - PAD - MAX_RADIUS);
    let (mut x0, mut x1, mut y0, mut y1) = (
        f64::INFINITY,
        f64::NEG_INFINITY,
        f64::INFINITY,
        f64::NEG_INFINITY,
    );
    for &(x, y) in &p {
        x0 = x0.min(x);
        x1 = x1.max(x);
        y0 = y0.min(y);
        y1 = y1.max(y);
    }
    let (bw, bh) = ((x1 - x0).max(1.0), (y1 - y0).max(1.0));
    let zoom = f64::min(
        ((right - left).max(1.0) / bw).min((lower - upper).max(1.0) / bh),
        MAX_ZOOM,
    );
    let (ox, oy) = (
        (left + right) / 2.0 - (x0 + x1) / 2.0 * zoom,
        (upper + lower) / 2.0 - (y0 + y1) / 2.0 * zoom,
    );
    let pos: Vec<(f64, f64)> = p
        .iter()
        .map(|&(x, y)| (ox + x * zoom, oy + y * zoom))
        .collect();
    let series = |i: usize| g.colored().then(|| g.names[i].clone());
    let max_v = g.edges.iter().map(|e| e.2).fold(0.0, f64::max);

    // Links stay neutral and recede: the nodes carry identity.
    for (k, &(a, b, v)) in g.edges.iter().enumerate() {
        let mut m = Mark::new(Shape::Line {
            x1: pos[a].0,
            y1: pos[a].1,
            x2: pos[b].0,
            y2: pos[b].1,
        })
        .stroke(Color::Axis, HAIRLINE + 2.0 * v / max_v)
        .hit(Hit::new(
            format!("link-{k}"),
            format!("{} – {}", g.names[a], g.names[b]),
            format::value(v),
        ));
        if let Some(s) = series(a) {
            m = m.series(s);
        }
        marks.push(m);
    }
    for (i, name) in g.names.iter().enumerate() {
        let links = edges.iter().filter(|e| e.0 == i || e.1 == i).count();
        let mut m = Mark::new(Shape::Circle {
            cx: pos[i].0,
            cy: pos[i].1,
            r: radius[i],
        })
        .fill(Color::Series(g.color(i)))
        .stroke(Color::Surface, SURFACE_GAP)
        .hit(
            Hit::new(
                format!("node-{i}"),
                name.as_str(),
                format::value(strength[i]),
            )
            .rows(vec![TipRow {
                color: None,
                name: "links".into(),
                value: links.to_string(),
            }]),
        );
        if let Some(s) = series(i) {
            m = m.series(s);
        }
        marks.push(m);
    }
    if labeled {
        for (i, name) in g.names.iter().enumerate() {
            halo_label(
                &mut marks,
                measure,
                (pos[i].0 + radius[i] + GAP / 2.0, pos[i].1),
                name,
                Anchor::Start,
                Color::TextSecondary,
            );
        }
    }
    Ok(Scene {
        width,
        height,
        label: describe(
            title,
            &format!(
                "Network diagram of {} and {}.",
                count(n, "node", "nodes"),
                count(g.edges.len(), "link", "links")
            ),
        ),
        marks,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;
    use crate::scene::HitKind;

    fn dots(s: &Scene) -> Vec<(f64, f64, f64, String)> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Circle { cx, cy, r }, Some(h)) if h.kind == HitKind::Mark => {
                    Some((*cx, *cy, *r, h.label.clone()))
                }
                _ => None,
            })
            .collect()
    }

    fn club() -> NetworkChart {
        let mut c = NetworkChart::new();
        for (a, b) in [
            ("Ann", "Bo"),
            ("Ann", "Cy"),
            ("Bo", "Cy"),
            ("Cy", "Di"),
            ("Di", "Ed"),
            ("Di", "Flo"),
            ("Ed", "Flo"),
        ] {
            c = c.link(a, b, 1.0);
        }
        c
    }

    #[test]
    fn every_dot_and_label_stays_inside_the_chart() {
        let s = club().scene(480.0, 320.0, &InterMetrics).unwrap();
        for (x, y, r, name) in dots(&s) {
            assert!(x - r >= PAD - 1e-6 && y + r <= 320.0 - PAD + 1e-6, "{name}");
            let w = InterMetrics.width(&name, TICK_SIZE, Weight::Regular);
            assert!(
                x + r + GAP / 2.0 + w <= 480.0 - PAD + 1e-6,
                "{name}'s label"
            );
        }
    }

    #[test]
    fn the_best_connected_node_is_the_biggest() {
        let s = club().scene(480.0, 320.0, &InterMetrics).unwrap();
        let d = dots(&s);
        let r = |n: &str| d.iter().find(|x| x.3 == n).unwrap().2;
        // Cy and Di have three links each; Ann has two.
        assert_eq!(r("Cy"), MAX_RADIUS);
        assert!(r("Ann") < r("Cy") && r("Ann") > DOT_RADIUS);
    }

    #[test]
    fn a_pair_is_not_stretched_across_the_whole_chart() {
        let s = NetworkChart::new()
            .link("a", "b", 1.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let d = dots(&s);
        let gap = ((d[0].0 - d[1].0).powi(2) + (d[0].1 - d[1].1).powi(2)).sqrt();
        assert!(gap < 200.0, "{gap}");
    }

    #[test]
    fn big_networks_drop_labels() {
        let mut c = NetworkChart::new();
        for i in 0..MAX_LABELS {
            c = c.link("hub", format!("n{i}"), 1.0);
        }
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        assert!(
            !s.marks
                .iter()
                .any(|m| matches!(m.shape, Shape::Text { .. }))
        );
    }

    #[test]
    fn too_many_nodes_are_refused() {
        let mut c = NetworkChart::new();
        for i in 0..MAX_NODES {
            c = c.link("hub", format!("n{i}"), 1.0);
        }
        let err = c.scene(640.0, 400.0, &InterMetrics).unwrap_err();
        assert!(err.to_string().contains("2001 nodes"), "{err}");
    }
}
