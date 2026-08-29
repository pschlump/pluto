/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package segment_tree_ts_test

import (
	"fmt"
	"math"

	"github.com/pschlump/pluto/segment_tree_ts"
)

// A thread-safe range-SUM segment tree over a small int slice: the
// same API as segment_tree, safe to share between goroutines.
func Example() {
	st := segment_tree_ts.NewSegmentTree([]int{3, 1, 4, 1, 5, 9, 2, 6})

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
	st := segment_tree_ts.NewSegmentTreeFunc([]int{3, 1, 4, 1, 5, 9, 2, 6}, min, math.MaxInt)

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

// Lock + the Nl-prefixed methods make a read-modify-write sequence
// atomic: no other goroutine can observe or change the tree between
// the read and the update.
func ExampleSegmentTree_Lock() {
	st := segment_tree_ts.NewSegmentTree([]int{1, 2, 3, 4, 5})

	st.Lock()
	v, _ := st.NlValue(2)
	st.NlUpdate(2, 10*v) // slot 2: 3 -> 30, atomically
	total, _ := st.NlQuery(0, st.NlLen()-1)
	st.Unlock()

	fmt.Println("read slot 2:", v)
	fmt.Println("total:", total)

	// Output:
	// read slot 2: 3
	// total: 42
}
