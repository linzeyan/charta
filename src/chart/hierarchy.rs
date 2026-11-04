use super::MAX_SERIES;
use crate::error::Error;
use crate::layout::hierarchy::Tree;

/// A node of a hierarchy: leaves carry values (where the chart sizes by
/// value), inner nodes carry children and take the sum of their leaves.
#[derive(Clone, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Node {
    pub name: String,
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub value: Option<f64>,
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Vec::is_empty")
    )]
    pub children: Vec<Node>,
}

impl Node {
    /// A node with neither value nor children (for trees and mind maps).
    pub fn new(name: impl Into<String>) -> Self {
        Node {
            name: name.into(),
            ..Self::default()
        }
    }

    pub fn leaf(name: impl Into<String>, value: f64) -> Self {
        Node {
            name: name.into(),
            value: Some(value),
            children: Vec::new(),
        }
    }

    pub fn branch(name: impl Into<String>, children: Vec<Node>) -> Self {
        Node {
            name: name.into(),
            value: None,
            children,
        }
    }
}

/// A hierarchy flattened for layout: the arena tree plus, per node, its
/// source node, its path of names below the root, and its top-level branch
/// (the root's child it descends from).
pub(crate) struct Flat<'a> {
    pub tree: Tree,
    pub nodes: Vec<&'a Node>,
    pub path: Vec<String>,
    pub branch: Vec<Option<usize>>,
    /// The root's children in data order: branch `b` is `tops[b]`. Kept
    /// apart from the tree, whose child order layouts may re-sort.
    tops: Vec<usize>,
}

impl Flat<'_> {
    /// The palette slot for node `i`: its top-level branch, when the root
    /// has a few branches to tell apart; otherwise one hue for all.
    pub fn color(&self, i: usize) -> u8 {
        if self.colored() {
            self.branch[i].unwrap_or(0) as u8
        } else {
            0
        }
    }

    /// Whether top-level branches get their own hues (and a legend): two
    /// to `MAX_SERIES` of them. More would need a ninth hue, which the
    /// palette never generates; one alone needs no key.
    pub fn colored(&self) -> bool {
        (2..=MAX_SERIES).contains(&self.tops.len())
    }

    /// Top-level branch names, for the legend (empty when uncolored).
    pub fn legend(&self) -> Vec<String> {
        if self.colored() {
            self.tops
                .iter()
                .map(|&c| self.nodes[c].name.clone())
                .collect()
        } else {
            Vec::new()
        }
    }

    /// The legend series a node belongs to, when colored.
    pub fn series(&self, i: usize) -> Option<&str> {
        if self.colored() {
            self.branch[i].map(|b| self.nodes[self.tops[b]].name.as_str())
        } else {
            None
        }
    }
}

/// Flattens `root` in preorder. With `values`, every leaf needs a finite,
/// non-negative value and inner nodes none (theirs is the sum), and the
/// total must be positive; without, values are ignored.
pub(crate) fn flatten(root: &Node, values: bool) -> Result<Flat<'_>, Error> {
    let mut flat = Flat {
        tree: Tree::default(),
        nodes: Vec::new(),
        path: Vec::new(),
        branch: Vec::new(),
        tops: Vec::new(),
    };
    let mut stack: Vec<(&Node, Option<usize>)> = vec![(root, None)];
    while let Some((node, parent)) = stack.pop() {
        let value = if values && node.children.is_empty() {
            match node.value {
                Some(v) if v.is_finite() && v >= 0.0 => v,
                Some(_) => {
                    return Err(Error::InvalidData(format!(
                        "node {:?} needs a finite value of at least zero",
                        node.name
                    )));
                }
                None => {
                    return Err(Error::InvalidData(format!(
                        "leaf {:?} has no value",
                        node.name
                    )));
                }
            }
        } else {
            if values && node.value.is_some() {
                return Err(Error::InvalidData(format!(
                    "node {:?} has both children and a value; its value is the sum of its children",
                    node.name
                )));
            }
            0.0
        };
        let i = flat.tree.push(parent, value);
        flat.nodes.push(node);
        let (path, branch) = match parent {
            None => (String::new(), None),
            Some(0) => (node.name.clone(), Some(flat.tree.children[0].len() - 1)),
            Some(p) => (format!("{} / {}", flat.path[p], node.name), flat.branch[p]),
        };
        flat.path.push(path);
        flat.branch.push(branch);
        stack.extend(node.children.iter().rev().map(|c| (c, Some(i))));
    }
    flat.tree.sum();
    flat.tops = flat.tree.children[0].clone();
    if values && !root.children.is_empty() && flat.tree.value[0] <= 0.0 {
        return Err(Error::InvalidData(
            "every leaf is zero: there is nothing to size".into(),
        ));
    }
    Ok(flat)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn sample() -> Node {
        Node::branch(
            "root",
            vec![
                Node::branch("A", vec![Node::leaf("a1", 1.0), Node::leaf("a2", 2.0)]),
                Node::leaf("B", 4.0),
            ],
        )
    }

    #[test]
    fn flattening_sums_values_and_records_paths_and_branches() {
        let root = sample();
        let f = flatten(&root, true).unwrap();
        let names: Vec<&str> = f.nodes.iter().map(|n| n.name.as_str()).collect();
        assert_eq!(names, ["root", "A", "a1", "a2", "B"]);
        assert_eq!(f.tree.value, [7.0, 3.0, 1.0, 2.0, 4.0]);
        assert_eq!(f.path[3], "A / a2");
        assert_eq!(f.branch, [None, Some(0), Some(0), Some(0), Some(1)]);
        assert_eq!(f.legend(), ["A", "B"]);
    }

    #[test]
    fn re_sorting_the_tree_keeps_branch_colors_and_legend_in_data_order() {
        // B is larger, so a by-value sort puts it first; its hue and legend
        // slot must still be the second, or the legend would name the
        // wrong colors.
        let root = sample();
        let mut f = flatten(&root, true).unwrap();
        f.tree.sort_by_value();
        assert_eq!(f.legend(), ["A", "B"]);
        assert_eq!((f.color(4), f.series(4)), (1, Some("B")));
    }

    #[test]
    fn values_must_be_on_leaves_only() {
        let leafless = Node::branch("r", vec![Node::new("x")]);
        assert!(flatten(&leafless, true).is_err());
        assert!(flatten(&leafless, false).is_ok());
        let mut both = sample();
        both.children[0].value = Some(3.0);
        assert!(flatten(&both, true).is_err());
        let zero = Node::branch("r", vec![Node::leaf("x", 0.0)]);
        assert!(flatten(&zero, true).is_err());
    }
}
