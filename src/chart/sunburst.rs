use super::frame::{self, Key, text};
use super::hierarchy::{Node, flatten};
use super::polar::sector;
use super::{Chart, GAP, PAD, SURFACE_GAP, TICK_SIZE, check_size, count, describe};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Hit, Mark, Scene, Shape, TipRow};
use std::f64::consts::TAU;

const TOTAL_SIZE: f64 = 18.0;

/// A hierarchy as rings: the root's children around the center, each
/// level outward, every node's angle proportional to its value.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct SunburstChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub root: Node,
}

impl SunburstChart {
    pub fn new(root: Node) -> Self {
        SunburstChart { title: None, root }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }
}

impl Chart for SunburstChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        // Data order is kept: around a circle, the order often means
        // something (months, stages) that sorting by size would scramble.
        let flat = flatten(&self.root, true)?;
        let t = &flat.tree;
        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &flat.legend(), Key::Swatch);
        let bottom = height - PAD;
        let (cx, cy) = (width / 2.0, (top + bottom) / 2.0);
        let radius = ((width - 2.0 * PAD).min(bottom - top) / 2.0).max(1.0);
        let depth = t.depth.iter().copied().max().unwrap_or(0);
        // The center (the root's ring) is a hole as wide as one ring.
        let ring = radius / (depth + 1) as f64;
        let total = t.value[0];

        let mut span = vec![(0.0, TAU); t.len()];
        for i in 0..t.len() {
            let (a0, a1) = span[i];
            let mut a = a0;
            for &c in &t.children[i] {
                let w = if t.value[i] > 0.0 {
                    (a1 - a0) * t.value[c] / t.value[i]
                } else {
                    0.0
                };
                span[c] = (a, a + w);
                a += w;
            }
            if i == 0 || t.value[i] <= 0.0 {
                continue;
            }
            let d = t.depth[i] as f64;
            let col = flat.color(i);
            let mut m = Mark::new(Shape::Path {
                ops: sector(cx, cy, d * ring, (d + 1.0) * ring, a0, a1),
            })
            .fill(Color::Series(col))
            .stroke(Color::Surface, SURFACE_GAP)
            .hit(
                Hit::new(
                    i.to_string(),
                    flat.path[i].as_str(),
                    format::value(t.value[i]),
                )
                .rows(vec![TipRow {
                    color: None,
                    name: "share".into(),
                    value: format!("{}%", format::approx(t.value[i] / total * 100.0)),
                }]),
            );
            if let Some(s) = flat.series(i) {
                m = m.series(s);
            }
            marks.push(m);
        }
        if !self.root.children.is_empty() {
            let label = format::value(total);
            if measure.width(&label, TOTAL_SIZE, Weight::Semibold) <= 1.6 * ring {
                marks.push(text(
                    cx,
                    cy,
                    &label,
                    TOTAL_SIZE,
                    Weight::Semibold,
                    Anchor::Middle,
                    Baseline::Bottom,
                    Color::TextPrimary,
                ));
                marks.push(text(
                    cx,
                    cy + GAP / 2.0,
                    if self.root.name.is_empty() {
                        "Total"
                    } else {
                        &self.root.name
                    },
                    TICK_SIZE,
                    Weight::Regular,
                    Anchor::Middle,
                    Baseline::Top,
                    Color::TextMuted,
                ));
            }
        }
        let leaves = (0..t.len()).filter(|&i| t.children[i].is_empty()).count();
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Sunburst of {} in {}.",
                    count(leaves, "item", "items"),
                    count(depth, "level", "levels")
                ),
            ),
            marks,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;
    use crate::scene::PathOp;

    fn chart() -> SunburstChart {
        SunburstChart::new(Node::branch(
            "All",
            vec![
                Node::branch("A", vec![Node::leaf("a1", 1.0), Node::leaf("a2", 3.0)]),
                Node::leaf("B", 4.0),
            ],
        ))
    }

    #[test]
    fn children_share_their_parent_s_angle_in_proportion_and_ring_outward() {
        let s = chart().scene(400.0, 400.0, &InterMetrics).unwrap();
        let start = |l: &str| {
            s.marks
                .iter()
                .find(|m| {
                    m.hit
                        .as_ref()
                        .is_some_and(|h| h.label == l && h.kind == crate::scene::HitKind::Mark)
                })
                .map(|m| match &m.shape {
                    Shape::Path { ops } => ops.clone(),
                    _ => unreachable!(),
                })
                .unwrap()
        };
        // A spans half the circle (4 of 8), from twelve o'clock; a1 takes
        // the first quarter of that, a2 the rest, one ring further out.
        let (a, a1, a2, b) = (start("A"), start("A / a1"), start("A / a2"), start("B"));
        let pt = |ops: &[PathOp], k: usize| match ops[k] {
            PathOp::Move(x, y) => (x, y),
            PathOp::Cubic(.., x, y) => (x, y),
            _ => unreachable!(),
        };
        let r = |p: (f64, f64), c: (f64, f64)| ((p.0 - c.0).powi(2) + (p.1 - c.1).powi(2)).sqrt();
        // B starts where A ends: straight down from the center.
        let a_end = pt(&a, 2);
        let b_start = pt(&b, 0);
        assert!((a_end.0 - b_start.0).abs() < 1e-9 && (a_end.0 - 200.0).abs() < 1e-9);
        // a2 starts a quarter of A's half-turn in: at 45°.
        let a2_start = pt(&a2, 0);
        let center = (200.0, (a_end.1 + pt(&a, 0).1) / 2.0);
        assert!(((a2_start.0 - center.0) - (center.1 - a2_start.1)).abs() < 1e-6);
        assert!(
            r(pt(&a1, 0), center) > r(pt(&a, 0), center),
            "a1 one ring out"
        );
    }

    #[test]
    fn the_center_shows_the_total() {
        let s = chart().scene(400.0, 400.0, &InterMetrics).unwrap();
        assert!(s.marks.iter().any(|m| matches!(
            &m.shape, Shape::Text { text, .. } if text == "8"
        )));
    }
}
