use super::frame::{self, Key};
use super::map::{Fit, check_lon_lat, land, map_chart, regions};
use super::{Chart, MAX_SERIES, MIN_TARGET, PAD, SURFACE_GAP, check_size, count, describe, finite};
use crate::base::format;
use crate::base::text::TextMeasure;
use crate::base::theme::Color;
use crate::error::Error;
use crate::scene::{Hit, Mark, Rect, Scene, Shape};

/// Dots on a dot map are texture, not individually read markers.
const DOT_R: f64 = 2.0;
/// Smaller than the bubble chart's 24px: places on a map crowd together
/// far more than points on a scatter plot do.
const BUBBLE_MAX: f64 = 16.0;
const BUBBLE_MIN: f64 = 2.0;

/// A named location with a value.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Place {
    pub name: String,
    pub lon: f64,
    pub lat: f64,
    pub value: f64,
}

/// One dot, optionally in a group (colored, with a legend entry).
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Dot {
    pub lon: f64,
    pub lat: f64,
    #[cfg_attr(
        feature = "serde",
        serde(default, skip_serializing_if = "Option::is_none")
    )]
    pub group: Option<String>,
}

map_chart!(
    /// Circles on a map, area proportional to each place's value.
    ///
    /// # Example
    ///
    /// ```
    /// use charta::base::text::InterMetrics;
    /// use charta::{BubbleMapChart, Chart};
    ///
    /// // No regions: just the places, positioned by longitude and latitude.
    /// let chart = BubbleMapChart::new(Vec::new())
    ///     .title("Largest urban areas (M people)")
    ///     .place("Tokyo", 139.69, 35.69, 37.4)
    ///     .place("Delhi", 77.21, 28.61, 31.0)
    ///     .place("New York", -74.01, 40.71, 18.8);
    /// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
    /// // The scene's label is what a screen reader announces.
    /// assert_eq!(scene.label, "Largest urban areas (M people). Bubble map of 3 places.");
    /// ```
    BubbleMapChart { places: Vec<Place> }
);

map_chart!(
    /// One small dot per item at its location: density shows as clusters.
    ///
    /// # Example
    ///
    /// ```
    /// use charta::base::text::InterMetrics;
    /// use charta::{Chart, DotMapChart};
    ///
    /// // No regions: just the dots, positioned by longitude and latitude.
    /// let chart = DotMapChart::new(Vec::new())
    ///     .title("Strong earthquakes (sample)")
    ///     .dot(142.8, 36.4, "Shallow")
    ///     .dot(140.7, 36.7, "Deep")
    ///     .dot(-73.7, -33.2, "Shallow")
    ///     .dot(-151.9, 58.1, "Deep");
    /// let scene = chart.scene(640.0, 400.0, &InterMetrics).unwrap();
    /// // The scene's label is what a screen reader announces.
    /// assert_eq!(scene.label, "Strong earthquakes (sample). Dot map of 4 dots.");
    /// ```
    DotMapChart { dots: Vec<Dot> }
);

impl BubbleMapChart {
    pub fn place(mut self, name: impl Into<String>, lon: f64, lat: f64, value: f64) -> Self {
        self.places.push(Place {
            name: name.into(),
            lon,
            lat,
            value,
        });
        self
    }
}

impl DotMapChart {
    /// An empty `group` means none.
    pub fn dot(mut self, lon: f64, lat: f64, group: impl Into<String>) -> Self {
        let g: String = group.into();
        self.dots.push(Dot {
            lon,
            lat,
            group: (!g.is_empty()).then_some(g),
        });
        self
    }
}

impl Chart for BubbleMapChart {
    fn scene(&self, width: f64, height: f64, _: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let regions = regions(&self.features, self.basemap.as_deref())?;
        for p in &self.places {
            check_lon_lat(&format!("{:?}", p.name), p.lon, p.lat)?;
            finite(&format!("{:?}'s value", p.name), p.value)?;
            if p.value < 0.0 {
                return Err(Error::InvalidData(format!(
                    "{:?} has a negative value; a bubble's area cannot be",
                    p.name
                )));
            }
        }
        let mut marks = Vec::new();
        let top = frame::title(&mut marks, self.title.as_deref());
        let pts: Vec<(f64, f64)> = self.places.iter().map(|p| (p.lon, p.lat)).collect();
        let fit = Fit::new(
            self.projection,
            &regions,
            &pts,
            (PAD, top, width - PAD, height - PAD),
        );
        land(&mut marks, &fit, &regions);
        let max = self.places.iter().map(|p| p.value).fold(0.0, f64::max);
        let mut order: Vec<usize> = (0..self.places.len()).collect();
        // Big bubbles first, so small ones stay visible on top.
        order.sort_by(|&a, &b| self.places[b].value.total_cmp(&self.places[a].value));
        for i in order {
            let p = &self.places[i];
            let (cx, cy) = fit.xy(p.lon, p.lat);
            let r = if max > 0.0 {
                BUBBLE_MAX * (p.value / max).sqrt()
            } else {
                0.0
            }
            .max(BUBBLE_MIN);
            let mut hit = Hit::new(
                format!("place-{i}"),
                p.name.as_str(),
                format::value(p.value),
            );
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
                    .fill(Color::Series(0))
                    .stroke(Color::Surface, SURFACE_GAP)
                    .hit(hit),
            );
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Bubble map of {}.",
                    count(self.places.len(), "place", "places")
                ),
            ),
            marks,
        })
    }
}

impl Chart for DotMapChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let regions = regions(&self.features, self.basemap.as_deref())?;
        let mut groups: Vec<String> = Vec::new();
        let mut slot = Vec::with_capacity(self.dots.len());
        for (i, d) in self.dots.iter().enumerate() {
            check_lon_lat(&format!("dot {i}"), d.lon, d.lat)?;
            // JSON can say "group": "", which is no group, as in the builder.
            slot.push(d.group.as_ref().filter(|g| !g.is_empty()).map(|g| {
                match groups.iter().position(|x| x == g) {
                    Some(k) => k,
                    None => {
                        groups.push(g.clone());
                        groups.len() - 1
                    }
                }
            }));
        }
        if groups.len() > MAX_SERIES {
            return Err(Error::InvalidData(format!(
                "{} groups, at most {MAX_SERIES}: fold the rest into \"Other\"",
                groups.len()
            )));
        }
        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        top = frame::legend(&mut marks, measure, top, width, &groups, Key::Swatch);
        let pts: Vec<(f64, f64)> = self.dots.iter().map(|d| (d.lon, d.lat)).collect();
        let fit = Fit::new(
            self.projection,
            &regions,
            &pts,
            (PAD, top, width - PAD, height - PAD),
        );
        land(&mut marks, &fit, &regions);
        for (d, s) in self.dots.iter().zip(&slot) {
            let (cx, cy) = fit.xy(d.lon, d.lat);
            let mut m = Mark::new(Shape::Circle { cx, cy, r: DOT_R })
                .fill(Color::Series(s.unwrap_or(0) as u8));
            if let Some(k) = s {
                m = m.series(groups[*k].as_str());
            }
            marks.push(m);
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!("Dot map of {}.", count(self.dots.len(), "dot", "dots")),
            ),
            marks,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::base::text::InterMetrics;

    fn circles(s: &Scene) -> Vec<(f64, f64, f64, Option<String>)> {
        s.marks
            .iter()
            .filter_map(|m| match m.shape {
                Shape::Circle { cx, cy, r } => {
                    Some((cx, cy, r, m.hit.as_ref().map(|h| h.label.clone())))
                }
                _ => None,
            })
            .collect()
    }

    #[test]
    fn bubble_area_tracks_value_and_big_ones_sit_underneath() {
        let s = BubbleMapChart::new(vec![])
            .place("Small", 10.0, 10.0, 25.0)
            .place("Big", 20.0, 20.0, 100.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let c = circles(&s);
        assert_eq!(c[0].3.as_deref(), Some("Big"), "drawn first");
        assert!(
            (c[1].2 / c[0].2 - 0.5).abs() < 1e-9,
            "a quarter of the value, half the radius"
        );
    }

    #[test]
    fn places_land_where_the_projection_puts_them() {
        // East is right, north is up.
        let s = BubbleMapChart::new(vec![])
            .place("West", -10.0, 0.0, 1.0)
            .place("North east", 10.0, 20.0, 1.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let c = circles(&s);
        let (w, ne) = (
            c.iter().find(|x| x.3.as_deref() == Some("West")).unwrap(),
            c.iter()
                .find(|x| x.3.as_deref() == Some("North east"))
                .unwrap(),
        );
        assert!(ne.0 > w.0 && ne.1 < w.1);
    }

    #[test]
    fn dots_take_their_group_s_color() {
        let s = DotMapChart::new(vec![])
            .dot(0.0, 0.0, "a")
            .dot(1.0, 1.0, "b")
            .dot(2.0, 2.0, "a")
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        let fills: Vec<Option<Color>> = s
            .marks
            .iter()
            .filter(|m| matches!(m.shape, Shape::Circle { .. }))
            .map(|m| m.fill)
            .collect();
        assert_eq!(
            fills,
            [
                Some(Color::Series(0)),
                Some(Color::Series(1)),
                Some(Color::Series(0))
            ]
        );
        assert!(
            DotMapChart::new(vec![])
                .dot(200.0, 0.0, "")
                .scene(640.0, 400.0, &InterMetrics)
                .is_err()
        );
    }

    #[test]
    fn an_empty_group_is_no_group() {
        let mut c = DotMapChart::new(vec![])
            .dot(0.0, 0.0, "a")
            .dot(1.0, 1.0, "");
        let built = c.scene(640.0, 400.0, &InterMetrics).unwrap();
        c.dots[1].group = Some(String::new());
        assert_eq!(c.scene(640.0, 400.0, &InterMetrics).unwrap(), built);
    }
}
