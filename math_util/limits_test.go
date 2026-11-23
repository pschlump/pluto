package math_util_test

import (
	"math"
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/pschlump/pluto/math_util"
)

func verifyIntegerLimits[T math_util.Integer](t *testing.T, minimum, maximum T) {
	t.Helper()
	if _, ok := math_util.CheckedAdd(maximum, 1); ok {
		t.Errorf("%T maximum + 1 succeeded", maximum)
	}
	if got := math_util.SaturatingAdd(maximum, 1); got != maximum {
		t.Errorf("%T saturating maximum + 1 = %v", maximum, got)
	}
	if _, ok := math_util.CheckedSubtract(minimum, 1); ok {
		t.Errorf("%T minimum - 1 succeeded", minimum)
	}
	if got := math_util.SaturatingSubtract(minimum, 1); got != minimum {
		t.Errorf("%T saturating minimum - 1 = %v", minimum, got)
	}
	if _, ok := math_util.CheckedMultiply(maximum, 2); ok {
		t.Errorf("%T maximum * 2 succeeded", maximum)
	}
	if got := math_util.SaturatingMultiply(maximum, 2); got != maximum {
		t.Errorf("%T saturating maximum * 2 = %v", maximum, got)
	}
	if _, ok := math_util.CheckedPow(maximum, 2); ok {
		t.Errorf("%T maximum squared succeeded", maximum)
	}
	if got, ok := math_util.CheckedConvert[T](maximum); !ok || got != maximum {
		t.Errorf("%T same-type conversion = %v,%v", maximum, got, ok)
	}
}

func TestEveryStandardIntegerLimit(t *testing.T) {
	verifyIntegerLimits(t, int(-int(^uint(0)>>1)-1), int(^uint(0)>>1))
	verifyIntegerLimits(t, int8(-128), int8(127))
	verifyIntegerLimits(t, int16(-32768), int16(32767))
	verifyIntegerLimits(t, int32(math.MinInt32), int32(math.MaxInt32))
	verifyIntegerLimits(t, int64(math.MinInt64), int64(math.MaxInt64))
	verifyIntegerLimits(t, uint(0), ^uint(0))
	verifyIntegerLimits(t, uint8(0), uint8(255))
	verifyIntegerLimits(t, uint16(0), uint16(65535))
	verifyIntegerLimits(t, uint32(0), uint32(math.MaxUint32))
	verifyIntegerLimits(t, uint64(0), uint64(math.MaxUint64))
	verifyIntegerLimits(t, uintptr(0), ^uintptr(0))
}

func TestInt64ArithmeticAgainstBigInt(t *testing.T) {
	random := rand.New(rand.NewPCG(1729, 64))
	values := []int64{math.MinInt64, math.MaxInt64, -1, 0, 1}
	for range 1000 {
		values = append(values, int64(random.Uint64()))
	}
	for range 1000 {
		a, b := values[random.IntN(len(values))], values[random.IntN(len(values))]
		for _, operation := range []string{"add", "subtract", "multiply"} {
			left, right := big.NewInt(a), big.NewInt(b)
			var expected *big.Int
			var got int64
			var ok bool
			switch operation {
			case "add":
				expected = new(big.Int).Add(left, right)
				got, ok = math_util.CheckedAdd(a, b)
			case "subtract":
				expected = new(big.Int).Sub(left, right)
				got, ok = math_util.CheckedSubtract(a, b)
			case "multiply":
				expected = new(big.Int).Mul(left, right)
				got, ok = math_util.CheckedMultiply(a, b)
			}
			if ok != expected.IsInt64() || (ok && got != expected.Int64()) {
				t.Fatalf("%s(%d,%d) = %d,%v; big.Int=%s", operation, a, b, got, ok, expected)
			}
		}
	}
}

func TestUint64ArithmeticAgainstBigInt(t *testing.T) {
	random := rand.New(rand.NewPCG(1729, 65))
	values := []uint64{0, 1, math.MaxUint64}
	for range 1000 {
		values = append(values, random.Uint64())
	}
	for range 1000 {
		a, b := values[random.IntN(len(values))], values[random.IntN(len(values))]
		left := new(big.Int).SetUint64(a)
		right := new(big.Int).SetUint64(b)
		for _, operation := range []string{"add", "subtract", "multiply"} {
			var expected *big.Int
			var got uint64
			var ok bool
			switch operation {
			case "add":
				expected = new(big.Int).Add(left, right)
				got, ok = math_util.CheckedAdd(a, b)
			case "subtract":
				expected = new(big.Int).Sub(left, right)
				got, ok = math_util.CheckedSubtract(a, b)
			case "multiply":
				expected = new(big.Int).Mul(left, right)
				got, ok = math_util.CheckedMultiply(a, b)
			}
			wantOK := expected.Sign() >= 0 && expected.IsUint64()
			if ok != wantOK || (ok && got != expected.Uint64()) {
				t.Fatalf("%s(%d,%d) = %d,%v; big.Int=%s", operation, a, b, got, ok, expected)
			}
		}
	}
}
