/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package heap_sort_test

import (
	"fmt"

	"github.com/pschlump/charon/heap_sort"
)

// A sorter of structs ordered by one field — the element type
// implements no interface.
func Example() {
	type Item struct {
		Weight int
		Name   string
	}

	hs := heap_sort.NewHeapSortFunc(func(a, b Item) int {
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
	hs := heap_sort.NewHeapSort[int]()
	for _, v := range []int{42, 7, 13} {
		hs.Insert(v)
	}
	fmt.Println(hs.Sort())
	// Output:
	// [7 13 42]
}

// SortDown drains the sorter in descending order.
func ExampleHeapSort_SortDown() {
	hs := heap_sort.NewHeapSort[string]()
	for _, v := range []string{"pear", "apple", "fig"} {
		hs.Insert(v)
	}
	fmt.Println(hs.SortDown(), hs.Len())
	// Output:
	// [pear fig apple] 0
}

// InsertArray bulk-adds a slice and rebuilds the heap in one pass —
// cheaper than Insert per element.
func ExampleHeapSort_InsertArray() {
	hs := heap_sort.NewHeapSort[int]()
	hs.InsertArray([]int{9, 4, 1, 7})
	hs.InsertArray([]int{2, 8})
	fmt.Println(hs.Len(), hs.Sort())
	// Output:
	// 6 [1 2 4 7 8 9]
}

// Sorting empties the sorter; it is immediately reusable.
func ExampleHeapSort_Truncate() {
	hs := heap_sort.NewHeapSort[int]()
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
