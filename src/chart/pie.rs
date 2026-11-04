use super::bar::Bar;
use super::frame::{self, Key, LEGEND_SIZE, text};
use super::polar::{at, sector};
use super::{
    Chart, GAP, HAIRLINE, MAX_SERIES, PAD, SURFACE_GAP, TICK_SIZE, check_size, count, describe,
    finite,
};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Hit, Mark, Scene, Shape, TipRow};
use std::f64::consts::TAU;

/// Leader line: radial run past the rim, then a horizontal run to the label.
const ELBOW: f64 = 12.0;
const RUN: f64 = 8.0;
const ROW: f64 = LEGEND_SIZE + 4.0;
/// Below this radius the outside labels cost the pie too much; the legend
/// alone names the slices.
const R_MIN: f64 = 60.0;
/// A donut's hole, as a share of its radius.
const HOLE: f64 = 0.6;
const TOTAL_SIZE: f64 = 22.0;

macro_rules! pie_chart {
    ($(#[$doc:meta])* $name:ident, $hole:expr) => {
        $(#[$doc])*
        #[derive(Clone, Debug, Default, PartialEq)]
        #[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
        pub struct $name {
            #[cfg_attr(
                feature = "serde",
                serde(default, skip_serializing_if = "Option::is_none")
            )]
            pub title: Option<String>,
            pub data: Vec<Bar>,
        }

        impl $name {
            pub fn new<L: Into<String>>(data: impl IntoIterator<Item = (L, f64)>) -> Self {
                $name {
                    title: None,
                    data: data
                        .into_iter()
                        .map(|(label, value)| Bar {
                            label: label.into(),
                            value,
                        })
                        .collect(),
                }
            }

            pub fn title(mut self, title: impl Into<String>) -> Self {
                self.title = Some(title.into());
                self
            }
        }

        impl Chart for $name {
            fn scene(
                &self,
                width: f64,
                height: f64,
                measure: &dyn TextMeasure,
            ) -> Result<Scene, Error> {
                build(self.title.as_deref(), &self.data, $hole, width, height, measure)
            }
        }
    };
}

pie_chart!(
    /// Shares of a whole as slices of a circle, clockwise from twelve
    /// o'clock in data order.
    PieChart,
    false
);
pie_chart!(
    /// A pie with its center cut out, where the total is shown.
    DonutChart,
    true
);

/// A share as a label: whole percents, with slivers kept visible as "<1".
fn percent(share: f64) -> String {
    let p = share * 100.0;
    if p > 0.0 && p < 1.0 {
        "<1%".into()
    } else {
        format!("{}%", p.round())
    }
}

/// A label beside the pie, joined to its slice by a leader line.
struct Label {
    slice: usize,
    mid: f64,
    y: f64,
}

/// Spreads one side's labels (sorted by y) at least a row apart within
/// `[lo, hi]`, each as close to where its slice points as it can be.
fn spread(labels: &mut [Label], lo: f64, hi: f64) {
    let mut prev = f64::NEG_INFINITY;
    for l in labels.iter_mut() {
        l.y = l.y.max(prev + ROW).max(lo);
        prev = l.y;
    }
    let mut next = f64::INFINITY;
    for l in labels.iter_mut().rev() {
        l.y = l.y.min(next - ROW).min(hi);
        next = l.y;
    }
}

fn build(
    title: Option<&str>,
    data: &[Bar],
    hole: bool,
    width: f64,
    height: f64,
    measure: &dyn TextMeasure,
) -> Result<Scene, Error> {
    check_size(width, height)?;
    if data.len() > MAX_SERIES {
        return Err(Error::InvalidData(format!(
            "{} slices, at most {MAX_SERIES}: fold the smallest into \"Other\"",
            data.len()
        )));
    }
    for (i, b) in data.iter().enumerate() {
        finite(&format!("slice {i} ({:?})", b.label), b.value)?;
        if b.value < 0.0 {
            return Err(Error::InvalidData(format!(
                "slice {i} ({:?}) is negative; a share cannot be",
                b.label
            )));
        }
        if data[..i].iter().any(|o| o.label == b.label) {
            return Err(Error::InvalidData(format!(
                "slice label {:?} is used twice",
                b.label
            )));
        }
    }
    let total: f64 = data.iter().map(|b| b.value).sum();
    if !data.is_empty() && total <= 0.0 {
        return Err(Error::InvalidData(
            "every slice is zero: there is no whole to divide".into(),
        ));
    }
    let kind = if hole { "Donut" } else { "Pie" };

    let names: Vec<String> = data.iter().map(|b| b.label.clone()).collect();
    let mut marks = Vec::new();
    let mut top = frame::title(&mut marks, title);
    top = frame::legend(&mut marks, measure, top, width, &names, Key::Swatch);
    let bottom = height - PAD;
    let (plot_w, plot_h) = ((width - 2.0 * PAD).max(1.0), (bottom - top).max(1.0));
    let (cx, cy) = (width / 2.0, (top + bottom) / 2.0);
    let texts: Vec<String> = data
        .iter()
        .map(|b| format!("{} {}", b.label, percent(b.value / total)))
        .collect();
    let widest = texts
        .iter()
        .map(|t| measure.width(t, LEGEND_SIZE, Weight::Regular))
        .fold(0.0, f64::max);
    let side = widest + ELBOW + RUN + GAP / 2.0;
    let with_labels = (plot_w / 2.0 - side).min(plot_h / 2.0 - ELBOW - ROW / 2.0);
    let labelled = with_labels >= R_MIN;
    let r = if labelled {
        with_labels
    } else {
        (plot_w.min(plot_h) / 2.0).max(1.0)
    };
    let r0 = if hole { r * HOLE } else { 0.0 };

    let mut cum = 0.0;
    let mut sides: [Vec<Label>; 2] = [Vec::new(), Vec::new()];
    for (i, b) in data.iter().enumerate() {
        let (a0, a1) = (TAU * cum / total, TAU * (cum + b.value) / total);
        cum += b.value;
        if b.value == 0.0 {
            continue;
        }
        marks.push(
            Mark::new(Shape::Path {
                ops: sector(cx, cy, r0, r, a0, a1),
            })
            .fill(Color::Series(i as u8))
            .stroke(Color::Surface, SURFACE_GAP)
            .series(b.label.as_str())
            .hit(
                Hit::new(i.to_string(), b.label.as_str(), format::value(b.value)).rows(vec![
                    TipRow {
                        color: None,
                        name: "share".into(),
                        value: format!("{}%", format::approx(b.value / total * 100.0)),
                    },
                ]),
            ),
        );
        let mid = (a0 + a1) / 2.0;
        let (s, c) = crate::dmath::sin_cos(mid);
        sides[usize::from(s < 0.0)].push(Label {
            slice: i,
            mid,
            y: cy - (r + ELBOW) * c,
        });
    }

    if labelled {
        // Labels always fit: at most MAX_SERIES slices, and a pie big enough
        // to be labelled (R_MIN) leaves more rows than that on each side.
        for (s, labels) in sides.iter_mut().enumerate() {
            labels.sort_by(|a, b| a.y.total_cmp(&b.y));
            spread(labels, top + ROW / 2.0, bottom - ROW / 2.0);
            let dir = if s == 0 { 1.0 } else { -1.0 };
            let tx = cx + dir * (r + ELBOW + RUN);
            for l in labels.iter() {
                let name = data[l.slice].label.as_str();
                marks.push(
                    Mark::new(Shape::Polyline {
                        points: vec![
                            at(cx, cy, r + 2.0, l.mid),
                            at(cx, cy, r + ELBOW, l.mid),
                            (tx - dir * GAP / 2.0, l.y),
                        ],
                    })
                    .stroke(Color::Axis, HAIRLINE)
                    .series(name),
                );
                marks.push(
                    text(
                        tx,
                        l.y,
                        &texts[l.slice],
                        LEGEND_SIZE,
                        Weight::Regular,
                        if s == 0 { Anchor::Start } else { Anchor::End },
                        Baseline::Middle,
                        Color::TextSecondary,
                    )
                    .series(name),
                );
            }
        }
    }

    if hole && !data.is_empty() {
        let label = format::value(total);
        if measure.width(&label, TOTAL_SIZE, Weight::Semibold) <= 1.6 * r0 {
            marks.push(text(
                cx,
                cy,
                &label,
                TOTAL_SIZE,
                Weight::Semibold,
                Anchor::Middle,
                Baseline::Bottom,
                Color::TextPrimary,
            ));
            marks.push(text(
                cx,
                cy + GAP,
                "Total",
                TICK_SIZE,
                Weight::Regular,
                Anchor::Middle,
                Baseline::Top,
                Color::TextMuted,
            ));
        }
    }

    let what = if hole {
        format!(
            "{kind} chart of {}, total {}.",
            count(data.len(), "slice", "slices"),
            format::value(total)
        )
    } else {
        format!("{kind} chart of {}.", count(data.len(), "slice", "slices"))
    };
    Ok(Scene {
        width,
        height,
        label: describe(title, &what),
        marks,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;
    use crate::scene::{PathOp, flatten};

    fn slices(s: &Scene) -> Vec<(&Vec<PathOp>, &Hit)> {
        s.marks
            .iter()
            .filter_map(|m| match (&m.shape, &m.hit) {
                (Shape::Path { ops }, Some(h)) => Some((ops, h)),
                _ => None,
            })
            .collect()
    }

    /// Polygon area of a flattened path (shoelace), which for a slice is
    /// its share of the disc.
    fn area(ops: &[PathOp]) -> f64 {
        flatten(ops)
            .iter()
            .map(|p| {
                (0..p.len())
                    .map(|k| {
                        let (a, b) = (p[k], p[(k + 1) % p.len()]);
                        a.0 * b.1 - b.0 * a.1
                    })
                    .sum::<f64>()
                    / 2.0
            })
            .sum::<f64>()
            .abs()
    }

    #[test]
    fn slice_areas_are_proportional_to_values() {
        // The one thing a pie must get right: area (and angle) is share.
        let s = PieChart::new([("a", 1.0), ("b", 2.0), ("c", 5.0)])
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let a: Vec<f64> = slices(&s).iter().map(|(ops, _)| area(ops)).collect();
        assert!((a[1] / a[0] - 2.0).abs() < 0.01, "{a:?}");
        assert!((a[2] / a[0] - 5.0).abs() < 0.01, "{a:?}");
    }

    #[test]
    fn the_first_slice_starts_at_twelve_o_clock_going_clockwise() {
        let s = PieChart::new([("a", 1.0), ("b", 3.0)])
            .scene(400.0, 400.0, &InterMetrics)
            .unwrap();
        let (ops, _) = slices(&s)[0];
        let (PathOp::Move(x, y), PathOp::Cubic(.., ex, _)) = (ops[0], ops[1]) else {
            panic!("{ops:?}")
        };
        let cx = 200.0;
        assert!((x - cx).abs() < 1e-9 && y < 200.0, "starts straight up");
        assert!(ex > cx, "runs to the right first");
    }

    #[test]
    fn labels_on_one_side_never_overlap() {
        // Many thin slices all crowd the right side's top.
        let s = PieChart::new([
            ("a", 1.0),
            ("b", 1.0),
            ("c", 1.0),
            ("d", 1.0),
            ("e", 1.0),
            ("f", 30.0),
        ])
        .scene(640.0, 400.0, &InterMetrics)
        .unwrap();
        let mut ys: Vec<(bool, f64)> = s
            .marks
            .iter()
            .filter_map(|m| match &m.shape {
                Shape::Text {
                    y, anchor, text, ..
                } if text.ends_with('%') => Some((*anchor == Anchor::Start, *y)),
                _ => None,
            })
            .collect();
        assert_eq!(ys.len(), 6);
        ys.sort_by(|a, b| a.partial_cmp(b).unwrap());
        for w in ys.windows(2) {
            if w[0].0 == w[1].0 {
                assert!(w[1].1 - w[0].1 >= ROW - 1e-9, "{ys:?}");
            }
        }
    }

    #[test]
    fn a_donut_has_a_hole_and_shows_the_total() {
        let s = DonutChart::new([("a", 40.0), ("b", 60.0)])
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let (cx, cy) = (320.0, (PAD + 400.0 - PAD) / 2.0);
        assert!(s.hit_test(cx, cy, &[]).is_none(), "the center is empty");
        assert!(s.marks.iter().any(|m| matches!(
            &m.shape,
            Shape::Text { text, .. } if text == "100"
        )));
    }

    #[test]
    fn tooltips_give_the_value_and_its_share() {
        let s = PieChart::new([("a", 1.0), ("b", 2.0)])
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let (_, h) = slices(&s)[1];
        assert_eq!((h.value.as_str(), h.rows[0].value.as_str()), ("2", "66.7%"));
    }

    #[test]
    fn negative_duplicate_and_all_zero_slices_are_rejected() {
        let m = &InterMetrics;
        assert!(PieChart::new([("a", -1.0)]).scene(640.0, 400.0, m).is_err());
        assert!(
            PieChart::new([("a", 1.0), ("a", 2.0)])
                .scene(640.0, 400.0, m)
                .is_err()
        );
        assert!(PieChart::new([("a", 0.0)]).scene(640.0, 400.0, m).is_err());
        let nine: Vec<(String, f64)> = (0..9).map(|i| (i.to_string(), 1.0)).collect();
        assert!(PieChart::new(nine).scene(640.0, 400.0, m).is_err());
    }

    #[test]
    fn slivers_read_as_under_one_percent() {
        assert_eq!(percent(0.004), "<1%");
        assert_eq!(percent(0.425), "43%");
        assert_eq!(percent(0.0), "0%");
    }
}
