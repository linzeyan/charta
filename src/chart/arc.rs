use super::frame::{self, Key, text};
use super::graph::{Graph, Link, graph, links_chart};
use super::{
    Chart, DOT_RADIUS, GAP, MIN_TARGET, PAD, SURFACE_GAP, TICK_SIZE, check_size, count, describe,
};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Hit, Mark, PathOp, Rect, Scene, Shape, TipRow};
use std::f64::consts::FRAC_1_SQRT_2;

/// Arc widths run from 1px to this.
const MAX_STROKE: f64 = 4.0;
/// 4/3·(√2 − 1): the control-point distance, as a fraction of the radius,
/// for a cubic tracing a quarter circle. A literal, so both ports agree.
const KAPPA: f64 = 0.552_284_749_830_793_6;

links_chart!(
    /// Nodes in a row with links arching over them: long-range connections
    /// rise high, local ones stay low.
    ArcChart,
    false,
    arcs
);

fn arcs(
    title: Option<&str>,
    g: &Graph,
    width: f64,
    height: f64,
    measure: &dyn TextMeasure,
) -> Result<Scene, Error> {
    let n = g.names.len();
    let mut marks = Vec::new();
    let mut top = frame::title(&mut marks, title);
    top = frame::legend(&mut marks, measure, top, width, &g.legend(), Key::Swatch);

    let step = (width - 2.0 * PAD) / n.max(1) as f64;
    let x = |i: usize| PAD + (i as f64 + 0.5) * step;
    let longest = g
        .names
        .iter()
        .map(|s| measure.width(s, TICK_SIZE, Weight::Regular))
        .fold(0.0, f64::max);
    // Labels turn 45° when they would run into each other, as on a
    // category axis.
    let rotated = longest > step - 4.0;
    let label_h = if rotated {
        (longest + TICK_SIZE) * FRAC_1_SQRT_2
    } else {
        TICK_SIZE
    };
    let base = (height - PAD - label_h - GAP - DOT_RADIUS).max(top + 1.0);
    // Arcs are semicircles until the widest would not fit; then all of
    // them flatten by the same factor, so heights still compare.
    let widest = g
        .edges
        .iter()
        .map(|&(a, b, _)| (x(a) - x(b)).abs() / 2.0)
        .fold(0.0, f64::max);
    let squash = if widest > 0.0 {
        ((base - top - MAX_STROKE / 2.0) / widest).min(1.0)
    } else {
        1.0
    };
    let series = |i: usize| g.colored().then(|| g.names[i].clone());
    let max_v = g.edges.iter().map(|e| e.2).fold(0.0, f64::max);

    for (k, &(a, b, v)) in g.edges.iter().enumerate() {
        let (xa, xb) = (x(a.min(b)), x(a.max(b)));
        let (cx, rx) = ((xa + xb) / 2.0, (xb - xa) / 2.0);
        let ry = rx * squash;
        let ops = vec![
            PathOp::Move(xa, base),
            PathOp::Cubic(
                xa,
                base - KAPPA * ry,
                cx - KAPPA * rx,
                base - ry,
                cx,
                base - ry,
            ),
            PathOp::Cubic(cx + KAPPA * rx, base - ry, xb, base - KAPPA * ry, xb, base),
        ];
        let mut m = Mark::new(Shape::Path { ops })
            .stroke(
                Color::Link(g.color(a)),
                1.0 + (MAX_STROKE - 1.0) * v / max_v,
            )
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
    let strength = g.strength();
    for (i, name) in g.names.iter().enumerate() {
        let links = g.edges.iter().filter(|e| e.0 == i || e.1 == i).count();
        let mut m = Mark::new(Shape::Circle {
            cx: x(i),
            cy: base,
            r: DOT_RADIUS,
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
            }])
            .area(Rect {
                x: x(i) - MIN_TARGET / 2.0,
                y: base - MIN_TARGET / 2.0,
                w: MIN_TARGET,
                h: MIN_TARGET,
            }),
        );
        if let Some(s) = series(i) {
            m = m.series(s);
        }
        marks.push(m);
        let mut t = text(
            x(i),
            base + DOT_RADIUS + GAP,
            name,
            TICK_SIZE,
            Weight::Regular,
            Anchor::Middle,
            Baseline::Top,
            Color::TextSecondary,
        );
        if rotated && let Shape::Text { anchor, angle, .. } = &mut t.shape {
            *anchor = Anchor::End;
            *angle = -45.0;
        }
        marks.push(t);
    }
    Ok(Scene {
        width,
        height,
        label: describe(
            title,
            &format!(
                "Arc diagram of {} and {}.",
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

    fn apex(s: &Scene, key: &str) -> f64 {
        let m = s
            .marks
            .iter()
            .find(|m| m.hit.as_ref().is_some_and(|h| h.key == key))
            .unwrap();
        let Shape::Path { ops } = &m.shape else {
            panic!()
        };
        let PathOp::Cubic(.., y) = ops[1] else {
            panic!()
        };
        y
    }

    #[test]
    fn longer_links_arch_higher_and_all_stay_in_the_plot() {
        let s = ArcChart::new()
            .link("a", "b", 1.0)
            .link("c", "d", 1.0)
            .link("a", "d", 1.0)
            .scene(640.0, 300.0, &InterMetrics)
            .unwrap();
        // a–d spans three of the four steps: as a semicircle it would rise
        // past the legend, so it is squashed to just touch the top.
        assert!(apex(&s, "link-2") < apex(&s, "link-0"));
        let top = PAD + 16.0 + GAP;
        assert!((apex(&s, "link-2") - MAX_STROKE / 2.0 - top).abs() < 1e-6);
        // Squashing is uniform: a–b and c–d (both one step) peak alike,
        // a third as high as a–d.
        assert!((apex(&s, "link-0") - apex(&s, "link-1")).abs() < 1e-9);
        let base = 300.0 - PAD - TICK_SIZE - GAP - DOT_RADIUS;
        assert!(((base - apex(&s, "link-2")) / (base - apex(&s, "link-0")) - 3.0).abs() < 1e-9);
    }

    #[test]
    fn crowded_labels_turn() {
        let mut c = ArcChart::new();
        for i in 0..20 {
            c = c.link(format!("station {i}"), format!("station {}", i + 1), 1.0);
        }
        let s = c.scene(480.0, 300.0, &InterMetrics).unwrap();
        assert!(
            s.marks
                .iter()
                .any(|m| matches!(m.shape, Shape::Text { angle, .. } if angle == -45.0))
        );
    }
}
