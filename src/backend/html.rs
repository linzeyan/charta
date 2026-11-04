//! HTML/SVG output with a small embedded runtime for tooltips, hover
//! highlight and keyboard focus. Colors are CSS variables, so the page can
//! switch light/dark without re-rendering.

use std::fmt::Write;

use crate::base::format::coord;
use crate::base::text::{CAP_HEIGHT, Weight};
use crate::base::theme::{Color, Theme, palette};
use crate::scene::{
    Anchor, Baseline, Corners, HitKind, Mark, Rect, Scene, Shape, json_str, path_data,
};

const RUNTIME_JS: &str = include_str!("../../assets/runtime.js");
const RUNTIME_CSS: &str = include_str!("../../assets/runtime.css");
const FONT_STACK: &str = "Inter,system-ui,-apple-system,\"Segoe UI\",sans-serif";

/// Output flavor.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum Format {
    /// A `<div>` with the SVG, tooltip, data table and runtime; embed it in a page.
    #[default]
    Html,
    /// A standalone SVG without script: native `<title>` tooltips only. Use
    /// it for `<img>`, documents and files.
    Svg,
}

/// How to render a chart.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Options {
    pub width: f64,
    pub height: f64,
    pub theme: Theme,
    pub format: Format,
}

impl Default for Options {
    fn default() -> Self {
        Options {
            width: 640.0,
            height: 400.0,
            theme: Theme::Auto,
            format: Format::Html,
        }
    }
}

/// Renders a scene; `opts.width`/`height` are ignored in favor of the scene's.
pub fn render(scene: &Scene, opts: &Options) -> String {
    let mut out = String::new();
    let (w, h) = (coord(scene.width), coord(scene.height));
    match opts.format {
        Format::Html => {
            let _ = write!(
                out,
                "<div class=\"charta\"><style>{}{RUNTIME_CSS}</style>\
                 <svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 {w} {h}\" width=\"{w}\" height=\"{h}\" role=\"group\" aria-label=\"{}\">",
                theme_css(opts.theme, false),
                esc(&scene.label)
            );
            for m in &scene.marks {
                mark(&mut out, m, true);
            }
            out.push_str(
                "</svg><div class=\"ch-tip\" hidden><strong></strong><span></span><div class=\"ch-rows\"></div></div>",
            );
            table(&mut out, scene);
            let _ = write!(out, "<script>{RUNTIME_JS}</script></div>");
        }
        Format::Svg => {
            let _ = write!(
                out,
                "<svg xmlns=\"http://www.w3.org/2000/svg\" class=\"charta\" viewBox=\"0 0 {w} {h}\" width=\"{w}\" height=\"{h}\" role=\"img\" aria-label=\"{}\">\
                 <style>{}svg.charta{{background:var(--ch-surface);font-family:{FONT_STACK}}}</style>",
                esc(&scene.label),
                theme_css(opts.theme, true)
            );
            for m in &scene.marks {
                mark(&mut out, m, false);
            }
            out.push_str("</svg>");
        }
    }
    out
}

/// CSS variables for the palette. In a standalone SVG the chart is the
/// document root, so the page-level `data-theme` override does not apply.
fn theme_css(theme: Theme, standalone: bool) -> String {
    let vars = |dark: bool| {
        let mut s = format!("color-scheme:{};", if dark { "dark" } else { "light" });
        for e in palette() {
            let _ = write!(
                s,
                "--ch-{}:{};",
                e.token,
                if dark { e.dark } else { e.light }.hex()
            );
        }
        s
    };
    match (theme, standalone) {
        (Theme::Light, _) => format!(".charta{{{}}}", vars(false)),
        (Theme::Dark, _) => format!(".charta{{{}}}", vars(true)),
        (Theme::Auto, true) => format!(
            ".charta{{{}}}@media (prefers-color-scheme:dark){{.charta{{{}}}}}",
            vars(false),
            vars(true)
        ),
        // The viewer's theme toggle (data-theme on the page root) must win in
        // both directions; :where() keeps the media block below the toggle.
        (Theme::Auto, false) => {
            let dark = vars(true);
            format!(
                ".charta{{{}}}@media (prefers-color-scheme:dark){{:root:where(:not([data-theme=\"light\"])) .charta{{{dark}}}}}\
                 :root[data-theme=\"dark\"] .charta{{{dark}}}",
                vars(false)
            )
        }
    }
}

fn mark(out: &mut String, m: &Mark, interactive: bool) {
    let series = m
        .series
        .as_ref()
        .map(|s| format!(" data-series=\"{}\"", esc(s)))
        .unwrap_or_default();
    let Some(hit) = &m.hit else {
        shape(out, m, None, &series);
        return;
    };
    if !interactive {
        match hit.kind {
            // A static file has no hover to reveal a crosshair with.
            HitKind::Crosshair => {}
            HitKind::Legend => shape(out, m, None, ""),
            HitKind::Mark => {
                let _ = write!(
                    out,
                    "<g><title>{}: {}</title>",
                    esc(&hit.label),
                    esc(&hit.value)
                );
                shape(out, m, None, "");
                out.push_str("</g>");
            }
        }
        return;
    }
    let class = match hit.kind {
        HitKind::Mark => "ch-m",
        HitKind::Crosshair => "ch-m ch-x",
        HitKind::Legend => "ch-m ch-leg",
    };
    let _ = write!(
        out,
        "<g class=\"{class}\" data-key=\"{}\"{series}",
        esc(&hit.key)
    );
    match hit.kind {
        HitKind::Mark | HitKind::Crosshair => {
            let rows: Vec<String> = hit
                .rows
                .iter()
                .map(|r| {
                    let c = r
                        .color
                        .map(|c| format!("\"c\":\"{}\",", c.token()))
                        .unwrap_or_default();
                    format!(
                        "{{{c}\"n\":{},\"v\":{}}}",
                        json_str(&r.name),
                        json_str(&r.value)
                    )
                })
                .collect();
            let mut spoken: Vec<String> = Vec::new();
            if !hit.value.is_empty() {
                spoken.push(hit.value.clone());
            }
            spoken.extend(hit.rows.iter().map(|r| format!("{} {}", r.name, r.value)));
            let _ = write!(out, " data-tip-label=\"{}\"", esc(&hit.label));
            if !hit.value.is_empty() {
                let _ = write!(out, " data-tip-value=\"{}\"", esc(&hit.value));
            }
            if !rows.is_empty() {
                let _ = write!(
                    out,
                    " data-rows=\"{}\"",
                    esc(&format!("[{}]", rows.join(",")))
                );
            }
            let _ = write!(
                out,
                " tabindex=\"0\" role=\"img\" aria-label=\"{}: {}\">",
                esc(&hit.label),
                esc(&spoken.join(", "))
            );
        }
        HitKind::Legend => {
            let _ = write!(
                out,
                " tabindex=\"0\" role=\"button\" aria-pressed=\"true\" aria-label=\"{}\">",
                esc(&hit.label)
            );
        }
    }
    if let Some(a) = hit.area {
        rect_el(out, a, &Corners::default(), "class=\"ch-hit\"");
    }
    shape(out, m, Some("ch-s"), "");
    out.push_str("</g>");
}

fn shape(out: &mut String, m: &Mark, class: Option<&str>, extra: &str) {
    let mut attrs = String::new();
    if let Some(c) = class {
        let _ = write!(attrs, "class=\"{c}\" ");
    }
    let _ = write!(attrs, "style=\"{}\"{extra}", style(m));
    match &m.shape {
        Shape::Rect { rect, radius } => rect_el(out, *rect, radius, &attrs),
        Shape::Line { x1, y1, x2, y2 } => {
            let _ = write!(
                out,
                "<line x1=\"{}\" y1=\"{}\" x2=\"{}\" y2=\"{}\" {attrs}/>",
                coord(*x1),
                coord(*y1),
                coord(*x2),
                coord(*y2)
            );
        }
        Shape::Polyline { points } => {
            let pts: Vec<String> = points
                .iter()
                .map(|&(x, y)| format!("{},{}", coord(x), coord(y)))
                .collect();
            let _ = write!(out, "<polyline points=\"{}\" {attrs}/>", pts.join(" "));
        }
        Shape::Area { points } => {
            let mut d = String::new();
            for (i, &(x, t, _)) in points.iter().enumerate() {
                let _ = write!(
                    d,
                    "{}{} {}",
                    if i == 0 { 'M' } else { 'L' },
                    coord(x),
                    coord(t)
                );
            }
            for &(x, _, b) in points.iter().rev() {
                let _ = write!(d, "L{} {}", coord(x), coord(b));
            }
            d.push('Z');
            let _ = write!(out, "<path d=\"{d}\" {attrs}/>");
        }
        Shape::Path { ops } => {
            let _ = write!(out, "<path d=\"{}\" {attrs}/>", path_data(ops));
        }
        Shape::Circle { cx, cy, r } => {
            let _ = write!(
                out,
                "<circle cx=\"{}\" cy=\"{}\" r=\"{}\" {attrs}/>",
                coord(*cx),
                coord(*cy),
                coord(*r)
            );
        }
        Shape::Text {
            x,
            y,
            text,
            anchor,
            baseline,
            size,
            angle,
            ..
        } => {
            let shift = match baseline {
                Baseline::Top => CAP_HEIGHT * size,
                Baseline::Middle => CAP_HEIGHT * size / 2.0,
                Baseline::Bottom => 0.0,
            };
            let anchor = match anchor {
                Anchor::Start => "start",
                Anchor::Middle => "middle",
                Anchor::End => "end",
            };
            let _ = write!(
                out,
                "<text x=\"{}\" y=\"{}\" text-anchor=\"{anchor}\" {attrs}",
                coord(*x),
                coord(y + shift)
            );
            if *angle != 0.0 {
                let _ = write!(
                    out,
                    " transform=\"rotate({} {} {})\"",
                    coord(*angle),
                    coord(*x),
                    coord(*y)
                );
            }
            let _ = write!(out, ">{}</text>", esc(text));
        }
    }
}

fn style(m: &Mark) -> String {
    let mut s = match m.fill {
        Some(c) => format!("fill:{}", var(c)),
        None => "fill:none".to_string(),
    };
    if let Some(st) = m.stroke {
        let _ = write!(
            s,
            ";stroke:{};stroke-width:{}",
            var(st.color),
            coord(st.width)
        );
    }
    if let Shape::Polyline { .. } | Shape::Path { .. } = &m.shape
        && m.stroke.is_some()
    {
        s.push_str(";stroke-linejoin:round;stroke-linecap:round");
    }
    if let Shape::Text { size, weight, .. } = &m.shape {
        let w = match weight {
            Weight::Regular => 400,
            Weight::Semibold => 600,
        };
        let _ = write!(s, ";font-size:{}px;font-weight:{w}", coord(*size));
    }
    s
}

fn var(c: Color) -> String {
    format!("var(--ch-{})", c.token())
}

/// A rect, or a path when only some corners are rounded (SVG's `rx` rounds all four).
fn rect_el(out: &mut String, r: Rect, c: &Corners, attrs: &str) {
    if c.tl == 0.0 && c.tr == 0.0 && c.br == 0.0 && c.bl == 0.0 {
        let _ = write!(
            out,
            "<rect x=\"{}\" y=\"{}\" width=\"{}\" height=\"{}\" {attrs}/>",
            coord(r.x),
            coord(r.y),
            coord(r.w),
            coord(r.h)
        );
        return;
    }
    let (x0, y0, x1, y1) = (r.x, r.y, r.x + r.w, r.y + r.h);
    let mut d = format!("M{} {}H{}", coord(x0 + c.tl), coord(y0), coord(x1 - c.tr));
    corner(&mut d, c.tr, x1, y0 + c.tr);
    let _ = write!(d, "V{}", coord(y1 - c.br));
    corner(&mut d, c.br, x1 - c.br, y1);
    let _ = write!(d, "H{}", coord(x0 + c.bl));
    corner(&mut d, c.bl, x0, y1 - c.bl);
    let _ = write!(d, "V{}", coord(y0 + c.tl));
    corner(&mut d, c.tl, x0 + c.tl, y0);
    d.push('Z');
    let _ = write!(out, "<path d=\"{d}\" {attrs}/>");
}

fn corner(d: &mut String, r: f64, x: f64, y: f64) {
    if r > 0.0 {
        let _ = write!(
            d,
            "A{} {} 0 0 1 {} {}",
            coord(r),
            coord(r),
            coord(x),
            coord(y)
        );
    }
}

/// Screen-reader table of every interactive mark: the values a tooltip shows
/// must also be reachable without hovering. A div hides it: a table grows to
/// fit its cells whatever its width, and its clipped box still widened a
/// narrow page into a horizontal scroll.
fn table(out: &mut String, scene: &Scene) {
    let _ = write!(
        out,
        "<div class=\"ch-sr\"><table><caption>{}</caption><thead><tr><th scope=\"col\">Label</th><th scope=\"col\">Value</th></tr></thead><tbody>",
        esc(&scene.label)
    );
    let row = |out: &mut String, th: &str, td: &str| {
        let _ = write!(
            out,
            "<tr><th scope=\"row\">{}</th><td>{}</td></tr>",
            esc(th),
            esc(td)
        );
    };
    for h in scene.marks.iter().filter_map(|m| m.hit.as_ref()) {
        if h.kind == HitKind::Legend {
            continue;
        }
        if !h.value.is_empty() || h.rows.is_empty() {
            row(out, &h.label, &h.value);
        }
        for r in &h.rows {
            row(out, &format!("{} · {}", h.label, r.name), &r.value);
        }
    }
    out.push_str("</tbody></table></div>");
}

fn esc(s: &str) -> String {
    let mut o = String::with_capacity(s.len());
    for c in s.chars() {
        match c {
            '&' => o.push_str("&amp;"),
            '<' => o.push_str("&lt;"),
            '>' => o.push_str("&gt;"),
            '"' => o.push_str("&quot;"),
            '\'' => o.push_str("&#39;"),
            c => o.push(c),
        }
    }
    o
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{BarChart, Chart};

    #[test]
    fn labels_are_escaped_everywhere() {
        // Category names come from user data; a label must never inject markup.
        let html = BarChart::new([("<img src=x onerror=alert(1)>", 1.0)])
            .render_html(&Options::default())
            .unwrap();
        assert!(!html.contains("<img"), "{html}");
        assert!(html.contains("&lt;img src=x onerror=alert(1)&gt;"));
    }

    #[test]
    fn html_ships_runtime_tooltip_and_data_table() {
        let html = BarChart::new([("A", 30.0)])
            .render_html(&Options::default())
            .unwrap();
        assert_eq!(html.matches("<script>").count(), 1);
        assert!(html.contains("class=\"ch-tip\""));
        assert!(html.contains("<tr><th scope=\"row\">A</th><td>30</td></tr>"));
        assert!(html.contains("data-tip-value=\"30\""));
    }

    #[test]
    fn svg_has_no_script_but_keeps_native_tooltips() {
        let opts = Options {
            format: Format::Svg,
            ..Options::default()
        };
        let svg = BarChart::new([("A", 30.0)]).render_html(&opts).unwrap();
        assert!(svg.starts_with("<svg") && !svg.contains("<script"));
        assert!(svg.contains("<title>A: 30</title>"));
    }

    #[test]
    fn auto_theme_honours_page_toggle_and_os_setting() {
        let css = theme_css(Theme::Auto, false);
        assert!(css.contains("prefers-color-scheme:dark"));
        assert!(css.contains(":root[data-theme=\"dark\"] .charta"));
        assert!(
            !theme_css(Theme::Light, false).contains("#1a1a19"),
            "light theme must not carry dark values"
        );
    }

    #[test]
    fn only_data_end_corners_are_rounded() {
        let mut out = String::new();
        let r = Rect {
            x: 0.0,
            y: 0.0,
            w: 10.0,
            h: 20.0,
        };
        rect_el(
            &mut out,
            r,
            &Corners {
                tl: 4.0,
                tr: 4.0,
                br: 0.0,
                bl: 0.0,
            },
            "",
        );
        assert_eq!(
            out,
            "<path d=\"M4 0H6A4 4 0 0 1 10 4V20H0V4A4 4 0 0 1 4 0Z\" />"
        );
    }
}
