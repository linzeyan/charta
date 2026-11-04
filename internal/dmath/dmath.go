// Package dmath holds elementary functions that return the same bits in Go
// and Rust.
//
// Go's math package differs from the platform libm the Rust crate calls,
// and from itself between amd64 and arm64, where the compiler fuses a*b+c
// into FMA. A last-ulp difference in an angle can move a scene coordinate
// across a 0.01 rounding boundary, so every layout path calls these.
//
// The algorithms are musl's (fdlibm, © Sun Microsystems, "freely
// distributable"), the same as the Rust crate's src/dmath.rs, using only
// IEEE basic operations and sqrt. Every product is wrapped in float64()
// before it is added or subtracted: the explicit conversion rounds it, so
// the compiler may not fuse it (Go spec, "Arithmetic operators"). Keep that
// when editing; a missed wrap shows up in `make test-conformance` on arm64.
package dmath

import "math"

const (
	pio2Hi = 1.57079632679489655800e+00
	pio2Lo = 6.12323399573676603587e-17
)

func high(x float64) uint32 { return uint32(math.Float64bits(x) >> 32) }

func low(x float64) uint32 { return uint32(math.Float64bits(x)) }

// clearLow is x with its low 32 bits cleared.
func clearLow(x float64) float64 {
	return math.Float64frombits(math.Float64bits(x) & 0xffffffff00000000)
}

func kernelSin(x, y float64, iy bool) float64 {
	const (
		s1 = -1.66666666666666324348e-01
		s2 = 8.33333333332248946124e-03
		s3 = -1.98412698298579493134e-04
		s4 = 2.75573137070700676789e-06
		s5 = -2.50507602534068634195e-08
		s6 = 1.58969099521155010221e-10
	)
	z := float64(x * x)
	w := float64(z * z)
	r := s2 + float64(z*(s3+float64(z*s4))) + float64(float64(z*w)*(s5+float64(z*s6)))
	v := float64(z * x)
	if !iy {
		return x + float64(v*(s1+float64(z*r)))
	}
	return x - ((float64(z*(float64(0.5*y)-float64(v*r))) - y) - float64(v*s1))
}

func kernelCos(x, y float64) float64 {
	const (
		c1 = 4.16666666666666019037e-02
		c2 = -1.38888888888741095749e-03
		c3 = 2.48015872894767294178e-05
		c4 = -2.75573143513906633035e-07
		c5 = 2.08757232129817482790e-09
		c6 = -1.13596475577881948265e-11
	)
	z := float64(x * x)
	w := float64(z * z)
	r := float64(z*(c1+float64(z*(c2+float64(z*c3))))) + float64(float64(w*w)*(c4+float64(z*(c5+float64(z*c6)))))
	hz := float64(0.5 * z)
	w = 1.0 - hz
	return w + (((1.0 - w) - hz) + (float64(z*r) - float64(x*y)))
}

// remPio2 reduces x by the nearest multiple n of π/2: n mod 4, head, tail.
func remPio2(x float64) (uint32, float64, float64) {
	const (
		invpio2 = 6.36619772367581382433e-01
		pio2_1  = 1.57079632673412561417e+00
		pio2_1t = 6.07710050650619224932e-11
		pio2_2  = 6.07710050630396597660e-11
		pio2_2t = 2.02226624879595063154e-21
		pio2_3  = 2.02226624871116645580e-21
		pio2_3t = 8.47842766036889956997e-32
	)
	f := math.RoundToEven(float64(x * invpio2))
	r := x - float64(f*pio2_1)
	w := float64(f * pio2_1t)
	y0 := r - w
	ex := int32((high(x) >> 20) & 0x7ff)
	if ex-int32((high(y0)>>20)&0x7ff) > 16 {
		t := r
		w = float64(f * pio2_2)
		r = t - w
		w = float64(f*pio2_2t) - ((t - r) - w)
		y0 = r - w
		if ex-int32((high(y0)>>20)&0x7ff) > 49 {
			t := r
			w = float64(f * pio2_3)
			r = t - w
			w = float64(f*pio2_3t) - ((t - r) - w)
			y0 = r - w
		}
	}
	y1 := (r - y0) - w
	// n mod 4 without converting a possibly huge f to an integer: every
	// step is exact.
	n := f - float64(4*math.Floor(float64(f*0.25)))
	return uint32(n), y0, y1
}

// Sincos is sin and cos of x, sharing one argument reduction.
func Sincos(x float64) (float64, float64) {
	if math.IsInf(x, 0) || math.IsNaN(x) {
		return math.NaN(), math.NaN()
	}
	if high(x)&0x7fffffff <= 0x3fe921fb {
		return kernelSin(x, 0, false), kernelCos(x, 0)
	}
	n, a, b := remPio2(x)
	s, c := kernelSin(a, b, true), kernelCos(a, b)
	switch n {
	case 0:
		return s, c
	case 1:
		return c, -s
	case 2:
		return -s, -c
	default:
		return -c, s
	}
}

func Sin(x float64) float64 { s, _ := Sincos(x); return s }

func Cos(x float64) float64 { _, c := Sincos(x); return c }

// Tan is sin/cos from one reduction: within a couple of ulps of the true
// tangent, which is all a chart needs, without fdlibm's separate tan kernel.
func Tan(x float64) float64 {
	s, c := Sincos(x)
	return s / c
}

func asinR(z float64) float64 {
	const (
		pS0 = 1.66666666666666657415e-01
		pS1 = -3.25565818622400915405e-01
		pS2 = 2.01212532134862925881e-01
		pS3 = -4.00555345006794114027e-02
		pS4 = 7.91534994289814532176e-04
		pS5 = 3.47933107596021167570e-05
		qS1 = -2.40339491173441421878e+00
		qS2 = 2.02094576023350569471e+00
		qS3 = -6.88283971605453293030e-01
		qS4 = 7.70381505559019352791e-02
	)
	p := float64(z * (pS0 + float64(z*(pS1+float64(z*(pS2+float64(z*(pS3+float64(z*(pS4+float64(z*pS5)))))))))))
	q := 1.0 + float64(z*(qS1+float64(z*(qS2+float64(z*(qS3+float64(z*qS4)))))))
	return p / q
}

func Asin(x float64) float64 {
	hx := high(x)
	ix := hx & 0x7fffffff
	if ix >= 0x3ff00000 {
		if ix == 0x3ff00000 && low(x) == 0 {
			return float64(x * pio2Hi)
		}
		return math.NaN()
	}
	if ix < 0x3fe00000 {
		if ix >= 0x00100000 && ix < 0x3e500000 {
			return x
		}
		return x + float64(x*asinR(float64(x*x)))
	}
	z := float64((1.0 - math.Abs(x)) * 0.5)
	s := math.Sqrt(z)
	r := asinR(z)
	var y float64
	if ix >= 0x3fef3333 {
		y = pio2Hi - (float64(2.0*(s+float64(s*r))) - pio2Lo)
	} else {
		f := clearLow(s)
		c := (z - float64(f*f)) / (s + f)
		y = float64(0.5*pio2Hi) - (float64(float64(2.0*s)*r) - (pio2Lo - float64(2.0*c)) - (float64(0.5*pio2Hi) - float64(2.0*f)))
	}
	if hx>>31 != 0 {
		return -y
	}
	return y
}

func Acos(x float64) float64 {
	hx := high(x)
	ix := hx & 0x7fffffff
	if ix >= 0x3ff00000 {
		if ix == 0x3ff00000 && low(x) == 0 {
			if hx>>31 != 0 {
				return float64(2.0 * pio2Hi)
			}
			return 0
		}
		return math.NaN()
	}
	if ix < 0x3fe00000 {
		if ix <= 0x3c600000 {
			return pio2Hi
		}
		return pio2Hi - (x - (pio2Lo - float64(x*asinR(float64(x*x)))))
	}
	if hx>>31 != 0 {
		z := float64((1.0 + x) * 0.5)
		s := math.Sqrt(z)
		w := float64(asinR(z)*s) - pio2Lo
		return float64(2.0 * (pio2Hi - (s + w)))
	}
	z := float64((1.0 - x) * 0.5)
	s := math.Sqrt(z)
	df := clearLow(s)
	c := (z - float64(df*df)) / (s + df)
	w := float64(asinR(z)*s) + c
	return float64(2.0 * (df + w))
}

var (
	atanHi = [4]float64{
		4.63647609000806093515e-01,
		7.85398163397448278999e-01,
		9.82793723247329054082e-01,
		1.57079632679489655800e+00,
	}
	atanLo = [4]float64{
		2.26987774529616870924e-17,
		3.06161699786838301793e-17,
		1.39033110312309984516e-17,
		6.12323399573676603587e-17,
	}
	aT = [11]float64{
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
	}
)

func Atan(x float64) float64 {
	negative := high(x)>>31 != 0
	ix := high(x) & 0x7fffffff
	if ix >= 0x44100000 {
		if math.IsNaN(x) {
			return x
		}
		if negative {
			return -atanHi[3]
		}
		return atanHi[3]
	}
	id := -1
	if ix < 0x3fdc0000 {
		if ix < 0x3e400000 {
			return x
		}
	} else {
		x = math.Abs(x)
		switch {
		case ix < 0x3fe60000:
			id, x = 0, (float64(2.0*x)-1.0)/(2.0+x)
		case ix < 0x3ff30000:
			id, x = 1, (x-1.0)/(x+1.0)
		case ix < 0x40038000:
			id, x = 2, (x-1.5)/(1.0+float64(1.5*x))
		default:
			id, x = 3, -1.0/x
		}
	}
	z := float64(x * x)
	w := float64(z * z)
	s1 := float64(z * (aT[0] + float64(w*(aT[2]+float64(w*(aT[4]+float64(w*(aT[6]+float64(w*(aT[8]+float64(w*aT[10])))))))))))
	s2 := float64(w * (aT[1] + float64(w*(aT[3]+float64(w*(aT[5]+float64(w*(aT[7]+float64(w*aT[9])))))))))
	if id < 0 {
		return x - float64(x*(s1+s2))
	}
	z = atanHi[id] - (float64(x*(s1+s2)) - atanLo[id] - x)
	if negative {
		return -z
	}
	return z
}

func Atan2(y, x float64) float64 {
	// Typed, so 3*pi/4 starts from the rounded pi as Rust's 3.0 * PI / 4.0
	// does; an untyped constant expression is evaluated exactly.
	const (
		pi   float64 = 3.1415926535897931160e+00
		piLo float64 = 1.2246467991473531772e-16
	)
	if math.IsNaN(x) || math.IsNaN(y) {
		return x + y
	}
	if x == 1 {
		return Atan(y)
	}
	m := ((high(y) >> 31) & 1) | ((high(x) >> 30) & 2)
	ix, iy := high(x)&0x7fffffff, high(y)&0x7fffffff
	if iy|low(y) == 0 {
		switch m {
		case 0, 1:
			return y
		case 2:
			return pi
		default:
			return -pi
		}
	}
	if ix|low(x) == 0 {
		if m&1 != 0 {
			return -pi / 2
		}
		return pi / 2
	}
	if ix == 0x7ff00000 {
		if iy == 0x7ff00000 {
			switch m {
			case 0:
				return pi / 4
			case 1:
				return -pi / 4
			case 2:
				return 3 * pi / 4
			default:
				return -3 * pi / 4
			}
		}
		switch m {
		case 0:
			return 0
		case 1:
			return math.Copysign(0, -1)
		case 2:
			return pi
		default:
			return -pi
		}
	}
	if ix+(64<<20) < iy || iy == 0x7ff00000 {
		if m&1 != 0 {
			return -pi / 2
		}
		return pi / 2
	}
	z := 0.0
	if m&2 == 0 || iy+(64<<20) >= ix {
		z = Atan(math.Abs(y / x))
	}
	switch m {
	case 0:
		return z
	case 1:
		return -z
	case 2:
		return pi - (z - piLo)
	default:
		return (z - piLo) - pi
	}
}

// scalbn is x × 2^n, exact unless the result is subnormal or overflows.
func scalbn(x float64, n int32) float64 {
	y := x
	if n > 1023 {
		y = float64(y * math.Float64frombits(0x7fe0000000000000))
		n -= 1023
		if n > 1023 {
			y = float64(y * math.Float64frombits(0x7fe0000000000000))
			n = min(n-1023, 1023)
		}
	} else if n < -1022 {
		// 2^-1022 × 2^53: keeps the final step clear of double rounding in
		// the subnormal range.
		k := math.Float64frombits(0x0360000000000000)
		y = float64(y * k)
		n += 1022 - 53
		if n < -1022 {
			y = float64(y * k)
			n = max(n+1022-53, -1022)
		}
	}
	return float64(y * math.Float64frombits(uint64(0x3ff+n)<<52))
}

func Exp(x float64) float64 {
	const (
		ln2Hi  = 6.93147180369123816490e-01
		ln2Lo  = 1.90821492927058770002e-10
		invln2 = 1.44269504088896338700e+00
		p1     = 1.66666666666666019037e-01
		p2     = -2.77777777770155933842e-03
		p3     = 6.61375632143793436117e-05
		p4     = -1.65339022054652515390e-06
		p5     = 4.13813679705723846039e-08
	)
	negative := high(x)>>31 != 0
	hx := high(x) & 0x7fffffff
	if hx >= 0x4086232b {
		if math.IsNaN(x) {
			return x
		}
		if x > 709.782712893383973096 {
			return math.Inf(1)
		}
		if x < -745.13321910194110842 {
			return 0
		}
	}
	var k int32
	var hi, lo float64
	switch {
	case hx > 0x3fd62e42:
		switch {
		case hx >= 0x3ff0a2b2:
			half := 0.5
			if negative {
				half = -0.5
			}
			k = int32(float64(invln2*x) + half)
		case negative:
			k = -1
		default:
			k = 1
		}
		hi = x - float64(float64(k)*ln2Hi)
		lo = float64(float64(k) * ln2Lo)
		x = hi - lo
	case hx > 0x3e300000:
		hi = x
	default:
		return 1 + x
	}
	xx := float64(x * x)
	c := x - float64(xx*(p1+float64(xx*(p2+float64(xx*(p3+float64(xx*(p4+float64(xx*p5)))))))))
	y := 1 + (float64(x*c)/(2-c) - lo + hi)
	if k == 0 {
		return y
	}
	return scalbn(y, k)
}

// Log is the natural logarithm.
func Log(x float64) float64 {
	const (
		ln2Hi = 6.93147180369123816490e-01
		ln2Lo = 1.90821492927058770002e-10
		lg1   = 6.666666666666735130e-01
		lg2   = 3.999999999940941908e-01
		lg3   = 2.857142874366239149e-01
		lg4   = 2.222219843214978396e-01
		lg5   = 1.818357216161805012e-01
		lg6   = 1.531383769920937332e-01
		lg7   = 1.479819860511658591e-01
	)
	bits := math.Float64bits(x)
	hx := uint32(bits >> 32)
	var k int32
	switch {
	case hx < 0x00100000 || hx>>31 != 0:
		if bits<<1 == 0 {
			return math.Inf(-1)
		}
		if hx>>31 != 0 {
			return math.NaN()
		}
		// Subnormal: scale into the normal range.
		k -= 54
		bits = math.Float64bits(float64(x * math.Float64frombits(0x4350000000000000)))
		hx = uint32(bits >> 32)
	case hx >= 0x7ff00000:
		return x
	case hx == 0x3ff00000 && bits<<32 == 0:
		return 0
	}
	// Reduce into [sqrt(2)/2, sqrt(2)].
	hx += 0x3ff00000 - 0x3fe6a09e
	k += int32(hx>>20) - 0x3ff
	hx = (hx & 0x000fffff) + 0x3fe6a09e
	x = math.Float64frombits(uint64(hx)<<32 | bits&0xffffffff)
	f := x - 1.0
	hfsq := float64(float64(0.5*f) * f)
	s := f / (2.0 + f)
	z := float64(s * s)
	w := float64(z * z)
	t1 := float64(w * (lg2 + float64(w*(lg4+float64(w*lg6)))))
	t2 := float64(z * (lg1 + float64(w*(lg3+float64(w*(lg5+float64(w*lg7)))))))
	r := t2 + t1
	dk := float64(k)
	return float64(s*(hfsq+r)) + float64(dk*ln2Lo) - hfsq + f + float64(dk*ln2Hi)
}

// Pow is x^y for x > 0, as exp(y·ln x): within about 1e-15 relative while
// |y·ln x| stays small, as for the bandwidth and wheel factors that use it.
func Pow(x, y float64) float64 { return Exp(float64(y * Log(x))) }
