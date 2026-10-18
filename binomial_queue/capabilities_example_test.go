/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Example tests for the set-convenience capabilities (capabilities.go).
// Slice and String use the internal forest order — not sorted order;
// DeleteMin is the sorted drain.  Equal is multiset equality; Clone
// rebuilds by re-insertion (shape may differ, drain order survives).

package binomial_queue_test

import (
	"fmt"

	"github.com/pschlump/pluto/binomial_queue"
)

func ExampleBinomialQueue_Slice() {
	q := binomial_queue.NewBinomialQueue[int]()
	q.Insert(3)
	q.Insert(1)
	q.Insert(2)
	// Forest order, not sorted: after the merges the forest is
	// [B0:2, B1:(1 with child 3)], walked pre-order -> [2 1 3].
	fmt.Println(q.Slice())
	// Output: [2 1 3]
}

func ExampleBinomialQueue_String() {
	q := binomial_queue.NewBinomialQueue[string]()
	q.Insert("delta")
	q.Insert("bravo")
	q.Insert("charlie")
	fmt.Println(q.String())
	// Output: [charlie bravo delta]
}

func ExampleBinomialQueue_StringFunc() {
	q := binomial_queue.NewBinomialQueue[int]()
	q.Insert(5)
	q.Insert(8)
	fmt.Println(q.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleBinomialQueue_Equal() {
	a := binomial_queue.NewBinomialQueue[int]()
	for _, v := range []int{1, 2, 3, 4, 5} {
		a.Insert(v)
	}
	b := binomial_queue.NewBinomialQueue[int]() // the same multiset,
	for _, v := range []int{5, 4, 3, 2, 1} {    //  a different forest
		b.Insert(v)
	}
	c := binomial_queue.NewBinomialQueue[int]()
	for _, v := range []int{1, 2, 3, 4} {
		c.Insert(v)
	}
	// Equal is multiset equality: same elements in any insertion or
	// merge history.
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleBinomialQueue_Clone() {
	q := binomial_queue.NewBinomialQueue[string]()
	q.Insert("apple")
	dup := q.Clone()
	dup.Insert("pear")
	fmt.Println(q.Len(), dup.Len())
	// Output: 1 2
}
