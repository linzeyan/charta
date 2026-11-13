//! One interactive chart in a Slint window: hover for tooltips, click legend
//! entries to hide a series, and zoom by wheel, drag, shift-drag and double
//! click. It follows Slint's light or dark palette.
//!
//! The chart is a bitmap: the TouchArea forwards pointer events to
//! [`SlintChart`], and the image is redrawn only when a call reports a
//! change.
//!
//! Usage: cargo run -p charta-slint-gallery --example minimal

use std::cell::RefCell;
use std::rc::Rc;

use charta::LineChart;
use charta::backend::slint::SlintChart;
use slint::ComponentHandle;

slint::slint! {
    import { Palette } from "std-widgets.slint";

    export component Demo inherits Window {
        title: "charta in Slint";
        preferred-width: 720px;
        preferred-height: 440px;
        background: Palette.background;

        in property <image> chart;
        out property <length> chart-width: area.width;
        out property <length> chart-height: area.height;
        out property <bool> dark: Palette.color-scheme == ColorScheme.dark;

        callback redraw();
        callback moved(length, length);
        callback clicked(length, length);
        callback double-clicked(length, length);
        callback pressed(length, length, bool);
        callback released(length, length);
        callback cancelled();
        // True when the chart zooms: otherwise the wheel is left to scroll
        // whatever encloses it.
        callback scrolled(length, length, length) -> bool;
        callback exited();

        changed dark => { root.redraw(); }

        area := TouchArea {
            changed width => { root.redraw(); }
            changed height => { root.redraw(); }
            // `moved` only fires while pressed; hover needs every move.
            pointer-event(e) => {
                if (e.kind == PointerEventKind.move) {
                    root.moved(self.mouse-x, self.mouse-y);
                } else if (e.kind == PointerEventKind.cancel) {
                    root.cancelled();
                } else if (e.button == PointerEventButton.left) {
                    if (e.kind == PointerEventKind.down) {
                        root.pressed(self.mouse-x, self.mouse-y, e.modifiers.shift);
                    } else if (e.kind == PointerEventKind.up) {
                        root.released(self.mouse-x, self.mouse-y);
                    }
                }
            }
            scroll-event(e) => {
                if (root.scrolled(self.mouse-x, self.mouse-y, e.delta-y)) {
                    return accept;
                }
                return reject;
            }
            clicked => { root.clicked(self.mouse-x, self.mouse-y); }
            double-clicked => { root.double-clicked(self.mouse-x, self.mouse-y); }
            changed has-hover => {
                if (!self.has-hover) { root.exited(); }
            }
            Image {
                width: parent.width;
                height: parent.height;
                source: root.chart;
                image-fit: fill;
            }
        }
    }
}

fn main() -> Result<(), slint::PlatformError> {
    let chart = LineChart::new(["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"])
        .title("Weekly active users (k)")
        .series("Web", [120.0, 132.0, 101.0, 134.0, 90.0, 230.0, 210.0])
        .series("iOS", [220.0, 182.0, 191.0, 234.0, 290.0, 330.0, 310.0]);
    let chart = Rc::new(RefCell::new(SlintChart::new(Box::new(chart))));
    let ui = Demo::new()?;

    let redraw = Rc::new({
        let (ui, chart) = (ui.as_weak(), chart.clone());
        move || {
            let ui = ui.unwrap();
            let (w, h) = (ui.get_chart_width(), ui.get_chart_height());
            if w >= 1.0 && h >= 1.0 {
                let scale = ui.window().scale_factor();
                ui.set_chart(chart.borrow_mut().render(w, h, scale, ui.get_dark()));
            }
        }
    });
    // Each handler asks the chart whether the event changed anything; the
    // borrow ends before the redraw takes its own.
    let on = |f: fn(&mut SlintChart, f32, f32) -> bool| {
        let (chart, redraw) = (chart.clone(), redraw.clone());
        move |x, y| {
            let changed = f(&mut chart.borrow_mut(), x, y);
            if changed {
                redraw();
            }
        }
    };
    ui.on_redraw({
        let redraw = redraw.clone();
        move || redraw()
    });
    ui.on_moved(on(SlintChart::pointer_moved));
    ui.on_clicked(on(SlintChart::clicked));
    ui.on_double_clicked(on(SlintChart::double_clicked));
    ui.on_released(on(SlintChart::released));
    ui.on_pressed({
        let chart = chart.clone();
        move |x, y, shift| chart.borrow_mut().pressed(x, y, shift)
    });
    ui.on_cancelled({
        let (chart, redraw) = (chart.clone(), redraw.clone());
        move || {
            let changed = chart.borrow_mut().cancelled();
            if changed {
                redraw();
            }
        }
    });
    ui.on_scrolled({
        let (chart, redraw) = (chart.clone(), redraw.clone());
        move |x, y, dy| {
            let mut c = chart.borrow_mut();
            if !c.zooms() {
                return false;
            }
            let changed = c.scrolled(x, y, dy);
            drop(c);
            if changed {
                redraw();
            }
            true
        }
    });
    ui.on_exited({
        let (chart, redraw) = (chart.clone(), redraw.clone());
        move || {
            let changed = chart.borrow_mut().pointer_exited();
            if changed {
                redraw();
            }
        }
    });

    redraw();
    ui.run()
}
