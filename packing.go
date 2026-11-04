package charta

import (
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/layout"
	"github.com/linzeyan/charta/scene"
)

// packPadding is the space between sibling circles and around them inside
// their parent.
const packPadding = 3.0

// CirclePackingChart shows nested circles: each leaf's area proportional to
// its value, siblings packed tightly inside their parent.
type CirclePackingChart struct {
	Title string `json:"title,omitempty"`
	Root  Node   `json:"root"`
}

func (c CirclePackingChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
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
	w, h := math.Max(width-2*pad, 1), math.Max(height-pad-top, 1)
	total := flat.tree.Value[0]

	if len(c.Root.Children) > 0 {
		circles := layout.Pack(&flat.tree, w, h, packPadding)
		// Parents before children (index order is preorder), so leaves land
		// on top and win the hover.
		for i := 1; i < len(circles); i++ {
			ci := circles[i]
			v := flat.tree.Value[i]
			if v <= 0 {
				continue
			}
			x, y := pad+ci.X, top+ci.Y
			leaf := len(flat.tree.Children[i]) == 0
			col := flat.color(i)
			rows := []scene.TipRow{{Name: "share", Value: base.Approx(v/total*100) + "%"}}
			if !leaf {
				rows = append(rows, scene.TipRow{Name: "items", Value: strconv.Itoa(len(flat.tree.Children[i]))})
			}
			mk := scene.Mark{
				Shape:  scene.CircleShape{CX: x, CY: y, R: ci.R},
				Series: flat.series(i),
				Hit:    &scene.Hit{Key: strconv.Itoa(i), Label: flat.path[i], Value: base.Value(v), Rows: rows},
			}
			// Leaves: a wash with an edge in the branch hue, so labels stay
			// dark text. Groups: an outline only, the leaves inside them
			// carry the color.
			if leaf {
				mk.Fill = base.Wash(col)
				mk.Stroke = &scene.Stroke{Color: base.Series(col), Width: 1}
			} else {
				mk.Stroke = &scene.Stroke{Color: base.Border, Width: hairline}
			}
			marks = append(marks, mk)
			name := flat.nodes[i].Name
			if leaf && m.Width(name, tickSize, base.Semibold) <= 1.8*ci.R && tickSize <= ci.R {
				t := text(x, y, name, tickSize, base.Semibold, scene.Middle, scene.Center, base.TextPrimary)
				t.Series = flat.series(i)
				marks = append(marks, t)
			}
		}
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Circle packing of %s.", count(flat.leaves(), "item", "items"))),
		Marks:  marks,
	}, nil
}
