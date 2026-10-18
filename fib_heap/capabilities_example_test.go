/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Example tests for the set-convenience capabilities (capabilities.go).
// Slice and String use priority order (minimum first — All's order).
// Equal compares index→value BINDINGS, the map-shaped exception within
// the heaps menu.

package fib_heap_test

import (
	"fmt"

	"github.com/pschlump/pluto/fib_heap"
)

func ExampleIndexFibonacciPQ_Slice() {
	q := fib_heap.NewIndexFibonacciPQ[int](8)
	q.Insert(0, 3)
	q.Insert(1, 1)
	q.Insert(2, 2)
	// Priority order (minimum first — All's order), not index order.
	fmt.Println(q.Slice())
	// Output: [1 2 3]
}

func ExampleIndexFibonacciPQ_String() {
	q := fib_heap.NewIndexFibonacciPQ[string](4)
	q.Insert(0, "delta")
	q.Insert(1, "bravo")
	q.Insert(2, "charlie")
	fmt.Println(q.String())
	// Output: [bravo charlie delta]
}

func ExampleIndexFibonacciPQ_StringFunc() {
	q := fib_heap.NewIndexFibonacciPQ[int](4)
	q.Insert(0, 5)
	q.Insert(1, 8)
	fmt.Println(q.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleIndexFibonacciPQ_Equal() {
	a := fib_heap.NewIndexFibonacciPQ[int](8)
	a.Insert(0, 10)
	a.Insert(1, 20)
	b := fib_heap.NewIndexFibonacciPQ[int](8)
	b.Insert(0, 20) // same value multiset as a, but the bindings are
	b.Insert(1, 10) // swapped: NOT equal.
	c := fib_heap.NewIndexFibonacciPQ[int](8)
	c.Insert(1, 20) // the same bindings as a, inserted in a different
	c.Insert(0, 10) // order: equal.
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: false true
}

func ExampleIndexFibonacciPQ_Clone() {
	q := fib_heap.NewIndexFibonacciPQ[string](8)
	q.Insert(0, "apple")
	dup := q.Clone()
	dup.Insert(1, "pear")
	fmt.Println(q.Len(), dup.Len())
	// Output: 1 2
}
