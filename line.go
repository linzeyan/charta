package charta

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/layout"
	"github.com/linzeyan/charta/scene"
)

// maxEndLabels: direct end labels ride the lines for this many series at
// most; beyond that the legend and tooltip carry identity.
const maxEndLabels = 4

// Curve says how a series runs from one point to the next. The zero value
// is CurveLinear, the Rust crate's default.
type Curve int

const (
	// CurveLinear draws straight segments: every bend is a data point.
	CurveLinear Curve = iota
	// CurveMonotone draws a smooth curve that never overshoots: between two
	// points it stays within their values, so it shows no peak or dip the
	// data lacks.
	CurveMonotone
)

var curveNames = [...]string{CurveLinear: "linear", CurveMonotone: "monotone"}

// MarshalJSON spells the curve the Rust serde way.
func (c Curve) MarshalJSON() ([]byte, error) {
	if c < 0 || int(c) >= len(curveNames) {
		return nil, fmt.Errorf("unknown curve %d", int(c))
	}
	return json.Marshal(curveNames[c])
}

func (c *Curve) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for k, n := range curveNames {
		if n == s {
			*c = Curve(k)
			return nil
		}
	}
	return fmt.Errorf("unknown curve %q", s)
}

// LineChart draws one line per series across ordered categories.
type LineChart struct {
	Title      string   `json:"title,omitempty"`
	Categories []string `json:"categories"`
	Series     []Series `json:"series"`
	Curve      Curve    `json:"curve,omitempty"`
}

// AreaChart draws lines with a soft wash down to the zero baseline.
type AreaChart struct {
	Title      string   `json:"title,omitempty"`
	Categories []string `json:"categories"`
	Series     []Series `json:"series"`
	Curve      Curve    `json:"curve,omitempty"`
}

// StackedAreaChart stacks series into bands; each band's top edge is drawn
// as a line. Values must be non-negative.
type StackedAreaChart struct {
	Title      string   `json:"title,omitempty"`
	Categories []string `json:"categories"`
	Series     []Series `json:"series"`
	Curve      Curve    `json:"curve,omitempty"`
}

func (c LineChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	return buildLine(kindLine, c.Curve, c.Title, c.Categories, c.Series, width, height, m)
}

func (c AreaChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	return buildLine(kindArea, c.Curve, c.Title, c.Categories, c.Series, width, height, m)
}

func (c StackedAreaChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	return buildLine(kindStacked, c.Curve, c.Title, c.Categories, c.Series, width, height, m)
}

func (c LineChart) CategoryCount() int { return len(c.Categories) }

func (c LineChart) Window(start, end int) Chart {
	c.Categories, c.Series = windowed(c.Categories, c.Series, start, end)
	return c
}

func (c AreaChart) CategoryCount() int { return len(c.Categories) }

func (c AreaChart) Window(start, end int) Chart {
	c.Categories, c.Series = windowed(c.Categories, c.Series, start, end)
	return c
}

func (c StackedAreaChart) CategoryCount() int { return len(c.Categories) }

func (c StackedAreaChart) Window(start, end int) Chart {
	c.Categories, c.Series = windowed(c.Categories, c.Series, start, end)
	return c
}

// windowed keeps categories start..end and the matching values, as copies:
// zoomed in, the y axis fits what is in view. Out-of-range bounds clip to
// what is there, as the Rust crate's skip/take does.
func windowed(categories []string, series []Series, start, end int) ([]string, []Series) {
	take := func(n int) (int, int) {
		lo := min(max(start, 0), n)
		return lo, max(lo, min(end, n))
	}
	lo, hi := take(len(categories))
	cats := slices.Clone(categories[lo:hi])
	out := make([]Series, len(series))
	for j, s := range series {
		lo, hi := take(len(s.Values))
		out[j] = Series{Name: s.Name, Values: slices.Clone(s.Values[lo:hi])}
	}
	return cats, out
}

type lineKind int

const (
	kindLine lineKind = iota
	kindArea
	kindStacked
)

// band is one series' (lower, upper) value at one category.
type band struct{ lower, upper float64 }

func buildLine(kind lineKind, curve Curve, title string, categories []string, series []Series, width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if err := checkSeries(categories, series, maxSeries); err != nil {
		return nil, err
	}
	if curve != CurveLinear && curve != CurveMonotone {
		return nil, invalid("unknown curve %d", int(curve))
	}
	if kind == kindStacked {
		for _, s := range series {
			for i, v := range s.Values {
				if v < 0 {
					return nil, invalid("stacked area series %q is negative at %q", s.Name, categories[i])
				}
			}
		}
	}
	n := len(categories)
	bands := make([][]band, len(series))
	sums := make([]float64, n)
	for j, s := range series {
		bands[j] = make([]band, len(s.Values))
		for i, v := range s.Values {
			if kind == kindStacked {
				sums[i] += v
				bands[j][i] = band{sums[i] - v, sums[i]}
			} else {
				bands[j][i] = band{0, v}
			}
		}
	}
	// Lines need not include zero; areas fill down to it.
	lo, hi := 0.0, 0.0
	if kind == kindLine {
		lo, hi = math.Inf(1), math.Inf(-1)
	}
	for _, b := range bands {
		for _, p := range b {
			lo, hi = math.Min(lo, p.upper), math.Max(hi, p.upper)
		}
	}
	if !isFinite(lo) {
		lo, hi = 0, 0
	}

	names := make([]string, len(series))
	for j, s := range series {
		names[j] = s.Name
	}
	var marks []scene.Mark
	top := drawTitle(&marks, title)
	key := keySwatch
	if kind == kindLine {
		key = keyLine
	}
	top = legend(&marks, m, top, width, names, key)

	// Label line ends directly when there are few series and the labels
	// would not collide; otherwise fall back to legend and tooltip rather
	// than nudging labels away from their lines.
	labelEnds := n > 0 && len(series) >= 2 && len(series) <= maxEndLabels
	reserve := 0.0
	if labelEnds {
		reserve = maxWidth(m, names) + gap
	}
	frameWith := func(reserve float64) categoryFrame {
		return newCategoryFrame(m, width, height, top, categories, lo, hi, 0, 0, reserve)
	}
	f := frameWith(reserve)
	if labelEnds {
		ends := make([]float64, len(bands))
		for j, b := range bands {
			ends[j] = f.y.scale.Map(b[n-1].upper)
		}
		sort.Float64s(ends)
		for k := 1; k < len(ends); k++ {
			if ends[k]-ends[k-1] < tickSize+surfaceGap {
				labelEnds = false
				f = frameWith(0)
				break
			}
		}
	}

	f.grid(&marks)
	f.yLabels(&marks)
	x := make([]float64, n)
	for i := range x {
		x[i] = f.center(i)
	}
	if kind != kindLine {
		for j, b := range bands {
			mk := scene.Mark{Fill: base.Wash(j), Series: names[j]}
			if curve == CurveLinear {
				pts := make([]scene.AreaPoint, len(b))
				for i, p := range b {
					pts[i] = scene.AreaPoint{X: x[i], Top: f.y.scale.Map(p.upper), Bottom: f.y.scale.Map(p.lower)}
				}
				mk.Shape = scene.AreaShape{Points: pts}
			} else {
				// Out along the top edge and back along the bottom, both curved
				// alike, so a stacked band meets its neighbor's.
				upper, lower := make([]scene.Point, len(b)), make([]scene.Point, len(b))
				for i, p := range b {
					upper[i] = scene.Point{X: x[i], Y: f.y.scale.Map(p.upper)}
					lower[len(b)-1-i] = scene.Point{X: x[i], Y: f.y.scale.Map(p.lower)}
				}
				ops := append(layout.Monotone(upper, false), layout.Monotone(lower, true)...)
				mk.Shape = scene.PathShape{Ops: append(ops, scene.Close())}
			}
			marks = append(marks, mk)
		}
	}
	for j, b := range bands {
		pts := make([]scene.Point, len(b))
		for i, p := range b {
			pts[i] = scene.Point{X: x[i], Y: f.y.scale.Map(p.upper)}
		}
		mk := scene.Mark{Series: names[j]}
		// A single point has no line to draw: show it as a dot.
		if len(pts) == 1 {
			mk.Shape = scene.CircleShape{CX: pts[0].X, CY: pts[0].Y, R: dotRadius}
			mk.Fill = base.Series(j)
		} else {
			mk.Shape = scene.PolylineShape{Points: pts}
			if curve == CurveMonotone {
				mk.Shape = scene.PathShape{Ops: layout.Monotone(pts, false)}
			}
			mk.Stroke = &scene.Stroke{Color: base.Series(j), Width: lineWidth}
		}
		marks = append(marks, mk)
	}
	f.baseline(&marks)

	// The crosshair finds the x: one column per category, listing every
	// series there, so the pointer never has to land on a 2px line.
	for i, cat := range categories {
		rows := make([]scene.TipRow, len(series))
		for j, s := range series {
			rows[j] = scene.TipRow{Color: base.Series(j), Name: s.Name, Value: base.Value(s.Values[i])}
		}
		col := f.column(i)
		marks = append(marks, scene.Mark{
			Shape:  scene.LineShape{X1: snap(x[i]), Y1: f.top, X2: snap(x[i]), Y2: f.bottom},
			Stroke: &scene.Stroke{Color: base.Axis, Width: hairline},
			Hit:    &scene.Hit{Key: fmt.Sprintf("x%d", i), Kind: scene.HitCrosshair, Label: cat, Rows: rows, Area: &col},
		})
	}
	if labelEnds {
		for j, b := range bands {
			mk := text(x[n-1]+gap, f.y.scale.Map(b[n-1].upper), names[j], tickSize, base.Regular, scene.Start, scene.Center, base.TextSecondary)
			mk.Series = names[j]
			marks = append(marks, mk)
		}
	}
	f.xLabels(&marks, categories)

	what := [...]string{"Line chart", "Area chart", "Stacked area chart"}[kind]
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(title, fmt.Sprintf("%s with %s and %s.", what, count(len(series), "series", "series"), count(n, "point", "points"))),
		Marks:  marks,
	}, nil
}
