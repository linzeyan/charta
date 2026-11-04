//! Linear and band scales (the subset of d3-scale charts need so far).

use super::ticks;

/// Maps a continuous domain linearly onto a pixel range.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct LinearScale {
    pub domain: (f64, f64),
    pub range: (f64, f64),
}

impl LinearScale {
    pub fn new(domain: (f64, f64), range: (f64, f64)) -> Self {
        LinearScale { domain, range }
    }

    pub fn map(&self, v: f64) -> f64 {
        let ((d0, d1), (r0, r1)) = (self.domain, self.range);
        if d1 == d0 {
            return (r0 + r1) / 2.0;
        }
        r0 + (v - d0) / (d1 - d0) * (r1 - r0)
    }

    /// Extends the domain to round tick values. Requires `domain.0 <= domain.1`.
    pub fn nice(self, count: usize) -> Self {
        let domain = ticks::nice(self.domain.0, self.domain.1, count);
        LinearScale { domain, ..self }
    }

    pub fn ticks(&self, count: usize) -> Vec<f64> {
        ticks::ticks(self.domain.0, self.domain.1, count)
    }
}

/// Splits a pixel range into `n` equal bands with padding, like d3's
/// `scaleBand` (align fixed at the center).
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct BandScale {
    start: f64,
    step: f64,
    bandwidth: f64,
}

impl BandScale {
    /// `padding_inner` is the gap between bands and `padding_outer` the gap
    /// at each end, both as fractions of the step.
    pub fn new(n: usize, range: (f64, f64), padding_inner: f64, padding_outer: f64) -> Self {
        let (r0, r1) = range;
        let n = n as f64;
        let step = (r1 - r0) / (n - padding_inner + padding_outer * 2.0).max(1.0);
        let start = r0 + (r1 - r0 - step * (n - padding_inner)) * 0.5;
        BandScale {
            start,
            step,
            bandwidth: step * (1.0 - padding_inner),
        }
    }

    /// Left edge of band `i`.
    pub fn position(&self, i: usize) -> f64 {
        self.start + self.step * i as f64
    }

    pub fn bandwidth(&self) -> f64 {
        self.bandwidth
    }

    /// Distance from one band's start to the next.
    pub fn step(&self) -> f64 {
        self.step
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn linear_maps_domain_ends_to_range_ends() {
        let s = LinearScale::new((0.0, 50.0), (300.0, 100.0));
        assert_eq!(s.map(0.0), 300.0);
        assert_eq!(s.map(50.0), 100.0);
        assert_eq!(s.map(25.0), 200.0);
    }

    #[test]
    fn degenerate_domain_maps_to_middle_instead_of_nan() {
        assert_eq!(LinearScale::new((5.0, 5.0), (0.0, 10.0)).map(5.0), 5.0);
    }

    #[test]
    fn bands_are_equal_and_stay_inside_range() {
        let b = BandScale::new(4, (100.0, 500.0), 0.2, 0.1);
        let last_end = b.position(3) + b.bandwidth();
        assert!(b.position(0) >= 100.0 && last_end <= 500.0);
        // Centered: the leftover at both ends is the same.
        assert!(((b.position(0) - 100.0) - (500.0 - last_end)).abs() < 1e-9);
        assert!((b.position(1) - b.position(0) - b.step()).abs() < 1e-9);
    }
}
