// Command site writes the GitHub Pages gallery: every demo chart on one page,
// as the Go port renders it to HTML, with the charts that zoom wired to
// zoom.js. Next to the page it writes, per zooming chart, the spec the page
// hands zoom.js and the HTML it shows, so the Rust check (cargo run -p
// charta-wasm --example site_specs) can prove the wasm build lays the same
// spec out to the same HTML: else the first zoom would swap in another chart.
//
// Usage: go run ./internal/cmd/site [-commit sha] <site-dir> <specs-dir>
// (make site runs it, then the check, then copies the wasm build in).
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/internal/demo"
	"github.com/linzeyan/charta/zoom"
)

// families group the demos in the order demo.All lists them, as the README's
// chart table does.
var families = []struct {
	name string
	n    int
}{
	{"Cartesian", 13}, {"Statistics", 6}, {"Time & finance", 10}, {"Polar", 6},
	{"Hierarchy", 6}, {"Relationships & flow", 7}, {"Maps", 5}, {"Units, text & sets", 6},
}

// specTypes names the zooming charts' types as DecodeSpec reads them. A
// chart that zooms and is missing here fails the build rather than going
// out without zoom.
var specTypes = map[string]string{
	"LineChart": "line", "AreaChart": "area", "StackedAreaChart": "stackedArea",
	"ScatterChart": "scatter", "BubbleChart": "bubble",
	"CandlestickChart": "candlestick", "OHLCChart": "ohlc", "GanttChart": "gantt", "StreamChart": "stream",
}

func main() {
	commit := flag.String("commit", "", "the commit the page was built from, shown in its footer")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: site [-commit sha] <site-dir> <specs-dir>")
		os.Exit(2)
	}
	if err := run(flag.Arg(0), flag.Arg(1), *commit); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(siteDir, specsDir, commit string) error {
	demos := demo.All()
	total := 0
	for _, f := range families {
		total += f.n
	}
	if total != len(demos) {
		return fmt.Errorf("the families cover %d demos, demo.All has %d: update families", total, len(demos))
	}
	for _, dir := range []string{siteDir, specsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	var nav, body strings.Builder
	specs := map[string]json.RawMessage{}
	i := 0
	for _, f := range families {
		id := slug(f.name)
		fmt.Fprintf(&nav, `<a href="#%s">%s <span>%d</span></a>`, id, html.EscapeString(f.name), f.n)
		fmt.Fprintf(&body, `<section id="%s"><h2>%s</h2><div class="grid">`, id, html.EscapeString(f.name))
		for _, d := range demos[i : i+f.n] {
			page, err := charta.RenderHTML(d.Chart, charta.Options{})
			if err != nil {
				return fmt.Errorf("%s: %w", d.Name, err)
			}
			card := fmt.Sprintf("c%02d", i)
			badge := ""
			if zooms(d.Chart) {
				spec, err := specOf(d.Chart, page)
				if err != nil {
					return fmt.Errorf("%s: %w", d.Name, err)
				}
				specs[card] = spec
				stem := filepath.Join(specsDir, fmt.Sprintf("%02d-%s", i, slug(d.Name)))
				if err := os.WriteFile(stem+".json", spec, 0o644); err != nil {
					return err
				}
				if err := os.WriteFile(stem+".html", []byte(page), 0o644); err != nil {
					return err
				}
				badge = `<span class="badge" title="Wheel to zoom, drag to pan, Shift-drag to brush, double-click to reset">Zoom</span>`
			}
			fmt.Fprintf(&body, `<article class="card"><h3>%s%s</h3><div id="%s">%s</div></article>`,
				html.EscapeString(d.Name), badge, card, page)
			i++
		}
		body.WriteString(`</div></section>`)
	}

	// encoding/json escapes <, > and &, so the specs cannot close the script.
	specJSON, err := json.Marshal(specs)
	if err != nil {
		return err
	}
	built := "the Go port"
	if commit != "" {
		built = fmt.Sprintf(`the Go port at <a href="https://github.com/linzeyan/charta/commit/%[1]s"><code>%[1]s</code></a>`, html.EscapeString(commit))
	}
	out := strings.NewReplacer(
		"{{nav}}", nav.String(),
		"{{body}}", body.String(),
		"{{specs}}", string(specJSON),
		"{{built}}", built,
		"{{count}}", fmt.Sprint(len(demos)),
		"{{zooms}}", fmt.Sprint(len(specs)),
	).Replace(page)
	return os.WriteFile(filepath.Join(siteDir, "index.html"), []byte(out), 0o644)
}

// zooms is whether a backend turns gestures on for c: a category axis wider
// than the narrowest window, or a continuous one.
func zooms(c charta.Chart) bool {
	if _, ok := c.(charta.Viewable); ok {
		return true
	}
	z, ok := c.(charta.Zoomable)
	return ok && z.CategoryCount() > zoom.MinSpan
}

// specOf is c as the JSON DecodeSpec reads, checked to render the very page
// it came from.
func specOf(c charta.Chart, page string) (json.RawMessage, error) {
	name := reflect.TypeOf(c).Name()
	typ, ok := specTypes[name]
	if !ok {
		return nil, fmt.Errorf("%s zooms but has no spec type in specTypes", name)
	}
	fields, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(fields, []byte("{")) || len(fields) < 3 {
		return nil, fmt.Errorf("%s does not marshal to an object with fields", name)
	}
	spec := append([]byte(fmt.Sprintf(`{"type":%q,`, typ)), fields[1:]...)
	back, err := charta.DecodeSpec(spec)
	if err != nil {
		return nil, fmt.Errorf("its spec does not decode: %w", err)
	}
	again, err := charta.RenderHTML(back, charta.Options{})
	if err != nil {
		return nil, err
	}
	if again != page {
		return nil, fmt.Errorf("its spec renders another chart than the page shows")
	}
	return spec, nil
}

func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

const page = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>charta — every chart</title>
<meta name="description" content="{{count}} interactive chart types, rendered by charta: a Rust crate and a Go module that produce the same output.">
<!-- The charts are laid out with Inter's widths: in Inter they read as measured. -->
<link rel="preconnect" href="https://rsms.me/">
<link rel="stylesheet" href="https://rsms.me/inter/inter.css">
<script>
  // Before first paint, so a forced theme never flashes the other one.
  const saved = localStorage.getItem("theme");
  if (saved === "light" || saved === "dark") document.documentElement.dataset.theme = saved;
</script>
<style>
  :root { color-scheme: light; --bg: #f4f3ef; --card: #fcfcfb; --text: #0b0b0b; --muted: #52514e;
          --faint: #898781; --line: #e1e0d9; --accent: #2a78d6; --chip: #ebeae4; }
  @media (prefers-color-scheme: dark) {
    :root:not([data-theme="light"]) { color-scheme: dark; --bg: #111110; --card: #1a1a19; --text: #f0efec;
      --muted: #c3c2b7; --faint: #898781; --line: #2c2c2a; --accent: #3987e5; --chip: #242422; }
  }
  :root[data-theme="dark"] { color-scheme: dark; --bg: #111110; --card: #1a1a19; --text: #f0efec;
    --muted: #c3c2b7; --faint: #898781; --line: #2c2c2a; --accent: #3987e5; --chip: #242422; }
  * { box-sizing: border-box; }
  body { margin: 0; background: var(--bg); color: var(--text);
         font: 15px/1.55 Inter, system-ui, -apple-system, "Segoe UI", sans-serif;
         -webkit-font-smoothing: antialiased; font-feature-settings: "cv11"; }
  a { color: var(--accent); text-decoration: none; }
  a:hover { text-decoration: underline; }
  .wrap { max-width: 1440px; margin: 0 auto; padding-inline: 24px; }
  header { padding: 48px 0 28px; }
  .top { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 12px 16px; }
  h1 { margin: 0; font-size: 32px; font-weight: 700; letter-spacing: -0.02em; }
  .lede { margin: 10px 0 0; font-size: 18px; color: var(--muted); max-width: 760px; text-wrap: pretty; }
  .how { margin: 14px 0 0; color: var(--faint); max-width: 760px; text-wrap: pretty; }
  kbd { font: 12px/1 ui-monospace, SFMono-Regular, Menlo, monospace; padding: 2px 5px; border-radius: 4px;
        border: 1px solid var(--line); border-bottom-width: 2px; color: var(--muted); }
  .links { display: flex; gap: 8px; align-items: center; }
  .links > a { font-weight: 600; padding: 6px 12px; border-radius: 8px; background: var(--chip); color: var(--text); }
  .theme { display: inline-flex; padding: 2px; border-radius: 8px; background: var(--chip); }
  .theme button { font: inherit; font-size: 13px; border: 0; padding: 4px 10px; border-radius: 6px;
                  background: none; color: var(--muted); cursor: pointer; }
  .theme button[aria-pressed="true"] { background: var(--card); color: var(--text);
                                       box-shadow: 0 1px 2px rgb(0 0 0 / 0.12); }
  nav { position: sticky; top: 0; z-index: 10; border-bottom: 1px solid var(--line);
        background: color-mix(in srgb, var(--bg) 86%, transparent); backdrop-filter: blur(10px); }
  nav .wrap { display: flex; gap: 4px; overflow-x: auto; padding-top: 8px; padding-bottom: 8px; scrollbar-width: none; }
  nav a { flex: none; font-size: 14px; font-weight: 500; color: var(--muted); padding: 4px 10px; border-radius: 6px; }
  nav a:hover { background: var(--chip); color: var(--text); text-decoration: none; }
  nav a span { color: var(--faint); font-variant-numeric: tabular-nums; margin-left: 2px; }
  section { padding-top: 32px; scroll-margin-top: 48px; }
  h2 { margin: 0 0 14px; font-size: 20px; font-weight: 650; letter-spacing: -0.01em; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 440px), 1fr)); gap: 16px; }
  .card { background: var(--card); border: 1px solid var(--line); border-radius: 12px; padding: 14px 14px 10px;
          min-width: 0; }
  .card h3 { margin: 0 0 6px; font-size: 14px; font-weight: 600; display: flex; align-items: center;
             justify-content: space-between; gap: 8px; }
  .badge { font-size: 11px; font-weight: 600; letter-spacing: 0.02em; color: var(--accent); cursor: help;
           border: 1px solid color-mix(in srgb, var(--accent) 45%, transparent); border-radius: 999px; padding: 0 8px; }
  .card .charta { display: block; }
  .card .charta svg { width: 100%; }
  footer { margin: 48px 0 0; padding: 24px 0 48px; border-top: 1px solid var(--line); color: var(--faint); font-size: 14px; }
  footer p { margin: 4px 0; }
</style>
</head>
<body>
<header class="wrap">
  <div class="top">
    <h1>charta</h1>
    <div class="links">
      <div class="theme" role="group" aria-label="Theme">
        <button type="button" data-set="auto">Auto</button><button type="button" data-set="light">Light</button><button type="button" data-set="dark">Dark</button>
      </div>
      <a href="https://github.com/linzeyan/charta">GitHub</a>
    </div>
  </div>
  <p class="lede">{{count}} interactive chart types for the web and native GUIs — a Rust crate and a Go module that render every chart identically.</p>
  <p class="how">Each chart below is charta's HTML output, as is. Hover for tooltips; click a legend entry to hide its series. On the {{zooms}} charts marked <span class="badge">Zoom</span>, the wheel zooms, a drag pans, <kbd>Shift</kbd>-drag brushes a range and a double click shows everything again.</p>
</header>
<nav><div class="wrap">{{nav}}</div></nav>
<main class="wrap">{{body}}</main>
<footer class="wrap">
  <p>Rendered by {{built}}; the Rust crate renders the same bytes, which CI checks. Zooming runs the Rust layout engine as WebAssembly through <code>zoom.js</code>.</p>
  <p><a href="https://github.com/linzeyan/charta">github.com/linzeyan/charta</a> · Apache-2.0</p>
</footer>
<script>
  const buttons = document.querySelectorAll(".theme button");
  const mark = () => {
    const now = document.documentElement.dataset.theme || "auto";
    buttons.forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.set === now)));
  };
  buttons.forEach((b) => b.addEventListener("click", () => {
    if (b.dataset.set === "auto") {
      delete document.documentElement.dataset.theme;
      localStorage.removeItem("theme");
    } else {
      document.documentElement.dataset.theme = b.dataset.set;
      localStorage.setItem("theme", b.dataset.set);
    }
    mark();
  }));
  mark();
</script>
<script type="module">
  import { zoom } from "./zoom.js";
  const specs = {{specs}};
  for (const [id, spec] of Object.entries(specs)) {
    zoom(document.querySelector("#" + id + " .charta"), spec).catch((e) => console.error(id, e));
  }
</script>
</body>
</html>
`
