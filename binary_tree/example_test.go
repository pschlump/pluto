/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package binary_tree_test

import (
	"fmt"
	"strings"

	"github.com/pschlump/pluto/binary_tree"
)

// A small task list kept in priority order (highest first, ties by name),
// drained highest-priority-first — a priority queue built from a tree with
// a reversed comparison function.
func Example() {
	type Task struct {
		Priority int
		Name     string
	}

	// Order by priority (natural order), ties broken by name; draining
	// from the tail with FindMax/DeleteAtTail then yields the highest
	// priority first.  Compare composes into custom comparison functions
	// field by field.
	tasks := binary_tree.NewBinaryTreeFunc(func(a, b Task) int {
		if c := binary_tree.Compare(a.Priority, b.Priority); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})

	for _, t := range []Task{
		{Priority: 2, Name: "compile"},
		{Priority: 1, Name: "email"},
		{Priority: 3, Name: "fix build"},
		{Priority: 2, Name: "test"},
	} {
		tasks.Insert(t)
	}

	for !tasks.IsEmpty() {
		top, _ := tasks.FindMax()
		tasks.DeleteAtTail()
		fmt.Printf("%d %s\n", top.Priority, top.Name)
	}
	// Output:
	// 3 fix build
	// 2 test
	// 2 compile
	// 1 email
}

// Naturally ordered keys — integers, floats, strings — need no comparison
// function at all.
func ExampleNewBinaryTree() {
	tree := binary_tree.NewBinaryTree[int]()
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
func ExampleNewBinaryTreeFunc() {
	type Employee struct {
		ID   int
		Name string
	}

	byID := binary_tree.NewBinaryTreeFunc(func(a, b Employee) int {
		return binary_tree.Compare(a.ID, b.ID)
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
	fmt.Println(binary_tree.Compare("apple", "banana"))
	fmt.Println(binary_tree.Compare(3.5, 1.5))
	fmt.Println(binary_tree.Compare(7, 7))
	// Output:
	// -1
	// 1
	// 0
}

// All iterates in ascending order; Backward iterates in descending order.
// Both work directly in a range loop (Go 1.23+).
func ExampleBinaryTree_All() {
	tree := binary_tree.NewBinaryTree[string]()
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

// The old-style iterator can be paused and resumed — here only the first
// two elements of the in-order traversal are consumed.
func ExampleBinaryTree_Front() {
	tree := binary_tree.NewBinaryTree[int]()
	for i := range 5 {
		tree.Insert(i * 3) // 0 3 6 9 12
	}

	var firstTwo []int
	count := 0
	for it := tree.Front(); !it.Done() && count < 2; it.Next() {
		v, _ := it.Value()
		firstTwo = append(firstTwo, v)
		count++
	}
	fmt.Println(firstTwo)
	// Output:
	// [0 3]
}

// Walk callbacks receive the position in walk order and the node depth;
// caller state is captured in the closure and keeps its static type.
// Returning false stops the walk.
func ExampleBinaryTree_WalkInOrder() {
	tree := binary_tree.NewBinaryTree[string]()
	for _, s := range []string{"mango", "apple", "peach", "banana"} {
		tree.Insert(s)
	}

	var visited []string
	tree.WalkInOrder(func(pos, depth int, data string) bool {
		visited = append(visited, fmt.Sprintf("%d:%d:%s", pos, depth, data))
		return len(visited) < 3 // false stops the walk
	})
	fmt.Println(strings.Join(visited, " "))
	// Output:
	// 0:1:apple 1:2:banana 2:0:mango
}

// Index answers rank queries on the in-order sequence: Index(0) is the
// minimum, Index(Len()-1) the maximum, and Index(Len()/2) the median.
func ExampleBinaryTree_Index() {
	tree := binary_tree.NewBinaryTree[int]()
	for _, v := range []int{50, 20, 80, 10, 30, 70, 90} {
		tree.Insert(v)
	}

	median, _ := tree.Index(tree.Len() / 2)
	fmt.Println(median, tree.Depth())
	// Output:
	// 50 3
}

// DeleteAtHead removes the smallest element in one descent — draining the
// tree this way enumerates the elements in ascending order.
func ExampleBinaryTree_DeleteAtHead() {
	tree := binary_tree.NewBinaryTree[int]()
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
func ExampleBinaryTree_Truncate() {
	tree := binary_tree.NewBinaryTreeFunc(func(a, b int) int {
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
