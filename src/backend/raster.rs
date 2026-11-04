//! The tiny-skia rasterizer behind the Slint backend and PNG output
//! (feature `png` or `slint`): a scene painted into a pixmap, text drawn
//! with the system's fonts.
//!
//! ```no_run
//! use charta::BarChart;
//! use charta::backend::raster::render_png;
//!
//! let png: Vec<u8> = render_png(&BarChart::new([("A", 30.0), ("B", 80.0)]), 640.0, 400.0, 2.0, false)
//!     .unwrap();
//! ```

use std::collections::HashMap;
use std::sync::{Mutex, OnceLock};

use fontdb::ID;
use tiny_skia::{
    FillRule, LineCap, LineJoin, Paint, Path, PathBuilder, Pixmap, Stroke as SStroke, Transform,
};
use ttf_parser::{Face, GlyphId, OutlineBuilder};

use super::system_fonts::system;
use crate::Chart;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::{Color, resolve};
use crate::error::Error;
use crate::scene::{
    Anchor, Baseline, Corners, Emphasis, Hit, HitKind, Mark, PathOp, Rect, Scene, Shape,
};

/// Hovered marks lighten by this much toward white, matching the HTML
/// runtime's `brightness(1.12)` lift.
const LIFT: f32 = 0.12;
/// Opacity of other series while one series is highlighted.
const DIM: f32 = 0.35;
/// Tooltip type, padding and offset, matching `.ch-tip` in runtime.css.
const TIP_SIZE: f64 = 12.0;
const TIP_LINE: f64 = 16.2;
const TIP_PAD: (f64, f64) = (8.0, 6.0);
pub(crate) const TIP_OFFSET: f64 = 12.0;
const TIP_RADIUS: f64 = 6.0;
const TIP_KEY: (f64, f64) = (12.0, 2.0);
const TIP_GAP: f64 = 6.0;

/// The chart as a PNG: laid out at `width` × `height` logical pixels with
/// the system fonts it is drawn with, painted at `scale` device pixels per
/// logical pixel, light or dark.
pub fn render_png(
    chart: &dyn Chart,
    width: f64,
    height: f64,
    scale: f32,
    dark: bool,
) -> Result<Vec<u8>, Error> {
    let scene = chart.scene(width, height, &SystemMeasure)?;
    paint(&scene, None, None, &[], scale, dark)
        .encode_png()
        .map_err(|e| Error::InvalidData(format!("PNG encoding failed: {e}")))
}

fn canvas(w: f64, h: f64, scale: f32, dark: bool) -> (Pixmap, Transform) {
    let pw = ((w * scale as f64).round() as u32).max(1);
    let ph = ((h * scale as f64).round() as u32).max(1);
    let mut p = Pixmap::new(pw, ph).expect("non-zero pixmap");
    p.fill(skia(Color::Surface, dark, Emphasis::Normal));
    (p, Transform::from_scale(scale, scale))
}

#[cfg(feature = "slint")]
pub(crate) fn error(msg: &str, w: f64, h: f64, scale: f32, dark: bool) -> Pixmap {
    let (mut p, t) = canvas(w, h, scale, dark);
    let c = skia(Color::TextPrimary, dark, Emphasis::Normal);
    let run = Run::new(msg, TIP_SIZE, Weight::Regular);
    run.draw(
        &mut p,
        t,
        (w / 2.0, h / 2.0),
        Anchor::Middle,
        Baseline::Middle,
        0.0,
        c,
    );
    p
}

/// Paints `scene` with the given interaction state.
pub(crate) fn paint(
    scene: &Scene,
    hovered: Option<usize>,
    pointer: Option<(f64, f64)>,
    hidden: &[String],
    scale: f32,
    dark: bool,
) -> Pixmap {
    let (mut p, t) = canvas(scene.width, scene.height, scale, dark);
    for (i, m) in scene.marks.iter().enumerate() {
        let e = scene.emphasis(i, hovered, hidden);
        if e != Emphasis::Hidden {
            mark(&mut p, t, m, dark, e);
        }
    }
    if let (Some(i), Some(at)) = (hovered, pointer)
        && let Some(hit) = &scene.marks[i].hit
        && hit.kind != HitKind::Legend
    {
        tooltip(
            &mut p,
            t,
            hit,
            hidden,
            at,
            (scene.width, scene.height),
            dark,
        );
    }
    p
}

fn skia(c: Color, dark: bool, e: Emphasis) -> tiny_skia::Color {
    let v = resolve(c, dark);
    let lift = |x: u8| x as f32 / 255.0 + (1.0 - x as f32 / 255.0) * LIFT;
    let (mut r, mut g, mut b, mut a) = (
        v.0 as f32 / 255.0,
        v.1 as f32 / 255.0,
        v.2 as f32 / 255.0,
        v.3 as f32 / 255.0,
    );
    match e {
        Emphasis::Lift => (r, g, b) = (lift(v.0), lift(v.1), lift(v.2)),
        Emphasis::Dim => a *= DIM,
        Emphasis::Normal | Emphasis::Hidden => {}
    }
    (r, g, b) = (r.min(1.0), g.min(1.0), b.min(1.0));
    tiny_skia::Color::from_rgba(r, g, b, a).expect("color channels in range")
}

fn solid(c: tiny_skia::Color) -> Paint<'static> {
    let mut p = Paint::default();
    p.set_color(c);
    p.anti_alias = true;
    p
}

fn mark(p: &mut Pixmap, t: Transform, m: &Mark, dark: bool, e: Emphasis) {
    let fill = m.fill.map(|c| solid(skia(c, dark, e)));
    let stroke = m.stroke.map(|s| (solid(skia(s.color, dark, e)), s.width));
    let fill_path = |p: &mut Pixmap, path: &Path| {
        if let Some(f) = &fill {
            p.fill_path(path, f, FillRule::Winding, t, None);
        }
    };
    let stroke_path = |p: &mut Pixmap, path: &Path, cap: LineCap, join: LineJoin| {
        if let Some((paint, width)) = &stroke {
            let s = SStroke {
                width: *width as f32,
                line_cap: cap,
                line_join: join,
                ..SStroke::default()
            };
            p.stroke_path(path, paint, &s, t, None);
        }
    };
    match &m.shape {
        Shape::Rect { rect, radius } => {
            if let Some(path) = rounded(*rect, *radius) {
                fill_path(p, &path);
                stroke_path(p, &path, LineCap::Butt, LineJoin::Miter);
            }
        }
        Shape::Line { x1, y1, x2, y2 } => {
            let mut b = PathBuilder::new();
            b.move_to(*x1 as f32, *y1 as f32);
            b.line_to(*x2 as f32, *y2 as f32);
            if let Some(path) = b.finish() {
                stroke_path(p, &path, LineCap::Butt, LineJoin::Miter);
            }
        }
        Shape::Polyline { points } => {
            let mut b = PathBuilder::new();
            for (k, &(x, y)) in points.iter().enumerate() {
                if k == 0 {
                    b.move_to(x as f32, y as f32);
                } else {
                    b.line_to(x as f32, y as f32);
                }
            }
            if let Some(path) = b.finish() {
                stroke_path(p, &path, LineCap::Round, LineJoin::Round);
            }
        }
        Shape::Area { points } => {
            let mut b = PathBuilder::new();
            for (k, &(x, top, _)) in points.iter().enumerate() {
                if k == 0 {
                    b.move_to(x as f32, top as f32);
                } else {
                    b.line_to(x as f32, top as f32);
                }
            }
            for &(x, _, bottom) in points.iter().rev() {
                b.line_to(x as f32, bottom as f32);
            }
            b.close();
            if let Some(path) = b.finish() {
                fill_path(p, &path);
            }
        }
        Shape::Path { ops } => {
            let mut b = PathBuilder::new();
            for op in ops {
                match *op {
                    PathOp::Move(x, y) => b.move_to(x as f32, y as f32),
                    PathOp::Line(x, y) => b.line_to(x as f32, y as f32),
                    PathOp::Cubic(x1, y1, x2, y2, x, y) => b.cubic_to(
                        x1 as f32, y1 as f32, x2 as f32, y2 as f32, x as f32, y as f32,
                    ),
                    PathOp::Close => b.close(),
                }
            }
            if let Some(path) = b.finish() {
                fill_path(p, &path);
                stroke_path(p, &path, LineCap::Round, LineJoin::Round);
            }
        }
        Shape::Circle { cx, cy, r } => {
            if let Some(path) = PathBuilder::from_circle(*cx as f32, *cy as f32, *r as f32) {
                fill_path(p, &path);
                stroke_path(p, &path, LineCap::Butt, LineJoin::Miter);
            }
        }
        Shape::Text {
            x,
            y,
            text,
            size,
            weight,
            anchor,
            baseline,
            angle,
        } => {
            let c = skia(m.fill.unwrap_or(Color::TextPrimary), dark, e);
            Run::new(text, *size, *weight).draw(p, t, (*x, *y), *anchor, *baseline, *angle, c);
        }
    }
}

/// A rectangle with independent corner radii (bars round only their data
/// end). Cubic arcs; radii clamp to half the shorter side.
fn rounded(r: Rect, c: Corners) -> Option<Path> {
    // Control point distance for a quarter circle drawn as one cubic.
    const K: f32 = 0.552_284_8;
    let (x, y, w, h) = (r.x as f32, r.y as f32, r.w as f32, r.h as f32);
    let max = w.min(h) / 2.0;
    let [tl, tr, br, bl] = [c.tl, c.tr, c.br, c.bl].map(|v| (v as f32).clamp(0.0, max));
    let mut b = PathBuilder::new();
    b.move_to(x + tl, y);
    b.line_to(x + w - tr, y);
    b.cubic_to(
        x + w - tr * (1.0 - K),
        y,
        x + w,
        y + tr * (1.0 - K),
        x + w,
        y + tr,
    );
    b.line_to(x + w, y + h - br);
    b.cubic_to(
        x + w,
        y + h - br * (1.0 - K),
        x + w - br * (1.0 - K),
        y + h,
        x + w - br,
        y + h,
    );
    b.line_to(x + bl, y + h);
    b.cubic_to(
        x + bl * (1.0 - K),
        y + h,
        x,
        y + h - bl * (1.0 - K),
        x,
        y + h - bl,
    );
    b.line_to(x, y + tl);
    b.cubic_to(x, y + tl * (1.0 - K), x + tl * (1.0 - K), y, x + tl, y);
    b.close();
    b.finish()
}

/// Values lead, labels follow, then any rows; a crosshair lists every
/// visible series, each keyed by a short stroke of its color. Same content
/// and order as the HTML runtime's tooltip.
fn tooltip(
    p: &mut Pixmap,
    t: Transform,
    hit: &Hit,
    hidden: &[String],
    at: (f64, f64),
    size: (f64, f64),
    dark: bool,
) {
    enum Line {
        Text(Run, Color),
        Row(Option<Color>, Run, Run),
    }
    let text = |s: &str, w: Weight, c: Color| Line::Text(Run::new(s, TIP_SIZE, w), c);
    let key_w = |c: &Option<Color>| c.map_or(0.0, |_| TIP_KEY.0 + TIP_GAP);
    let mut lines: Vec<Line> = Vec::new();
    if !hit.value.is_empty() {
        lines.push(text(&hit.value, Weight::Semibold, Color::TextPrimary));
    }
    lines.push(text(&hit.label, Weight::Regular, Color::TextSecondary));
    lines.extend(
        hit.rows
            .iter()
            .filter(|r| !hidden.contains(&r.name))
            .map(|r| {
                Line::Row(
                    r.color,
                    Run::new(&r.value, TIP_SIZE, Weight::Semibold),
                    Run::new(&r.name, TIP_SIZE, Weight::Regular),
                )
            }),
    );
    let width = lines
        .iter()
        .map(|l| match l {
            Line::Text(r, _) => r.width,
            Line::Row(c, v, n) => key_w(c) + v.width + TIP_GAP + n.width,
        })
        .fold(0.0, f64::max);
    let (bw, bh) = (
        width + 2.0 * TIP_PAD.0,
        lines.len() as f64 * TIP_LINE + 2.0 * TIP_PAD.1,
    );
    // Flip to the pointer's left/top when the box would leave the chart.
    let mut x = at.0 + TIP_OFFSET;
    if x + bw > size.0 {
        x = at.0 - TIP_OFFSET - bw;
    }
    let mut y = at.1 + TIP_OFFSET;
    if y + bh > size.1 {
        y = at.1 - TIP_OFFSET - bh;
    }
    let (x, y) = (x.max(0.0), y.max(0.0));

    let box_ = Rect { x, y, w: bw, h: bh };
    if let Some(path) = rounded(box_, Corners::all(TIP_RADIUS)) {
        let surface = solid(skia(Color::Surface, dark, Emphasis::Normal));
        p.fill_path(&path, &surface, FillRule::Winding, t, None);
        let border = solid(skia(Color::Border, dark, Emphasis::Normal));
        p.stroke_path(&path, &border, &SStroke::default(), t, None);
    }
    let ink = |c: Color| skia(c, dark, Emphasis::Normal);
    for (k, line) in lines.iter().enumerate() {
        let mid = y + TIP_PAD.1 + (k as f64 + 0.5) * TIP_LINE;
        let left = x + TIP_PAD.0;
        match line {
            Line::Text(run, c) => run.draw(
                p,
                t,
                (left, mid),
                Anchor::Start,
                Baseline::Middle,
                0.0,
                ink(*c),
            ),
            Line::Row(c, value, name) => {
                if let Some(c) = c {
                    let key = Rect {
                        x: left,
                        y: mid - TIP_KEY.1 / 2.0,
                        w: TIP_KEY.0,
                        h: TIP_KEY.1,
                    };
                    if let Some(path) = rounded(key, Corners::all(TIP_KEY.1 / 2.0)) {
                        p.fill_path(&path, &solid(ink(*c)), FillRule::Winding, t, None);
                    }
                }
                let vx = left + key_w(c);
                let tp = ink(Color::TextPrimary);
                value.draw(p, t, (vx, mid), Anchor::Start, Baseline::Middle, 0.0, tp);
                let nx = vx + value.width + TIP_GAP;
                let ts = ink(Color::TextSecondary);
                name.draw(p, t, (nx, mid), Anchor::Start, Baseline::Middle, 0.0, ts);
            }
        }
    }
}

// --- Text -----------------------------------------------------------------

/// Measures with the same system fonts the bitmap is drawn with, so labels
/// fit their glyphs.
pub struct SystemMeasure;

impl TextMeasure for SystemMeasure {
    fn width(&self, text: &str, size: f64, weight: Weight) -> f64 {
        Run::new(text, size, weight).width
    }
}

/// Glyphs of the system fonts ([`system`]), drawn the way Slint itself
/// draws text, cached per character and per outline.
#[derive(Default)]
struct Fonts {
    glyphs: Mutex<HashMap<(char, bool), Option<Glyph>>>,
    outlines: Mutex<HashMap<(ID, u16), Option<Path>>>,
}

#[derive(Clone, Copy)]
struct Glyph {
    face: ID,
    id: u16,
    /// Advance in ems.
    advance: f64,
}

fn fonts() -> &'static Fonts {
    static FONTS: OnceLock<Fonts> = OnceLock::new();
    FONTS.get_or_init(Fonts::default)
}

impl Fonts {
    fn glyph(&self, c: char, weight: Weight) -> Option<Glyph> {
        let key = (c, weight == Weight::Semibold);
        if let Some(g) = self.glyphs.lock().unwrap().get(&key) {
            return *g;
        }
        let g = system()
            .candidates(weight)
            .find_map(|face| self.lookup(face, c));
        self.glyphs.lock().unwrap().insert(key, g);
        g
    }

    fn lookup(&self, face: ID, c: char) -> Option<Glyph> {
        system()
            .db
            .with_face_data(face, |data, i| {
                let f = Face::parse(data, i).ok()?;
                let id = f.glyph_index(c)?;
                let advance = f.glyph_hor_advance(id)? as f64 / f.units_per_em() as f64;
                Some(Glyph {
                    face,
                    id: id.0,
                    advance,
                })
            })
            .flatten()
    }

    /// The glyph outline in ems, y down, origin on the baseline.
    fn outline(&self, g: Glyph) -> Option<Path> {
        let key = (g.face, g.id);
        if let Some(p) = self.outlines.lock().unwrap().get(&key) {
            return p.clone();
        }
        let path = system()
            .db
            .with_face_data(g.face, |data, i| {
                let f = Face::parse(data, i).ok()?;
                let mut b = Outline {
                    b: PathBuilder::new(),
                    s: 1.0 / f.units_per_em() as f32,
                };
                f.outline_glyph(GlyphId(g.id), &mut b)?;
                b.b.finish()
            })
            .flatten();
        self.outlines.lock().unwrap().insert(key, path.clone());
        path
    }
}

struct Outline {
    b: PathBuilder,
    s: f32,
}

impl OutlineBuilder for Outline {
    fn move_to(&mut self, x: f32, y: f32) {
        self.b.move_to(x * self.s, -y * self.s);
    }
    fn line_to(&mut self, x: f32, y: f32) {
        self.b.line_to(x * self.s, -y * self.s);
    }
    fn quad_to(&mut self, x1: f32, y1: f32, x: f32, y: f32) {
        let s = self.s;
        self.b.quad_to(x1 * s, -y1 * s, x * s, -y * s);
    }
    fn curve_to(&mut self, x1: f32, y1: f32, x2: f32, y2: f32, x: f32, y: f32) {
        let s = self.s;
        self.b
            .cubic_to(x1 * s, -y1 * s, x2 * s, -y2 * s, x * s, -y * s);
    }
    fn close(&mut self) {
        self.b.close();
    }
}

/// One line of text resolved to glyphs and pen positions (no kerning or
/// shaping: chart labels are short, and measuring uses the same advances).
struct Run {
    glyphs: Vec<(Glyph, f64)>,
    size: f64,
    width: f64,
}

impl Run {
    fn new(text: &str, size: f64, weight: Weight) -> Self {
        let fonts = fonts();
        let mut pen = 0.0;
        let mut glyphs = Vec::new();
        for c in text.chars() {
            match fonts.glyph(c, weight) {
                Some(g) => {
                    glyphs.push((g, pen));
                    pen += g.advance * size;
                }
                // Nothing installed has it: leave a gap rather than nothing.
                None => pen += size * 0.6,
            }
        }
        Run {
            glyphs,
            size,
            width: pen,
        }
    }

    #[allow(clippy::too_many_arguments)]
    fn draw(
        &self,
        p: &mut Pixmap,
        t: Transform,
        at: (f64, f64),
        anchor: Anchor,
        baseline: Baseline,
        angle: f64,
        color: tiny_skia::Color,
    ) {
        let fonts = fonts();
        let dx = match anchor {
            Anchor::Start => 0.0,
            Anchor::Middle => -self.width / 2.0,
            Anchor::End => -self.width,
        };
        let cap = system().cap * self.size;
        let dy = match baseline {
            Baseline::Top => cap,
            Baseline::Middle => cap / 2.0,
            Baseline::Bottom => 0.0,
        };
        let placed = t
            .pre_translate(at.0 as f32, at.1 as f32)
            .pre_rotate(angle as f32);
        let paint = solid(color);
        for &(g, pen) in &self.glyphs {
            if let Some(path) = fonts.outline(g) {
                let gt = placed
                    .pre_translate((dx + pen) as f32, dy as f32)
                    .pre_scale(self.size as f32, self.size as f32);
                p.fill_path(&path, &paint, FillRule::Winding, gt, None);
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::BarChart;

    #[test]
    fn a_png_is_the_chart_at_device_size() {
        let png = render_png(
            &BarChart::new([("A", 30.0), ("B", 80.0)]),
            320.0,
            200.0,
            2.0,
            true,
        )
        .unwrap();
        let p = Pixmap::decode_png(&png).unwrap();
        assert_eq!((p.width(), p.height()), (640, 400));
        // Dark surface in the corner, not the light one.
        let c = p.pixel(1, 1).unwrap();
        let s = resolve(Color::Surface, true);
        assert_eq!((c.red(), c.green(), c.blue()), (s.0, s.1, s.2));
    }
}
