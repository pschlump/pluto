/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package priority_queue_test

import (
	"fmt"

	"github.com/pschlump/charon/priority_queue"
)

// A priority queue of tasks ordered by a priority field — the element
// type implements no interface.
func Example() {
	type Task struct {
		Priority int
		Name     string
	}

	pq := priority_queue.NewPriorityQueueFunc(func(a, b Task) int {
		return a.Priority - b.Priority
	})
	pq.Insert(Task{3, "write"})
	pq.Insert(Task{1, "review"})
	pq.Insert(Task{2, "ship"})

	for !pq.IsEmpty() {
		v, _ := pq.Pop()
		fmt.Printf("%d:%s ", v.Priority, v.Name)
	}
	fmt.Println()
	// Output:
	// 1:review 2:ship 3:write
}

// Naturally ordered priorities need no comparison function at all.
func ExampleNewPriorityQueue() {
	pq := priority_queue.NewPriorityQueue[int]()
	for _, v := range []int{42, 7, 13} {
		pq.Insert(v)
	}

	min, _ := pq.Peek()
	fmt.Println("next:", min)

	var drained []int
	for !pq.IsEmpty() {
		v, _ := pq.Pop()
		drained = append(drained, v)
	}
	fmt.Println(drained)
	// Output:
	// next: 7
	// [7 13 42]
}

// A reversed comparison makes the highest priority come out first.
func ExampleNewPriorityQueueFunc() {
	pq := priority_queue.NewPriorityQueueFunc(func(a, b int) int { return b - a })
	for _, v := range []int{1, 5, 3} {
		pq.Insert(v)
	}

	for !pq.IsEmpty() {
		v, _ := pq.Pop()
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output:
	// 5 3 1
}

// Search finds an element by a key-only probe; the position it returns
// composes with UpdatePriority.
func ExamplePriorityQueue_UpdatePriority() {
	type Task struct {
		Priority int
		Name     string
	}
	byPriority := func(a, b Task) int { return a.Priority - b.Priority }

	pq := priority_queue.NewPriorityQueueFunc(byPriority)
	pq.Insert(Task{10, "write"})
	pq.Insert(Task{20, "review"})
	pq.Insert(Task{30, "ship"})

	// Demote "ship" (30) to the front of the line.
	if _, pos, found := pq.Search(Task{Priority: 30}); found {
		pq.UpdatePriority(pos, Task{Priority: 1, Name: "ship"})
	}

	v, _ := pq.Peek()
	fmt.Println(v.Name)
	// Output:
	// ship
}

// Search composes with Delete too: find by probe, remove by position.
func ExamplePriorityQueue_Search() {
	type rec struct {
		ID   int
		Name string
	}
	byID := priority_queue.NewPriorityQueueFunc(func(a, b rec) int { return a.ID - b.ID })
	byID.Insert(rec{3, "ada"})
	byID.Insert(rec{1, "grace"})
	byID.Insert(rec{2, "edsger"})

	if v, idx, found := byID.Search(rec{ID: 2}); found {
		fmt.Println(v.Name, idx)
		byID.Delete(idx)
	}
	if _, _, found := byID.Search(rec{ID: 2}); found {
		fmt.Println("still there")
	} else {
		fmt.Println("2 is gone")
	}
	// Output:
	// edsger 2
	// 2 is gone
}

// All iterates in priority order (minimum first) without draining the
// queue; it walks a snapshot, so mutating from inside the loop is safe.
func ExamplePriorityQueue_All() {
	pq := priority_queue.NewPriorityQueue[int]()
	for _, v := range []int{5, 1, 3} {
		pq.Insert(v)
	}

	var order []int
	for v := range pq.All() {
		order = append(order, v)
	}
	fmt.Println(order, pq.Len())
	// Output:
	// [1 3 5] 3
}

// Truncate drops every element in O(1); the queue is immediately
// reusable.
func ExamplePriorityQueue_Truncate() {
	pq := priority_queue.NewPriorityQueue[int]()
	for i := range 5 {
		pq.Insert(i)
	}

	pq.Truncate()
	fmt.Println(pq.Len(), pq.IsEmpty())

	pq.Insert(9)
	fmt.Println(pq.Len())
	// Output:
	// 0 true
	// 1
}
