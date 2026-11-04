//! The backend-agnostic picture a chart produces.
//!
//! A scene is a flat list of marks in paint order. Interactive marks carry a
//! [`Hit`]: the HTML backend turns it into `data-*` attributes for the JS
//! runtime, native backends call [`Scene::hit_test`] on pointer events and
//! [`Scene::emphasis`] to style each mark. The same scene drives both, so
//! hover behaves alike everywhere.

use std::fmt::Write;

use crate::base::format::coord;
use crate::base::text::Weight;
use crate::base::theme::Color;

/// Lines are hard to hit; the pointer only has to come this close (px).
pub const LINE_HIT_TOLERANCE: f64 = 4.0;

#[derive(Clone, Debug, PartialEq)]
pub struct Scene {
    pub width: f64,
    pub height: f64,
    /// Accessible name: what the chart shows, in one sentence.
    pub label: String,
    pub marks: Vec<Mark>,
}

#[derive(Clone, Debug, PartialEq)]
pub struct Mark {
    pub shape: Shape,
    /// Fill for areas, text color for text.
    pub fill: Option<Color>,
    pub stroke: Option<Stroke>,
    /// The series the mark draws. Hovering a series' mark or legend entry
    /// dims the other series; the legend can hide it.
    pub series: Option<String>,
    pub hit: Option<Hit>,
}

impl Mark {
    pub fn new(shape: Shape) -> Self {
        Mark {
            shape,
            fill: None,
            stroke: None,
            series: None,
            hit: None,
        }
    }

    pub fn fill(mut self, c: Color) -> Self {
        self.fill = Some(c);
        self
    }

    pub fn stroke(mut self, color: Color, width: f64) -> Self {
        self.stroke = Some(Stroke { color, width });
        self
    }

    pub fn series(mut self, s: impl Into<String>) -> Self {
        self.series = Some(s.into());
        self
    }

    pub fn hit(mut self, h: Hit) -> Self {
        self.hit = Some(h);
        self
    }
}

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Stroke {
    pub color: Color,
    pub width: f64,
}

#[derive(Clone, Copy, Debug, Default, PartialEq)]
pub struct Rect {
    pub x: f64,
    pub y: f64,
    pub w: f64,
    pub h: f64,
}

impl Rect {
    pub fn contains(&self, x: f64, y: f64) -> bool {
        x >= self.x && x <= self.x + self.w && y >= self.y && y <= self.y + self.h
    }
}

/// Where a chart with continuous axes has its plot, and the data at the
/// plot's edges: what a backend needs to turn pointer positions into data
/// when zooming (see [`crate::zoom::View`]). Not part of the scene itself,
/// so scenes, and the HTML made from them, are the same with or without it.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Plot {
    pub rect: Rect,
    /// The x data at the rect's left and right edges.
    pub x: (f64, f64),
    /// The y data at the rect's bottom and top edges, for charts that zoom
    /// y as well (scatter, bubble).
    pub y: Option<(f64, f64)>,
}

/// Per-corner radii: top-left, top-right, bottom-right, bottom-left.
#[derive(Clone, Copy, Debug, Default, PartialEq)]
pub struct Corners {
    pub tl: f64,
    pub tr: f64,
    pub br: f64,
    pub bl: f64,
}

impl Corners {
    pub fn all(r: f64) -> Self {
        Corners {
            tl: r,
            tr: r,
            br: r,
            bl: r,
        }
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Anchor {
    Start,
    Middle,
    End,
}

/// Vertical placement of text relative to its `y`, measured on the cap
/// height so it does not depend on each font's ascent metrics.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Baseline {
    /// Top of capital letters at `y`.
    Top,
    /// Middle of capital letters at `y`.
    Middle,
    /// Alphabetic baseline at `y`.
    Bottom,
}

#[derive(Clone, Debug, PartialEq)]
pub enum Shape {
    Rect {
        rect: Rect,
        radius: Corners,
    },
    Line {
        x1: f64,
        y1: f64,
        x2: f64,
        y2: f64,
    },
    /// An open polyline (series lines). Stroked, never filled.
    Polyline {
        points: Vec<(f64, f64)>,
    },
    /// The band between two x-monotone edges, as (x, top y, bottom y)
    /// triples in x order. Filled, never stroked. A dedicated shape instead
    /// of a general path because some toolkits only fill convex polygons;
    /// this one splits into convex slices trivially.
    Area {
        points: Vec<(f64, f64, f64)>,
    },
    Circle {
        cx: f64,
        cy: f64,
        r: f64,
    },
    /// A general outline: violins, sectors, ribbons, map regions. Filled
    /// with the nonzero rule when the mark has a fill, stroked when it has a
    /// stroke. Arcs arrive as cubics, so backends only need four commands.
    Path {
        ops: Vec<PathOp>,
    },
    Text {
        x: f64,
        y: f64,
        text: String,
        size: f64,
        weight: Weight,
        anchor: Anchor,
        baseline: Baseline,
        /// Clockwise rotation in degrees around (`x`, `y`).
        angle: f64,
    },
}

/// One command of a [`Shape::Path`], in absolute coordinates.
#[derive(Clone, Copy, Debug, PartialEq)]
pub enum PathOp {
    Move(f64, f64),
    Line(f64, f64),
    /// Two control points, then the end point.
    Cubic(f64, f64, f64, f64, f64, f64),
    Close,
}

/// Segments each cubic is split into when a path is flattened for hit
/// testing or for toolkits that only take polygons. Fixed, not adaptive, so
/// both ports flatten identically.
pub const CUBIC_STEPS: usize = 16;

/// The path as SVG path data; also its form in scene dumps.
pub fn path_data(ops: &[PathOp]) -> String {
    let mut d = String::new();
    for op in ops {
        match *op {
            PathOp::Move(x, y) => {
                let _ = write!(d, "M{} {}", coord(x), coord(y));
            }
            PathOp::Line(x, y) => {
                let _ = write!(d, "L{} {}", coord(x), coord(y));
            }
            PathOp::Cubic(x1, y1, x2, y2, x, y) => {
                let _ = write!(
                    d,
                    "C{} {} {} {} {} {}",
                    coord(x1),
                    coord(y1),
                    coord(x2),
                    coord(y2),
                    coord(x),
                    coord(y)
                );
            }
            PathOp::Close => d.push('Z'),
        }
    }
    d
}

/// The path as polylines, one per subpath, cubics split into
/// [`CUBIC_STEPS`] segments. A closed subpath repeats its first point last.
pub fn flatten(ops: &[PathOp]) -> Vec<Vec<(f64, f64)>> {
    let mut subs: Vec<Vec<(f64, f64)>> = Vec::new();
    for op in ops {
        match *op {
            PathOp::Move(x, y) => subs.push(vec![(x, y)]),
            PathOp::Line(x, y) => {
                if let Some(s) = subs.last_mut() {
                    s.push((x, y));
                }
            }
            PathOp::Cubic(x1, y1, x2, y2, x, y) => {
                if let Some(s) = subs.last_mut() {
                    let (x0, y0) = *s.last().expect("subpath starts with a point");
                    for k in 1..=CUBIC_STEPS {
                        let t = k as f64 / CUBIC_STEPS as f64;
                        let u = 1.0 - t;
                        let (a, b, c, e) = (u * u * u, 3.0 * u * u * t, 3.0 * u * t * t, t * t * t);
                        s.push((
                            a * x0 + b * x1 + c * x2 + e * x,
                            a * y0 + b * y1 + c * y2 + e * y,
                        ));
                    }
                }
            }
            PathOp::Close => {
                if let Some(s) = subs.last_mut() {
                    let first = s[0];
                    s.push(first);
                }
            }
        }
    }
    subs
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum HitKind {
    /// A data mark: tooltip with its value and label; the mark lifts.
    Mark,
    /// A vertical hairline over a whole x column, drawn only while hovered;
    /// its tooltip lists every series at that x (`rows`).
    Crosshair,
    /// A legend entry: hover highlights its series, click toggles it.
    Legend,
}

/// What an interactive mark reports when hovered or focused.
#[derive(Clone, Debug, PartialEq)]
pub struct Hit {
    /// Unique within the scene.
    pub key: String,
    pub kind: HitKind,
    /// The category or name; secondary in the tooltip.
    pub label: String,
    /// The formatted value; the tooltip's lead. Empty for crosshairs and
    /// legend entries.
    pub value: String,
    /// Extra tooltip lines under the label: per-series readout for
    /// crosshairs, statistics for summary marks.
    pub rows: Vec<TipRow>,
    /// Hit target when it should be larger than the painted shape.
    pub area: Option<Rect>,
}

impl Hit {
    pub fn new(key: impl Into<String>, label: impl Into<String>, value: impl Into<String>) -> Self {
        Hit {
            key: key.into(),
            kind: HitKind::Mark,
            label: label.into(),
            value: value.into(),
            rows: Vec::new(),
            area: None,
        }
    }

    pub fn kind(mut self, k: HitKind) -> Self {
        self.kind = k;
        self
    }

    pub fn rows(mut self, rows: Vec<TipRow>) -> Self {
        self.rows = rows;
        self
    }

    pub fn area(mut self, r: Rect) -> Self {
        self.area = Some(r);
        self
    }
}

/// One line of a multi-row tooltip: an optional short key in a series
/// color, the value (strong) and its name. Crosshairs key every row by
/// series; a statistic summary (box plot, OHLC) has no keys.
#[derive(Clone, Debug, PartialEq)]
pub struct TipRow {
    pub color: Option<Color>,
    pub name: String,
    pub value: String,
}

/// How a native backend paints a mark in the current interaction state.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Emphasis {
    Normal,
    /// The hovered mark: lightened slightly.
    Lift,
    /// Another series while one is highlighted, or a legend entry whose
    /// series is hidden.
    Dim,
    /// Not painted: a hidden series, or a crosshair that is not hovered.
    Hidden,
}

impl Scene {
    /// Index of the topmost interactive mark under (`x`, `y`), skipping
    /// marks of `hidden` series.
    pub fn hit_test(&self, x: f64, y: f64, hidden: &[String]) -> Option<usize> {
        self.marks.iter().enumerate().rev().find_map(|(i, m)| {
            let hit = m.hit.as_ref()?;
            if hit.kind != HitKind::Legend && is_hidden(m, hidden) {
                return None;
            }
            let inside = match hit.area {
                Some(a) => a.contains(x, y),
                None => m
                    .shape
                    .contains(x, y, m.stroke.map_or(0.0, |s| s.width), m.fill.is_some()),
            };
            inside.then_some(i)
        })
    }

    /// The single rule every native backend follows, mirroring the HTML
    /// runtime: the hovered mark lifts; hovering a series' mark or legend
    /// entry dims the other series, but only when there are two or more;
    /// crosshairs show only while hovered; hidden series are not painted.
    pub fn emphasis(&self, i: usize, hovered: Option<usize>, hidden: &[String]) -> Emphasis {
        let m = &self.marks[i];
        let kind = m.hit.as_ref().map(|h| h.kind);
        if kind == Some(HitKind::Legend) {
            return if is_hidden(m, hidden) {
                Emphasis::Dim
            } else if hovered == Some(i) {
                Emphasis::Lift
            } else {
                Emphasis::Normal
            };
        }
        if is_hidden(m, hidden) {
            return Emphasis::Hidden;
        }
        if kind == Some(HitKind::Crosshair) {
            return if hovered == Some(i) {
                Emphasis::Normal
            } else {
                Emphasis::Hidden
            };
        }
        let Some(h) = hovered else {
            return Emphasis::Normal;
        };
        if h == i {
            return Emphasis::Lift;
        }
        // Only a legend entry can be hovered while hidden; a hidden series
        // has nothing on the plot to focus on.
        if is_hidden(&self.marks[h], hidden) {
            return Emphasis::Normal;
        }
        match (&self.marks[h].series, &m.series) {
            (Some(focus), Some(own)) if focus != own && self.series_count() > 1 => Emphasis::Dim,
            _ => Emphasis::Normal,
        }
    }

    /// Distinct series among the plot's marks (legend entries excluded).
    fn series_count(&self) -> usize {
        let mut names: Vec<&str> = self
            .marks
            .iter()
            .filter(|m| m.hit.as_ref().map(|h| h.kind) != Some(HitKind::Legend))
            .filter_map(|m| m.series.as_deref())
            .collect();
        names.sort_unstable();
        names.dedup();
        names.len()
    }

    /// Deterministic JSON, one mark per line, used to check that the Rust
    /// and Go ports draw the same picture. Coordinates are rounded to 0.01.
    pub fn dump(&self) -> String {
        let mut s = String::new();
        let _ = write!(
            s,
            "{{\"width\":{},\"height\":{},\"label\":{},\"marks\":[",
            coord(self.width),
            coord(self.height),
            json_str(&self.label)
        );
        for (i, m) in self.marks.iter().enumerate() {
            s.push_str(if i == 0 { "\n" } else { ",\n" });
            dump_mark(&mut s, m);
        }
        s.push_str("\n]}\n");
        s
    }
}

fn is_hidden(m: &Mark, hidden: &[String]) -> bool {
    m.series.as_ref().is_some_and(|s| hidden.contains(s))
}

impl Shape {
    fn contains(&self, x: f64, y: f64, stroke_width: f64, filled: bool) -> bool {
        let tolerance = (stroke_width / 2.0).max(LINE_HIT_TOLERANCE);
        match self {
            Shape::Rect { rect, .. } => rect.contains(x, y),
            Shape::Line { x1, y1, x2, y2 } => {
                segment_distance(x, y, *x1, *y1, *x2, *y2) <= tolerance
            }
            Shape::Polyline { points } => points
                .windows(2)
                .any(|w| segment_distance(x, y, w[0].0, w[0].1, w[1].0, w[1].1) <= tolerance),
            Shape::Area { points } => points.windows(2).any(|w| {
                let ((x0, t0, b0), (x1, t1, b1)) = (w[0], w[1]);
                if x < x0 || x > x1 || x1 == x0 {
                    return false;
                }
                let f = (x - x0) / (x1 - x0);
                let (top, bottom) = (t0 + (t1 - t0) * f, b0 + (b1 - b0) * f);
                y >= top.min(bottom) && y <= top.max(bottom)
            }),
            Shape::Circle { cx, cy, r } => {
                ((x - cx).powi(2) + (y - cy).powi(2)).sqrt() <= r + stroke_width / 2.0
            }
            Shape::Path { ops } => {
                let subs = flatten(ops);
                let near = |s: &Vec<(f64, f64)>| {
                    s.windows(2).any(|w| {
                        segment_distance(x, y, w[0].0, w[0].1, w[1].0, w[1].1) <= tolerance
                    })
                };
                if filled {
                    winding(&subs, x, y) != 0
                } else {
                    subs.iter().any(near)
                }
            }
            Shape::Text { .. } => false,
        }
    }
}

/// Nonzero winding number of (`x`, `y`) against the subpaths, each closed
/// implicitly.
fn winding(subs: &[Vec<(f64, f64)>], x: f64, y: f64) -> i32 {
    let mut w = 0;
    for s in subs {
        for k in 0..s.len() {
            let (a, b) = (s[k], s[(k + 1) % s.len()]);
            let cross = (b.0 - a.0) * (y - a.1) - (x - a.0) * (b.1 - a.1);
            if a.1 <= y {
                if b.1 > y && cross > 0.0 {
                    w += 1;
                }
            } else if b.1 <= y && cross < 0.0 {
                w -= 1;
            }
        }
    }
    w
}

fn segment_distance(px: f64, py: f64, x1: f64, y1: f64, x2: f64, y2: f64) -> f64 {
    let (dx, dy) = (x2 - x1, y2 - y1);
    let len2 = dx * dx + dy * dy;
    let t = if len2 == 0.0 {
        0.0
    } else {
        (((px - x1) * dx + (py - y1) * dy) / len2).clamp(0.0, 1.0)
    };
    let (cx, cy) = (x1 + t * dx, y1 + t * dy);
    ((px - cx).powi(2) + (py - cy).powi(2)).sqrt()
}

fn dump_mark(s: &mut String, m: &Mark) {
    match &m.shape {
        Shape::Rect { rect, radius } => {
            let _ = write!(
                s,
                "{{\"rect\":[{}],\"radius\":[{}]",
                nums(&[rect.x, rect.y, rect.w, rect.h]),
                nums(&[radius.tl, radius.tr, radius.br, radius.bl])
            );
        }
        Shape::Line { x1, y1, x2, y2 } => {
            let _ = write!(s, "{{\"line\":[{}]", nums(&[*x1, *y1, *x2, *y2]));
        }
        Shape::Polyline { points } => {
            let flat: Vec<f64> = points.iter().flat_map(|&(x, y)| [x, y]).collect();
            let _ = write!(s, "{{\"polyline\":[{}]", nums(&flat));
        }
        Shape::Area { points } => {
            let flat: Vec<f64> = points.iter().flat_map(|&(x, t, b)| [x, t, b]).collect();
            let _ = write!(s, "{{\"area\":[{}]", nums(&flat));
        }
        Shape::Circle { cx, cy, r } => {
            let _ = write!(s, "{{\"circle\":[{}]", nums(&[*cx, *cy, *r]));
        }
        Shape::Path { ops } => {
            let _ = write!(s, "{{\"path\":{}", json_str(&path_data(ops)));
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
            let _ = write!(
                s,
                "{{\"text\":{},\"at\":[{}],\"size\":{},\"weight\":\"{}\",\"anchor\":\"{}\",\"baseline\":\"{}\",\"angle\":{}",
                json_str(text),
                nums(&[*x, *y]),
                coord(*size),
                match weight {
                    Weight::Regular => "regular",
                    Weight::Semibold => "semibold",
                },
                match anchor {
                    Anchor::Start => "start",
                    Anchor::Middle => "middle",
                    Anchor::End => "end",
                },
                match baseline {
                    Baseline::Top => "top",
                    Baseline::Middle => "middle",
                    Baseline::Bottom => "bottom",
                },
                coord(*angle)
            );
        }
    }
    if let Some(c) = m.fill {
        let _ = write!(s, ",\"fill\":\"{}\"", c.token());
    }
    if let Some(st) = m.stroke {
        let _ = write!(
            s,
            ",\"stroke\":\"{}\",\"strokeWidth\":{}",
            st.color.token(),
            coord(st.width)
        );
    }
    if let Some(series) = &m.series {
        let _ = write!(s, ",\"series\":{}", json_str(series));
    }
    if let Some(h) = &m.hit {
        let _ = write!(s, ",\"hit\":{{\"key\":{}", json_str(&h.key));
        match h.kind {
            HitKind::Mark => {}
            HitKind::Crosshair => s.push_str(",\"kind\":\"crosshair\""),
            HitKind::Legend => s.push_str(",\"kind\":\"legend\""),
        }
        let _ = write!(
            s,
            ",\"label\":{},\"value\":{}",
            json_str(&h.label),
            json_str(&h.value)
        );
        if !h.rows.is_empty() {
            s.push_str(",\"rows\":[");
            for (i, r) in h.rows.iter().enumerate() {
                if i > 0 {
                    s.push(',');
                }
                s.push('{');
                if let Some(c) = r.color {
                    let _ = write!(s, "\"color\":\"{}\",", c.token());
                }
                let _ = write!(
                    s,
                    "\"name\":{},\"value\":{}}}",
                    json_str(&r.name),
                    json_str(&r.value)
                );
            }
            s.push(']');
        }
        if let Some(a) = h.area {
            let _ = write!(s, ",\"area\":[{}]", nums(&[a.x, a.y, a.w, a.h]));
        }
        s.push('}');
    }
    s.push('}');
}

fn nums(v: &[f64]) -> String {
    v.iter().map(|&x| coord(x)).collect::<Vec<_>>().join(",")
}

/// JSON string literal with a fixed escaping rule both ports share: quote,
/// backslash and control characters escaped, everything else verbatim UTF-8.
pub(crate) fn json_str(v: &str) -> String {
    let mut s = String::with_capacity(v.len() + 2);
    s.push('"');
    for c in v.chars() {
        match c {
            '"' => s.push_str("\\\""),
            '\\' => s.push_str("\\\\"),
            '\n' => s.push_str("\\n"),
            '\r' => s.push_str("\\r"),
            '\t' => s.push_str("\\t"),
            c if (c as u32) < 0x20 => {
                let _ = write!(s, "\\u{:04x}", c as u32);
            }
            c => s.push(c),
        }
    }
    s.push('"');
    s
}

#[cfg(test)]
mod tests {
    use super::*;

    fn rect(x: f64, w: f64) -> Shape {
        Shape::Rect {
            rect: Rect {
                x,
                y: 0.0,
                w,
                h: 10.0,
            },
            radius: Corners::default(),
        }
    }

    fn bar(x: f64, key: &str) -> Mark {
        Mark::new(rect(x, 10.0))
            .fill(Color::Series(0))
            .hit(Hit::new(key, key, "1"))
    }

    fn scene(marks: Vec<Mark>) -> Scene {
        Scene {
            width: 100.0,
            height: 100.0,
            label: String::new(),
            marks,
        }
    }

    #[test]
    fn hit_test_finds_the_mark_under_the_pointer() {
        let s = scene(vec![bar(0.0, "a"), bar(20.0, "b")]);
        assert_eq!(s.hit_test(25.0, 5.0, &[]), Some(1));
        assert_eq!(s.hit_test(15.0, 5.0, &[]), None);
    }

    #[test]
    fn topmost_mark_wins_and_decoration_is_ignored() {
        // A gridline or label drawn over a bar must not steal its hover.
        let s = scene(vec![
            bar(0.0, "under"),
            bar(0.0, "over"),
            Mark::new(rect(0.0, 10.0)),
        ]);
        assert_eq!(s.hit_test(5.0, 5.0, &[]), Some(1));
    }

    #[test]
    fn hit_area_extends_beyond_the_painted_shape() {
        // A short bar is still easy to hover: its area covers the whole band.
        let area = Rect {
            x: 0.0,
            y: 0.0,
            w: 10.0,
            h: 100.0,
        };
        let s = scene(vec![
            Mark::new(rect(0.0, 10.0)).hit(Hit::new("a", "a", "1").area(area)),
        ]);
        assert_eq!(s.hit_test(5.0, 80.0, &[]), Some(0));
    }

    #[test]
    fn thin_lines_get_a_tolerance() {
        let line = Mark::new(Shape::Polyline {
            points: vec![(0.0, 50.0), (50.0, 50.0), (100.0, 50.0)],
        })
        .stroke(Color::Series(0), 2.0)
        .hit(Hit::new("l", "l", "1"));
        let s = scene(vec![line]);
        assert_eq!(s.hit_test(70.0, 53.5, &[]), Some(0));
        assert_eq!(s.hit_test(70.0, 55.0, &[]), None);
    }

    #[test]
    fn area_and_circle_hit_inside_only() {
        let s = scene(vec![
            Mark::new(Shape::Area {
                points: vec![(0.0, 10.0, 50.0), (40.0, 30.0, 50.0)],
            })
            .hit(Hit::new("a", "a", "1")),
            Mark::new(Shape::Circle {
                cx: 80.0,
                cy: 80.0,
                r: 5.0,
            })
            .hit(Hit::new("c", "c", "1")),
        ]);
        // At x=20 the top edge is at y=20.
        assert_eq!(s.hit_test(20.0, 25.0, &[]), Some(0));
        assert_eq!(s.hit_test(20.0, 15.0, &[]), None);
        assert_eq!(s.hit_test(83.0, 83.0, &[]), Some(1));
        assert_eq!(s.hit_test(86.0, 86.0, &[]), None);
    }

    #[test]
    fn hidden_series_cannot_be_hovered_but_their_legend_can() {
        let s = scene(vec![
            bar(0.0, "a").series("A"),
            Mark::new(rect(0.0, 10.0))
                .series("A")
                .hit(Hit::new("leg", "A", "").kind(HitKind::Legend)),
        ]);
        let hidden = ["A".to_string()];
        assert_eq!(s.hit_test(5.0, 5.0, &hidden), Some(1));
        let s = scene(vec![bar(0.0, "a").series("A")]);
        assert_eq!(s.hit_test(5.0, 5.0, &hidden), None);
    }

    #[test]
    fn emphasis_dims_other_series_only_when_there_are_several() {
        let two = scene(vec![bar(0.0, "a").series("A"), bar(20.0, "b").series("B")]);
        assert_eq!(two.emphasis(0, Some(0), &[]), Emphasis::Lift);
        assert_eq!(two.emphasis(1, Some(0), &[]), Emphasis::Dim);
        assert_eq!(two.emphasis(1, None, &[]), Emphasis::Normal);
        let one = scene(vec![bar(0.0, "a").series("A"), bar(20.0, "b").series("A")]);
        assert_eq!(one.emphasis(1, Some(0), &[]), Emphasis::Normal);
    }

    #[test]
    fn crosshair_shows_only_while_hovered_and_dims_nothing() {
        let s = scene(vec![
            bar(0.0, "a").series("A"),
            bar(20.0, "b").series("B"),
            Mark::new(Shape::Line {
                x1: 5.0,
                y1: 0.0,
                x2: 5.0,
                y2: 100.0,
            })
            .hit(Hit::new("x", "Jan", "").kind(HitKind::Crosshair)),
        ]);
        assert_eq!(s.emphasis(2, None, &[]), Emphasis::Hidden);
        assert_eq!(s.emphasis(2, Some(2), &[]), Emphasis::Normal);
        assert_eq!(s.emphasis(1, Some(2), &[]), Emphasis::Normal);
    }

    #[test]
    fn hidden_series_vanish_and_their_legend_entry_greys_out() {
        let s = scene(vec![
            bar(0.0, "a").series("A"),
            Mark::new(rect(50.0, 10.0))
                .series("A")
                .hit(Hit::new("leg", "A", "").kind(HitKind::Legend)),
        ]);
        let hidden = ["A".to_string()];
        assert_eq!(s.emphasis(0, None, &hidden), Emphasis::Hidden);
        assert_eq!(s.emphasis(1, None, &hidden), Emphasis::Dim);
    }

    #[test]
    fn hovering_a_hidden_series_legend_entry_dims_nothing() {
        // Right after a click hides a series the pointer still rests on its
        // legend entry; dimming the survivors then would grey the whole plot
        // out with nothing highlighted.
        let s = scene(vec![
            bar(0.0, "a").series("A"),
            bar(20.0, "b").series("B"),
            Mark::new(rect(50.0, 10.0))
                .series("A")
                .hit(Hit::new("leg", "A", "").kind(HitKind::Legend)),
        ]);
        assert_eq!(s.emphasis(1, Some(2), &[]), Emphasis::Dim);
        assert_eq!(s.emphasis(1, Some(2), &["A".to_string()]), Emphasis::Normal);
    }

    #[test]
    fn filled_paths_hit_inside_by_winding_and_open_ones_near_the_stroke() {
        // A concave "U": the notch is outside even though it is inside the
        // bounding box, which a convex fill would get wrong.
        let u = vec![
            PathOp::Move(0.0, 0.0),
            PathOp::Line(30.0, 0.0),
            PathOp::Line(30.0, 30.0),
            PathOp::Line(20.0, 30.0),
            PathOp::Line(20.0, 10.0),
            PathOp::Line(10.0, 10.0),
            PathOp::Line(10.0, 30.0),
            PathOp::Line(0.0, 30.0),
            PathOp::Close,
        ];
        let filled = scene(vec![
            Mark::new(Shape::Path { ops: u.clone() })
                .fill(Color::Series(0))
                .hit(Hit::new("u", "u", "1")),
        ]);
        assert_eq!(filled.hit_test(5.0, 20.0, &[]), Some(0));
        assert_eq!(filled.hit_test(15.0, 20.0, &[]), None);
        let open = scene(vec![
            Mark::new(Shape::Path { ops: u })
                .stroke(Color::Series(0), 2.0)
                .hit(Hit::new("u", "u", "1")),
        ]);
        assert_eq!(open.hit_test(5.0, 20.0, &[]), None);
        assert_eq!(open.hit_test(1.0, 20.0, &[]), Some(0));
    }

    #[test]
    fn cubics_flatten_through_their_end_points() {
        let ops = [
            PathOp::Move(0.0, 0.0),
            PathOp::Cubic(0.0, 10.0, 10.0, 10.0, 10.0, 0.0),
            PathOp::Close,
        ];
        let subs = flatten(&ops);
        assert_eq!(subs.len(), 1);
        assert_eq!(subs[0].len(), CUBIC_STEPS + 2);
        assert_eq!(subs[0][CUBIC_STEPS], (10.0, 0.0));
        assert_eq!(path_data(&ops), "M0 0C0 10 10 10 10 0Z");
    }

    #[test]
    fn dump_escapes_strings_and_rounds_coordinates() {
        let mut s = scene(vec![bar(1.0 / 3.0, "a\"b")]);
        s.label = "x\ny".into();
        let d = s.dump();
        assert!(d.contains("\"label\":\"x\\ny\""), "{d}");
        assert!(d.contains("\"rect\":[0.33,0,10,10]"), "{d}");
        assert!(d.contains("\"key\":\"a\\\"b\""), "{d}");
    }

    #[test]
    fn dump_writes_series_kind_and_rows() {
        let s = scene(vec![
            Mark::new(Shape::Line {
                x1: 0.0,
                y1: 0.0,
                x2: 0.0,
                y2: 1.0,
            })
            .series("A")
            .hit(
                Hit::new("x0", "Jan", "")
                    .kind(HitKind::Crosshair)
                    .rows(vec![TipRow {
                        color: Some(Color::Series(0)),
                        name: "A".into(),
                        value: "3".into(),
                    }]),
            ),
        ]);
        assert!(
            s.dump().contains(
                "\"series\":\"A\",\"hit\":{\"key\":\"x0\",\"kind\":\"crosshair\",\"label\":\"Jan\",\"value\":\"\",\"rows\":[{\"color\":\"series-1\",\"name\":\"A\",\"value\":\"3\"}]}"
            ),
            "{}",
            s.dump()
        );
    }
}
