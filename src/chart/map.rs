//! Pieces shared by the map charts: the basemap, fitting a projection to
//! the plot, and regions as paths.

use super::finite;
use crate::base::theme::Color;
use crate::error::Error;
use crate::geo::{Feature, Geometry, Projection};
use crate::scene::{Mark, PathOp, Shape};
use std::borrow::Cow;

/// Country borders: thinner than a hairline, at world scale a full pixel
/// turns small countries into blobs of outline.
pub(crate) const BORDER: f64 = 0.5;

/// The regions to draw: the chart's own, or a named basemap (JSON specs
/// say `"basemap": "world"`), not both.
pub(crate) fn regions<'a>(
    own: &'a [Feature],
    basemap: Option<&str>,
) -> Result<Cow<'a, [Feature]>, Error> {
    // "" is no basemap, as in the Go port where the field is a plain string.
    match basemap.filter(|b| !b.is_empty()) {
        None => Ok(Cow::Borrowed(own)),
        Some(_) if !own.is_empty() => Err(Error::InvalidData(
            "give either features or a basemap, not both".into(),
        )),
        #[cfg(feature = "geo-world")]
        Some("world") => Ok(Cow::Owned(crate::geo::world())),
        #[cfg(not(feature = "geo-world"))]
        Some("world") => Err(Error::InvalidData(
            "basemap \"world\" needs the geo-world feature".into(),
        )),
        Some(other) => Err(Error::InvalidData(format!(
            "unknown basemap {other:?}; the built-in one is \"world\""
        ))),
    }
}

/// Longitude and latitude in degrees, in range.
pub(crate) fn check_lon_lat(what: &str, lon: f64, lat: f64) -> Result<(), Error> {
    finite(&format!("{what}'s longitude"), lon)?;
    finite(&format!("{what}'s latitude"), lat)?;
    if !(-180.0..=180.0).contains(&lon) || !(-90.0..=90.0).contains(&lat) {
        return Err(Error::InvalidData(format!(
            "{what} at ({lon}, {lat}) is off the globe: longitude runs −180 to 180, latitude −90 to 90"
        )));
    }
    Ok(())
}

/// A projection scaled and centered into the plot box.
pub(crate) struct Fit {
    proj: Projection,
    k: f64,
    ox: f64,
    oy: f64,
}

impl Fit {
    /// Fits the projected bounds of `regions` and `points` (lon, lat) into
    /// `(left, top, right, bottom)`, keeping the aspect ratio.
    pub fn new(
        proj: Projection,
        regions: &[Feature],
        points: &[(f64, f64)],
        (left, top, right, bottom): (f64, f64, f64, f64),
    ) -> Self {
        let (mut x0, mut y0, mut x1, mut y1) = (
            f64::INFINITY,
            f64::INFINITY,
            f64::NEG_INFINITY,
            f64::NEG_INFINITY,
        );
        let mut grow = |(lon, lat): (f64, f64)| {
            let (x, y) = proj.project(lon, lat);
            x0 = x0.min(x);
            y0 = y0.min(y);
            x1 = x1.max(x);
            y1 = y1.max(y);
        };
        for f in regions {
            for poly in f.geometry.polygons() {
                // Holes lie inside their exterior.
                for &p in poly.first().into_iter().flatten() {
                    grow(p);
                }
            }
        }
        for &p in points {
            grow(p);
        }
        if x0 > x1 {
            (x0, y0, x1, y1) = (0.0, 0.0, 0.0, 0.0);
        }
        let (w, h) = ((right - left).max(1.0), (bottom - top).max(1.0));
        let (bw, bh) = (x1 - x0, y1 - y0);
        let k = if bw > 0.0 || bh > 0.0 {
            f64::min(
                if bw > 0.0 { w / bw } else { f64::INFINITY },
                if bh > 0.0 { h / bh } else { f64::INFINITY },
            )
        } else {
            1.0
        };
        Fit {
            proj,
            k,
            ox: (left + right) / 2.0 - (x0 + x1) / 2.0 * k,
            oy: (top + bottom) / 2.0 - (y0 + y1) / 2.0 * k,
        }
    }

    pub fn xy(&self, lon: f64, lat: f64) -> (f64, f64) {
        let (x, y) = self.proj.project(lon, lat);
        (self.ox + x * self.k, self.oy + y * self.k)
    }

    /// A region as one path, each polygon's exterior wound one way and its
    /// holes the other, whatever the input's winding: the nonzero fill rule
    /// then leaves the holes empty.
    pub fn region(&self, g: &Geometry) -> Vec<PathOp> {
        let mut ops = Vec::new();
        for poly in g.polygons() {
            for (r, ring) in poly.iter().enumerate() {
                let mut pts: Vec<(f64, f64)> =
                    ring.iter().map(|&(lon, lat)| self.xy(lon, lat)).collect();
                if pts.len() < 3 {
                    continue;
                }
                let area: f64 = (0..pts.len())
                    .map(|i| {
                        let (a, b) = (pts[i], pts[(i + 1) % pts.len()]);
                        a.0 * b.1 - b.0 * a.1
                    })
                    .sum();
                if (area < 0.0) == (r == 0) {
                    pts.reverse();
                }
                ops.push(PathOp::Move(pts[0].0, pts[0].1));
                ops.extend(pts[1..].iter().map(|&(x, y)| PathOp::Line(x, y)));
                ops.push(PathOp::Close);
            }
        }
        ops
    }
}

/// The regions as a quiet backdrop for marks placed on the map.
pub(crate) fn land(marks: &mut Vec<Mark>, fit: &Fit, regions: &[Feature]) {
    for f in regions {
        let ops = fit.region(&f.geometry);
        if !ops.is_empty() {
            marks.push(
                Mark::new(Shape::Path { ops })
                    .fill(Color::Grid)
                    .stroke(Color::Surface, BORDER),
            );
        }
    }
}

/// A map chart: title, the regions (own features or a named basemap), the
/// projection, and the chart's own data fields with their types.
macro_rules! map_chart {
    ($(#[$doc:meta])* $name:ident { $($field:ident: $ty:ty),+ }) => {
        $(#[$doc])*
        #[derive(Clone, Debug, Default, PartialEq)]
        #[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
        pub struct $name {
            #[cfg_attr(
                feature = "serde",
                serde(default, skip_serializing_if = "Option::is_none")
            )]
            pub title: Option<String>,
            #[cfg_attr(feature = "serde", serde(default, skip_serializing_if = "Vec::is_empty"))]
            pub features: Vec<$crate::geo::Feature>,
            /// A built-in map instead of `features`: "world" (needs `geo-world`).
            #[cfg_attr(
                feature = "serde",
                serde(default, skip_serializing_if = "Option::is_none")
            )]
            pub basemap: Option<String>,
            #[cfg_attr(feature = "serde", serde(default))]
            pub projection: $crate::geo::Projection,
            $(pub $field: $ty,)+
        }

        impl $name {
            pub fn new(features: Vec<$crate::geo::Feature>) -> Self {
                $name {
                    features,
                    ..Default::default()
                }
            }

            pub fn title(mut self, title: impl Into<String>) -> Self {
                self.title = Some(title.into());
                self
            }

            pub fn projection(mut self, p: $crate::geo::Projection) -> Self {
                self.projection = p;
                self
            }
        }
    };
}

pub(crate) use map_chart;

#[cfg(test)]
mod tests {
    use super::*;
    use crate::scene::flatten;

    fn square(x: f64, y: f64, s: f64, clockwise: bool) -> Vec<(f64, f64)> {
        let mut r = vec![(x, y), (x + s, y), (x + s, y + s), (x, y + s)];
        if clockwise {
            r.reverse();
        }
        r
    }

    fn signed(p: &[(f64, f64)]) -> f64 {
        (0..p.len() - 1)
            .map(|i| p[i].0 * p[i + 1].1 - p[i + 1].0 * p[i].1)
            .sum()
    }

    #[test]
    fn holes_wind_against_their_exterior_whatever_the_input() {
        // Both rings given the same way round, as sloppy GeoJSON does: the
        // hole must still come out opposite so nonzero filling skips it.
        let fit = Fit::new(
            Projection::Equirectangular,
            &[],
            &[(0.0, 0.0), (10.0, 10.0)],
            (0.0, 0.0, 100.0, 100.0),
        );
        let g = Geometry::Polygon(vec![
            square(0.0, 0.0, 10.0, false),
            square(4.0, 4.0, 2.0, false),
        ]);
        let subs = flatten(&fit.region(&g));
        assert!(signed(&subs[0]) * signed(&subs[1]) < 0.0);
    }

    #[test]
    fn the_fit_fills_one_side_and_centers_the_other() {
        // A 20°×10° box into a 200×200 plot: width-bound, centered vertically.
        let fit = Fit::new(
            Projection::Equirectangular,
            &[],
            &[(0.0, 0.0), (20.0, 10.0)],
            (0.0, 0.0, 200.0, 200.0),
        );
        let (a, b) = (fit.xy(0.0, 10.0), fit.xy(20.0, 0.0));
        assert!((a.0 - 0.0).abs() < 1e-9 && (b.0 - 200.0).abs() < 1e-9);
        assert!(((a.1 + b.1) / 2.0 - 100.0).abs() < 1e-9);
    }

    #[test]
    fn a_basemap_and_own_features_cannot_mix() {
        let own = [Feature {
            id: "X".into(),
            name: "X".into(),
            geometry: Geometry::Polygon(vec![square(0.0, 0.0, 1.0, false)]),
        }];
        assert!(regions(&own, Some("world")).is_err());
        assert_eq!(regions(&own, Some("")).unwrap().len(), 1);
        // A polygon with no rings is drawn as nothing, not a panic.
        let empty = Feature {
            id: "E".into(),
            name: "E".into(),
            geometry: Geometry::Polygon(vec![]),
        };
        let fit = Fit::new(
            Projection::Equirectangular,
            std::slice::from_ref(&empty),
            &[],
            (0.0, 0.0, 10.0, 10.0),
        );
        assert!(fit.region(&empty.geometry).is_empty());
        assert!(regions(&[], Some("mars")).is_err());
        assert!(check_lon_lat("x", 181.0, 0.0).is_err());
        assert!(check_lon_lat("x", 0.0, -91.0).is_err());
    }
}
