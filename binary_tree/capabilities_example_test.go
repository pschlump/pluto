package binary_tree_test

// Checked examples for the set-convenience capabilities (capabilities.go).
// The same file (import aside) runs against the binary_tree_ts twin.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"

	"github.com/pschlump/pluto/binary_tree"
)

// Contains is the membership predicate — the found flag of Search.
func ExampleBinaryTree_Contains() {
	tree := binary_tree.NewBinaryTree[string]()
	tree.Insert("alpha")
	fmt.Println(tree.Contains("alpha"), tree.Contains("beta"))
	// Output:
	// true false
}

// Slice materializes the elements into a fresh slice, in-order
// (ascending) — no sorting needed.
func ExampleBinaryTree_Slice() {
	tree := binary_tree.NewBinaryTree[int]()
	tree.InsertSlice([]int{42, 7, 13})
	fmt.Println(tree.Slice())
	// Output:
	// [7 13 42]
}

// String renders the elements in-order with fmt.Sprint: ascending and
// deterministic.
func ExampleBinaryTree_String() {
	tree := binary_tree.NewBinaryTree[string]()
	tree.InsertSlice([]string{"pear", "apple", "fig"})
	fmt.Println(tree.String())
	// Output:
	// [apple fig pear]
}

// StringFunc is String with a caller supplied per-element format.
func ExampleBinaryTree_StringFunc() {
	tree := binary_tree.NewBinaryTree[int]()
	tree.InsertSlice([]int{7, 42})
	fmt.Println(tree.StringFunc(func(v int) string { return fmt.Sprintf("%02d", v) }))
	// Output:
	// [07 42]
}

// Equal compares membership, never tree shape; a nil tree equals an
// empty one.  Both trees must use consistent comparison functions.
func ExampleBinaryTree_Equal() {
	a := binary_tree.NewBinaryTree[int]()
	a.InsertSlice([]int{1, 2, 3})
	b := binary_tree.NewBinaryTree[int]()
	b.InsertSlice([]int{3, 1, 2})
	fmt.Println(a.Equal(b), a.Equal(nil))
	// Output:
	// true false
}

// Clone is a deep copy — same comparison function, fresh nodes; mutating
// either tree never affects the other.
func ExampleBinaryTree_Clone() {
	tree := binary_tree.NewBinaryTree[string]()
	tree.InsertSlice([]string{"a", "b"})
	dup := tree.Clone()
	dup.Insert("c")
	fmt.Println(tree.Len(), dup.Len(), tree.Equal(dup))
	// Output:
	// 2 3 false
}

// ContainsSlice reports whether every item is present.
func ExampleBinaryTree_ContainsSlice() {
	tree := binary_tree.NewBinaryTree[int]()
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.ContainsSlice([]int{1, 3}), tree.ContainsSlice([]int{1, 9}))
	// Output:
	// true false
}

// ContainsAny reports whether at least one item is present.
func ExampleBinaryTree_ContainsAny() {
	tree := binary_tree.NewBinaryTree[int]()
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.ContainsAny([]int{9, 2}), tree.ContainsAny([]int{9, 8}))
	// Output:
	// true false
}

// InsertSlice inserts each item and reports whether any was new.
func ExampleBinaryTree_InsertSlice() {
	tree := binary_tree.NewBinaryTree[int]()
	fmt.Println(tree.InsertSlice([]int{1, 2}), tree.InsertSlice([]int{2, 3}), tree.Len())
	// Output:
	// true true 3
}

// InsertIter is the batch insert over a range-over-func iterator: any
// container's All() feeds it, with no interface between.
func ExampleBinaryTree_InsertIter() {
	src := binary_tree.NewBinaryTree[string]()
	src.InsertSlice([]string{"a", "b"})
	dst := binary_tree.NewBinaryTree[string]()
	dst.Insert("z")
	fmt.Println(dst.InsertIter(src.All()), dst.Len(), dst.Contains("b"))
	// Output:
	// true 3 true
}

// DeleteSlice deletes each item and reports whether any was present.
func ExampleBinaryTree_DeleteSlice() {
	tree := binary_tree.NewBinaryTree[int]()
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.DeleteSlice([]int{2, 9}), tree.DeleteSlice([]int{9}), tree.Len())
	// Output:
	// true false 2
}

// DeleteFunc bulk-deletes by predicate; the matches are collected first,
// then deleted, so the walk is never corrupted.
func ExampleBinaryTree_DeleteFunc() {
	tree := binary_tree.NewBinaryTree[int]()
	tree.InsertSlice([]int{1, 2, 3, 4, 5, 6})
	fmt.Println(tree.DeleteFunc(func(v int) bool { return v%2 == 0 }), tree.Len(), tree.ContainsSlice([]int{1, 3, 5}))
	// Output:
	// true 3 true
}
