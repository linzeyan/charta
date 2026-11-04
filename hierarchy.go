package charta

import (
	"slices"

	"github.com/linzeyan/charta/internal/layout"
)

// Node is a node of a hierarchy: leaves carry values (where the chart sizes
// by value), inner nodes carry children and take the sum of their leaves.
type Node struct {
	Name     string   `json:"name"`
	Value    *float64 `json:"value,omitempty"`
	Children []Node   `json:"children,omitempty"`
}

// flatHierarchy is a hierarchy flattened for layout: the arena tree plus,
// per node, its source node, its path of names below the root, and its
// top-level branch (the root's child it descends from; -1 for the root).
type flatHierarchy struct {
	tree   layout.Tree
	nodes  []*Node
	path   []string
	branch []int
	// tops are the root's children in data order: branch b is tops[b]. Kept
	// apart from the tree, whose child order layouts may re-sort.
	tops []int
}

// colored: top-level branches get their own hues (and a legend) when there
// are two to maxSeries of them. More would need a ninth hue, which the
// palette never generates; one alone needs no key.
func (f *flatHierarchy) colored() bool { return len(f.tops) >= 2 && len(f.tops) <= maxSeries }

// color is the palette slot for node i: its top-level branch when colored,
// otherwise one hue for all.
func (f *flatHierarchy) color(i int) int {
	if f.colored() && f.branch[i] >= 0 {
		return f.branch[i]
	}
	return 0
}

// legend is the top-level branch names (none when uncolored).
func (f *flatHierarchy) legend() []string {
	if !f.colored() {
		return nil
	}
	names := make([]string, len(f.tops))
	for b, c := range f.tops {
		names[b] = f.nodes[c].Name
	}
	return names
}

// series is the legend series node i belongs to, when colored; "" for none.
func (f *flatHierarchy) series(i int) string {
	if !f.colored() || f.branch[i] < 0 {
		return ""
	}
	return f.nodes[f.tops[f.branch[i]]].Name
}

// leaves counts the nodes without children.
func (f *flatHierarchy) leaves() int {
	n := 0
	for _, c := range f.tree.Children {
		if len(c) == 0 {
			n++
		}
	}
	return n
}

// maxDepth is the depth of the deepest node.
func (f *flatHierarchy) maxDepth() int {
	d := 0
	for _, v := range f.tree.Depth {
		d = max(d, v)
	}
	return d
}

// flatten flattens root in preorder. With values, every leaf needs a finite,
// non-negative value and inner nodes none (theirs is the sum), and the total
// must be positive; without, values are ignored.
func flatten(root *Node, values bool) (*flatHierarchy, error) {
	f := &flatHierarchy{}
	type item struct {
		node   *Node
		parent int
	}
	stack := []item{{root, -1}}
	for len(stack) > 0 {
		it := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		node := it.node
		value := 0.0
		if values && len(node.Children) == 0 {
			switch {
			case node.Value == nil:
				return nil, invalid("leaf %q has no value", node.Name)
			case !isFinite(*node.Value) || !(*node.Value >= 0):
				return nil, invalid("node %q needs a finite value of at least zero", node.Name)
			}
			value = *node.Value
		} else if values && node.Value != nil {
			return nil, invalid("node %q has both children and a value; its value is the sum of its children", node.Name)
		}
		i := f.tree.Push(it.parent, value)
		f.nodes = append(f.nodes, node)
		switch p := it.parent; p {
		case -1:
			f.path = append(f.path, "")
			f.branch = append(f.branch, -1)
		case 0:
			f.path = append(f.path, node.Name)
			f.branch = append(f.branch, len(f.tree.Children[0])-1)
		default:
			f.path = append(f.path, f.path[p]+" / "+node.Name)
			f.branch = append(f.branch, f.branch[p])
		}
		for k := len(node.Children) - 1; k >= 0; k-- {
			stack = append(stack, item{&node.Children[k], i})
		}
	}
	f.tree.Sum()
	f.tops = slices.Clone(f.tree.Children[0])
	if values && len(root.Children) > 0 && f.tree.Value[0] <= 0 {
		return nil, invalid("every leaf is zero: there is nothing to size")
	}
	return f, nil
}
