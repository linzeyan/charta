use super::frame::{self, Key, text};
use super::hierarchy::{Flat, Node, flatten};
use super::{Chart, PAD, TICK_SIZE, check_size, count, describe};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::layout::hierarchy::squarify;
use crate::scene::{Anchor, Baseline, Corners, Hit, Mark, Rect, Scene, Shape, TipRow};

/// Header strip naming a group above its cells.
const HEADER: f64 = 16.0;
/// Space between a group's edge and its cells.
const INSET: f64 = 2.0;
const LABEL_PAD: f64 = 4.0;
const RADIUS: f64 = 2.0;

/// Nested rectangles, each leaf's area proportional to its value, grouped
/// by branch (squarified, so cells stay close to square).
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct TreemapChart {
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub title: Option<String>,
    pub root: Node,
}

impl TreemapChart {
    pub fn new(root: Node) -> Self {
        TreemapChart { title: None, root }
    }

    pub fn title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }
}

struct Ctx<'a, 'b> {
    flat: &'b Flat<'a>,
    measure: &'b dyn TextMeasure,
    marks: Vec<Mark>,
}

impl Ctx<'_, '_> {
    fn layout(&mut self, i: usize, (x0, y0, x1, y1): (f64, f64, f64, f64)) {
        let f = self.flat;
        let kids = &f.tree.children[i];
        if kids.is_empty() {
            self.leaf(i, (x0, y0, x1, y1));
            return;
        }
        let (w, h) = (x1 - x0, y1 - y0);
        let mut inner = (x0, y0, x1, y1);
        // Groups below the root get a header naming them, room permitting.
        if i != 0 {
            let name = &f.nodes[i].name;
            let tw = self.measure.width(name, TICK_SIZE, Weight::Semibold);
            if h >= 2.0 * HEADER && w >= tw.min(40.0) + 2.0 * LABEL_PAD {
                let label = if tw > w - 2.0 * LABEL_PAD {
                    ellipsize(self.measure, name, w - 2.0 * LABEL_PAD)
                } else {
                    name.clone()
                };
                let mut t = text(
                    x0 + LABEL_PAD,
                    y0 + HEADER / 2.0,
                    &label,
                    TICK_SIZE,
                    Weight::Semibold,
                    Anchor::Start,
                    Baseline::Middle,
                    Color::TextSecondary,
                );
                if let Some(s) = f.series(i) {
                    t = t.series(s);
                }
                self.marks.push(t);
                inner = (x0 + INSET, y0 + HEADER, x1 - INSET, y1 - INSET);
            } else {
                inner = (x0 + INSET, y0 + INSET, x1 - INSET, y1 - INSET);
            }
        }
        if inner.2 <= inner.0 || inner.3 <= inner.1 {
            return;
        }
        let values: Vec<f64> = kids.iter().map(|&k| f.tree.value[k]).collect();
        let cells = squarify(&values, inner);
        for (&k, cell) in kids.iter().zip(cells) {
            if f.tree.value[k] > 0.0 {
                self.layout(k, cell);
            }
        }
    }

    fn leaf(&mut self, i: usize, (x0, y0, x1, y1): (f64, f64, f64, f64)) {
        let f = self.flat;
        let c = f.color(i);
        // Half the 2px surface gap on every side of every cell.
        let r = Rect {
            x: x0 + 1.0,
            y: y0 + 1.0,
            w: (x1 - x0 - 2.0).max(0.0),
            h: (y1 - y0 - 2.0).max(0.0),
        };
        let v = f.tree.value[i];
        let total = f.tree.value[0];
        // A wash with an edge in the branch hue keeps the labels dark text
        // on every hue, which a solid fill would not.
        let mut cell = Mark::new(Shape::Rect {
            rect: r,
            radius: Corners::all(RADIUS.min(r.w / 2.0).min(r.h / 2.0)),
        })
        .fill(Color::Wash(c))
        .stroke(Color::Series(c), 1.0)
        .hit(
            Hit::new(i.to_string(), f.path[i].as_str(), format::value(v)).rows(vec![TipRow {
                color: None,
                name: "share".into(),
                value: format!("{}%", format::approx(v / total * 100.0)),
            }]),
        );
        if let Some(s) = f.series(i) {
            cell = cell.series(s);
        }
        self.marks.push(cell);
        let name = &f.nodes[i].name;
        let room = r.w - 2.0 * LABEL_PAD;
        if r.h >= TICK_SIZE + 2.0 * LABEL_PAD
            && self.measure.width(name, TICK_SIZE, Weight::Semibold) <= room
        {
            let mut lines = vec![(name.clone(), Weight::Semibold, Color::TextPrimary)];
            let value = format::value(v);
            if r.h >= 2.0 * TICK_SIZE + 3.0 * LABEL_PAD
                && self.measure.width(&value, TICK_SIZE, Weight::Regular) <= room
            {
                lines.push((value, Weight::Regular, Color::TextSecondary));
            }
            for (k, (t, weight, color)) in lines.into_iter().enumerate() {
                let mut m = text(
                    r.x + LABEL_PAD,
                    r.y + LABEL_PAD + k as f64 * (TICK_SIZE + LABEL_PAD),
                    &t,
                    TICK_SIZE,
                    weight,
                    Anchor::Start,
                    Baseline::Top,
                    color,
                );
                if let Some(s) = f.series(i) {
                    m = m.series(s);
                }
                self.marks.push(m);
            }
        }
    }
}

/// `s` cut to fit `max` px, ending in an ellipsis.
fn ellipsize(measure: &dyn TextMeasure, s: &str, max: f64) -> String {
    let chars: Vec<char> = s.chars().collect();
    for n in (0..chars.len()).rev() {
        let t: String = chars[..n].iter().collect::<String>() + "…";
        if measure.width(&t, TICK_SIZE, Weight::Semibold) <= max {
            return t;
        }
    }
    String::new()
}

impl Chart for TreemapChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let mut flat = flatten(&self.root, true)?;
        flat.tree.sort_by_value();
        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &flat.legend(), Key::Swatch);
        let leaves = (0..flat.tree.len())
            .filter(|&i| flat.tree.children[i].is_empty())
            .count();
        let mut ctx = Ctx {
            flat: &flat,
            measure,
            marks,
        };
        let rect = (PAD, top, (width - PAD).max(PAD), (height - PAD).max(top));
        if !self.root.children.is_empty() {
            ctx.layout(0, rect);
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!("Treemap of {}.", count(leaves, "item", "items")),
            ),
            marks: ctx.marks,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;

    fn cells(s: &Scene) -> Vec<(Rect, &Hit)> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Rect { rect, .. }, Some(h)) if h.kind == crate::scene::HitKind::Mark => {
                    Some((*rect, h))
                }
                _ => None,
            })
            .collect()
    }

    #[test]
    fn a_flat_treemap_gives_each_leaf_area_in_proportion() {
        let c = TreemapChart::new(Node::branch(
            "r",
            vec![
                Node::leaf("a", 1.0),
                Node::leaf("b", 3.0),
                Node::leaf("c", 6.0),
            ],
        ));
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        // The 1px inset on each side is the gap, not data: add it back.
        let area = |r: Rect| (r.w + 2.0) * (r.h + 2.0);
        let mut by: Vec<(String, f64)> = cells(&s)
            .iter()
            .map(|(r, h)| (h.label.clone(), area(*r)))
            .collect();
        by.sort_by(|a, b| a.0.cmp(&b.0));
        assert!((by[1].1 / by[0].1 - 3.0).abs() < 1e-6, "{by:?}");
        assert!((by[2].1 / by[0].1 - 6.0).abs() < 1e-6, "{by:?}");
    }

    #[test]
    fn groups_nest_their_cells_and_color_them_by_branch() {
        let c = TreemapChart::new(Node::branch(
            "r",
            vec![
                Node::branch("A", vec![Node::leaf("a1", 2.0), Node::leaf("a2", 2.0)]),
                Node::branch("B", vec![Node::leaf("b1", 4.0)]),
            ],
        ));
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let fill = |label: &str| {
            s.marks
                .iter()
                .find(|m| m.hit.as_ref().is_some_and(|h| h.label == label))
                .and_then(|m| m.fill)
        };
        assert_eq!(fill("A / a1"), Some(Color::Wash(0)));
        assert_eq!(fill("B / b1"), Some(Color::Wash(1)));
        // The header names each group above its cells.
        assert!(
            s.marks
                .iter()
                .any(|m| matches!(&m.shape, Shape::Text { text, .. } if text == "A"))
        );
    }

    #[test]
    fn zero_leaves_get_no_cell() {
        let c = TreemapChart::new(Node::branch(
            "r",
            vec![Node::leaf("a", 1.0), Node::leaf("z", 0.0)],
        ));
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        assert_eq!(cells(&s).len(), 1);
    }
}
