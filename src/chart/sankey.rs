use super::frame::{self, Key, halo_label, text};
use super::graph::{Link, band, graph};
use super::{Chart, GAP, MIN_TARGET, PAD, TICK_SIZE, check_size, count, describe};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::layout::sankey::sankey;
use crate::scene::{Anchor, Baseline, Corners, Hit, Mark, Rect, Scene, Shape, TipRow};

const NODE_W: f64 = 12.0;
/// Room between nodes in a column: more than a label's height, so labels of
/// neighboring thin nodes cannot collide.
const NODE_GAP: f64 = 16.0;

/// Flows between stages: each node a bar as tall as what passes through it,
/// each link a band as thick as its value.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct SankeyChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub links: Vec<Link>,
}

impl SankeyChart {
    pub fn new() -> Self {
        SankeyChart::default()
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    pub fn link(
        mut self,
        source: impl Into<String>,
        target: impl Into<String>,
        value: f64,
    ) -> Self {
        self.links.push(Link::new(source, target, value));
        self
    }
}

impl Chart for SankeyChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let g = graph(&self.links, true)?;
        let label = describe(
            self.title.as_deref(),
            &format!(
                "Sankey diagram of {} between {}.",
                count(g.edges.len(), "flow", "flows"),
                count(g.names.len(), "node", "nodes")
            ),
        );
        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &g.legend(), Key::Swatch);
        if g.edges.is_empty() {
            return Ok(Scene {
                width,
                height,
                label,
                marks,
            });
        }
        let bottom = (height - PAD).max(top + 1.0);
        let s = sankey(g.names.len(), &g.edges, (top, bottom), NODE_GAP).map_err(|i| {
            Error::InvalidData(format!(
                "links loop back to {:?}: a Sankey diagram's flows only move forward",
                g.names[i]
            ))
        })?;

        // The first column's labels go left of it and the last column's
        // right of it, outside the flows; the rest sit beside their bar.
        let last = s.columns - 1;
        let label_w = |col: usize| {
            (0..g.names.len())
                .filter(|&i| s.layer[i] == col)
                .map(|i| measure.width(&g.names[i], TICK_SIZE, Weight::Regular))
                .fold(0.0, f64::max)
        };
        let left = PAD + label_w(0) + GAP / 2.0;
        let right = (width - PAD - label_w(last) - GAP / 2.0).max(left + NODE_W);
        let kx = if last > 0 {
            (right - left - NODE_W) / last as f64
        } else {
            0.0
        };
        let node_x = |i: usize| left + s.layer[i] as f64 * kx;
        let series = |i: usize| g.colored().then(|| g.names[i].clone());

        let mut out = vec![0.0; g.names.len()];
        let mut inc = vec![0.0; g.names.len()];
        for &(a, b, v) in &g.edges {
            out[a] += v;
            inc[b] += v;
        }
        for (k, &(a, b, v)) in g.edges.iter().enumerate() {
            // Hairline flows still draw: a missing link reads as no flow.
            let w = s.width[k].max(1.0);
            let mut m = Mark::new(Shape::Path {
                ops: band(
                    node_x(a) + NODE_W,
                    s.source_y[k] - w / 2.0,
                    node_x(b),
                    s.target_y[k] - w / 2.0,
                    w,
                ),
            })
            .fill(Color::Link(g.color(a)))
            .hit(
                Hit::new(
                    format!("link-{k}"),
                    format!("{} → {}", g.names[a], g.names[b]),
                    format::value(v),
                )
                .rows(vec![TipRow {
                    color: None,
                    name: format!("of {}'s outflow", g.names[a]),
                    value: format!("{}%", format::approx(v / out[a] * 100.0)),
                }]),
            );
            if let Some(name) = series(a) {
                m = m.series(name);
            }
            marks.push(m);
        }

        for (i, name) in g.names.iter().enumerate() {
            let (x, y0) = (node_x(i), s.y0[i]);
            let h = (s.y1[i] - y0).max(1.0);
            let mut rows = Vec::new();
            if inc[i] > 0.0 && out[i] > 0.0 {
                for (n, v) in [("in", inc[i]), ("out", out[i])] {
                    rows.push(TipRow {
                        color: None,
                        name: n.into(),
                        value: format::value(v),
                    });
                }
            }
            let cy = y0 + h / 2.0;
            let target_h = h.max(MIN_TARGET);
            let mut m = Mark::new(Shape::Rect {
                rect: Rect {
                    x,
                    y: y0,
                    w: NODE_W,
                    h,
                },
                radius: Corners::all(0.0),
            })
            .fill(Color::Series(g.color(i)))
            .hit(
                Hit::new(
                    format!("node-{i}"),
                    name.as_str(),
                    format::value(s.value[i]),
                )
                .rows(rows)
                .area(Rect {
                    x,
                    y: cy - target_h / 2.0,
                    w: NODE_W,
                    h: target_h,
                }),
            );
            if let Some(n) = series(i) {
                m = m.series(n);
            }
            marks.push(m);
        }
        // Labels last, over every flow.
        for (i, name) in g.names.iter().enumerate() {
            let cy = (s.y0[i] + s.y1[i]) / 2.0;
            if s.layer[i] == 0 {
                marks.push(text(
                    node_x(i) - GAP / 2.0,
                    cy,
                    name,
                    TICK_SIZE,
                    Weight::Regular,
                    Anchor::End,
                    Baseline::Middle,
                    Color::TextPrimary,
                ));
            } else if s.layer[i] == last {
                marks.push(text(
                    node_x(i) + NODE_W + GAP / 2.0,
                    cy,
                    name,
                    TICK_SIZE,
                    Weight::Regular,
                    Anchor::Start,
                    Baseline::Middle,
                    Color::TextPrimary,
                ));
            } else {
                halo_label(
                    &mut marks,
                    measure,
                    (node_x(i) + NODE_W + GAP / 2.0, cy),
                    name,
                    Anchor::Start,
                    Color::TextPrimary,
                );
            }
        }
        Ok(Scene {
            width,
            height,
            label,
            marks,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;
    use crate::scene::{HitKind, PathOp};

    fn energy() -> SankeyChart {
        SankeyChart::new()
            .link("Coal", "Power", 40.0)
            .link("Gas", "Power", 25.0)
            .link("Gas", "Homes", 15.0)
            .link("Power", "Homes", 30.0)
            .link("Power", "Industry", 35.0)
    }

    fn hit<'a>(s: &'a Scene, label: &str) -> &'a Mark {
        s.marks
            .iter()
            .find(|m| {
                m.hit
                    .as_ref()
                    .is_some_and(|h| h.kind == HitKind::Mark && h.label == label)
            })
            .unwrap_or_else(|| panic!("no mark {label}"))
    }

    fn rect(m: &Mark) -> Rect {
        match m.shape {
            Shape::Rect { rect, .. } => rect,
            _ => panic!("not a rect"),
        }
    }

    #[test]
    fn a_node_is_as_tall_as_its_throughput() {
        let s = energy().scene(640.0, 400.0, &InterMetrics).unwrap();
        // Power takes in 65 and sends out 65; Homes only receives 45.
        let (power, homes) = (rect(hit(&s, "Power")), rect(hit(&s, "Homes")));
        assert!((power.h / homes.h - 65.0 / 45.0).abs() < 1e-9);
    }

    #[test]
    fn links_join_the_bars_they_connect() {
        let s = energy().scene(640.0, 400.0, &InterMetrics).unwrap();
        let (coal, power) = (rect(hit(&s, "Coal")), rect(hit(&s, "Power")));
        let Shape::Path { ops } = &hit(&s, "Coal → Power").shape else {
            panic!()
        };
        let (PathOp::Move(x0, y0), PathOp::Cubic(.., x1, y1)) = (ops[0], ops[1]) else {
            panic!()
        };
        assert_eq!(x0, coal.x + coal.w);
        assert_eq!(x1, power.x);
        // All of Coal's output is this link: it spans Coal's whole bar.
        assert!((y0 - coal.y).abs() < 1e-9);
        assert!(y1 >= power.y - 1e-9 && y1 <= power.y + power.h);
    }

    #[test]
    fn the_last_column_is_labeled_outside_on_the_right() {
        let s = energy().scene(640.0, 400.0, &InterMetrics).unwrap();
        let industry = rect(hit(&s, "Industry"));
        // The legend names Industry too; node labels are drawn last.
        let label = s
            .marks
            .iter()
            .rev()
            .find_map(|m| match &m.shape {
                Shape::Text {
                    text, x, anchor, ..
                } if text == "Industry" => Some((*x, *anchor)),
                _ => None,
            })
            .unwrap();
        assert!(label.0 > industry.x + industry.w && label.1 == Anchor::Start);
        assert!(
            label.0 + InterMetrics.width("Industry", TICK_SIZE, Weight::Regular) <= 640.0 - PAD
        );
    }

    #[test]
    fn a_loop_is_rejected_by_name() {
        let err = SankeyChart::new()
            .link("a", "b", 1.0)
            .link("b", "c", 1.0)
            .link("c", "b", 1.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap_err();
        assert!(err.to_string().contains("\"b\""), "{err}");
    }
}
