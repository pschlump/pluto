/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package math_util

import "math/bits"

// Modulo returns the nonnegative remainder of dividend modulo divisor.
// The divisor must be positive.
func Modulo[T Integer](dividend, divisor T) (T, error) {
	if divisor == 0 {
		return 0, ErrDivideByZero
	}
	if isSigned[T]() && divisor < 0 {
		return 0, ErrNegative
	}
	result := dividend % divisor
	if isSigned[T]() && result < 0 {
		result += divisor
	}
	return result, nil
}

// GCD returns the nonnegative greatest common divisor. A signed result that
// cannot represent the magnitude, such as gcd(MinInt64, 0), returns ErrOverflow.
func GCD[T Integer](a, b T) (T, error) {
	x, y := magnitude(a), magnitude(b)
	for y != 0 {
		x, y = y, x%y
	}
	if isSigned[T]() {
		_, maximum := signedBounds[T]()
		if x > uint64(maximum) {
			return 0, ErrOverflow
		}
	}
	return T(x), nil
}

func magnitude[T Integer](value T) uint64 {
	if !isSigned[T]() {
		return uint64(value)
	}
	signed := int64(value)
	if signed >= 0 {
		return uint64(signed)
	}
	return uint64(-(signed + 1)) + 1
}

// IsPowerOfTwo reports whether value is positive and has exactly one set bit.
func IsPowerOfTwo[T Integer](value T) bool {
	if isSigned[T]() && value < 0 {
		return false
	}
	unsigned := uint64(value)
	return unsigned != 0 && unsigned&(unsigned-1) == 0
}

// Mean returns the arithmetic mean rounded toward negative infinity without
// overflowing the input type.
func Mean[T Integer](a, b T) T {
	return (a & b) + ((a ^ b) >> 1)
}

// IsPrime performs deterministic Miller-Rabin primality testing over uint64.
// Complexity is O(log value) Montgomery multiplications per witness, seven
// witnesses at most.
func IsPrime(value uint64) bool {
	if value < 2 {
		return false
	}
	for _, prime := range []uint64{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37} {
		if value%prime == 0 {
			return value == prime
		}
	}
	// value is now odd and > 37: Montgomery arithmetic modulo value with
	// R = 2^64. Residues are kept in Montgomery form (x*R mod value), so
	// 1 and value-1 become one and minusOne.
	inverse := montgomeryInverse(value)
	one := bits.Rem64(1, 0, value)
	minusOne := value - one
	d := value - 1
	shift := bits.TrailingZeros64(d)
	d >>= shift
	for _, base := range []uint64{2, 325, 9375, 28178, 450775, 9780504, 1795265022} {
		reduced := base % value
		if reduced == 0 {
			continue
		}
		x := montgomeryPow(bits.Rem64(reduced, 0, value), d, one, value, inverse)
		if x == one || x == minusOne {
			continue
		}
		composite := true
		for range shift - 1 {
			x = montgomeryMul(x, x, value, inverse)
			if x == minusOne {
				composite = false
				break
			}
		}
		if composite {
			return false
		}
	}
	return true
}

// montgomeryInverse returns value^-1 mod 2^64 for odd value by Newton
// iteration: value*value ≡ 1 (mod 8) gives 3 correct bits and each step
// doubles them (3, 6, 12, 24, 48, 96).
func montgomeryInverse(value uint64) uint64 {
	inverse := value
	for range 5 {
		inverse *= 2 - value*inverse
	}
	return inverse
}

// montgomeryMul returns a*b*2^-64 mod modulus for a, b < modulus and odd
// modulus, where inverse = modulus^-1 mod 2^64. With m = lo*inverse the low
// words of a*b and m*modulus are equal, so (a*b - m*modulus)/2^64 is
// hi - mhi exactly, which lies in (-modulus, modulus).
func montgomeryMul(a, b, modulus, inverse uint64) uint64 {
	hi, lo := bits.Mul64(a, b)
	mhi, _ := bits.Mul64(lo*inverse, modulus)
	result := hi - mhi
	if hi < mhi {
		result += modulus
	}
	return result
}

// montgomeryPow returns base^exponent in Montgomery form; base and one are
// in Montgomery form.
func montgomeryPow(base, exponent, one, modulus, inverse uint64) uint64 {
	result := one
	for exponent != 0 {
		if exponent&1 != 0 {
			result = montgomeryMul(result, base, modulus, inverse)
		}
		exponent >>= 1
		if exponent != 0 {
			base = montgomeryMul(base, base, modulus, inverse)
		}
	}
	return result
}
