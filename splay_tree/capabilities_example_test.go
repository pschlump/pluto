package splay_tree_test

// Example tests for the set-convenience capabilities.  There is no _ts
// twin of this package (see the package documentation).

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"slices"

	"github.com/pschlump/pluto/splay_tree"
)

func ExampleSplayTree_Contains() {
	tree := splay_tree.NewSplayTree[string]()
	tree.Insert("alpha")
	tree.Insert("beta")
	fmt.Println(tree.Contains("alpha"), tree.Contains("gamma"))
	// Output: true false
}

func ExampleSplayTree_Slice() {
	tree := splay_tree.NewSplayTree[string]()
	tree.Insert("pear")
	tree.Insert("apple")
	tree.Insert("fig")
	fmt.Println(tree.Slice())
	// Output: [apple fig pear]
}

func ExampleSplayTree_String() {
	tree := splay_tree.NewSplayTree[int]()
	tree.Insert(5)
	tree.Insert(2)
	tree.Insert(8)
	fmt.Println(tree.String())
	// Output: [2 5 8]
}

func ExampleSplayTree_StringFunc() {
	tree := splay_tree.NewSplayTree[int]()
	tree.Insert(5)
	tree.Insert(8)
	fmt.Println(tree.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleSplayTree_Equal() {
	a := splay_tree.NewSplayTree[string]()
	a.Insert("apple")
	a.Insert("fig")
	b := splay_tree.NewSplayTree[string]()
	b.Insert("fig")
	b.Insert("apple")
	c := splay_tree.NewSplayTree[string]()
	c.Insert("fig")
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleSplayTree_Clone() {
	tree := splay_tree.NewSplayTree[string]()
	tree.Insert("apple")
	dup := tree.Clone()
	dup.Insert("pear")
	fmt.Println(tree.Slice(), dup.Slice())
	// Output: [apple] [apple pear]
}

func ExampleSplayTree_ContainsSlice() {
	tree := splay_tree.NewSplayTree[int]()
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.ContainsSlice([]int{1, 3}), tree.ContainsSlice([]int{1, 9}))
	// Output: true false
}

func ExampleSplayTree_ContainsAny() {
	tree := splay_tree.NewSplayTree[int]()
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.ContainsAny([]int{9, 2}), tree.ContainsAny([]int{9, 8}))
	// Output: true false
}

func ExampleSplayTree_InsertSlice() {
	tree := splay_tree.NewSplayTree[int]()
	fmt.Println(tree.InsertSlice([]int{3, 1, 2}), tree.Slice())
	fmt.Println(tree.InsertSlice([]int{2, 3}))
	// Output:
	// true [1 2 3]
	// false
}

func ExampleSplayTree_InsertIter() {
	other := splay_tree.NewSplayTree[string]()
	other.Insert("fig")
	other.Insert("apple")
	tree := splay_tree.NewSplayTree[string]()
	// All is an iter.Seq2 here, so adapt it with Slice + slices.Values (or
	// any iter.Seq source).
	fmt.Println(tree.InsertIter(slices.Values(other.Slice())), tree.Slice())
	// Output: true [apple fig]
}

func ExampleSplayTree_DeleteSlice() {
	tree := splay_tree.NewSplayTree[int]()
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.DeleteSlice([]int{2, 9}), tree.Slice())
	fmt.Println(tree.DeleteSlice([]int{9}))
	// Output:
	// true [1 3]
	// false
}

func ExampleSplayTree_DeleteFunc() {
	tree := splay_tree.NewSplayTree[int]()
	tree.InsertSlice([]int{1, 2, 3, 4, 5, 6})
	fmt.Println(tree.DeleteFunc(func(v int) bool { return v%2 == 0 }), tree.Slice())
	// Output: true [1 3 5]
}
