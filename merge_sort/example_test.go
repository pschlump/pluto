/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package merge_sort_test

import (
	"fmt"

	"github.com/pschlump/pluto/merge_sort"
)

// Stability: sorting records by one field keeps equal-keyed records
// in their original order — the finish times with no distance given
// keep their lane order.
func Example() {
	type Finish struct {
		Time int
		Lane int
	}

	finishes := []Finish{
		{Time: 41, Lane: 1}, {Time: 39, Lane: 2}, {Time: 41, Lane: 3}, {Time: 39, Lane: 4},
	}
	merge_sort.SortFunc(finishes, func(a, b Finish) int {
		return a.Time - b.Time
	})

	for _, f := range finishes {
		fmt.Printf("%d:%d ", f.Time, f.Lane)
	}
	fmt.Println()
	// Output:
	// 39:2 39:4 41:1 41:3
}

// Naturally ordered types need no comparison function at all.
func ExampleSort() {
	data := []int{42, 7, 13, 7, 1}
	merge_sort.Sort(data)
	fmt.Println(data)
	// Output:
	// [1 7 7 13 42]
}

// SortFunc orders any type by a caller supplied comparison function;
// a reversed comparison yields descending order.
func ExampleSortFunc() {
	data := []string{"pear", "apple", "fig", "date"}
	merge_sort.SortFunc(data, func(a, b string) int {
		return len(a) - len(b) // ascending by length; ties keep order
	})
	fmt.Println(data)
	// Output:
	// [fig pear date apple]
}

// The bottom-up variant merges runs of width 1, 2, 4, ... without
// any recursion.
func ExampleSortBottomUp() {
	data := []int{9, 5, 7, 1, 8, 2}
	merge_sort.SortBottomUp(data)
	fmt.Println(data)
	// Output:
	// [1 2 5 7 8 9]
}

// Inversions counts the out-of-order pairs — the minimum number of
// adjacent swaps that would sort the slice — without modifying it.
func ExampleInversions() {
	data := []int{4, 1, 3, 2}
	fmt.Println(merge_sort.Inversions(data), data)
	// Output:
	// 4 [4 1 3 2]
}
