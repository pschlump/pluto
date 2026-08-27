/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package heap_test

import (
	"fmt"

	"github.com/pschlump/pluto/heap"
)

// A min-heap: Push adds, Peek sees the minimum, Pop drains in ascending
// order.
func Example() {
	hp := heap.NewHeap[int]()
	for _, v := range []int{42, 7, 13, 99, 55} {
		hp.Push(v)
	}

	min, _ := hp.Peek()
	fmt.Println("minimum:", min)

	for hp.Len() > 0 {
		v, _ := hp.Pop()
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output:
	// minimum: 7
	// 7 13 42 55 99
}

// A max-heap: reverse the comparison function and the same structure
// pops in descending order.
func ExampleNewHeapFunc() {
	maxHeap := heap.NewHeapFunc(func(a, b int) int { return -heap.Compare(a, b) })
	for _, v := range []int{5, 1, 9, 3} {
		maxHeap.Push(v)
	}

	for maxHeap.Len() > 0 {
		v, _ := maxHeap.Pop()
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output:
	// 9 5 3 1
}

// A priority queue: structs ordered by a priority field — the element
// type implements no interface.
func ExampleHeap_Push() {
	type Task struct {
		Priority int
		Name     string
	}

	pq := heap.NewHeapFunc(func(a, b Task) int {
		return heap.Compare(a.Priority, b.Priority)
	})
	pq.Push(Task{3, "write"})
	pq.Push(Task{1, "review"})
	pq.Push(Task{2, "ship"})

	for pq.Len() > 0 {
		v, _ := pq.Pop()
		fmt.Printf("%d:%s ", v.Priority, v.Name)
	}
	fmt.Println()
	// Output:
	// 1:review 2:ship 3:write
}

// Compare orders any naturally ordered type and is handy inside custom
// comparison functions.
func ExampleCompare() {
	fmt.Println(heap.Compare("apple", "banana"))
	fmt.Println(heap.Compare(3.5, 1.5))
	fmt.Println(heap.Compare(7, 7))
	// Output:
	// -1
	// 1
	// 0
}

// Fix replaces the element at an index and restores the heap ordering in
// place — cheaper than a Delete followed by a Push.
func ExampleHeap_Fix() {
	hp := heap.NewHeap[string]()
	for _, s := range []string{"delta", "bravo", "charlie"} {
		hp.Push(s)
	}

	// Replace the minimum ("bravo") with something large; it sinks to its
	// new place.
	hp.Fix(0, "zulu")

	for hp.Len() > 0 {
		v, _ := hp.Pop()
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output:
	// charlie delta zulu
}

// Search finds an element and its internal index; the index works with
// Delete, Fix and GetValue.  A probe only needs the fields the
// comparison function reads.
func ExampleHeap_Search() {
	type rec struct {
		ID   int
		Name string
	}
	byID := heap.NewHeapFunc(func(a, b rec) int { return heap.Compare(a.ID, b.ID) })
	byID.Push(rec{3, "ada"})
	byID.Push(rec{1, "grace"})
	byID.Push(rec{2, "edsger"})

	if v, idx, found := byID.Search(rec{ID: 3}); found {
		fmt.Println(v.Name, idx)
		byID.Delete(idx)
	}
	if v, _, found := byID.Search(rec{ID: 3}); found {
		fmt.Println("still there:", v.Name)
	} else {
		fmt.Println("3 is gone")
	}
	// Output:
	// ada 1
	// 3 is gone
}

// AppendHeap bulk-appends without reordering; Heapify rebuilds the heap
// in O(n).
func ExampleHeap_AppendHeap() {
	hp := heap.NewHeap[int]()
	hp.Push(5)

	hp.AppendHeap([]int{2, 9, 0, 3})
	// Rebuild after the bulk append.
	for i := hp.Len()/2 - 1; i >= 0; i-- {
		hp.Heapify(hp.Len(), i)
	}

	for hp.Len() > 0 {
		v, _ := hp.Pop()
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output:
	// 0 2 3 5 9
}

// Truncate drops every element in O(1); the heap is immediately
// reusable.
func ExampleHeap_Truncate() {
	hp := heap.NewHeap[int]()
	for i := range 5 {
		hp.Push(i)
	}

	hp.Truncate()
	fmt.Println(hp.Len(), hp.IsEmpty())

	hp.Push(9)
	fmt.Println(hp.Len())
	// Output:
	// 0 true
	// 1
}
