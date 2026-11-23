package math_util_test

import (
	"testing"

	"github.com/pschlump/pluto/math_util"
)

var (
	mathIntSink  int64
	mathBoolSink bool
)

func BenchmarkMathDivide(b *testing.B) {
	for range b.N {
		mathIntSink, _ = math_util.Divide(int64(9223372036854775806), 37, math_util.RoundHalfEven)
	}
}

func BenchmarkMathLog10(b *testing.B) {
	for range b.N {
		value, _ := math_util.Log10(uint64(9223372036854775783), math_util.RoundHalfEven)
		mathIntSink = int64(value)
	}
}

func BenchmarkMathSqrt(b *testing.B) {
	for range b.N {
		value, _ := math_util.Sqrt(uint64(9223372036854775783), math_util.RoundHalfEven)
		mathIntSink = int64(value)
	}
}

func BenchmarkMathIsPrime(b *testing.B) {
	for range b.N {
		mathBoolSink = math_util.IsPrime(9223372036854775783)
	}
}

func BenchmarkMathBinomial(b *testing.B) {
	for range b.N {
		value, _ := math_util.Binomial(30, 15)
		mathIntSink = int64(value)
	}
}
