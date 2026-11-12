package chart

import "math"

// niceStep returns the smallest 1/2/2.5/5 x 10^k step that is >= raw.
func niceStep(raw float64) float64 {
	if raw <= 0 || math.IsNaN(raw) || math.IsInf(raw, 0) {
		return 1
	}
	exp := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if m*exp >= raw*(1-1e-12) {
			return m * exp
		}
	}
	return 10 * exp
}

// tidy removes floating point noise from k*step.
func tidy(v float64) float64 {
	return math.Round(v*1e9) / 1e9
}

// NiceTicks returns ticks from 0 up to a nice maximum >= max, aiming for n steps.
// A max <= 0 yields [0, 1].
func NiceTicks(max float64, n int) []float64 {
	if max <= 0 || math.IsNaN(max) || math.IsInf(max, 0) {
		return []float64{0, 1}
	}
	if n < 1 {
		n = 1
	}
	step := niceStep(max / float64(n))
	count := int(math.Ceil(max/step - 1e-9))
	if count < 1 {
		count = 1
	}
	out := make([]float64, 0, count+1)
	for i := 0; i <= count; i++ {
		out = append(out, tidy(float64(i)*step))
	}
	return out
}

// NiceRange is NiceTicks for a non-zero-based axis covering [min, max].
// A degenerate range (min == max) is padded so the axis has a span.
func NiceRange(min, max float64, n int) []float64 {
	if n < 1 {
		n = 1
	}
	if max < min {
		min, max = max, min
	}
	if max-min < 1e-12 {
		pad := math.Abs(max) * 0.1
		if pad < 1e-9 {
			pad = 1
		}
		min -= pad
		max += pad
	}
	step := niceStep((max - min) / float64(n))
	first := math.Floor(min/step + 1e-9)
	last := math.Ceil(max/step - 1e-9)
	if last <= first {
		last = first + 1
	}
	out := make([]float64, 0, int(last-first)+1)
	for i := first; i <= last; i++ {
		out = append(out, tidy(i*step))
	}
	return out
}
