#!/bin/bash
# Zooms charts in headless Chrome through wasm/zoom.js and the charta-wasm
# build, on HTML the Go port rendered (the ports render the same markup):
#   - the wheel zooms a line chart's months, keeping a series hidden through
#     the legend hidden, and the page's runtime works on the new SVG;
#   - a drag pans, and the click that ends it on a legend entry is ignored;
#   - a double click shows everything again;
#   - a shift-drag on a scatterplot shades a box, then zooms to it; one the
#     browser cancels takes its shading and leaves the zoom;
#   - a bar chart, which does not zoom, leaves the wheel to the page;
#   - a chart in a box narrower than itself does not widen it (its hidden
#     screen-reader table once did, scrolling narrow pages sideways).
#
# Usage: make test-web   (builds build/wasm first; from the repo root)
# Needs: Go, python3 (a local server: wasm does not load from file://), and
#   Chrome: CHROME=..., else the macOS app or google-chrome/chromium on PATH.
# Output: the probe's JSON; build/web-zoom/ keeps the page's inputs and
#   Chrome's log. Exit 0 when every expectation holds, 1 if not, 2 on setup.
# Plain bash 3.2: macOS /bin/bash and the Linux runners both run it.

root=$(cd "$(dirname "$0")/.." && pwd)
out="$root/build/wasm"
work="$root/build/web-zoom"
[ -f "$out/charta_wasm_bg.wasm" ] || { echo "no wasm build; run make wasm" >&2; exit 2; }

chrome=${CHROME:-}
if [ -z "$chrome" ]; then
  for c in "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
    google-chrome google-chrome-stable chromium chromium-browser; do
    if [ -x "$c" ] || command -v "$c" >/dev/null 2>&1; then chrome=$c; break; fi
  done
fi
[ -n "$chrome" ] || { echo "no Chrome found; set CHROME" >&2; exit 2; }

rm -rf "$work"
mkdir -p "$work/fixtures"
cat >"$work/fixtures/line.json" <<'JSON'
{"width": 640, "height": 400, "chart": {"type": "line",
  "categories": ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug"],
  "series": [{"name": "Visits", "values": [10, 20, 15, 30, 25, 40, 35, 50]},
             {"name": "Signups", "values": [4, 8, 6, 12, 10, 16, 14, 20]}]}}
JSON
cat >"$work/fixtures/scatter.json" <<'JSON'
{"width": 640, "height": 400, "chart": {"type": "scatter", "series": [{"name": "Trials", "points": [
  {"x": 1, "y": 3}, {"x": 2, "y": 8}, {"x": 3, "y": 4}, {"x": 4, "y": 5}, {"x": 5, "y": 1},
  {"x": 6, "y": 9}, {"x": 7, "y": 6}, {"x": 8, "y": 2}, {"x": 9, "y": 7}, {"x": 10, "y": 5}]}]}}
JSON
cat >"$work/fixtures/bar.json" <<'JSON'
{"width": 640, "height": 400, "chart": {"type": "bar",
  "data": [{"label": "A", "value": 3}, {"label": "B", "value": 5}]}}
JSON
(cd "$root" && go run ./internal/cmd/fixtures "$work/fixtures" "$work/html") >"$work/fixtures.log" 2>&1 ||
  { cat "$work/fixtures.log" >&2; exit 2; }

page="$out/zoom-test.html"
{
  echo '<!doctype html><html><head><meta charset="utf-8"></head><body style="margin:0">'
  for c in line scatter bar; do
    echo "<div id=\"$c\" style=\"margin:8px\">"
    cat "$work/html/$c.html"
    echo '</div>'
  done
  echo "<div id=\"narrow\" style=\"width:100px;overflow:auto\">$(cat "$work/html/bar.html")</div>"
  echo '<script type="module">'
  echo 'import { zoom } from "./zoom.js";'
  echo 'const specs = {'
  for c in line scatter bar; do echo "$c: $(cat "$work/fixtures/$c.json"),"; done
  echo '};'
  cat <<'JS'
// The probe is written as the steps go, so a page that stalls still shows
// how far it got.
const out = { step: "start" };
const pre = document.createElement("pre");
pre.id = "probe";
document.body.append(pre);
const step = (name) => {
  out.step = name;
  pre.textContent = JSON.stringify(out);
};
step("start");
const settle = () => new Promise((r) => setTimeout(r, 0));
// Scene point (x, y) of a chart as client coordinates.
const client = (root, x, y) => {
  const r = root.querySelector("svg").getBoundingClientRect();
  return { clientX: r.left + (x * r.width) / 640, clientY: r.top + (y * r.height) / 400 };
};
const send = (root, type, x, y, extra = {}) =>
  root.querySelector("svg").dispatchEvent(
    new PointerEvent(type, { bubbles: true, cancelable: true, pointerId: 1, button: 0, ...client(root, x, y), ...extra }),
  );
const drag = async (root, [x0, y0], [x1, y1], shiftKey, mid) => {
  send(root, "pointerdown", x0, y0, { shiftKey });
  for (let k = 1; k <= 10; k++) {
    send(root, "pointermove", x0 + ((x1 - x0) * k) / 10, y0 + ((y1 - y0) * k) / 10, { shiftKey });
    if (k === 5 && mid) mid();
  }
  send(root, "pointerup", x1, y1);
  await settle();
};
const wheel = (root, deltaY) => {
  const e = new WheelEvent("wheel", { bubbles: true, cancelable: true, deltaY, ...client(root, 320, 200) });
  root.querySelector("svg").dispatchEvent(e);
  return e.defaultPrevented;
};
const columns = (root) => root.querySelectorAll(".ch-x").length;
const firstColumn = (root) => root.querySelector(".ch-x").dataset.tipLabel;
const legend = (root, s) => root.querySelector(`.ch-leg[data-series="${s}"]`);
const shade = (root) => root.querySelector('div[style*="ch-series-1-link"]');

try {
  const [line, scatter, bar] = ["line", "scatter", "bar"].map((id) => document.querySelector(`#${id} .charta`));
  step("zoom");
  out.zooms = [await zoom(line, specs.line.chart), await zoom(scatter, specs.scatter.chart), await zoom(bar, specs.bar.chart)];
  out.columns = columns(line);

  step("wheel");
  legend(line, "Signups").dispatchEvent(new MouseEvent("click", { bubbles: true }));
  const prevented = wheel(line, -200);
  await settle();
  const signups = Array.from(line.querySelectorAll('[data-series="Signups"]:not(.ch-leg)'));
  out.wheel = {
    prevented,
    columns: columns(line),
    signupsOff: legend(line, "Signups").classList.contains("ch-off"),
    signupsHidden: signups.length > 0 && signups.every((m) => getComputedStyle(m).display === "none"),
  };
  const col = line.querySelector(".ch-x").getBoundingClientRect();
  line.querySelector(".ch-x").dispatchEvent(
    new PointerEvent("pointermove", { bubbles: true, clientX: col.left + col.width / 2, clientY: col.top + col.height / 2 }),
  );
  out.hoverTip = !line.querySelector(".ch-tip").hidden;

  step("pan");
  const from = firstColumn(line);
  await drag(line, [420, 200], [220, 200], false);
  legend(line, "Visits").dispatchEvent(new MouseEvent("click", { bubbles: true }));
  out.pan = { from, to: firstColumn(line), columns: columns(line), visitsOn: !legend(line, "Visits").classList.contains("ch-off") };

  step("reset");
  line.querySelector("svg").dispatchEvent(new MouseEvent("dblclick", { bubbles: true, ...client(line, 320, 200) }));
  await settle();
  out.reset = columns(line);

  out.barPrevented = wheel(bar, -200);
  out.narrowWidth = document.querySelector("#narrow").scrollWidth;

  step("brush");
  const dots = scatter.querySelectorAll(".ch-m").length;
  let box = null;
  await drag(scatter, [200, 100], [400, 300], true, () => {
    const r = shade(scatter)?.getBoundingClientRect();
    box = r && { w: Math.round(r.width), h: Math.round(r.height) };
  });
  out.brush = { box, dotsBefore: dots, dotsAfter: scatter.querySelectorAll(".ch-m").length, shadeGone: !shade(scatter) };

  step("cancel");
  const html = scatter.querySelector("svg").outerHTML;
  send(scatter, "pointerdown", 200, 100, { shiftKey: true });
  for (let k = 1; k <= 5; k++) send(scatter, "pointermove", 200 + 20 * k, 100 + 20 * k, { shiftKey: true });
  const shaded = !!shade(scatter);
  send(scatter, "pointercancel", 300, 200);
  await settle();
  out.cancel = { shaded, shadeGone: !shade(scatter), zoomKept: scatter.querySelector("svg").outerHTML === html };

  step("done");
  out.pass =
    out.zooms.join() === "true,true,false" && out.columns === 8 &&
    out.wheel.prevented && out.wheel.columns === 5 && out.wheel.signupsOff && out.wheel.signupsHidden &&
    out.hoverTip &&
    out.pan.to !== out.pan.from && out.pan.columns === 5 && out.pan.visitsOn &&
    out.reset === 8 && !out.barPrevented && out.narrowWidth <= 100 &&
    out.brush.box && out.brush.box.w === 100 && out.brush.box.h === 100 &&
    out.brush.dotsAfter < out.brush.dotsBefore && out.brush.dotsAfter > 0 && out.brush.shadeGone &&
    out.cancel.shaded && out.cancel.shadeGone && out.cancel.zoomKept;
} catch (e) {
  out.error = String(e && e.stack || e);
  out.pass = false;
}
pre.textContent = JSON.stringify(out);
JS
  echo '</script></body></html>'
} >"$page"

port=$(python3 -I -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')
python3 -I -m http.server "$port" --bind 127.0.0.1 --directory "$out" >"$work/server.log" 2>&1 &
server=$!
trap 'kill $server 2>/dev/null' EXIT
for _ in 1 2 3 4 5 6 7 8 9 10; do
  curl -sf -o /dev/null "http://127.0.0.1:$port/zoom-test.html" && break
  sleep 0.3
done

# --no-sandbox: the Linux runners' AppArmor refuses Chrome's sandbox; the
# page is our own.
# --dump-dom now and then never returns: a minute is far past any real run.
"$chrome" --headless=new --disable-gpu --no-sandbox --window-size=900,1400 \
  --virtual-time-budget=10000 --dump-dom "http://127.0.0.1:$port/zoom-test.html" \
  >"$work/dom.html" 2>"$work/chrome.log" &
browser=$!
# Off our stdout: the sleep it leaves behind must not hold the pipe open.
(sleep 60; kill -9 $browser) >/dev/null 2>&1 &
watchdog=$!
wait $browser
kill $watchdog
wait $watchdog 2>/dev/null
json=$(python3 -I -c '
import html, re, sys
m = re.search(r"<pre id=\"probe\">(.*?)</pre>", open(sys.argv[1]).read(), re.S)
print(html.unescape(m.group(1)) if m else "")' "$work/dom.html")
[ -n "$json" ] || { echo "the page produced no probe; see $work" >&2; exit 2; }
echo "$json"
case $json in *'"pass":true'*) exit 0 ;; *) exit 1 ;; esac
