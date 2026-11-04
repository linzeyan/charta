// Package html renders a scene as HTML/SVG with a small embedded runtime for
// tooltips, hover highlight and keyboard focus. Colors are CSS variables, so
// the page can switch light/dark without re-rendering. It mirrors the Rust
// crate's backend::html.
package html

import (
	"fmt"
	"strings"

	"github.com/linzeyan/charta/assets"
	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const fontStack = `Inter,system-ui,-apple-system,"Segoe UI",sans-serif`

// Format is the output flavor.
type Format int

const (
	// FormatHTML is a <div> with the SVG, tooltip, data table and runtime;
	// embed it in a page.
	FormatHTML Format = iota
	// FormatSVG is a standalone SVG without script: native <title> tooltips
	// only. Use it for <img>, documents and files.
	FormatSVG
)

// Render renders a scene in the given theme and format.
func Render(s *scene.Scene, theme base.Theme, format Format) string {
	var b strings.Builder
	w, h := base.Coord(s.Width), base.Coord(s.Height)
	switch format {
	case FormatSVG:
		fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" class="charta" viewBox="0 0 %s %s" width="%s" height="%s" role="img" aria-label="%s">`+
			`<style>%ssvg.charta{background:var(--ch-surface);font-family:%s}</style>`,
			w, h, w, h, esc(s.Label), themeCSS(theme, true), fontStack)
		for _, m := range s.Marks {
			mark(&b, m, false)
		}
		b.WriteString("</svg>")
	default:
		fmt.Fprintf(&b, `<div class="charta"><style>%s%s</style>`+
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %s %s" width="%s" height="%s" role="group" aria-label="%s">`,
			themeCSS(theme, false), assets.RuntimeCSS, w, h, w, h, esc(s.Label))
		for _, m := range s.Marks {
			mark(&b, m, true)
		}
		b.WriteString(`</svg><div class="ch-tip" hidden><strong></strong><span></span><div class="ch-rows"></div></div>`)
		table(&b, s)
		fmt.Fprintf(&b, "<script>%s</script></div>", assets.RuntimeJS)
	}
	return b.String()
}

// themeCSS emits the palette as CSS variables. In a standalone SVG the chart
// is the document root, so the page-level data-theme override does not apply.
func themeCSS(theme base.Theme, standalone bool) string {
	vars := func(dark bool) string {
		var b strings.Builder
		if dark {
			b.WriteString("color-scheme:dark;")
		} else {
			b.WriteString("color-scheme:light;")
		}
		for _, e := range base.Palette() {
			c := e.Light
			if dark {
				c = e.Dark
			}
			fmt.Fprintf(&b, "--ch-%s:%s;", e.Token, c.Hex())
		}
		return b.String()
	}
	switch {
	case theme == base.ThemeLight:
		return ".charta{" + vars(false) + "}"
	case theme == base.ThemeDark:
		return ".charta{" + vars(true) + "}"
	case standalone:
		return ".charta{" + vars(false) + "}@media (prefers-color-scheme:dark){.charta{" + vars(true) + "}}"
	}
	// The viewer's theme toggle (data-theme on the page root) must win in
	// both directions; :where() keeps the media block below the toggle.
	dark := vars(true)
	return ".charta{" + vars(false) + `}@media (prefers-color-scheme:dark){:root:where(:not([data-theme="light"])) .charta{` + dark + "}}" +
		`:root[data-theme="dark"] .charta{` + dark + "}"
}

func mark(b *strings.Builder, m scene.Mark, interactive bool) {
	series := ""
	if m.Series != "" {
		series = ` data-series="` + esc(m.Series) + `"`
	}
	hit := m.Hit
	if hit == nil {
		shape(b, m, "", series)
		return
	}
	if !interactive {
		switch hit.Kind {
		case scene.HitCrosshair:
			// A static file has no hover to reveal a crosshair with.
		case scene.HitLegend:
			shape(b, m, "", "")
		default:
			fmt.Fprintf(b, "<g><title>%s: %s</title>", esc(hit.Label), esc(hit.Value))
			shape(b, m, "", "")
			b.WriteString("</g>")
		}
		return
	}
	class := "ch-m"
	switch hit.Kind {
	case scene.HitCrosshair:
		class = "ch-m ch-x"
	case scene.HitLegend:
		class = "ch-m ch-leg"
	}
	fmt.Fprintf(b, `<g class="%s" data-key="%s"%s`, class, esc(hit.Key), series)
	if hit.Kind == scene.HitLegend {
		fmt.Fprintf(b, ` tabindex="0" role="button" aria-pressed="true" aria-label="%s">`, esc(hit.Label))
	} else {
		// Marks and crosshairs share one rule: the value leads when there is
		// one, rows follow when there are any, and a row names its key color
		// only when it has one.
		rows := make([]string, len(hit.Rows))
		var spoken []string
		if hit.Value != "" {
			spoken = append(spoken, hit.Value)
		}
		for i, r := range hit.Rows {
			c := ""
			if r.Color != base.NoColor {
				c = `"c":"` + r.Color.Token() + `",`
			}
			rows[i] = fmt.Sprintf(`{%s"n":%s,"v":%s}`, c, scene.JSONString(r.Name), scene.JSONString(r.Value))
			spoken = append(spoken, r.Name+" "+r.Value)
		}
		fmt.Fprintf(b, ` data-tip-label="%s"`, esc(hit.Label))
		if hit.Value != "" {
			fmt.Fprintf(b, ` data-tip-value="%s"`, esc(hit.Value))
		}
		if len(rows) > 0 {
			fmt.Fprintf(b, ` data-rows="%s"`, esc("["+strings.Join(rows, ",")+"]"))
		}
		fmt.Fprintf(b, ` tabindex="0" role="img" aria-label="%s: %s">`, esc(hit.Label), esc(strings.Join(spoken, ", ")))
	}
	if hit.Area != nil {
		rectEl(b, *hit.Area, scene.Corners{}, `class="ch-hit"`)
	}
	shape(b, m, "ch-s", "")
	b.WriteString("</g>")
}

func shape(b *strings.Builder, m scene.Mark, class, extra string) {
	attrs := ""
	if class != "" {
		attrs = `class="` + class + `" `
	}
	attrs += `style="` + style(m) + `"` + extra
	switch s := m.Shape.(type) {
	case scene.RectShape:
		rectEl(b, s.Rect, s.Radius, attrs)
	case scene.LineShape:
		fmt.Fprintf(b, `<line x1="%s" y1="%s" x2="%s" y2="%s" %s/>`, base.Coord(s.X1), base.Coord(s.Y1), base.Coord(s.X2), base.Coord(s.Y2), attrs)
	case scene.PolylineShape:
		pts := make([]string, len(s.Points))
		for i, p := range s.Points {
			pts[i] = base.Coord(p.X) + "," + base.Coord(p.Y)
		}
		fmt.Fprintf(b, `<polyline points="%s" %s/>`, strings.Join(pts, " "), attrs)
	case scene.AreaShape:
		var d strings.Builder
		for i, p := range s.Points {
			cmd := "L"
			if i == 0 {
				cmd = "M"
			}
			fmt.Fprintf(&d, "%s%s %s", cmd, base.Coord(p.X), base.Coord(p.Top))
		}
		for i := len(s.Points) - 1; i >= 0; i-- {
			p := s.Points[i]
			fmt.Fprintf(&d, "L%s %s", base.Coord(p.X), base.Coord(p.Bottom))
		}
		d.WriteString("Z")
		fmt.Fprintf(b, `<path d="%s" %s/>`, d.String(), attrs)
	case scene.PathShape:
		fmt.Fprintf(b, `<path d="%s" %s/>`, scene.PathData(s.Ops), attrs)
	case scene.CircleShape:
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s" %s/>`, base.Coord(s.CX), base.Coord(s.CY), base.Coord(s.R), attrs)
	case scene.TextShape:
		shift := 0.0
		switch s.Base {
		case scene.Top:
			shift = base.CapHeight * s.Size
		case scene.Center:
			shift = base.CapHeight * s.Size / 2
		}
		anchor := [...]string{"start", "middle", "end"}[s.Anchor]
		fmt.Fprintf(b, `<text x="%s" y="%s" text-anchor="%s" %s`, base.Coord(s.X), base.Coord(s.Y+float64(shift)), anchor, attrs)
		if s.Angle != 0 {
			fmt.Fprintf(b, ` transform="rotate(%s %s %s)"`, base.Coord(s.Angle), base.Coord(s.X), base.Coord(s.Y))
		}
		fmt.Fprintf(b, ">%s</text>", esc(s.Text))
	}
}

func style(m scene.Mark) string {
	s := "fill:none"
	if m.Fill != base.NoColor {
		s = "fill:" + cssVar(m.Fill)
	}
	if m.Stroke != nil {
		s += fmt.Sprintf(";stroke:%s;stroke-width:%s", cssVar(m.Stroke.Color), base.Coord(m.Stroke.Width))
	}
	switch m.Shape.(type) {
	case scene.PolylineShape, scene.PathShape:
		if m.Stroke != nil {
			s += ";stroke-linejoin:round;stroke-linecap:round"
		}
	}
	if t, ok := m.Shape.(scene.TextShape); ok {
		w := 400
		if t.Weight == base.Semibold {
			w = 600
		}
		s += fmt.Sprintf(";font-size:%spx;font-weight:%d", base.Coord(t.Size), w)
	}
	return s
}

func cssVar(c base.Color) string { return "var(--ch-" + c.Token() + ")" }

// rectEl writes a rect, or a path when only some corners are rounded (SVG's
// rx rounds all four).
func rectEl(b *strings.Builder, r scene.Rect, c scene.Corners, attrs string) {
	if c == (scene.Corners{}) {
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" %s/>`, base.Coord(r.X), base.Coord(r.Y), base.Coord(r.W), base.Coord(r.H), attrs)
		return
	}
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.W, r.Y+r.H
	var d strings.Builder
	fmt.Fprintf(&d, "M%s %sH%s", base.Coord(x0+c.TL), base.Coord(y0), base.Coord(x1-c.TR))
	corner(&d, c.TR, x1, y0+c.TR)
	fmt.Fprintf(&d, "V%s", base.Coord(y1-c.BR))
	corner(&d, c.BR, x1-c.BR, y1)
	fmt.Fprintf(&d, "H%s", base.Coord(x0+c.BL))
	corner(&d, c.BL, x0, y1-c.BL)
	fmt.Fprintf(&d, "V%s", base.Coord(y0+c.TL))
	corner(&d, c.TL, x0+c.TL, y0)
	d.WriteString("Z")
	fmt.Fprintf(b, `<path d="%s" %s/>`, d.String(), attrs)
}

func corner(d *strings.Builder, r, x, y float64) {
	if r > 0 {
		fmt.Fprintf(d, "A%s %s 0 0 1 %s %s", base.Coord(r), base.Coord(r), base.Coord(x), base.Coord(y))
	}
}

// table writes a screen-reader table of every interactive mark: the values
// a tooltip shows must also be reachable without hovering. A div hides it: a
// table grows to fit its cells whatever its width, and its clipped box still
// widened a narrow page into a horizontal scroll.
func table(b *strings.Builder, s *scene.Scene) {
	fmt.Fprintf(b, `<div class="ch-sr"><table><caption>%s</caption><thead><tr><th scope="col">Label</th><th scope="col">Value</th></tr></thead><tbody>`, esc(s.Label))
	row := func(th, td string) {
		fmt.Fprintf(b, `<tr><th scope="row">%s</th><td>%s</td></tr>`, esc(th), esc(td))
	}
	for _, m := range s.Marks {
		h := m.Hit
		if h == nil {
			continue
		}
		if h.Kind == scene.HitLegend {
			continue
		}
		// A summary mark with rows but no value of its own would otherwise
		// add an empty row before its statistics.
		if h.Value != "" || len(h.Rows) == 0 {
			row(h.Label, h.Value)
		}
		for _, r := range h.Rows {
			row(h.Label+" · "+r.Name, r.Value)
		}
	}
	b.WriteString("</tbody></table></div>")
}

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")

func esc(s string) string { return escaper.Replace(s) }
