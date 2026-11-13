use super::frame::{self, Key, halo_label, halo_rect};
use super::graph::{Graph, Link, graph, quad};
use super::map::{Fit, check_lon_lat, land, map_chart, regions};
use super::{Chart, DOT_RADIUS, GAP, MIN_TARGET, PAD, SURFACE_GAP, check_size, count, describe};
use crate::base::format;
use crate::base::text::TextMeasure;
use crate::base::theme::Color;
use crate::error::Error;
use crate::geo::great_circle;
use crate::scene::{Anchor, Hit, Mark, PathOp, Rect, Scene, Shape, TipRow};

/// Segments per great-circle route: smooth at any map size, few points.
const ROUTE_STEPS: usize = 32;
/// Route widths run from 1px to this; flow widths from `FLOW_MIN`.
const ROUTE_MAX: f64 = 3.0;
const FLOW_MIN: f64 = 1.5;
const FLOW_MAX: f64 = 10.0;
/// How far a flow bows sideways, as a share of its length: enough that
/// flows in opposite directions between two places separate.
const BOW: f64 = 0.2;
/// Past this many places, labels would pile up; tooltips name them.
const MAX_LABELS: usize = 20;
/// Space between a label and the dots or labels around it.
const LABEL_GAP: f64 = 2.0;

/// A named location.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Location {
    pub name: String,
    pub lon: f64,
    pub lat: f64,
}

map_chart!(
    /// Places joined by their shortest routes on the globe (great
    /// circles): which places connect.
    ///
    /// # Example
    ///
    /// ```
    /// use charta::base::text::InterMetrics;
    /// use charta::{Chart, ConnectionMapChart};
    ///
    /// let chart = ConnectionMapChart::new(Vec::new())
    ///     .title("Nonstop flights from Singapore")
    ///     .place("Singapore", 103.82, 1.35)
    ///     .place("London", -0.13, 51.51)
    ///     .place("Sydney", 151.21, -33.87)
    ///     .link("Singapore", "London", 28.0)
    ///     .link("Singapore", "Sydney", 35.0);
    /// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
    /// // The scene's label is what a screen reader announces.
    /// assert_eq!(scene.label, "Nonstop flights from Singapore. Connection map of 2 routes between 3 places.");
    /// ```
    ConnectionMapChart {
        places: Vec<Location>,
        links: Vec<Link>
    }
);

map_chart!(
    /// Curved arrows from place to place, as thick as what moves: where
    /// things go and how much.
    ///
    /// # Example
    ///
    /// ```
    /// use charta::base::text::InterMetrics;
    /// use charta::{Chart, FlowMapChart};
    ///
    /// let chart = FlowMapChart::new(Vec::new())
    ///     .title("Green coffee exports (kt)")
    ///     .place("Brazil", -51.9, -14.2)
    ///     .place("USA", -98.6, 39.8)
    ///     .place("Germany", 10.5, 51.2)
    ///     .link("Brazil", "USA", 400.0)
    ///     .link("Brazil", "Germany", 300.0);
    /// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
    /// // The scene's label is what a screen reader announces.
    /// assert_eq!(scene.label, "Green coffee exports (kt). Flow map of 2 flows between 3 places.");
    /// ```
    FlowMapChart {
        places: Vec<Location>,
        links: Vec<Link>
    }
);

macro_rules! route_builders {
    ($($name:ident),+) => {$(
        impl $name {
            pub fn place(mut self, name: impl Into<String>, lon: f64, lat: f64) -> Self {
                self.places.push(Location {
                    name: name.into(),
                    lon,
                    lat,
                });
                self
            }

            pub fn link(
                mut self,
                source: impl Into<String>,
                target: impl Into<String>,
                value: f64,
            ) -> Self {
                self.links.push(Link::new(source, target, value));
                self
            }
        }
    )+};
}

route_builders!(ConnectionMapChart, FlowMapChart);

/// The links resolved against the places: graph node `i` sits at
/// `at[i]`. Places no link mentions are still drawn, after the linked ones.
struct Routes {
    g: Graph,
    at: Vec<(f64, f64)>,
}

fn routes(places: &[Location], links: &[Link], directed: bool) -> Result<Routes, Error> {
    for (i, p) in places.iter().enumerate() {
        check_lon_lat(&format!("{:?}", p.name), p.lon, p.lat)?;
        if places[..i].iter().any(|o| o.name == p.name) {
            return Err(Error::InvalidData(format!(
                "place {:?} is listed twice",
                p.name
            )));
        }
    }
    let mut g = graph(links, directed)?;
    let mut at = Vec::with_capacity(places.len());
    for name in &g.names {
        let p = places.iter().find(|p| p.name == *name).ok_or_else(|| {
            Error::InvalidData(format!("a link names {name:?}, which is not a place"))
        })?;
        at.push((p.lon, p.lat));
    }
    for p in places {
        if !g.names.contains(&p.name) {
            g.names.push(p.name.clone());
            at.push((p.lon, p.lat));
        }
    }
    Ok(Routes { g, at })
}

/// Place dots, then labels on top of everything when there are few.
fn draw_places(
    marks: &mut Vec<Mark>,
    measure: &dyn TextMeasure,
    r: &Routes,
    xy: &[(f64, f64)],
    width: f64,
) {
    let series = |i: usize| r.g.colored().then(|| r.g.names[i].clone());
    for (i, name) in r.g.names.iter().enumerate() {
        let links = r.g.edges.iter().filter(|e| e.0 == i || e.1 == i).count();
        let (cx, cy) = xy[i];
        let mut m = Mark::new(Shape::Circle {
            cx,
            cy,
            r: DOT_RADIUS,
        })
        .fill(Color::Series(r.g.color(i)))
        .stroke(Color::Surface, SURFACE_GAP)
        .hit(
            Hit::new(format!("place-{i}"), name.as_str(), "")
                .rows(vec![TipRow {
                    color: None,
                    name: "links".into(),
                    value: links.to_string(),
                }])
                .area(Rect {
                    x: cx - MIN_TARGET / 2.0,
                    y: cy - MIN_TARGET / 2.0,
                    w: MIN_TARGET,
                    h: MIN_TARGET,
                }),
        );
        if let Some(s) = series(i) {
            m = m.series(s);
        }
        marks.push(m);
    }
    if r.g.names.len() > MAX_LABELS {
        return;
    }
    // Places cluster (Europe, East Asia), so labels go greedily: hubs first,
    // each to the right of its dot or else the left, and dropped when both
    // sides hit a placed label, another dot or the chart edge. A dropped
    // name is still in the dot's tooltip.
    let mut order: Vec<usize> = (0..r.g.names.len()).collect();
    let degree = |i: usize| r.g.edges.iter().filter(|e| e.0 == i || e.1 == i).count();
    order.sort_by_key(|&i| std::cmp::Reverse(degree(i)));
    let dots: Vec<Rect> = xy
        .iter()
        .map(|&(cx, cy)| Rect {
            x: cx - DOT_RADIUS,
            y: cy - DOT_RADIUS,
            w: 2.0 * DOT_RADIUS,
            h: 2.0 * DOT_RADIUS,
        })
        .collect();
    let mut placed: Vec<Rect> = Vec::new();
    let clashes = |a: &Rect, b: &Rect| {
        a.x < b.x + b.w + LABEL_GAP
            && b.x < a.x + a.w + LABEL_GAP
            && a.y < b.y + b.h + LABEL_GAP
            && b.y < a.y + a.h + LABEL_GAP
    };
    for i in order {
        let name = r.g.names[i].as_str();
        let off = DOT_RADIUS + GAP / 2.0;
        let spot = [
            ((xy[i].0 + off, xy[i].1), Anchor::Start),
            ((xy[i].0 - off, xy[i].1), Anchor::End),
        ]
        .into_iter()
        .find(|&(at, anchor)| {
            let b = halo_rect(measure, at, name, anchor);
            b.x >= 0.0
                && b.x + b.w <= width
                && !placed.iter().any(|p| clashes(p, &b))
                && !dots
                    .iter()
                    .enumerate()
                    .any(|(j, d)| j != i && clashes(d, &b))
        });
        if let Some((at, anchor)) = spot {
            placed.push(halo_rect(measure, at, name, anchor));
            halo_label(marks, measure, at, name, anchor, Color::TextSecondary);
        }
    }
}

impl Chart for ConnectionMapChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let regions = regions(&self.features, self.basemap.as_deref())?;
        let r = routes(&self.places, &self.links, false)?;
        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &r.g.legend(), Key::Swatch);
        let fit = Fit::new(
            self.projection,
            &regions,
            &r.at,
            (PAD, top, width - PAD, height - PAD),
        );
        land(&mut marks, &fit, &regions);
        let max = r.g.edges.iter().map(|e| e.2).fold(0.0, f64::max);
        for (k, &(a, b, v)) in r.g.edges.iter().enumerate() {
            // A route crossing the antimeridian leaves one edge of the map
            // and re-enters at the other: break it there instead of
            // drawing a line across the whole world.
            let mut ops = Vec::new();
            let mut prev: Option<f64> = None;
            for (lon, lat) in great_circle(r.at[a], r.at[b], ROUTE_STEPS) {
                let (x, y) = fit.xy(lon, lat);
                match prev {
                    Some(p) if (lon - p).abs() <= 180.0 => ops.push(PathOp::Line(x, y)),
                    _ => ops.push(PathOp::Move(x, y)),
                }
                prev = Some(lon);
            }
            let mut m = Mark::new(Shape::Path { ops })
                .stroke(Color::Link(r.g.color(a)), 1.0 + (ROUTE_MAX - 1.0) * v / max)
                .hit(Hit::new(
                    format!("route-{k}"),
                    format!("{} – {}", r.g.names[a], r.g.names[b]),
                    format::value(v),
                ));
            if r.g.colored() {
                m = m.series(r.g.names[a].as_str());
            }
            marks.push(m);
        }
        let xy: Vec<(f64, f64)> = r.at.iter().map(|&(lon, lat)| fit.xy(lon, lat)).collect();
        draw_places(&mut marks, measure, &r, &xy, width);
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Connection map of {} between {}.",
                    count(r.g.edges.len(), "route", "routes"),
                    count(r.g.names.len(), "place", "places")
                ),
            ),
            marks,
        })
    }
}

impl Chart for FlowMapChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let regions = regions(&self.features, self.basemap.as_deref())?;
        let r = routes(&self.places, &self.links, true)?;
        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &r.g.legend(), Key::Swatch);
        let fit = Fit::new(
            self.projection,
            &regions,
            &r.at,
            (PAD, top, width - PAD, height - PAD),
        );
        land(&mut marks, &fit, &regions);
        let xy: Vec<(f64, f64)> = r.at.iter().map(|&(lon, lat)| fit.xy(lon, lat)).collect();
        let max = r.g.edges.iter().map(|e| e.2).fold(0.0, f64::max);
        for (k, &(a, b, v)) in r.g.edges.iter().enumerate() {
            let (p, q) = (xy[a], xy[b]);
            let (dx, dy) = (q.0 - p.0, q.1 - p.1);
            let len = (dx * dx + dy * dy).sqrt();
            if len == 0.0 {
                continue;
            }
            // Bow to the right of the direction of travel, so A → B and
            // B → A curve apart instead of overlapping.
            let c = ((p.0 + q.0) / 2.0 - dy * BOW, (p.1 + q.1) / 2.0 + dx * BOW);
            let w = FLOW_MIN + (FLOW_MAX - FLOW_MIN) * v / max;
            // The head points along the curve's final tangent (from the
            // control point to the end) and the shaft stops at its base.
            let (tx, ty) = (q.0 - c.0, q.1 - c.1);
            let tl = (tx * tx + ty * ty).sqrt();
            let (ux, uy) = (tx / tl, ty / tl);
            let (head, half) = (2.0 * w + 4.0, w + 3.0);
            let base = (q.0 - ux * head, q.1 - uy * head);
            let mut shaft = vec![PathOp::Move(p.0, p.1)];
            quad(&mut shaft, p, c, base);
            let color = Color::Link(r.g.color(a));
            let mut m = Mark::new(Shape::Path { ops: shaft })
                .stroke(color, w)
                .hit(Hit::new(
                    format!("flow-{k}"),
                    format!("{} → {}", r.g.names[a], r.g.names[b]),
                    format::value(v),
                ));
            let mut h = Mark::new(Shape::Path {
                ops: vec![
                    PathOp::Move(q.0, q.1),
                    PathOp::Line(base.0 - uy * half, base.1 + ux * half),
                    PathOp::Line(base.0 + uy * half, base.1 - ux * half),
                    PathOp::Close,
                ],
            })
            .fill(color);
            if r.g.colored() {
                m = m.series(r.g.names[a].as_str());
                h = h.series(r.g.names[a].as_str());
            }
            marks.push(m);
            marks.push(h);
        }
        draw_places(&mut marks, measure, &r, &xy, width);
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Flow map of {} between {}.",
                    count(r.g.edges.len(), "flow", "flows"),
                    count(r.g.names.len(), "place", "places")
                ),
            ),
            marks,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;

    fn keyed<'a>(s: &'a Scene, key: &str) -> &'a Mark {
        s.marks
            .iter()
            .find(|m| m.hit.as_ref().is_some_and(|h| h.key == key))
            .unwrap()
    }

    #[test]
    fn a_route_across_the_date_line_breaks_instead_of_spanning_the_map() {
        let s = ConnectionMapChart::new(vec![])
            .place("Tokyo", 139.7, 35.7)
            .place("San Francisco", -122.4, 37.8)
            .link("Tokyo", "San Francisco", 1.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let Shape::Path { ops } = &keyed(&s, "route-0").shape else {
            panic!()
        };
        let moves = ops.iter().filter(|o| matches!(o, PathOp::Move(..))).count();
        assert_eq!(moves, 2);
        // No segment jumps more than a fraction of the width.
        for w in crate::scene::flatten(ops) {
            for p in w.windows(2) {
                assert!((p[1].0 - p[0].0).abs() < 100.0);
            }
        }
    }

    /// Every label's backing box, and every dot's center.
    fn labels_and_dots(s: &Scene) -> (Vec<Rect>, Vec<(f64, f64)>) {
        let mut halos = Vec::new();
        let mut dots = Vec::new();
        for m in &s.marks {
            match m.shape {
                Shape::Rect { rect, .. } if m.fill == Some(Color::Surface) => halos.push(rect),
                Shape::Circle { cx, cy, .. } => dots.push((cx, cy)),
                _ => {}
            }
        }
        (halos, dots)
    }

    fn assert_clear(s: &Scene) {
        let (halos, dots) = labels_and_dots(s);
        let meet = |a: &Rect, b: &Rect| {
            a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h
        };
        for (i, a) in halos.iter().enumerate() {
            for b in &halos[i + 1..] {
                assert!(!meet(a, b), "{a:?} {b:?}");
            }
            for &(cx, cy) in &dots {
                let inside = (a.x..=a.x + a.w).contains(&cx) && (a.y..=a.y + a.h).contains(&cy);
                assert!(!inside, "a label covers the dot at ({cx}, {cy})");
            }
        }
    }

    #[test]
    fn crowded_labels_dodge_the_dots() {
        // London and Paris sit a few pixels apart at world scale: drawn
        // naively, London's label runs over Paris's dot.
        let s = ConnectionMapChart::new(vec![])
            .place("London", -0.1, 51.5)
            .place("Paris", 2.4, 48.9)
            .place("Brussels", 4.4, 50.8)
            .place("Sydney", 151.2, -33.9)
            .link("London", "Sydney", 1.0)
            .link("Paris", "Sydney", 1.0)
            .link("Brussels", "Sydney", 1.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        assert_clear(&s);
        assert!(labels_and_dots(&s).0.len() >= 3, "most names still fit");
    }

    #[test]
    fn stacked_labels_dodge_each_other() {
        // One place a label's height above another: the dots stay clear of
        // both labels, so only label against label can catch the clash.
        let s = ConnectionMapChart::new(vec![])
            .place("Upper", 0.0, 3.2)
            .place("Lower", 0.0, 0.0)
            .place("West", -100.0, -20.0)
            .place("East", 100.0, -20.0)
            .link("Upper", "Lower", 1.0)
            .link("Lower", "West", 1.0)
            .link("Lower", "East", 1.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let (_, dots) = labels_and_dots(&s);
        let dy = (dots[0].1 - dots[1].1).abs();
        assert!((2.0 * DOT_RADIUS..15.0).contains(&dy), "the setup: {dy}");
        assert_clear(&s);
        assert_eq!(
            labels_and_dots(&s).0.len(),
            4,
            "the hub Lower keeps the right side, Upper moves left"
        );
    }

    #[test]
    fn a_link_to_an_unknown_place_is_named() {
        let err = ConnectionMapChart::new(vec![])
            .place("A", 0.0, 0.0)
            .link("A", "B", 1.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap_err();
        assert!(err.to_string().contains("\"B\""), "{err}");
    }

    #[test]
    fn opposite_flows_curve_apart_and_bigger_flows_are_thicker() {
        let s = FlowMapChart::new(vec![])
            .place("A", 0.0, 0.0)
            .place("B", 40.0, 0.0)
            .link("A", "B", 10.0)
            .link("B", "A", 2.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let mid_y = |key: &str| {
            let Shape::Path { ops } = &keyed(&s, key).shape else {
                panic!()
            };
            let pts = &crate::scene::flatten(ops)[0];
            pts[pts.len() / 2].1
        };
        let (ab, ba) = (mid_y("flow-0"), mid_y("flow-1"));
        assert!((ab - ba).abs() > 20.0, "{ab} {ba}");
        let w = |key: &str| keyed(&s, key).stroke.unwrap().width;
        assert_eq!(w("flow-0"), FLOW_MAX);
        assert!(w("flow-1") < w("flow-0"));
    }

    #[test]
    fn the_arrowhead_tip_is_the_destination() {
        let s = FlowMapChart::new(vec![])
            .place("A", 0.0, 0.0)
            .place("B", 40.0, 10.0)
            .link("A", "B", 1.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let dest = keyed(&s, "place-1");
        let Shape::Circle { cx, cy, .. } = dest.shape else {
            panic!()
        };
        let tip = s.marks.iter().find_map(|m| match &m.shape {
            Shape::Path { ops } if m.fill.is_some() && m.stroke.is_none() => match ops[0] {
                PathOp::Move(x, y) => Some((x, y)),
                _ => None,
            },
            _ => None,
        });
        let tip = tip.unwrap();
        assert!((tip.0 - cx).abs() < 1e-9 && (tip.1 - cy).abs() < 1e-9);
    }
}
