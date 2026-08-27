/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package rb_tree_test

import (
	"fmt"

	"github.com/pschlump/pluto/rb_tree"
)

// Sorted input into a plain BST builds a degenerate chain; the red-black
// tree recolors and rotates as it goes, so the depth stays logarithmic.
func Example() {
	tree := rb_tree.NewRbTree[int]()
	for i := range 7 {
		tree.Insert(i) // 0..6 in ascending order
	}
	fmt.Println(tree.Depth(), tree.Length())
	// Output:
	// 4 7
}

// Naturally ordered keys — integers, floats, strings — need no comparison
// function at all.
func ExampleNewRbTree() {
	tree := rb_tree.NewRbTree[int]()
	for _, v := range []int{42, 7, 13, 99, 55} {
		tree.Insert(v)
	}

	fmt.Println(tree.Len(), tree.IsEmpty())

	min, _ := tree.FindMin()
	max, _ := tree.FindMax()
	fmt.Println(min, max)

	if _, found := tree.Search(13); found {
		fmt.Println("found 13")
	}
	if _, found := tree.Search(14); !found {
		fmt.Println("14 is not in the tree")
	}

	tree.Delete(42)
	fmt.Println(tree.Len())
	// Output:
	// 5 false
	// 7 99
	// found 13
	// 14 is not in the tree
	// 4
}

// Structs ordered by a field — the element type implements no interface;
// ordering is a plain function, and Compare builds the field comparison.
func ExampleNewRbTreeFunc() {
	type Employee struct {
		ID   int
		Name string
	}

	byID := rb_tree.NewRbTreeFunc(func(a, b Employee) int {
		return rb_tree.Compare(a.ID, b.ID)
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
	fmt.Println(rb_tree.Compare("apple", "banana"))
	fmt.Println(rb_tree.Compare(3.5, 1.5))
	fmt.Println(rb_tree.Compare(7, 7))
	// Output:
	// -1
	// 1
	// 0
}

// All iterates in ascending order; Backward iterates in descending order.
// Both are successor/predecessor walks over the parent pointers, with O(1)
// extra space.
func ExampleRbTree_All() {
	tree := rb_tree.NewRbTree[string]()
	for _, s := range []string{"pear", "fig", "apple", "date"} {
		tree.Insert(s)
	}

	var ascending []string
	for s := range tree.All() {
		ascending = append(ascending, s)
	}

	var descending []string
	for s := range tree.Backward() {
		descending = append(descending, s)
	}

	fmt.Println(ascending)
	fmt.Println(descending)
	// Output:
	// [apple date fig pear]
	// [pear fig date apple]
}

// DeleteAtHead removes the smallest element — draining the tree this way
// enumerates the elements in ascending order.
func ExampleRbTree_DeleteAtHead() {
	tree := rb_tree.NewRbTree[int]()
	for _, v := range []int{42, 7, 13, 99} {
		tree.Insert(v)
	}

	var drained []int
	for !tree.IsEmpty() {
		v, _ := tree.FindMin()
		drained = append(drained, v)
		tree.DeleteAtHead()
	}
	fmt.Println(drained)
	// Output:
	// [7 13 42 99]
}

// Truncate drops every element in O(1) but keeps the comparison function,
// so the tree is immediately reusable.
func ExampleRbTree_Truncate() {
	tree := rb_tree.NewRbTreeFunc(func(a, b int) int {
		return a - b
	})
	tree.Insert(1)
	tree.Insert(2)

	tree.Truncate()
	fmt.Println(tree.Len(), tree.IsEmpty())

	tree.Insert(9)
	fmt.Println(tree.Len())
	// Output:
	// 0 true
	// 1
}
