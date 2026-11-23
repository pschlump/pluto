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
func IsPrime(value uint64) bool {
	if value < 2 {
		return false
	}
	for _, prime := range []uint64{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37} {
		if value%prime == 0 {
			return value == prime
		}
	}
	d := value - 1
	shift := bits.TrailingZeros64(d)
	d >>= shift
	for _, base := range []uint64{2, 325, 9375, 28178, 450775, 9780504, 1795265022} {
		if base%value == 0 {
			continue
		}
		x := powMod(base%value, d, value)
		if x == 1 || x == value-1 {
			continue
		}
		composite := true
		for range shift - 1 {
			x = mulMod(x, x, value)
			if x == value-1 {
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

func powMod(base, exponent, modulus uint64) uint64 {
	result := uint64(1)
	for exponent != 0 {
		if exponent&1 != 0 {
			result = mulMod(result, base, modulus)
		}
		exponent >>= 1
		if exponent != 0 {
			base = mulMod(base, base, modulus)
		}
	}
	return result
}

func mulMod(a, b, modulus uint64) uint64 {
	result := uint64(0)
	for b != 0 {
		if b&1 != 0 {
			if result >= modulus-a {
				result -= modulus - a
			} else {
				result += a
			}
		}
		b >>= 1
		if b == 0 {
			break
		}
		if a >= modulus-a {
			a -= modulus - a
		} else {
			a += a
		}
	}
	return result
}
