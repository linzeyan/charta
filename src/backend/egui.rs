//! egui backend (feature `egui`): paints a scene with egui shapes and drives
//! hover, tooltips and legend toggles from [`Scene::hit_test`] and
//! [`Scene::emphasis`].
//!
//! ```no_run
//! # fn show(ui: &mut egui::Ui) {
//! use charta::BarChart;
//! use charta::backend::egui::ChartView;
//!
//! let chart = BarChart::new([("A", 30.0), ("B", 80.0)]).title("Sales");
//! ui.add(ChartView::new(&chart).size(egui::vec2(640.0, 400.0)));
//! # }
//! ```

use std::sync::{Arc, Mutex};

use ::egui::epaint::text::{Blob, FallbackRequest, FontInsert, FontProvider};
use ::egui::epaint::{CornerRadius, Mesh, PathShape, PathStroke, RectShape, StrokeKind, TextShape};
use ::egui::{
    Align2, Color32, Context, CursorIcon, Event, FontData, FontFamily, FontId, Galley,
    MouseWheelUnit, Painter, Pos2, Rect as ERect, Response, RichText, Sense, Shape as EShape,
    Stroke as EStroke, Ui, Vec2, Widget, emath::Rot2, pos2, vec2,
};
use fontdb::ID;

use super::system_fonts::system;
use crate::Chart;
use crate::base::text::{CAP_HEIGHT, TextMeasure, Weight};
use crate::base::theme::{Color, resolve};
use crate::scene::{
    Anchor, Baseline, Emphasis, Hit, HitKind, Mark, PathOp, Plot, Scene, Shape, flatten,
};
use crate::zoom::{self, View, Window};

/// Hovered marks lighten by this much toward white, matching the HTML
/// runtime's `brightness(1.12)` lift.
const LIFT: f32 = 0.12;
/// Opacity of other series while one series is highlighted.
const DIM: f32 = 0.35;

/// A chart as an egui widget. Fills the available space unless sized.
/// Clicking a legend entry hides or shows its series; the hidden set lives
/// in egui's memory under the widget's id. Charts with a category axis
/// ([`Chart::category_count`]) or a continuous one ([`Chart::plot_scene`])
/// zoom: the wheel zooms around the pointer, dragging pans, shift-dragging
/// brushes a range (a box, on a chart that zooms y too) to zoom to, and a
/// double click shows everything again.
pub struct ChartView<'a> {
    chart: &'a dyn Chart,
    size: Option<Vec2>,
}

impl<'a> ChartView<'a> {
    pub fn new(chart: &'a dyn Chart) -> Self {
        ChartView { chart, size: None }
    }

    pub fn size(mut self, size: Vec2) -> Self {
        self.size = Some(size);
        self
    }
}

/// Measures with the fonts [`ChartView`] draws with, through egui, so labels
/// fit the glyphs on screen.
pub struct EguiMeasure(pub Context);

impl TextMeasure for EguiMeasure {
    fn width(&self, text: &str, size: f64, weight: Weight) -> f64 {
        let family = families(&self.0)[weight as usize].clone();
        self.0.fonts_mut(|f| {
            f.layout_no_wrap(
                text.to_owned(),
                FontId::new(size as f32, family),
                Color32::WHITE,
            )
            .size()
            .x as f64
        })
    }
}

const REGULAR: &str = "charta";
const SEMIBOLD: &str = "charta-semibold";

/// The font families chart text is set in, indexed by [`Weight`]: the
/// system's sans (Inter when installed), regular and semibold, the faces
/// Slint and PNG output draw with. egui bundles no semibold, so with its own
/// fonts a title would not stand out. The first call on a context installs
/// [`ChartaFonts`]; egui applies a provider from the next pass at the
/// latest, so until then, and on a system with no sans-serif, both weights
/// fall back to egui's proportional font.
fn families(ctx: &Context) -> [FontFamily; 2] {
    let id = ::egui::Id::unique("charta-fonts");
    let pass = ctx.cumulative_pass_nr();
    let installed: Option<Option<u64>> = ctx.data(|d| d.get_temp(id));
    let installed = installed.unwrap_or_else(|| {
        let at = system().primary(Weight::Regular).map(|_| {
            ctx.add_font_provider(Arc::new(ChartaFonts));
            pass
        });
        ctx.data_mut(|d| d.insert_temp(id, at));
        at
    });
    match installed {
        Some(at) if pass > at => [
            FontFamily::Name(REGULAR.into()),
            FontFamily::Name(SEMIBOLD.into()),
        ],
        _ => [FontFamily::Proportional, FontFamily::Proportional],
    }
}

/// Supplies the system faces for the chart families, and any face that has
/// a character they lack (CJK labels).
struct ChartaFonts;

impl ChartaFonts {
    fn weight(family: &FontFamily) -> Option<Weight> {
        match family {
            FontFamily::Name(n) if &**n == REGULAR => Some(Weight::Regular),
            FontFamily::Name(n) if &**n == SEMIBOLD => Some(Weight::Semibold),
            _ => None,
        }
    }

    /// The face as an egui font. A face's bytes are read once and shared:
    /// CJK collections run to tens of megabytes.
    fn insert(face: ID) -> Option<FontInsert> {
        static BLOBS: Mutex<Vec<(ID, Blob, u32)>> = Mutex::new(Vec::new());
        let mut blobs = BLOBS.lock().unwrap();
        let (blob, index) = match blobs.iter().find(|(id, ..)| *id == face) {
            Some((_, b, i)) => (b.clone(), *i),
            None => {
                let (bytes, index) = system()
                    .db
                    .with_face_data(face, |data, i| (data.to_vec(), i))?;
                let blob: Blob = Arc::new(bytes);
                blobs.push((face, blob.clone(), index));
                (blob, index)
            }
        };
        Some(FontInsert::new(
            &format!("charta-{face:?}"),
            FontData::from_blob(blob, index),
            Vec::new(),
        ))
    }
}

impl FontProvider for ChartaFonts {
    fn fonts_for_family(&self, family: &FontFamily) -> Vec<FontInsert> {
        Self::weight(family)
            .and_then(|w| system().primary(w))
            .and_then(Self::insert)
            .into_iter()
            .collect()
    }

    fn font_for(&self, request: &FallbackRequest<'_>) -> Option<FontInsert> {
        let w = Self::weight(request.family)?;
        let c = request.cluster.chars().next()?;
        Self::insert(system().face_for(c, w)?)
    }
}

impl Widget for ChartView<'_> {
    fn ui(self, ui: &mut Ui) -> Response {
        let size = self.size.unwrap_or_else(|| ui.available_size());
        let len = self.chart.category_count().filter(|&n| n > zoom::MIN_SPAN);
        let sense = |zooms: bool| {
            if zooms {
                Sense::click_and_drag()
            } else {
                Sense::click()
            }
        };
        // Space first, sense after: whether a chart zooms continuously is
        // known only from its laid-out plot, and a chart scrolled out of view
        // should not be laid out to find out.
        let (id, space) = ui.allocate_space(size);
        let rect = ui.layout().align_size_within_rect(size, space);
        if !ui.is_rect_visible(rect) {
            return ui.interact(space, id, sense(len.is_some()));
        }
        let painter = ui.painter_at(rect);
        let dark = ui.visuals().dark_mode;
        let measure = EguiMeasure(ui.ctx().clone());
        let (w, h) = (size.x as f64, size.y as f64);
        let zoom_id = id.with("zoom");
        let window = len.map(|n| {
            ui.data(|d| d.get_temp::<Window>(zoom_id))
                .filter(|w| w.len == n)
                .unwrap_or(Window::full(n))
        });
        let narrowed = window
            .filter(|w| !w.is_full())
            .and_then(|w| self.chart.window(w.start..w.end));
        let chart: &dyn Chart = narrowed.as_deref().unwrap_or(self.chart);
        let laid = chart.plot_scene(w, h, &measure);
        // A chart with a plot zooms continuously; the full layout's plot is
        // the extent, and a view kept from other data is dropped.
        let full = laid.as_ref().ok().and_then(|(_, p)| *p);
        let view = full.map(|p| {
            ui.data(|d| d.get_temp::<View>(zoom_id))
                .filter(|v| v.fits(&p))
                .unwrap_or(View::full(&p))
        });
        let viewed = view.filter(|v| !v.is_full()).and_then(|v| {
            let (x, y) = v.ranges();
            self.chart.view(x, y)
        });
        let laid = match viewed {
            Some(c) => c.plot_scene(w, h, &measure),
            None => laid,
        };
        let mut response = ui.interact(space, id, sense(len.is_some() || full.is_some()));
        response.set_intrinsic_size(size);
        let (scene, plot) = match laid {
            Ok(s) => s,
            Err(e) => {
                painter.text(
                    rect.center(),
                    Align2::CENTER_CENTER,
                    e.to_string(),
                    FontId::proportional(12.0),
                    ui.visuals().error_fg_color,
                );
                return response;
            }
        };

        let mut hidden: Vec<String> = ui.data(|d| d.get_temp(id)).unwrap_or_default();
        let at = |p: Pos2| ((p.x - rect.min.x) as f64, (p.y - rect.min.y) as f64);
        let hovered = response
            .hover_pos()
            .and_then(|p| scene.hit_test(at(p).0, at(p).1, &hidden));
        let legend = hovered.filter(|&i| kind(&scene, i) == Some(HitKind::Legend));
        if let Some(i) = legend {
            ui.ctx().set_cursor_icon(CursorIcon::PointingHand);
            if response.clicked()
                && let Some(s) = &scene.marks[i].series
            {
                match hidden.iter().position(|h| h == s) {
                    Some(k) => {
                        hidden.remove(k);
                    }
                    None => hidden.push(s.clone()),
                }
                ui.data_mut(|d| d.insert_temp(id, hidden.clone()));
            }
        }

        painter.rect_filled(rect, 0.0, color(Color::Surface, dark));
        for (i, m) in scene.marks.iter().enumerate() {
            let e = scene.emphasis(i, hovered, &hidden);
            if e != Emphasis::Hidden {
                paint(&painter, rect.min, m, dark, e);
            }
        }

        if let Some(w) = window {
            gestures(
                ui,
                &response,
                &painter,
                &scene,
                w,
                zoom_id,
                dark,
                legend.is_some(),
            );
        }
        if let (Some(v), Some(full), Some(plot)) = (view, full, plot) {
            let zoom = Zoom {
                id: zoom_id,
                dark,
                on_legend: legend.is_some(),
            };
            view_gestures(ui, &response, &painter, &plot, v, &full, zoom);
        }

        match hovered.and_then(|i| scene.marks[i].hit.clone()) {
            Some(hit) if hit.kind != HitKind::Legend => {
                response.on_hover_ui_at_pointer(|ui| tooltip(ui, &hit, &hidden, dark))
            }
            _ => response,
        }
    }
}

/// Wheel notches this frame, positive in. Raw wheel events, not egui's
/// smoothed delta: smoothing spreads a notch over frames and never quite
/// sums to it. A notch is about 50 points or one line; trackpads send
/// fractions.
fn wheel_notches(ui: &Ui) -> f64 {
    ui.input(|i| {
        i.events
            .iter()
            .map(|e| match e {
                Event::MouseWheel { unit, delta, .. } => match unit {
                    MouseWheelUnit::Point => delta.y as f64 / 50.0,
                    MouseWheelUnit::Line | MouseWheelUnit::Page => delta.y as f64,
                },
                _ => 0.0,
            })
            .sum()
    })
}

/// Wheel, drag, shift-drag and double click move the zoom window, kept in
/// egui's memory for the next frame. Pointer positions map to categories
/// through the scene's crosshair columns. A double click on a legend entry
/// toggles its series twice and leaves the zoom alone.
#[allow(clippy::too_many_arguments)]
fn gestures(
    ui: &Ui,
    response: &Response,
    painter: &Painter,
    scene: &Scene,
    w: Window,
    id: ::egui::Id,
    dark: bool,
    on_legend: bool,
) {
    let origin = response.rect.min;
    let col = |p: Pos2| zoom::column_at(scene, (p.x - origin.x) as f64);
    let last = (w.end - w.start).saturating_sub(1);
    let (brush_id, pan_id, wheel_id) = (id.with("brush"), id.with("pan"), id.with("wheel"));
    let mut next = w;
    if response.hovered() {
        // Fractions of a notch add up to whole categories.
        let notches = wheel_notches(ui);
        if notches != 0.0
            && let Some(at) = response.hover_pos().and_then(col)
        {
            let acc = ui.data(|d| d.get_temp::<f64>(wheel_id)).unwrap_or(0.0) + notches;
            let whole = acc.trunc();
            if whole != 0.0 {
                next = next.wheel(at, whole);
            }
            ui.data_mut(|d| d.insert_temp(wheel_id, acc - whole));
        }
    }
    // The brush starts where the button went down: by the time egui calls
    // it a drag the pointer has already moved past the drag threshold.
    if response.drag_started()
        && ui.input(|i| i.modifiers.shift)
        && let Some(at) = ui.input(|i| i.pointer.press_origin()).and_then(col)
    {
        ui.data_mut(|d| d.insert_temp(brush_id, at));
    }
    let brush: Option<f64> = ui.data(|d| d.get_temp(brush_id));
    let pointer = response.interact_pointer_pos().and_then(col);
    match (brush, pointer) {
        (Some(from), Some(to)) => {
            if response.drag_stopped() {
                let (a, b) = (from as usize, to as usize);
                next = next.brush(a.min(last), b.min(last));
                ui.data_mut(|d| d.remove::<f64>(brush_id));
            } else if let (lo, hi) = zoom::brush_edges(from, to)
                && let (Some(x0), Some(x1)) = (zoom::column_x(scene, lo), zoom::column_x(scene, hi))
            {
                let (l, r) = (x0 as f32, x1 as f32);
                painter.rect_filled(
                    ERect::from_x_y_ranges(origin.x + l..=origin.x + r, response.rect.y_range()),
                    0.0,
                    color(Color::Link(0), dark),
                );
            }
        }
        (Some(_), None) => ui.data_mut(|d| d.remove::<f64>(brush_id)),
        (None, _) if response.dragged() => {
            // Pan by whole categories; the remainder carries over so a slow
            // drag still moves.
            let mut acc: f32 = ui.data(|d| d.get_temp(pan_id)).unwrap_or(0.0);
            acc += response.drag_delta().x;
            if let Some(cw) = zoom::column_width(scene).filter(|&c| c > 0.0) {
                let cols = (acc as f64 / cw).trunc();
                if cols != 0.0 {
                    next = next.pan(-cols as i64);
                    acc -= (cols * cw) as f32;
                }
            }
            ui.data_mut(|d| d.insert_temp(pan_id, acc));
        }
        _ => {}
    }
    if response.drag_stopped() {
        ui.data_mut(|d| d.remove::<f32>(pan_id));
    }
    if response.double_clicked() && !on_legend {
        next = Window::full(w.len);
    }
    if next != w {
        ui.data_mut(|d| d.insert_temp(id, next));
        ui.ctx().request_repaint();
    }
}

/// Where a continuous zoom keeps its state, and how it draws.
struct Zoom {
    id: ::egui::Id,
    dark: bool,
    on_legend: bool,
}

/// Wheel, drag, shift-drag and double click move a continuous view, kept in
/// egui's memory for the next frame. Pointer positions map to data through
/// `plot`, the plot on screen; `full` is the whole chart's, for a reset.
/// Unlike a category window, a view moves by any amount, so wheel and drag
/// carry no remainders.
fn view_gestures(
    ui: &Ui,
    response: &Response,
    painter: &Painter,
    plot: &Plot,
    v: View,
    full: &Plot,
    zoom: Zoom,
) {
    let origin = response.rect.min;
    let at = |p: Pos2| ((p.x - origin.x) as f64, (p.y - origin.y) as f64);
    let brush_id = zoom.id.with("brush");
    let mut next = v;
    if response.hovered()
        && let Some(p) = response.hover_pos()
    {
        let notches = wheel_notches(ui);
        if notches != 0.0 {
            next = next.wheel(plot, at(p), notches);
        }
    }
    // As with categories, the brush starts where the button went down.
    if response.drag_started()
        && ui.input(|i| i.modifiers.shift)
        && let Some(p) = ui.input(|i| i.pointer.press_origin())
    {
        ui.data_mut(|d| d.insert_temp(brush_id, at(p)));
    }
    let brush: Option<(f64, f64)> = ui.data(|d| d.get_temp(brush_id));
    match (brush, response.interact_pointer_pos().map(at)) {
        (Some(from), Some(to)) => {
            if response.drag_stopped() {
                next = next.brush(plot, from, to);
                ui.data_mut(|d| d.remove::<(f64, f64)>(brush_id));
            } else {
                let r = v.brush_rect(plot, from, to);
                painter.rect_filled(
                    ERect::from_min_size(
                        origin + vec2(r.x as f32, r.y as f32),
                        vec2(r.w as f32, r.h as f32),
                    ),
                    0.0,
                    color(Color::Link(0), zoom.dark),
                );
            }
        }
        (Some(_), None) => ui.data_mut(|d| d.remove::<(f64, f64)>(brush_id)),
        (None, _) if response.dragged() => {
            let d = response.drag_delta();
            next = next.pan(plot, (d.x as f64, d.y as f64));
        }
        _ => {}
    }
    if response.double_clicked() && !zoom.on_legend {
        next = View::full(full);
    }
    if next != v {
        ui.data_mut(|d| d.insert_temp(zoom.id, next));
        ui.ctx().request_repaint();
    }
}

fn kind(scene: &Scene, i: usize) -> Option<HitKind> {
    scene.marks[i].hit.as_ref().map(|h| h.kind)
}

/// Values lead, labels follow, then any rows. A crosshair lists every visible
/// series, each keyed by a short stroke of its color.
fn tooltip(ui: &mut Ui, hit: &Hit, hidden: &[String], dark: bool) {
    if !hit.value.is_empty() {
        ui.label(RichText::new(&hit.value).strong());
    }
    ui.label(RichText::new(&hit.label).weak());
    for r in hit.rows.iter().filter(|r| !hidden.contains(&r.name)) {
        ui.horizontal(|ui| {
            if let Some(c) = r.color {
                let (key, _) = ui.allocate_exact_size(vec2(12.0, 2.0), Sense::hover());
                ui.painter().rect_filled(key, 1.0, color(c, dark));
            }
            ui.label(RichText::new(&r.value).strong());
            ui.label(RichText::new(&r.name).weak());
        });
    }
}

fn color(c: Color, dark: bool) -> Color32 {
    let v = resolve(c, dark);
    Color32::from_rgba_unmultiplied(v.0, v.1, v.2, v.3)
}

fn styled(c: Color, dark: bool, e: Emphasis) -> Color32 {
    let base = color(c, dark);
    match e {
        Emphasis::Lift => base.lerp_to_gamma(Color32::WHITE, LIFT),
        Emphasis::Dim => base.gamma_multiply(DIM),
        Emphasis::Normal | Emphasis::Hidden => base,
    }
}

fn paint(painter: &Painter, origin: Pos2, m: &Mark, dark: bool, e: Emphasis) {
    let at = |x: f64, y: f64| pos2(origin.x + x as f32, origin.y + y as f32);
    let stroke = m
        .stroke
        .map(|s| EStroke::new(s.width as f32, styled(s.color, dark, e)));
    match &m.shape {
        Shape::Rect { rect, radius } => {
            if m.fill.is_none() && stroke.is_none() {
                return;
            }
            let r = |v: f64| v.round().clamp(0.0, 255.0) as u8;
            let corners = CornerRadius {
                nw: r(radius.tl),
                ne: r(radius.tr),
                sw: r(radius.bl),
                se: r(radius.br),
            };
            let er = ERect::from_min_size(at(rect.x, rect.y), vec2(rect.w as f32, rect.h as f32));
            // Centered on the edge, as SVG strokes are.
            painter.add(RectShape::new(
                er,
                corners,
                m.fill.map_or(Color32::TRANSPARENT, |f| styled(f, dark, e)),
                stroke.unwrap_or(EStroke::NONE),
                StrokeKind::Middle,
            ));
        }
        Shape::Line { x1, y1, x2, y2 } => {
            if let Some(s) = stroke {
                painter.line_segment([at(*x1, *y1), at(*x2, *y2)], s);
            }
        }
        Shape::Polyline { points } => {
            if let Some(s) = stroke {
                let pts = points.iter().map(|&(x, y)| at(x, y)).collect();
                painter.add(PathShape::line(pts, PathStroke::from(s)));
            }
        }
        Shape::Area { points } => {
            // egui only fills convex polygons, so tessellate the band into
            // one triangle strip: no seams between slices.
            let Some(fill) = m.fill else { return };
            let c = styled(fill, dark, e);
            let mut mesh = Mesh::default();
            for (k, &(x, t, b)) in points.iter().enumerate() {
                mesh.colored_vertex(at(x, t), c);
                mesh.colored_vertex(at(x, b), c);
                if k > 0 {
                    let i = (2 * k) as u32;
                    mesh.add_triangle(i - 2, i - 1, i);
                    mesh.add_triangle(i - 1, i + 1, i);
                }
            }
            painter.add(EShape::mesh(mesh));
        }
        Shape::Path { ops } => {
            if let Some(fill) = m.fill {
                painter.add(EShape::mesh(fill_mesh(ops, origin, styled(fill, dark, e))));
            }
            if let Some(s) = stroke {
                for sub in flatten(ops) {
                    let pts = sub.iter().map(|&(x, y)| at(x, y)).collect();
                    painter.add(PathShape::line(pts, PathStroke::from(s)));
                }
            }
        }
        Shape::Circle { cx, cy, r } => {
            let fill = m.fill.map_or(Color32::TRANSPARENT, |f| styled(f, dark, e));
            painter.circle(
                at(*cx, *cy),
                *r as f32,
                fill,
                stroke.unwrap_or(EStroke::NONE),
            );
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
            let c = styled(m.fill.unwrap_or(Color::TextPrimary), dark, e);
            let family = families(painter.ctx())[*weight as usize].clone();
            let galley = painter.layout_no_wrap(text.clone(), FontId::new(*size as f32, family), c);
            let local = anchor_in_galley(&galley, *size, *anchor, *baseline);
            // TextShape rotates around its top-left corner, so offset the
            // corner such that the anchor point lands on (x, y) after rotation.
            let rot = (*angle as f32).to_radians();
            let pos = at(*x, *y) - Rot2::from_angle(rot) * local;
            painter.add(TextShape::new(pos, galley, c).with_angle(rot));
        }
    }
}

/// Tessellates a path's nonzero fill, concave parts and holes included.
fn fill_mesh(ops: &[PathOp], origin: Pos2, color: Color32) -> Mesh {
    use lyon_tessellation::VertexBuffers;
    use lyon_tessellation::geom::point;
    use lyon_tessellation::path::Path as LPath;
    use lyon_tessellation::{BuffersBuilder, FillOptions, FillRule, FillTessellator, FillVertex};

    let p = |x: f64, y: f64| point(x as f32, y as f32);
    let mut b = LPath::builder();
    let mut open = false;
    for op in ops {
        match *op {
            PathOp::Move(x, y) => {
                if open {
                    b.end(false);
                }
                b.begin(p(x, y));
                open = true;
            }
            PathOp::Line(x, y) => {
                b.line_to(p(x, y));
            }
            PathOp::Cubic(x1, y1, x2, y2, x, y) => {
                b.cubic_bezier_to(p(x1, y1), p(x2, y2), p(x, y));
            }
            PathOp::Close => {
                if open {
                    b.end(true);
                    open = false;
                }
            }
        }
    }
    if open {
        b.end(false);
    }
    let path = b.build();
    let mut buf: VertexBuffers<Pos2, u32> = VertexBuffers::new();
    let opts = FillOptions::default().with_fill_rule(FillRule::NonZero);
    let mut mesh = Mesh::default();
    if FillTessellator::new()
        .tessellate_path(
            &path,
            &opts,
            &mut BuffersBuilder::new(&mut buf, |v: FillVertex| {
                pos2(origin.x + v.position().x, origin.y + v.position().y)
            }),
        )
        .is_ok()
    {
        for v in buf.vertices {
            mesh.colored_vertex(v, color);
        }
        mesh.indices = buf.indices;
    }
    mesh
}

/// The scene's anchor point, in the galley's own coordinates.
fn anchor_in_galley(galley: &Arc<Galley>, size: f64, anchor: Anchor, baseline: Baseline) -> Vec2 {
    let w = galley.size().x;
    let ax = match anchor {
        Anchor::Start => 0.0,
        Anchor::Middle => w / 2.0,
        Anchor::End => w,
    };
    // Distance from the galley top to the alphabetic baseline.
    let ascent = galley
        .rows
        .first()
        .and_then(|row| row.glyphs.first().map(|g| row.pos.y + g.pos.y))
        .unwrap_or(size as f32 * 0.8);
    let cap = (CAP_HEIGHT * size) as f32;
    let ay = match baseline {
        Baseline::Top => ascent - cap,
        Baseline::Middle => ascent - cap / 2.0,
        Baseline::Bottom => ascent,
    };
    vec2(ax, ay)
}
