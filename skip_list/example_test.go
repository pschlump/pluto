/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.  Only
// height-independent output is printed: the tower heights are drawn at
// random, so anything involving levels would be nondeterministic.
package skip_list_test

import (
	"fmt"

	"github.com/pschlump/pluto/skip_list"
)

// Sorted input into a skip list is no problem: the express lanes keep
// search, insert and delete at O(log₂ n) expected regardless of the order
// the items arrive in.
func Example() {
	list := skip_list.NewSkipList[int]()
	for i := range 7 {
		list.Insert(i) // 0..6 in ascending order
	}

	min, _ := list.FindMin()
	max, _ := list.FindMax()
	fmt.Println(list.Len(), min, max)
	// Output:
	// 7 0 6
}

// Naturally ordered keys — integers, floats, strings — need no comparison
// function at all.
func ExampleNewSkipList() {
	list := skip_list.NewSkipList[int]()
	for _, v := range []int{42, 7, 13, 99, 55} {
		list.Insert(v)
	}

	fmt.Println(list.Len(), list.IsEmpty())

	min, _ := list.FindMin()
	max, _ := list.FindMax()
	fmt.Println(min, max)

	if _, found := list.Search(13); found {
		fmt.Println("found 13")
	}
	if _, found := list.Search(14); !found {
		fmt.Println("14 is not in the list")
	}

	list.Delete(42)
	fmt.Println(list.Len())
	// Output:
	// 5 false
	// 7 99
	// found 13
	// 14 is not in the list
	// 4
}

// Structs ordered by a field — the element type implements no interface;
// ordering is a plain function, and Compare builds the field comparison.
func ExampleNewSkipListFunc() {
	type Employee struct {
		ID   int
		Name string
	}

	byID := skip_list.NewSkipListFunc(func(a, b Employee) int {
		return skip_list.Compare(a.ID, b.ID)
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
	fmt.Println(skip_list.Compare("apple", "banana"))
	fmt.Println(skip_list.Compare(3.5, 1.5))
	fmt.Println(skip_list.Compare(7, 7))
	// Output:
	// -1
	// 1
	// 0
}

// All iterates in ascending order; Backward iterates in descending order.
// Backward collects the items first, because skip list nodes have no back
// pointers.
func ExampleSkipList_All() {
	list := skip_list.NewSkipList[string]()
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

// DeleteAtHead removes the smallest element in O(1) — draining the list
// this way enumerates the elements in ascending order.
func ExampleSkipList_DeleteAtHead() {
	list := skip_list.NewSkipList[int]()
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
	list := skip_list.NewSkipListFunc(func(a, b int) int {
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

// Rank reports the 0-based position of a key in ascending order, without
// walking the list — the span counters on the forward pointers carry the
// rank arithmetic.  AtIndex is its inverse.
func ExampleSkipList_Rank() {
	list := skip_list.NewSkipList[string]()
	for _, s := range []string{"pear", "fig", "apple", "date"} {
		list.Insert(s)
	}

	r, _ := list.Rank("date")
	fmt.Println("date ranks", r)

	v, _ := list.AtIndex(1)
	fmt.Println("rank 1 holds", v)

	if _, found := list.Rank("kiwi"); !found {
		fmt.Println("kiwi is not in the list")
	}
	// Output:
	// date ranks 1
	// rank 1 holds date
	// kiwi is not in the list
}

// Range iterates the elements x with lo <= x <= hi in ascending order,
// yielding each element's global rank alongside its value.  RangeBackward
// is the same walk from largest to smallest.
func ExampleSkipList_Range() {
	list := skip_list.NewSkipList[int]()
	for _, v := range []int{42, 7, 13, 99, 55, 3} {
		list.Insert(v)
	}

	for i, v := range list.Range(10, 60) {
		fmt.Println(i, v)
	}
	fmt.Println("count:", list.CountRange(10, 60))

	n := list.DeleteRange(10, 60)
	fmt.Println("removed:", n, "leaving", list.Len())
	// Output:
	// 2 13
	// 3 42
	// 4 55
	// count: 3
	// removed: 3 leaving 3
}
