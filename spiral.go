package charta

import (
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/dmath"
	"github.com/linzeyan/charta/scene"
)

const (
	// spiralHole is the hole in the middle, as a share of the radius: the
	// first turn would otherwise crush into a point.
	spiralHole = 0.18
	// spiralThick is the share of each turn's band a segment fills; the rest
	// separates turns.
	spiralThick = 0.85
	// spiralEdgeSteps is the line segments per step along each curved edge.
	spiralEdgeSteps = 6
)

// spiralSpan is the share of each step a segment spans; the rest separates
// neighbours. A variable, not a constant: Go folds constant expressions
// exactly, and (1 - 0.94) / 2 must round at each step as in Rust.
var spiralSpan = 0.94

// SpiralChart places values in sequence around an Archimedean spiral, one
// turn per Period (a year of months, a week of days), so cycles line up
// radially. AngleLabels name the positions within a turn (e.g. month
// names), shown around the outside: empty, or one per position.
type SpiralChart struct {
	Title       string   `json:"title,omitempty"`
	Period      int      `json:"period"`
	AngleLabels []string `json:"angleLabels,omitempty"`
	Data        []Bar    `json:"data"`
}

func (c SpiralChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if c.Period < 1 {
		return nil, invalid("period must be at least 1")
	}
	if len(c.AngleLabels) > 0 && len(c.AngleLabels) != c.Period {
		return nil, invalid("%d angle labels for a period of %d", len(c.AngleLabels), c.Period)
	}
	for i, b := range c.Data {
		if err := finite(fmt.Sprintf("value %d (%q)", i, b.Label), b.Value); err != nil {
			return nil, err
		}
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, d := range c.Data {
		lo, hi = math.Min(lo, d.Value), math.Max(hi, d.Value)
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	if isFinite(lo) {
		top = seqLegend(&marks, m, top, lo, hi)
	}
	labelRoom := 0.0
	if len(c.AngleLabels) > 0 {
		labelRoom = tickSize + gap + maxWidth(m, c.AngleLabels)/2
	}
	cx, cy := width/2, (top+height-pad)/2
	radius := math.Max(math.Min(width-2*pad, height-pad-top)/2-labelRoom, 1)
	turns := float64(max((len(c.Data)+c.Period-1)/c.Period, 1))
	r0 := radius * spiralHole
	// The last turn's outer edge sits one segment thickness beyond its inner
	// edge, and must still end at radius.
	band := (radius - r0) / (turns + spiralThick)
	p := float64(c.Period)
	// Angle 0 is at twelve o'clock, increasing clockwise.
	at := func(u, r float64) (float64, float64) {
		s, c := dmath.Sincos(2 * math.Pi * u / p)
		return cx + float64(r*s), cy - float64(r*c)
	}
	inner := func(u float64) float64 { return r0 + band*u/p }
	for i, d := range c.Data {
		u0, u1 := float64(i)+(1-spiralSpan)/2, float64(i)+(1+spiralSpan)/2
		us := make([]float64, spiralEdgeSteps+1)
		for k := range us {
			us[k] = u0 + (u1-u0)*float64(k)/spiralEdgeSteps
		}
		ops := make([]scene.PathOp, 0, 2*len(us)+1)
		for k, u := range us {
			x, y := at(u, inner(u)+float64(band*spiralThick))
			if k == 0 {
				ops = append(ops, scene.Move(x, y))
			} else {
				ops = append(ops, scene.Line(x, y))
			}
		}
		for k := len(us) - 1; k >= 0; k-- {
			x, y := at(us[k], inner(us[k]))
			ops = append(ops, scene.Line(x, y))
		}
		ops = append(ops, scene.Close())
		marks = append(marks, scene.Mark{
			Shape: scene.PathShape{Ops: ops},
			Fill:  base.Seq(seqClass(d.Value, lo, hi)),
			Hit:   &scene.Hit{Key: strconv.Itoa(i), Label: d.Label, Value: base.Value(d.Value)},
		})
	}
	for k, l := range c.AngleLabels {
		x, y := at(float64(k)+0.5, radius+gap+tickSize/2)
		marks = append(marks, text(x, y, l, tickSize, base.Regular, scene.Middle, scene.Center, base.TextMuted))
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Spiral plot of %s, %d per turn.", count(len(c.Data), "value", "values"), c.Period)),
		Marks:  marks,
	}, nil
}
