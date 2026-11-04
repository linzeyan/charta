//! The Slint gallery window, wired to the demo charts. A library so the
//! headless test drives exactly the app's wiring.
//!
//! The chart is a bitmap: the TouchArea forwards pointer, wheel and click
//! events to [`SlintChart`], and the image is redrawn only when it reports a
//! change.

use std::cell::RefCell;
use std::rc::Rc;

use charta::backend::slint::SlintChart;
use charta_demos::demos;
use slint::{ComponentHandle, ModelRc, SharedString, VecModel};

slint::slint! {
    import { Button, ListView, Palette } from "std-widgets.slint";

    export component Gallery inherits Window {
        title: "charta slint gallery";
        preferred-width: 920px;
        preferred-height: 540px;
        background: Palette.background;

        in property <[string]> names;
        in property <image> chart;
        in-out property <int> pick: 0;
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

        HorizontalLayout {
            padding: 12px;
            spacing: 12px;
            // Scrolls: the catalogue outgrows any window height.
            ListView {
                width: 180px;
                for name[i] in root.names: VerticalLayout {
                    padding-bottom: 4px;
                    Button {
                        text: name;
                        primary: i == root.pick;
                        clicked => {
                            root.pick = i;
                            root.redraw();
                        }
                    }
                }
            }
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
}

pub fn gallery() -> Result<Gallery, slint::PlatformError> {
    let ui = Gallery::new()?;
    let (names, charts): (Vec<_>, Vec<_>) = demos()
        .into_iter()
        .map(|(name, chart)| (SharedString::from(name), SlintChart::new(chart)))
        .unzip();
    ui.set_names(ModelRc::new(VecModel::from(names)));
    let charts = Rc::new(RefCell::new(charts));

    let redraw = {
        let (ui, charts) = (ui.as_weak(), charts.clone());
        move || {
            let ui = ui.unwrap();
            let (w, h) = (ui.get_chart_width(), ui.get_chart_height());
            if w < 1.0 || h < 1.0 {
                return;
            }
            let scale = ui.window().scale_factor();
            let mut charts = charts.borrow_mut();
            let chart = &mut charts[ui.get_pick() as usize];
            ui.set_chart(chart.render(w, h, scale, ui.get_dark()));
        }
    };
    let current = {
        let (ui, charts) = (ui.as_weak(), charts.clone());
        move |f: &dyn Fn(&mut SlintChart) -> bool| {
            let pick = ui.unwrap().get_pick() as usize;
            f(&mut charts.borrow_mut()[pick])
        }
    };
    let current = Rc::new(current);
    let redraw = Rc::new(redraw);

    ui.on_redraw({
        let redraw = redraw.clone();
        move || redraw()
    });
    ui.on_moved({
        let (current, redraw) = (current.clone(), redraw.clone());
        move |x, y| {
            if current(&|c| c.pointer_moved(x, y)) {
                redraw();
            }
        }
    });
    ui.on_clicked({
        let (current, redraw) = (current.clone(), redraw.clone());
        move |x, y| {
            if current(&|c| c.clicked(x, y)) {
                redraw();
            }
        }
    });
    ui.on_double_clicked({
        let (current, redraw) = (current.clone(), redraw.clone());
        move |x, y| {
            if current(&|c| c.double_clicked(x, y)) {
                redraw();
            }
        }
    });
    ui.on_pressed({
        let current = current.clone();
        move |x, y, shift| {
            current(&|c| {
                c.pressed(x, y, shift);
                false
            });
        }
    });
    ui.on_released({
        let (current, redraw) = (current.clone(), redraw.clone());
        move |x, y| {
            if current(&|c| c.released(x, y)) {
                redraw();
            }
        }
    });
    ui.on_cancelled({
        let (current, redraw) = (current.clone(), redraw.clone());
        move || {
            if current(&|c| c.cancelled()) {
                redraw();
            }
        }
    });
    ui.on_scrolled({
        let (current, redraw) = (current.clone(), redraw.clone());
        move |x, y, dy| {
            let zooms = current(&|c| c.zooms());
            if zooms && current(&|c| c.scrolled(x, y, dy)) {
                redraw();
            }
            zooms
        }
    });
    ui.on_exited({
        let (current, redraw) = (current.clone(), redraw.clone());
        move || {
            if current(&|c| c.pointer_exited()) {
                redraw();
            }
        }
    });

    redraw();
    Ok(ui)
}
