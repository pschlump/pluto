/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hyperloglog

import "math"

// The estimator combines the two classic corrections on top of the raw
// harmonic-mean estimate of Flajolet et al.:
//
//   - LinearCounting for small cardinalities: while zero registers
//     remain and the estimate is under ~2.5m, m·ln(m/z) is the sharper
//     estimate (near-exact well below the threshold).
//   - Ertl's corrected estimator (Otmar Ertl, "New Estimators for
//     HyperLogLog", arXiv:1702.01284) for everything above it, via the
//     σ and τ closed forms below.  This is the estimator current Redis
//     ships (note/redis/src/hyperloglog.c, BSD-3 licensed portions,
//     Copyright (c) 2014-Present, Redis Ltd.); the choice over the HLL++
//     bias tables (Heule et al.) is deliberate — same O(m) cost, no
//     tables to embed, and no bias-table boundary jumps — and over the
//     LogLog-Beta polynomial (Qin et al.) because it is the strictly
//     better-behaved successor Redis itself switched to.
const alphaInf = 0.721347520444481703680

// lcThreshold is the LinearCounting switchover point in elements: 2.5m,
// the classic crossover above which the harmonic-mean family estimates
// more sharply than the zero-register count.
const lcThreshold = 2.5 * float64(Registers)

// histogram builds the 64-bin histogram of register values by walking
// the packed array three bytes (four registers) at a time.  Complexity
// is O(m) with no allocation.
func (h *Hll) histogram() (reghisto [64]int) {
	p := &h.dense
	for i := 0; i < DenseSize; i += 3 {
		b0, b1, b2 := p[i], p[i+1], p[i+2]
		reghisto[b0&registerMask]++
		reghisto[(b0>>6)|(b1&0x0f)<<2]++
		reghisto[(b1>>4)|(b2&0x03)<<4]++
		reghisto[b2>>2]++
	}
	return reghisto
}

// sigma is Ertl's σ(x), x ∈ [0,1] — the small-cardinality correction
// that replaces the harmonic mean's blind spot when many registers are
// still zero.  σ(1) is +∞, which correctly drives the estimate of an
// empty sketch to 0.  Converges in a handful of iterations (x squares
// each round).
func sigma(x float64) float64 {
	if x == 1 {
		return math.Inf(1)
	}
	y := 1.0
	z := x
	for {
		x *= x
		zPrime := z
		z += x * y
		y += y
		if zPrime == z {
			return z
		}
	}
}

// tau is Ertl's τ(x), x ∈ [0,1] — the large-cardinality correction for
// hash-collision saturation in the top histogram bins (registers at the
// maximum rank Q+1).  At the cardinalities a 64-bit hash admits it is
// almost always 0; it is kept for fidelity with the Redis estimator.
// Converges in a handful of iterations (x takes square roots).
func tau(x float64) float64 {
	if x == 0 || x == 1 {
		return 0
	}
	y := 1.0
	z := 1 - x
	for {
		x = math.Sqrt(x)
		zPrime := z
		y *= 0.5
		z -= (1 - x) * (1 - x) * y
		if zPrime == z {
			return z / 3
		}
	}
}

// estimate computes the cardinality estimate from a register histogram:
// Ertl's corrected estimate, switching to LinearCounting while the
// estimate sits under the 2.5m threshold and zero registers remain.
// Complexity is O(64) given the histogram.
func estimate(reghisto *[64]int) uint64 {
	const m = float64(Registers)

	z := m * tau((m-float64(reghisto[rankMax]))/m)
	for j := rankMax - 1; j >= 1; j-- {
		z += float64(reghisto[j])
		z *= 0.5
	}
	z += m * sigma(float64(reghisto[0])/m)
	e := alphaInf * m * m / z

	if zeros := reghisto[0]; zeros > 0 && e <= lcThreshold {
		return uint64(math.Round(m * math.Log(m/float64(zeros))))
	}
	// Defensive: a crafted sketch (e.g. every register at rankMax, which
	// no real hash stream produces) can zero the denominator.
	if e < 0 || math.IsNaN(e) || math.IsInf(e, 0) {
		return 0
	}
	return uint64(math.Round(e))
}
