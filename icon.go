package charta

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/linzeyan/charta/scene"
)

// Pictogram icons: built-in shapes or SVG path data, placed into a box and
// cut at a vertical line for the fractional last icon of a row. Mirrors the
// Rust crate's chart/icon.rs.

// builtinBox: built-in icons are drawn in a 24×24 box, the common icon-set
// grid. Circles use arc.go's kappa.
const builtinBox = 24.0

// IconKind is a built-in icon, or IconPath for SVG path data. The zero value
// is IconPerson, the Rust crate's default.
type IconKind int

const (
	IconPerson IconKind = iota
	IconCircle
	IconSquare
	// IconPath is Icon.D, SVG path data (M L H V C Q Z, absolute or
	// relative) drawn in an Icon.Size × Icon.Size box, e.g. 24 for most
	// icon sets.
	IconPath
)

var iconNames = [...]string{IconPerson: "person", IconCircle: "circle", IconSquare: "square", IconPath: "path"}

// Icon is what each unit of a pictogram looks like.
type Icon struct {
	Kind IconKind
	// D and Size are IconPath's path data and box size.
	D    string
	Size float64
}

type iconPathJSON struct {
	D    *string  `json:"d"`
	Size *float64 `json:"size"`
}

// MarshalJSON spells the icon the Rust serde way: a lowercase name, or
// {"path": {"d": …, "size": …}}.
func (ic Icon) MarshalJSON() ([]byte, error) {
	switch {
	case ic.Kind == IconPath:
		return json.Marshal(map[string]any{"path": map[string]any{"d": ic.D, "size": ic.Size}})
	case ic.Kind >= 0 && int(ic.Kind) < len(iconNames):
		return json.Marshal(iconNames[ic.Kind])
	}
	return nil, fmt.Errorf("unknown icon kind %d", int(ic.Kind))
}

func (ic *Icon) UnmarshalJSON(data []byte) error {
	if t := bytes.TrimSpace(data); len(t) > 0 && t[0] == '"' {
		var s string
		if err := json.Unmarshal(t, &s); err != nil {
			return err
		}
		for k, n := range iconNames {
			if n == s && IconKind(k) != IconPath {
				*ic = Icon{Kind: IconKind(k)}
				return nil
			}
		}
		return fmt.Errorf("unknown icon %q", s)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	raw, ok := m["path"]
	if len(m) != 1 || !ok {
		return errors.New(`an icon is "person", "circle", "square" or {"path": {"d": …, "size": …}}`)
	}
	var p iconPathJSON
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	if p.D == nil || p.Size == nil {
		return errors.New("an icon path needs d and size")
	}
	*ic = Icon{Kind: IconPath, D: *p.D, Size: *p.Size}
	return nil
}

// place is the icon as path ops filling the square at (x, y) with side s.
func (ic Icon) place(x, y, s float64) ([]scene.PathOp, error) {
	var ops []scene.PathOp
	size := builtinBox
	switch ic.Kind {
	case IconPerson:
		ops = person()
	case IconCircle:
		ops = circleOps(12, 12, 10)
	case IconSquare:
		ops = []scene.PathOp{scene.Move(2, 2), scene.Line(22, 2), scene.Line(22, 22), scene.Line(2, 22), scene.Close()}
	case IconPath:
		if !(isFinite(ic.Size) && ic.Size > 0) {
			return nil, invalid("an icon path's size must be a positive number")
		}
		var err error
		if ops, err = parsePath(ic.D); err != nil {
			return nil, err
		}
		size = ic.Size
	default:
		return nil, invalid("unknown icon kind %d", int(ic.Kind))
	}
	k := s / size
	at := func(px, py float64) (float64, float64) { return x + float64(px*k), y + float64(py*k) }
	for i, op := range ops {
		switch op.Verb {
		case scene.MoveTo, scene.LineTo:
			op.X, op.Y = at(op.X, op.Y)
		case scene.CubicTo:
			op.X1, op.Y1 = at(op.X1, op.Y1)
			op.X2, op.Y2 = at(op.X2, op.Y2)
			op.X, op.Y = at(op.X, op.Y)
		}
		ops[i] = op
	}
	return ops, nil
}

// circleOps is a circle as four cubics, clockwise from the top.
func circleOps(cx, cy, r float64) []scene.PathOp {
	k := float64(kappa * r)
	return []scene.PathOp{
		scene.Move(cx, cy-r),
		scene.Cubic(cx+k, cy-r, cx+r, cy-k, cx+r, cy),
		scene.Cubic(cx+r, cy+k, cx+k, cy+r, cx, cy+r),
		scene.Cubic(cx-k, cy+r, cx-r, cy+k, cx-r, cy),
		scene.Cubic(cx-r, cy-k, cx-k, cy-r, cx, cy-r),
		scene.Close(),
	}
}

// person is a head and rounded shoulders, in the 24 box.
func person() []scene.PathOp {
	return append(circleOps(12, 6.5, 4.5),
		scene.Move(4, 22),
		scene.Line(4, 17.5),
		scene.Cubic(4, 14.5, 7.5, 12.5, 12, 12.5),
		scene.Cubic(16.5, 12.5, 20, 14.5, 20, 17.5),
		scene.Line(20, 22),
		scene.Close(),
	)
}

func isSpaceByte(c byte) bool {
	// Rust's u8::is_ascii_whitespace: no vertical tab.
	return c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r'
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

// parsePath reads the subset of SVG path data icon sets use once arcs are
// converted: moves, lines, cubics and quadratics, absolute or relative.
func parsePath(d string) ([]scene.PathOp, error) {
	bad := func(format string, a ...any) error { return invalid("icon path: "+format, a...) }
	b := d
	i := 0
	skip := func() {
		for i < len(b) && (isSpaceByte(b[i]) || b[i] == ',') {
			i++
		}
	}
	number := func() (float64, bool) {
		skip()
		start := i
		if i < len(b) && (b[i] == '+' || b[i] == '-') {
			i++
		}
		dot, digits := false, false
		for i < len(b) && (isDigit(b[i]) || (b[i] == '.' && !dot)) {
			dot = dot || b[i] == '.'
			digits = digits || isDigit(b[i])
			i++
		}
		if digits && i < len(b) && (b[i] == 'e' || b[i] == 'E') {
			mark := i
			i++
			if i < len(b) && (b[i] == '+' || b[i] == '-') {
				i++
			}
			exp := i
			for i < len(b) && isDigit(b[i]) {
				i++
			}
			if i == exp {
				i = mark
			}
		}
		if !digits {
			i = start
			return 0, false
		}
		// Out of range is ±Inf, as Rust parses it, not a syntax error.
		v, err := strconv.ParseFloat(b[start:i], 64)
		return v, err == nil || errors.Is(err, strconv.ErrRange)
	}

	var ops []scene.PathOp
	cx, cy, sx, sy := 0.0, 0.0, 0.0, 0.0
	var cmd byte
	for {
		skip()
		if i >= len(b) {
			break
		}
		if isAlpha(b[i]) {
			cmd = b[i]
			i++
		} else if cmd == 0 {
			return nil, bad("numbers without a command")
		}
		upper := cmd &^ 0x20
		if len(ops) == 0 && upper != 'M' {
			return nil, bad("must start with M")
		}
		rel := cmd >= 'a' && cmd <= 'z'
		ox, oy := 0.0, 0.0
		if rel {
			ox, oy = cx, cy
		}
		nums := func(n int) ([]float64, error) {
			v := make([]float64, n)
			for k := range v {
				var ok bool
				if v[k], ok = number(); !ok {
					return nil, bad("%c needs %d numbers", cmd, n)
				}
			}
			return v, nil
		}
		switch upper {
		case 'M':
			v, err := nums(2)
			if err != nil {
				return nil, err
			}
			cx, cy = ox+v[0], oy+v[1]
			sx, sy = cx, cy
			ops = append(ops, scene.Move(cx, cy))
			// Further pairs after a move are lines.
			if rel {
				cmd = 'l'
			} else {
				cmd = 'L'
			}
		case 'L':
			v, err := nums(2)
			if err != nil {
				return nil, err
			}
			cx, cy = ox+v[0], oy+v[1]
			ops = append(ops, scene.Line(cx, cy))
		case 'H':
			v, err := nums(1)
			if err != nil {
				return nil, err
			}
			cx = ox + v[0]
			ops = append(ops, scene.Line(cx, cy))
		case 'V':
			v, err := nums(1)
			if err != nil {
				return nil, err
			}
			cy = oy + v[0]
			ops = append(ops, scene.Line(cx, cy))
		case 'C':
			v, err := nums(6)
			if err != nil {
				return nil, err
			}
			ops = append(ops, scene.Cubic(ox+v[0], oy+v[1], ox+v[2], oy+v[3], ox+v[4], oy+v[5]))
			cx, cy = ox+v[4], oy+v[5]
		case 'Q':
			// A quadratic is the cubic with controls two thirds of the way
			// from each end to its one control point.
			v, err := nums(4)
			if err != nil {
				return nil, err
			}
			qx, qy, x, y := ox+v[0], oy+v[1], ox+v[2], oy+v[3]
			ops = quad(ops, cx, cy, qx, qy, x, y)
			cx, cy = x, y
		case 'Z':
			ops = append(ops, scene.Close())
			cx, cy = sx, sy
			cmd = 0
			skip()
			if i < len(b) && !isAlpha(b[i]) {
				return nil, bad("numbers after Z")
			}
		default:
			return nil, bad("command %q is not supported; use M, L, H, V, C, Q and Z (convert arcs to curves)", rune(upper))
		}
	}
	if len(ops) == 0 {
		return nil, bad("empty")
	}
	return ops, nil
}

// clipLeft is the part of the filled path left of x: every subpath
// flattened and clipped to the half-plane. Clipping against one straight
// edge keeps any polygon, concave or not, a single correct polygon, so
// nonzero filling of the result matches the original up to the cut.
func clipLeft(ops []scene.PathOp, x float64) []scene.PathOp {
	var out []scene.PathOp
	for _, pts := range scene.Flatten(ops) {
		if len(pts) > 1 && pts[0] == pts[len(pts)-1] {
			pts = pts[:len(pts)-1]
		}
		n := len(pts)
		var poly []scene.Point
		for k := range n {
			p, c := pts[(k+n-1)%n], pts[k]
			cross := func() scene.Point {
				t := (x - p.X) / (c.X - p.X)
				return scene.Point{X: x, Y: p.Y + float64(t*(c.Y-p.Y))}
			}
			switch pin, cin := p.X <= x, c.X <= x; {
			case pin && cin:
				poly = append(poly, c)
			case pin && !cin:
				poly = append(poly, cross())
			case !pin && cin:
				poly = append(poly, cross(), c)
			}
		}
		if len(poly) >= 3 {
			out = append(out, scene.Move(poly[0].X, poly[0].Y))
			for _, q := range poly[1:] {
				out = append(out, scene.Line(q.X, q.Y))
			}
			out = append(out, scene.Close())
		}
	}
	return out
}
