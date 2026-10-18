/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Example tests for the set-convenience capabilities (capabilities.go).
// Slice and String use (key, insertion) order — minimum key first, ties
// oldest first — the exact order Pop produces.  Equal is key-multiset
// equality (insertion stamps ignored); Clone preserves the FIFO tie
// order.

package stable_pq_test

import (
	"fmt"

	"github.com/pschlump/pluto/stable_pq"
)

func ExampleStablePQ_Slice() {
	pq := stable_pq.NewStablePQ[int]()
	pq.Insert(2)
	pq.Insert(1)
	pq.Insert(2)
	pq.Insert(1)
	// (key, insertion) order — for keys this is sorted order, with ties
	// oldest first, so it is deterministic even under duplicates.
	fmt.Println(pq.Slice())
	// Output: [1 1 2 2]
}

func ExampleStablePQ_String() {
	pq := stable_pq.NewStablePQ[string]()
	pq.Insert("delta")
	pq.Insert("bravo")
	pq.Insert("charlie")
	fmt.Println(pq.String())
	// Output: [bravo charlie delta]
}

func ExampleStablePQ_StringFunc() {
	pq := stable_pq.NewStablePQ[int]()
	pq.Insert(5)
	pq.Insert(8)
	fmt.Println(pq.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleStablePQ_Equal() {
	a := stable_pq.NewStablePQ[int]()
	for _, v := range []int{1, 2, 3, 4, 5} {
		a.Insert(v)
	}
	b := stable_pq.NewStablePQ[int]()
	for _, v := range []int{5, 4, 3, 2, 1} {
		b.Insert(v)
	}
	c := stable_pq.NewStablePQ[int]()
	for _, v := range []int{1, 2, 3, 4} {
		c.Insert(v)
	}
	// Equal is key-multiset equality: the same keys in any insertion
	// order — hence with different internal stamps — are equal.
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleStablePQ_Clone() {
	pq := stable_pq.NewStablePQ[string]()
	pq.Insert("apple")
	dup := pq.Clone()
	dup.Insert("pear")
	fmt.Println(pq.Len(), dup.Len())
	// Output: 1 2
}
