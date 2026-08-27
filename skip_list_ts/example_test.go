/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.  Only
// height-independent output is printed: the tower heights are drawn at
// random, so anything involving levels would be nondeterministic.
package skip_list_ts_test

import (
	"fmt"
	"sync"

	"github.com/pschlump/pluto/skip_list_ts"
)

// Writers from many goroutines share one list; readers see a consistent
// result once the writers finish.
func Example() {
	list := skip_list_ts.NewSkipList[int]()

	var wg sync.WaitGroup
	for w := range 4 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range 25 {
				list.Insert(w*100 + i)
			}
		}(w)
	}
	wg.Wait()

	fmt.Println(list.Len(), list.IsEmpty())

	min, _ := list.FindMin()
	max, _ := list.FindMax()
	fmt.Println(min, max)
	// Output:
	// 100 false
	// 0 324
}

// Sorted input into a skip list is no problem: the express lanes keep
// search, insert and delete at O(log₂ n) expected regardless of the order
// the items arrive in.
func ExampleNewSkipList() {
	list := skip_list_ts.NewSkipList[int]()
	for i := range 7 {
		list.Insert(i) // 0..6 in ascending order
	}

	min, _ := list.FindMin()
	max, _ := list.FindMax()
	fmt.Println(list.Len(), min, max)
	// Output:
	// 7 0 6
}

// Structs ordered by a field — the element type implements no interface;
// ordering is a plain function, and Compare builds the field comparison.
func ExampleNewSkipListFunc() {
	type Employee struct {
		ID   int
		Name string
	}

	byID := skip_list_ts.NewSkipListFunc(func(a, b Employee) int {
		return skip_list_ts.Compare(a.ID, b.ID)
	})
	byID.Insert(Employee{ID: 3, Name: "Ada"})
	byID.Insert(Employee{ID: 1, Name: "Grace"})
	byID.Insert(Employee{ID: 2, Name: "Edsger"})

	// A search probe only needs the fields the comparison reads: the ID.
	if e, found := byID.Search(Employee{ID: 2}); found {
		fmt.Println(e.Name)
	}

	for e := range byID.All() {
		fmt.Println(e.ID, e.Name)
	}
	// Output:
	// Edsger
	// 1 Grace
	// 2 Edsger
	// 3 Ada
}

// Compare orders any naturally ordered type and is handy inside custom
// comparison functions.
func ExampleCompare() {
	fmt.Println(skip_list_ts.Compare("apple", "banana"))
	fmt.Println(skip_list_ts.Compare(3.5, 1.5))
	fmt.Println(skip_list_ts.Compare(7, 7))
	// Output:
	// -1
	// 1
	// 0
}

// All iterates in ascending order; Backward iterates in descending order.
// Both operate on a snapshot taken when they are called.
func ExampleSkipList_All() {
	list := skip_list_ts.NewSkipList[string]()
	for _, s := range []string{"pear", "fig", "apple", "date"} {
		list.Insert(s)
	}

	var ascending []string
	for s := range list.All() {
		ascending = append(ascending, s)
	}

	var descending []string
	for s := range list.Backward() {
		descending = append(descending, s)
	}

	fmt.Println(ascending)
	fmt.Println(descending)
	// Output:
	// [apple date fig pear]
	// [pear fig date apple]
}

// Because All iterates a snapshot, mutating the list from inside the loop
// is safe — the loop still visits every element captured at the start.
func ExampleSkipList_All_snapshot() {
	list := skip_list_ts.NewSkipList[string]()
	for _, s := range []string{"pear", "fig", "apple"} {
		list.Insert(s)
	}

	var visited []string
	for v := range list.All() {
		visited = append(visited, v)
		list.Delete(v)
	}
	fmt.Println(visited, list.Len())
	// Output:
	// [apple fig pear] 0
}

// DeleteAtHead removes the smallest element in O(1) — draining the list
// this way enumerates the elements in ascending order.
func ExampleSkipList_DeleteAtHead() {
	list := skip_list_ts.NewSkipList[int]()
	for _, v := range []int{42, 7, 13, 99} {
		list.Insert(v)
	}

	var drained []int
	for !list.IsEmpty() {
		v, _ := list.FindMin()
		drained = append(drained, v)
		list.DeleteAtHead()
	}
	fmt.Println(drained)
	// Output:
	// [7 13 42 99]
}

// Truncate drops every element in O(1) but keeps the comparison function,
// so the list is immediately reusable.
func ExampleSkipList_Truncate() {
	list := skip_list_ts.NewSkipListFunc(func(a, b int) int {
		return a - b
	})
	list.Insert(1)
	list.Insert(2)

	list.Truncate()
	fmt.Println(list.Len(), list.IsEmpty())

	list.Insert(9)
	fmt.Println(list.Len())
	// Output:
	// 0 true
	// 1
}
