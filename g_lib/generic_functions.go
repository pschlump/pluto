/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package g_lib provides small generic utility functions: min/max,
// absolute value and exponentiation, slice searching and manipulation,
// set conversion, map key extraction, and a pointer helper.  There are
// no data structures here — just pure functions on slices and maps,
// plus the numeric constraint interfaces (Numeric, SignedInteger,
// SignedNumeric, Unsigned, Number) they are parameterized with.
//
// All functions are safe for concurrent use; they operate only on their
// arguments and keep no shared state.  The functions that return a
// slice never modify their input; RemoveAt returns a fresh copy even on
// a hit, and the others (Remove, RemoveComparable, Unique) build a new
// slice from the kept elements only.
package g_lib

import (
	"cmp"
	"reflect"
	"slices"
	"sort"
)

// Min returns the smaller of a and b.
func Min[T cmp.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

// Max returns the larger of a and b.
func Max[T cmp.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

// MinArray returns the smallest element of a.  If a is empty it returns the
// zero value of T.
func MinArray[T cmp.Ordered](a []T) (rv T) {
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

// MaxArray returns the largest element of a.  If a is empty it returns the
// zero value of T.
func MaxArray[T cmp.Ordered](a []T) (rv T) {
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

// IfTrue returns a when on is true, otherwise b.
func IfTrue[T any](on bool, a, b T) T {
	if on {
		return a
	}
	return b
}

// InArray reports whether needle occurs in haystack, using a linear search.
// The argument order follows the standard-library convention of
// strings.Contains and slices.Contains: haystack first, needle second.
func InArray[T comparable](haystack []T, needle T) bool {
	return slices.Contains(haystack, needle)
}

// LocationInArray returns the index of the first occurrence of needle in
// haystack, or -1 if needle is not present.  The argument order matches
// InArray: haystack first, needle second.
func LocationInArray[T comparable](haystack []T, needle T) int {
	for ii, val := range haystack {
		if val == needle {
			return ii
		}
	}
	return -1
}

// KeysForStringMap returns the keys of aMap in unspecified order.
func KeysForStringMap[T any](aMap map[string]T) (rv []string) {
	for key := range aMap {
		rv = append(rv, key)
	}
	return
}

// Numeric is a constraint with a type set of all integer and
// floating-point types, including types defined from them (~T).
// uintptr is not in the set; see Unsigned.
type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// SignedInteger is a constraint with a type set of all signed integer
// types, including types defined from them (~T).
type SignedInteger interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

// SignedNumeric is a constraint with a type set of all signed integer
// and floating-point types, including types defined from them (~T).
type SignedNumeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~float32 | ~float64
}

// Unsigned is a constraint with a type set of all unsigned integer
// types plus uintptr, including types defined from them (~T).
type Unsigned interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Abs returns the absolute value of a.  Note that for the most negative
// value of a signed integer type the result overflows and stays negative.
func Abs[T SignedNumeric](a T) T {
	if a < 0 {
		return -a
	}
	return a
}

// GetMapKeys returns the keys of m in unspecified order.
func GetMapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// SortSlice sorts s in place in ascending order.
func SortSlice[T cmp.Ordered](s []T) {
	slices.Sort(s)
}

// EqualSlice reports whether s and t have the same length and equal
// elements in the same order.
func EqualSlice[T comparable](s, t []T) bool {
	if len(s) != len(t) {
		return false
	}
	for ii := range s {
		if s[ii] != t[ii] {
			return false
		}
	}
	return true
}

// SortedKeysForStringMap extracts the keys from aMap, sorts them, and
// returns the sorted slice.
func SortedKeysForStringMap[T any](aMap map[string]T) (rv []string) {
	for key := range aMap {
		rv = append(rv, key)
	}
	sort.Strings(rv)
	return
}

// RemoveAt returns a copy of slice with the item at position pos removed.
// If pos is out of range it returns slice unchanged.  The input slice is
// never modified.
func RemoveAt[T any](slice []T, pos int) []T {
	if pos < 0 || pos >= len(slice) {
		return slice
	}
	result := make([]T, 0, len(slice)-1)
	result = append(result, slice[:pos]...)
	return append(result, slice[pos+1:]...)
}

// Remove returns a new slice containing the items of haystack that are not
// equal to needle, using reflect.DeepEqual for the comparison.
func Remove[T any](haystack []T, needle T) (result []T) {
	for _, item := range haystack {
		if !reflect.DeepEqual(item, needle) {
			result = append(result, item)
		}
	}
	return
}

// RemoveComparable returns a new slice containing the items of slice that
// are not equal to element.
func RemoveComparable[T comparable](slice []T, element T) (result []T) {
	for _, item := range slice {
		if item != element {
			result = append(result, item)
		}
	}
	return
}

// Unique returns the elements of s with duplicates removed, preserving the
// order of first occurrence.
func Unique[T comparable](s []T) []T {
	inResult := make(map[T]bool)
	var result []T
	for _, str := range s {
		if _, ok := inResult[str]; !ok {
			inResult[str] = true
			result = append(result, str)
		}
	}
	return result
}

// ToBoolMap converts a slice to a set, represented as a map with true
// values, suitable for membership tests.
//
// For example,
//
//	x := []string{"a", "b"}
//
// produces
//
//	map[string]bool{"a": true, "b": true}
func ToBoolMap[T comparable](src []T) map[T]bool {
	result := make(map[T]bool, len(src))
	for _, v := range src {
		result[v] = true
	}
	return result
}

// Ptr returns a pointer to a copy of the given value — the Go 1.26
// new(expr) form.  v is evaluated at call time, so mutating the
// original variable afterwards does not change *Ptr(v).
func Ptr[T any](v T) *T {
	return new(v)
}

// Return true if the array, haystack, has the value, needle in it.
// This uses the Dijkstra L algorithm for the search.
func ArrayHas[T comparable](haystack []T, needle T) bool {
	for _, val := range haystack {
		if val == needle {
			return true
		}
	}
	return false
}
