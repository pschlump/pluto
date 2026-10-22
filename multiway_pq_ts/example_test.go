/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiway_pq_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/multiway_pq_ts"
)

func Example() {
	h := multiway_pq_ts.NewDHeap[int](4) // d=4: shallower tree, cheaper decrease-key
	for _, v := range []int{9, 4, 7, 1, 8} {
		h.Push(v)
	}
	for !h.IsEmpty() {
		v, _ := h.Pop()
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output:
	// 1 4 7 8 9
}

func ExampleNewDHeapFunc() {
	type task struct {
		name string
		work int
	}
	// A reversed comparison turns the min-heap into a max-heap: the
	// largest task pops first.
	h := multiway_pq_ts.NewDHeapFunc(3, func(a, b task) int { return b.work - a.work })
	h.Push(task{"deploy", 9})
	h.Push(task{"fmt", 1})
	h.Push(task{"test", 5})
	v, _ := h.Pop()
	fmt.Println(v.name)
	// Output:
	// deploy
}

func ExampleDHeap_lock() {
	h := multiway_pq_ts.NewDHeap[int](4)

	// The canonical compound bulk build: NlAppendHeap leaves a
	// non-heap, and the bottom-up NlHeapify sweep under the SAME hold
	// of Lock makes the whole build atomic (the heap_sort_ts
	// precedent).
	x := []int{9, 4, 7, 1, 8, 2}
	h.Lock()
	h.NlAppendHeap(x)
	for i := (h.NlLen() - 2) / h.D(); i >= 0; i-- {
		h.NlHeapify(h.NlLen(), i)
	}
	h.Unlock()

	v, _ := h.Peek()
	fmt.Println(v, h.Len())
	// Output:
	// 1 6
}

func ExampleDHeap_all() {
	h := multiway_pq_ts.NewDHeap[int](3)
	for _, v := range []int{5, 1, 4} {
		h.Push(v)
	}
	// All walks a snapshot in internal breadth-first order — NOT
	// sorted order (Pop is the sorted drain) — so mutating inside the
	// loop is safe.
	for v := range h.All() {
		fmt.Print(v, " ")
		h.Pop()
	}
	fmt.Println(h.IsEmpty())
	// Output:
	// 1 5 4 true
}
