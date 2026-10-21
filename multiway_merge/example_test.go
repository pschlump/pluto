/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package multiway_merge_test

import (
	"fmt"

	"github.com/pschlump/pluto/multiway_merge"
)

// The Multiway.java shape: three sorted runs merged into one for the
// price of O(n log k), not a full O(n log n) sort.
func Example() {
	runs := [][]int{
		{1, 4, 7, 10},
		{2, 5, 8},
		{3, 6, 9, 12, 15},
	}
	merged := multiway_merge.MergeK(runs...)
	fmt.Println(merged)
	// Output:
	// [1 2 3 4 5 6 7 8 9 10 12 15]
}

// Stability: records that compare equal keep stream order — all of
// run 0's "b" finishers (and within a run, input order) come before
// run 1's, like merge_sort's take-left-on-tie merge.
func ExampleMergeKFunc() {
	type Finish struct {
		Lane int
		Time int
	}
	runs := [][]Finish{
		{{Lane: 1, Time: 41}, {Lane: 2, Time: 41}, {Lane: 3, Time: 55}},
		{{Lane: 4, Time: 39}, {Lane: 5, Time: 41}},
	}

	merged := multiway_merge.MergeKFunc(func(a, b Finish) int {
		return a.Time - b.Time
	}, runs...)
	for _, f := range merged {
		fmt.Printf("%d:%d ", f.Time, f.Lane)
	}
	fmt.Println()
	// Output:
	// 39:4 41:1 41:2 41:5 55:3
}
