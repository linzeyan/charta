package charta

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// Unit charts: counts shown as repeated marks (icons, tally strokes, dots)
// or as shapes sized by area. Mirrors the Rust crate's chart/unit.rs.

const (
	// iconMax: icons are at most this big, and below iconMin they stop
	// reading as pictures.
	iconMax = 24.0
	iconMin = 8.0
	iconGap = 4.0
	// keyIcon is the icon drawn in the "= unit" key.
	keyIcon = 12.0
	// Tally strokes: spacing, height, and how far the fifth (diagonal)
	// stroke reaches past the first and fourth.
	strokeGap = 6.0
	tallyH    = 20.0
	overhang  = 3.0
	groupGap  = 8.0
	groupStep = 3*strokeGap + 2*overhang + groupGap
	// maxTally: past this many marks a tally stops being countable at a
	// glance.
	maxTally = 500.0
	// Dot matrix cells: at most 20px, at least 4px; the dot fills 80% of its
	// cell so neighbors stay apart.
	cellMax = 20.0
	cellMin = 4.0
	dotFill = 0.8
	maxDots = 2500
	// slotGap is the space between proportional shapes; lineGap between a
	// value and its name.
	slotGap = 16.0
	lineGap = 4.0
)

// PictogramChart draws rows of icons, each standing for Unit; the last one
// cut to the fraction left over.
type PictogramChart struct {
	Title string `json:"title,omitempty"`
	// Unit is what one icon is worth.
	Unit float64 `json:"unit"`
	Icon Icon    `json:"icon"`
	Data []Bar   `json:"data"`
}

// TallyChart shows counts as tally marks in groups of five.
type TallyChart struct {
	Title string `json:"title,omitempty"`
	Data  []Bar  `json:"data"`
}

// DotMatrixChart draws one dot per Unit, colored by category and filled in
// order.
type DotMatrixChart struct {
	Title string `json:"title,omitempty"`
	// Unit is what one dot is worth.
	Unit float64 `json:"unit"`
	Data []Bar   `json:"data"`
}

// AreaShape is the shape a proportional area chart sizes. The zero value is
// AreaCircle, the Rust crate's default; JSON spells shapes in lowercase.
type AreaShape int

const (
	AreaCircle AreaShape = iota
	AreaSquare
)

var areaShapeNames = [...]string{AreaCircle: "circle", AreaSquare: "square"}

func (s AreaShape) MarshalJSON() ([]byte, error) {
	if s < 0 || int(s) >= len(areaShapeNames) {
		return nil, fmt.Errorf("unknown area shape %d", int(s))
	}
	return json.Marshal(areaShapeNames[s])
}

func (s *AreaShape) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	for i, n := range areaShapeNames {
		if n == v {
			*s = AreaShape(i)
			return nil
		}
	}
	return fmt.Errorf("unknown area shape %q", v)
}

// ProportionalAreaChart puts shapes side by side, area proportional to
// value: for values too far apart for bars, at the cost of precise
// comparison.
type ProportionalAreaChart struct {
	Title string    `json:"title,omitempty"`
	Shape AreaShape `json:"shape"`
	Data  []Bar     `json:"data"`
}

// checkCounts: values must be finite and not negative: a count of marks
// cannot be.
func checkCounts(data []Bar) error {
	for _, b := range data {
		if err := finite(fmt.Sprintf("%q", b.Label), b.Value); err != nil {
			return err
		}
		if b.Value < 0 {
			return invalid("%q is negative; a count of marks cannot be", b.Label)
		}
	}
	return nil
}

func checkUnit(unit float64) error {
	if isFinite(unit) && unit > 0 {
		return nil
	}
	return invalid("the unit must be a positive number")
}

// unitRows are labelled rows, top-aligned: the label column, where marks
// may run to (leaving room for the value after them), and the most each row
// can take.
type unitRows struct{ left, right, room float64 }

func newUnitRows(m base.TextMeasure, width, height, top float64, data []Bar) unitRows {
	labelW, valueW := 0.0, 0.0
	for _, b := range data {
		labelW = math.Max(labelW, m.Width(b.Label, tickSize, base.Regular))
	}
	for _, b := range data {
		valueW = math.Max(valueW, m.Width(base.Value(b.Value), tickSize, base.Regular))
	}
	left := pad + labelW + gap
	right := math.Max(width-pad-valueW-gap, left+1)
	room := math.Max(height-pad-top, 1) / float64(max(len(data), 1))
	return unitRows{left, right, room}
}

// text draws a row's label, its value after the marks at end, and the
// hover target: the whole row, so a zero-mark row can be hovered too.
func (r unitRows) text(marks *[]scene.Mark, i int, b Bar, y, h, end float64) {
	cy := y + h/2
	*marks = append(*marks, text(r.left-gap, cy, b.Label, tickSize, base.Regular, scene.End, scene.Center, base.TextMuted))
	v := text(end, cy, base.Value(b.Value), tickSize, base.Regular, scene.Start, scene.Center, base.TextSecondary)
	v.Hit = &scene.Hit{
		Key:   strconv.Itoa(i),
		Label: b.Label,
		Value: base.Value(b.Value),
		Area:  &scene.Rect{X: pad, Y: y, W: math.Max(r.right-pad, 0) + gap, H: h},
	}
	*marks = append(*marks, v)
}

func (c PictogramChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if err := checkUnit(c.Unit); err != nil {
		return nil, err
	}
	if err := checkCounts(c.Data); err != nil {
		return nil, err
	}
	// Whole icons and the fraction of the last; a hair over a whole number
	// is float noise, not a sliver of an icon.
	type split struct {
		full int
		frac float64
	}
	splits := make([]split, len(c.Data))
	most := 0
	for i, b := range c.Data {
		q := b.Value / c.Unit
		full := math.Floor(q)
		frac := q - full
		// Far past any row that fits; capped so the int conversion stays
		// defined (Rust's saturates, Go's does not on every platform).
		full = math.Min(full, 1<<40)
		switch {
		case frac < 1e-9:
			splits[i] = split{int(full), 0}
		case frac > 1-1e-9:
			splits[i] = split{int(full) + 1, 0}
		default:
			splits[i] = split{int(full), frac}
		}
		n := splits[i].full
		if splits[i].frac > 0 {
			n++
		}
		most = max(most, n)
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	key, err := c.Icon.place(pad, top+(legendRow-keyIcon)/2, keyIcon)
	if err != nil {
		return nil, err
	}
	marks = append(marks,
		scene.Mark{Shape: scene.PathShape{Ops: key}, Fill: base.Series(0)},
		text(pad+keyIcon+lineGap, top+legendRow/2, "= "+base.Value(c.Unit), legendSize, base.Regular, scene.Start, scene.Center, base.TextSecondary))
	top += legendRow + gap

	r := newUnitRows(m, width, height, top, c.Data)
	fit := iconMax
	if most > 0 {
		fit = (r.right-r.left+iconGap)/float64(most) - iconGap
	}
	s := math.Min(math.Min(iconMax, fit), r.room-gap)
	if s < iconMin {
		return nil, invalid("%d icons in a row do not fit at %s×%s; raise the unit", most, px(width), px(height))
	}
	pitch := math.Min(r.room, s+2*gap)
	for i, b := range c.Data {
		full, frac := splits[i].full, splits[i].frac
		y := top + float64(float64(i)*pitch)
		iy := y + (pitch-s)/2
		n := full
		if frac > 0 {
			n++
		}
		for k := range n {
			x := r.left + float64(float64(k)*(s+iconGap))
			ops, err := c.Icon.place(x, iy, s)
			if err != nil {
				return nil, err
			}
			if k < full {
				marks = append(marks, scene.Mark{Shape: scene.PathShape{Ops: ops}, Fill: base.Series(0)})
				continue
			}
			// The whole icon as a faint track shows what the cut one is a
			// fraction of.
			part := clipLeft(ops, x+float64(frac*s))
			marks = append(marks,
				scene.Mark{Shape: scene.PathShape{Ops: ops}, Fill: base.Grid},
				scene.Mark{Shape: scene.PathShape{Ops: part}, Fill: base.Series(0)})
		}
		end := r.left
		if n > 0 {
			end = r.left + float64(float64(n)*(s+iconGap)) - iconGap + gap
		}
		r.text(&marks, i, b, y, pitch, end)
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Pictogram of %s, each icon %s.", count(len(c.Data), "row", "rows"), base.Value(c.Unit))),
		Marks:  marks,
	}, nil
}

func (c TallyChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if err := checkCounts(c.Data); err != nil {
		return nil, err
	}
	for _, b := range c.Data {
		if b.Value-math.Trunc(b.Value) != 0 || b.Value > maxTally {
			return nil, invalid("%q is %s: tallies count whole things, up to %s", b.Label, base.Value(b.Value), px(maxTally))
		}
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	r := newUnitRows(m, width, height, top, c.Data)
	h := math.Min(tallyH, r.room-gap)
	if h < iconMin {
		return nil, invalid("%s do not fit at %s×%s", count(len(c.Data), "row", "rows"), px(width), px(height))
	}
	pitch := math.Min(r.room, h+2*gap)
	for i, b := range c.Data {
		n := int(b.Value)
		groups, rest := n/5, n%5
		// Where the marks end: the last group's diagonal, or the last loose
		// stroke.
		first := r.left + overhang
		var end float64
		switch {
		case rest > 0:
			end = first + float64(float64(groups)*groupStep) + float64(float64(rest-1)*strokeGap)
		case groups > 0:
			end = first + float64(float64(groups-1)*groupStep) + 3*strokeGap + overhang
		default:
			end = r.left
		}
		if end > r.right {
			return nil, invalid("%q: %d tally marks do not fit at %s×%s; a bar chart reads better", b.Label, n, px(width), px(height))
		}
		y := top + float64(float64(i)*pitch)
		y0, y1 := y+(pitch-h)/2, y+(pitch+h)/2
		var ops []scene.PathOp
		for g := 0; g <= groups; g++ {
			gx := first + float64(float64(g)*groupStep)
			strokes := rest
			if g < groups {
				strokes = 4
			}
			for k := range strokes {
				x := gx + float64(float64(k)*strokeGap)
				ops = append(ops, scene.Move(x, y0), scene.Line(x, y1))
			}
			if g < groups {
				ops = append(ops, scene.Move(gx-overhang, y1-overhang), scene.Line(gx+3*strokeGap+overhang, y0+overhang))
			}
		}
		if len(ops) > 0 {
			marks = append(marks, scene.Mark{Shape: scene.PathShape{Ops: ops}, Stroke: &scene.Stroke{Color: base.Series(0), Width: lineWidth}})
		}
		if n > 0 {
			end += gap
		} else {
			end = r.left
		}
		r.text(&marks, i, b, y, pitch, end)
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Tally chart of %s.", count(len(c.Data), "row", "rows"))),
		Marks:  marks,
	}, nil
}

func (c DotMatrixChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if err := checkUnit(c.Unit); err != nil {
		return nil, err
	}
	if err := checkCounts(c.Data); err != nil {
		return nil, err
	}
	if len(c.Data) > maxSeries {
		return nil, invalid(`%d categories, at most %d: fold the rest into "Other"`, len(c.Data), maxSeries)
	}
	for i, b := range c.Data {
		for _, o := range c.Data[:i] {
			if o.Label == b.Label {
				return nil, invalid("category %q is listed twice", b.Label)
			}
		}
	}
	// Each category rounds on its own: a dot always means one unit of that
	// category, never a share borrowed from a neighbor.
	dots := make([]int, len(c.Data))
	total := 0
	for i, b := range c.Data {
		q := math.Round(b.Value / c.Unit)
		if q > maxDots {
			// Past the cap the count no longer matters, only that it is too
			// many; clamping keeps the sum from overflowing.
			q = maxDots + 1
		}
		dots[i] = int(q)
		total += dots[i]
	}
	if total > maxDots {
		return nil, invalid("%d dots, at most %d: raise the unit", total, maxDots)
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	names := make([]string, len(c.Data))
	for i, b := range c.Data {
		names[i] = b.Label
	}
	top = legend(&marks, m, top, width, names, keySwatch)
	marks = append(marks, text(pad, top+legendRow/2, "Each dot = "+base.Value(c.Unit), legendSize, base.Regular, scene.Start, scene.Center, base.TextMuted))
	top += legendRow + gap

	w, h := math.Max(width-2*pad, 1), math.Max(height-pad-top, 1)
	// The column count that makes the cells biggest.
	cols, best := 1, 0.0
	for cc := 1; cc <= max(total, 1); cc++ {
		rows := max((total+cc-1)/cc, 1)
		if cell := math.Min(w/float64(cc), h/float64(rows)); cell > best {
			cols, best = cc, cell
		}
	}
	cell := math.Min(best, cellMax)
	if cell < cellMin {
		return nil, invalid("%d dots do not fit at %s×%s; raise the unit", total, px(width), px(height))
	}
	k := 0
	for i, b := range c.Data {
		for range dots[i] {
			col, row := k%cols, k/cols
			mk := scene.Mark{
				Shape: scene.CircleShape{
					CX: pad + float64((float64(col)+0.5)*cell),
					CY: top + float64((float64(row)+0.5)*cell),
					R:  cell * dotFill / 2,
				},
				Fill: base.Series(i),
				Hit:  &scene.Hit{Key: fmt.Sprintf("dot-%d", k), Label: b.Label, Value: base.Value(b.Value)},
			}
			if len(names) > 1 {
				mk.Series = b.Label
			}
			marks = append(marks, mk)
			k++
		}
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Dot matrix of %s in %s, each dot %s.",
			count(total, "dot", "dots"), count(len(c.Data), "category", "categories"), base.Value(c.Unit))),
		Marks: marks,
	}, nil
}

func (c ProportionalAreaChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if err := checkCounts(c.Data); err != nil {
		return nil, err
	}
	if c.Shape < 0 || int(c.Shape) >= len(areaShapeNames) {
		return nil, invalid("unknown area shape %d", int(c.Shape))
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	roots := make([]float64, len(c.Data))
	labelW := make([]float64, len(c.Data))
	for i, b := range c.Data {
		roots[i] = math.Sqrt(b.Value)
		labelW[i] = math.Max(m.Width(b.Label, tickSize, base.Regular), m.Width(base.Value(b.Value), tickSize, base.Semibold))
	}
	labelsH := gap + 2*tickSize + lineGap
	w, h := math.Max(width-2*pad, 1), math.Max(height-pad-top-labelsH, 1)
	// A shape's side is k·√value; each slot is as wide as its shape or its
	// label. Find the largest k whose slots fit across, capped by the
	// biggest shape's height. Total width grows with k, so bisect.
	used := func(k float64) float64 {
		s := 0.0
		for i, q := range roots {
			s += math.Max(float64(k*q), labelW[i])
		}
		return s + float64(slotGap*float64(max(len(c.Data)-1, 0)))
	}
	if used(0) > w {
		return nil, invalid("%d labels do not fit side by side at %s", len(c.Data), px(width))
	}
	maxRoot := 0.0
	for _, q := range roots {
		maxRoot = math.Max(maxRoot, q)
	}
	k := 0.0
	if maxRoot > 0 {
		k = h / maxRoot
	}
	if used(k) > w {
		lo, hi := 0.0, k
		for range 60 {
			mid := (lo + hi) / 2
			if used(mid) <= w {
				lo = mid
			} else {
				hi = mid
			}
		}
		k = lo
	}
	baseY := top + float64(k*maxRoot)
	x := pad + (w-used(k))/2
	for i, b := range c.Data {
		side := float64(k * roots[i])
		slot := math.Max(side, labelW[i])
		cx := x + slot/2
		var ops []scene.PathOp
		if c.Shape == AreaSquare {
			ops = rectOps(scene.Rect{X: cx - side/2, Y: baseY - side, W: side, H: side})
		} else {
			ops = circleOps(cx, baseY-side/2, side/2)
		}
		if side > 0 {
			marks = append(marks, scene.Mark{
				Shape: scene.PathShape{Ops: ops},
				Fill:  base.Series(0),
				Hit:   &scene.Hit{Key: strconv.Itoa(i), Label: b.Label, Value: base.Value(b.Value)},
			})
		}
		marks = append(marks,
			text(cx, baseY+gap, base.Value(b.Value), tickSize, base.Semibold, scene.Middle, scene.Top, base.TextPrimary),
			text(cx, baseY+gap+tickSize+lineGap, b.Label, tickSize, base.Regular, scene.Middle, scene.Top, base.TextSecondary))
		x += slot + slotGap
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, fmt.Sprintf("Proportional area chart of %s.", count(len(c.Data), "value", "values"))),
		Marks:  marks,
	}, nil
}
