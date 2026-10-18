/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Example tests for the set-convenience capabilities (capabilities.go).
// Slice and String use the internal breadth-first backing order — not
// sorted order; Pop is the sorted drain.  Equal is multiset equality;
// Clone preserves the exact backing layout.

package multiway_pq_test

import (
	"fmt"

	"github.com/pschlump/pluto/multiway_pq"
)

func ExampleDHeap_Slice() {
	h := multiway_pq.NewDHeap[int](4)
	h.Push(3)
	h.Push(1)
	h.Push(2)
	// Backing (breadth-first) order, not sorted: after the sifts the
	// array holds [1 3 2].
	fmt.Println(h.Slice())
	// Output: [1 3 2]
}

func ExampleDHeap_String() {
	h := multiway_pq.NewDHeap[string](4)
	h.Push("delta")
	h.Push("bravo")
	h.Push("charlie")
	fmt.Println(h.String())
	// Output: [bravo delta charlie]
}

func ExampleDHeap_StringFunc() {
	h := multiway_pq.NewDHeap[int](4)
	h.Push(5)
	h.Push(8)
	fmt.Println(h.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleDHeap_Equal() {
	a := multiway_pq.NewDHeap[int](4)
	for _, v := range []int{1, 2, 3, 4, 5} {
		a.Push(v)
	}
	b := multiway_pq.NewDHeap[int](4)
	for _, v := range []int{5, 4, 3, 2, 1} {
		b.Push(v)
	}
	c := multiway_pq.NewDHeap[int](8) // a different d, same content
	for _, v := range []int{3, 1, 4, 5, 2} {
		c.Push(v)
	}
	// Equal is multiset equality: same elements in any arrangement, at
	// any branching factor.
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true true
}

func ExampleDHeap_Clone() {
	h := multiway_pq.NewDHeap[string](4)
	h.Push("apple")
	dup := h.Clone()
	dup.Push("pear")
	fmt.Println(h.Len(), dup.Len())
	// Output: 1 2
}
