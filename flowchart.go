package charta

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/layout"
	"github.com/linzeyan/charta/scene"
)

const (
	flowBoxH     = 28.0
	flowDiamondH = 40.0
	flowBoxPad   = 12.0
	flowMinBoxW  = 64.0
	// flowHGap is the horizontal room between neighbors in a row.
	flowHGap = 24.0
	// flowMinVGap / flowMaxVGap bound the vertical room between rows: at
	// least an arrowhead and a label; at most enough that a short chart does
	// not stretch into a ladder.
	flowMinVGap = 24.0
	flowMaxVGap = 40.0
	flowArrowL  = 7.0
	flowArrowW  = 8.0
	flowEdgeW   = 1.5
	// flowLoop is how far a loop back up runs to the right of the boxes it
	// joins, plus flowLoopStep per extra row it spans: longer loops run
	// outside shorter ones, so nested loops never cross.
	flowLoop     = 24.0
	flowLoopStep = 16.0
	flowCorner   = 8.0
)

// StepKind is the kind of a flow chart step, shown by its outline. The zero
// value is Process, the Rust crate's default; JSON spells kinds in
// lowercase.
type StepKind int

const (
	// Process is an action: a rounded box.
	Process StepKind = iota
	// Terminal is a start or end: a pill.
	Terminal
	// Decision is a question with several ways out: a diamond.
	Decision
)

var stepKindNames = [...]string{Process: "Process", Terminal: "Terminal", Decision: "Decision"}

func (k StepKind) String() string {
	if k < 0 || int(k) >= len(stepKindNames) {
		return "StepKind(" + strconv.Itoa(int(k)) + ")"
	}
	return stepKindNames[k]
}

func (k StepKind) MarshalJSON() ([]byte, error) {
	if k < 0 || int(k) >= len(stepKindNames) {
		return nil, fmt.Errorf("unknown step kind %d", int(k))
	}
	return json.Marshal(strings.ToLower(stepKindNames[k]))
}

func (k *StepKind) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for i, n := range stepKindNames {
		if strings.ToLower(n) == s {
			*k = StepKind(i)
			return nil
		}
	}
	return fmt.Errorf("unknown step kind %q", s)
}

// Step is one box of a flow chart.
type Step struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Kind  StepKind `json:"kind"`
}

// Arrow leads from one step to the next, optionally labeled ("yes", "no");
// an empty Label means none.
type Arrow struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

// FlowChart is steps joined by arrows, laid out top to bottom in rows so
// that arrows point down; an arrow back up (a loop) curves around the right
// side.
type FlowChart struct {
	Title  string  `json:"title,omitempty"`
	Steps  []Step  `json:"steps"`
	Arrows []Arrow `json:"arrows"`
}

func (c FlowChart) edges() ([][2]int, error) {
	for i, s := range c.Steps {
		if s.Kind < 0 || int(s.Kind) >= len(stepKindNames) {
			return nil, invalid("step %q has an unknown kind", s.ID)
		}
		for _, o := range c.Steps[:i] {
			if o.ID == s.ID {
				return nil, invalid("step id %q is used twice", s.ID)
			}
		}
	}
	find := func(id string) (int, error) {
		i := slices.IndexFunc(c.Steps, func(s Step) bool { return s.ID == id })
		if i < 0 {
			return 0, invalid("an arrow names step %q, which does not exist", id)
		}
		return i, nil
	}
	edges := make([][2]int, 0, len(c.Arrows))
	for _, a := range c.Arrows {
		s, err := find(a.From)
		if err != nil {
			return nil, err
		}
		t, err := find(a.To)
		if err != nil {
			return nil, err
		}
		if s == t {
			return nil, invalid("an arrow leads from %q to itself", a.From)
		}
		e := [2]int{s, t}
		if slices.Contains(edges, e) {
			return nil, invalid("the arrow %q → %q is given twice", a.From, a.To)
		}
		edges = append(edges, e)
	}
	return edges, nil
}

// px prints a length the way Rust's f64 Display does, for error messages.
func px(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func (c FlowChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	edges, err := c.edges()
	if err != nil {
		return nil, err
	}
	n := len(c.Steps)
	sc := &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Flow chart of %s.", count(n, "step", "steps"))),
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	if n == 0 {
		sc.Marks = marks
		return sc, nil
	}

	size := make([]scene.Point, n)
	widths := make([]float64, n)
	for i, s := range c.Steps {
		tw := m.Width(s.Label, legendSize, base.Regular)
		switch s.Kind {
		case Process:
			size[i] = scene.Point{X: math.Max(tw+2*flowBoxPad, flowMinBoxW), Y: flowBoxH}
		case Terminal:
			size[i] = scene.Point{X: math.Max(tw+2*flowBoxPad+flowBoxH/2, flowMinBoxW), Y: flowBoxH}
		case Decision:
			// Wide enough that the text's cap height, centered, clears the
			// slanted sides with flowBoxPad / 2 to spare.
			size[i] = scene.Point{X: math.Max((tw+flowBoxPad)*flowDiamondH/(flowDiamondH-legendSize), flowMinBoxW), Y: flowDiamondH}
		}
		widths[i] = size[i].X
	}
	l := layout.Layered(widths, edges, flowHGap)

	rowH := make([]float64, l.Layers)
	for i := range n {
		rowH[l.Layer[i]] = math.Max(rowH[l.Layer[i]], size[i].Y)
	}
	availH := height - pad - top
	rowsH := 0.0
	for _, h := range rowH {
		rowsH += h
	}
	vGap := 0.0
	if l.Layers > 1 {
		vGap = math.Min((availH-rowsH)/float64(l.Layers-1), flowMaxVGap)
	}
	if l.Layers > 1 && vGap < flowMinVGap {
		return nil, invalid("%s of steps need %spx of height; the chart has %spx",
			count(l.Layers, "row", "rows"), px(rowsH+float64(flowMinVGap*float64(l.Layers-1))), px(math.Max(availH, 0)))
	}
	span := func(k int) int {
		s, t := l.Layer[edges[k][0]], l.Layer[edges[k][1]]
		if s > t {
			return s - t
		}
		return t - s
	}
	loopOut := func(k int) float64 { return flowLoop + float64(flowLoopStep*float64(span(k)-1)) }
	widestLoop := 0.0
	for k := range edges {
		if l.Reversed[k] {
			widestLoop = math.Max(widestLoop, loopOut(k))
		}
	}
	needW := l.Width + widestLoop
	availW := width - 2*pad
	if needW > availW {
		return nil, invalid("the widest row of steps needs %spx; the chart has %spx", px(math.Ceil(needW)), px(math.Max(availW, 0)))
	}
	ox := pad + (availW-needW)/2
	rowY := make([]float64, 0, l.Layers)
	y := top
	for _, h := range rowH {
		rowY = append(rowY, y)
		y += h + vGap
	}
	cx := func(i int) float64 { return ox + l.X[i] }
	cy := func(i int) float64 { return rowY[l.Layer[i]] + rowH[l.Layer[i]]/2 }

	// Arrows first: boxes cover their ends.
	type arrowLabel struct {
		at   scene.Point
		text string
	}
	var labels []arrowLabel
	for k, e := range edges {
		s, t := e[0], e[1]
		var mid scene.Point
		if l.Reversed[k] {
			// s is below t: out of s's right side, up a vertical run clear
			// of both boxes, into t's right side pointing left.
			start := scene.Point{X: cx(s) + size[s].X/2, Y: cy(s)}
			tip := scene.Point{X: cx(t) + size[t].X/2, Y: cy(t)}
			end := scene.Point{X: tip.X + flowArrowL, Y: tip.Y}
			run := math.Max(start.X, end.X) + flowLoop + float64(flowLoopStep*float64(span(k)-1))
			r, kr := flowCorner, float64(kappa*flowCorner)
			ops := []scene.PathOp{
				scene.Move(start.X, start.Y),
				scene.Line(run-r, start.Y),
				scene.Cubic(run-r+kr, start.Y, run, start.Y-r+kr, run, start.Y-r),
				scene.Line(run, end.Y+r),
				scene.Cubic(run, end.Y+r-kr, run-r+kr, end.Y, run-r, end.Y),
				scene.Line(end.X, end.Y),
			}
			mid = scene.Point{X: run, Y: (start.Y + end.Y) / 2}
			marks = append(marks, scene.Mark{Shape: scene.PathShape{Ops: ops}, Stroke: &scene.Stroke{Color: base.TextMuted, Width: flowEdgeW}})
			marks = append(marks, scene.Mark{
				Shape: scene.PathShape{Ops: []scene.PathOp{
					scene.Move(tip.X, tip.Y),
					scene.Line(tip.X+flowArrowL, tip.Y-flowArrowW/2),
					scene.Line(tip.X+flowArrowL, tip.Y+flowArrowW/2),
					scene.Close(),
				}},
				Fill: base.TextMuted,
			})
		} else {
			pts := []scene.Point{{X: cx(s), Y: cy(s) + size[s].Y/2}}
			for b, bx := range l.Bends[k] {
				layer := l.Layer[s] + 1 + b
				pts = append(pts, scene.Point{X: ox + bx, Y: rowY[layer]}, scene.Point{X: ox + bx, Y: rowY[layer] + rowH[layer]})
			}
			tip := scene.Point{X: cx(t), Y: cy(t) - size[t].Y/2}
			pts = append(pts, scene.Point{X: tip.X, Y: tip.Y - flowArrowL})
			ops := []scene.PathOp{scene.Move(pts[0].X, pts[0].Y)}
			for w := 1; w < len(pts); w++ {
				p, q := pts[w-1], pts[w]
				if p.X == q.X {
					ops = append(ops, scene.Line(q.X, q.Y))
				} else {
					my := (p.Y + q.Y) / 2
					ops = append(ops, scene.Cubic(p.X, my, q.X, my, q.X, q.Y))
				}
			}
			// A label goes halfway along the first leg, where arrows leaving
			// the same step have already parted.
			mid = scene.Point{X: (pts[0].X + pts[1].X) / 2, Y: (pts[0].Y + pts[1].Y) / 2}
			marks = append(marks, scene.Mark{Shape: scene.PathShape{Ops: ops}, Stroke: &scene.Stroke{Color: base.TextMuted, Width: flowEdgeW}})
			marks = append(marks, scene.Mark{
				Shape: scene.PathShape{Ops: []scene.PathOp{
					scene.Move(tip.X, tip.Y),
					scene.Line(tip.X-flowArrowW/2, tip.Y-flowArrowL),
					scene.Line(tip.X+flowArrowW/2, tip.Y-flowArrowL),
					scene.Close(),
				}},
				Fill: base.TextMuted,
			})
		}
		if lbl := c.Arrows[k].Label; lbl != "" {
			labels = append(labels, arrowLabel{mid, lbl})
		}
	}

	for i, step := range c.Steps {
		w, h := size[i].X, size[i].Y
		x, y := cx(i), cy(i)
		r := scene.Rect{X: x - w/2, Y: y - h/2, W: w, H: h}
		var shape scene.Shape
		switch step.Kind {
		case Decision:
			shape = scene.PathShape{Ops: []scene.PathOp{
				scene.Move(x, r.Y),
				scene.Line(r.X+w, y),
				scene.Line(x, r.Y+h),
				scene.Line(r.X, y),
				scene.Close(),
			}}
		case Terminal:
			shape = scene.RectShape{Rect: r, Radius: scene.AllCorners(h / 2)}
		default:
			shape = scene.RectShape{Rect: r, Radius: scene.AllCorners(4)}
		}
		// An opaque base under the wash, so a loop passing behind a box does
		// not show through it.
		marks = append(marks, scene.Mark{Shape: shape, Fill: base.Surface})
		var rows []scene.TipRow
		for k, e := range edges {
			if e[0] != i {
				continue
			}
			name := c.Arrows[k].Label
			if name == "" {
				name = "next"
			}
			rows = append(rows, scene.TipRow{Name: name, Value: c.Steps[e[1]].Label})
		}
		marks = append(marks, scene.Mark{
			Shape:  shape,
			Fill:   base.Wash(0),
			Stroke: &scene.Stroke{Color: base.Series(0), Width: 1},
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("step-%d", i),
				Label: step.Label,
				Value: step.Kind.String(),
				Rows:  rows,
			},
		})
		marks = append(marks, text(x, y, step.Label, legendSize, base.Regular, scene.Middle, scene.Center, base.TextPrimary))
	}
	for _, lb := range labels {
		haloLabel(&marks, m, lb.at.X, lb.at.Y, lb.text, scene.Middle, base.TextSecondary)
	}
	sc.Marks = marks
	return sc, nil
}
