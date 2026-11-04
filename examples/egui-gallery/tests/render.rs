//! Headless checks of the egui backend, in both palettes: hovering a bar shows
//! its tooltip, a line chart's crosshair lists every visible series, and
//! clicking a legend entry hides that series; a category axis zooms by
//! wheel, pans by drag, brushes by shift-drag and resets on double click, and
//! so do continuous axes: both of a scatterplot's, a time chart's x.
//! Also writes
//! `build/snapshots/egui-*.png`, every gallery demo included, so the painted
//! result can be eyeballed without opening a window.
//!
//! Run with `cargo test -p charta-egui-gallery` (needs a wgpu adapter; Metal on macOS).

use std::cell::{Cell, RefCell};
use std::path::PathBuf;
use std::rc::Rc;

use charta::backend::egui::{ChartView, EguiMeasure};
use charta::base::text::{TextMeasure, Weight};
use charta::scene::{HitKind, Plot, Rect as SRect, Scene};
use charta::zoom::{View, Window};
use charta::{Candle, CandlestickChart, Chart, LineChart, Point, ScatterChart};
use charta_demos::{demos, line};
use eframe::egui::{
    Event, Id, Modifiers, MouseWheelSource, MouseWheelUnit, PointerButton, Pos2, Rect, Theme,
    TouchPhase, pos2, vec2,
};
use egui_kittest::Harness;
use egui_kittest::kittest::Queryable;

const SIZE: (f32, f32) = (720.0, 440.0);

/// A harness showing `chart`, plus the widget's rect and the scene the
/// widget painted (same size, same measure), to aim the pointer with.
fn open(chart: &dyn Chart, theme: Theme) -> (Harness<'_>, Rect, Scene) {
    let size = vec2(SIZE.0, SIZE.1);
    let widget = Rc::new(Cell::new(Rect::NOTHING));
    let sink = widget.clone();
    let mut harness = Harness::builder()
        .with_size(size + vec2(32.0, 32.0))
        .wgpu()
        .build_ui(move |ui| sink.set(ui.add(ChartView::new(chart).size(size)).rect));
    harness.ctx.set_theme(theme);
    // Tooltips normally wait for the pointer to rest; the test has no clock.
    harness
        .ctx
        .all_styles_mut(|s| s.interaction.tooltip_delay = 0.0);
    harness.run();
    let rect = widget.get();
    let scene = chart
        .scene(
            SIZE.0 as f64,
            SIZE.1 as f64,
            &EguiMeasure(harness.ctx.clone()),
        )
        .unwrap();
    (harness, rect, scene)
}

fn center(rect: Rect, area: SRect) -> Pos2 {
    pos2(
        rect.min.x + (area.x + area.w / 2.0) as f32,
        rect.min.y + (area.y + area.h / 2.0) as f32,
    )
}

fn snapshot(harness: &mut Harness<'_>, name: &str, theme: Theme) {
    let out = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../build/snapshots");
    std::fs::create_dir_all(&out).unwrap();
    let mode = match theme {
        Theme::Dark => "dark",
        Theme::Light => "light",
    };
    harness
        .render()
        .unwrap()
        .save(out.join(format!("egui-{name}-{mode}.png")))
        .unwrap();
}

fn hover_bar(theme: Theme) {
    let demos = demos();
    let (mut harness, rect, scene) = open(demos[0].1.as_ref(), theme);
    let area = scene
        .marks
        .iter()
        .find_map(|m| m.hit.as_ref().filter(|h| h.label == "Pears")?.area)
        .expect("Pears has a hit area");
    harness.hover_at(center(rect, area));
    harness.run();

    assert!(
        harness.query_by_label("18").is_some(),
        "tooltip value for Pears is not shown ({theme:?})"
    );
    assert!(
        harness.query_by_label("Pears").is_some(),
        "tooltip label for Pears is not shown ({theme:?})"
    );
    snapshot(&mut harness, "bar", theme);
}

fn crosshair_and_legend(theme: Theme) {
    let chart = line();
    let (mut harness, rect, scene) = open(&chart, theme);
    let hit = |kind: HitKind, label: &str| {
        scene
            .marks
            .iter()
            .find_map(|m| {
                let h = m.hit.as_ref()?;
                let named = h.label == label || m.series.as_deref() == Some(label);
                (h.kind == kind && named).then_some(h.area?)
            })
            .unwrap_or_else(|| panic!("no {kind:?} hit for {label}"))
    };
    let apr = center(rect, hit(HitKind::Crosshair, "Apr"));

    harness.hover_at(apr);
    harness.run();
    for v in ["160", "108", "85"] {
        assert!(
            harness.query_by_label(v).is_some(),
            "crosshair row {v} missing ({theme:?})"
        );
    }

    let ios = center(rect, hit(HitKind::Legend, "iOS"));
    harness.hover_at(ios);
    harness.run();
    for pressed in [true, false] {
        harness.event(Event::PointerButton {
            pos: ios,
            button: PointerButton::Primary,
            pressed,
            modifiers: Modifiers::NONE,
        });
        harness.run();
    }

    harness.hover_at(apr);
    harness.run();
    assert!(
        harness.query_by_label("160").is_some(),
        "visible series left the tooltip ({theme:?})"
    );
    assert!(
        harness.query_by_label("108").is_none(),
        "hidden iOS series still in the tooltip ({theme:?})"
    );
    snapshot(&mut harness, "line-hidden", theme);
}

fn gallery(theme: Theme) {
    for (name, chart) in demos() {
        let (mut harness, _, _) = open(chart.as_ref(), theme);
        let slug = name.to_lowercase().replace(' ', "-");
        snapshot(&mut harness, &format!("gallery-{slug}"), theme);
    }
}

#[test]
fn hover_shows_tooltip_light() {
    hover_bar(Theme::Light);
}

#[test]
fn hover_shows_tooltip_dark() {
    hover_bar(Theme::Dark);
}

#[test]
fn crosshair_lists_visible_series_light() {
    crosshair_and_legend(Theme::Light);
}

#[test]
fn crosshair_lists_visible_series_dark() {
    crosshair_and_legend(Theme::Dark);
}

#[test]
fn every_demo_renders_light() {
    gallery(Theme::Light);
}

#[test]
fn every_demo_renders_dark() {
    gallery(Theme::Dark);
}

/// Presses at `from`, moves to `to` in steps (egui needs real movement to
/// call it a drag), releases. Steps a fixed number of frames instead of
/// running to rest: the tooltip trailing the pointer keeps animating.
fn drag(harness: &mut Harness<'_>, from: Pos2, to: Pos2) {
    harness.hover_at(from);
    harness.run_steps(2);
    let button = |pos, pressed| Event::PointerButton {
        pos,
        button: PointerButton::Primary,
        pressed,
        modifiers: Modifiers::NONE,
    };
    harness.event(button(from, true));
    harness.run_steps(2);
    for k in 1..=10 {
        harness.event(Event::PointerMoved(from + (to - from) * (k as f32 / 10.0)));
        harness.run_steps(2);
    }
    harness.event(button(to, false));
    harness.run_steps(2);
}

#[test]
fn a_category_axis_zooms_pans_brushes_and_resets() {
    const MONTHS: [&str; 12] = [
        "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
    ];
    let chart = LineChart::new(MONTHS)
        .series("Visits", (1..=12).map(|i| i as f64 * 10.0))
        .series("Signups", (1..=12).map(|i| i as f64 * 4.0));
    let full = chart.clone();
    let size = vec2(SIZE.0, SIZE.1);
    let widget = Rc::new(Cell::new((Rect::NOTHING, Id::NULL)));
    let sink = widget.clone();
    // Frames 1/60 s apart, so two clicks a frame apart count as a double
    // click (the default step is longer than egui's 0.3 s window).
    let mut harness = Harness::builder()
        .with_size(size + vec2(32.0, 32.0))
        .with_step_dt(1.0 / 60.0)
        .wgpu()
        .build_ui(move |ui| {
            let r = ui.add(ChartView::new(&chart).size(size));
            sink.set((r.rect, r.id));
        });
    harness.ctx.set_theme(Theme::Light);
    harness.run_steps(2);
    let (rect, id) = widget.get();
    let window = |h: &Harness<'_>| {
        h.ctx
            .data(|d| d.get_temp::<Window>(id.with("zoom")))
            .unwrap_or(Window::full(12))
    };

    // Two notches in over the middle: fewer months, the middle kept.
    let mid = rect.center();
    harness.hover_at(mid);
    harness.run_steps(2);
    harness.event(Event::MouseWheel {
        unit: MouseWheelUnit::Line,
        delta: vec2(0.0, 2.0),
        phase: TouchPhase::Move,
        source: MouseWheelSource::Unknown,
        modifiers: Modifiers::NONE,
    });
    harness.run_steps(2);
    let zoomed = window(&harness);
    assert!(zoomed.start > 0 && zoomed.end < 12, "{zoomed:?}");

    // Dragging left brings later months in.
    drag(&mut harness, mid, mid - vec2(200.0, 0.0));
    let panned = window(&harness);
    assert!(panned.start > zoomed.start, "{zoomed:?} → {panned:?}");
    assert_eq!(panned.end - panned.start, zoomed.end - zoomed.start);

    // A double click shows the whole year again.
    harness.hover_at(mid);
    for pressed in [true, false, true, false] {
        harness.event(Event::PointerButton {
            pos: mid,
            button: PointerButton::Primary,
            pressed,
            modifiers: Modifiers::NONE,
        });
        harness.step();
    }
    assert!(window(&harness).is_full());

    // Shift-dragging across a quarter of the plot zooms to those months.
    harness.event(Event::ModifiersChanged(Modifiers::SHIFT));
    let left = pos2(rect.min.x + SIZE.0 * 0.3, mid.y);
    drag(&mut harness, left, left + vec2(SIZE.0 * 0.25, 0.0));
    harness.event(Event::ModifiersChanged(Modifiers::NONE));
    let brushed = window(&harness);
    assert!(
        (2..=5).contains(&(brushed.end - brushed.start)) && brushed.start >= 2,
        "{brushed:?}"
    );

    // A double click on a legend entry toggles the series twice and leaves
    // the zoom alone.
    let legend = full
        .scene(
            SIZE.0 as f64,
            SIZE.1 as f64,
            &EguiMeasure(harness.ctx.clone()),
        )
        .unwrap()
        .marks
        .iter()
        .find_map(|m| m.hit.as_ref().filter(|h| h.kind == HitKind::Legend)?.area)
        .unwrap();
    let at = center(rect, legend);
    harness.hover_at(at);
    for pressed in [true, false, true, false] {
        harness.event(Event::PointerButton {
            pos: at,
            button: PointerButton::Primary,
            pressed,
            modifiers: Modifiers::NONE,
        });
        harness.step();
    }
    assert_eq!(window(&harness), brushed);
    snapshot(&mut harness, "zoomed", Theme::Light);
}

/// A harness showing `chart` at SIZE with frames 1/60 s apart (so two clicks
/// a frame apart are a double click), the widget's rect, and a reader of the
/// continuous view it keeps, full until a gesture moves it.
fn zoomable(chart: &dyn Chart) -> (Harness<'_>, Rect, Plot, impl Fn(&Harness<'_>) -> View) {
    let size = vec2(SIZE.0, SIZE.1);
    let widget = Rc::new(Cell::new((Rect::NOTHING, Id::NULL)));
    let sink = widget.clone();
    let mut harness = Harness::builder()
        .with_size(size + vec2(32.0, 32.0))
        .with_step_dt(1.0 / 60.0)
        .wgpu()
        .build_ui(move |ui| {
            let r = ui.add(ChartView::new(chart).size(size));
            sink.set((r.rect, r.id));
        });
    harness.ctx.set_theme(Theme::Light);
    harness.run_steps(2);
    let (rect, id) = widget.get();
    let (_, plot) = chart
        .plot_scene(
            SIZE.0 as f64,
            SIZE.1 as f64,
            &EguiMeasure(harness.ctx.clone()),
        )
        .unwrap();
    let plot = plot.expect("the chart zooms continuously");
    let view = move |h: &Harness<'_>| {
        h.ctx
            .data(|d| d.get_temp::<View>(id.with("zoom")))
            .unwrap_or(View::full(&plot))
    };
    (harness, rect, plot, view)
}

/// The screen point `(tx, ty)` of the way across `plot`.
fn across(rect: Rect, plot: &Plot, tx: f64, ty: f64) -> Pos2 {
    let r = plot.rect;
    pos2(
        rect.min.x + (r.x + tx * r.w) as f32,
        rect.min.y + (r.y + ty * r.h) as f32,
    )
}

fn wheel(harness: &mut Harness<'_>, at: Pos2, notches: f32) {
    harness.hover_at(at);
    harness.run_steps(2);
    harness.event(Event::MouseWheel {
        unit: MouseWheelUnit::Line,
        delta: vec2(0.0, notches),
        phase: TouchPhase::Move,
        source: MouseWheelSource::Unknown,
        modifiers: Modifiers::NONE,
    });
    harness.run_steps(2);
}

fn double_click(harness: &mut Harness<'_>, at: Pos2) {
    harness.hover_at(at);
    for pressed in [true, false, true, false] {
        harness.event(Event::PointerButton {
            pos: at,
            button: PointerButton::Primary,
            pressed,
            modifiers: Modifiers::NONE,
        });
        harness.step();
    }
}

#[test]
fn a_scatterplot_zooms_both_axes_by_wheel_drag_and_box() {
    let chart = ScatterChart::new().series(
        "Trials",
        (0..60)
            .map(|i| Point::new(i as f64, ((i * 37) % 100) as f64))
            .collect(),
    );
    let (mut harness, rect, plot, view) = zoomable(&chart);
    let full = View::full(&plot);
    let (fx, fy) = (full.x, full.y.unwrap());

    // Two notches in at a third across, a third down: both axes narrow
    // around the data under the pointer.
    wheel(&mut harness, across(rect, &plot, 1.0 / 3.0, 1.0 / 3.0), 2.0);
    let zoomed = view(&harness);
    let (zx, zy) = (zoomed.x, zoomed.y.unwrap());
    let under = (fx.lo + (fx.hi - fx.lo) / 3.0, fy.hi - (fy.hi - fy.lo) / 3.0);
    assert!(zx.hi - zx.lo < 0.7 * (fx.hi - fx.lo), "{zoomed:?}");
    assert!(zy.hi - zy.lo < 0.7 * (fy.hi - fy.lo), "{zoomed:?}");
    let share = |lo: f64, hi: f64, v: f64| (v - lo) / (hi - lo);
    assert!((share(zx.lo, zx.hi, under.0) - 1.0 / 3.0).abs() < 0.02);
    assert!((share(zy.lo, zy.hi, under.1) - 2.0 / 3.0).abs() < 0.02);

    // Dragging left and down: the data follows the pointer, so later x
    // and higher y come into view, by the same amount on screen.
    let mid = across(rect, &plot, 0.5, 0.5);
    drag(&mut harness, mid, mid + vec2(-120.0, 60.0));
    let panned = view(&harness);
    let (px, py) = (panned.x, panned.y.unwrap());
    assert!(px.lo > zx.lo && py.lo > zy.lo, "{zoomed:?} → {panned:?}");
    assert!(((px.hi - px.lo) - (zx.hi - zx.lo)).abs() < 1e-9);

    double_click(&mut harness, mid);
    assert!(view(&harness).is_full(), "{:?}", view(&harness));

    // Shift-dragging a box from a quarter in to past the middle zooms to
    // exactly the data it covers, on both axes.
    harness.event(Event::ModifiersChanged(Modifiers::SHIFT));
    drag(
        &mut harness,
        across(rect, &plot, 0.25, 0.25),
        across(rect, &plot, 0.6, 0.7),
    );
    harness.event(Event::ModifiersChanged(Modifiers::NONE));
    let boxed = view(&harness);
    let (bx, by) = (boxed.x, boxed.y.unwrap());
    let near = |a: f64, b: f64, of: f64| (a - b).abs() < 0.01 * of;
    let (wx, wy) = (fx.hi - fx.lo, fy.hi - fy.lo);
    assert!(
        near(bx.lo, fx.lo + 0.25 * wx, wx) && near(bx.hi, fx.lo + 0.6 * wx, wx),
        "{boxed:?}"
    );
    assert!(
        near(by.hi, fy.hi - 0.25 * wy, wy) && near(by.lo, fy.hi - 0.7 * wy, wy),
        "{boxed:?}"
    );
    snapshot(&mut harness, "zoomed-scatter", Theme::Light);
}

#[test]
fn new_data_shows_whole_rather_than_through_an_old_view() {
    let points = |n: i32| (0..n).map(|i| Point::new(i as f64, i as f64)).collect();
    let shared = Rc::new(RefCell::new(ScatterChart::new().series("a", points(60))));
    let size = vec2(SIZE.0, SIZE.1);
    let widget = Rc::new(Cell::new(Rect::NOTHING));
    let (chart, sink) = (shared.clone(), widget.clone());
    let mut harness = Harness::builder()
        .with_size(size + vec2(32.0, 32.0))
        .wgpu()
        .build_ui(move |ui| {
            let c = chart.borrow();
            sink.set(ui.add(ChartView::new(&*c).size(size)).rect);
        });
    harness
        .ctx
        .all_styles_mut(|s| s.interaction.tooltip_delay = 0.0);
    harness.run_steps(2);
    let rect = widget.get();
    let m = EguiMeasure(harness.ctx.clone());
    let plot = |c: &ScatterChart| c.plot_scene(SIZE.0 as f64, SIZE.1 as f64, &m).unwrap();
    let old = plot(&shared.borrow()).1.unwrap();
    wheel(&mut harness, across(rect, &old, 0.2, 0.8), 4.0);

    // Far beyond the old view: shown only if the view starts over.
    *shared.borrow_mut() = ScatterChart::new().series("a", {
        let mut p = points(60);
        p.push(Point::new(500.0, 500.0));
        p
    });
    harness.run_steps(2);
    let (scene, _) = plot(&shared.borrow());
    let far = scene
        .marks
        .iter()
        .find_map(|m| m.hit.as_ref().filter(|h| h.value == "500, 500")?.area)
        .unwrap();
    harness.hover_at(center(rect, far));
    harness.run_steps(2);
    assert!(
        harness.query_by_label("500, 500").is_some(),
        "the new point is not shown"
    );
}

#[test]
fn a_time_axis_zooms_along_x_only() {
    const DAY: i64 = 86_400_000;
    let start = 1_767_225_600_000; // 2026-01-01
    let chart = CandlestickChart::new(
        (0..30)
            .map(|i| {
                let o = 100.0 + (i * 7 % 11) as f64;
                Candle::new(start + i * DAY, o, o + 4.0, o - 3.0, o + 1.0)
            })
            .collect(),
    );
    let (mut harness, rect, plot, view) = zoomable(&chart);
    assert!(View::full(&plot).y.is_none());

    wheel(&mut harness, across(rect, &plot, 0.5, 0.5), 3.0);
    let zoomed = view(&harness);
    assert!(zoomed.y.is_none() && !zoomed.is_full(), "{zoomed:?}");

    // A drag straight down has nothing to pan.
    let mid = across(rect, &plot, 0.5, 0.5);
    drag(&mut harness, mid, mid + vec2(0.0, 120.0));
    assert_eq!(view(&harness), zoomed);

    // A shift-drag brushes a band: any height will do.
    double_click(&mut harness, mid);
    harness.event(Event::ModifiersChanged(Modifiers::SHIFT));
    drag(
        &mut harness,
        across(rect, &plot, 0.2, 0.5),
        across(rect, &plot, 0.5, 0.5),
    );
    harness.event(Event::ModifiersChanged(Modifiers::NONE));
    let brushed = view(&harness);
    let (x0, x1) = brushed.ranges().0;
    let shown = chart
        .view((x0, x1), None)
        .unwrap()
        .scene(
            SIZE.0 as f64,
            SIZE.1 as f64,
            &EguiMeasure(harness.ctx.clone()),
        )
        .unwrap();
    let candles = shown.marks.iter().filter(|m| m.hit.is_some()).count();
    // Three tenths of thirty days, give or take the cut ones.
    assert!(
        (7..=10).contains(&candles),
        "{candles} candles in {brushed:?}"
    );
    snapshot(&mut harness, "zoomed-candles", Theme::Light);
}

#[test]
fn titles_are_set_in_a_heavier_face() {
    // egui bundles no semibold; the chart families come from the system so
    // a title stands out from its labels, as in every other backend.
    let chart = line();
    let (harness, ..) = open(&chart, Theme::Light);
    let m = EguiMeasure(harness.ctx.clone());
    let text = "Weekly signups";
    let (regular, semibold) = (
        m.width(text, 16.0, Weight::Regular),
        m.width(text, 16.0, Weight::Semibold),
    );
    assert!(regular > 0.0, "no font measured the text");
    assert!(
        semibold > regular * 1.02,
        "semibold {semibold} is no wider than regular {regular}"
    );
}
