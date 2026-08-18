package g_lib

// Number is a constraint for the numeric types that arithmetic can be
// performed on.
type Number interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | float32 | float64
}

// Pow returns base raised to the power exponent, computed by repeated
// multiplication.  The possibility of numeric overflow is ignored.  A
// negative exponent returns T(1) for integer types.
func Pow[T Number](base T, exponent int) T {
	result := T(1)
	for i := 0; i < exponent; i++ {
		result *= base
	}
	return result
}

/* vim: set noai ts=4 sw=4: */
