package base

import "math"

// LinearScale maps a continuous domain linearly onto a pixel range.
type LinearScale struct {
	D0, D1 float64
	R0, R1 float64
}

func (s LinearScale) Map(v float64) float64 {
	if s.D1 == s.D0 {
		return (s.R0 + s.R1) / 2
	}
	return s.R0 + float64((v-s.D0)/(s.D1-s.D0)*(s.R1-s.R0))
}

// Nice extends the domain to round tick values. Requires D0 <= D1.
func (s LinearScale) Nice(count int) LinearScale {
	s.D0, s.D1 = Nice(s.D0, s.D1, count)
	return s
}

func (s LinearScale) Ticks(count int) []float64 {
	return Ticks(s.D0, s.D1, count)
}

// BandScale splits a pixel range into n equal bands with padding, like d3's
// scaleBand with align fixed at the center.
type BandScale struct {
	start, step, bandwidth float64
}

// NewBandScale: paddingInner is the gap between bands and paddingOuter the
// gap at each end, both as fractions of the step.
func NewBandScale(n int, r0, r1, paddingInner, paddingOuter float64) BandScale {
	fn := float64(n)
	step := (r1 - r0) / math.Max(fn-paddingInner+float64(paddingOuter*2), 1)
	start := r0 + float64((r1-r0-float64(step*(fn-paddingInner)))*0.5)
	return BandScale{start: start, step: step, bandwidth: step * (1 - paddingInner)}
}

// Position is the left edge of band i.
func (b BandScale) Position(i int) float64 { return b.start + float64(b.step*float64(i)) }

func (b BandScale) Bandwidth() float64 { return b.bandwidth }

// Step is the distance from one band's start to the next.
func (b BandScale) Step() float64 { return b.step }
