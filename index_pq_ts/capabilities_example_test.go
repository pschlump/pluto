/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Example tests for the set-convenience capabilities (capabilities.go).
// The examples are single-goroutine; run the tests with -race for the
// concurrency guarantees.  Slice and String use the BACKING order — the
// breadth-first order of the index heap, not priority order; the
// (index, value) pairs in priority order are All's.

package index_pq_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/index_pq_ts"
)

func ExampleIndexPQ_Slice() {
	q := index_pq_ts.NewIndexPQ[int](6)
	q.Insert(0, 5)
	q.Insert(1, 3)
	q.Insert(2, 4)
	// Backing order: the index heap is [1 0 2], so the values render
	// [3 5 4] — not priority order (use All for that).
	fmt.Println(q.Slice())
	// Output: [3 5 4]
}

func ExampleIndexPQ_String() {
	q := index_pq_ts.NewIndexPQ[string](3)
	q.Insert(0, "delta")
	q.Insert(1, "bravo")
	q.Insert(2, "charlie")
	fmt.Println(q.String())
	// Output: [bravo delta charlie]
}

func ExampleIndexPQ_StringFunc() {
	q := index_pq_ts.NewIndexPQ[int](2)
	q.Insert(0, 5)
	q.Insert(1, 8)
	fmt.Println(q.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleIndexPQ_Equal() {
	a := index_pq_ts.NewIndexPQ[string](2)
	a.Insert(0, "apple")
	a.Insert(1, "pear")
	b := index_pq_ts.NewIndexPQ[string](2)
	b.Insert(1, "pear") // the same bindings, built in the other order
	b.Insert(0, "apple")
	c := index_pq_ts.NewIndexPQ[string](2)
	c.Insert(0, "pear") // the values swapped between the indices
	c.Insert(1, "apple")
	// Equal compares the index→value bindings: same multiset, different
	// bindings — c is NOT equal.
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleIndexPQ_Clone() {
	q := index_pq_ts.NewIndexPQ[string](4)
	q.Insert(0, "apple")
	dup := q.Clone()
	dup.Insert(1, "pear")
	fmt.Println(q.Len(), dup.Len())
	// Output: 1 2
}
