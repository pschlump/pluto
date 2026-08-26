/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package heap_ts_test

import (
	"fmt"
	"sync"

	"github.com/pschlump/charon/heap_ts"
)

// Writers from many goroutines share one heap; the total drain is
// deterministic even though the interleaving is not.
func Example() {
	hp := heap_ts.NewHeap[int]()

	const producers = 4
	const perProducer = 25

	var wg sync.WaitGroup
	for p := range producers {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			for i := range perProducer {
				hp.Push(p*perProducer + i + 1) // values 1..100
			}
		}(p)
	}
	wg.Wait()

	fmt.Println(hp.Len(), hp.IsEmpty())

	// Pops drain in ascending order.
	min, _ := hp.Peek()
	last, _ := hp.Pop()
	drained := 1
	for !hp.IsEmpty() {
		v, _ := hp.Pop()
		drained++
		last = v
	}
	fmt.Println(min, last, drained)
	// Output:
	// 100 false
	// 1 100 100
}

// A min-heap: Push adds, Peek sees the minimum, Pop drains ascending.  A
// reversed comparison makes it a max-heap.
func ExampleNewHeapFunc() {
	maxHeap := heap_ts.NewHeapFunc(func(a, b int) int { return -heap_ts.Compare(a, b) })
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

	pq := heap_ts.NewHeapFunc(func(a, b Task) int {
		return heap_ts.Compare(a.Priority, b.Priority)
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
	fmt.Println(heap_ts.Compare("apple", "banana"))
	fmt.Println(heap_ts.Compare(3.5, 1.5))
	fmt.Println(heap_ts.Compare(7, 7))
	// Output:
	// -1
	// 1
	// 0
}

// Fix replaces the element at an index and restores the heap ordering in
// place — cheaper than a Delete followed by a Push.
func ExampleHeap_Fix() {
	hp := heap_ts.NewHeap[string]()
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

// Search finds an element and its internal index; the index composes
// with Delete and Fix.  A probe only needs the fields the comparison
// function reads.
func ExampleHeap_Search() {
	type rec struct {
		ID   int
		Name string
	}
	byID := heap_ts.NewHeapFunc(func(a, b rec) int { return heap_ts.Compare(a.ID, b.ID) })
	byID.Push(rec{3, "ada"})
	byID.Push(rec{1, "grace"})
	byID.Push(rec{2, "edsger"})

	if v, idx, found := byID.Search(rec{ID: 3}); found {
		fmt.Println(v.Name, idx)
		byID.Delete(idx)
	}
	if _, _, found := byID.Search(rec{ID: 3}); found {
		fmt.Println("still there")
	} else {
		fmt.Println("3 is gone")
	}
	// Output:
	// ada 1
	// 3 is gone
}

// All iterates a snapshot of the heap in internal (breadth-first) order —
// NOT sorted order — so it is safe to mutate the heap from inside the
// loop.
func ExampleHeap_All() {
	hp := heap_ts.NewHeap[int]()
	for i := range 5 {
		hp.Push(i)
	}

	for v := range hp.All() {
		fmt.Print(v, " ")
		hp.Pop() // safe: All walks a snapshot
	}
	fmt.Println(hp.IsEmpty())
	// Output:
	// 0 1 2 3 4 true
}

// Lock and the Nl-prefixed methods build compound operations that are
// atomic with respect to other writers.
func ExampleHeap_Lock() {
	hp := heap_ts.NewHeap[int]()
	hp.Push(5)
	hp.Push(1)

	hp.Lock()
	// While the lock is held only the Nl (no-lock) methods may be used.
	if v, found := hp.NlGetValue(0); found && v > 3 {
		hp.NlFix(0, 3) // clamp the minimum down, atomically
	}
	n := hp.NlLen()
	hp.Unlock()

	fmt.Println(n)
	v, _ := hp.Peek()
	fmt.Println(v)
	// Output:
	// 2
	// 1
}

// AppendHeap bulk-appends unordered; Heapify rebuilds the heap in O(n).
func ExampleHeap_AppendHeap() {
	hp := heap_ts.NewHeap[int]()
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
	hp := heap_ts.NewHeap[int]()
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
