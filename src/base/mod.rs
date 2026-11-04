//! Building blocks shared by every chart: scales, ticks, number formatting,
//! text measurement and the color theme.
//!
//! Each item has a Go twin in `base/` that must return identical values, so
//! the numeric code avoids anything the two languages define differently
//! (library `log10`/`pow` last-ulp results, rounding modes, float printing).

pub mod format;
pub mod scale;
pub mod text;
pub mod theme;
pub mod ticks;
pub mod time;
