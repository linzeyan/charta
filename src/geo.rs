//! Geographic input and map projections for the map charts.
//!
//! A [`Feature`]'s geometry has the shape of a GeoJSON geometry object, so
//! a GeoJSON FeatureCollection maps onto `Vec<Feature>` by lifting an id and
//! a name out of each feature's properties. The `geo-world` feature adds
//! [`world`], Natural Earth's 1:110m countries.

use crate::dmath;

/// One region: a country, a state.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
pub struct Feature {
    /// Matches data to the region (ISO 3166 alpha-3 in [`world`]).
    pub id: String,
    pub name: String,
    pub geometry: Geometry,
}

/// Polygons as rings of `(longitude, latitude)` in degrees: each polygon's
/// exterior ring, then its holes. Ring direction does not matter; holes
/// are recognized by position, not winding.
#[derive(Clone, Debug, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(tag = "type", content = "coordinates"))]
pub enum Geometry {
    Polygon(Vec<Vec<(f64, f64)>>),
    MultiPolygon(Vec<Vec<Vec<(f64, f64)>>>),
}

impl Geometry {
    /// Every polygon, exterior ring first.
    pub fn polygons(&self) -> Vec<&[Vec<(f64, f64)>]> {
        match self {
            Geometry::Polygon(p) => vec![p.as_slice()],
            Geometry::MultiPolygon(ps) => ps.iter().map(Vec::as_slice).collect(),
        }
    }
}

/// π / 180 as a literal: Go evaluates `math.Pi / 180` exactly before
/// rounding, Rust rounds π first, and the two can differ in the last bit.
pub(crate) const DEG: f64 = 0.017_453_292_519_943_295;
/// Web Mercator's cutoff, where the map becomes square; the poles are at
/// infinity.
const MERCATOR_LAT: f64 = 85.051_128_779_806_59;

/// How the globe is flattened.
#[derive(Clone, Copy, Debug, Default, PartialEq)]
#[cfg_attr(feature = "serde", derive(serde::Serialize, serde::Deserialize))]
#[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
pub enum Projection {
    /// Longitude and latitude as x and y: simple, distorts area and shape.
    Equirectangular,
    /// Conformal (shapes kept locally), area grossly inflated toward the
    /// poles; cut off at ±85.05° latitude.
    Mercator,
    /// A compromise for world maps: little distortion of either kind.
    #[default]
    NaturalEarth,
    /// Equal-area conic for mid-latitude regions (the USA: 29.5°, 45.5°,
    /// centered on −96°). Regional only: a feature straddling the meridian
    /// opposite `lon0` is torn across the map.
    #[cfg_attr(feature = "serde", serde(rename_all = "camelCase"))]
    Albers { lat1: f64, lat2: f64, lon0: f64 },
}

impl Projection {
    /// The projected point in unit-less map coordinates, y growing
    /// downward like the screen.
    pub fn project(&self, lon: f64, lat: f64) -> (f64, f64) {
        match *self {
            Projection::Equirectangular => (lon * DEG, -lat * DEG),
            Projection::Mercator => {
                let phi = lat.clamp(-MERCATOR_LAT, MERCATOR_LAT) * DEG;
                (
                    lon * DEG,
                    -dmath::ln(dmath::tan(std::f64::consts::FRAC_PI_4 + phi / 2.0)),
                )
            }
            Projection::NaturalEarth => {
                // Šavrič et al. (2015), as d3-geo's geoNaturalEarth1Raw:
                // polynomials only, no trigonometry.
                let (l, p) = (lon * DEG, lat * DEG);
                let p2 = p * p;
                let p4 = p2 * p2;
                let x = l
                    * (0.8707 - 0.131979 * p2
                        + p4 * (-0.013791 + p4 * (0.003971 * p2 - 0.001529 * p4)));
                let y = p
                    * (1.007226
                        + p2 * (0.015085 + p4 * (-0.044475 + 0.028874 * p2 - 0.005916 * p4)));
                (x, -y)
            }
            Projection::Albers { lat1, lat2, lon0 } => {
                // d3-geo's conicEqualAreaRaw.
                let mut l = lon - lon0;
                if l > 180.0 {
                    l -= 360.0;
                } else if l < -180.0 {
                    l += 360.0;
                }
                let sy0 = dmath::sin(lat1 * DEG);
                let n = (sy0 + dmath::sin(lat2 * DEG)) / 2.0;
                let c = 1.0 + sy0 * (2.0 * n - sy0);
                let r0 = c.sqrt() / n;
                let r = (c - 2.0 * n * dmath::sin(lat * DEG)).sqrt() / n;
                let (sa, ca) = dmath::sin_cos(l * DEG * n);
                (r * sa, -(r0 - r * ca))
            }
        }
    }
}

/// Points along the great circle from `a` to `b` (degrees), `steps`
/// segments, as d3-geo's geoInterpolate: the shortest route on the globe.
pub(crate) fn great_circle(a: (f64, f64), b: (f64, f64), steps: usize) -> Vec<(f64, f64)> {
    let (x0, y0, x1, y1) = (a.0 * DEG, a.1 * DEG, b.0 * DEG, b.1 * DEG);
    let ((sy0, cy0), (sy1, cy1)) = (dmath::sin_cos(y0), dmath::sin_cos(y1));
    let (kx0, ky0, kx1, ky1) = (
        cy0 * dmath::cos(x0),
        cy0 * dmath::sin(x0),
        cy1 * dmath::cos(x1),
        cy1 * dmath::sin(x1),
    );
    let hav = |v: f64| {
        let s = dmath::sin(v / 2.0);
        s * s
    };
    let d = 2.0 * dmath::asin((hav(y1 - y0) + cy0 * cy1 * hav(x1 - x0)).sqrt());
    if d == 0.0 {
        return vec![a; steps + 1];
    }
    let k = dmath::sin(d);
    (0..=steps)
        .map(|i| {
            let t = i as f64 / steps as f64 * d;
            let (bb, aa) = (dmath::sin(t) / k, dmath::sin(d - t) / k);
            let x = aa * kx0 + bb * kx1;
            let y = aa * ky0 + bb * ky1;
            let z = aa * sy0 + bb * sy1;
            (
                dmath::atan2(y, x) / DEG,
                dmath::atan2(z, (x * x + y * y).sqrt()) / DEG,
            )
        })
        .collect()
}

/// Natural Earth's 1:110m countries (public domain), id = ISO 3166-1
/// alpha-3 (Natural Earth's own code where there is none, e.g. "KOS").
#[cfg(feature = "geo-world")]
pub fn world() -> Vec<Feature> {
    static WORLD: std::sync::OnceLock<Vec<Feature>> = std::sync::OnceLock::new();
    WORLD
        .get_or_init(|| parse_basemap(include_str!("../geo/world/world-110m.txt")))
        .clone()
}

/// Reads the compact basemap format documented in docs/scripts/gen-world.py.
#[cfg(any(feature = "geo-world", test))]
fn parse_basemap(src: &str) -> Vec<Feature> {
    src.lines()
        .filter(|l| !l.is_empty() && !l.starts_with('#'))
        .map(|l| {
            let mut f = l.splitn(3, '\t');
            let (id, name, geo) = (f.next().unwrap(), f.next().unwrap(), f.next().unwrap());
            let polys: Vec<Vec<Vec<(f64, f64)>>> = geo
                .split('|')
                .map(|p| {
                    p.split(';')
                        .map(|r| {
                            r.split(' ')
                                .map(|pt| {
                                    let (x, y) = pt.split_once(',').expect("lon,lat");
                                    (x.parse().expect("lon"), y.parse().expect("lat"))
                                })
                                .collect()
                        })
                        .collect()
                })
                .collect();
            let geometry = if polys.len() == 1 {
                Geometry::Polygon(polys.into_iter().next().unwrap())
            } else {
                Geometry::MultiPolygon(polys)
            };
            Feature {
                id: id.into(),
                name: name.into(),
                geometry,
            }
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn projections_match_d3() {
        // Reference values from d3-geo 3 (y negated: screen y grows down).
        let (x, y) = Projection::NaturalEarth.project(10.0, 50.0);
        assert!((x - 0.1331534461932999).abs() < 1e-12, "{x}");
        assert!((y + 0.8790066371341401).abs() < 1e-12, "{y}");
        let (x, y) = Projection::Mercator.project(0.0, 60.0);
        assert!(x == 0.0 && (y + 1.316957896924816).abs() < 1e-12, "{y}");
        let albers = Projection::Albers {
            lat1: 29.5,
            lat2: 45.5,
            lon0: -96.0,
        };
        let (x, y) = albers.project(-80.0, 40.0);
        assert!((x - 0.2109801886836941).abs() < 1e-12, "{x}");
        assert!((y + 0.686842147796396).abs() < 1e-12, "{y}");
    }

    #[test]
    fn mercator_keeps_shapes_and_stops_short_of_the_pole() {
        // Conformal: at 60° a degree of latitude is stretched exactly as
        // much as a degree of longitude, 1 / cos 60° = 2.
        let m = Projection::Mercator;
        let dy = m.project(0.0, 60.0 - 1e-4).1 - m.project(0.0, 60.0 + 1e-4).1;
        let dx = m.project(1e-4, 60.0).0 - m.project(-1e-4, 60.0).0;
        assert!((dy / (2e-4 * DEG) - 2.0).abs() < 1e-6 && (dx / (2e-4 * DEG) - 1.0).abs() < 1e-12);
        assert!(m.project(0.0, 90.0).1.is_finite());
    }

    #[test]
    fn albers_keeps_areas() {
        // Two 1°×1° cells, one near each standard parallel and one at the
        // equator: an equal-area projection draws all three the same size.
        let p = Projection::Albers {
            lat1: 29.5,
            lat2: 45.5,
            lon0: -96.0,
        };
        let cell = |lon: f64, lat: f64| {
            let c = [
                p.project(lon, lat),
                p.project(lon + 1.0, lat),
                p.project(lon + 1.0, lat + 1.0),
                p.project(lon, lat + 1.0),
            ];
            (0..4)
                .map(|i| c[i].0 * c[(i + 1) % 4].1 - c[(i + 1) % 4].0 * c[i].1)
                .sum::<f64>()
                .abs()
                / 2.0
        };
        // Cells at 30° and 60° latitude span different true areas (cos φ);
        // compare each to its true spherical area instead.
        let true_area = |lat: f64| DEG * ((lat + 1.0) * DEG).sin() - DEG * (lat * DEG).sin();
        for (lon, lat) in [(-96.0, 30.0), (-80.0, 45.0), (-120.0, 0.0), (-100.0, 60.0)] {
            assert!(
                (cell(lon, lat) / true_area(lat) - 1.0).abs() < 1e-3,
                "{lon} {lat}"
            );
        }
    }

    #[test]
    fn a_great_circle_from_new_york_to_london_bows_north() {
        let path = great_circle((-74.0, 40.7), (-0.1, 51.5), 16);
        // Both ends go through the sphere and back, so within an ulp or so.
        assert!((path[0].0 + 74.0).abs() < 1e-9 && (path[0].1 - 40.7).abs() < 1e-9);
        assert!((path[16].0 + 0.1).abs() < 1e-9 && (path[16].1 - 51.5).abs() < 1e-9);
        // The shortest route arcs above the straight line on the map;
        // midpoint per d3-geo's geoInterpolate.
        assert!(
            (path[8].0 + 41.275706935569964).abs() < 1e-9,
            "{:?}",
            path[8]
        );
        assert!(
            (path[8].1 - 52.36254604648877).abs() < 1e-9,
            "{:?}",
            path[8]
        );
    }

    #[test]
    fn the_basemap_format_parses() {
        let f =
            parse_basemap("# c\nAAA\tA land\t0,0 1,0 1,1|5,5 6,5 6,6;5.2,5.2 5.8,5.2 5.8,5.8\n");
        assert_eq!(f.len(), 1);
        assert_eq!((f[0].id.as_str(), f[0].name.as_str()), ("AAA", "A land"));
        let Geometry::MultiPolygon(ps) = &f[0].geometry else {
            panic!()
        };
        assert_eq!(ps.len(), 2);
        assert_eq!(ps[1].len(), 2, "exterior plus one hole");
        assert_eq!(ps[1][1][0], (5.2, 5.2));
    }

    #[cfg(feature = "geo-world")]
    #[test]
    fn the_world_has_every_country_once() {
        let w = world();
        assert_eq!(w.len(), 177);
        let mut ids: Vec<&str> = w.iter().map(|f| f.id.as_str()).collect();
        ids.sort_unstable();
        ids.dedup();
        assert_eq!(ids.len(), 177);
        assert!(w.iter().any(|f| f.id == "FRA" && f.name == "France"));
    }
}
