package rb_tree_ts_test

// Example tests for the set-convenience capabilities.  The same examples
// (package name aside) run against the plain rb_tree — the twins must
// behave identically.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"

	"github.com/pschlump/pluto/rb_tree_ts"
)

func ExampleRbTree_Contains() {
	tree := rb_tree_ts.NewRbTree[string]()
	tree.Insert("alpha")
	tree.Insert("beta")
	fmt.Println(tree.Contains("alpha"), tree.Contains("gamma"))
	// Output: true false
}

func ExampleRbTree_Slice() {
	tree := rb_tree_ts.NewRbTree[string]()
	tree.Insert("pear")
	tree.Insert("apple")
	tree.Insert("fig")
	fmt.Println(tree.Slice())
	// Output: [apple fig pear]
}

func ExampleRbTree_String() {
	tree := rb_tree_ts.NewRbTree[int]()
	tree.Insert(5)
	tree.Insert(2)
	tree.Insert(8)
	fmt.Println(tree.String())
	// Output: [2 5 8]
}

func ExampleRbTree_StringFunc() {
	tree := rb_tree_ts.NewRbTree[int]()
	tree.Insert(5)
	tree.Insert(8)
	fmt.Println(tree.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleRbTree_Equal() {
	a := rb_tree_ts.NewRbTree[string]()
	a.Insert("apple")
	a.Insert("fig")
	b := rb_tree_ts.NewRbTree[string]()
	b.Insert("fig")
	b.Insert("apple")
	c := rb_tree_ts.NewRbTree[string]()
	c.Insert("fig")
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleRbTree_Clone() {
	tree := rb_tree_ts.NewRbTree[string]()
	tree.Insert("apple")
	dup := tree.Clone()
	dup.Insert("pear")
	fmt.Println(tree.Slice(), dup.Slice())
	// Output: [apple] [apple pear]
}

func ExampleRbTree_ContainsSlice() {
	tree := rb_tree_ts.NewRbTree[int]()
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.ContainsSlice([]int{1, 3}), tree.ContainsSlice([]int{1, 9}))
	// Output: true false
}

func ExampleRbTree_ContainsAny() {
	tree := rb_tree_ts.NewRbTree[int]()
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.ContainsAny([]int{9, 2}), tree.ContainsAny([]int{9, 8}))
	// Output: true false
}

func ExampleRbTree_InsertSlice() {
	tree := rb_tree_ts.NewRbTree[int]()
	fmt.Println(tree.InsertSlice([]int{3, 1, 2}), tree.Slice())
	fmt.Println(tree.InsertSlice([]int{2, 3}))
	// Output:
	// true [1 2 3]
	// false
}

func ExampleRbTree_InsertIter() {
	other := rb_tree_ts.NewRbTree[string]()
	other.Insert("fig")
	other.Insert("apple")
	tree := rb_tree_ts.NewRbTree[string]()
	fmt.Println(tree.InsertIter(other.All()), tree.Slice())
	// Output: true [apple fig]
}

func ExampleRbTree_DeleteSlice() {
	tree := rb_tree_ts.NewRbTree[int]()
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.DeleteSlice([]int{2, 9}), tree.Slice())
	fmt.Println(tree.DeleteSlice([]int{9}))
	// Output:
	// true [1 3]
	// false
}

func ExampleRbTree_DeleteFunc() {
	tree := rb_tree_ts.NewRbTree[int]()
	tree.InsertSlice([]int{1, 2, 3, 4, 5, 6})
	fmt.Println(tree.DeleteFunc(func(v int) bool { return v%2 == 0 }), tree.Slice())
	// Output: true [1 3 5]
}
