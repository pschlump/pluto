/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package b_tree_test

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pschlump/pluto/b_tree"
)

// Sorted input into a plain BST builds a degenerate chain; the B-tree
// splits full nodes as it goes, so the depth stays logarithmic.
func Example() {
	tree := b_tree.NewBTree[int](4)
	for i := range 100 {
		tree.Insert(i) // 0..99 in ascending order
	}
	fmt.Println(tree.Depth(), tree.Length())
	// Output:
	// 4 100
}

// Naturally ordered keys — integers, floats, strings — need no comparison
// function at all.  order is the maximum number of children per node.
func ExampleNewBTree() {
	tree := b_tree.NewBTree[int](4)
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

	tree.Delete(42)
	fmt.Println(tree.Len())
	// Output:
	// 5 false
	// 7 99
	// found 13
	// 4
}

// Structs ordered by a field — the element type implements no interface;
// ordering is a plain function, and Compare builds the field comparison.
func ExampleNewBTreeFunc() {
	type Employee struct {
		ID   int
		Name string
	}

	byID := b_tree.NewBTreeFunc(4, func(a, b Employee) int {
		return b_tree.Compare(a.ID, b.ID)
	})
	byID.Insert(Employee{ID: 3, Name: "Ada"})
	byID.Insert(Employee{ID: 1, Name: "Grace"})
	byID.Insert(Employee{ID: 2, Name: "Edsger"})

	// A search probe only needs the fields the comparison reads: the ID.
	if e, found := byID.Search(Employee{ID: 2}); found {
		fmt.Println(e.Name)
	}

	for _, e := range byID.All() {
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
	fmt.Println(b_tree.Compare("apple", "banana"))
	fmt.Println(b_tree.Compare(3.5, 1.5))
	fmt.Println(b_tree.Compare(7, 7))
	// Output:
	// -1
	// 1
	// 0
}

// All iterates in ascending order; Backward iterates in descending
// order.  Both yield (index, value) pairs — a single-variable range
// yields the index.
func ExampleBTree_All() {
	tree := b_tree.NewBTree[string](4)
	for _, s := range []string{"pear", "fig", "apple", "date"} {
		tree.Insert(s)
	}

	var ascending []string
	for _, s := range tree.All() {
		ascending = append(ascending, s)
	}

	var descending []string
	for _, s := range tree.Backward() {
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
func ExampleBTree_DeleteAtHead() {
	tree := b_tree.NewBTree[int](4)
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

// Truncate drops every element in O(1) but keeps the comparison function
// and the order, so the tree is immediately reusable.
func ExampleBTree_Truncate() {
	tree := b_tree.NewBTreeFunc(4, func(a, b int) int {
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

// Dump shows the node structure of the tree, one line per node with its
// keys, indented by depth.
func ExampleBTree_Dump() {
	tree := b_tree.NewBTree[int](3)
	for v := 1; v <= 7; v++ {
		tree.Insert(v)
	}
	tree.Dump(os.Stdout)
	// Output:
	// [4]
	//   [2]
	//     [1]
	//     [3]
	//   [6]
	//     [5]
	//     [7]
}

// MarshalJSON encodes the tree as a JSON array of its elements in
// ascending order, regardless of insert order.
func ExampleBTree_MarshalJSON() {
	tree := b_tree.NewBTree[int](4)
	tree.Insert(3)
	tree.Insert(1)
	tree.Insert(2)

	b, err := json.Marshal(tree)
	fmt.Println(string(b), err)
	// Output:
	// [1,2,3] <nil>
}

// UnmarshalJSON replaces the contents of the tree from a JSON array; the
// tree is an ordered set, so iteration afterwards is ascending.
func ExampleBTree_UnmarshalJSON() {
	tree := b_tree.NewBTree[string](4)
	if err := json.Unmarshal([]byte(`["c","a"]`), tree); err != nil {
		fmt.Println("error:", err)
		return
	}
	for i, v := range tree.All() {
		fmt.Println(i, v)
	}
	// Output:
	// 0 a
	// 1 c
}
