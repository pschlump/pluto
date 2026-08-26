/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package g_lib

// Number is a constraint for the numeric types that arithmetic can be
// performed on.  Unlike Numeric it uses exact type sets (no ~T), so it
// accepts only the predeclared types — use Numeric when defined types
// should be allowed too.  uintptr is not in the set; see Unsigned.
type Number interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | float32 | float64
}

// Pow returns base raised to the power exponent, computed by repeated
// multiplication.  The possibility of numeric overflow is ignored.  A
// negative exponent returns T(1) — the loop runs zero times — for
// integer and floating-point types alike.
func Pow[T Number](base T, exponent int) T {
	result := T(1)
	for range exponent {
		result *= base
	}
	return result
}

/* vim: set noai ts=4 sw=4: */
