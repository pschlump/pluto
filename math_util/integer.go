/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package math_util provides checked integer arithmetic, explicit rounding,
// and number-theoretic utilities for standard integer types.
package math_util

import (
	"errors"
	"math/big"
	"unsafe"
)

// Signed includes all built-in signed integer types and defined types with
// those underlying types.
type Signed interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

// Unsigned includes all built-in unsigned integer types and defined types with
// those underlying types.
type Unsigned interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Integer includes signed and unsigned integer types.
type Integer interface {
	Signed | Unsigned
}

var (
	ErrDivideByZero = errors.New("math_util: division by zero")
	ErrOverflow     = errors.New("math_util: integer overflow")
	ErrInexact      = errors.New("math_util: inexact result")
	ErrNegative     = errors.New("math_util: negative input")
	ErrNonPositive  = errors.New("math_util: non-positive input")
	ErrRoundingMode = errors.New("math_util: invalid rounding mode")
)

func integerBits[T Integer]() int {
	return int(unsafe.Sizeof(*new(T)) * 8)
}

func isSigned[T Integer]() bool {
	var zero T
	return ^zero < zero
}

func signedBounds[T Integer]() (T, T) {
	bits := integerBits[T]()
	maximum := T(uint64(1)<<(bits-1) - 1)
	return -maximum - 1, maximum
}

func unsignedMax[T Integer]() T {
	bits := integerBits[T]()
	if bits == 64 {
		maximum := ^uint64(0)
		return T(maximum)
	}
	return T(uint64(1)<<bits - 1)
}

func integerBounds[T Integer]() (T, T) {
	if isSigned[T]() {
		return signedBounds[T]()
	}
	var zero T
	return zero, unsignedMax[T]()
}

// CheckedAdd returns a+b and true, or zero and false on overflow.
func CheckedAdd[T Integer](a, b T) (T, bool) {
	if isSigned[T]() {
		minimum, maximum := signedBounds[T]()
		if (b > 0 && a > maximum-b) || (b < 0 && a < minimum-b) {
			var zero T
			return zero, false
		}
	} else if a > unsignedMax[T]()-b {
		var zero T
		return zero, false
	}
	return a + b, true
}

// CheckedSubtract returns a-b and true, or zero and false on overflow.
func CheckedSubtract[T Integer](a, b T) (T, bool) {
	if isSigned[T]() {
		minimum, maximum := signedBounds[T]()
		if (b > 0 && a < minimum+b) || (b < 0 && a > maximum+b) {
			var zero T
			return zero, false
		}
	} else if a < b {
		var zero T
		return zero, false
	}
	return a - b, true
}

// CheckedMultiply returns a*b and true, or zero and false on overflow.
func CheckedMultiply[T Integer](a, b T) (T, bool) {
	if a == 0 || b == 0 {
		var zero T
		return zero, true
	}
	result := a * b
	if isSigned[T]() {
		minimum, _ := signedBounds[T]()
		if (a == minimum && b == ^T(0)) || (b == minimum && a == ^T(0)) {
			var zero T
			return zero, false
		}
		if result/b != a {
			var zero T
			return zero, false
		}
	} else if result/a != b {
		var zero T
		return zero, false
	}
	return result, true
}

// CheckedPow returns base^exponent and true, or zero and false on overflow.
// A negative exponent is a programmer error and panics.
func CheckedPow[T Integer](base T, exponent int) (T, bool) {
	if exponent < 0 {
		panic("math_util: negative exponent")
	}
	result := T(1)
	for exponent > 0 {
		if exponent&1 != 0 {
			var ok bool
			result, ok = CheckedMultiply(result, base)
			if !ok {
				var zero T
				return zero, false
			}
		}
		exponent >>= 1
		if exponent != 0 {
			var ok bool
			base, ok = CheckedMultiply(base, base)
			if !ok {
				var zero T
				return zero, false
			}
		}
	}
	return result, true
}

// SaturatingAdd returns a+b, clamped to the destination type's limits.
func SaturatingAdd[T Integer](a, b T) T {
	if result, ok := CheckedAdd(a, b); ok {
		return result
	}
	minimum, maximum := integerBounds[T]()
	if b > 0 {
		return maximum
	}
	return minimum
}

// SaturatingSubtract returns a-b, clamped to the destination type's limits.
func SaturatingSubtract[T Integer](a, b T) T {
	if result, ok := CheckedSubtract(a, b); ok {
		return result
	}
	minimum, maximum := integerBounds[T]()
	if b > 0 {
		return minimum
	}
	return maximum
}

// SaturatingMultiply returns a*b, clamped to the destination type's limits.
func SaturatingMultiply[T Integer](a, b T) T {
	if result, ok := CheckedMultiply(a, b); ok {
		return result
	}
	minimum, maximum := integerBounds[T]()
	if isSigned[T]() && (a < 0) != (b < 0) {
		return minimum
	}
	return maximum
}

// SaturatingPow returns base^exponent, clamped to the destination type's
// limits. A negative exponent is a programmer error and panics.
func SaturatingPow[T Integer](base T, exponent int) T {
	if result, ok := CheckedPow(base, exponent); ok {
		return result
	}
	minimum, maximum := integerBounds[T]()
	if isSigned[T]() && base < 0 && exponent&1 != 0 {
		return minimum
	}
	return maximum
}

// CheckedConvert converts value to To without silently wrapping.
func CheckedConvert[To, From Integer](value From) (To, bool) {
	var zero To
	if isSigned[From]() {
		signed := int64(value)
		if isSigned[To]() {
			minimum, maximum := signedBounds[To]()
			if integerBits[From]() > integerBits[To]() && (signed < int64(minimum) || signed > int64(maximum)) {
				return zero, false
			}
			return To(value), true
		}
		if signed < 0 || uint64(signed) > uint64(unsignedMax[To]()) {
			return zero, false
		}
		return To(value), true
	}
	unsigned := uint64(value)
	if isSigned[To]() {
		_, maximum := signedBounds[To]()
		if unsigned > uint64(maximum) {
			return zero, false
		}
		return To(value), true
	}
	if unsigned > uint64(unsignedMax[To]()) {
		return zero, false
	}
	return To(value), true
}

// SaturatingConvert converts value to To, clamping values outside its range.
func SaturatingConvert[To, From Integer](value From) To {
	if result, ok := CheckedConvert[To](value); ok {
		return result
	}
	if isSigned[From]() && int64(value) < 0 {
		minimum, _ := integerBounds[To]()
		return minimum
	}
	_, maximum := integerBounds[To]()
	return maximum
}

// Factorial returns n! and true when it fits in uint64. Negative inputs and
// results above MaxUint64 return false.
func Factorial(n int) (uint64, bool) {
	if n < 0 {
		return 0, false
	}
	result := uint64(1)
	for value := 2; value <= n; value++ {
		var ok bool
		result, ok = CheckedMultiply(result, uint64(value))
		if !ok {
			return 0, false
		}
	}
	return result, true
}

// Binomial returns n choose k and true when it fits in uint64. Invalid negative
// inputs return false; k>n returns zero and true.
func Binomial(n, k int) (uint64, bool) {
	if n < 0 || k < 0 {
		return 0, false
	}
	if k > n {
		return 0, true
	}
	if k > n-k {
		k = n - k
	}
	result := big.NewInt(1)
	for index := 0; index < k; index++ {
		result.Mul(result, big.NewInt(int64(n-index)))
		result.Quo(result, big.NewInt(int64(index+1)))
	}
	if !result.IsUint64() {
		return 0, false
	}
	return result.Uint64(), true
}
