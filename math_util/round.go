/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package math_util

import (
	"math"
	"math/bits"
)

// sqrt2Floor63 is floor(sqrt(2) * 2^63) = floor(sqrt(2^127)). Shifting it
// right by 63-f yields floor(sqrt(2^(2f+1))), the Log2 half-mode threshold.
const sqrt2Floor63 = 0xB504F333F9DE6484

// powersOf10[i] is 10^i for every power of ten representable in uint64.
var powersOf10 = [20]uint64{
	1, 10, 100, 1000, 10000, 100000, 1000000, 10000000, 100000000,
	1000000000, 10000000000, 100000000000, 1000000000000, 10000000000000,
	100000000000000, 1000000000000000, 10000000000000000,
	100000000000000000, 1000000000000000000, 10000000000000000000,
}

// halfPowersOf10[i] is floor(sqrt(10^(2i+1))), the Log10 half-mode
// threshold for floor log10 i. The i=19 threshold exceeds MaxUint64, so its
// entry is MaxUint64 (no uint64 value is greater).
var halfPowersOf10 = [20]uint64{
	3, 31, 316, 3162, 31622, 316227, 3162277, 31622776, 316227766,
	3162277660, 31622776601, 316227766016, 3162277660168, 31622776601683,
	316227766016837, 3162277660168379, 31622776601683793,
	316227766016837933, 3162277660168379331, math.MaxUint64,
}

// log10Floor returns floor(log10(value)) for value > 0. The bit length
// gives an estimate (bits*1233/4096 ≈ bits*log10(2)) that is either exact
// or one too high; one table comparison corrects it.
func log10Floor(value uint64) int {
	guess := bits.Len64(value) * 1233 >> 12
	if value < powersOf10[guess] {
		guess--
	}
	return guess
}

// RoundingMode specifies how an inexact integer result is rounded.
type RoundingMode int

const (
	RoundFloor RoundingMode = iota
	RoundCeiling
	RoundTowardZero
	RoundAwayFromZero
	RoundHalfEven
	RoundHalfUp
	RoundHalfDown
	RoundExact
)

// Divide returns dividend/divisor with explicit rounding. RoundExact returns
// ErrInexact when there is a nonzero remainder.
// Complexity is O(1).
func Divide[T Integer](dividend, divisor T, mode RoundingMode) (T, error) {
	if divisor == 0 {
		return 0, ErrDivideByZero
	}
	if isSigned[T]() {
		minimum, _ := signedBounds[T]()
		if dividend == minimum && divisor == ^T(0) {
			return 0, ErrOverflow
		}
	}
	quotient := dividend / divisor
	remainder := dividend % divisor
	// The rounding decision is made inline (no helper calls): this is the
	// hot path, and the former three call layers cost more than the divide.
	// An invalid mode and RoundExact still return the truncated quotient
	// alongside their error.
	if mode < RoundFloor || mode > RoundExact {
		return quotient, ErrRoundingMode
	}
	if remainder == 0 {
		return quotient, nil
	}
	same := sameSign(remainder, divisor)
	var increment bool
	switch mode {
	case RoundFloor:
		increment = !same
	case RoundCeiling:
		increment = same
	case RoundTowardZero:
		return quotient, nil
	case RoundAwayFromZero:
		increment = true
	case RoundExact:
		return quotient, ErrInexact
	default:
		// Compare 2*|remainder| with |divisor| without doubling: above
		// half rounds up; exactly half is possible only for an even divisor.
		x, y := magnitude(remainder), magnitude(divisor)
		half := y >> 1
		tie := x == half && y&1 == 0
		increment = x > half
		if tie {
			increment = mode == RoundHalfUp || (mode == RoundHalfEven && quotient&1 != 0)
		}
	}
	if !increment {
		return quotient, nil
	}
	if same {
		result, ok := CheckedAdd(quotient, T(1))
		if !ok {
			return 0, ErrOverflow
		}
		return result, nil
	}
	result, ok := CheckedSubtract(quotient, T(1))
	if !ok {
		return 0, ErrOverflow
	}
	return result, nil
}

func checkMode(mode RoundingMode) error {
	if mode < RoundFloor || mode > RoundExact {
		return ErrRoundingMode
	}
	return nil
}

func sameSign[T Integer](a, b T) bool {
	if !isSigned[T]() {
		return true
	}
	return (a < 0) == (b < 0)
}

// Log2 returns the base-2 logarithm of a positive integer with explicit
// rounding. Half modes compare exactly against the irrational midpoint.
// Complexity is O(1).
func Log2[T Integer](value T, mode RoundingMode) (int, error) {
	if value == 0 || (isSigned[T]() && value < 0) {
		return 0, ErrNonPositive
	}
	if err := checkMode(mode); err != nil {
		return 0, err
	}
	unsigned := magnitude(value)
	floor := bits.Len64(unsigned) - 1
	exact := unsigned&(unsigned-1) == 0
	if exact || mode == RoundFloor || mode == RoundTowardZero {
		return floor, nil
	}
	if mode == RoundExact {
		return 0, ErrInexact
	}
	if mode == RoundCeiling || mode == RoundAwayFromZero {
		return floor + 1, nil
	}
	// The midpoint 2^(floor+1/2) is irrational, so unsigned² never equals
	// 2^(2*floor+1): every half mode rounds up exactly when unsigned exceeds
	// floor(sqrt(2^(2*floor+1))) = sqrt2Floor63 >> (63-floor).
	if unsigned > sqrt2Floor63>>(63-floor) {
		return floor + 1, nil
	}
	return floor, nil
}

// Log10 returns the base-10 logarithm of a positive integer with explicit
// rounding. Half modes compare exactly against the irrational midpoint.
// Complexity is O(1).
func Log10[T Integer](value T, mode RoundingMode) (int, error) {
	if value == 0 || (isSigned[T]() && value < 0) {
		return 0, ErrNonPositive
	}
	if err := checkMode(mode); err != nil {
		return 0, err
	}
	unsigned := magnitude(value)
	floor := log10Floor(unsigned)
	if unsigned == powersOf10[floor] || mode == RoundFloor || mode == RoundTowardZero {
		return floor, nil
	}
	if mode == RoundExact {
		return 0, ErrInexact
	}
	if mode == RoundCeiling || mode == RoundAwayFromZero {
		return floor + 1, nil
	}
	// 10^(2*floor+1) is never a perfect square, so there is no exact tie:
	// every half mode rounds up exactly when unsigned exceeds
	// floor(sqrt(10^(2*floor+1))).
	if unsigned > halfPowersOf10[floor] {
		return floor + 1, nil
	}
	return floor, nil
}

// Sqrt returns the integer square root of a nonnegative integer with explicit
// rounding. An overflowing rounded result returns ErrOverflow.
// Complexity is O(1).
func Sqrt[T Integer](value T, mode RoundingMode) (T, error) {
	if isSigned[T]() && value < 0 {
		return 0, ErrNegative
	}
	if err := checkMode(mode); err != nil {
		return 0, err
	}
	unsigned := magnitude(value)
	// The float estimate is within one of the true floor root; correct it
	// with multiplications, not divisions. Capping at 2^32-1 (the largest
	// root of any uint64) keeps every square below in uint64 range.
	root := min(uint64(math.Sqrt(float64(unsigned))), math.MaxUint32)
	for root*root > unsigned {
		root--
	}
	for root < math.MaxUint32 && (root+1)*(root+1) <= unsigned {
		root++
	}
	exact := root*root == unsigned
	if exact || mode == RoundFloor || mode == RoundTowardZero {
		result, ok := CheckedConvert[T](root)
		if !ok {
			return 0, ErrOverflow
		}
		return result, nil
	}
	if mode == RoundExact {
		return 0, ErrInexact
	}
	roundUp := mode == RoundCeiling || mode == RoundAwayFromZero
	if !roundUp {
		roundUp = unsigned > root*(root+1)
	}
	if roundUp {
		root++
	}
	result, ok := CheckedConvert[T](root)
	if !ok {
		return 0, ErrOverflow
	}
	return result, nil
}
