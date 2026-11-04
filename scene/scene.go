// Package scene is the backend-agnostic picture a chart produces.
//
// A scene is a flat list of marks in paint order. Interactive marks carry a
// Hit: the HTML backend turns it into data-* attributes for the JS runtime,
// native backends call HitTest on pointer events and Emphasis to style each
// mark. The same scene drives both, so hover behaves alike everywhere. It
// mirrors the Rust crate's scene module, and Dump must produce byte-identical
// output.
package scene

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/linzeyan/charta/base"
)

// LineHitTolerance is how close the pointer must come to a line (px).
const LineHitTolerance = 4.0

type Scene struct {
	Width, Height float64
	// Label is the accessible name: what the chart shows, in one sentence.
	Label string
	Marks []Mark
}

type Mark struct {
	Shape Shape
	// Fill colors areas and text; base.NoColor for none.
	Fill   base.Color
	Stroke *Stroke
	// Series is the series the mark draws; "" for none. Hovering a series'
	// mark or legend entry dims the other series; the legend can hide it.
	Series string
	Hit    *Hit
}

type Stroke struct {
	Color base.Color
	Width float64
}

type Rect struct{ X, Y, W, H float64 }

// Plot is where a chart with continuous axes has its plot, and the data at
// the plot's edges: what a backend needs to turn pointer positions into data
// when zooming (package zoom's View). It is not part of the scene itself, so
// scenes, and the HTML made from them, are the same with or without it.
type Plot struct {
	Rect Rect
	// X is the x data at the rect's left and right edges.
	X [2]float64
	// Y is the y data at the rect's bottom and top edges, for charts that zoom
	// y as well (scatter, bubble); nil for the rest.
	Y *[2]float64
}

func (r Rect) Contains(x, y float64) bool {
	return x >= r.X && x <= r.X+r.W && y >= r.Y && y <= r.Y+r.H
}

// Corners are per-corner radii.
type Corners struct{ TL, TR, BR, BL float64 }

// AllCorners rounds every corner by r.
func AllCorners(r float64) Corners { return Corners{r, r, r, r} }

type Anchor int

const (
	Start Anchor = iota
	Middle
	End
)

// Baseline places text relative to its Y, measured on the cap height so it
// does not depend on each font's ascent metrics.
type Baseline int

const (
	// Top puts the top of capital letters at Y.
	Top Baseline = iota
	// Center puts the middle of capital letters at Y.
	Center
	// Bottom puts the alphabetic baseline at Y.
	Bottom
)

// Shape is one of RectShape, LineShape, PolylineShape, AreaShape,
// CircleShape, PathShape or TextShape.
type Shape interface{ isShape() }

type RectShape struct {
	Rect   Rect
	Radius Corners
}

type LineShape struct{ X1, Y1, X2, Y2 float64 }

type Point struct{ X, Y float64 }

// PolylineShape is an open polyline (series lines). Stroked, never filled.
type PolylineShape struct{ Points []Point }

// AreaPoint is one x of an AreaShape with the band's top and bottom y.
type AreaPoint struct{ X, Top, Bottom float64 }

// AreaShape is the band between two x-monotone edges, in x order. Filled,
// never stroked. A dedicated shape instead of a general path because some
// toolkits only fill convex polygons; this one splits into convex slices
// trivially.
type AreaShape struct{ Points []AreaPoint }

type CircleShape struct{ CX, CY, R float64 }

// PathShape is a general outline: violins, sectors, ribbons, map regions.
// Filled with the nonzero rule when the mark has a fill, stroked when it
// has a stroke. Arcs arrive as cubics, so backends only need four commands.
type PathShape struct{ Ops []PathOp }

// PathVerb is the command of a PathOp.
type PathVerb int

const (
	MoveTo PathVerb = iota
	LineTo
	// CubicTo has two control points, (X1, Y1) and (X2, Y2), then the end
	// point (X, Y).
	CubicTo
	ClosePath
)

// PathOp is one command of a PathShape, in absolute coordinates. MoveTo and
// LineTo use only X and Y; ClosePath uses nothing.
type PathOp struct {
	Verb           PathVerb
	X1, Y1, X2, Y2 float64
	X, Y           float64
}

func Move(x, y float64) PathOp { return PathOp{Verb: MoveTo, X: x, Y: y} }
func Line(x, y float64) PathOp { return PathOp{Verb: LineTo, X: x, Y: y} }
func Cubic(x1, y1, x2, y2, x, y float64) PathOp {
	return PathOp{Verb: CubicTo, X1: x1, Y1: y1, X2: x2, Y2: y2, X: x, Y: y}
}
func Close() PathOp { return PathOp{Verb: ClosePath} }

// CubicSteps is how many segments each cubic is split into when a path is
// flattened for hit testing or for toolkits that only take polygons. Fixed,
// not adaptive, so both ports flatten identically.
const CubicSteps = 16

// PathData is the path as SVG path data; also its form in scene dumps.
func PathData(ops []PathOp) string {
	var b strings.Builder
	for _, op := range ops {
		switch op.Verb {
		case MoveTo:
			fmt.Fprintf(&b, "M%s %s", base.Coord(op.X), base.Coord(op.Y))
		case LineTo:
			fmt.Fprintf(&b, "L%s %s", base.Coord(op.X), base.Coord(op.Y))
		case CubicTo:
			fmt.Fprintf(&b, "C%s %s %s %s %s %s", base.Coord(op.X1), base.Coord(op.Y1), base.Coord(op.X2), base.Coord(op.Y2), base.Coord(op.X), base.Coord(op.Y))
		case ClosePath:
			b.WriteByte('Z')
		}
	}
	return b.String()
}

// Flatten is the path as polylines, one per subpath, cubics split into
// CubicSteps segments. A closed subpath repeats its first point last.
func Flatten(ops []PathOp) [][]Point {
	var subs [][]Point
	for _, op := range ops {
		if op.Verb == MoveTo {
			subs = append(subs, []Point{{op.X, op.Y}})
			continue
		}
		if len(subs) == 0 {
			continue
		}
		s := &subs[len(subs)-1]
		switch op.Verb {
		case LineTo:
			*s = append(*s, Point{op.X, op.Y})
		case CubicTo:
			p0 := (*s)[len(*s)-1]
			for k := 1; k <= CubicSteps; k++ {
				t := float64(k) / CubicSteps
				u := 1 - t
				a, b, c, e := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
				*s = append(*s, Point{
					float64(a*p0.X) + float64(b*op.X1) + float64(c*op.X2) + float64(e*op.X),
					float64(a*p0.Y) + float64(b*op.Y1) + float64(c*op.Y2) + float64(e*op.Y),
				})
			}
		case ClosePath:
			*s = append(*s, (*s)[0])
		}
	}
	return subs
}

type TextShape struct {
	X, Y   float64
	Text   string
	Size   float64
	Weight base.Weight
	Anchor Anchor
	Base   Baseline
	// Angle is the clockwise rotation in degrees around (X, Y).
	Angle float64
}

func (RectShape) isShape()     {}
func (LineShape) isShape()     {}
func (PolylineShape) isShape() {}
func (AreaShape) isShape()     {}
func (CircleShape) isShape()   {}
func (PathShape) isShape()     {}
func (TextShape) isShape()     {}

// HitKind says how a backend treats an interactive mark.
type HitKind int

const (
	// HitMark is a data mark: tooltip with its value and label; the mark lifts.
	HitMark HitKind = iota
	// HitCrosshair is a vertical hairline over a whole x column, drawn only
	// while hovered; its tooltip lists every series at that x (Rows).
	HitCrosshair
	// HitLegend is a legend entry: hover highlights its series, click
	// toggles it.
	HitLegend
)

// Hit is what an interactive mark reports when hovered or focused.
type Hit struct {
	// Key is unique within the scene.
	Key  string
	Kind HitKind
	// Label is the category or name, secondary in the tooltip.
	Label string
	// Value is the formatted value, the tooltip's lead. Empty for
	// crosshairs and legend entries.
	Value string
	// Rows are extra tooltip lines under the label: the per-series readout
	// of a crosshair, statistics for summary marks.
	Rows []TipRow
	// Area is the hit target when it should exceed the painted shape.
	Area *Rect
}

// TipRow is one line of a multi-row tooltip: an optional short key in a
// series color, the value (strong) and its name. Crosshairs key every row by
// series; a statistic summary (box plot, OHLC) has no keys.
type TipRow struct {
	// Color is the key's color; base.NoColor for none.
	Color base.Color
	Name  string
	Value string
}

// Emphasis is how a native backend paints a mark in the current
// interaction state.
type Emphasis int

const (
	Normal Emphasis = iota
	// Lift is the hovered mark: lightened slightly.
	Lift
	// Dim is another series while one is highlighted, or a legend entry
	// whose series is hidden.
	Dim
	// Hidden is not painted: a hidden series, or a crosshair that is not
	// hovered.
	Hidden
)

// HitTest returns the index of the topmost interactive mark under (x, y),
// skipping marks of hidden series.
func (s *Scene) HitTest(x, y float64, hidden []string) (int, bool) {
	for i := len(s.Marks) - 1; i >= 0; i-- {
		m := s.Marks[i]
		if m.Hit == nil {
			continue
		}
		if m.Hit.Kind != HitLegend && isHidden(m, hidden) {
			continue
		}
		var inside bool
		if m.Hit.Area != nil {
			inside = m.Hit.Area.Contains(x, y)
		} else {
			w := 0.0
			if m.Stroke != nil {
				w = m.Stroke.Width
			}
			inside = contains(m.Shape, x, y, w, m.Fill != base.NoColor)
		}
		if inside {
			return i, true
		}
	}
	return 0, false
}

// Emphasis is the single rule every native backend follows, mirroring the
// HTML runtime: the hovered mark lifts; hovering a series' mark or legend
// entry dims the other series, but only when there are two or more;
// crosshairs show only while hovered; hidden series are not painted.
// hovered is the HitTest index, or negative when nothing is hovered.
func (s *Scene) Emphasis(i, hovered int, hidden []string) Emphasis {
	m := s.Marks[i]
	kind := HitKind(-1)
	if m.Hit != nil {
		kind = m.Hit.Kind
	}
	if kind == HitLegend {
		switch {
		case isHidden(m, hidden):
			return Dim
		case hovered == i:
			return Lift
		}
		return Normal
	}
	if isHidden(m, hidden) {
		return Hidden
	}
	if kind == HitCrosshair {
		if hovered == i {
			return Normal
		}
		return Hidden
	}
	if hovered < 0 {
		return Normal
	}
	if hovered == i {
		return Lift
	}
	// Only a legend entry can be hovered while hidden; a hidden series has
	// nothing on the plot to focus on.
	if isHidden(s.Marks[hovered], hidden) {
		return Normal
	}
	focus, own := s.Marks[hovered].Series, m.Series
	if focus != "" && own != "" && focus != own && s.seriesCount() > 1 {
		return Dim
	}
	return Normal
}

// seriesCount is the number of distinct series among the plot's marks
// (legend entries excluded).
func (s *Scene) seriesCount() int {
	seen := map[string]bool{}
	for _, m := range s.Marks {
		if m.Series != "" && (m.Hit == nil || m.Hit.Kind != HitLegend) {
			seen[m.Series] = true
		}
	}
	return len(seen)
}

func isHidden(m Mark, hidden []string) bool {
	return m.Series != "" && slices.Contains(hidden, m.Series)
}

func contains(s Shape, x, y, strokeWidth float64, filled bool) bool {
	tolerance := math.Max(strokeWidth/2, LineHitTolerance)
	switch s := s.(type) {
	case RectShape:
		return s.Rect.Contains(x, y)
	case LineShape:
		return segmentDistance(x, y, s.X1, s.Y1, s.X2, s.Y2) <= tolerance
	case PolylineShape:
		for k := 1; k < len(s.Points); k++ {
			a, b := s.Points[k-1], s.Points[k]
			if segmentDistance(x, y, a.X, a.Y, b.X, b.Y) <= tolerance {
				return true
			}
		}
	case AreaShape:
		for k := 1; k < len(s.Points); k++ {
			a, b := s.Points[k-1], s.Points[k]
			if x < a.X || x > b.X || b.X == a.X {
				continue
			}
			f := (x - a.X) / (b.X - a.X)
			top := a.Top + float64((b.Top-a.Top)*f)
			bottom := a.Bottom + float64((b.Bottom-a.Bottom)*f)
			if y >= math.Min(top, bottom) && y <= math.Max(top, bottom) {
				return true
			}
		}
	case CircleShape:
		dx, dy := x-s.CX, y-s.CY
		return math.Sqrt(float64(dx*dx)+float64(dy*dy)) <= s.R+strokeWidth/2
	case PathShape:
		subs := Flatten(s.Ops)
		if filled {
			return winding(subs, x, y) != 0
		}
		for _, sub := range subs {
			for k := 1; k < len(sub); k++ {
				a, b := sub[k-1], sub[k]
				if segmentDistance(x, y, a.X, a.Y, b.X, b.Y) <= tolerance {
					return true
				}
			}
		}
	}
	return false
}

// winding is the nonzero winding number of (x, y) against the subpaths,
// each closed implicitly.
func winding(subs [][]Point, x, y float64) int {
	w := 0
	for _, s := range subs {
		for k := range s {
			a, b := s[k], s[(k+1)%len(s)]
			cross := float64((b.X-a.X)*(y-a.Y)) - float64((x-a.X)*(b.Y-a.Y))
			if a.Y <= y {
				if b.Y > y && cross > 0 {
					w++
				}
			} else if b.Y <= y && cross < 0 {
				w--
			}
		}
	}
	return w
}

func segmentDistance(px, py, x1, y1, x2, y2 float64) float64 {
	dx, dy := x2-x1, y2-y1
	len2 := float64(dx*dx) + float64(dy*dy)
	t := 0.0
	if len2 != 0 {
		t = math.Max(0, math.Min(1, (float64((px-x1)*dx)+float64((py-y1)*dy))/len2))
	}
	cx, cy := x1+float64(t*dx), y1+float64(t*dy)
	ex, ey := px-cx, py-cy
	return math.Sqrt(float64(ex*ex) + float64(ey*ey))
}

// Dump is deterministic JSON, one mark per line, used to check that the Go
// and Rust ports draw the same picture. Coordinates are rounded to 0.01.
func (s *Scene) Dump() string {
	var b strings.Builder
	fmt.Fprintf(&b, `{"width":%s,"height":%s,"label":%s,"marks":[`, base.Coord(s.Width), base.Coord(s.Height), jsonStr(s.Label))
	for i, m := range s.Marks {
		if i == 0 {
			b.WriteString("\n")
		} else {
			b.WriteString(",\n")
		}
		dumpMark(&b, m)
	}
	b.WriteString("\n]}\n")
	return b.String()
}

func dumpMark(b *strings.Builder, m Mark) {
	switch s := m.Shape.(type) {
	case RectShape:
		fmt.Fprintf(b, `{"rect":[%s],"radius":[%s]`, nums(s.Rect.X, s.Rect.Y, s.Rect.W, s.Rect.H), nums(s.Radius.TL, s.Radius.TR, s.Radius.BR, s.Radius.BL))
	case LineShape:
		fmt.Fprintf(b, `{"line":[%s]`, nums(s.X1, s.Y1, s.X2, s.Y2))
	case PolylineShape:
		flat := make([]float64, 0, 2*len(s.Points))
		for _, p := range s.Points {
			flat = append(flat, p.X, p.Y)
		}
		fmt.Fprintf(b, `{"polyline":[%s]`, nums(flat...))
	case AreaShape:
		flat := make([]float64, 0, 3*len(s.Points))
		for _, p := range s.Points {
			flat = append(flat, p.X, p.Top, p.Bottom)
		}
		fmt.Fprintf(b, `{"area":[%s]`, nums(flat...))
	case CircleShape:
		fmt.Fprintf(b, `{"circle":[%s]`, nums(s.CX, s.CY, s.R))
	case PathShape:
		fmt.Fprintf(b, `{"path":%s`, jsonStr(PathData(s.Ops)))
	case TextShape:
		weight := "regular"
		if s.Weight == base.Semibold {
			weight = "semibold"
		}
		anchor := [...]string{"start", "middle", "end"}[s.Anchor]
		baseline := [...]string{"top", "middle", "bottom"}[s.Base]
		fmt.Fprintf(b, `{"text":%s,"at":[%s],"size":%s,"weight":"%s","anchor":"%s","baseline":"%s","angle":%s`,
			jsonStr(s.Text), nums(s.X, s.Y), base.Coord(s.Size), weight, anchor, baseline, base.Coord(s.Angle))
	}
	if m.Fill != base.NoColor {
		fmt.Fprintf(b, `,"fill":"%s"`, m.Fill.Token())
	}
	if m.Stroke != nil {
		fmt.Fprintf(b, `,"stroke":"%s","strokeWidth":%s`, m.Stroke.Color.Token(), base.Coord(m.Stroke.Width))
	}
	if m.Series != "" {
		fmt.Fprintf(b, `,"series":%s`, jsonStr(m.Series))
	}
	if h := m.Hit; h != nil {
		fmt.Fprintf(b, `,"hit":{"key":%s`, jsonStr(h.Key))
		switch h.Kind {
		case HitCrosshair:
			b.WriteString(`,"kind":"crosshair"`)
		case HitLegend:
			b.WriteString(`,"kind":"legend"`)
		}
		fmt.Fprintf(b, `,"label":%s,"value":%s`, jsonStr(h.Label), jsonStr(h.Value))
		if len(h.Rows) > 0 {
			b.WriteString(`,"rows":[`)
			for i, r := range h.Rows {
				if i > 0 {
					b.WriteByte(',')
				}
				b.WriteByte('{')
				if r.Color != base.NoColor {
					fmt.Fprintf(b, `"color":"%s",`, r.Color.Token())
				}
				fmt.Fprintf(b, `"name":%s,"value":%s}`, jsonStr(r.Name), jsonStr(r.Value))
			}
			b.WriteByte(']')
		}
		if a := h.Area; a != nil {
			fmt.Fprintf(b, `,"area":[%s]`, nums(a.X, a.Y, a.W, a.H))
		}
		b.WriteString("}")
	}
	b.WriteString("}")
}

func nums(v ...float64) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = base.Coord(x)
	}
	return strings.Join(parts, ",")
}

// JSONString quotes v with the escaping rule both ports share: quote,
// backslash and control characters escaped, everything else verbatim UTF-8.
// encoding/json would also escape <, > and &, which the Rust side does not.
func JSONString(v string) string { return jsonStr(v) }

func jsonStr(v string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range v {
		switch {
		case c == '"':
			b.WriteString(`\"`)
		case c == '\\':
			b.WriteString(`\\`)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c == '\t':
			b.WriteString(`\t`)
		case c < 0x20:
			fmt.Fprintf(&b, `\u%04x`, c)
		default:
			b.WriteRune(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}
