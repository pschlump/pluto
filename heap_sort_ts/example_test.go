/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package heap_sort_ts_test

import (
	"fmt"
	"sync"

	"github.com/pschlump/charon/heap_sort_ts"
)

// A sorter of structs ordered by one field — the element type
// implements no interface.  The calls are the plain heap_sort API; the
// sorter may be shared between goroutines.
func Example() {
	type Item struct {
		Weight int
		Name   string
	}

	hs := heap_sort_ts.NewHeapSortFunc(func(a, b Item) int {
		return a.Weight - b.Weight
	})
	hs.Insert(Item{7, "anchor"})
	hs.Insert(Item{2, "oar"})
	hs.Insert(Item{5, "plank"})

	for _, it := range hs.Sort() {
		fmt.Printf("%d:%s ", it.Weight, it.Name)
	}
	fmt.Println()
	// Output:
	// 2:oar 5:plank 7:anchor
}

// Naturally ordered types need no comparison function at all.
func ExampleNewHeapSort() {
	hs := heap_sort_ts.NewHeapSort[int]()
	for _, v := range []int{42, 7, 13} {
		hs.Insert(v)
	}
	fmt.Println(hs.Sort())
	// Output:
	// [7 13 42]
}

// SortDown drains the sorter in descending order.
func ExampleHeapSort_SortDown() {
	hs := heap_sort_ts.NewHeapSort[string]()
	for _, v := range []string{"pear", "apple", "fig"} {
		hs.Insert(v)
	}
	fmt.Println(hs.SortDown(), hs.Len())
	// Output:
	// [pear fig apple] 0
}

// InsertArray bulk-adds a slice and rebuilds the heap in one pass —
// cheaper than Insert per element, and atomic: the whole batch lands
// under one hold of the write lock.
func ExampleHeapSort_InsertArray() {
	hs := heap_sort_ts.NewHeapSort[int]()
	hs.InsertArray([]int{9, 4, 1, 7})
	hs.InsertArray([]int{2, 8})
	fmt.Println(hs.Len(), hs.Sort())
	// Output:
	// 6 [1 2 4 7 8 9]
}

// Sorting empties the sorter; it is immediately reusable.
func ExampleHeapSort_Truncate() {
	hs := heap_sort_ts.NewHeapSort[int]()
	for i := range 5 {
		hs.Insert(i)
	}
	_ = hs.Sort() // drains the sorter

	hs.Truncate() // also drops any elements still waiting
	fmt.Println(hs.Len(), hs.IsEmpty())

	hs.Insert(9)
	fmt.Println(hs.Len(), hs.Sort())
	// Output:
	// 0 true
	// 1 [9]
}

// An atomic insert-batch-then-sort sequence under the exposed write
// lock: no other goroutine can observe or interleave with the batch.
func ExampleHeapSort_Lock() {
	hs := heap_sort_ts.NewHeapSort[int]()
	hs.Insert(20)

	var wg sync.WaitGroup
	wg.Go(func() { hs.Insert(30) })
	wg.Wait()

	hs.Lock()
	hs.NlInsertArray([]int{5, 25})
	// NlSort drains everything present under the same hold of the lock.
	fmt.Println(hs.NlSort())
	hs.Unlock()

	fmt.Println(hs.IsEmpty())
	// Output:
	// [5 20 25 30]
	// true
}
