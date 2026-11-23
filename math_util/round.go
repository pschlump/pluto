/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package math_util

import (
	"math"
	"math/big"
	"math/bits"
)

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
	if remainder == 0 {
		return quotient, checkMode(mode)
	}
	increment, err := shouldRound(divisor, remainder, quotient, mode)
	if err != nil || !increment {
		return quotient, err
	}
	if sameSign(remainder, divisor) {
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

func shouldRound[T Integer](divisor, remainder, quotient T, mode RoundingMode) (bool, error) {
	if err := checkMode(mode); err != nil {
		return false, err
	}
	same := sameSign(remainder, divisor)
	switch mode {
	case RoundFloor:
		return !same, nil
	case RoundCeiling:
		return same, nil
	case RoundTowardZero:
		return false, nil
	case RoundAwayFromZero:
		return true, nil
	case RoundExact:
		return false, ErrInexact
	}
	comparison := compareTwiceMagnitude(remainder, divisor)
	switch mode {
	case RoundHalfUp:
		return comparison >= 0, nil
	case RoundHalfDown:
		return comparison > 0, nil
	case RoundHalfEven:
		return comparison > 0 || (comparison == 0 && quotient&1 != 0), nil
	}
	return false, ErrRoundingMode
}

func compareTwiceMagnitude[T Integer](a, b T) int {
	x, y := magnitude(a), magnitude(b)
	half := y / 2
	if y&1 != 0 {
		if x <= half {
			return -1
		}
		return 1
	}
	if x < half {
		return -1
	}
	if x > half {
		return 1
	}
	return 0
}

// Log2 returns the base-2 logarithm of a positive integer with explicit
// rounding. Half modes compare exactly against the irrational midpoint.
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
	left := new(big.Int).SetUint64(unsigned)
	left.Mul(left, left)
	right := new(big.Int).Lsh(big.NewInt(1), uint(2*floor+1))
	if roundHalf(mode, left.Cmp(right), floor&1 != 0) {
		return floor + 1, nil
	}
	return floor, nil
}

// Log10 returns the base-10 logarithm of a positive integer with explicit
// rounding. Half modes compare exactly against the irrational midpoint.
func Log10[T Integer](value T, mode RoundingMode) (int, error) {
	if value == 0 || (isSigned[T]() && value < 0) {
		return 0, ErrNonPositive
	}
	if err := checkMode(mode); err != nil {
		return 0, err
	}
	unsigned := magnitude(value)
	floor, power := 0, uint64(1)
	for power <= unsigned/10 {
		power *= 10
		floor++
	}
	if unsigned == power || mode == RoundFloor || mode == RoundTowardZero {
		return floor, nil
	}
	if mode == RoundExact {
		return 0, ErrInexact
	}
	if mode == RoundCeiling || mode == RoundAwayFromZero {
		return floor + 1, nil
	}
	left := new(big.Int).SetUint64(unsigned)
	left.Mul(left, left)
	right := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(2*floor+1)), nil)
	if roundHalf(mode, left.Cmp(right), floor&1 != 0) {
		return floor + 1, nil
	}
	return floor, nil
}

// Sqrt returns the integer square root of a nonnegative integer with explicit
// rounding. An overflowing rounded result returns ErrOverflow.
func Sqrt[T Integer](value T, mode RoundingMode) (T, error) {
	if isSigned[T]() && value < 0 {
		return 0, ErrNegative
	}
	if err := checkMode(mode); err != nil {
		return 0, err
	}
	unsigned := magnitude(value)
	root := uint64(math.Sqrt(float64(unsigned)))
	for root > 0 && root > unsigned/root {
		root--
	}
	for root+1 <= unsigned/(root+1) {
		root++
	}
	exact := false
	if root == 0 {
		exact = unsigned == 0
	} else {
		exact = root == unsigned/root && unsigned%root == 0
	}
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

func roundHalf(mode RoundingMode, comparison int, odd bool) bool {
	switch mode {
	case RoundHalfUp:
		return comparison >= 0
	case RoundHalfDown:
		return comparison > 0
	case RoundHalfEven:
		return comparison > 0 || (comparison == 0 && odd)
	}
	return false
}
