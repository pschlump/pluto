/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package fenwick_tree_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/fenwick_tree_ts"
)

// A thread-safe Fenwick tree over a small int slice: the same API as
// fenwick_tree, safe to share between goroutines.
func Example() {
	ft := fenwick_tree_ts.NewFenwickTreeFrom([]int{3, 1, 4, 1, 5, 9, 2, 6})

	fmt.Println("Sum(0..3):", ft.Sum(3)) // 3+1+4+1
	if s, ok := ft.RangeSum(2, 5); ok {
		fmt.Println("RangeSum(2..5):", s) // 4+1+5+9
	}

	ft.Add(0, 10) // 3 -> 13
	ft.Set(1, 20) // 1 -> 20
	fmt.Println("Sum(0..3):", ft.Sum(3))
	v, _ := ft.Value(7)
	fmt.Println("Value(7):", v)

	// Output:
	// Sum(0..3): 9
	// RangeSum(2..5): 19
	// Sum(0..3): 38
	// Value(7): 6
}

// Lock + the Nl-prefixed methods make a read-modify-write sequence
// atomic: no other goroutine can observe or change the tree between
// the read and the update.
func ExampleFenwickTree_Lock() {
	ft := fenwick_tree_ts.NewFenwickTreeFrom([]int{1, 2, 3, 4, 5})

	ft.Lock()
	v, _ := ft.NlValue(2)
	ft.NlSet(2, 10*v) // slot 2: 3 -> 30, atomically
	total, _ := ft.NlRangeSum(0, ft.NlLen()-1)
	ft.Unlock()

	fmt.Println("doubled-and-then-some slot 2:", v)
	fmt.Println("total:", total)

	// Output:
	// doubled-and-then-some slot 2: 3
	// total: 42
}
