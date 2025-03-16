package g_lib

// Number defines a constraint for numeric types that can have calcuations performed on them.
type Number interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | float32 | float64
}

// Pow calculates the power of a number ignoring the possibility of numeric oveflow
func Pow[T Number](base T, exponent int) T {
	result := T(1)
	for i := 0; i < exponent; i++ {
		result *= base
	}
	return result
}

// See: https://www.codecademy.com/resources/docs/go/math-functions/ceil
