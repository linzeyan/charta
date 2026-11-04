//! Slint backend (feature `slint`): rasterizes a scene with tiny-skia into a
//! [`slint::Image`]. Slint has no general path drawing API, so the chart is a
//! bitmap that is redrawn on hover; the tooltip is drawn into the same bitmap.
//!
//! Slint is licensed GPL-3.0-only, royalty-free or commercially; an app that
//! enables this feature is bound by one of those licenses.
//!
//! Wire it to an `Image` and a `TouchArea` in your `.slint` file:
//!
//! ```no_run
//! use charta::BarChart;
//! use charta::backend::slint::SlintChart;
//!
//! let mut chart = SlintChart::new(Box::new(BarChart::new([("A", 30.0), ("B", 80.0)])));
//! // pointer-event move:  if chart.pointer_moved(x, y) { redraw }
//! // pointer-event down:  chart.pressed(x, y, e.modifiers.shift)
//! // pointer-event up:    if chart.released(x, y) { redraw }
//! // pointer-event cancel: if chart.cancelled() { redraw }
//! // `clicked`:           if chart.clicked(x, y) { redraw }
//! // `double-clicked`:    if chart.double_clicked(x, y) { redraw }
//! // `scroll-event`:      reject unless chart.zooms();
//! //                      if chart.scrolled(x, y, e.delta-y) { redraw }
//! // hover end:           if chart.pointer_exited() { redraw }
//! // redraw: image.set_source(chart.render(width, height, scale_factor, dark))
//! let image: slint::Image = chart.render(640.0, 400.0, 2.0, false);
//! ```
//!
//! A chart with a category axis ([`Chart::category_count`]) or a continuous
//! one ([`Chart::plot_scene`]) zooms: the wheel zooms around the pointer,
//! dragging pans, shift-dragging brushes a range (a box, on a chart that
//! zooms y too) to zoom to, and a double click shows everything again.

use ::slint::{Image, Rgba8Pixel, SharedPixelBuffer};
use tiny_skia::Pixmap;

use super::gestures::{Change, Gestures};
use super::raster::{SystemMeasure, error, paint};
use crate::Chart;
use crate::base::theme::Color;
use crate::scene::{HitKind, Mark, Rect, Scene, Shape};

/// One wheel notch of scroll distance (logical px): Slint's winit backend
/// reports a line as 60.
const WHEEL_NOTCH: f64 = 60.0;

/// A chart plus the interaction state Slint does not keep for us: pointer
/// position, hidden series and zoom. Coordinates are logical pixels.
pub struct SlintChart {
    chart: Box<dyn Chart>,
    scene: Option<Scene>,
    pointer: Option<(f64, f64)>,
    hidden: Vec<String>,
    gestures: Gestures,
    /// The zoom moved since the scene was laid out.
    stale: bool,
}

impl SlintChart {
    pub fn new(chart: Box<dyn Chart>) -> Self {
        SlintChart {
            chart,
            scene: None,
            pointer: None,
            hidden: Vec::new(),
            gestures: Gestures::new(),
            stale: false,
        }
    }

    /// Whether the wheel and drags zoom the chart, as of the last render.
    /// Reject scroll events otherwise, so an enclosing Flickable scrolls.
    pub fn zooms(&self) -> bool {
        self.gestures.zooms()
    }

    /// Series hidden through the legend.
    pub fn hidden(&self) -> &[String] {
        &self.hidden
    }

    /// Records the pointer, panning or brushing while the button is down;
    /// true when the image must be redrawn (the hovered mark changed, a
    /// tooltip follows the pointer, or the zoom moved).
    pub fn pointer_moved(&mut self, x: f32, y: f32) -> bool {
        let before = self.hovered();
        let p = (x as f64, y as f64);
        self.pointer = Some(p);
        let after = self.hovered();
        let hover = before != after || after.is_some_and(|i| self.kind(i) != Some(HitKind::Legend));
        self.gesture(|g, s| g.moved(s, p)) || hover
    }

    /// The primary button went down; `shift` makes a drag from here a
    /// brush.
    pub fn pressed(&mut self, x: f32, y: f32, shift: bool) {
        let at = (x as f64, y as f64);
        self.pointer = Some(at);
        self.gestures.pressed(at, shift);
    }

    /// The primary button came up: a brush zooms to what it covered. True
    /// when the image must be redrawn.
    pub fn released(&mut self, x: f32, y: f32) -> bool {
        let to = (x as f64, y as f64);
        self.pointer = Some(to);
        self.gesture(|g, s| g.released(s, to))
    }

    /// The press ended without a release (the pointer was grabbed away):
    /// a drag stops where it is, and a brush is dropped. True when one was
    /// shading.
    pub fn cancelled(&mut self) -> bool {
        let c = self.gestures.cancelled();
        self.changed(c)
    }

    /// The wheel turned by `delta_y` (logical px, Slint's scroll-event
    /// delta-y; up is positive and zooms in) with the pointer at (x, y).
    pub fn scrolled(&mut self, x: f32, y: f32, delta_y: f32) -> bool {
        let at = (x as f64, y as f64);
        self.gesture(|g, s| g.wheel(s, at, delta_y as f64 / WHEEL_NOTCH))
    }

    /// Shows everything again, unless the double click was on a legend
    /// entry: that toggled its series twice.
    pub fn double_clicked(&mut self, x: f32, y: f32) -> bool {
        self.pointer = Some((x as f64, y as f64));
        if self
            .hovered()
            .is_some_and(|i| self.kind(i) == Some(HitKind::Legend))
        {
            return false;
        }
        let c = self.gestures.reset();
        self.changed(c)
    }

    /// Runs a gesture against the scene on screen; true when it calls for
    /// a redraw.
    fn gesture(&mut self, f: impl FnOnce(&mut Gestures, &Scene) -> Change) -> bool {
        let Some(scene) = &self.scene else {
            return false;
        };
        let c = f(&mut self.gestures, scene);
        self.changed(c)
    }

    fn changed(&mut self, c: Change) -> bool {
        self.stale |= c == Change::Zoom;
        c != Change::None
    }

    fn brush(&self) -> Option<Rect> {
        self.gestures.brush(self.scene.as_ref()?)
    }

    /// Clears the pointer; true when something was highlighted.
    pub fn pointer_exited(&mut self) -> bool {
        let was = self.hovered().is_some();
        self.pointer = None;
        was
    }

    /// Toggles the series of a clicked legend entry; true when it did. The
    /// click Slint reports at the end of a drag is ignored.
    pub fn clicked(&mut self, x: f32, y: f32) -> bool {
        self.pointer = Some((x as f64, y as f64));
        if self.gestures.dragged() {
            return false;
        }
        let Some(i) = self.hovered() else {
            return false;
        };
        if self.kind(i) != Some(HitKind::Legend) {
            return false;
        }
        let Some(s) = self
            .scene
            .as_ref()
            .and_then(|sc| sc.marks[i].series.clone())
        else {
            return false;
        };
        match self.hidden.iter().position(|h| *h == s) {
            Some(k) => {
                self.hidden.remove(k);
            }
            None => self.hidden.push(s),
        }
        true
    }

    /// Lays out (when the size or zoom changed) and paints the chart at
    /// `scale` physical pixels per logical pixel, with a brush under way
    /// shaded.
    pub fn render(&mut self, width: f32, height: f32, scale: f32, dark: bool) -> Image {
        let (w, h) = (width as f64, height as f64);
        let stale = self.stale
            || self
                .scene
                .as_ref()
                .is_none_or(|s| s.width != w || s.height != h);
        self.stale = false;
        let pixmap = if stale {
            match self
                .gestures
                .lay_out(self.chart.as_ref(), w, h, &SystemMeasure)
            {
                Ok(s) => {
                    self.scene = Some(s);
                    None
                }
                Err(e) => {
                    self.scene = None;
                    Some(error(&e.to_string(), w, h, scale, dark))
                }
            }
        } else {
            None
        };
        let pixmap = pixmap.unwrap_or_else(|| {
            let mut scene = self.scene.clone().expect("scene laid out above");
            if let Some(rect) = self.brush() {
                scene.marks.push(
                    Mark::new(Shape::Rect {
                        rect,
                        radius: Default::default(),
                    })
                    .fill(Color::Link(0)),
                );
            }
            paint(
                &scene,
                self.hovered(),
                self.pointer,
                &self.hidden,
                scale,
                dark,
            )
        });
        image(&pixmap)
    }

    fn hovered(&self) -> Option<usize> {
        let (x, y) = self.pointer?;
        self.scene.as_ref()?.hit_test(x, y, &self.hidden)
    }

    fn kind(&self, i: usize) -> Option<HitKind> {
        self.scene.as_ref()?.marks[i].hit.as_ref().map(|h| h.kind)
    }
}

fn image(p: &Pixmap) -> Image {
    // tiny-skia stores premultiplied RGBA, exactly what Slint accepts.
    Image::from_rgba8_premultiplied(SharedPixelBuffer::<Rgba8Pixel>::clone_from_slice(
        p.data(),
        p.width(),
        p.height(),
    ))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::backend::gestures::Zoom;
    use crate::backend::raster::TIP_OFFSET;
    use crate::base::text::{TextMeasure, Weight};
    use crate::base::theme::{Color, resolve};
    use crate::scene::{Mark, Shape};
    use crate::zoom::{self, View, Window};
    use crate::{BarChart, LineChart, Point, ScatterChart};

    fn px(p: &Pixmap, x: f64, y: f64, scale: f32) -> [u8; 4] {
        let c = p
            .pixel((x * scale as f64) as u32, (y * scale as f64) as u32)
            .unwrap();
        [c.red(), c.green(), c.blue(), c.alpha()]
    }

    fn rgb(c: Color, dark: bool) -> [u8; 3] {
        let v = resolve(c, dark);
        [v.0, v.1, v.2]
    }

    fn near(a: [u8; 4], b: [u8; 3]) -> bool {
        a.iter().zip(b).all(|(&x, y)| x.abs_diff(y) <= 2)
    }

    fn snapshot(p: &Pixmap, name: &str) {
        let out = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("build/snapshots");
        std::fs::create_dir_all(&out).unwrap();
        p.save_png(out.join(format!("slint-{name}.png"))).unwrap();
    }

    fn signups() -> LineChart {
        LineChart::new(["Jan", "Feb", "Mar", "Apr"])
            .title("Signups")
            .series("Web", [120.0, 135.0, 128.0, 160.0])
            .series("iOS", [80.0, 95.0, 110.0, 108.0])
    }

    /// The point a hit's area is centered on.
    fn aim(scene: &Scene, pick: impl Fn(&Mark) -> bool) -> (f64, f64) {
        let a = scene
            .marks
            .iter()
            .find(|m| pick(m))
            .and_then(|m| m.hit.as_ref()?.area)
            .expect("mark with a hit area");
        (a.x + a.w / 2.0, a.y + a.h / 2.0)
    }

    /// The image's pixel at (x, y), rendered at scale 1.
    fn at(img: &Image, x: f64, y: f64) -> [u8; 3] {
        let buf = img.to_rgba8().expect("a pixel buffer");
        let p = buf.as_slice()[y as usize * buf.width() as usize + x as usize];
        [p.r, p.g, p.b]
    }

    fn months() -> LineChart {
        LineChart::new([
            "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
        ])
        .series("Visits", (1..=12).map(|i| i as f64 * 10.0))
        .series("Signups", (1..=12).map(|i| i as f64 * 4.0))
    }

    /// Presses at `from`, moves to `to` in ten steps and releases there;
    /// `mid` sees the image halfway.
    fn drag(
        c: &mut SlintChart,
        from: (f64, f64),
        to: (f64, f64),
        shift: bool,
        mid: impl Fn(Image),
    ) {
        c.pressed(from.0 as f32, from.1 as f32, shift);
        for k in 1..=10 {
            let t = k as f64 / 10.0;
            let p = (from.0 + (to.0 - from.0) * t, from.1 + (to.1 - from.1) * t);
            c.pointer_moved(p.0 as f32, p.1 as f32);
            let img = c.render(640.0, 400.0, 1.0, false);
            if k == 5 {
                mid(img);
            }
        }
        c.released(to.0 as f32, to.1 as f32);
        c.render(640.0, 400.0, 1.0, false);
    }

    #[test]
    fn a_category_axis_zooms_pans_brushes_and_resets() {
        let mut c = SlintChart::new(Box::new(months()));
        c.render(640.0, 400.0, 1.0, false);
        let window = |c: &SlintChart| match c.gestures.zoom {
            Zoom::Categories(w) => w,
            z => panic!("{z:?}"),
        };
        let mid = (320.0, 200.0);
        let col = zoom::column_at(c.scene.as_ref().unwrap(), mid.0).unwrap();

        // Half a notch is not a step yet; the second half is.
        assert!(!c.scrolled(320.0, 200.0, 30.0));
        assert!(c.scrolled(320.0, 200.0, 30.0));
        assert_eq!(window(&c), Window::full(12).wheel(col, 1.0));
        c.render(640.0, 400.0, 1.0, false);
        let zoomed = window(&c);
        let columns = c
            .scene
            .as_ref()
            .unwrap()
            .marks
            .iter()
            .filter(|m| m.hit.as_ref().is_some_and(|h| h.kind == HitKind::Crosshair))
            .count();
        assert_eq!(
            columns,
            zoomed.end - zoomed.start,
            "the zoom was not laid out"
        );

        // Dragging left by a column and a bit brings one later month in.
        let cw = zoom::column_width(c.scene.as_ref().unwrap()).unwrap();
        drag(&mut c, mid, (mid.0 - cw - 4.0, mid.1), false, |_| {});
        assert_eq!(window(&c), zoomed.pan(1));
        assert_eq!(c.gestures.pan, 0.0, "the remainder outlived the drag");

        assert!(c.double_clicked(320.0, 200.0));
        c.render(640.0, 400.0, 1.0, false);
        assert!(window(&c).is_full());

        // Shift-dragging from column 2.9 to 6.5 zooms to months 2..=6,
        // shading the columns covered so far, and only those, halfway.
        let scene = c.scene.clone().unwrap();
        let x = |col: f64| zoom::column_x(&scene, col).unwrap();
        let before = c.render(640.0, 400.0, 1.0, false);
        let top = 1.0;
        drag(&mut c, (x(2.9), mid.1), (x(6.5), mid.1), true, |img| {
            assert_ne!(
                at(&img, x(3.5), top),
                at(&before, x(3.5), top),
                "no shading"
            );
            assert_eq!(
                at(&img, x(6.5), top),
                at(&before, x(6.5), top),
                "shading ahead"
            );
        });
        assert_eq!(window(&c), Window::full(12).brush(2, 6));
        assert!(c.brush().is_none(), "the brush outlived the drag");

        // A drag that ends on a legend entry is no click on it.
        let legend = aim(c.scene.as_ref().unwrap(), |m| {
            m.hit.as_ref().is_some_and(|h| h.kind == HitKind::Legend)
        });
        drag(&mut c, (legend.0, legend.1 + 40.0), legend, false, |_| {});
        assert!(!c.clicked(legend.0 as f32, legend.1 as f32));
        assert!(c.hidden().is_empty());
        // A double click there toggles twice and leaves the zoom. Slint
        // reports each click's press and release first.
        let kept = window(&c);
        let (lx, ly) = (legend.0 as f32, legend.1 as f32);
        for _ in 0..2 {
            c.pressed(lx, ly, false);
            c.released(lx, ly);
            assert!(c.clicked(lx, ly));
        }
        assert!(!c.double_clicked(lx, ly));
        assert_eq!(window(&c), kept);
        // A click may jitter a little and stay a click.
        c.pressed(lx, ly, false);
        c.pointer_moved(lx + 3.0, ly);
        c.released(lx + 3.0, ly);
        assert!(c.clicked(lx + 3.0, ly));
        assert_eq!(c.hidden().len(), 1);
        assert_eq!(window(&c), kept);
    }

    #[test]
    fn a_scatterplot_zooms_both_axes_by_wheel_drag_and_box() {
        let points = (0..60)
            .map(|i| Point::new(i as f64, ((i * 37) % 100) as f64))
            .collect();
        let mut c = SlintChart::new(Box::new(ScatterChart::new().series("Trials", points)));
        assert!(!c.zooms(), "zooms before it is laid out");
        c.render(640.0, 400.0, 1.0, false);
        assert!(c.zooms());
        let Zoom::Range { full, .. } = c.gestures.zoom else {
            panic!("{:?}", c.gestures.zoom)
        };
        let view = |c: &SlintChart| match c.gestures.zoom {
            Zoom::Range { view, .. } => view,
            z => panic!("{z:?}"),
        };
        // On the f32 grid the pointer arrives on.
        let across = |tx: f64, ty: f64| {
            let r = full.rect;
            (
                (r.x + tx * r.w) as f32 as f64,
                (r.y + ty * r.h) as f32 as f64,
            )
        };

        let p = across(1.0 / 3.0, 1.0 / 3.0);
        assert!(c.scrolled(p.0 as f32, p.1 as f32, 60.0));
        assert_eq!(view(&c), View::full(&full).wheel(&full, p, 1.0));
        c.render(640.0, 400.0, 1.0, false);
        let zoomed = view(&c);

        // Dragging left and down: later x and higher y, the spans kept.
        let mid = across(0.5, 0.5);
        drag(&mut c, mid, (mid.0 - 120.0, mid.1 + 60.0), false, |_| {});
        let panned = view(&c);
        let (zy, py) = (zoomed.y.unwrap(), panned.y.unwrap());
        assert!(panned.x.lo > zoomed.x.lo && py.lo > zy.lo, "{panned:?}");
        assert!(((panned.x.hi - panned.x.lo) - (zoomed.x.hi - zoomed.x.lo)).abs() < 1e-9);

        assert!(c.double_clicked(mid.0 as f32, mid.1 as f32));
        c.render(640.0, 400.0, 1.0, false);
        assert!(view(&c).is_full());

        // The brush is a box: halfway it shades from where the button went
        // down to the pointer, and nothing below or right of that.
        let (from, to) = (across(0.25, 0.25), across(0.6, 0.7));
        let before = c.render(640.0, 400.0, 1.0, false);
        drag(&mut c, from, to, true, |img| {
            let changed = |(x, y): (f64, f64)| at(&img, x, y) != at(&before, x, y);
            assert!(changed(across(0.3, 0.32)), "no shading inside the box");
            assert!(!changed(across(0.3, 0.9)), "shading below the box");
            assert!(!changed(across(0.9, 0.32)), "shading right of the box");
        });
        assert_eq!(view(&c), View::full(&full).brush(&full, from, to));
    }

    #[test]
    fn a_view_that_no_longer_fits_starts_over() {
        // At another size the axes nice to other ends: a view of the old
        // extent would show a range the new layout does not span.
        let points = (0..60)
            .map(|i| Point::new(i as f64 * 1.7, i as f64))
            .collect();
        let mut c = SlintChart::new(Box::new(ScatterChart::new().series("a", points)));
        c.render(640.0, 400.0, 1.0, false);
        assert!(c.scrolled(320.0, 200.0, 120.0));
        c.render(640.0, 400.0, 1.0, false);
        let Zoom::Range { full: before, .. } = c.gestures.zoom else {
            panic!()
        };
        c.render(160.0, 120.0, 1.0, false);
        let Zoom::Range { view, full, .. } = c.gestures.zoom else {
            panic!()
        };
        assert_ne!(
            (full.x, full.y),
            (before.x, before.y),
            "the sizes nice alike"
        );
        assert_eq!(view, View::full(&full));
    }

    #[test]
    fn rect_outlines_are_drawn() {
        // Treemap cells and mind-map pills are outlined rects: an outline
        // dropped by the rasterizer leaves them unbounded.
        let scene = Scene {
            width: 40.0,
            height: 40.0,
            label: String::new(),
            marks: vec![
                Mark::new(Shape::Rect {
                    rect: crate::scene::Rect {
                        x: 10.0,
                        y: 10.0,
                        w: 20.0,
                        h: 20.0,
                    },
                    radius: Default::default(),
                })
                .stroke(Color::Series(1), 2.0),
            ],
        };
        let p = paint(&scene, None, None, &[], 1.0, false);
        assert!(near(px(&p, 10.0, 20.0, 1.0), rgb(Color::Series(1), false)));
        assert!(!near(px(&p, 20.0, 20.0, 1.0), rgb(Color::Series(1), false)));
    }

    #[test]
    fn system_fonts_measure_real_glyphs() {
        // Proportional widths prove glyphs came from a real face rather than
        // the gap left for missing characters.
        let m = SystemMeasure;
        let (i, w) = (
            m.width("i", 12.0, Weight::Regular),
            m.width("W", 12.0, Weight::Regular),
        );
        assert!(w > i * 1.5, "i={i} W={w}");
        assert!(m.width("台北", 12.0, Weight::Regular) > 0.0);
    }

    #[test]
    fn hover_lifts_the_bar_and_draws_its_tooltip() {
        let mut c = SlintChart::new(Box::new(
            BarChart::new([("Apples", 42.0), ("Pears", 18.0)]).title("Fruit"),
        ));
        c.render(640.0, 400.0, 1.0, false);
        let scene = c.scene.clone().unwrap();
        let bar = scene
            .marks
            .iter()
            .find_map(|m| match (&m.shape, &m.hit) {
                (Shape::Rect { rect, .. }, Some(h)) if h.label == "Pears" => Some(*rect),
                _ => None,
            })
            .unwrap();
        let (bx, by) = (bar.x + bar.w / 2.0, bar.y + bar.h - 2.0);
        let idle = paint(&scene, None, None, &[], 1.0, false);
        assert!(near(px(&idle, bx, by, 1.0), rgb(Color::Series(0), false)));

        assert!(c.pointer_moved(bx as f32, (bar.y + 4.0) as f32));
        let i = c.hovered();
        let hovered = paint(&scene, i, c.pointer, &[], 1.0, false);
        assert!(
            !near(px(&hovered, bx, by, 1.0), rgb(Color::Series(0), false)),
            "hovered bar is not lifted"
        );
        // The tooltip sits below-right of the pointer, clear of the bar; its
        // fill is the surface, so look for its border and text instead.
        let (tx, ty) = (bx + TIP_OFFSET, bar.y + 4.0 + TIP_OFFSET);
        let changed = (0..60)
            .flat_map(|dx| (0..40).map(move |dy| (tx + dx as f64, ty + dy as f64)))
            .filter(|&(x, y)| px(&idle, x, y, 1.0) != px(&hovered, x, y, 1.0))
            .count();
        assert!(changed > 50, "no tooltip drawn ({changed} pixels changed)");
        snapshot(&hovered, "bar-hover");
        assert!(c.pointer_exited());
    }

    #[test]
    fn legend_click_hides_the_series_and_drops_it_from_the_tooltip() {
        let mut c = SlintChart::new(Box::new(signups()));
        c.render(640.0, 400.0, 2.0, true);
        let scene = c.scene.clone().unwrap();
        let ios = aim(&scene, |m| {
            m.series.as_deref() == Some("iOS")
                && m.hit.as_ref().is_some_and(|h| h.kind == HitKind::Legend)
        });
        // A point on the iOS line, away from the Web line.
        let on_line = scene
            .marks
            .iter()
            .find_map(|m| match &m.shape {
                Shape::Polyline { points } if m.series.as_deref() == Some("iOS") => {
                    let (a, b) = (points[0], points[1]);
                    Some(((a.0 + b.0) / 2.0, (a.1 + b.1) / 2.0))
                }
                _ => None,
            })
            .unwrap();
        let before = paint(&scene, None, None, &[], 2.0, true);
        assert!(!near(
            px(&before, on_line.0, on_line.1, 2.0),
            rgb(Color::Surface, true)
        ));

        assert!(c.clicked(ios.0 as f32, ios.1 as f32));
        assert_eq!(c.hidden(), ["iOS"]);
        c.pointer_exited();
        let after = paint(&scene, None, None, c.hidden(), 2.0, true);
        let at = px(&after, on_line.0, on_line.1, 2.0);
        assert!(
            near(at, rgb(Color::Surface, true)),
            "hidden line still painted: {at:?}"
        );

        // Hovering a column: the crosshair tooltip lists only Web.
        let apr = aim(&scene, |m| {
            m.hit
                .as_ref()
                .is_some_and(|h| h.kind == HitKind::Crosshair && h.label == "Apr")
        });
        assert!(c.pointer_moved(apr.0 as f32, apr.1 as f32));
        let i = c.hovered().unwrap();
        let hit = scene.marks[i].hit.as_ref().unwrap();
        let rows = hit.rows.iter().filter(|r| !c.hidden().contains(&r.name));
        assert_eq!(rows.map(|r| r.name.as_str()).collect::<Vec<_>>(), ["Web"]);
        snapshot(
            &paint(&scene, Some(i), c.pointer, c.hidden(), 2.0, true),
            "line-hidden-dark",
        );

        assert!(c.clicked(ios.0 as f32, ios.1 as f32));
        assert!(c.hidden().is_empty());
    }

    #[test]
    fn render_returns_a_physical_size_image() {
        let mut c = SlintChart::new(Box::new(signups()));
        let img = c.render(320.0, 200.0, 2.0, false);
        assert_eq!((img.size().width, img.size().height), (640, 400));
    }
}
