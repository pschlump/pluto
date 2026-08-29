/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package quick_sort_test

import (
	"fmt"

	"github.com/pschlump/pluto/quick_sort"
)

// Sorting a slice of structs by one field — the element type
// implements no interface.
func Example() {
	type Item struct {
		Name   string
		Weight int
	}

	items := []Item{{"anchor", 7}, {"oar", 2}, {"plank", 5}}
	quick_sort.SortFunc(items, func(a, b Item) int {
		return a.Weight - b.Weight
	})

	for _, it := range items {
		fmt.Printf("%d:%s ", it.Weight, it.Name)
	}
	fmt.Println()
	// Output:
	// 2:oar 5:plank 7:anchor
}

// Naturally ordered types need no comparison function at all.  Sort
// orders the slice in place.
func ExampleSort() {
	data := []int{42, 7, 13, 7, 1}
	quick_sort.Sort(data)
	fmt.Println(data)
	// Output:
	// [1 7 7 13 42]
}

// SortFunc orders any type by a caller supplied comparison function;
// a reversed comparison yields descending order.
func ExampleSortFunc() {
	data := []string{"pear", "apple", "fig"}
	quick_sort.SortFunc(data, func(a, b string) int {
		return len(a) - len(b) // ascending by length
	})
	fmt.Println(data)
	// Output:
	// [fig pear apple]
}
