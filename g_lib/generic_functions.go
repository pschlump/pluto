package g_lib

import (
	"constraints"
)

func Min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func Max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func MinArray[T constraints.Ordered](a []T) (rv T) {
	if len(a) > 0 {
		rv = a[0]
	}
	for _, v := range a {
		if v < rv {
			rv = v
		}
	}
	return
}

func MaxArray[T constraints.Ordered](a []T) (rv T) {
	if len(a) > 0 {
		rv = a[0]
	}
	for _, v := range a {
		if v > rv {
			rv = v
		}
	}
	return
}

func InArray[T comparable](needle T, haystack []T) bool {
	for _, val := range haystack {
		if val == needle {
			return true
		}
	}
	return false
}

func LocationInArray[T comparable](needle T, haystack []T) int {
	for ii, val := range haystack {
		if val == needle {
			return ii
		}
	}
	return -1
}

func KeysForStringMap[T any](aMap map[string]T) (rv []string) {
	for key := range aMap {
		rv = append(rv, key)
	}
	return
}

type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// Signed is a constraint with a type set of all signed integer types.
type SignedInteger interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

// SignedNumeric is a constraint with a type set of all signed types.
type SignedNumeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~float32 | ~float64
}

// Unsigned is a constraint with a type set of all unsigned integer types.
type Unsigned interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

func Abs[T SignedNumeric](a T) T {
	if a < 0 {
		return -a
	}
	return a
}
