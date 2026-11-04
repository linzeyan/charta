use super::frame::{self, seq_class, seq_legend};
use super::map::{BORDER, Fit, map_chart, regions};
use super::{Chart, PAD, check_size, count, describe, finite};
use crate::base::format;
use crate::base::text::TextMeasure;
use crate::base::theme::Color;
use crate::error::Error;

use crate::scene::{Hit, Mark, Scene, Shape};

/// A value for the region with this id.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Region {
    pub id: String,
    pub value: f64,
}

map_chart!(
    /// Regions shaded by value on the sequential ramp; regions without a
    /// value stay neutral.
    ChoroplethChart { values: Vec<Region> }
);

impl ChoroplethChart {
    pub fn value(mut self, id: impl Into<String>, value: f64) -> Self {
        self.values.push(Region {
            id: id.into(),
            value,
        });
        self
    }
}

impl Chart for ChoroplethChart {
    fn scene(&self, width: f64, height: f64, measure: &dyn TextMeasure) -> Result<Scene, Error> {
        check_size(width, height)?;
        let regions = regions(&self.features, self.basemap.as_deref())?;
        let mut value: Vec<Option<f64>> = vec![None; regions.len()];
        for (k, r) in self.values.iter().enumerate() {
            finite(&format!("the value for {:?}", r.id), r.value)?;
            if self.values[..k].iter().any(|o| o.id == r.id) {
                return Err(Error::InvalidData(format!(
                    "region {:?} has two values",
                    r.id
                )));
            }
            // An unmatched id is almost always a code mismatch (ISO 2 vs
            // 3 letters, a typo): silently dropping it would hide data.
            let i = regions
                .iter()
                .position(|f| f.id == r.id)
                .ok_or_else(|| Error::InvalidData(format!("no region has id {:?}", r.id)))?;
            value[i] = Some(r.value);
        }
        let (lo, hi) = self
            .values
            .iter()
            .fold((f64::INFINITY, f64::NEG_INFINITY), |(lo, hi), r| {
                (lo.min(r.value), hi.max(r.value))
            });

        let mut marks = Vec::new();
        let mut top = frame::title(&mut marks, self.title.as_deref());
        if !self.values.is_empty() {
            top = seq_legend(&mut marks, measure, top, lo, hi);
        }
        let fit = Fit::new(
            self.projection,
            &regions,
            &[],
            (PAD, top, width - PAD, height - PAD),
        );
        for (i, f) in regions.iter().enumerate() {
            let ops = fit.region(&f.geometry);
            if ops.is_empty() {
                continue;
            }
            let (fill, shown) = match value[i] {
                Some(v) => (Color::Seq(seq_class(v, lo, hi)), format::value(v)),
                None => (Color::Grid, "no data".to_string()),
            };
            marks.push(
                Mark::new(Shape::Path { ops })
                    .fill(fill)
                    .stroke(Color::Surface, BORDER)
                    .hit(Hit::new(format!("region-{i}"), f.name.as_str(), shown)),
            );
        }
        Ok(Scene {
            width,
            height,
            label: describe(
                self.title.as_deref(),
                &format!(
                    "Choropleth map of {}, {} with data.",
                    count(regions.len(), "region", "regions"),
                    self.values.len()
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
    use crate::geo::{Feature, Geometry};

    fn tiles() -> Vec<Feature> {
        (0..3)
            .map(|i| {
                let x = i as f64 * 10.0;
                Feature {
                    id: format!("T{i}"),
                    name: format!("Tile {i}"),
                    geometry: Geometry::Polygon(vec![vec![
                        (x, 0.0),
                        (x + 10.0, 0.0),
                        (x + 10.0, 10.0),
                        (x, 10.0),
                    ]]),
                }
            })
            .collect()
    }

    fn fill_of<'a>(s: &'a Scene, name: &str) -> (&'a Option<Color>, &'a str) {
        let m = s
            .marks
            .iter()
            .find(|m| m.hit.as_ref().is_some_and(|h| h.label == name))
            .unwrap();
        (&m.fill, m.hit.as_ref().unwrap().value.as_str())
    }

    #[test]
    fn values_pick_ramp_classes_and_missing_regions_stay_neutral() {
        let s = ChoroplethChart::new(tiles())
            .value("T0", 1.0)
            .value("T2", 8.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        assert_eq!(fill_of(&s, "Tile 0").0, &Some(Color::Seq(0)));
        assert_eq!(fill_of(&s, "Tile 2").0, &Some(Color::Seq(6)));
        assert_eq!(fill_of(&s, "Tile 1"), (&Some(Color::Grid), "no data"));
    }

    #[test]
    fn an_unknown_id_is_an_error_not_a_silent_gap() {
        let err = ChoroplethChart::new(tiles())
            .value("T9", 1.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap_err();
        assert!(err.to_string().contains("\"T9\""));
        assert!(
            ChoroplethChart::new(tiles())
                .value("T0", 1.0)
                .value("T0", 2.0)
                .scene(640.0, 400.0, &InterMetrics)
                .is_err()
        );
    }

    #[cfg(feature = "geo-world")]
    #[test]
    fn the_world_fits_inside_the_plot() {
        let s = ChoroplethChart::new(crate::geo::world())
            .value("BRA", 1.0)
            .scene(640.0, 400.0, &InterMetrics)
            .unwrap();
        for m in &s.marks {
            if let Shape::Path { ops } = &m.shape {
                for sub in crate::scene::flatten(ops) {
                    for (x, y) in sub {
                        assert!((PAD - 1e-6..=640.0 - PAD + 1e-6).contains(&x), "{x}");
                        assert!((PAD - 1e-6..=400.0 - PAD + 1e-6).contains(&y), "{y}");
                    }
                }
            }
        }
    }
}
