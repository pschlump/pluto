package math_util_test

import (
	"errors"
	"math"
	"testing"

	"github.com/pschlump/pluto/math_util"
)

func TestDivideRounding(t *testing.T) {
	tests := []struct {
		p, q int
		mode math_util.RoundingMode
		want int
	}{
		{7, 3, math_util.RoundFloor, 2}, {7, 3, math_util.RoundCeiling, 3}, {7, 3, math_util.RoundTowardZero, 2}, {7, 3, math_util.RoundAwayFromZero, 3},
		{-7, 3, math_util.RoundFloor, -3}, {-7, 3, math_util.RoundCeiling, -2}, {-7, 3, math_util.RoundTowardZero, -2}, {-7, 3, math_util.RoundAwayFromZero, -3},
		{7, -3, math_util.RoundFloor, -3}, {7, -3, math_util.RoundCeiling, -2}, {-7, -3, math_util.RoundFloor, 2}, {-7, -3, math_util.RoundCeiling, 3},
		{5, 2, math_util.RoundHalfUp, 3}, {5, 2, math_util.RoundHalfDown, 2}, {5, 2, math_util.RoundHalfEven, 2}, {7, 2, math_util.RoundHalfEven, 4},
		{-5, 2, math_util.RoundHalfUp, -3}, {-5, 2, math_util.RoundHalfDown, -2}, {6, 4, math_util.RoundHalfEven, 2},
		{6, 3, math_util.RoundExact, 2}, {6, 3, math_util.RoundHalfDown, 2},
	}
	for _, test := range tests {
		got, err := math_util.Divide(test.p, test.q, test.mode)
		if err != nil || got != test.want {
			t.Errorf("Divide(%d,%d,%d) = %d,%v; want %d", test.p, test.q, test.mode, got, err, test.want)
		}
	}
	if _, err := math_util.Divide(1, 2, math_util.RoundExact); !errors.Is(err, math_util.ErrInexact) {
		t.Fatalf("inexact error = %v", err)
	}
	if _, err := math_util.Divide(1, 0, math_util.RoundFloor); !errors.Is(err, math_util.ErrDivideByZero) {
		t.Fatalf("zero divisor error = %v", err)
	}
	if _, err := math_util.Divide(int64(math.MinInt64), -1, math_util.RoundFloor); !errors.Is(err, math_util.ErrOverflow) {
		t.Fatalf("overflow error = %v", err)
	}
	if got, err := math_util.Divide(uint(7), 3, math_util.RoundCeiling); err != nil || got != 3 {
		t.Fatalf("unsigned divide = %d,%v", got, err)
	}
	if _, err := math_util.Divide(1, 1, math_util.RoundingMode(99)); !errors.Is(err, math_util.ErrRoundingMode) {
		t.Fatalf("invalid mode error = %v", err)
	}
}

func TestLogRounding(t *testing.T) {
	for _, test := range []struct {
		value, floor, ceiling, nearest int
	}{{1, 0, 0, 0}, {2, 1, 1, 1}, {3, 1, 2, 2}, {4, 2, 2, 2}, {5, 2, 3, 2}, {6, 2, 3, 3}, {7, 2, 3, 3}, {8, 3, 3, 3}} {
		floor, err := math_util.Log2(test.value, math_util.RoundFloor)
		if err != nil || floor != test.floor {
			t.Errorf("Log2 floor(%d) = %d,%v", test.value, floor, err)
		}
		ceiling, err := math_util.Log2(test.value, math_util.RoundCeiling)
		if err != nil || ceiling != test.ceiling {
			t.Errorf("Log2 ceiling(%d) = %d,%v", test.value, ceiling, err)
		}
		nearest, err := math_util.Log2(test.value, math_util.RoundHalfEven)
		if err != nil || nearest != test.nearest {
			t.Errorf("Log2 nearest(%d) = %d,%v", test.value, nearest, err)
		}
	}
	for _, test := range []struct {
		value, floor, ceiling, nearest int
	}{{1, 0, 0, 0}, {3, 0, 1, 0}, {4, 0, 1, 1}, {10, 1, 1, 1}, {31, 1, 2, 1}, {32, 1, 2, 2}, {100, 2, 2, 2}} {
		floor, _ := math_util.Log10(test.value, math_util.RoundFloor)
		ceiling, _ := math_util.Log10(test.value, math_util.RoundCeiling)
		nearest, _ := math_util.Log10(test.value, math_util.RoundHalfEven)
		if floor != test.floor || ceiling != test.ceiling || nearest != test.nearest {
			t.Errorf("Log10(%d) = %d,%d,%d; want %d,%d,%d", test.value, floor, ceiling, nearest, test.floor, test.ceiling, test.nearest)
		}
	}
	if _, err := math_util.Log2(0, math_util.RoundFloor); !errors.Is(err, math_util.ErrNonPositive) {
		t.Fatalf("Log2(0) error = %v", err)
	}
	if _, err := math_util.Log10(-1, math_util.RoundFloor); !errors.Is(err, math_util.ErrNonPositive) {
		t.Fatalf("Log10(-1) error = %v", err)
	}
	if _, err := math_util.Log2(3, math_util.RoundExact); !errors.Is(err, math_util.ErrInexact) {
		t.Fatalf("Log2 inexact error = %v", err)
	}
}

func TestSqrtRounding(t *testing.T) {
	for _, test := range []struct {
		value, floor, ceiling, nearest uint64
	}{{0, 0, 0, 0}, {1, 1, 1, 1}, {2, 1, 2, 1}, {3, 1, 2, 2}, {4, 2, 2, 2}, {15, 3, 4, 4}, {16, 4, 4, 4}, {17, 4, 5, 4}, {24, 4, 5, 5}, {25, 5, 5, 5}, {26, 5, 6, 5}, {math.MaxUint64, 4294967295, 4294967296, 4294967296}} {
		floor, err := math_util.Sqrt(test.value, math_util.RoundFloor)
		if err != nil || floor != test.floor {
			t.Errorf("Sqrt floor(%d) = %d,%v", test.value, floor, err)
		}
		ceiling, err := math_util.Sqrt(test.value, math_util.RoundCeiling)
		if err != nil || ceiling != test.ceiling {
			t.Errorf("Sqrt ceiling(%d) = %d,%v", test.value, ceiling, err)
		}
		nearest, err := math_util.Sqrt(test.value, math_util.RoundHalfEven)
		if err != nil || nearest != test.nearest {
			t.Errorf("Sqrt nearest(%d) = %d,%v", test.value, nearest, err)
		}
	}
	if got, err := math_util.Sqrt(uint32(math.MaxUint32), math_util.RoundFloor); err != nil || got != 65535 {
		t.Fatalf("uint32 floor = %d,%v", got, err)
	}
	if got, err := math_util.Sqrt(uint32(math.MaxUint32), math_util.RoundCeiling); err != nil || got != 65536 {
		t.Fatalf("uint32 ceiling = %d, error = %v", got, err)
	}
	if _, err := math_util.Sqrt(-1, math_util.RoundFloor); !errors.Is(err, math_util.ErrNegative) {
		t.Fatalf("negative error = %v", err)
	}
	if _, err := math_util.Sqrt(2, math_util.RoundExact); !errors.Is(err, math_util.ErrInexact) {
		t.Fatalf("inexact error = %v", err)
	}
}
