/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package segment_tree_test

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/pschlump/pluto/segment_tree"
)

// A range-SUM segment tree over a small int slice: inclusive range
// queries with Query and point assignment with Update.
func Example() {
	st := segment_tree.NewSegmentTree([]int{3, 1, 4, 1, 5, 9, 2, 6})

	if s, ok := st.Query(2, 5); ok {
		fmt.Println("Query(2..5):", s) // 4+1+5+9
	}
	s, _ := st.Query(0, 7)
	fmt.Println("Query(0..7):", s)

	st.Update(0, 10) // 3 -> 10
	s, _ = st.Query(0, 7)
	fmt.Println("Query(0..7):", s)

	// Output:
	// Query(2..5): 19
	// Query(0..7): 31
	// Query(0..7): 38
}

// NewSegmentTreeFunc carries any associative combine with an identity —
// here a range-MIN tree (identity +Inf).
func ExampleNewSegmentTreeFunc() {
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	st := segment_tree.NewSegmentTreeFunc([]int{3, 1, 4, 1, 5, 9, 2, 6}, min, math.MaxInt)

	if v, ok := st.Query(0, 7); ok {
		fmt.Println("min(0..7):", v)
	}
	if v, ok := st.Query(4, 7); ok {
		fmt.Println("min(4..7):", v)
	}

	st.Update(3, -2) // the second 1 -> -2
	if v, ok := st.Query(0, 7); ok {
		fmt.Println("min(0..7):", v)
	}

	// Output:
	// min(0..7): 1
	// min(4..7): 2
	// min(0..7): -2
}

// MarshalJSON encodes the tree as a JSON array of the per-slot values:
// element i is Value(i).
func ExampleSegmentTree_MarshalJSON() {
	st := segment_tree.NewSegmentTree([]int{3, 1, 2})

	b, err := json.Marshal(st)
	fmt.Println(string(b), err)
	// Output:
	// [3,1,2] <nil>
}

// UnmarshalJSON replaces the contents of the tree from a JSON array;
// element i becomes the new Value(i) and the slot count follows the
// array length.  The combine function is kept, so Query still works.
func ExampleSegmentTree_UnmarshalJSON() {
	st := segment_tree.NewSegmentTree([]int{0})
	if err := json.Unmarshal([]byte(`[4,1,5]`), st); err != nil {
		fmt.Println("error:", err)
		return
	}
	for i := 0; i < st.Len(); i++ {
		v, _ := st.Value(i)
		fmt.Println(i, v)
	}
	s, _ := st.Query(0, 2)
	fmt.Println("Query(0..2):", s)
	// Output:
	// 0 4
	// 1 1
	// 2 5
	// Query(0..2): 10
}
