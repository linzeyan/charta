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
	// treemapHeader is the strip naming a group above its cells.
	treemapHeader = 16.0
	// treemapInset is the space between a group's edge and its cells.
	treemapInset    = 2.0
	treemapLabelPad = 4.0
	treemapRadius   = 2.0
)

// TreemapChart shows nested rectangles, each leaf's area proportional to its
// value, grouped by branch (squarified, so cells stay close to square).
type TreemapChart struct {
	Title string `json:"title,omitempty"`
	Root  Node   `json:"root"`
}

type treemapCtx struct {
	flat  *flatHierarchy
	m     base.TextMeasure
	marks []scene.Mark
}

func (c *treemapCtx) layout(i int, b layout.Cell) {
	f := c.flat
	kids := f.tree.Children[i]
	if len(kids) == 0 {
		c.leaf(i, b)
		return
	}
	w, h := b.X1-b.X0, b.Y1-b.Y0
	inner := b
	// Groups below the root get a header naming them, room permitting.
	if i != 0 {
		name := f.nodes[i].Name
		tw := c.m.Width(name, tickSize, base.Semibold)
		if h >= 2*treemapHeader && w >= math.Min(tw, 40)+2*treemapLabelPad {
			label := name
			if tw > w-2*treemapLabelPad {
				label = ellipsize(c.m, name, w-2*treemapLabelPad)
			}
			t := text(b.X0+treemapLabelPad, b.Y0+treemapHeader/2, label, tickSize, base.Semibold, scene.Start, scene.Center, base.TextSecondary)
			t.Series = f.series(i)
			c.marks = append(c.marks, t)
			inner = layout.Cell{X0: b.X0 + treemapInset, Y0: b.Y0 + treemapHeader, X1: b.X1 - treemapInset, Y1: b.Y1 - treemapInset}
		} else {
			inner = layout.Cell{X0: b.X0 + treemapInset, Y0: b.Y0 + treemapInset, X1: b.X1 - treemapInset, Y1: b.Y1 - treemapInset}
		}
	}
	if inner.X1 <= inner.X0 || inner.Y1 <= inner.Y0 {
		return
	}
	values := make([]float64, len(kids))
	for j, k := range kids {
		values[j] = f.tree.Value[k]
	}
	cells := layout.Squarify(values, inner)
	for j, k := range kids {
		if f.tree.Value[k] > 0 {
			c.layout(k, cells[j])
		}
	}
}

func (c *treemapCtx) leaf(i int, b layout.Cell) {
	f := c.flat
	col := f.color(i)
	// Half the 2px surface gap on every side of every cell.
	r := scene.Rect{
		X: b.X0 + 1,
		Y: b.Y0 + 1,
		W: math.Max(b.X1-b.X0-2, 0),
		H: math.Max(b.Y1-b.Y0-2, 0),
	}
	v := f.tree.Value[i]
	total := f.tree.Value[0]
	series := f.series(i)
	// A wash with an edge in the branch hue keeps the labels dark text on
	// every hue, which a solid fill would not.
	c.marks = append(c.marks, scene.Mark{
		Shape:  scene.RectShape{Rect: r, Radius: scene.AllCorners(math.Min(math.Min(treemapRadius, r.W/2), r.H/2))},
		Fill:   base.Wash(col),
		Stroke: &scene.Stroke{Color: base.Series(col), Width: 1},
		Series: series,
		Hit: &scene.Hit{
			Key:   strconv.Itoa(i),
			Label: f.path[i],
			Value: base.Value(v),
			Rows:  []scene.TipRow{{Name: "share", Value: base.Approx(v/total*100) + "%"}},
		},
	})
	name := f.nodes[i].Name
	room := r.W - 2*treemapLabelPad
	if r.H >= tickSize+2*treemapLabelPad && c.m.Width(name, tickSize, base.Semibold) <= room {
		type line struct {
			s     string
			w     base.Weight
			color base.Color
		}
		lines := []line{{name, base.Semibold, base.TextPrimary}}
		value := base.Value(v)
		if r.H >= 2*tickSize+3*treemapLabelPad && c.m.Width(value, tickSize, base.Regular) <= room {
			lines = append(lines, line{value, base.Regular, base.TextSecondary})
		}
		for k, l := range lines {
			t := text(r.X+treemapLabelPad, r.Y+treemapLabelPad+float64(float64(k)*(tickSize+treemapLabelPad)), l.s, tickSize, l.w, scene.Start, scene.Top, l.color)
			t.Series = series
			c.marks = append(c.marks, t)
		}
	}
}

// ellipsize cuts s to fit max px, ending in an ellipsis.
func ellipsize(m base.TextMeasure, s string, max float64) string {
	chars := []rune(s)
	for n := len(chars) - 1; n >= 0; n-- {
		t := string(chars[:n]) + "…"
		if m.Width(t, tickSize, base.Semibold) <= max {
			return t
		}
	}
	return ""
}

func (c TreemapChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	flat, err := flatten(&c.Root, true)
	if err != nil {
		return nil, err
	}
	flat.tree.SortByValue()
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, flat.legend(), keySwatch)
	ctx := &treemapCtx{flat: flat, m: m, marks: marks}
	if len(c.Root.Children) > 0 {
		ctx.layout(0, layout.Cell{X0: pad, Y0: top, X1: math.Max(width-pad, pad), Y1: math.Max(height-pad, top)})
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Treemap of %s.", count(flat.leaves(), "item", "items"))),
		Marks:  ctx.marks,
	}, nil
}
