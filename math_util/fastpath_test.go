/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package math_util_test

import (
	"errors"
	"math"
	"math/big"
	"math/bits"
	"math/rand/v2"
	"testing"

	"github.com/pschlump/pluto/math_util"
)

// The functions below are the pre-2026-11-27 math/big implementations of the
// half-mode logarithms and Binomial, plus the bit-serial Miller-Rabin
// multiplication. They are kept verbatim (modulo the generic wrapper) as
// trusted oracles for the word-size fast paths.

func oracleRoundHalf(mode math_util.RoundingMode, comparison int, odd bool) bool {
	switch mode {
	case math_util.RoundHalfUp:
		return comparison >= 0
	case math_util.RoundHalfDown:
		return comparison > 0
	case math_util.RoundHalfEven:
		return comparison > 0 || (comparison == 0 && odd)
	}
	return false
}

func oracleLog2(unsigned uint64, mode math_util.RoundingMode) (int, error) {
	if unsigned == 0 {
		return 0, math_util.ErrNonPositive
	}
	if mode < math_util.RoundFloor || mode > math_util.RoundExact {
		return 0, math_util.ErrRoundingMode
	}
	floor := bits.Len64(unsigned) - 1
	exact := unsigned&(unsigned-1) == 0
	if exact || mode == math_util.RoundFloor || mode == math_util.RoundTowardZero {
		return floor, nil
	}
	if mode == math_util.RoundExact {
		return 0, math_util.ErrInexact
	}
	if mode == math_util.RoundCeiling || mode == math_util.RoundAwayFromZero {
		return floor + 1, nil
	}
	left := new(big.Int).SetUint64(unsigned)
	left.Mul(left, left)
	right := new(big.Int).Lsh(big.NewInt(1), uint(2*floor+1))
	if oracleRoundHalf(mode, left.Cmp(right), floor&1 != 0) {
		return floor + 1, nil
	}
	return floor, nil
}

func oracleLog10(unsigned uint64, mode math_util.RoundingMode) (int, error) {
	if unsigned == 0 {
		return 0, math_util.ErrNonPositive
	}
	if mode < math_util.RoundFloor || mode > math_util.RoundExact {
		return 0, math_util.ErrRoundingMode
	}
	floor, power := 0, uint64(1)
	for power <= unsigned/10 {
		power *= 10
		floor++
	}
	if unsigned == power || mode == math_util.RoundFloor || mode == math_util.RoundTowardZero {
		return floor, nil
	}
	if mode == math_util.RoundExact {
		return 0, math_util.ErrInexact
	}
	if mode == math_util.RoundCeiling || mode == math_util.RoundAwayFromZero {
		return floor + 1, nil
	}
	left := new(big.Int).SetUint64(unsigned)
	left.Mul(left, left)
	right := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(2*floor+1)), nil)
	if oracleRoundHalf(mode, left.Cmp(right), floor&1 != 0) {
		return floor + 1, nil
	}
	return floor, nil
}

func oracleBinomial(n, k int) (uint64, bool) {
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

// oracleMulMod is the original bit-serial (a*b) mod m; it requires a,b < m.
func oracleMulMod(a, b, modulus uint64) uint64 {
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

func oracleIsPrime(value uint64) bool {
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
		x := uint64(1)
		b, e := base%value, d
		for e != 0 {
			if e&1 != 0 {
				x = oracleMulMod(x, b, value)
			}
			e >>= 1
			if e != 0 {
				b = oracleMulMod(b, b, value)
			}
		}
		if x == 1 || x == value-1 {
			continue
		}
		composite := true
		for range shift - 1 {
			x = oracleMulMod(x, x, value)
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

var allModes = []math_util.RoundingMode{
	math_util.RoundFloor, math_util.RoundCeiling, math_util.RoundTowardZero,
	math_util.RoundAwayFromZero, math_util.RoundHalfEven, math_util.RoundHalfUp,
	math_util.RoundHalfDown, math_util.RoundExact, math_util.RoundingMode(-1),
	math_util.RoundingMode(99),
}

// logProbeValues returns every power of two and ten, each midpoint
// threshold floor(sqrt(2^(2f+1))) and floor(sqrt(10^(2f+1))), each ±2, the
// small range 0..5000, MaxUint64 and a seeded random sample spread over
// every bit length.
func logProbeValues() []uint64 {
	var values []uint64
	add := func(center uint64) {
		for delta := uint64(0); delta <= 2; delta++ {
			values = append(values, center+delta, center-delta)
		}
	}
	for value := uint64(0); value <= 5000; value++ {
		values = append(values, value)
	}
	for shift := 0; shift < 64; shift++ {
		add(uint64(1) << shift)
		midpoint := new(big.Int).Sqrt(new(big.Int).Lsh(big.NewInt(1), uint(2*shift+1)))
		add(midpoint.Uint64())
	}
	ten := big.NewInt(10)
	power := big.NewInt(1)
	for exponent := 0; exponent <= 19; exponent++ {
		add(power.Uint64())
		midpoint := new(big.Int).Sqrt(new(big.Int).Mul(new(big.Int).Mul(power, power), ten))
		if midpoint.IsUint64() {
			add(midpoint.Uint64())
		}
		power.Mul(power, ten)
	}
	add(math.MaxUint64)
	random := rand.New(rand.NewPCG(20261127, 10))
	for range 200000 {
		values = append(values, random.Uint64()>>random.IntN(64))
	}
	return values
}

func sameResult(gotValue int, gotErr error, wantValue int, wantErr error) bool {
	if wantErr != nil {
		return errors.Is(gotErr, wantErr) && gotValue == wantValue
	}
	return gotErr == nil && gotValue == wantValue
}

func TestLogFastPathsMatchBigIntOracle(t *testing.T) {
	for _, value := range logProbeValues() {
		for _, mode := range allModes {
			want2, wantErr2 := oracleLog2(value, mode)
			got2, gotErr2 := math_util.Log2(value, mode)
			if !sameResult(got2, gotErr2, want2, wantErr2) {
				t.Fatalf("Log2(%d,%d) = %d,%v; oracle %d,%v", value, mode, got2, gotErr2, want2, wantErr2)
			}
			want10, wantErr10 := oracleLog10(value, mode)
			got10, gotErr10 := math_util.Log10(value, mode)
			if !sameResult(got10, gotErr10, want10, wantErr10) {
				t.Fatalf("Log10(%d,%d) = %d,%v; oracle %d,%v", value, mode, got10, gotErr10, want10, wantErr10)
			}
			if value <= math.MaxInt64 {
				signed := int64(value)
				if got, err := math_util.Log10(signed, mode); !sameResult(got, err, want10, wantErr10) {
					t.Fatalf("Log10(int64 %d,%d) = %d,%v; oracle %d,%v", signed, mode, got, err, want10, wantErr10)
				}
				if got, err := math_util.Log2(signed, mode); !sameResult(got, err, want2, wantErr2) {
					t.Fatalf("Log2(int64 %d,%d) = %d,%v; oracle %d,%v", signed, mode, got, err, want2, wantErr2)
				}
			}
		}
	}
}

func TestLogFastPathsExhaustiveSmallTypes(t *testing.T) {
	for value := 0; value <= math.MaxUint16; value++ {
		for _, mode := range allModes {
			want2, wantErr2 := oracleLog2(uint64(value), mode)
			want10, wantErr10 := oracleLog10(uint64(value), mode)
			if got, err := math_util.Log2(uint16(value), mode); !sameResult(got, err, want2, wantErr2) {
				t.Fatalf("Log2(uint16 %d,%d) = %d,%v; oracle %d,%v", value, mode, got, err, want2, wantErr2)
			}
			if got, err := math_util.Log10(uint16(value), mode); !sameResult(got, err, want10, wantErr10) {
				t.Fatalf("Log10(uint16 %d,%d) = %d,%v; oracle %d,%v", value, mode, got, err, want10, wantErr10)
			}
		}
	}
	for value := math.MinInt16; value < 0; value++ {
		for _, mode := range allModes {
			if _, err := math_util.Log10(int16(value), mode); !errors.Is(err, math_util.ErrNonPositive) {
				t.Fatalf("Log10(int16 %d) error = %v", value, err)
			}
			if _, err := math_util.Log2(int16(value), mode); !errors.Is(err, math_util.ErrNonPositive) {
				t.Fatalf("Log2(int16 %d) error = %v", value, err)
			}
		}
	}
}

func TestBinomialFastPathMatchesBigIntOracle(t *testing.T) {
	check := func(n, k int) {
		t.Helper()
		want, wantOK := oracleBinomial(n, k)
		got, ok := math_util.Binomial(n, k)
		if got != want || ok != wantOK {
			t.Fatalf("Binomial(%d,%d) = %d,%v; oracle %d,%v", n, k, got, ok, want, wantOK)
		}
	}
	// Every (n, k) through the table edge and well past it.
	for n := -3; n <= 300; n++ {
		for k := -3; k <= n+3; k++ {
			check(n, k)
		}
	}
	// Large n with small and complementary k: around the overflow frontier
	// for each k, where C(n, k) crosses 2^64.
	for k := 1; k <= 34; k++ {
		low, high := k, 1<<40
		for low < high { // smallest n with overflow, by bisection on the oracle
			middle := low + (high-low)/2
			if _, ok := oracleBinomial(middle, k); ok {
				low = middle + 1
			} else {
				high = middle
			}
		}
		for n := low - 3; n <= low+3; n++ {
			check(n, k)
			check(n, n-k)
		}
	}
	random := rand.New(rand.NewPCG(20261127, 11))
	for range 20000 {
		n := random.IntN(1 << random.IntN(40))
		k := random.IntN(70)
		check(n, k)
		check(n, n-k)
	}
	check(math.MaxInt64, 1)
	check(math.MaxInt64, 0)
	check(math.MaxInt64, math.MaxInt64)
	check(math.MaxInt64, math.MaxInt64-1)
	check(math.MaxInt64, 2)
	check(math.MaxInt32, 2)
}

func TestIsPrimeMatchesBitSerialOracle(t *testing.T) {
	values := []uint64{math.MaxUint64, math.MaxUint64 - 1, 1<<63 - 25, 1<<63 + 1, 18446744073709551557,
		3215031751, 3825123056546413051, 4294967291 * 4294967279, 4294967311,
	}
	for value := uint64(0); value < 200000; value++ {
		values = append(values, value)
	}
	random := rand.New(rand.NewPCG(20261127, 12))
	for range 20000 {
		values = append(values, random.Uint64()|1, random.Uint64()>>random.IntN(64))
	}
	for _, value := range values {
		if got, want := math_util.IsPrime(value), oracleIsPrime(value); got != want {
			t.Fatalf("IsPrime(%d) = %v; oracle %v", value, got, want)
		}
	}
}

func oracleIsSigned[T math_util.Integer]() bool {
	var zero T
	return ^zero < zero
}

func oracleMagnitude[T math_util.Integer](value T) uint64 {
	if !oracleIsSigned[T]() {
		return uint64(value)
	}
	signed := int64(value)
	if signed >= 0 {
		return uint64(signed)
	}
	return uint64(-(signed + 1)) + 1
}

func oracleSameSign[T math_util.Integer](a, b T) bool {
	if !oracleIsSigned[T]() {
		return true
	}
	return (a < 0) == (b < 0)
}

func oracleCheckMode(mode math_util.RoundingMode) error {
	if mode < math_util.RoundFloor || mode > math_util.RoundExact {
		return math_util.ErrRoundingMode
	}
	return nil
}

func oracleShouldRound[T math_util.Integer](divisor, remainder, quotient T, mode math_util.RoundingMode) (bool, error) {
	if err := oracleCheckMode(mode); err != nil {
		return false, err
	}
	same := oracleSameSign(remainder, divisor)
	switch mode {
	case math_util.RoundFloor:
		return !same, nil
	case math_util.RoundCeiling:
		return same, nil
	case math_util.RoundTowardZero:
		return false, nil
	case math_util.RoundAwayFromZero:
		return true, nil
	case math_util.RoundExact:
		return false, math_util.ErrInexact
	}
	x, y := oracleMagnitude(remainder), oracleMagnitude(divisor)
	comparison := 0
	half := y / 2
	if y&1 != 0 {
		comparison = 1
		if x <= half {
			comparison = -1
		}
	} else if x < half {
		comparison = -1
	} else if x > half {
		comparison = 1
	}
	switch mode {
	case math_util.RoundHalfUp:
		return comparison >= 0, nil
	case math_util.RoundHalfDown:
		return comparison > 0, nil
	case math_util.RoundHalfEven:
		return comparison > 0 || (comparison == 0 && quotient&1 != 0), nil
	}
	return false, math_util.ErrRoundingMode
}

// oracleDivide is the pre-2026-11-27 Divide (three call layers).
func oracleDivide[T math_util.Integer](dividend, divisor T, mode math_util.RoundingMode) (T, error) {
	if divisor == 0 {
		return 0, math_util.ErrDivideByZero
	}
	if oracleIsSigned[T]() && divisor == ^T(0) && dividend < 0 && dividend-1 > dividend {
		return 0, math_util.ErrOverflow
	}
	quotient := dividend / divisor
	remainder := dividend % divisor
	if remainder == 0 {
		return quotient, oracleCheckMode(mode)
	}
	increment, err := oracleShouldRound(divisor, remainder, quotient, mode)
	if err != nil || !increment {
		return quotient, err
	}
	if oracleSameSign(remainder, divisor) {
		result, ok := math_util.CheckedAdd(quotient, T(1))
		if !ok {
			return 0, math_util.ErrOverflow
		}
		return result, nil
	}
	result, ok := math_util.CheckedSubtract(quotient, T(1))
	if !ok {
		return 0, math_util.ErrOverflow
	}
	return result, nil
}

// oracleSqrt is the pre-2026-11-27 division-corrected Sqrt.
func oracleSqrt[T math_util.Integer](value T, mode math_util.RoundingMode) (T, error) {
	if oracleIsSigned[T]() && value < 0 {
		return 0, math_util.ErrNegative
	}
	if err := oracleCheckMode(mode); err != nil {
		return 0, err
	}
	unsigned := oracleMagnitude(value)
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
	if !exact && mode == math_util.RoundExact {
		return 0, math_util.ErrInexact
	}
	if !exact && mode != math_util.RoundFloor && mode != math_util.RoundTowardZero {
		roundUp := mode == math_util.RoundCeiling || mode == math_util.RoundAwayFromZero
		if !roundUp {
			roundUp = unsigned > root*(root+1)
		}
		if roundUp {
			root++
		}
	}
	result, ok := math_util.CheckedConvert[T](root)
	if !ok {
		return 0, math_util.ErrOverflow
	}
	return result, nil
}

func checkSqrt[T math_util.Integer](t *testing.T, value T) {
	t.Helper()
	for _, mode := range allModes {
		want, wantErr := oracleSqrt(value, mode)
		got, err := math_util.Sqrt(value, mode)
		if got != want || !errors.Is(err, wantErr) || (wantErr == nil) != (err == nil) {
			t.Fatalf("Sqrt[%T](%d,%d) = %d,%v; oracle %d,%v", value, value, mode, got, err, want, wantErr)
		}
	}
}

func TestSqrtMatchesOracle(t *testing.T) {
	for value := 0; value <= math.MaxUint16; value++ {
		checkSqrt(t, uint16(value))
		checkSqrt(t, int16(value))
		checkSqrt(t, uint64(value))
	}
	roots := []uint64{1, 2, 3, 1 << 16, 1<<16 - 1, 1<<31 - 1, 1 << 31, 3037000499, 3037000500, math.MaxUint32 - 1, math.MaxUint32}
	random := rand.New(rand.NewPCG(20261127, 14))
	for range 50000 {
		roots = append(roots, random.Uint64()>>(32+random.IntN(32)))
	}
	for _, root := range roots {
		// Squares, the half-mode boundary root*(root+1), and neighbours.
		for _, center := range []uint64{root * root, root * (root + 1)} {
			for delta := uint64(0); delta <= 2; delta++ {
				for _, value := range []uint64{center + delta, center - delta} {
					checkSqrt(t, value)
					checkSqrt(t, int64(value))
					checkSqrt(t, uint32(value))
					checkSqrt(t, int32(value))
				}
			}
		}
	}
	for range 100000 {
		value := random.Uint64() >> random.IntN(64)
		checkSqrt(t, value)
		checkSqrt(t, int64(value))
	}
	for _, value := range []uint64{0, math.MaxUint64, math.MaxUint64 - 1, math.MaxInt64, 1 << 63} {
		checkSqrt(t, value)
		checkSqrt(t, int64(value))
	}
}

func checkDivide[T math_util.Integer](t *testing.T, dividend, divisor T) {
	t.Helper()
	for _, mode := range allModes {
		want, wantErr := oracleDivide(dividend, divisor, mode)
		got, err := math_util.Divide(dividend, divisor, mode)
		if got != want || !errors.Is(err, wantErr) || (wantErr == nil) != (err == nil) {
			t.Fatalf("Divide[%T](%d,%d,%d) = %d,%v; oracle %d,%v", dividend, dividend, divisor, mode, got, err, want, wantErr)
		}
	}
}

func TestDivideMatchesOracleExhaustive8Bit(t *testing.T) {
	for a := math.MinInt8; a <= math.MaxInt8; a++ {
		for b := math.MinInt8; b <= math.MaxInt8; b++ {
			checkDivide(t, int8(a), int8(b))
		}
	}
	for a := 0; a <= math.MaxUint8; a++ {
		for b := 0; b <= math.MaxUint8; b++ {
			checkDivide(t, uint8(a), uint8(b))
		}
	}
}

func TestDivideMatchesOracleWide(t *testing.T) {
	edges := []int64{math.MinInt64, math.MinInt64 + 1, -3, -2, -1, 0, 1, 2, 3, 37, math.MaxInt64 - 1, math.MaxInt64}
	for _, a := range edges {
		for _, b := range edges {
			checkDivide(t, a, b)
			checkDivide(t, uint64(a), uint64(b))
		}
	}
	random := rand.New(rand.NewPCG(20261127, 13))
	for range 100000 {
		a, b := random.Uint64()>>random.IntN(64), random.Uint64()>>random.IntN(64)
		checkDivide(t, a, b)
		checkDivide(t, int64(a), int64(b))
		checkDivide(t, -int64(a), int64(b))
		checkDivide(t, int32(a), int32(b))
		// Exact halves: even divisor, remainder exactly half of it.
		if even := b &^ 1; even != 0 && even < 1<<40 {
			quotient := a % (1 << 20)
			checkDivide(t, quotient*even+even/2, even)
			checkDivide(t, -int64(quotient*even+even/2), int64(even))
			checkDivide(t, int64(quotient*even+even/2), -int64(even))
		}
	}
}
