package g_lib

import (
	"reflect"
	"sort"

	"golang.org/x/exp/constraints"
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

func IfTrue[T any](on bool, a T, b T) (rv T) {
	if on {
		return a
	}
	return b
}

// InArray uses Dijkstra's "L" algorythm to search 'haystack' for 'needle'.  "L" is the linear search algorythm.
// Exampel of Use:
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

func GetMapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func SortSlice[T constraints.Ordered](s []T) {
	sort.Slice(s, func(i, j int) bool {
		return s[i] < s[j]
	})
}

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

// SortedKeysForStringMap will Extract the keys from a map[string]interface{}, sort them and return the sorted slice.
func SortedKeysForStringMap[T any](aMap map[string]T) (rv []string) {
	for key := range aMap {
		rv = append(rv, key)
	}
	sort.Strings(rv)
	return
}

// RemoveAt removes from `slice` the item at postion `pos`.  If pos is out of range it returns the original `slice`.
func RemoveAt[T any](slice []T, pos int) []T {
	if pos < 0 {
		return slice
	} else if pos >= len(slice) {
		return slice
	} else if pos == 0 {
		return slice[1:]
	} else if pos == len(slice)-1 {
		return slice[0:pos]
	}
	return append(slice[:pos], slice[pos+1:]...)
}

// Remove will take 'needle' from 'haystack' if it matches based on reflect.DeepEqual
func Remove[T any](haystack []T, needle T) (result []T) {
	for _, item := range haystack {
		if !reflect.DeepEqual(item, needle) {
			result = append(result, item)
		}
	}
	return
}

func RemoveComparable[T comparable](slice []T, element T) (result []T) {
	for _, item := range slice {
		if item != element {
			result = append(result, item)
		}
	}
	return
}

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

// ToBoolMap will convert a slice to a set of true flags in a map that can then be looked up in (or set to false to get rid of items)
//
// For eample,
//
//	x := []string{ "a", "b" }
//
// Will produce `x`, a map[string]bool with values of:
//
//	x == map[string]bool{ "a": true, "b": true }
func ToBoolMap[T comparable](src []T) map[T]bool {
	var result = make(map[T]bool)
	for _, v := range src {
		result[v] = true
	}
	return result
}

/* vim: set noai ts=4 sw=4: */
