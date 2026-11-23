package math_util_test

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/pschlump/pluto/math_util"
)

type guavaMathVector struct {
	Operation string `json:"operation"`
	A         string `json:"a"`
	B         string `json:"b"`
	Mode      string `json:"mode"`
	Result    string `json:"result"`
	OK        bool   `json:"ok"`
}

func guavaRoundingMode(mode string) math_util.RoundingMode {
	switch mode {
	case "":
		return math_util.RoundTowardZero
	case "FLOOR":
		return math_util.RoundFloor
	case "CEILING":
		return math_util.RoundCeiling
	case "DOWN":
		return math_util.RoundTowardZero
	case "UP":
		return math_util.RoundAwayFromZero
	case "HALF_EVEN":
		return math_util.RoundHalfEven
	case "HALF_UP":
		return math_util.RoundHalfUp
	case "HALF_DOWN":
		return math_util.RoundHalfDown
	case "UNNECESSARY":
		return math_util.RoundExact
	default:
		panic("unknown Guava rounding mode " + mode)
	}
}

func TestGuavaMathVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/guava_math.json")
	if err != nil {
		t.Fatal(err)
	}
	var root struct {
		Vectors []guavaMathVector `json:"vectors"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	for _, vector := range root.Vectors {
		t.Run(vector.Operation+"/"+vector.A+"/"+vector.B+"/"+vector.Mode, func(t *testing.T) {
			a, err := strconv.ParseInt(vector.A, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			b, err := strconv.ParseInt(vector.B, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			want, err := strconv.ParseInt(vector.Result, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := executeGuavaMath(vector.Operation, a, b, guavaRoundingMode(vector.Mode))
			if got != want || ok != vector.OK {
				t.Fatalf("got %d,%v; want %d,%v", got, ok, want, vector.OK)
			}
		})
	}
}

func executeGuavaMath(operation string, a, b int64, mode math_util.RoundingMode) (int64, bool) {
	var result int64
	var ok bool
	var err error
	switch operation {
	case "checkedAdd":
		result, ok = math_util.CheckedAdd(a, b)
	case "checkedSubtract":
		result, ok = math_util.CheckedSubtract(a, b)
	case "checkedMultiply":
		result, ok = math_util.CheckedMultiply(a, b)
	case "saturatingAdd":
		result, ok = math_util.SaturatingAdd(a, b), true
	case "saturatingSubtract":
		result, ok = math_util.SaturatingSubtract(a, b), true
	case "saturatingMultiply":
		result, ok = math_util.SaturatingMultiply(a, b), true
	case "checkedPow":
		result, ok = math_util.CheckedPow(a, int(b))
	case "saturatingPow":
		result, ok = math_util.SaturatingPow(a, int(b)), true
	case "divide":
		result, err = math_util.Divide(a, b, mode)
	case "log2":
		var rounded int
		rounded, err = math_util.Log2(a, mode)
		result = int64(rounded)
	case "log10":
		var rounded int
		rounded, err = math_util.Log10(a, mode)
		result = int64(rounded)
	case "sqrt":
		result, err = math_util.Sqrt(a, mode)
	case "isPrime":
		if math_util.IsPrime(uint64(a)) {
			result = 1
		}
		ok = true
	case "mod":
		result, err = math_util.Modulo(a, b)
	case "gcd":
		result, err = math_util.GCD(a, b)
	case "factorial":
		var factorial uint64
		factorial, ok = math_util.Factorial(int(a))
		result = int64(factorial)
	case "binomial":
		var binomial uint64
		binomial, ok = math_util.Binomial(int(a), int(b))
		result = int64(binomial)
	case "mean":
		result, ok = math_util.Mean(a, b), true
	case "checkedConvert":
		var converted int32
		converted, ok = math_util.CheckedConvert[int32](a)
		result = int64(converted)
	case "saturatingConvert":
		result, ok = int64(math_util.SaturatingConvert[int32](a)), true
	default:
		panic("unknown operation " + operation)
	}
	if err != nil {
		return 0, false
	}
	switch operation {
	case "divide", "log2", "log10", "sqrt", "mod", "gcd":
		ok = true
	}
	return result, ok
}
