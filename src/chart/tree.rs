use super::frame::{Key, halo_label, legend, text, title as draw_title};
use super::hierarchy::{Flat, Node, flatten};
use super::polar::{around, at, label_room};
use super::{
    Chart, DOT_RADIUS, GAP, MIN_TARGET, PAD, SURFACE_GAP, TICK_SIZE, TITLE_SIZE, check_size, count,
    describe,
};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::layout::hierarchy::tidy;
use crate::scene::{Anchor, Baseline, Corners, Hit, Mark, PathOp, Rect, Scene, Shape, TipRow};
use std::f64::consts::TAU;

const LINK: f64 = 1.5;
/// Mind-map links from the center are drawn heavier: they are the
/// branches everything else hangs from.
const TRUNK: f64 = 2.5;
const ROOT_PAD: f64 = 8.0;
const BRANCH_PAD: f64 = 5.0;

macro_rules! tree_chart {
    ($(#[$doc:meta])* $name:ident, $build:ident) => {
        $(#[$doc])*
        #[derive(Clone, Debug, Default, PartialEq)]
        #[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
        pub struct $name {
            #[cfg_attr(
                feature = "serde",
                serde(default, skip_serializing_if = "Option::is_none")
            )]
            pub title: Option<String>,
            pub root: Node,
        }

        impl $name {
            pub fn new(root: Node) -> Self {
                $name { title: None, root }
            }

            pub fn title(mut self, title: impl Into<String>) -> Self {
                self.title = Some(title.into());
                self
            }
        }

        impl Chart for $name {
            fn scene(
                &self,
                width: f64,
                height: f64,
                measure: &dyn TextMeasure,
            ) -> Result<Scene, Error> {
                check_size(width, height)?;
                let flat = flatten(&self.root, false)?;
                $build(self.title.as_deref(), &flat, width, height, measure)
            }
        }
    };
}

tree_chart!(
    /// A hierarchy as a node-link diagram, root on the left and each level
    /// a column further right (a tidy tree: subtrees never overlap and
    /// parents sit centered beside their children).
    ///
    /// # Example
    ///
    /// ```
    /// use charta::base::text::InterMetrics;
    /// use charta::{Chart, Node, TreeChart};
    ///
    /// let org = Node::branch(
    ///     "CEO",
    ///     vec![
    ///         Node::branch("CTO", vec![Node::new("Platform"), Node::new("Data")]),
    ///         Node::branch("CFO", vec![Node::new("Finance"), Node::new("Legal")]),
    ///     ],
    /// );
    /// let chart = TreeChart::new(org).title("Organization");
    /// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
    /// // The scene's label is what a screen reader announces.
    /// assert_eq!(scene.label, "Organization. Tree diagram of 7 nodes in 3 levels.");
    /// ```
    TreeChart,
    tree_diagram
);
tree_chart!(
    /// A mind map: the central topic in the middle, ideas branching outward
    /// in every direction, each main branch in its own hue.
    ///
    /// # Example
    ///
    /// ```
    /// use charta::base::text::InterMetrics;
    /// use charta::{BrainstormChart, Chart, Node};
    ///
    /// let plan = Node::branch(
    ///     "Launch plan",
    ///     vec![
    ///         Node::branch("Marketing", vec![Node::new("Social"), Node::new("Email")]),
    ///         Node::branch("Product", vec![Node::new("Pricing"), Node::new("Docs")]),
    ///         Node::new("Support"),
    ///     ],
    /// );
    /// let chart = BrainstormChart::new(plan).title("Launch plan");
    /// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
    /// // The scene's label is what a screen reader announces.
    /// assert_eq!(scene.label, "Launch plan. Mind map of 7 ideas around \"Launch plan\".");
    /// ```
    BrainstormChart,
    brainstorm
);

/// The node's tooltip: its path, its value if it has one, and how many
/// children it has.
fn node_hit(flat: &Flat, i: usize, x: f64, y: f64) -> Hit {
    let label = if i == 0 {
        flat.nodes[0].name.clone()
    } else {
        flat.path[i].clone()
    };
    let value = flat.nodes[i].value.map(format::value).unwrap_or_default();
    let kids = flat.tree.children[i].len();
    let rows = if kids > 0 {
        vec![TipRow {
            color: None,
            name: "children".into(),
            value: kids.to_string(),
        }]
    } else {
        Vec::new()
    };
    Hit::new(i.to_string(), label, value).rows(rows).area(Rect {
        x: x - MIN_TARGET / 2.0,
        y: y - MIN_TARGET / 2.0,
        w: MIN_TARGET,
        h: MIN_TARGET,
    })
}

/// A rounded label box centered on `(x, y)`: surface fill, `edge` outline,
/// the text inside; the whole box is the hover target.
#[allow(clippy::too_many_arguments)]
fn pill(
    marks: &mut Vec<Mark>,
    measure: &dyn TextMeasure,
    (x, y): (f64, f64),
    name: &str,
    (size, pad): (f64, f64),
    edge: Color,
    hit: Hit,
    series: Option<&str>,
) {
    let w = measure.width(name, size, Weight::Semibold) + 2.0 * pad;
    let h = size + 2.0 * pad;
    let rect = Rect {
        x: x - w / 2.0,
        y: y - h / 2.0,
        w,
        h,
    };
    let mut boxed = Mark::new(Shape::Rect {
        rect,
        radius: Corners::all(h / 2.0),
    })
    .fill(Color::Surface)
    .stroke(edge, LINK)
    .hit(hit.area(rect));
    let mut label = text(
        x,
        y,
        name,
        size,
        Weight::Semibold,
        Anchor::Middle,
        Baseline::Middle,
        Color::TextPrimary,
    );
    if let Some(s) = series {
        boxed = boxed.series(s);
        label = label.series(s);
    }
    marks.push(boxed);
    marks.push(label);
}

fn tree_diagram(
    title: Option<&str>,
    flat: &Flat,
    width: f64,
    height: f64,
    measure: &dyn TextMeasure,
) -> Result<Scene, Error> {
    let t = &flat.tree;
    let mut marks = Vec::new();
    let top = draw_title(&mut marks, title);
    let bottom = height - PAD;
    let label_w = |i: usize| measure.width(&flat.nodes[i].name, TICK_SIZE, Weight::Regular);
    let leaf_w = (0..t.len())
        .filter(|&i| t.children[i].is_empty() && i != 0)
        .map(label_w)
        .fold(0.0, f64::max);
    let left = PAD + label_w(0) + GAP;
    let right = (width - PAD - leaf_w - GAP).max(left + 1.0);
    let depth = t.depth.iter().copied().max().unwrap_or(0).max(1) as f64;
    let breadth = tidy(t, |same, _| if same { 1.0 } else { 2.0 });
    let pos = |i: usize| {
        (
            left + (right - left) * t.depth[i] as f64 / depth,
            top + (bottom - top) * breadth[i],
        )
    };
    for i in 1..t.len() {
        let (p, c) = (pos(t.parent[i].expect("non-root")), pos(i));
        let mx = (p.0 + c.0) / 2.0;
        marks.push(
            Mark::new(Shape::Path {
                ops: vec![
                    PathOp::Move(p.0, p.1),
                    PathOp::Cubic(mx, p.1, mx, c.1, c.0, c.1),
                ],
            })
            .stroke(Color::Axis, LINK),
        );
    }
    for i in 0..t.len() {
        let (x, y) = pos(i);
        let leaf = t.children[i].is_empty();
        // Inner nodes solid, leaves hollow: the shape says which nodes
        // branch further.
        let m = Mark::new(Shape::Circle {
            cx: x,
            cy: y,
            r: DOT_RADIUS,
        })
        .hit(node_hit(flat, i, x, y));
        marks.push(if leaf && i != 0 {
            m.fill(Color::Surface).stroke(Color::Series(0), LINK)
        } else {
            m.fill(Color::Series(0)).stroke(Color::Surface, SURFACE_GAP)
        });
        // Leaves label to the right; inner nodes to the left, over the
        // link coming in, on a backing.
        let name = &flat.nodes[i].name;
        if leaf && i != 0 {
            marks.push(text(
                x + DOT_RADIUS + GAP / 2.0,
                y,
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
                (x - DOT_RADIUS - GAP / 2.0, y),
                name,
                Anchor::End,
                Color::TextPrimary,
            );
        }
    }
    Ok(Scene {
        width,
        height,
        label: describe(
            title,
            &format!(
                "Tree diagram of {} in {}.",
                count(t.len(), "node", "nodes"),
                count(depth as usize + 1, "level", "levels")
            ),
        ),
        marks,
    })
}

fn brainstorm(
    title: Option<&str>,
    flat: &Flat,
    width: f64,
    height: f64,
    measure: &dyn TextMeasure,
) -> Result<Scene, Error> {
    let t = &flat.tree;
    let mut marks = Vec::new();
    let mut top = draw_title(&mut marks, title);
    top = legend(&mut marks, measure, top, width, &flat.legend(), Key::Line);
    let bottom = height - PAD;
    let (cx, cy) = (width / 2.0, (top + bottom) / 2.0);
    let names: Vec<String> = (1..t.len()).map(|i| flat.nodes[i].name.clone()).collect();
    let (room_x, room_y) = label_room(measure, &names);
    let radius = ((width - 2.0 * PAD) / 2.0 - room_x - DOT_RADIUS)
        .min((bottom - top) / 2.0 - room_y - DOT_RADIUS)
        .max(1.0);
    let depth = t.depth.iter().copied().max().unwrap_or(0).max(1) as f64;
    // Deeper rings are longer, so neighbours there need less angle; d3's
    // radial separation divides by depth.
    let breadth = tidy(t, |same, d| {
        (if same { 1.0 } else { 2.0 }) / d.max(1) as f64
    });
    let polar = |i: usize| (TAU * breadth[i], radius * t.depth[i] as f64 / depth);
    let pos = |i: usize| {
        let (a, r) = polar(i);
        at(cx, cy, r, a)
    };
    for i in 1..t.len() {
        let p = t.parent[i].expect("non-root");
        let ((a0, r0), (a1, r1)) = (polar(p), polar(i));
        let mid = (r0 + r1) / 2.0;
        let (x0, y0) = pos(p);
        let (c1x, c1y) = at(cx, cy, mid, a0);
        let (c2x, c2y) = at(cx, cy, mid, a1);
        let (x1, y1) = pos(i);
        let col = flat.color(i);
        let mut m = Mark::new(Shape::Path {
            ops: vec![
                PathOp::Move(x0, y0),
                PathOp::Cubic(c1x, c1y, c2x, c2y, x1, y1),
            ],
        })
        .stroke(Color::Series(col), if p == 0 { TRUNK } else { LINK });
        if let Some(s) = flat.series(i) {
            m = m.series(s);
        }
        marks.push(m);
    }
    for i in 1..t.len() {
        let (x, y) = pos(i);
        let col = flat.color(i);
        let series = flat.series(i);
        // Main branches are pills with their name inside: a label beside
        // the node would sit on the links fanning out from it.
        if t.depth[i] == 1 {
            pill(
                &mut marks,
                measure,
                (x, y),
                &flat.nodes[i].name,
                (TICK_SIZE, BRANCH_PAD),
                Color::Series(col),
                node_hit(flat, i, x, y),
                series,
            );
            continue;
        }
        let (a, r) = polar(i);
        let mut dot = Mark::new(Shape::Circle {
            cx: x,
            cy: y,
            r: DOT_RADIUS,
        })
        .fill(Color::Series(col))
        .stroke(Color::Surface, SURFACE_GAP)
        .hit(node_hit(flat, i, x, y));
        let (lx, ly, anchor, baseline) = around(cx, cy, r + DOT_RADIUS + GAP / 2.0, a);
        let mut label = text(
            lx,
            ly,
            &flat.nodes[i].name,
            TICK_SIZE,
            Weight::Regular,
            anchor,
            baseline,
            Color::TextSecondary,
        );
        if let Some(s) = series {
            dot = dot.series(s);
            label = label.series(s);
        }
        marks.push(dot);
        marks.push(label);
    }
    // The central topic, over the links' common origin.
    let name = &flat.nodes[0].name;
    pill(
        &mut marks,
        measure,
        (cx, cy),
        name,
        (TITLE_SIZE, ROOT_PAD),
        Color::Axis,
        node_hit(flat, 0, cx, cy),
        None,
    );
    Ok(Scene {
        width,
        height,
        label: describe(
            title,
            &format!(
                "Mind map of {} around {:?}.",
                count(t.len() - 1, "idea", "ideas"),
                name
            ),
        ),
        marks,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;

    fn org() -> Node {
        Node::branch(
            "CEO",
            vec![
                Node::branch("CTO", vec![Node::new("Dev"), Node::new("Ops")]),
                Node::new("CFO"),
                Node::branch("COO", vec![Node::new("Sales")]),
            ],
        )
    }

    /// Every node's center and label, whether drawn as a dot or a pill.
    fn dots(s: &Scene) -> Vec<(f64, f64, String)> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (_, Some(h)) if h.kind != crate::scene::HitKind::Mark => None,
                (Shape::Circle { cx, cy, .. }, Some(h)) => Some((*cx, *cy, h.label.clone())),
                (Shape::Rect { rect, .. }, Some(h)) => Some((
                    rect.x + rect.w / 2.0,
                    rect.y + rect.h / 2.0,
                    h.label.clone(),
                )),
                _ => None,
            })
            .collect()
    }

    #[test]
    fn each_level_is_a_column_and_parents_sit_beside_their_children() {
        let s = TreeChart::new(org())
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let d = dots(&s);
        let get = |l: &str| d.iter().find(|n| n.2 == l).unwrap().clone();
        let (ceo, cto, cfo, dev, ops) = (
            get("CEO"),
            get("CTO"),
            get("CFO"),
            get("CTO / Dev"),
            get("CTO / Ops"),
        );
        assert!(ceo.0 < cto.0 && cto.0 < dev.0);
        assert!((cto.0 - cfo.0).abs() < 1e-9, "one column per level");
        assert!(
            (cto.1 - (dev.1 + ops.1) / 2.0).abs() < 1e-9,
            "centered on its children"
        );
    }

    #[test]
    fn mind_map_branches_spread_around_the_center_by_depth() {
        let s = BrainstormChart::new(org())
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let d = dots(&s);
        let get = |l: &str| d.iter().find(|n| n.2 == l).unwrap().clone();
        let c = get("CEO");
        let r = |l: &str| {
            let n = get(l);
            ((n.0 - c.0).powi(2) + (n.1 - c.1).powi(2)).sqrt()
        };
        assert!((r("CTO") - r("CFO")).abs() < 1e-6, "one ring per level");
        assert!((r("CTO / Dev") - 2.0 * r("CTO")).abs() < 1e-6);
    }

    #[test]
    fn each_main_branch_gets_its_own_hue_down_to_its_leaves() {
        let s = BrainstormChart::new(org())
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let mark = |l: &str| {
            s.marks
                .iter()
                .find(|m| {
                    m.hit
                        .as_ref()
                        .is_some_and(|h| h.label == l && h.kind == crate::scene::HitKind::Mark)
                })
                .unwrap()
        };
        // The branch's pill wears its hue as an outline; its ideas as dots.
        assert_eq!(mark("CTO").stroke.map(|k| k.color), Some(Color::Series(0)));
        assert_eq!(mark("CTO / Ops").fill, Some(Color::Series(0)));
        assert_eq!(mark("COO / Sales").fill, Some(Color::Series(2)));
    }

    #[test]
    fn branch_names_sit_inside_their_pills() {
        let s = BrainstormChart::new(org())
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let cto = dots(&s).into_iter().find(|n| n.2 == "CTO").unwrap();
        assert!(s.marks.iter().any(|m| matches!(
            &m.shape,
            Shape::Text { x, y, text, anchor: Anchor::Middle, .. }
                if text == "CTO" && (*x - cto.0).abs() < 1e-9 && (*y - cto.1).abs() < 1e-9
        )));
    }
}
