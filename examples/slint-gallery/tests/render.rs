//! Headless check of the Slint gallery's wiring, on Slint's software
//! renderer (no window, no GPU): the chart image appears, hovering a bar
//! redraws it with a tooltip, clicking a legend entry hides that series, the
//! wheel, double click and shift-drag zoom, and every demo paints.
//! Writes `build/snapshots/slint-gallery-*.png` to eyeball.
//!
//! One test function: Slint's platform is per process and its UI is
//! single-threaded, so the steps cannot be split across test threads.

use std::path::PathBuf;
use std::rc::Rc;

use charta::backend::raster::SystemMeasure;
use charta::scene::{HitKind, Mark, Scene, Shape};
use charta_demos::demos;
use charta_slint_gallery::gallery;
use slint::platform::software_renderer::{
    MinimalSoftwareWindow, PremultipliedRgbaColor, RepaintBufferType,
};
use slint::platform::{Key, Platform, PointerEventButton, WindowAdapter, WindowEvent};
use slint::{ComponentHandle, LogicalPosition, PhysicalSize, PlatformError};

const WINDOW: (u32, u32) = (920, 540);
/// Where the gallery's layout puts the chart: padding, the 180px list and
/// spacing on the left, padding elsewhere.
const ORIGIN: (f32, f32) = (12.0 + 180.0 + 12.0, 12.0);
const CHART: (f64, f64) = ((920 - 204 - 12) as f64, (540 - 24) as f64);

struct Headless(Rc<MinimalSoftwareWindow>);

impl Platform for Headless {
    fn create_window_adapter(&self) -> Result<Rc<dyn WindowAdapter>, PlatformError> {
        Ok(self.0.clone())
    }
}

struct Frame(Vec<PremultipliedRgbaColor>);

impl Frame {
    fn at(&self, x: f64, y: f64) -> [u8; 3] {
        let (x, y) = (x as f32 + ORIGIN.0, y as f32 + ORIGIN.1);
        let p = self.0[y as usize * WINDOW.0 as usize + x as usize];
        [p.red, p.green, p.blue]
    }

    /// Whether the chart looks the same in both; the rest of the window
    /// may still be animating a button.
    fn same(&self, other: &Frame) -> bool {
        (0..CHART.1 as usize).all(|y| {
            (0..CHART.0 as usize)
                .all(|x| self.at(x as f64, y as f64) == other.at(x as f64, y as f64))
        })
    }

    fn save(&self, name: &str) {
        let mut pm = tiny_skia::Pixmap::new(WINDOW.0, WINDOW.1).unwrap();
        for (d, s) in pm.pixels_mut().iter_mut().zip(&self.0) {
            *d = tiny_skia::PremultipliedColorU8::from_rgba(s.red, s.green, s.blue, s.alpha)
                .unwrap();
        }
        let out = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../build/snapshots");
        std::fs::create_dir_all(&out).unwrap();
        pm.save_png(out.join(format!("slint-gallery-{name}.png")))
            .unwrap();
    }
}

/// Runs change handlers (`changed width => redraw()`), then paints.
fn frame(window: &MinimalSoftwareWindow) -> Frame {
    slint::platform::update_timers_and_animations();
    let mut buf = vec![PremultipliedRgbaColor::default(); (WINDOW.0 * WINDOW.1) as usize];
    window.request_redraw();
    window.draw_if_needed(|r| {
        r.render(&mut buf, WINDOW.0 as usize);
    });
    Frame(buf)
}

fn layout(i: usize) -> Scene {
    demos()[i]
        .1
        .scene(CHART.0, CHART.1, &SystemMeasure)
        .unwrap()
}

fn center(scene: &Scene, pick: impl Fn(&Mark) -> bool) -> LogicalPosition {
    let a = scene
        .marks
        .iter()
        .find(|m| pick(m))
        .and_then(|m| m.hit.as_ref()?.area)
        .expect("mark with a hit area");
    LogicalPosition::new(
        ORIGIN.0 + (a.x + a.w / 2.0) as f32,
        ORIGIN.1 + (a.y + a.h / 2.0) as f32,
    )
}

fn near(a: [u8; 3], b: [u8; 3]) -> bool {
    a.iter().zip(b).all(|(&x, y)| x.abs_diff(y) <= 3)
}

#[test]
fn gallery_draws_hovers_and_toggles() {
    let window = MinimalSoftwareWindow::new(RepaintBufferType::NewBuffer);
    slint::platform::set_platform(Box::new(Headless(window.clone()))).unwrap();
    window.set_size(PhysicalSize::new(WINDOW.0, WINDOW.1));
    let ui = gallery().unwrap();
    ui.show().unwrap();
    frame(&window); // first layout: sizes the TouchArea, fires redraw
    let bar = frame(&window);
    bar.save("bar");

    // The bar demo is showing: the Pears bar is painted in series-1.
    let scene = layout(0);
    let pears = scene
        .marks
        .iter()
        .find_map(|m| match (&m.shape, &m.hit) {
            (Shape::Rect { rect, .. }, Some(h)) if h.label == "Pears" => Some(*rect),
            _ => None,
        })
        .unwrap();
    let inside = (pears.x + pears.w / 2.0, pears.y + pears.h - 2.0);
    let series = bar.at(inside.0, inside.1);
    assert!(
        !near(series, bar.at(2.0, 2.0)),
        "no chart image in the gallery: {series:?}"
    );

    // Hover: the bar lifts.
    let over = center(&scene, |m| {
        m.hit.as_ref().is_some_and(|h| h.label == "Pears")
    });
    ui.window()
        .dispatch_event(WindowEvent::PointerMoved { position: over });
    let hovered = frame(&window);
    hovered.save("bar-hover");
    assert_ne!(
        hovered.at(inside.0, inside.1),
        series,
        "hover did not redraw the chart"
    );

    // Switch to the line demo, then click its iOS legend entry.
    let line = demos().iter().position(|(n, _)| *n == "Line").unwrap();
    ui.window().dispatch_event(WindowEvent::PointerExited);
    ui.set_pick(line as i32);
    ui.invoke_redraw();
    let scene = layout(line);
    let ios_line = scene
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
    let shown = frame(&window);
    let surface = shown.at(2.0, CHART.1 - 2.0);
    assert!(
        !near(shown.at(ios_line.0, ios_line.1), surface),
        "iOS line not painted before the toggle"
    );

    let legend = center(&scene, |m| {
        m.series.as_deref() == Some("iOS")
            && m.hit.as_ref().is_some_and(|h| h.kind == HitKind::Legend)
    });
    for event in [
        WindowEvent::PointerMoved { position: legend },
        WindowEvent::PointerPressed {
            position: legend,
            button: PointerEventButton::Left,
        },
        WindowEvent::PointerReleased {
            position: legend,
            button: PointerEventButton::Left,
        },
    ] {
        ui.window().dispatch_event(event);
    }
    let hidden = frame(&window);
    hidden.save("line-hidden");
    let at = hidden.at(ios_line.0, ios_line.1);
    assert!(
        near(at, surface),
        "iOS still painted after the legend click: {at:?}"
    );

    // The wheel zooms the line demo's months; a double click shows the
    // whole year again.
    let mid = LogicalPosition::new(
        ORIGIN.0 + CHART.0 as f32 / 2.0,
        ORIGIN.1 + CHART.1 as f32 / 2.0,
    );
    let send = |events: &[WindowEvent]| {
        for e in events {
            ui.window().dispatch_event(e.clone());
        }
    };
    let click = |at| {
        [
            WindowEvent::PointerPressed {
                position: at,
                button: PointerEventButton::Left,
            },
            WindowEvent::PointerReleased {
                position: at,
                button: PointerEventButton::Left,
            },
        ]
    };
    // Frames are compared with the pointer in the chart's bare corner, so
    // no tooltip differs.
    let corner = WindowEvent::PointerMoved {
        position: LogicalPosition::new(ORIGIN.0 + 2.0, ORIGIN.1 + 2.0),
    };
    send(std::slice::from_ref(&corner));
    let year = frame(&window);
    send(&[
        WindowEvent::PointerMoved { position: mid },
        WindowEvent::PointerScrolled {
            position: mid,
            delta_x: 0.0,
            delta_y: 120.0,
        },
        corner.clone(),
    ]);
    assert!(!frame(&window).same(&year), "the wheel did not zoom");
    send(&[&click(mid)[..], &click(mid), std::slice::from_ref(&corner)].concat());
    assert!(frame(&window).same(&year), "a double click did not reset");

    // On the scatterplot a plain drag over the whole view has nothing to
    // pan; with shift held it brushes a box and zooms to it.
    let scatter = demos().iter().position(|(n, _)| *n == "Scatter").unwrap();
    send(&[WindowEvent::PointerExited]);
    ui.set_pick(scatter as i32);
    ui.invoke_redraw();
    let drag = |events: &mut Vec<WindowEvent>| {
        let at =
            |t: f32| LogicalPosition::new(mid.x - 150.0 + 300.0 * t, mid.y - 100.0 + 200.0 * t);
        events.push(WindowEvent::PointerMoved { position: at(0.0) });
        events.push(WindowEvent::PointerPressed {
            position: at(0.0),
            button: PointerEventButton::Left,
        });
        events.extend((1..=10).map(|k| WindowEvent::PointerMoved {
            position: at(k as f32 / 10.0),
        }));
        events.push(WindowEvent::PointerReleased {
            position: at(1.0),
            button: PointerEventButton::Left,
        });
        events.push(corner.clone());
    };
    send(std::slice::from_ref(&corner));
    let whole = frame(&window);
    let mut plain = Vec::new();
    drag(&mut plain);
    send(&plain);
    assert!(
        frame(&window).same(&whole),
        "a drag over the whole view moved it"
    );
    let mut brush = vec![WindowEvent::KeyPressed {
        text: Key::Shift.into(),
    }];
    drag(&mut brush);
    brush.push(WindowEvent::KeyReleased {
        text: Key::Shift.into(),
    });
    send(&brush);
    let boxed = frame(&window);
    boxed.save("scatter-zoomed");
    assert!(!boxed.same(&whole), "a shift-drag did not zoom");

    // Every demo paints something besides the surface (a shape the
    // rasterizer silently drops would leave its chart blank).
    ui.window().dispatch_event(WindowEvent::PointerExited);
    for (i, (name, _)) in demos().iter().enumerate() {
        ui.set_pick(i as i32);
        ui.invoke_redraw();
        let f = frame(&window);
        let surface = f.at(2.0, 2.0);
        let inked = (0..CHART.1 as usize)
            .step_by(4)
            .flat_map(|y| (0..CHART.0 as usize).step_by(4).map(move |x| (x, y)))
            .filter(|&(x, y)| !near(f.at(x as f64, y as f64), surface))
            .count();
        assert!(inked > 40, "{name}: only {inked} inked samples");
        f.save(&format!("demo-{}", name.to_lowercase().replace(' ', "-")));
    }
}
