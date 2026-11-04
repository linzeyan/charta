(() => {
  // Each chart carries its own copy of this script; currentScript pins it to
  // its own root so several charts on one page don't interfere.
  const root = document.currentScript && document.currentScript.parentElement;
  if (!root || root.dataset.chReady) return;
  root.dataset.chReady = "1";
  const svg = root.querySelector("svg");
  const tip = root.querySelector(".ch-tip");
  const [tipValue, tipLabel, tipRows] = tip.children;
  const marks = Array.from(svg.querySelectorAll(".ch-m"));
  // Everything drawn for a series except its legend entry: dims while
  // another series is highlighted, disappears while its series is hidden.
  const members = Array.from(svg.querySelectorAll("[data-series]:not(.ch-leg)"));
  // Series dimming only means something with two or more series; a single
  // series just lifts the hovered mark.
  const multi = new Set(members.map((m) => m.dataset.series)).size > 1;
  const hidden = new Set();
  let current = null;

  function emphasize(on) {
    // A hidden series (its legend entry is still hoverable) has nothing on
    // the plot to focus on, so it dims nothing.
    const series = on && !hidden.has(on.dataset.series) ? on.dataset.series : undefined;
    for (const x of marks) x.classList.toggle("ch-on", x === on);
    for (const x of members) {
      x.classList.toggle("ch-dim", multi && series !== undefined && x.dataset.series !== series);
    }
  }

  function show(m) {
    if (current === m) return;
    current = m;
    emphasize(m);
    // Legend entries highlight their series; they have no tooltip.
    if (m.classList.contains("ch-leg")) {
      tip.hidden = true;
      return;
    }
    // Labels are untrusted data: textContent, never innerHTML.
    tipValue.textContent = m.dataset.tipValue || "";
    tipValue.hidden = !m.dataset.tipValue;
    tipLabel.textContent = m.dataset.tipLabel;
    tipRows.replaceChildren();
    for (const r of m.dataset.rows ? JSON.parse(m.dataset.rows) : []) {
      if (hidden.has(r.n)) continue;
      const row = document.createElement("div");
      if (r.c) {
        const key = document.createElement("i");
        key.style.background = `var(--ch-${r.c})`;
        row.append(key);
      }
      const value = document.createElement("strong");
      value.textContent = r.v;
      const name = document.createElement("span");
      name.textContent = r.n;
      row.append(value, name);
      tipRows.append(row);
    }
    tip.hidden = false;
  }

  function hide() {
    current = null;
    emphasize(null);
    tip.hidden = true;
  }

  function place(clientX, clientY) {
    const r = root.getBoundingClientRect();
    let x = clientX - r.left + 12;
    const y = clientY - r.top + 12;
    // Flip to the pointer's left when the tooltip would overflow the chart.
    if (x + tip.offsetWidth > r.width) x = clientX - r.left - 12 - tip.offsetWidth;
    tip.style.transform = `translate(${Math.max(0, x)}px, ${y}px)`;
  }

  // Hiding only changes visibility; the layout (and every other series'
  // color) stays put.
  function toggle(leg) {
    const s = leg.dataset.series;
    if (hidden.has(s)) hidden.delete(s);
    else hidden.add(s);
    leg.classList.toggle("ch-off", hidden.has(s));
    leg.setAttribute("aria-pressed", String(!hidden.has(s)));
    for (const x of members) x.classList.toggle("ch-hide", hidden.has(x.dataset.series));
    // Redraw whatever is under the pointer: a crosshair tooltip's rows and
    // the dimming from a hovered legend entry both depend on what is hidden
    // (a keyboard toggle can land while a crosshair tooltip is up).
    const m = current;
    current = null;
    if (m) show(m);
  }

  svg.addEventListener("pointermove", (e) => {
    const m = e.target.closest(".ch-m");
    if (!m) return hide();
    show(m);
    if (!tip.hidden) place(e.clientX, e.clientY);
  });
  svg.addEventListener("pointerleave", hide);
  svg.addEventListener("click", (e) => {
    const leg = e.target.closest(".ch-leg");
    if (leg) toggle(leg);
  });
  svg.addEventListener("keydown", (e) => {
    const leg = e.target.closest(".ch-leg");
    if (!leg || (e.key !== "Enter" && e.key !== " ")) return;
    e.preventDefault();
    toggle(leg);
  });
  svg.addEventListener("focusin", (e) => {
    const m = e.target.closest(".ch-m");
    if (!m) return;
    show(m);
    const b = m.getBoundingClientRect();
    if (!tip.hidden) place(b.left + b.width / 2, b.top);
  });
  svg.addEventListener("focusout", hide);
})();
