//! The input shared by the relationship charts: weighted links between
//! named nodes. Nodes are not listed separately; they appear in the order
//! links first mention them, so the same data always lays out the same way.

use super::MAX_SERIES;
use crate::error::Error;
use crate::scene::PathOp;

/// A weighted connection from `source` to `target`. Charts that ignore
/// direction (network, arc, non-ribbon chord) treat it as undirected.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Link {
    pub source: String,
    pub target: String,
    pub value: f64,
}

impl Link {
    pub fn new(source: impl Into<String>, target: impl Into<String>, value: f64) -> Self {
        Link {
            source: source.into(),
            target: target.into(),
            value,
        }
    }
}

/// Links resolved to node indices.
pub(crate) struct Graph {
    pub names: Vec<String>,
    /// (source, target, value), in input order.
    pub edges: Vec<(usize, usize, f64)>,
}

impl Graph {
    /// Every node gets its own palette slot when there are few enough;
    /// past that, one hue for all rather than a generated ninth.
    pub fn color(&self, i: usize) -> u8 {
        if self.colored() { i as u8 } else { 0 }
    }

    pub fn colored(&self) -> bool {
        self.names.len() <= MAX_SERIES
    }

    /// Node names for the legend: only when nodes carry distinct colors.
    pub fn legend(&self) -> Vec<String> {
        if self.colored() {
            self.names.clone()
        } else {
            Vec::new()
        }
    }

    /// Sum of the values of the links touching each node.
    pub fn strength(&self) -> Vec<f64> {
        let mut s = vec![0.0; self.names.len()];
        for &(a, b, v) in &self.edges {
            s[a] += v;
            s[b] += v;
        }
        s
    }
}

/// Values must be positive and finite (a zero link is no link), no node
/// links to itself, and each pair appears once: in both directions too when
/// the chart ignores direction, since the two would draw over each other.
pub(crate) fn graph(links: &[Link], directed: bool) -> Result<Graph, Error> {
    let mut names: Vec<String> = Vec::new();
    let index = |n: &str, names: &mut Vec<String>| match names.iter().position(|m| m == n) {
        Some(i) => i,
        None => {
            names.push(n.to_string());
            names.len() - 1
        }
    };
    let mut edges = Vec::with_capacity(links.len());
    for l in links {
        if !(l.value.is_finite() && l.value > 0.0) {
            return Err(Error::InvalidData(format!(
                "link {:?} → {:?} needs a positive value",
                l.source, l.target
            )));
        }
        if l.source == l.target {
            return Err(Error::InvalidData(format!(
                "link {:?} → {:?} connects a node to itself",
                l.source, l.target
            )));
        }
        let (a, b) = (index(&l.source, &mut names), index(&l.target, &mut names));
        if edges
            .iter()
            .any(|&(s, t, _)| (s, t) == (a, b) || (!directed && (s, t) == (b, a)))
        {
            return Err(Error::InvalidData(format!(
                "link {:?} → {:?} is given twice",
                l.source, l.target
            )));
        }
        edges.push((a, b, l.value));
    }
    Ok(Graph { names, edges })
}

/// A chart whose whole input is a list of links, with the builder every
/// such chart shares. `$build` turns the resolved graph into the scene.
macro_rules! links_chart {
    ($(#[$doc:meta])* $name:ident, $directed:expr, $build:ident) => {
        $(#[$doc])*
        #[derive(Clone, Debug, Default, PartialEq)]
        #[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
        pub struct $name {
            #[cfg_attr(
                feature = "serde",
                serde(default, skip_serializing_if = "Option::is_none")
            )]
            pub title: Option<String>,
            pub links: Vec<Link>,
        }

        impl $name {
            pub fn new() -> Self {
                $name::default()
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

        impl Chart for $name {
            fn scene(
                &self,
                width: f64,
                height: f64,
                measure: &dyn TextMeasure,
            ) -> Result<Scene, Error> {
                check_size(width, height)?;
                let g = graph(&self.links, $directed)?;
                $build(self.title.as_deref(), &g, width, height, measure)
            }
        }
    };
}

pub(crate) use links_chart;

/// A flow band of thickness `w` from `(x0, y0)` to `(x1, y1)` (top edges),
/// easing horizontally at both ends so it meets each bar square-on.
pub(crate) fn band(x0: f64, y0: f64, x1: f64, y1: f64, w: f64) -> Vec<PathOp> {
    let xm = (x0 + x1) / 2.0;
    vec![
        PathOp::Move(x0, y0),
        PathOp::Cubic(xm, y0, xm, y1, x1, y1),
        PathOp::Line(x1, y1 + w),
        PathOp::Cubic(xm, y1 + w, xm, y0 + w, x0, y0 + w),
        PathOp::Close,
    ]
}

/// Appends a quadratic Bézier from the current point `from` through the
/// control point `c` to `to`, as the cubic that traces it exactly (paths
/// carry cubics only).
pub(crate) fn quad(ops: &mut Vec<PathOp>, from: (f64, f64), c: (f64, f64), to: (f64, f64)) {
    ops.push(PathOp::Cubic(
        from.0 + 2.0 / 3.0 * (c.0 - from.0),
        from.1 + 2.0 / 3.0 * (c.1 - from.1),
        to.0 + 2.0 / 3.0 * (c.0 - to.0),
        to.1 + 2.0 / 3.0 * (c.1 - to.1),
        to.0,
        to.1,
    ));
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn nodes_follow_first_mention() {
        let g = graph(&[Link::new("b", "a", 1.0), Link::new("c", "b", 2.0)], true).unwrap();
        assert_eq!(g.names, ["b", "a", "c"]);
        assert_eq!(g.edges, [(0, 1, 1.0), (2, 0, 2.0)]);
        assert_eq!(g.strength(), [3.0, 1.0, 2.0]);
    }

    #[test]
    fn a_reversed_pair_is_a_duplicate_only_when_direction_is_ignored() {
        let l = [Link::new("a", "b", 1.0), Link::new("b", "a", 1.0)];
        assert!(graph(&l, true).is_ok());
        assert!(graph(&l, false).is_err());
    }

    #[test]
    fn bad_links_are_rejected() {
        for l in [
            Link::new("a", "b", 0.0),
            Link::new("a", "b", -1.0),
            Link::new("a", "b", f64::NAN),
            Link::new("a", "a", 1.0),
        ] {
            assert!(graph(&[l], true).is_err());
        }
    }

    #[test]
    fn a_ninth_node_turns_colors_off() {
        let eight: Vec<Link> = (1..8)
            .map(|i| Link::new("hub", format!("n{i}"), 1.0))
            .collect();
        let g = graph(&eight, false).unwrap();
        assert!(g.colored() && g.color(7) == 7 && g.legend().len() == 8);
        let mut nine = eight.clone();
        nine.push(Link::new("hub", "n8", 1.0));
        let g = graph(&nine, false).unwrap();
        assert!(!g.colored() && g.color(8) == 0 && g.legend().is_empty());
    }
}
