/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.  Splaying makes the
// tree's shape depend on the full history of operations, so only
// shape-independent results (in-order sequences, min/max, membership) are
// printed here — they are deterministic for any history.
package splay_tree_test

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pschlump/pluto/splay_tree"
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
	tasks := splay_tree.NewSplayTreeFunc(func(a, b Task) int {
		if c := splay_tree.Compare(a.Priority, b.Priority); c != 0 {
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
func ExampleNewSplayTree() {
	tree := splay_tree.NewSplayTree[int]()
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
func ExampleNewSplayTreeFunc() {
	type Employee struct {
		ID   int
		Name string
	}

	byID := splay_tree.NewSplayTreeFunc(func(a, b Employee) int {
		return splay_tree.Compare(a.ID, b.ID)
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
	fmt.Println(splay_tree.Compare("apple", "banana"))
	fmt.Println(splay_tree.Compare(3.5, 1.5))
	fmt.Println(splay_tree.Compare(7, 7))
	// Output:
	// -1
	// 1
	// 0
}

// All iterates in ascending order; Backward iterates in descending order.
// Both yield (index, value) pairs — a single-variable range yields the
// INDEX, not the value.
func ExampleSplayTree_All() {
	tree := splay_tree.NewSplayTree[string]()
	for _, s := range []string{"pear", "fig", "apple", "date"} {
		tree.Insert(s)
	}

	var ascending []string
	for _, s := range tree.All() {
		ascending = append(ascending, s)
	}

	var descending []string
	for i, s := range tree.Backward() {
		descending = append(descending, fmt.Sprintf("%d:%s", i, s))
	}

	fmt.Println(ascending)
	fmt.Println(descending)
	// Output:
	// [apple date fig pear]
	// [3:pear 2:fig 1:date 0:apple]
}

// Repeated searches for the same key are the splay tree's best case: the
// first Search splays the found node to the root, so the next one is a
// single comparison.
func ExampleSplayTree_Search() {
	tree := splay_tree.NewSplayTree[int]()
	for _, v := range []int{50, 20, 80, 10, 30, 70, 90} {
		tree.Insert(v)
	}

	for range 2 {
		if v, found := tree.Search(30); found {
			fmt.Println(v)
		}
	}
	// Output:
	// 30
	// 30
}

// DeleteAtHead removes the smallest element — draining the tree this way
// enumerates the elements in ascending order.
func ExampleSplayTree_DeleteAtHead() {
	tree := splay_tree.NewSplayTree[int]()
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
func ExampleSplayTree_Truncate() {
	tree := splay_tree.NewSplayTreeFunc(func(a, b int) int {
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

// MarshalJSON encodes the tree as a JSON array of its elements in in-order
// (ascending) order.
func ExampleSplayTree_MarshalJSON() {
	tree := splay_tree.NewSplayTree[int]()
	tree.Insert(3)
	tree.Insert(1)
	tree.Insert(2)

	b, err := json.Marshal(tree)
	fmt.Println(string(b), err)
	// Output:
	// [1,2,3] <nil>
}

// UnmarshalJSON replaces the contents of the tree from a JSON array.
func ExampleSplayTree_UnmarshalJSON() {
	tree := splay_tree.NewSplayTree[string]()
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
