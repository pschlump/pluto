/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package priority_queue_ts_test

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/pschlump/pluto/priority_queue_ts"
)

// Writers from many goroutines share one queue; the total drain is
// deterministic even though the interleaving is not.
func Example() {
	pq := priority_queue_ts.NewPriorityQueueFunc(func(a, b int) int { return a - b })

	const producers = 4
	const perProducer = 25

	var wg sync.WaitGroup
	for p := range producers {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			for i := range perProducer {
				pq.Insert(p*perProducer + i + 1) // values 1..100
			}
		}(p)
	}
	wg.Wait()

	fmt.Println(pq.Len(), pq.IsEmpty())

	// Pops drain in ascending order.
	first, _ := pq.Peek()
	drained := 0
	for !pq.IsEmpty() {
		if _, found := pq.Pop(); !found {
			break
		}
		drained++
	}
	fmt.Println(first, drained)
	// Output:
	// 100 false
	// 1 100
}

// A priority queue of tasks ordered by a priority field — the element
// type implements no interface.
func ExamplePriorityQueue_Insert() {
	type Task struct {
		Priority int
		Name     string
	}

	pq := priority_queue_ts.NewPriorityQueueFunc(func(a, b Task) int {
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
	pq := priority_queue_ts.NewPriorityQueue[int]()
	for _, v := range []int{42, 7, 13} {
		pq.Insert(v)
	}

	min, _ := pq.Peek()
	fmt.Println("next:", min)
	// Output:
	// next: 7
}

// A reversed comparison makes the highest priority come out first.
func ExampleNewPriorityQueueFunc() {
	pq := priority_queue_ts.NewPriorityQueueFunc(func(a, b int) int { return b - a })
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

	pq := priority_queue_ts.NewPriorityQueueFunc(byPriority)
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

// All iterates in priority order (minimum first) over a snapshot taken
// when it is called, so it is safe to mutate the queue from inside the
// loop.
func ExamplePriorityQueue_All() {
	pq := priority_queue_ts.NewPriorityQueue[int]()
	for _, v := range []int{5, 1, 3} {
		pq.Insert(v)
	}

	var order []int
	for v := range pq.All() {
		order = append(order, v)
		pq.Pop() // safe: All walks a snapshot
	}
	fmt.Println(order, pq.Len())
	// Output:
	// [1 3 5] 0
}

// Lock and the Nl-prefixed methods build atomic search-then-update
// sequences; while the lock is held only the Nl methods may be used.
func ExamplePriorityQueue_Lock() {
	type Task struct {
		Priority int
		Name     string
	}
	byPriority := func(a, b Task) int { return a.Priority - b.Priority }

	pq := priority_queue_ts.NewPriorityQueueFunc(byPriority)
	pq.Insert(Task{20, "review"})
	pq.Insert(Task{10, "write"})

	pq.Lock()
	// Atomically find the priority-20 element and demote it below the
	// current minimum.
	if v, pos, found := pq.NlSearch(Task{Priority: 20}); found {
		pq.NlUpdatePriority(pos, Task{Priority: v.Priority - 100, Name: v.Name})
	}
	pq.Unlock()

	v, _ := pq.Peek()
	fmt.Println(v.Name)
	// Output:
	// review
}

// Truncate drops every element in O(1); the queue is immediately
// reusable.
func ExamplePriorityQueue_Truncate() {
	pq := priority_queue_ts.NewPriorityQueue[int]()
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

// MarshalJSON encodes the queue as a JSON array of its elements in
// priority order, minimum first — the insertion order does not survive
// the heap.
func ExamplePriorityQueue_MarshalJSON() {
	pq := priority_queue_ts.NewPriorityQueue[int]()
	pq.Insert(3)
	pq.Insert(1)
	pq.Insert(2)

	b, err := json.Marshal(pq)
	fmt.Println(string(b), err)
	// Output:
	// [1,2,3] <nil>
}

// UnmarshalJSON replaces the contents of the queue from a JSON array;
// the elements come back out in priority order.
func ExamplePriorityQueue_UnmarshalJSON() {
	pq := priority_queue_ts.NewPriorityQueue[string]()
	if err := json.Unmarshal([]byte(`["c","a"]`), pq); err != nil {
		fmt.Println("error:", err)
		return
	}
	for v := range pq.All() {
		fmt.Println(v)
	}
	// Output:
	// a
	// c
}
