use super::frame::{self, Key, ValueAxis, X_TICK_SPACING, hline, text, vline};
use super::{
    Chart, DOT_RADIUS, GAP, MIN_TARGET, PAD, Ranges, SURFACE_GAP, TICK_SIZE, TICK_SPACING,
    Viewable, check_size, count, describe, finite, snap, viewable,
};
use crate::base::format;
use crate::base::text::{TextMeasure, Weight};
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Anchor, Baseline, Hit, Mark, Plot, Rect, Scene, Shape};

/// Scatter and bubble charts compare every series against every other, so
/// the palette's all-pairs check applies: only its first three slots stay
/// distinguishable. Past three, fold into "Other" or facet.
const MAX_POINT_SERIES: usize = 3;
/// The largest bubble's radius.
const BUBBLE_MAX: f64 = 24.0;
/// A bubble never shrinks below a visible dot.
const BUBBLE_MIN: f64 = 2.0;

#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Point {
    pub x: f64,
    pub y: f64,
    /// Bubble size (area-proportional). Required by bubble charts, ignored by scatterplots.
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub size: Option<f64>,
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub label: Option<String>,
}

#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct PointSeries {
    pub name: String,
    pub points: Vec<Point>,
}

macro_rules! point_chart {
    ($(#[$doc:meta])* $name:ident) => {
        $(#[$doc])*
        #[derive(Clone, Debug, Default, PartialEq)]
        #[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
        #[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
        pub struct $name {
            #[cfg_attr(feature = "serde", serde(default, skip_serializing_if = "Option::is_none"))]
            pub title: Option<String>,
            /// Name of the x quantity, shown under the x axis.
            #[cfg_attr(feature = "serde", serde(default, skip_serializing_if = "Option::is_none"))]
            pub x_label: Option<String>,
            /// Name of the y quantity, shown above the y axis.
            #[cfg_attr(feature = "serde", serde(default, skip_serializing_if = "Option::is_none"))]
            pub y_label: Option<String>,
            pub series: Vec<PointSeries>,
        }

        impl $name {
            pub fn new() -> Self {
                Self::default()
            }

            pub fn title(mut self, title: impl Into<String>) -> Self {
                self.title = Some(title.into());
                self
            }

            pub fn axes(mut self, x: impl Into<String>, y: impl Into<String>) -> Self {
                self.x_label = Some(x.into());
                self.y_label = Some(y.into());
                self
            }

            pub fn series(mut self, name: impl Into<String>, points: Vec<Point>) -> Self {
                self.series.push(PointSeries { name: name.into(), points });
                self
            }
        }
    };
}

point_chart!(
    /// Points on two value axes.
    ScatterChart
);
point_chart!(
    /// A scatterplot whose point areas encode a third value.
    BubbleChart
);

impl Point {
    pub fn new(x: f64, y: f64) -> Self {
        Point {
            x,
            y,
            size: None,
            label: None,
        }
    }

    pub fn size(mut self, size: f64) -> Self {
        self.size = Some(size);
        self
    }

    pub fn label(mut self, label: impl Into<String>) -> Self {
        self.label = Some(label.into());
        self
    }
}

impl Viewable for ScatterChart {
    fn laid_out(
        &self,
        width: f64,
        height: f64,
        measure: &dyn TextMeasure,
        view: Option<Ranges>,
    ) -> Result<(Scene, Plot), Error> {
        build(false, self, (width, height), measure, view)
    }
}
viewable!(ScatterChart);

impl Viewable for BubbleChart {
    fn laid_out(
        &self,
        width: f64,
        height: f64,
        measure: &dyn TextMeasure,
        view: Option<Ranges>,
    ) -> Result<(Scene, Plot), Error> {
        let as_scatter = ScatterChart {
            title: self.title.clone(),
            x_label: self.x_label.clone(),
            y_label: self.y_label.clone(),
            series: self.series.clone(),
        };
        build(true, &as_scatter, (width, height), measure, view)
    }
}
viewable!(BubbleChart);

/// Lays the points out over all the data, niced, or over exactly the
/// ranges in `view`, showing the points inside them. Bubble sizes scale to
/// the biggest of all, so they keep their size as the view moves.
fn build(
    bubble: bool,
    c: &ScatterChart,
    (width, height): (f64, f64),
    measure: &dyn TextMeasure,
    view: Option<Ranges>,
) -> Result<(Scene, Plot), Error> {
    check_size(width, height)?;
    if c.series.len() > MAX_POINT_SERIES {
        return Err(Error::InvalidData(format!(
            "{} series, at most {MAX_POINT_SERIES} on a scatter or bubble chart: fold the rest into \"Other\" or use small multiples",
            c.series.len()
        )));
    }
    let mut max_size = 0.0_f64;
    for (j, s) in c.series.iter().enumerate() {
        if c.series[..j].iter().any(|o| o.name == s.name) {
            return Err(Error::InvalidData(format!(
                "series name {:?} is used twice",
                s.name
            )));
        }
        for (p, pt) in s.points.iter().enumerate() {
            finite(&format!("series {:?} point {p} x", s.name), pt.x)?;
            finite(&format!("series {:?} point {p} y", s.name), pt.y)?;
            if bubble {
                let size = pt.size.ok_or_else(|| {
                    Error::InvalidData(format!("series {:?} point {p} has no size", s.name))
                })?;
                if !(size.is_finite() && size >= 0.0) {
                    return Err(Error::InvalidData(format!(
                        "series {:?} point {p} size must be a non-negative number",
                        s.name
                    )));
                }
                max_size = max_size.max(size);
            }
        }
    }
    let all = || c.series.iter().flat_map(|s| s.points.iter());
    let extent = |f: fn(&Point) -> f64| {
        all().fold((f64::INFINITY, f64::NEG_INFINITY), |(lo, hi), p| {
            (lo.min(f(p)), hi.max(f(p)))
        })
    };
    let fix = |(lo, hi): (f64, f64)| if lo.is_finite() { (lo, hi) } else { (0.0, 0.0) };
    let (xlo, xhi) = view.map_or_else(|| fix(extent(|p| p.x)), |v| v.0);
    let (ylo, yhi) = match view {
        Some((_, Some(y))) => y,
        _ => fix(extent(|p| p.y)),
    };
    let axis = if view.is_some() {
        ValueAxis::exact
    } else {
        ValueAxis::new
    };

    let names: Vec<String> = c.series.iter().map(|s| s.name.clone()).collect();
    let mut marks = Vec::new();
    let mut top = frame::title(&mut marks, c.title.as_deref());
    top = frame::legend(&mut marks, measure, top, width, &names, Key::Swatch);
    if let Some(l) = &c.y_label {
        marks.push(text(
            PAD,
            top,
            l,
            TICK_SIZE,
            Weight::Regular,
            Anchor::Start,
            Baseline::Top,
            Color::TextMuted,
        ));
        top += TICK_SIZE + GAP;
    }
    let plot_top = top + TICK_SIZE / 2.0;
    let x_area = TICK_SIZE
        + GAP
        + if c.x_label.is_some() {
            TICK_SIZE + GAP
        } else {
            0.0
        };
    let bottom = (height - PAD - x_area).max(plot_top + 1.0);
    // The data scales stop short of the plot edges by the largest mark's
    // radius, so a point at an extreme never covers the axis or its labels.
    let inset = if bubble {
        BUBBLE_MAX
    } else {
        DOT_RADIUS + SURFACE_GAP
    };
    let count_y = (((bottom - plot_top - 2.0 * inset) / TICK_SPACING).round() as usize).max(2);
    let y = axis(ylo, yhi, (bottom - inset, plot_top + inset), count_y);
    let y_w = y
        .ticks
        .iter()
        .map(|&t| measure.width(&y.fmt.format(t), TICK_SIZE, Weight::Regular))
        .fold(0.0, f64::max);
    let left = PAD + y_w + GAP;
    let span_x = (width - PAD - left - 2.0 * inset).max(1.0);
    let count_x = ((span_x / X_TICK_SPACING).round() as usize).max(2);
    let mut x = axis(xlo, xhi, (left + inset, left + inset + span_x), count_x);
    let last = x.ticks.last().map_or(0.0, |&t| {
        measure.width(&x.fmt.format(t), TICK_SIZE, Weight::Regular)
    });
    let x_end = (width - PAD - (last / 2.0).max(inset)).max(left + inset + 1.0);
    x.scale.range.1 = x_end;
    let right = x_end + inset;

    // Axis lines sit on zero when the domain spans it, else on the plot edge.
    let spans_zero = |a: &ValueAxis| a.scale.domain.0 <= 0.0 && a.scale.domain.1 >= 0.0;
    let (x_zero, y_zero) = (spans_zero(&x), spans_zero(&y));
    for &t in &y.ticks {
        if !(y_zero && t == 0.0) {
            marks.push(hline(left, right, snap(y.scale.map(t)), Color::Grid));
        }
    }
    for &t in &x.ticks {
        if !(x_zero && t == 0.0) {
            marks.push(vline(snap(x.scale.map(t)), plot_top, bottom, Color::Grid));
        }
    }
    for &t in &y.ticks {
        marks.push(text(
            left - GAP,
            y.scale.map(t),
            &y.fmt.format(t),
            TICK_SIZE,
            Weight::Regular,
            Anchor::End,
            Baseline::Middle,
            Color::TextMuted,
        ));
    }
    for &t in &x.ticks {
        marks.push(text(
            x.scale.map(t),
            bottom + GAP,
            &x.fmt.format(t),
            TICK_SIZE,
            Weight::Regular,
            Anchor::Middle,
            Baseline::Top,
            Color::TextMuted,
        ));
    }
    let y_axis = if y_zero { y.scale.map(0.0) } else { bottom };
    let x_axis = if x_zero { x.scale.map(0.0) } else { left };
    marks.push(hline(left, right, snap(y_axis), Color::Axis));
    marks.push(vline(snap(x_axis), plot_top, bottom, Color::Axis));
    if let Some(l) = &c.x_label {
        marks.push(text(
            right,
            bottom + GAP + TICK_SIZE + GAP,
            l,
            TICK_SIZE,
            Weight::Regular,
            Anchor::End,
            Baseline::Top,
            Color::TextMuted,
        ));
    }

    // Big bubbles first, so small ones stay visible on top.
    let (dx, dy) = (x.scale.domain, y.scale.domain);
    let inside = |pt: &Point| {
        view.is_none() || (pt.x >= dx.0 && pt.x <= dx.1 && pt.y >= dy.0 && pt.y <= dy.1)
    };
    let mut order: Vec<(usize, usize, f64)> = Vec::new();
    for (j, s) in c.series.iter().enumerate() {
        for (p, pt) in s.points.iter().enumerate().filter(|(_, pt)| inside(pt)) {
            let r = if bubble {
                let size = pt.size.unwrap_or(0.0);
                let r = if max_size > 0.0 {
                    BUBBLE_MAX * (size / max_size).sqrt()
                } else {
                    0.0
                };
                r.max(BUBBLE_MIN)
            } else {
                DOT_RADIUS
            };
            order.push((j, p, r));
        }
    }
    if bubble {
        order.sort_by(|a, b| b.2.total_cmp(&a.2));
    }
    let n = order.len();
    let single = c.series.len() == 1;
    for (j, p, r) in order {
        let s = &c.series[j];
        let pt = &s.points[p];
        let (cx, cy) = (x.scale.map(pt.x), y.scale.map(pt.y));
        let mut label = match (&pt.label, single) {
            (Some(l), true) => l.clone(),
            (Some(l), false) => format!("{l} · {}", s.name),
            (None, _) => s.name.clone(),
        };
        if bubble {
            label = format!("{label} · size {}", format::value(pt.size.unwrap_or(0.0)));
        }
        let mut hit = Hit::new(
            format!("{j}-{p}"),
            label,
            format!("{}, {}", format::value(pt.x), format::value(pt.y)),
        );
        // A small dot is a pinpoint: give it a square target of MIN_TARGET.
        if r + SURFACE_GAP < MIN_TARGET / 2.0 {
            hit = hit.area(Rect {
                x: cx - MIN_TARGET / 2.0,
                y: cy - MIN_TARGET / 2.0,
                w: MIN_TARGET,
                h: MIN_TARGET,
            });
        }
        marks.push(
            Mark::new(Shape::Circle { cx, cy, r })
                .fill(Color::Series(j as u8))
                // The surface ring keeps overlapping dots legible.
                .stroke(Color::Surface, SURFACE_GAP)
                .series(s.name.as_str())
                .hit(hit),
        );
    }

    let what = if bubble {
        "Bubble chart"
    } else {
        "Scatterplot"
    };
    let plot = Plot {
        rect: Rect {
            x: x.scale.range.0,
            y: y.scale.range.1,
            w: x.scale.range.1 - x.scale.range.0,
            h: y.scale.range.0 - y.scale.range.1,
        },
        x: x.scale.domain,
        y: Some(y.scale.domain),
    };
    let scene = Scene {
        width,
        height,
        label: describe(
            c.title.as_deref(),
            &format!(
                "{what} with {} and {}.",
                count(c.series.len(), "series", "series"),
                count(n, "point", "points")
            ),
        ),
        marks,
    };
    Ok((scene, plot))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;

    fn dots(s: &Scene) -> Vec<(f64, f64, f64)> {
        s.marks
            .iter()
            .filter_map(|m| match m.shape {
                Shape::Circle { cx, cy, r } => Some((cx, cy, r)),
                _ => None,
            })
            .collect()
    }

    #[test]
    fn small_dots_get_a_full_size_hover_target() {
        let c = ScatterChart::new().series("a", vec![Point::new(1.0, 1.0), Point::new(5.0, 3.0)]);
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let (cx, cy, r) = dots(&s)[0];
        assert!(r * 2.0 >= 8.0, "markers are at least 8px");
        // 10px off-center is outside the dot but inside the target.
        let i = s.hit_test(cx + 10.0, cy, &[]).expect("hit");
        assert!(s.marks[i].hit.is_some());
    }

    #[test]
    fn bubble_area_not_radius_tracks_size() {
        let c = BubbleChart::new().series(
            "a",
            vec![
                Point::new(0.0, 0.0).size(100.0),
                Point::new(1.0, 1.0).size(25.0),
            ],
        );
        let s = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        let d = dots(&s);
        // Sorted big first; a quarter of the size is half the radius.
        assert!((d[0].2 / d[1].2 - 2.0).abs() < 1e-9);
        assert!(
            BubbleChart::new()
                .series("a", vec![Point::new(0.0, 0.0)])
                .scene(640.0, 400.0, &InterMetrics)
                .is_err()
        );
    }

    #[test]
    fn more_than_three_series_is_rejected() {
        let mut c = ScatterChart::new();
        for n in ["a", "b", "c", "d"] {
            c = c.series(n, vec![Point::new(0.0, 0.0)]);
        }
        assert!(c.scene(640.0, 400.0, &InterMetrics).is_err());
    }

    #[test]
    fn a_view_shows_the_points_inside_over_exactly_its_ranges() {
        let c = ScatterChart::new().series(
            "a",
            (0..10)
                .map(|i| Point::new(i as f64, (i * i) as f64))
                .collect(),
        );
        let (_, full) = c.plot_scene(640.0, 400.0, &InterMetrics).unwrap();
        assert_eq!(
            full.unwrap().y.map(|y| y.0),
            Some(0.0),
            "the full view is niced"
        );
        let (s, p) = c
            .view((2.0, 5.0), Some((0.0, 20.0)))
            .unwrap()
            .plot_scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let p = p.unwrap();
        assert_eq!((p.x, p.y), ((2.0, 5.0), Some((0.0, 20.0))));
        let dots: Vec<(f64, f64)> = s
            .marks
            .iter()
            .filter_map(|m| match m.shape {
                Shape::Circle { cx, cy, .. } => Some((cx, cy)),
                _ => None,
            })
            .collect();
        // (2, 4), (3, 9) and (4, 16); (5, 25) is above the view.
        assert_eq!(dots.len(), 3);
        for (cx, cy) in dots {
            assert!(p.rect.contains(cx, cy), "({cx}, {cy}) outside {:?}", p.rect);
        }
        // The point at x = 2 sits on the plot's left edge.
        assert!(
            s.marks.iter().any(
                |m| matches!(m.shape, Shape::Circle { cx, .. } if (cx - p.rect.x).abs() < 1e-9)
            )
        );
    }
}
