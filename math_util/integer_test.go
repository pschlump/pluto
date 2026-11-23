package math_util_test

import (
	"errors"
	"math"
	"testing"

	"github.com/pschlump/pluto/math_util"
)

func TestCheckedArithmeticExhaustiveInt8(t *testing.T) {
	for a := -128; a <= 127; a++ {
		for b := -128; b <= 127; b++ {
			x, y := int8(a), int8(b)
			sum, sumOK := math_util.CheckedAdd(x, y)
			if wantOK := a+b >= -128 && a+b <= 127; sumOK != wantOK || (wantOK && int(sum) != a+b) {
				t.Fatalf("CheckedAdd(%d,%d) = %d,%v", a, b, sum, sumOK)
			}
			difference, differenceOK := math_util.CheckedSubtract(x, y)
			if wantOK := a-b >= -128 && a-b <= 127; differenceOK != wantOK || (wantOK && int(difference) != a-b) {
				t.Fatalf("CheckedSubtract(%d,%d) = %d,%v", a, b, difference, differenceOK)
			}
			product, productOK := math_util.CheckedMultiply(x, y)
			if wantOK := a*b >= -128 && a*b <= 127; productOK != wantOK || (wantOK && int(product) != a*b) {
				t.Fatalf("CheckedMultiply(%d,%d) = %d,%v", a, b, product, productOK)
			}
			if got := math_util.SaturatingAdd(x, y); int(got) != max(-128, min(127, a+b)) {
				t.Fatalf("SaturatingAdd(%d,%d) = %d", a, b, got)
			}
			if got := math_util.SaturatingSubtract(x, y); int(got) != max(-128, min(127, a-b)) {
				t.Fatalf("SaturatingSubtract(%d,%d) = %d", a, b, got)
			}
			if got := math_util.SaturatingMultiply(x, y); int(got) != max(-128, min(127, a*b)) {
				t.Fatalf("SaturatingMultiply(%d,%d) = %d", a, b, got)
			}
		}
	}
}

func TestCheckedArithmeticExhaustiveUint8(t *testing.T) {
	for a := 0; a <= 255; a++ {
		for b := 0; b <= 255; b++ {
			x, y := uint8(a), uint8(b)
			sum, sumOK := math_util.CheckedAdd(x, y)
			if wantOK := a+b <= 255; sumOK != wantOK || (wantOK && int(sum) != a+b) {
				t.Fatalf("CheckedAdd(%d,%d) = %d,%v", a, b, sum, sumOK)
			}
			difference, differenceOK := math_util.CheckedSubtract(x, y)
			if wantOK := a >= b; differenceOK != wantOK || (wantOK && int(difference) != a-b) {
				t.Fatalf("CheckedSubtract(%d,%d) = %d,%v", a, b, difference, differenceOK)
			}
			product, productOK := math_util.CheckedMultiply(x, y)
			if wantOK := a*b <= 255; productOK != wantOK || (wantOK && int(product) != a*b) {
				t.Fatalf("CheckedMultiply(%d,%d) = %d,%v", a, b, product, productOK)
			}
			if got := math_util.SaturatingAdd(x, y); int(got) != min(255, a+b) {
				t.Fatalf("SaturatingAdd(%d,%d) = %d", a, b, got)
			}
			wantSubtraction := 0
			if a >= b {
				wantSubtraction = a - b
			}
			if got := math_util.SaturatingSubtract(x, y); int(got) != wantSubtraction {
				t.Fatalf("SaturatingSubtract(%d,%d) = %d", a, b, got)
			}
			if got := math_util.SaturatingMultiply(x, y); int(got) != min(255, a*b) {
				t.Fatalf("SaturatingMultiply(%d,%d) = %d", a, b, got)
			}
		}
	}
}

func TestCheckedBoundaries(t *testing.T) {
	if _, ok := math_util.CheckedMultiply(int64(math.MinInt64), -1); ok {
		t.Fatal("MinInt64 * -1 did not overflow")
	}
	if _, ok := math_util.CheckedMultiply(-1, int64(math.MinInt64)); ok {
		t.Fatal("-1 * MinInt64 did not overflow")
	}
	if got, ok := math_util.CheckedAdd(uint64(math.MaxUint64), 1); ok || got != 0 {
		t.Fatal("MaxUint64 + 1 did not fail cleanly")
	}
	if got := math_util.SaturatingMultiply(int64(math.MinInt64), 2); got != math.MinInt64 {
		t.Fatalf("signed saturation = %d", got)
	}
	if got := math_util.SaturatingMultiply(uint64(math.MaxUint64), 2); got != math.MaxUint64 {
		t.Fatalf("unsigned saturation = %d", got)
	}
}

func TestPow(t *testing.T) {
	for _, test := range []struct {
		base     int64
		exponent int
		want     int64
		ok       bool
	}{{2, 0, 1, true}, {-2, 3, -8, true}, {2, 62, 1 << 62, true}, {2, 63, 0, false}, {-2, 63, math.MinInt64, true}, {0, 0, 1, true}} {
		got, ok := math_util.CheckedPow(test.base, test.exponent)
		if got != test.want || ok != test.ok {
			t.Errorf("CheckedPow(%d,%d) = %d,%v; want %d,%v", test.base, test.exponent, got, ok, test.want, test.ok)
		}
	}
	if got := math_util.SaturatingPow(int8(-3), 5); got != -128 {
		t.Fatalf("negative odd saturation = %d", got)
	}
	if got := math_util.SaturatingPow(int8(-4), 4); got != 127 {
		t.Fatalf("negative even saturation = %d", got)
	}
	if got := math_util.SaturatingPow(uint8(4), 4); got != 255 {
		t.Fatalf("unsigned saturation = %d", got)
	}
}

func TestConversions(t *testing.T) {
	if _, ok := math_util.CheckedConvert[uint32](uint64(65536)); !ok {
		t.Fatal("65536 should fit uint32")
	}
	if _, ok := math_util.CheckedConvert[uint32](uint64(math.MaxUint32) + 1); ok {
		t.Fatal("2^32 should not fit uint32")
	}
	if got, ok := math_util.CheckedConvert[uint8](int16(255)); !ok || got != 255 {
		t.Fatalf("conversion = %d,%v", got, ok)
	}
	if _, ok := math_util.CheckedConvert[uint8](int16(-1)); ok {
		t.Fatal("negative to unsigned succeeded")
	}
	if _, ok := math_util.CheckedConvert[int8](uint64(128)); ok {
		t.Fatal("128 to int8 succeeded")
	}
	if got, ok := math_util.CheckedConvert[uint64](int64(math.MaxInt64)); !ok || got != math.MaxInt64 {
		t.Fatalf("wide conversion = %d,%v", got, ok)
	}
	if got := math_util.SaturatingConvert[uint8](int16(-9)); got != 0 {
		t.Fatalf("negative saturation = %d", got)
	}
	if got := math_util.SaturatingConvert[int8](uint16(999)); got != 127 {
		t.Fatalf("positive saturation = %d", got)
	}
	if got := math_util.SaturatingConvert[int8](int16(-999)); got != -128 {
		t.Fatalf("negative signed saturation = %d", got)
	}
}

func TestFactorialAndBinomial(t *testing.T) {
	if got, ok := math_util.Factorial(20); !ok || got != 2432902008176640000 {
		t.Fatalf("20! = %d,%v", got, ok)
	}
	if _, ok := math_util.Factorial(21); ok {
		t.Fatal("21! unexpectedly fits")
	}
	if _, ok := math_util.Factorial(-1); ok {
		t.Fatal("negative factorial succeeded")
	}
	if got, ok := math_util.Binomial(67, 33); !ok || got != 14226520737620288370 {
		t.Fatalf("67 choose 33 = %d,%v", got, ok)
	}
	if _, ok := math_util.Binomial(68, 34); ok {
		t.Fatal("68 choose 34 unexpectedly fits")
	}
	if got, ok := math_util.Binomial(5, 9); !ok || got != 0 {
		t.Fatalf("5 choose 9 = %d,%v", got, ok)
	}
}

func TestNumberUtilities(t *testing.T) {
	for _, test := range []struct {
		a, b, want int
	}{{-7, 3, 2}, {7, 3, 1}, {-7, -3, 0}, {0, 5, 0}} {
		got, err := math_util.Modulo(test.a, test.b)
		if test.b < 0 {
			if !errors.Is(err, math_util.ErrNegative) {
				t.Errorf("Modulo(%d,%d) error = %v", test.a, test.b, err)
			}
		} else if err != nil || got != test.want {
			t.Errorf("Modulo(%d,%d) = %d,%v", test.a, test.b, got, err)
		}
	}
	if got, err := math_util.GCD(int64(-54), 24); err != nil || got != 6 {
		t.Fatalf("GCD = %d,%v", got, err)
	}
	if _, err := math_util.GCD(int64(math.MinInt64), 0); !errors.Is(err, math_util.ErrOverflow) {
		t.Fatalf("GCD MinInt64 error = %v", err)
	}
	if got, err := math_util.GCD(uint64(math.MaxUint64), 5); err != nil || got != 5 {
		t.Fatalf("unsigned GCD = %d,%v", got, err)
	}
	for _, test := range []struct {
		a, b, want int64
	}{{math.MaxInt64, math.MaxInt64, math.MaxInt64}, {math.MinInt64, math.MaxInt64, -1}, {-4, -2, -3}, {-3, 2, -1}} {
		if got := math_util.Mean(test.a, test.b); got != test.want {
			t.Errorf("Mean(%d,%d) = %d, want %d", test.a, test.b, got, test.want)
		}
	}
	for _, value := range []uint64{1, 2, 4, 1 << 63} {
		if !math_util.IsPowerOfTwo(value) {
			t.Errorf("%d is power of two", value)
		}
	}
	for _, value := range []uint64{0, 3, 6, math.MaxUint64} {
		if math_util.IsPowerOfTwo(value) {
			t.Errorf("%d is not power of two", value)
		}
	}
	if math_util.IsPowerOfTwo(int64(math.MinInt64)) {
		t.Fatal("negative signed value is power of two")
	}
}

func TestPrimality(t *testing.T) {
	for _, prime := range []uint64{2, 3, 37, 7919, 2305843009213693951, 9223372036854775783, 18446744073709551557} {
		if !math_util.IsPrime(prime) {
			t.Errorf("%d is prime", prime)
		}
	}
	for _, composite := range []uint64{0, 1, 4, 341, 561, 1105, 3215031751, 3825123056546413051, math.MaxUint64} {
		if math_util.IsPrime(composite) {
			t.Errorf("%d is composite", composite)
		}
	}
}
