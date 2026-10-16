/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package string_sort_test

import (
	"fmt"

	"github.com/pschlump/pluto/string_sort"
)

// LSDSort — fixed-width keys, one counting pass per byte.  The classic
// application: keys that are all the same length (license plates, IP
// addresses, dates) sort in w linear passes with no comparisons.
func ExampleLSDSort() {
	plates := []string{"4PGC938", "2IYE230", "3ATR721", "1ICK750"}
	string_sort.LSDSort(plates, 7)
	fmt.Println(plates)
	// Output:
	// [1ICK750 2IYE230 3ATR721 4PGC938]
}

// MSDSort — variable-width keys, most-significant byte first.  A key
// that is a prefix of another sorts first, exactly like Go's native
// string ordering.
func ExampleMSDSort() {
	words := []string{"she", "sells", "sea", "shells", "by", "the", "sea", "shore"}
	string_sort.MSDSort(words)
	fmt.Println(words)
	// Output:
	// [by sea sea sells she shells shore the]
}

// Quick3String — 3-way radix quicksort, in place with no auxiliary
// buffer; the usual choice for general-purpose variable-width keys.
func ExampleQuick3String() {
	words := []string{"she", "sells", "sea", "shells", "by", "the", "sea", "shore"}
	string_sort.Quick3String(words)
	fmt.Println(words)
	// Output:
	// [by sea sea sells she shells shore the]
}
