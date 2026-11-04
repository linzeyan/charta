package charta

import (
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/layout"
	"github.com/linzeyan/charta/scene"
)

const (
	treeLink = 1.5
	// treeTrunk: mind-map links from the center are drawn heavier; they are
	// the branches everything else hangs from.
	treeTrunk     = 2.5
	treeRootPad   = 8.0
	treeBranchPad = 5.0
)

// TreeChart shows a hierarchy as a node-link diagram, root on the left and
// each level a column further right (a tidy tree: subtrees never overlap and
// parents sit centered beside their children).
type TreeChart struct {
	Title string `json:"title,omitempty"`
	Root  Node   `json:"root"`
}

func (c TreeChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	flat, err := flatten(&c.Root, false)
	if err != nil {
		return nil, err
	}
	return treeDiagram(c.Title, flat, width, height, m), nil
}

// BrainstormChart is a mind map: the central topic in the middle, ideas
// branching outward in every direction, each main branch in its own hue.
type BrainstormChart struct {
	Title string `json:"title,omitempty"`
	Root  Node   `json:"root"`
}

func (c BrainstormChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	flat, err := flatten(&c.Root, false)
	if err != nil {
		return nil, err
	}
	return brainstorm(c.Title, flat, width, height, m), nil
}

// nodeHit is the node's tooltip: its path, its value if it has one, and how
// many children it has.
func nodeHit(f *flatHierarchy, i int, x, y float64) *scene.Hit {
	label := f.path[i]
	if i == 0 {
		label = f.nodes[0].Name
	}
	value := ""
	if v := f.nodes[i].Value; v != nil {
		value = base.Value(*v)
	}
	var rows []scene.TipRow
	if kids := len(f.tree.Children[i]); kids > 0 {
		rows = []scene.TipRow{{Name: "children", Value: strconv.Itoa(kids)}}
	}
	return &scene.Hit{
		Key:   strconv.Itoa(i),
		Label: label,
		Value: value,
		Rows:  rows,
		Area:  &scene.Rect{X: x - minTarget/2, Y: y - minTarget/2, W: minTarget, H: minTarget},
	}
}

// pill is a rounded label box centered on (x, y): surface fill, edge
// outline, the text inside; the whole box is the hover target.
func pill(marks *[]scene.Mark, m base.TextMeasure, x, y float64, name string, size, padding float64, edge base.Color, hit *scene.Hit, series string) {
	w := m.Width(name, size, base.Semibold) + float64(2*padding)
	h := size + float64(2*padding)
	rect := scene.Rect{X: x - w/2, Y: y - h/2, W: w, H: h}
	hit.Area = &rect
	*marks = append(*marks, scene.Mark{
		Shape:  scene.RectShape{Rect: rect, Radius: scene.AllCorners(h / 2)},
		Fill:   base.Surface,
		Stroke: &scene.Stroke{Color: edge, Width: treeLink},
		Series: series,
		Hit:    hit,
	})
	label := text(x, y, name, size, base.Semibold, scene.Middle, scene.Center, base.TextPrimary)
	label.Series = series
	*marks = append(*marks, label)
}

func treeDiagram(title string, f *flatHierarchy, width, height float64, m base.TextMeasure) *scene.Scene {
	t := &f.tree
	var marks []scene.Mark
	top := drawTitle(&marks, title)
	bottom := height - pad
	labelW := func(i int) float64 { return m.Width(f.nodes[i].Name, tickSize, base.Regular) }
	leafW := 0.0
	for i := 1; i < t.Len(); i++ {
		if len(t.Children[i]) == 0 {
			leafW = math.Max(leafW, labelW(i))
		}
	}
	left := pad + labelW(0) + gap
	right := math.Max(width-pad-leafW-gap, left+1)
	depth := float64(max(f.maxDepth(), 1))
	breadth := layout.Tidy(t, func(same bool, _ int) float64 {
		if same {
			return 1
		}
		return 2
	})
	pos := func(i int) (float64, float64) {
		return left + (right-left)*float64(t.Depth[i])/depth, top + float64((bottom-top)*breadth[i])
	}
	for i := 1; i < t.Len(); i++ {
		px, py := pos(t.Parent[i])
		cx, cy := pos(i)
		mx := (px + cx) / 2
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: []scene.PathOp{scene.Move(px, py), scene.Cubic(mx, py, mx, cy, cx, cy)}},
			Stroke: &scene.Stroke{Color: base.Axis, Width: treeLink},
		})
	}
	for i := range t.Len() {
		x, y := pos(i)
		leaf := len(t.Children[i]) == 0 && i != 0
		// Inner nodes solid, leaves hollow: the shape says which nodes
		// branch further.
		mk := scene.Mark{Shape: scene.CircleShape{CX: x, CY: y, R: dotRadius}, Hit: nodeHit(f, i, x, y)}
		if leaf {
			mk.Fill, mk.Stroke = base.Surface, &scene.Stroke{Color: base.Series(0), Width: treeLink}
		} else {
			mk.Fill, mk.Stroke = base.Series(0), &scene.Stroke{Color: base.Surface, Width: surfaceGap}
		}
		marks = append(marks, mk)
		// Leaves label to the right; inner nodes to the left, over the link
		// coming in, on a backing.
		name := f.nodes[i].Name
		if leaf {
			marks = append(marks, text(x+dotRadius+gap/2, y, name, tickSize, base.Regular, scene.Start, scene.Center, base.TextPrimary))
		} else {
			haloLabel(&marks, m, x-dotRadius-gap/2, y, name, scene.End, base.TextPrimary)
		}
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(title, fmt.Sprintf("Tree diagram of %s in %s.",
			count(t.Len(), "node", "nodes"), count(int(depth)+1, "level", "levels"))),
		Marks: marks,
	}
}

func brainstorm(title string, f *flatHierarchy, width, height float64, m base.TextMeasure) *scene.Scene {
	t := &f.tree
	var marks []scene.Mark
	top := drawTitle(&marks, title)
	top = legend(&marks, m, top, width, f.legend(), keyLine)
	bottom := height - pad
	cx, cy := width/2, (top+bottom)/2
	names := make([]string, 0, t.Len())
	for i := 1; i < t.Len(); i++ {
		names = append(names, f.nodes[i].Name)
	}
	roomX, roomY := labelRoom(m, names)
	radius := math.Max(math.Min((width-2*pad)/2-roomX-dotRadius, (bottom-top)/2-roomY-dotRadius), 1)
	depth := float64(max(f.maxDepth(), 1))
	// Deeper rings are longer, so neighbours there need less angle; d3's
	// radial separation divides by depth.
	breadth := layout.Tidy(t, func(same bool, d int) float64 {
		s := 2.0
		if same {
			s = 1
		}
		return s / float64(max(d, 1))
	})
	polar := func(i int) (float64, float64) {
		return tau * breadth[i], radius * float64(t.Depth[i]) / depth
	}
	pos := func(i int) (float64, float64) {
		a, r := polar(i)
		return polarAt(cx, cy, r, a)
	}
	for i := 1; i < t.Len(); i++ {
		p := t.Parent[i]
		a0, r0 := polar(p)
		a1, r1 := polar(i)
		mid := (r0 + r1) / 2
		x0, y0 := pos(p)
		c1x, c1y := polarAt(cx, cy, mid, a0)
		c2x, c2y := polarAt(cx, cy, mid, a1)
		x1, y1 := pos(i)
		lw := treeLink
		if p == 0 {
			lw = treeTrunk
		}
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: []scene.PathOp{scene.Move(x0, y0), scene.Cubic(c1x, c1y, c2x, c2y, x1, y1)}},
			Stroke: &scene.Stroke{Color: base.Series(f.color(i)), Width: lw},
			Series: f.series(i),
		})
	}
	for i := 1; i < t.Len(); i++ {
		x, y := pos(i)
		col := f.color(i)
		series := f.series(i)
		// Main branches are pills with their name inside: a label beside the
		// node would sit on the links fanning out from it.
		if t.Depth[i] == 1 {
			pill(&marks, m, x, y, f.nodes[i].Name, tickSize, treeBranchPad, base.Series(col), nodeHit(f, i, x, y), series)
			continue
		}
		a, r := polar(i)
		marks = append(marks, scene.Mark{
			Shape:  scene.CircleShape{CX: x, CY: y, R: dotRadius},
			Fill:   base.Series(col),
			Stroke: &scene.Stroke{Color: base.Surface, Width: surfaceGap},
			Series: series,
			Hit:    nodeHit(f, i, x, y),
		})
		lx, ly, anchor, baseline := around(cx, cy, r+dotRadius+gap/2, a)
		label := text(lx, ly, f.nodes[i].Name, tickSize, base.Regular, anchor, baseline, base.TextSecondary)
		label.Series = series
		marks = append(marks, label)
	}
	// The central topic, over the links' common origin.
	name := f.nodes[0].Name
	pill(&marks, m, cx, cy, name, titleSize, treeRootPad, base.Axis, nodeHit(f, 0, cx, cy), "")
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(title, fmt.Sprintf("Mind map of %s around %q.",
			count(t.Len()-1, "idea", "ideas"), name)),
		Marks: marks,
	}
}
