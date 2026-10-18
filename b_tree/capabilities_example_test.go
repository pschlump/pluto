package b_tree_test

// Example tests for the set-convenience capabilities.  There is no _ts
// twin of this package (see the package documentation).

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"slices"

	"github.com/pschlump/pluto/b_tree"
)

func ExampleBTree_Contains() {
	tree := b_tree.NewBTree[string](4)
	tree.Insert("alpha")
	tree.Insert("beta")
	fmt.Println(tree.Contains("alpha"), tree.Contains("gamma"))
	// Output: true false
}

func ExampleBTree_Slice() {
	tree := b_tree.NewBTree[string](4)
	tree.Insert("pear")
	tree.Insert("apple")
	tree.Insert("fig")
	fmt.Println(tree.Slice())
	// Output: [apple fig pear]
}

func ExampleBTree_String() {
	tree := b_tree.NewBTree[int](4)
	tree.Insert(5)
	tree.Insert(2)
	tree.Insert(8)
	fmt.Println(tree.String())
	// Output: [2 5 8]
}

func ExampleBTree_StringFunc() {
	tree := b_tree.NewBTree[int](4)
	tree.Insert(5)
	tree.Insert(8)
	fmt.Println(tree.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleBTree_Equal() {
	a := b_tree.NewBTree[string](4)
	a.Insert("apple")
	a.Insert("fig")
	b := b_tree.NewBTree[string](7) // different order, same elements
	b.Insert("fig")
	b.Insert("apple")
	c := b_tree.NewBTree[string](4)
	c.Insert("fig")
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleBTree_Clone() {
	tree := b_tree.NewBTree[string](4)
	tree.Insert("apple")
	dup := tree.Clone()
	dup.Insert("pear")
	fmt.Println(tree.Slice(), dup.Slice())
	// Output: [apple] [apple pear]
}

func ExampleBTree_ContainsSlice() {
	tree := b_tree.NewBTree[int](4)
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.ContainsSlice([]int{1, 3}), tree.ContainsSlice([]int{1, 9}))
	// Output: true false
}

func ExampleBTree_ContainsAny() {
	tree := b_tree.NewBTree[int](4)
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.ContainsAny([]int{9, 2}), tree.ContainsAny([]int{9, 8}))
	// Output: true false
}

func ExampleBTree_InsertSlice() {
	tree := b_tree.NewBTree[int](4)
	fmt.Println(tree.InsertSlice([]int{3, 1, 2}), tree.Slice())
	fmt.Println(tree.InsertSlice([]int{2, 3}))
	// Output:
	// true [1 2 3]
	// false
}

func ExampleBTree_InsertIter() {
	other := b_tree.NewBTree[string](5)
	other.Insert("fig")
	other.Insert("apple")
	tree := b_tree.NewBTree[string](4)
	// All is an iter.Seq2 here, so adapt it with Slice + slices.Values (or
	// any iter.Seq source).
	fmt.Println(tree.InsertIter(slices.Values(other.Slice())), tree.Slice())
	// Output: true [apple fig]
}

func ExampleBTree_DeleteSlice() {
	tree := b_tree.NewBTree[int](4)
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.DeleteSlice([]int{2, 9}), tree.Slice())
	fmt.Println(tree.DeleteSlice([]int{9}))
	// Output:
	// true [1 3]
	// false
}

func ExampleBTree_DeleteFunc() {
	tree := b_tree.NewBTree[int](4)
	tree.InsertSlice([]int{1, 2, 3, 4, 5, 6})
	fmt.Println(tree.DeleteFunc(func(v int) bool { return v%2 == 0 }), tree.Slice())
	// Output: true [1 3 5]
}
