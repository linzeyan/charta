use super::frame::{self, Key, text};
use super::hierarchy::{Node, flatten};
use super::{Chart, HAIRLINE, PAD, TICK_SIZE, check_size, count, describe};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::layout::hierarchy::pack;
use crate::scene::{Anchor, Baseline, Hit, Mark, Scene, Shape, TipRow};

/// Space between sibling circles and around them inside their parent.
const PADDING: f64 = 3.0;

/// Nested circles: each leaf's area proportional to its value, siblings
/// packed tightly inside their parent.
///
/// # Example
///
/// ```
/// use charta::base::text::InterMetrics;
/// use charta::{Chart, CirclePackingChart, Node};
///
/// let teams = Node::branch(
///     "Company",
///     vec![
///         Node::branch(
///             "Engineering",
///             vec![Node::leaf("Frontend", 12.0), Node::leaf("Backend", 18.0)],
///         ),
///         Node::branch("Product", vec![Node::leaf("PM", 6.0), Node::leaf("Design", 9.0)]),
///     ],
/// );
/// let chart = CirclePackingChart::new(teams).title("Team sizes");
/// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
/// // The scene's label is what a screen reader announces.
/// assert_eq!(scene.label, "Team sizes. Circle packing of 4 items.");
/// ```
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct CirclePackingChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub root: Node,
}

impl CirclePackingChart {
    pub fn new(root: Node) -> Self {
        CirclePackingChart { title: None, root }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }
}

impl Chart for CirclePackingChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let mut flat = flatten(&self.root, true)?;
        flat.tree.sort_by_value();
        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &flat.legend(), Key::Swatch);
        let (w, h) = ((width - 2.0 * PAD).max(1.0), (height - PAD - top).max(1.0));
        let leaves = (0..flat.tree.len())
            .filter(|&i| flat.tree.children[i].is_empty())
            .count();
        let total = flat.tree.value[0];

        if !self.root.children.is_empty() {
            let circles = pack(&flat.tree, w, h, PADDING);
            // Parents before children (index order is preorder), so leaves
            // land on top and win the hover.
            for (i, c) in circles.iter().enumerate().skip(1) {
                let v = flat.tree.value[i];
                if v <= 0.0 {
                    continue;
                }
                let (x, y) = (PAD + c.x, top + c.y);
                let leaf = flat.tree.children[i].is_empty();
                let col = flat.color(i);
                let mut rows = vec![TipRow {
                    color: None,
                    name: "share".into(),
                    value: format!("{}%", format::approx(v / total * 100.0)),
                }];
                if !leaf {
                    rows.push(TipRow {
                        color: None,
                        name: "items".into(),
                        value: flat.tree.children[i].len().to_string(),
                    });
                }
                let mut m = Mark::new(Shape::Circle {
                    cx: x,
                    cy: y,
                    r: c.r,
                })
                .hit(Hit::new(i.to_string(), flat.path[i].as_str(), format::value(v)).rows(rows));
                // Leaves: a wash with an edge in the branch hue, so labels
                // stay dark text. Groups: an outline only, the leaves inside
                // them carry the color.
                m = if leaf {
                    m.fill(Color::Wash(col)).stroke(Color::Series(col), 1.0)
                } else {
                    m.stroke(Color::Border, HAIRLINE)
                };
                if let Some(s) = flat.series(i) {
                    m = m.series(s);
                }
                marks.push(m);
                let name = &flat.nodes[i].name;
                if leaf
                    && measure.width(name, TICK_SIZE, Weight::Semibold) <= 1.8 * c.r
                    && TICK_SIZE <= c.r
                {
                    let mut t = text(
                        x,
                        y,
                        name,
                        TICK_SIZE,
                        Weight::Semibold,
                        Anchor::Middle,
                        Baseline::Middle,
                        Color::TextPrimary,
                    );
                    if let Some(s) = flat.series(i) {
                        t = t.series(s);
                    }
                    marks.push(t);
                }
            }
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!("Circle packing of {}.", count(leaves, "item", "items")),
            ),
            marks,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;

    fn circles(s: &Scene) -> Vec<(f64, f64, f64, &Hit)> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Circle { cx, cy, r }, Some(h)) => Some((*cx, *cy, *r, h)),
                _ => None,
            })
            .collect()
    }

    #[test]
    fn leaves_sit_inside_their_group_and_the_chart() {
        let c = CirclePackingChart::new(Node::branch(
            "r",
            vec![
                Node::branch(
                    "A",
                    vec![
                        Node::leaf("a1", 3.0),
                        Node::leaf("a2", 2.0),
                        Node::leaf("a3", 1.0),
                    ],
                ),
                Node::branch("B", vec![Node::leaf("b1", 4.0), Node::leaf("b2", 1.0)]),
            ],
        ));
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let cs = circles(&s);
        let find = |l: &str| *cs.iter().find(|c| c.3.label == l).unwrap();
        for (leaf, group) in [("A / a1", "A"), ("A / a3", "A"), ("B / b2", "B")] {
            let (l, g) = (find(leaf), find(group));
            let d = ((l.0 - g.0).powi(2) + (l.1 - g.1).powi(2)).sqrt();
            assert!(d + l.2 <= g.2 + 1e-6, "{leaf} leaves {group}");
        }
        for c in &cs {
            assert!(c.0 - c.2 >= -1e-6 && c.0 + c.2 <= 640.0 + 1e-6);
            assert!(c.1 + c.2 <= 400.0 + 1e-6);
        }
    }

    #[test]
    fn leaf_area_is_proportional_to_value() {
        let c = CirclePackingChart::new(Node::branch(
            "r",
            vec![Node::leaf("a", 1.0), Node::leaf("b", 4.0)],
        ));
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let cs = circles(&s);
        let r = |l: &str| cs.iter().find(|c| c.3.label == l).unwrap().2;
        assert!((r("b") / r("a") - 2.0).abs() < 1e-9);
    }

    #[test]
    fn hovering_a_leaf_beats_its_group() {
        let c = CirclePackingChart::new(Node::branch(
            "r",
            vec![
                Node::branch("A", vec![Node::leaf("a1", 1.0), Node::leaf("a2", 1.0)]),
                Node::leaf("B", 1.0),
            ],
        ));
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let leaf = *circles(&s).iter().find(|c| c.3.label == "A / a1").unwrap();
        let i = s.hit_test(leaf.0, leaf.1, &[]).unwrap();
        assert_eq!(s.marks[i].hit.as_ref().unwrap().label, "A / a1");
    }
}
