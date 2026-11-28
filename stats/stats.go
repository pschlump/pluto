/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package stats provides mergeable online statistics, paired regression, and
// exact Type-7 quantiles.
package stats

import "math"

// Accumulator collects floating-point statistics with stable online updates.
// Its zero value is ready to use and it must not be copied after first use.
type Accumulator struct {
	count uint64
	mean  float64
	m2    float64
	min   float64
	max   float64
}

// Stats is an immutable snapshot of accumulated statistics.
type Stats struct {
	count uint64
	mean  float64
	m2    float64
	min   float64
	max   float64
}

// Add adds one value. Any NaN makes mean and variance NaN. Infinities follow
// ordinary floating-point mean semantics and make variance NaN.
func (accumulator *Accumulator) Add(value float64) {
	if accumulator.count == 0 {
		accumulator.count = 1
		accumulator.mean = value
		accumulator.min, accumulator.max = value, value
		if math.IsNaN(value) || math.IsInf(value, 0) {
			accumulator.m2 = math.NaN()
		}
		return
	}
	accumulator.count++
	if finite(value) && finite(accumulator.mean) {
		delta := value - accumulator.mean
		accumulator.mean += delta / float64(accumulator.count)
		accumulator.m2 += delta * (value - accumulator.mean)
	} else {
		accumulator.mean = nonFiniteMean(accumulator.mean, value)
		accumulator.m2 = math.NaN()
	}
	accumulator.min = minFloat(accumulator.min, value)
	accumulator.max = maxFloat(accumulator.max, value)
}

// AddAll adds every value in order.
func (accumulator *Accumulator) AddAll(values ...float64) {
	for _, value := range values {
		accumulator.Add(value)
	}
}

// Merge incorporates an immutable snapshot.
func (accumulator *Accumulator) Merge(other Stats) {
	if other.count == 0 {
		return
	}
	if accumulator.count == 0 {
		*accumulator = Accumulator(other)
		return
	}
	accumulator.count += other.count
	if finite(accumulator.mean) && finite(other.mean) {
		delta := other.mean - accumulator.mean
		accumulator.mean += delta * float64(other.count) / float64(accumulator.count)
		accumulator.m2 += other.m2 + delta*(other.mean-accumulator.mean)*float64(other.count)
	} else {
		accumulator.mean = nonFiniteMean(accumulator.mean, other.mean)
		accumulator.m2 = math.NaN()
	}
	accumulator.min = minFloat(accumulator.min, other.min)
	accumulator.max = maxFloat(accumulator.max, other.max)
}

// MergeAccumulator incorporates another accumulator's current state.
func (accumulator *Accumulator) MergeAccumulator(other *Accumulator) {
	accumulator.Merge(other.Snapshot())
}

// Snapshot returns an independent immutable value.
func (accumulator *Accumulator) Snapshot() Stats {
	return Stats(*accumulator)
}

// Count returns the number of values.
func (stats Stats) Count() uint64 { return stats.count }

// Sum returns mean*count. It is zero for an empty dataset.
func (stats Stats) Sum() float64 {
	if stats.count == 0 {
		return 0
	}
	return stats.mean * float64(stats.count)
}

// Mean returns NaN for an empty dataset.
func (stats Stats) Mean() float64 {
	if stats.count == 0 {
		return math.NaN()
	}
	return stats.mean
}

// Min returns NaN for an empty dataset.
func (stats Stats) Min() float64 {
	if stats.count == 0 {
		return math.NaN()
	}
	return stats.min
}

// Max returns NaN for an empty dataset.
func (stats Stats) Max() float64 {
	if stats.count == 0 {
		return math.NaN()
	}
	return stats.max
}

// PopulationVariance returns NaN for an empty or non-finite dataset.
func (stats Stats) PopulationVariance() float64 {
	if stats.count == 0 {
		return math.NaN()
	}
	if math.IsNaN(stats.m2) {
		return math.NaN()
	}
	if stats.count == 1 {
		return 0
	}
	return nonNegative(stats.m2) / float64(stats.count)
}

// SampleVariance returns NaN when fewer than two values are available.
func (stats Stats) SampleVariance() float64 {
	if stats.count < 2 || math.IsNaN(stats.m2) {
		return math.NaN()
	}
	return nonNegative(stats.m2) / float64(stats.count-1)
}

// PopulationStandardDeviation returns the square root of population variance.
func (stats Stats) PopulationStandardDeviation() float64 {
	return math.Sqrt(stats.PopulationVariance())
}

// SampleStandardDeviation returns the square root of sample variance.
func (stats Stats) SampleStandardDeviation() float64 {
	return math.Sqrt(stats.SampleVariance())
}

// minFloat is math.Min, bit for bit, without its out-of-line call. The
// builtin min already matches math.Min for signed zeros, ordinary NaN
// propagation and infinities, except that math.Min returns -Inf (not NaN)
// when one operand is -Inf and the other NaN; that pair is patched up on the
// rarely taken NaN branch.
func minFloat(x, y float64) float64 {
	result := min(x, y)
	if result != result {
		if x < -math.MaxFloat64 { // x is -Inf
			return x
		}
		if y < -math.MaxFloat64 {
			return y
		}
	}
	return result
}

// maxFloat is math.Max, bit for bit (see minFloat): math.Max returns +Inf,
// not NaN, for a +Inf/NaN pair.
func maxFloat(x, y float64) float64 {
	result := max(x, y)
	if result != result {
		if x > math.MaxFloat64 { // x is +Inf
			return x
		}
		if y > math.MaxFloat64 {
			return y
		}
	}
	return result
}

// finite reports whether value is neither NaN nor infinite: value-value is
// exactly 0 for every finite value and NaN for NaN and both infinities.
func finite(value float64) bool {
	return value-value == 0
}

func nonFiniteMean(previous, value float64) float64 {
	if finite(previous) {
		return value
	}
	if finite(value) || previous == value {
		return previous
	}
	return math.NaN()
}

func nonNegative(value float64) float64 {
	if value > 0 {
		return value
	}
	return 0
}
