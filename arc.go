package charta

import (
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	// arcMaxStroke: arc widths run from 1px to this.
	arcMaxStroke = 4.0
	// kappa is 4/3·(√2 − 1): the control-point distance, as a fraction of
	// the radius, for a cubic tracing a quarter circle. A literal, so both
	// ports agree.
	kappa = 0.5522847498307936
)

// ArcChart puts nodes in a row with links arching over them: long-range
// connections rise high, local ones stay low.
type ArcChart struct {
	Title string `json:"title,omitempty"`
	Links []Link `json:"links"`
}

func (c ArcChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	g, err := graph(c.Links, false)
	if err != nil {
		return nil, err
	}
	n := len(g.names)
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, g.legend(), keySwatch)

	step := (width - 2*pad) / float64(max(n, 1))
	x := func(i int) float64 { return pad + float64((float64(i)+0.5)*step) }
	longest := maxWidth(m, g.names)
	// Labels turn 45° when they would run into each other, as on a category
	// axis.
	rotated := longest > step-4
	labelH := tickSize
	if rotated {
		labelH = (longest + tickSize) * sqrtHalf
	}
	baseY := math.Max(height-pad-labelH-gap-dotRadius, top+1)
	// Arcs are semicircles until the widest would not fit; then all of them
	// flatten by the same factor, so heights still compare.
	widest := 0.0
	for _, e := range g.edges {
		widest = math.Max(widest, math.Abs(x(e.S)-x(e.T))/2)
	}
	squash := 1.0
	if widest > 0 {
		squash = math.Min((baseY-top-arcMaxStroke/2)/widest, 1)
	}
	maxV := 0.0
	for _, e := range g.edges {
		maxV = math.Max(maxV, e.V)
	}

	for k, e := range g.edges {
		xa, xb := x(min(e.S, e.T)), x(max(e.S, e.T))
		cx, rx := (xa+xb)/2, (xb-xa)/2
		ry := rx * squash
		ops := []scene.PathOp{
			scene.Move(xa, baseY),
			scene.Cubic(xa, baseY-float64(kappa*ry), cx-float64(kappa*rx), baseY-ry, cx, baseY-ry),
			scene.Cubic(cx+float64(kappa*rx), baseY-ry, xb, baseY-float64(kappa*ry), xb, baseY),
		}
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: ops},
			Stroke: &scene.Stroke{Color: base.Link(g.color(e.S)), Width: 1 + (arcMaxStroke-1)*e.V/maxV},
			Series: g.series(e.S),
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("link-%d", k),
				Label: g.names[e.S] + " – " + g.names[e.T],
				Value: base.Value(e.V),
			},
		})
	}
	strength := g.strength()
	for i, name := range g.names {
		marks = append(marks, scene.Mark{
			Shape:  scene.CircleShape{CX: x(i), CY: baseY, R: dotRadius},
			Fill:   base.Series(g.color(i)),
			Stroke: &scene.Stroke{Color: base.Surface, Width: surfaceGap},
			Series: g.series(i),
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("node-%d", i),
				Label: name,
				Value: base.Value(strength[i]),
				Rows:  []scene.TipRow{{Name: "links", Value: strconv.Itoa(g.links(i))}},
				Area:  &scene.Rect{X: x(i) - minTarget/2, Y: baseY - minTarget/2, W: minTarget, H: minTarget},
			},
		})
		t := text(x(i), baseY+dotRadius+gap, name, tickSize, base.Regular, scene.Middle, scene.Top, base.TextSecondary)
		if rotated {
			t = rotate(t)
		}
		marks = append(marks, t)
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Arc diagram of %s and %s.",
			count(n, "node", "nodes"), count(len(g.edges), "link", "links"))),
		Marks: marks,
	}, nil
}
