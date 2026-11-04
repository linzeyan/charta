//! Elementary functions that return the same bits in Rust and Go.
//!
//! `f64::sin` and friends call the platform libm, which differs between
//! macOS and glibc in the last ulp, and Go's `math` differs from both (and
//! between amd64 and arm64, where Go fuses `a*b+c` into FMA). A last-ulp
//! difference in an angle can move a scene coordinate across a 0.01
//! rounding boundary, so every layout path calls these instead.
//!
//! The algorithms are musl's (fdlibm, © Sun Microsystems, "freely
//! distributable"), using only IEEE basic operations and `sqrt`, which are
//! correctly rounded everywhere; the Go twin (`internal/dmath`) rounds every
//! product before it is added so no step fuses. Argument reduction for the
//! trigonometric functions is fdlibm's medium path (exact to ~1e6 radians;
//! beyond that the result stays deterministic but loses accuracy, far past
//! any angle a chart computes).

// The constants are fdlibm's, digit for digit, so they can be checked
// against the source; a few equal std consts (PI, LOG2_E), kept as written
// for the same reason.
#![allow(clippy::excessive_precision, clippy::approx_constant)]

const PIO2_HI: f64 = 1.57079632679489655800e+00;
const PIO2_LO: f64 = 6.12323399573676603587e-17;

fn high(x: f64) -> u32 {
    (x.to_bits() >> 32) as u32
}

fn low(x: f64) -> u32 {
    x.to_bits() as u32
}

/// `x` with its low 32 bits cleared.
fn clear_low(x: f64) -> f64 {
    f64::from_bits(x.to_bits() & 0xffff_ffff_0000_0000)
}

fn kernel_sin(x: f64, y: f64, iy: bool) -> f64 {
    const S1: f64 = -1.66666666666666324348e-01;
    const S2: f64 = 8.33333333332248946124e-03;
    const S3: f64 = -1.98412698298579493134e-04;
    const S4: f64 = 2.75573137070700676789e-06;
    const S5: f64 = -2.50507602534068634195e-08;
    const S6: f64 = 1.58969099521155010221e-10;
    let z = x * x;
    let w = z * z;
    let r = S2 + z * (S3 + z * S4) + z * w * (S5 + z * S6);
    let v = z * x;
    if !iy {
        x + v * (S1 + z * r)
    } else {
        x - ((z * (0.5 * y - v * r) - y) - v * S1)
    }
}

fn kernel_cos(x: f64, y: f64) -> f64 {
    const C1: f64 = 4.16666666666666019037e-02;
    const C2: f64 = -1.38888888888741095749e-03;
    const C3: f64 = 2.48015872894767294178e-05;
    const C4: f64 = -2.75573143513906633035e-07;
    const C5: f64 = 2.08757232129817482790e-09;
    const C6: f64 = -1.13596475577881948265e-11;
    let z = x * x;
    let w = z * z;
    let r = z * (C1 + z * (C2 + z * C3)) + w * w * (C4 + z * (C5 + z * C6));
    let hz = 0.5 * z;
    let w = 1.0 - hz;
    w + (((1.0 - w) - hz) + (z * r - x * y))
}

/// `x` reduced by the nearest multiple `n` of π/2: (n mod 4, head, tail).
fn rem_pio2(x: f64) -> (u32, f64, f64) {
    const INVPIO2: f64 = 6.36619772367581382433e-01;
    const PIO2_1: f64 = 1.57079632673412561417e+00;
    const PIO2_1T: f64 = 6.07710050650619224932e-11;
    const PIO2_2: f64 = 6.07710050630396597660e-11;
    const PIO2_2T: f64 = 2.02226624879595063154e-21;
    const PIO2_3: f64 = 2.02226624871116645580e-21;
    const PIO2_3T: f64 = 8.47842766036889956997e-32;
    let f = (x * INVPIO2).round_ties_even();
    let mut r = x - f * PIO2_1;
    let mut w = f * PIO2_1T;
    let mut y0 = r - w;
    let ex = (high(x) >> 20) & 0x7ff;
    if ex as i32 - ((high(y0) >> 20) & 0x7ff) as i32 > 16 {
        let t = r;
        w = f * PIO2_2;
        r = t - w;
        w = f * PIO2_2T - ((t - r) - w);
        y0 = r - w;
        if ex as i32 - ((high(y0) >> 20) & 0x7ff) as i32 > 49 {
            let t = r;
            w = f * PIO2_3;
            r = t - w;
            w = f * PIO2_3T - ((t - r) - w);
            y0 = r - w;
        }
    }
    let y1 = (r - y0) - w;
    // n mod 4 without converting a possibly huge f to an integer: every
    // step is exact.
    let n = f - 4.0 * (f * 0.25).floor();
    (n as u32, y0, y1)
}

/// sin and cos of `x`, sharing one argument reduction.
pub fn sin_cos(x: f64) -> (f64, f64) {
    if !x.is_finite() {
        return (f64::NAN, f64::NAN);
    }
    if high(x) & 0x7fff_ffff <= 0x3fe9_21fb {
        return (kernel_sin(x, 0.0, false), kernel_cos(x, 0.0));
    }
    let (n, a, b) = rem_pio2(x);
    let (s, c) = (kernel_sin(a, b, true), kernel_cos(a, b));
    match n {
        0 => (s, c),
        1 => (c, -s),
        2 => (-s, -c),
        _ => (-c, s),
    }
}

pub fn sin(x: f64) -> f64 {
    sin_cos(x).0
}

pub fn cos(x: f64) -> f64 {
    sin_cos(x).1
}

/// sin/cos from one reduction: within a couple of ulps of the true tangent,
/// which is all a chart needs, without fdlibm's separate tan kernel.
pub fn tan(x: f64) -> f64 {
    let (s, c) = sin_cos(x);
    s / c
}

fn asin_r(z: f64) -> f64 {
    const PS0: f64 = 1.66666666666666657415e-01;
    const PS1: f64 = -3.25565818622400915405e-01;
    const PS2: f64 = 2.01212532134862925881e-01;
    const PS3: f64 = -4.00555345006794114027e-02;
    const PS4: f64 = 7.91534994289814532176e-04;
    const PS5: f64 = 3.47933107596021167570e-05;
    const QS1: f64 = -2.40339491173441421878e+00;
    const QS2: f64 = 2.02094576023350569471e+00;
    const QS3: f64 = -6.88283971605453293030e-01;
    const QS4: f64 = 7.70381505559019352791e-02;
    let p = z * (PS0 + z * (PS1 + z * (PS2 + z * (PS3 + z * (PS4 + z * PS5)))));
    let q = 1.0 + z * (QS1 + z * (QS2 + z * (QS3 + z * QS4)));
    p / q
}

pub fn asin(x: f64) -> f64 {
    let hx = high(x);
    let ix = hx & 0x7fff_ffff;
    if ix >= 0x3ff0_0000 {
        if ix == 0x3ff0_0000 && low(x) == 0 {
            return x * PIO2_HI;
        }
        return f64::NAN;
    }
    if ix < 0x3fe0_0000 {
        if (0x0010_0000..0x3e50_0000).contains(&ix) {
            return x;
        }
        return x + x * asin_r(x * x);
    }
    let z = (1.0 - x.abs()) * 0.5;
    let s = z.sqrt();
    let r = asin_r(z);
    let y = if ix >= 0x3fef_3333 {
        PIO2_HI - (2.0 * (s + s * r) - PIO2_LO)
    } else {
        let f = clear_low(s);
        let c = (z - f * f) / (s + f);
        0.5 * PIO2_HI - (2.0 * s * r - (PIO2_LO - 2.0 * c) - (0.5 * PIO2_HI - 2.0 * f))
    };
    if hx >> 31 != 0 { -y } else { y }
}

pub fn acos(x: f64) -> f64 {
    let hx = high(x);
    let ix = hx & 0x7fff_ffff;
    if ix >= 0x3ff0_0000 {
        if ix == 0x3ff0_0000 && low(x) == 0 {
            return if hx >> 31 != 0 { 2.0 * PIO2_HI } else { 0.0 };
        }
        return f64::NAN;
    }
    if ix < 0x3fe0_0000 {
        if ix <= 0x3c60_0000 {
            return PIO2_HI;
        }
        return PIO2_HI - (x - (PIO2_LO - x * asin_r(x * x)));
    }
    if hx >> 31 != 0 {
        let z = (1.0 + x) * 0.5;
        let s = z.sqrt();
        let w = asin_r(z) * s - PIO2_LO;
        return 2.0 * (PIO2_HI - (s + w));
    }
    let z = (1.0 - x) * 0.5;
    let s = z.sqrt();
    let df = clear_low(s);
    let c = (z - df * df) / (s + df);
    let w = asin_r(z) * s + c;
    2.0 * (df + w)
}

pub fn atan(x: f64) -> f64 {
    const ATANHI: [f64; 4] = [
        4.63647609000806093515e-01,
        7.85398163397448278999e-01,
        9.82793723247329054082e-01,
        1.57079632679489655800e+00,
    ];
    const ATANLO: [f64; 4] = [
        2.26987774529616870924e-17,
        3.06161699786838301793e-17,
        1.39033110312309984516e-17,
        6.12323399573676603587e-17,
    ];
    const AT: [f64; 11] = [
        3.33333333333329318027e-01,
        -1.99999999998764832476e-01,
        1.42857142725034663711e-01,
        -1.11111104054623557880e-01,
        9.09088713343650656196e-02,
        -7.69187620504482999495e-02,
        6.66107313738753120669e-02,
        -5.83357013379057348645e-02,
        4.97687799461593236017e-02,
        -3.65315727442169155270e-02,
        1.62858201153657823623e-02,
    ];
    let negative = high(x) >> 31 != 0;
    let ix = high(x) & 0x7fff_ffff;
    if ix >= 0x4410_0000 {
        if x.is_nan() {
            return x;
        }
        return if negative { -ATANHI[3] } else { ATANHI[3] };
    }
    let (id, x) = if ix < 0x3fdc_0000 {
        if ix < 0x3e40_0000 {
            return x;
        }
        (None, x)
    } else {
        let x = x.abs();
        if ix < 0x3ff3_0000 {
            if ix < 0x3fe6_0000 {
                (Some(0), (2.0 * x - 1.0) / (2.0 + x))
            } else {
                (Some(1), (x - 1.0) / (x + 1.0))
            }
        } else if ix < 0x4003_8000 {
            (Some(2), (x - 1.5) / (1.0 + 1.5 * x))
        } else {
            (Some(3), -1.0 / x)
        }
    };
    let z = x * x;
    let w = z * z;
    let s1 = z * (AT[0] + w * (AT[2] + w * (AT[4] + w * (AT[6] + w * (AT[8] + w * AT[10])))));
    let s2 = w * (AT[1] + w * (AT[3] + w * (AT[5] + w * (AT[7] + w * AT[9]))));
    let Some(id) = id else {
        return x - x * (s1 + s2);
    };
    let z = ATANHI[id] - (x * (s1 + s2) - ATANLO[id] - x);
    if negative { -z } else { z }
}

pub fn atan2(y: f64, x: f64) -> f64 {
    const PI: f64 = 3.1415926535897931160E+00;
    const PI_LO: f64 = 1.2246467991473531772E-16;
    if x.is_nan() || y.is_nan() {
        return x + y;
    }
    if x.to_bits() == 1f64.to_bits() {
        return atan(y);
    }
    let m = ((high(y) >> 31) & 1) | ((high(x) >> 30) & 2);
    let (ix, iy) = (high(x) & 0x7fff_ffff, high(y) & 0x7fff_ffff);
    if iy | low(y) == 0 {
        return match m {
            0 | 1 => y,
            2 => PI,
            _ => -PI,
        };
    }
    if ix | low(x) == 0 {
        return if m & 1 != 0 { -PI / 2.0 } else { PI / 2.0 };
    }
    if ix == 0x7ff0_0000 {
        return if iy == 0x7ff0_0000 {
            match m {
                0 => PI / 4.0,
                1 => -PI / 4.0,
                2 => 3.0 * PI / 4.0,
                _ => -3.0 * PI / 4.0,
            }
        } else {
            match m {
                0 => 0.0,
                1 => -0.0,
                2 => PI,
                _ => -PI,
            }
        };
    }
    if ix + (64 << 20) < iy || iy == 0x7ff0_0000 {
        return if m & 1 != 0 { -PI / 2.0 } else { PI / 2.0 };
    }
    let z = if m & 2 != 0 && iy + (64 << 20) < ix {
        0.0
    } else {
        atan((y / x).abs())
    };
    match m {
        0 => z,
        1 => -z,
        2 => PI - (z - PI_LO),
        _ => (z - PI_LO) - PI,
    }
}

/// `x` × 2^`n`, exact unless the result is subnormal or overflows.
fn scalbn(x: f64, mut n: i32) -> f64 {
    let mut y = x;
    if n > 1023 {
        y *= f64::from_bits(0x7fe0_0000_0000_0000);
        n -= 1023;
        if n > 1023 {
            y *= f64::from_bits(0x7fe0_0000_0000_0000);
            n = (n - 1023).min(1023);
        }
    } else if n < -1022 {
        // 2^-1022 × 2^53: keeps the final step clear of double rounding in
        // the subnormal range.
        let k = f64::from_bits(0x0360_0000_0000_0000);
        y *= k;
        n += 1022 - 53;
        if n < -1022 {
            y *= k;
            n = (n + 1022 - 53).max(-1022);
        }
    }
    y * f64::from_bits(((0x3ff + n) as u64) << 52)
}

pub fn exp(x: f64) -> f64 {
    const LN2HI: f64 = 6.93147180369123816490e-01;
    const LN2LO: f64 = 1.90821492927058770002e-10;
    const INVLN2: f64 = 1.44269504088896338700e+00;
    const P1: f64 = 1.66666666666666019037e-01;
    const P2: f64 = -2.77777777770155933842e-03;
    const P3: f64 = 6.61375632143793436117e-05;
    const P4: f64 = -1.65339022054652515390e-06;
    const P5: f64 = 4.13813679705723846039e-08;
    let negative = high(x) >> 31 != 0;
    let hx = high(x) & 0x7fff_ffff;
    if hx >= 0x4086_232b {
        if x.is_nan() {
            return x;
        }
        if x > 709.782712893383973096 {
            return f64::INFINITY;
        }
        if x < -745.13321910194110842 {
            return 0.0;
        }
    }
    let (k, hi, lo, x) = if hx > 0x3fd6_2e42 {
        let k = if hx >= 0x3ff0_a2b2 {
            (INVLN2 * x + if negative { -0.5 } else { 0.5 }) as i32
        } else if negative {
            -1
        } else {
            1
        };
        let hi = x - k as f64 * LN2HI;
        let lo = k as f64 * LN2LO;
        (k, hi, lo, hi - lo)
    } else if hx > 0x3e30_0000 {
        (0, x, 0.0, x)
    } else {
        return 1.0 + x;
    };
    let xx = x * x;
    let c = x - xx * (P1 + xx * (P2 + xx * (P3 + xx * (P4 + xx * P5))));
    let y = 1.0 + (x * c / (2.0 - c) - lo + hi);
    if k == 0 { y } else { scalbn(y, k) }
}

/// Natural logarithm.
pub fn ln(x: f64) -> f64 {
    const LN2_HI: f64 = 6.93147180369123816490e-01;
    const LN2_LO: f64 = 1.90821492927058770002e-10;
    const LG1: f64 = 6.666666666666735130e-01;
    const LG2: f64 = 3.999999999940941908e-01;
    const LG3: f64 = 2.857142874366239149e-01;
    const LG4: f64 = 2.222219843214978396e-01;
    const LG5: f64 = 1.818357216161805012e-01;
    const LG6: f64 = 1.531383769920937332e-01;
    const LG7: f64 = 1.479819860511658591e-01;
    let mut bits = x.to_bits();
    let mut hx = (bits >> 32) as u32;
    let mut k: i32 = 0;
    if hx < 0x0010_0000 || hx >> 31 != 0 {
        if bits << 1 == 0 {
            return f64::NEG_INFINITY;
        }
        if hx >> 31 != 0 {
            return f64::NAN;
        }
        // Subnormal: scale into the normal range.
        k -= 54;
        bits = (x * f64::from_bits(0x4350_0000_0000_0000)).to_bits();
        hx = (bits >> 32) as u32;
    } else if hx >= 0x7ff0_0000 {
        return x;
    } else if hx == 0x3ff0_0000 && bits << 32 == 0 {
        return 0.0;
    }
    // Reduce into [sqrt(2)/2, sqrt(2)].
    hx += 0x3ff0_0000 - 0x3fe6_a09e;
    k += (hx >> 20) as i32 - 0x3ff;
    hx = (hx & 0x000f_ffff) + 0x3fe6_a09e;
    let x = f64::from_bits(((hx as u64) << 32) | (bits & 0xffff_ffff));
    let f = x - 1.0;
    let hfsq = 0.5 * f * f;
    let s = f / (2.0 + f);
    let z = s * s;
    let w = z * z;
    let t1 = w * (LG2 + w * (LG4 + w * LG6));
    let t2 = z * (LG1 + w * (LG3 + w * (LG5 + w * LG7)));
    let r = t2 + t1;
    let dk = k as f64;
    s * (hfsq + r) + dk * LN2_LO - hfsq + f + dk * LN2_HI
}

/// `x`^`y` for `x` > 0, as exp(y·ln x): within about 1e-15 relative while
/// |y·ln x| stays small, as for the bandwidth and wheel factors that use it.
pub fn powf(x: f64, y: f64) -> f64 {
    exp(y * ln(x))
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The inputs the Go twin evaluates too: a fixed LCG over the ranges
    /// charts use, plus edge values. Both ports write their results for
    /// `make test-conformance` to diff.
    pub(crate) fn inputs() -> Vec<f64> {
        let mut v = vec![
            0.0,
            -0.0,
            1.0,
            -1.0,
            0.5,
            -0.5,
            f64::MIN_POSITIVE,
            5e-324,
            1e-300,
            std::f64::consts::PI,
            std::f64::consts::FRAC_PI_2,
            std::f64::consts::FRAC_PI_4,
            1e6,
            -1e6,
            700.0,
            -740.0,
        ];
        let mut s: u64 = 0x2545_f491_4f6c_dd1d;
        let mut next = || {
            s = s
                .wrapping_mul(6364136223846793005)
                .wrapping_add(1442695040888963407);
            (s >> 11) as f64 / (1u64 << 53) as f64
        };
        for (lo, hi) in [
            (-1.0, 1.0),
            (-7.0, 7.0),
            (-30.0, 30.0),
            (-800.0, 800.0),
            (0.0, 1e6),
        ] {
            for _ in 0..4000 {
                let u = next();
                v.push(lo + u * (hi - lo));
            }
        }
        v
    }

    fn ulps(a: f64, b: f64) -> u64 {
        if a == b || (a.is_nan() && b.is_nan()) {
            return 0;
        }
        let k = |x: f64| {
            let i = x.to_bits() as i64;
            if i < 0 { i64::MIN - i } else { i }
        };
        k(a).abs_diff(k(b))
    }

    #[test]
    fn matches_the_platform_libm_within_an_ulp_or_two() {
        // The point is determinism, not a new libm: anything more than a
        // couple of ulps from the platform's answer is a porting mistake.
        for x in inputs() {
            let small = x.abs() <= 1e5;
            if small {
                assert!(ulps(sin(x), x.sin()) <= 1, "sin({x})");
                assert!(ulps(cos(x), x.cos()) <= 1, "cos({x})");
                let t = x.tan();
                if t.abs() < 1e8 {
                    assert!(ulps(tan(x), t) <= 4, "tan({x}) {} vs {t}", tan(x));
                }
                assert!(ulps(atan(x), x.atan()) <= 1, "atan({x})");
                assert!(ulps(exp(x), x.exp()) <= 1, "exp({x})");
            }
            if x.abs() <= 1.0 {
                assert!(ulps(asin(x), x.asin()) <= 1, "asin({x})");
                assert!(ulps(acos(x), x.acos()) <= 1, "acos({x})");
            }
            if x > 0.0 {
                assert!(ulps(ln(x), x.ln()) <= 1, "ln({x})");
            }
            let y = x * 0.37 - 1.0;
            assert!(ulps(atan2(y, x), y.atan2(x)) <= 2, "atan2({y}, {x})");
        }
        for (x, y) in [(1000.0, -0.2), (0.8, 3.0), (0.8, -7.0), (2.0, 0.5)] {
            let (a, b) = (powf(x, y), f64::powf(x, y));
            assert!((a - b).abs() <= b * 1e-15, "{x}^{y}");
        }
    }

    #[test]
    fn edge_values_follow_ieee() {
        assert!(sin(f64::INFINITY).is_nan() && cos(f64::NAN).is_nan());
        assert_eq!(acos(1.0), 0.0);
        assert_eq!(acos(-1.0), std::f64::consts::PI);
        assert!(acos(1.5).is_nan() && asin(-1.5).is_nan());
        assert_eq!(asin(1.0), std::f64::consts::FRAC_PI_2);
        assert_eq!(atan2(0.0, -1.0), std::f64::consts::PI);
        assert_eq!(atan2(-1.0, 0.0), -std::f64::consts::FRAC_PI_2);
        assert_eq!(exp(0.0), 1.0);
        assert_eq!(exp(-1000.0), 0.0);
        assert_eq!(exp(1000.0), f64::INFINITY);
        assert_eq!(ln(1.0), 0.0);
        assert_eq!(ln(0.0), f64::NEG_INFINITY);
        assert!(ln(-1.0).is_nan());
        assert!(ulps(ln(5e-324), (5e-324f64).ln()) <= 1);
    }

    #[test]
    fn writes_results_for_the_cross_language_diff() {
        let mut out = String::new();
        for x in inputs() {
            let y = x * 0.37 - 1.0;
            let (s, c) = sin_cos(x);
            let row = [
                s,
                c,
                tan(x),
                asin(x),
                acos(x),
                atan(x),
                atan2(y, x),
                exp(x),
                ln(x),
                powf(x.abs() + 1.0, -0.2),
            ];
            out.push_str(&format!("{:016x}", x.to_bits()));
            for r in row {
                // NaN payloads differ between the languages' constants; any
                // NaN is the same answer.
                if r.is_nan() {
                    out.push_str(" nan");
                } else {
                    out.push_str(&format!(" {:016x}", r.to_bits()));
                }
            }
            out.push('\n');
        }
        let dir = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("build/conformance");
        std::fs::create_dir_all(&dir).unwrap();
        std::fs::write(dir.join("dmath-rust.txt"), out).unwrap();
    }
}
