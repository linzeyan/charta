# charta

Interactive charts for every data-driven method in the
[Data Viz Catalogue](https://datavizcatalogue.com/) — 59 chart types, from bar
charts to Sankey diagrams, choropleth maps and word clouds.
**[See them all](https://linzeyan.github.io/charta/)**, live: hover, legend
toggles and zoom work on the page.

- **Native in both languages.** A pure Rust crate and a pure Go module, kept
  in lockstep by cross-language conformance tests: the same chart spec yields
  the same scene in both, bit for bit.
- **Web and native GUI.** Charts render to HTML or SVG with a small embedded
  runtime for tooltips, hover highlight and legend toggles — usable in
  browsers, Tauri and Wails. Native backends draw into egui and Slint (Rust)
  or Gio, Fyne and MyGo (Go), with the same interactions.
- **Zoom.** Category, time and numeric axes zoom by wheel, drag, shift-drag
  brush and double click, in every native backend and — through a small
  WebAssembly module — in the browser.
- **Pay for what you use.** GUI backends are optional features (Rust) or
  separate packages (Go); server-side rendering pulls in no GUI toolkit.

## Charts

| Family | Charts |
|---|---|
| Cartesian | bar, multi-set bar, stacked bar, line, area, stacked area, scatterplot, bubble, heatmap, span, population pyramid, bullet, error bars |
| Statistics | histogram, density, box & whisker, violin, stem & leaf, parallel coordinates |
| Time & finance | candlestick, OHLC, kagi, point & figure, Gantt, timeline, stream graph, calendar, timetable, spiral |
| Polar | pie, donut, Nightingale rose, radar, radial bar, radial column |
| Hierarchy | treemap, circle packing, sunburst, tree diagram, mind map, Marimekko |
| Relationships & flow | network, arc diagram, chord, non-ribbon chord, Sankey, parallel sets, flow chart |
| Maps | choropleth, bubble map, dot map, connection map, flow map |
| Units, text & sets | pictogram, proportional area, tally, dot matrix, word cloud, Venn |

Status: pre-alpha. Every chart is in both languages; the API may still change.

## Usage

Rust:

```rust
use charta::{BarChart, Chart, Options};

let chart = BarChart::new([("Apples", 30.0), ("Pears", 80.0)]).title("Fruit sold");
// Options::default() is 640×400, following the page's light or dark scheme.
let html = chart.render_html(&Options::default()).unwrap();
```

Go:

```go
chart := charta.BarChart{Title: "Fruit sold", Data: []charta.Bar{{Label: "Apples", Value: 30}, {Label: "Pears", Value: 80}}}
// The zero Options is 640×400, following the page's light or dark scheme.
page, err := charta.RenderHTML(chart, charta.Options{})
```

Both ports also read a chart from the same JSON — `charta::ChartSpec` (Rust,
with the `serde` feature) and `charta.DecodeSpec` (Go) — which is what the
conformance tests feed them.

### Backends

| Output | Rust | Go |
|---|---|---|
| HTML / SVG | built in | built in |
| PNG | feature `png` | package `png` |
| egui | feature `egui` | |
| Slint | feature `slint` (see License) | |
| Gio | | package `gio` |
| Fyne | | package `fyne` |
| MyGo | | package `mygo` |
| World basemap for maps | feature `geo-world` | package `geo/world` |

PNG, Slint, egui and MyGo chart text and Fyne's missing glyphs use the
system fonts: charta embeds none. Gio and Fyne need cgo, as those toolkits
do; the core, HTML, PNG and MyGo do not.

### Zoom in the browser

The HTML output is static apart from its runtime; zooming lays the chart out
again, so the browser needs the layout engine. `make wasm` builds it into
`build/wasm/` — `zoom.js`, `charta_wasm.js` and `charta_wasm_bg.wasm`
(about 300 KB gzipped). Serve the three together and, on a page with a chart
rendered by either port:

```html
<script type="module">
  import { zoom } from "./zoom.js";
  // The spec is the JSON the chart was rendered from. Resolves to false,
  // leaving the wheel to the page, for charts that do not zoom.
  await zoom(document.querySelector(".charta"), spec);
</script>
```

The wheel zooms around the pointer, dragging pans, shift-dragging brushes a
range (a box on scatter and bubble charts) and a double click shows
everything again. Series hidden through the legend stay hidden.

## Development

```sh
make setup      # pinned tools from mise.toml, and the wasm32 target
make check      # what CI runs: lint, tests, Rust/Go conformance, browser zoom, site
make site       # the GitHub Pages gallery, into build/site
make gallery    # Rust/Go scene conformance + side-by-side HTML gallery
make run-egui   # native galleries: run-egui, run-slint, run-gio, run-fyne, run-mygo
```

The native backends' tests render headless. On Linux without a GPU they run
on Mesa's software drivers; `sudo scripts/linux-deps.sh` installs those, the
C headers Gio and Fyne build against, and fonts (Ubuntu 24.04, as in CI).

## License

[Apache-2.0](LICENSE)

The optional `slint` feature links [Slint](https://slint.dev), which is
licensed GPL-3.0-only, royalty-free or commercially. charta itself stays
Apache-2.0, but an application built with that feature must comply with one
of Slint's licenses — distributing it under GPL-3.0, for example. The Slint
gallery in `examples/slint-gallery` uses GPL-3.0. Every other feature and
the Go module carry no such obligation.
