use super::frame::{self, Key, text};
use super::graph::{Graph, Link, graph, links_chart, quad};
use super::polar::{arc, around, at, label_room, sector};
use super::{
    Chart, DOT_RADIUS, GAP, MAX_SERIES, PAD, SURFACE_GAP, TICK_SIZE, check_size, count, describe,
};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Hit, Mark, PathOp, Scene, Shape, TipRow};
use std::f64::consts::{PI, TAU};

const RING: f64 = 12.0;
/// Gap between neighboring groups on the ring, in radians.
const PAD_ANGLE: f64 = 0.04;
/// Link widths in the non-ribbon chord run from 1px to this.
const MAX_STROKE: f64 = 4.0;

links_chart!(
    /// Flows between a handful of groups: each group an arc as long as all
    /// it sends and receives, each pair a ribbon as wide as their exchange.
    ChordChart,
    true,
    chord
);

links_chart!(
    /// Connections between nodes on a circle, drawn as curves through the
    /// middle: which nodes connect, not how much flows.
    NonRibbonChordChart,
    false,
    non_ribbon
);

fn chord(
    title: Option<&str>,
    g: &Graph,
    width: f64,
    height: f64,
    measure: &dyn TextMeasure,
) -> Result<Scene, Error> {
    let n = g.names.len();
    // Ribbons are told apart by their group's color alone.
    if n > MAX_SERIES {
        return Err(Error::InvalidData(format!(
            "{n} groups, at most {MAX_SERIES}: fold the rest into \"Other\""
        )));
    }
    let mut m = vec![vec![0.0; n]; n];
    for &(a, b, v) in &g.edges {
        m[a][b] = v;
    }
    // A group's arc covers everything it exchanges, both directions; a
    // pair's ribbon is as wide as their two flows together at both ends.
    let pair = |i: usize, j: usize| m[i][j] + m[j][i];
    let totals: Vec<f64> = (0..n).map(|i| (0..n).map(|j| pair(i, j)).sum()).collect();
    let grand: f64 = totals.iter().sum();

    let mut marks = Vec::new();
    let mut top = frame::title(&mut marks, title);
    top = frame::legend(&mut marks, measure, top, width, &g.legend(), Key::Swatch);
    let (room_x, room_y) = label_room(measure, &g.names);
    let cx = width / 2.0;
    let cy = (top + height - PAD) / 2.0;
    let r = f64::min(
        width / 2.0 - PAD - room_x,
        (height - PAD - top) / 2.0 - room_y,
    )
    .max(RING + 1.0);
    let r_in = r - RING;
    let rr = r_in - SURFACE_GAP;

    // Each group's arc is split among its partners, nearest counterclockwise
    // first, so ribbons to neighbors leave from the facing ends and the
    // arcs do not twist across each other.
    let k = if grand > 0.0 {
        (TAU - PAD_ANGLE * n as f64) / grand
    } else {
        0.0
    };
    let mut group = vec![(0.0, 0.0); n];
    let mut end = vec![vec![(0.0, 0.0); n]; n];
    let mut a = PAD_ANGLE / 2.0;
    for i in 0..n {
        let a0 = a;
        let mut partners: Vec<usize> = (0..n).filter(|&j| j != i).collect();
        partners.sort_by_key(|&j| (i + n - j) % n);
        for j in partners {
            end[i][j] = (a, a + pair(i, j) * k);
            a += pair(i, j) * k;
        }
        group[i] = (a0, a);
        a += PAD_ANGLE;
    }

    for i in 0..n {
        for j in i + 1..n {
            if pair(i, j) == 0.0 {
                continue;
            }
            // Colored by whichever sends more; a tie goes to the first.
            let lead = if m[j][i] > m[i][j] { j } else { i };
            let (s0, s1) = end[i][j];
            let (t0, t1) = end[j][i];
            let start = at(cx, cy, rr, s0);
            let mut ops = vec![PathOp::Move(start.0, start.1)];
            arc(&mut ops, cx, cy, rr, s0, s1);
            quad(&mut ops, at(cx, cy, rr, s1), (cx, cy), at(cx, cy, rr, t0));
            arc(&mut ops, cx, cy, rr, t0, t1);
            quad(&mut ops, at(cx, cy, rr, t1), (cx, cy), start);
            ops.push(PathOp::Close);
            let row = |from: usize, to: usize| TipRow {
                color: Some(Color::Series(from as u8)),
                name: format!("{} → {}", g.names[from], g.names[to]),
                value: format::value(m[from][to]),
            };
            marks.push(
                Mark::new(Shape::Path { ops })
                    .fill(Color::Link(lead as u8))
                    .series(g.names[lead].as_str())
                    .hit(
                        Hit::new(
                            format!("ribbon-{i}-{j}"),
                            format!("{} ↔ {}", g.names[i], g.names[j]),
                            format::value(pair(i, j)),
                        )
                        .rows(vec![row(i, j), row(j, i)]),
                    ),
            );
        }
    }
    for i in 0..n {
        let (a0, a1) = group[i];
        let out: f64 = m[i].iter().sum();
        let inc: f64 = (0..n).map(|j| m[j][i]).sum();
        let tip = |name: &str, v: f64| TipRow {
            color: None,
            name: name.into(),
            value: format::value(v),
        };
        marks.push(
            Mark::new(Shape::Path {
                ops: sector(cx, cy, r_in, r, a0, a1),
            })
            .fill(Color::Series(i as u8))
            .series(g.names[i].as_str())
            .hit(
                Hit::new(
                    format!("group-{i}"),
                    g.names[i].as_str(),
                    format::value(totals[i]),
                )
                .rows(vec![tip("out", out), tip("in", inc)]),
            ),
        );
        let (x, y, anchor, baseline) = around(cx, cy, r + GAP / 2.0, (a0 + a1) / 2.0);
        marks.push(text(
            x,
            y,
            &g.names[i],
            TICK_SIZE,
            Weight::Regular,
            anchor,
            baseline,
            Color::TextSecondary,
        ));
    }
    Ok(Scene {
        width,
        height,
        label: describe(
            title,
            &format!(
                "Chord diagram of {} among {}.",
                count(g.edges.len(), "flow", "flows"),
                count(n, "group", "groups")
            ),
        ),
        marks,
    })
}

fn non_ribbon(
    title: Option<&str>,
    g: &Graph,
    width: f64,
    height: f64,
    measure: &dyn TextMeasure,
) -> Result<Scene, Error> {
    let n = g.names.len();
    let mut marks = Vec::new();
    let mut top = frame::title(&mut marks, title);
    top = frame::legend(&mut marks, measure, top, width, &g.legend(), Key::Swatch);
    // Labels run outward along the radius, so however many nodes there are
    // each needs only a line's width of the circumference.
    let longest = g
        .names
        .iter()
        .map(|s| measure.width(s, TICK_SIZE, Weight::Regular))
        .fold(0.0, f64::max);
    let room = DOT_RADIUS + GAP / 2.0 + longest;
    let cx = width / 2.0;
    let cy = (top + height - PAD) / 2.0;
    let r = f64::min(width / 2.0 - PAD, (height - PAD - top) / 2.0) - room;
    let r = r.max(DOT_RADIUS + 1.0);
    let angle = |i: usize| i as f64 * TAU / n as f64;
    let series = |i: usize| g.colored().then(|| g.names[i].clone());
    let strength = g.strength();
    let max = g.edges.iter().map(|e| e.2).fold(0.0, f64::max);

    for (k, &(a, b, v)) in g.edges.iter().enumerate() {
        let (p, q) = (at(cx, cy, r, angle(a)), at(cx, cy, r, angle(b)));
        let mut ops = vec![PathOp::Move(p.0, p.1)];
        quad(&mut ops, p, (cx, cy), q);
        let mut m = Mark::new(Shape::Path { ops })
            .stroke(Color::Link(g.color(a)), 1.0 + (MAX_STROKE - 1.0) * v / max)
            .hit(Hit::new(
                format!("link-{k}"),
                format!("{} – {}", g.names[a], g.names[b]),
                format::value(v),
            ));
        if let Some(s) = series(a) {
            m = m.series(s);
        }
        marks.push(m);
    }
    for (i, name) in g.names.iter().enumerate() {
        let a = angle(i);
        let (x, y) = at(cx, cy, r, a);
        let links = g.edges.iter().filter(|e| e.0 == i || e.1 == i).count();
        let mut m = Mark::new(Shape::Circle {
            cx: x,
            cy: y,
            r: DOT_RADIUS,
        })
        .fill(Color::Series(g.color(i)))
        .stroke(Color::Surface, SURFACE_GAP)
        .hit(
            Hit::new(
                format!("node-{i}"),
                name.as_str(),
                format::value(strength[i]),
            )
            .rows(vec![TipRow {
                color: None,
                name: "links".into(),
                value: links.to_string(),
            }]),
        );
        if let Some(s) = series(i) {
            m = m.series(s);
        }
        marks.push(m);
        // Right half reads outward left to right; the left half is flipped
        // so no label is upside down.
        let (lx, ly) = at(cx, cy, r + DOT_RADIUS + GAP / 2.0, a);
        let deg = a * 180.0 / PI;
        let (anchor, rot) = if a <= PI {
            (Anchor::Start, deg - 90.0)
        } else {
            (Anchor::End, deg - 270.0)
        };
        let mut t = text(
            lx,
            ly,
            name,
            TICK_SIZE,
            Weight::Regular,
            anchor,
            Baseline::Middle,
            Color::TextSecondary,
        );
        if let Shape::Text { angle, .. } = &mut t.shape {
            *angle = rot;
        }
        marks.push(t);
    }
    Ok(Scene {
        width,
        height,
        label: describe(
            title,
            &format!(
                "Non-ribbon chord diagram of {} among {}.",
                count(g.edges.len(), "link", "links"),
                count(n, "node", "nodes")
            ),
        ),
        marks,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;
    use crate::scene::HitKind;

    fn hits<'a>(s: &'a Scene, prefix: &str) -> Vec<&'a Mark> {
        s.marks
            .iter()
            .filter(|m| {
                m.hit
                    .as_ref()
                    .is_some_and(|h| h.kind == HitKind::Mark && h.key.starts_with(prefix))
            })
            .collect()
    }

    fn migration() -> ChordChart {
        ChordChart::new()
            .link("North", "South", 30.0)
            .link("South", "North", 10.0)
            .link("North", "East", 20.0)
            .link("East", "West", 15.0)
            .link("West", "South", 25.0)
    }

    #[test]
    fn arcs_are_as_long_as_each_group_s_exchange() {
        let s = migration().scene(640.0, 480.0, &InterMetrics).unwrap();
        // A ring sector's area grows linearly with its angle.
        let area = |label: &str| {
            let m = hits(&s, "group")
                .into_iter()
                .find(|m| m.hit.as_ref().unwrap().label == label)
                .unwrap();
            let Shape::Path { ops } = &m.shape else {
                panic!()
            };
            let p = &crate::scene::flatten(ops)[0];
            p.windows(2)
                .map(|w| w[0].0 * w[1].1 - w[1].0 * w[0].1)
                .sum::<f64>()
                .abs()
                / 2.0
        };
        // North exchanges 30 + 10 + 20 = 60, East 20 + 15 = 35.
        assert!((area("North") / area("East") - 60.0 / 35.0).abs() < 1e-3);
    }

    #[test]
    fn a_ribbon_takes_the_color_of_the_bigger_sender() {
        let s = migration().scene(640.0, 480.0, &InterMetrics).unwrap();
        let r = hits(&s, "ribbon")
            .into_iter()
            .find(|m| m.hit.as_ref().unwrap().label == "North ↔ South")
            .unwrap();
        // North (slot 0) sends 30, South (slot 1) sends 10.
        assert_eq!(r.fill, Some(Color::Link(0)));
        assert_eq!(r.hit.as_ref().unwrap().value, "40");
        let w = hits(&s, "ribbon")
            .into_iter()
            .find(|m| m.hit.as_ref().unwrap().label == "South ↔ West")
            .unwrap();
        assert_eq!(w.fill, Some(Color::Link(3)), "West sends to South");
    }

    #[test]
    fn hovering_inside_a_ribbon_finds_it() {
        let s = ChordChart::new()
            .link("A", "B", 1.0)
            .scene(400.0, 400.0, &InterMetrics)
            .unwrap();
        // Two groups, one ribbon: the middle of the circle is the ribbon.
        // The legend (one row) sits above the circle.
        let top = PAD + 16.0 + GAP;
        let (cx, cy) = (200.0, (top + 400.0 - PAD) / 2.0);
        let i = s.hit_test(cx, cy, &[]).unwrap();
        assert_eq!(s.marks[i].hit.as_ref().unwrap().label, "A ↔ B");
    }

    #[test]
    fn a_ninth_group_is_refused() {
        let mut c = ChordChart::new();
        for i in 1..9 {
            c = c.link("hub", format!("g{i}"), 1.0);
        }
        assert!(c.scene(640.0, 480.0, &InterMetrics).is_err());
    }

    #[test]
    fn non_ribbon_labels_never_read_upside_down() {
        let mut c = NonRibbonChordChart::new();
        for i in 1..12 {
            c = c.link("n0", format!("n{i}"), i as f64);
        }
        let s = c.scene(640.0, 480.0, &InterMetrics).unwrap();
        for m in &s.marks {
            if let Shape::Text { angle, .. } = m.shape {
                assert!((-90.0..=90.0).contains(&angle), "{angle}");
            }
        }
        // Heavier links draw thicker.
        let w = |k: &str| {
            s.marks
                .iter()
                .find(|m| m.hit.as_ref().is_some_and(|h| h.key == k))
                .unwrap()
                .stroke
                .unwrap()
                .width
        };
        assert!(w("link-10") > w("link-0"));
        assert_eq!(w("link-10"), MAX_STROKE);
    }
}
