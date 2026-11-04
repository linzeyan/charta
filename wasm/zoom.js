// Zoom a chart charta rendered to HTML: the wheel zooms around the pointer,
// dragging pans, shift-dragging brushes a range (a box on scatterplots) to
// zoom to, and a double click shows everything again. Charts with a
// category axis (line, area) or a continuous one (scatter, bubble,
// candlestick, OHLC, Gantt, stream) zoom; others are left alone.
//
//   import { zoom } from "./zoom.js";
//   await zoom(document.querySelector(".charta"), specJson);
//
// `spec` is the JSON the chart was rendered from (either port renders the
// same HTML from it); pass `{ theme }` when it was rendered with a theme
// other than "auto". The size is read from the SVG. Serve this file with
// charta_wasm.js and charta_wasm_bg.wasm beside it (`make wasm`).

import init, { ZoomChart } from "./charta_wasm.js";

let ready;

// What a gesture changed (see charta-wasm).
const BRUSH = 1;
const ZOOM = 2;

/** Makes `root`, a `.charta` element, zoomable; resolves to whether its chart zooms. */
export async function zoom(root, spec, { theme = "auto" } = {}) {
  ready ??= init();
  await ready;
  const box = root.querySelector("svg").viewBox.baseVal;
  const [w, h] = [box.width, box.height];
  const chart = new ZoomChart(typeof spec === "string" ? spec : JSON.stringify(spec), w, h, theme);
  if (!chart.zooms()) {
    chart.free();
    return false;
  }
  // Drags pan rather than select text or scroll the page.
  root.style.userSelect = "none";
  root.style.touchAction = "none";

  // The SVG is swapped out on every zoom: look it up each time.
  const svg = () => root.querySelector("svg");
  const at = (e) => {
    const r = svg().getBoundingClientRect();
    return [((e.clientX - r.left) * w) / r.width, ((e.clientY - r.top) * h) / r.height];
  };

  let shade = null;
  function drawBrush() {
    const b = chart.brush();
    if (b.length === 0) {
      shade?.remove();
      shade = null;
      return;
    }
    if (!shade) {
      shade = document.createElement("div");
      shade.style.cssText = "position:absolute;pointer-events:none;background:var(--ch-series-1-link)";
    }
    if (!shade.isConnected) root.append(shade);
    const s = svg();
    const [kx, ky] = [s.clientWidth / w, s.clientHeight / h];
    Object.assign(shade.style, {
      left: `${s.offsetLeft + b[0] * kx}px`,
      top: `${s.offsetTop + b[1] * ky}px`,
      width: `${b[2] * kx}px`,
      height: `${b[3] * ky}px`,
    });
  }

  function show() {
    const hidden = new Set(Array.from(root.querySelectorAll(".ch-leg.ch-off"), (l) => l.dataset.series));
    const t = document.createElement("template");
    t.innerHTML = chart.html();
    // Keep the root, and the listeners on it: swap its contents, then run
    // its runtime again (a parsed script never runs), which binds to the
    // new SVG.
    root.replaceChildren(...t.content.firstElementChild.childNodes);
    delete root.dataset.chReady;
    const parsed = root.querySelector("script");
    const script = document.createElement("script");
    script.textContent = parsed.textContent;
    parsed.replaceWith(script);
    // Series hidden through the legend stay hidden.
    for (const leg of root.querySelectorAll(".ch-leg")) {
      if (hidden.has(leg.dataset.series)) {
        leg.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
      }
    }
    drawBrush();
  }

  // Browsers dispatch wheel turns and pointer moves once a frame at most,
  // so laying out on each one is laying out once a frame.
  function apply(change) {
    if (change === ZOOM) show();
    else if (change === BRUSH) drawBrush();
  }

  root.addEventListener(
    "wheel",
    (e) => {
      if (!e.target.closest("svg")) return;
      e.preventDefault();
      // A notch: 100 px, 3 lines or a page.
      const notch = e.deltaMode === 1 ? 3 : e.deltaMode === 2 ? 1 : 100;
      apply(chart.wheel(...at(e), -e.deltaY / notch));
    },
    { passive: false },
  );
  root.addEventListener("pointerdown", (e) => {
    if (e.button === 0 && e.target.closest("svg")) chart.pressed(...at(e), e.shiftKey);
  });
  root.addEventListener("pointermove", (e) => {
    const change = chart.moved(...at(e));
    apply(change);
    // Capture only once a drag is under way: a captured press would send
    // its click to the root, not the legend entry it was on.
    if (change !== 0 && !root.hasPointerCapture(e.pointerId)) {
      try {
        root.setPointerCapture(e.pointerId);
      } catch {
        // The pointer is up already (or synthetic): nothing to hold.
      }
    }
  });
  root.addEventListener("pointerup", (e) => {
    if (e.button === 0) apply(chart.released(...at(e)));
  });
  root.addEventListener("pointercancel", () => apply(chart.cancelled()));
  // The click a browser fires at the end of a drag is none: a drag that
  // ends on a legend entry must not toggle it.
  root.addEventListener(
    "click",
    (e) => {
      if (chart.dragged()) e.stopPropagation();
    },
    true,
  );
  root.addEventListener("dblclick", (e) => apply(chart.reset(...at(e))));
  return true;
}
