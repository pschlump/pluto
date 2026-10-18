/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Example tests for the set-convenience capabilities (capabilities.go).
// The examples are single-goroutine; run the tests with -race for the
// concurrency guarantees.  Slice and String use the internal
// breadth-first backing order — not sorted order; Pop is the sorted
// drain.

package heap_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/heap_ts"
)

func ExampleHeap_Slice() {
	hp := heap_ts.NewHeap[int]()
	hp.Push(3)
	hp.Push(1)
	hp.Push(2)
	// Backing (breadth-first) order, not sorted: after the sifts the
	// array holds [1 3 2].
	fmt.Println(hp.Slice())
	// Output: [1 3 2]
}

func ExampleHeap_String() {
	hp := heap_ts.NewHeap[string]()
	hp.Push("delta")
	hp.Push("bravo")
	hp.Push("charlie")
	fmt.Println(hp.String())
	// Output: [bravo delta charlie]
}

func ExampleHeap_StringFunc() {
	hp := heap_ts.NewHeap[int]()
	hp.Push(5)
	hp.Push(8)
	fmt.Println(hp.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleHeap_Equal() {
	a := heap_ts.NewHeap[int]()
	for _, v := range []int{1, 2, 3, 4, 5} {
		a.Push(v)
	}
	b := heap_ts.NewHeap[int]()
	for _, v := range []int{5, 4, 3, 2, 1} {
		b.Push(v)
	}
	c := heap_ts.NewHeap[int]()
	for _, v := range []int{1, 2, 3, 4} {
		c.Push(v)
	}
	// Equal is multiset equality: same elements in any arrangement.
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleHeap_Clone() {
	hp := heap_ts.NewHeap[string]()
	hp.Push("apple")
	dup := hp.Clone()
	dup.Push("pear")
	fmt.Println(hp.Len(), dup.Len())
	// Output: 1 2
}
