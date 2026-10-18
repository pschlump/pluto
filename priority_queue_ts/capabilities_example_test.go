/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Example tests for the set-convenience capabilities (capabilities.go).
// The examples are single-goroutine; run the tests with -race for the
// concurrency guarantees.  This package's Slice and String order is
// PRIORITY order — sorted, minimum first (its All is sorted; the one
// §4.5 exception).

package priority_queue_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/priority_queue_ts"
)

func ExamplePriorityQueue_Slice() {
	pq := priority_queue_ts.NewPriorityQueue[int]()
	pq.Insert(3)
	pq.Insert(1)
	pq.Insert(2)
	// Priority order, minimum first — All's order (sorted).
	fmt.Println(pq.Slice())
	// Output: [1 2 3]
}

func ExamplePriorityQueue_String() {
	pq := priority_queue_ts.NewPriorityQueue[string]()
	pq.Insert("delta")
	pq.Insert("bravo")
	pq.Insert("charlie")
	fmt.Println(pq.String())
	// Output: [bravo charlie delta]
}

func ExamplePriorityQueue_StringFunc() {
	pq := priority_queue_ts.NewPriorityQueue[int]()
	pq.Insert(5)
	pq.Insert(8)
	fmt.Println(pq.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExamplePriorityQueue_Equal() {
	a := priority_queue_ts.NewPriorityQueue[int]()
	for _, v := range []int{1, 2, 3, 4, 5} {
		a.Insert(v)
	}
	b := priority_queue_ts.NewPriorityQueue[int]()
	for _, v := range []int{5, 4, 3, 2, 1} {
		b.Insert(v)
	}
	c := priority_queue_ts.NewPriorityQueue[int]()
	for _, v := range []int{1, 2, 3, 4} {
		c.Insert(v)
	}
	// Equal is multiset equality: same elements from any insertion order.
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExamplePriorityQueue_Clone() {
	pq := priority_queue_ts.NewPriorityQueue[string]()
	pq.Insert("apple")
	dup := pq.Clone()
	dup.Insert("pear")
	fmt.Println(pq.Len(), dup.Len())
	// Output: 1 2
}
