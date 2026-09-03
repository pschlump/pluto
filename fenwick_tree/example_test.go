/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package fenwick_tree_test

import (
	"encoding/json"
	"fmt"

	"github.com/pschlump/pluto/fenwick_tree"
)

// A Fenwick tree over a small int slice: point updates with Add and
// Set, prefix sums with Sum, and inclusive range sums with RangeSum.
func Example() {
	ft := fenwick_tree.NewFenwickTreeFrom([]int{3, 1, 4, 1, 5, 9, 2, 6})

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

// A fresh Fenwick tree starts with every slot at zero; out-of-range
// indices report instead of panicking.
func ExampleNewFenwickTree() {
	ft := fenwick_tree.NewFenwickTree[int](5)
	fmt.Println(ft.Len(), ft.IsEmpty())

	ft.Add(2, 7)
	fmt.Println(ft.Sum(1), ft.Sum(2)) // the update is visible from slot 2 on

	fmt.Println("Sum(-1):", ft.Sum(-1))     // the empty prefix
	fmt.Println("Sum(5):", ft.Sum(5))       // out of range: zero
	fmt.Println("Add(5, 1):", ft.Add(5, 1)) // out of range: false

	// Output:
	// 5 false
	// 0 7
	// Sum(-1): 0
	// Sum(5): 0
	// Add(5, 1): false
}

// MarshalJSON encodes the tree as a JSON array of the per-index values:
// element i is Value(i).
func ExampleFenwickTree_MarshalJSON() {
	ft := fenwick_tree.NewFenwickTreeFrom([]int{3, 1, 2})

	b, err := json.Marshal(ft)
	fmt.Println(string(b), err)
	// Output:
	// [3,1,2] <nil>
}

// UnmarshalJSON replaces the contents of the tree from a JSON array;
// element i becomes the new Value(i) and the slot count follows the
// array length.
func ExampleFenwickTree_UnmarshalJSON() {
	ft := fenwick_tree.NewFenwickTree[int](1)
	if err := json.Unmarshal([]byte(`[4,1,5]`), ft); err != nil {
		fmt.Println("error:", err)
		return
	}
	for i := 0; i < ft.Len(); i++ {
		v, _ := ft.Value(i)
		fmt.Println(i, v)
	}
	fmt.Println("Sum(0..2):", ft.Sum(2))
	// Output:
	// 0 4
	// 1 1
	// 2 5
	// Sum(0..2): 10
}
