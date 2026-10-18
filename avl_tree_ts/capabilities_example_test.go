package avl_tree_ts_test

// Example tests for the set-convenience capabilities and the phase-2 avl
// riders (SymmetricDifference, Floor, Ceil).  The same examples (package
// name aside) run against the plain avl_tree — the twins must behave
// identically.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"

	"github.com/pschlump/pluto/avl_tree_ts"
)

func ExampleAvlTree_Contains() {
	tree := avl_tree_ts.NewAvlTree[string]()
	tree.Insert("alpha")
	tree.Insert("beta")
	fmt.Println(tree.Contains("alpha"), tree.Contains("gamma"))
	// Output: true false
}

func ExampleAvlTree_Slice() {
	tree := avl_tree_ts.NewAvlTree[string]()
	tree.Insert("pear")
	tree.Insert("apple")
	tree.Insert("fig")
	fmt.Println(tree.Slice())
	// Output: [apple fig pear]
}

func ExampleAvlTree_String() {
	tree := avl_tree_ts.NewAvlTree[int]()
	tree.Insert(5)
	tree.Insert(2)
	tree.Insert(8)
	fmt.Println(tree.String())
	// Output: [2 5 8]
}

func ExampleAvlTree_StringFunc() {
	tree := avl_tree_ts.NewAvlTree[int]()
	tree.Insert(5)
	tree.Insert(8)
	fmt.Println(tree.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleAvlTree_Equal() {
	a := avl_tree_ts.NewAvlTree[string]()
	a.Insert("apple")
	a.Insert("fig")
	b := avl_tree_ts.NewAvlTree[string]()
	b.Insert("fig")
	b.Insert("apple")
	c := avl_tree_ts.NewAvlTree[string]()
	c.Insert("fig")
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleAvlTree_Clone() {
	tree := avl_tree_ts.NewAvlTree[string]()
	tree.Insert("apple")
	dup := tree.Clone()
	dup.Insert("pear")
	fmt.Println(tree.Slice(), dup.Slice())
	// Output: [apple] [apple pear]
}

func ExampleAvlTree_ContainsSlice() {
	tree := avl_tree_ts.NewAvlTree[int]()
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.ContainsSlice([]int{1, 3}), tree.ContainsSlice([]int{1, 9}))
	// Output: true false
}

func ExampleAvlTree_ContainsAny() {
	tree := avl_tree_ts.NewAvlTree[int]()
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.ContainsAny([]int{9, 2}), tree.ContainsAny([]int{9, 8}))
	// Output: true false
}

func ExampleAvlTree_InsertSlice() {
	tree := avl_tree_ts.NewAvlTree[int]()
	fmt.Println(tree.InsertSlice([]int{3, 1, 2}), tree.Slice())
	fmt.Println(tree.InsertSlice([]int{2, 3}))
	// Output:
	// true [1 2 3]
	// false
}

func ExampleAvlTree_InsertIter() {
	other := avl_tree_ts.NewAvlTree[string]()
	other.Insert("fig")
	other.Insert("apple")
	tree := avl_tree_ts.NewAvlTree[string]()
	fmt.Println(tree.InsertIter(other.All()), tree.Slice())
	// Output: true [apple fig]
}

func ExampleAvlTree_DeleteSlice() {
	tree := avl_tree_ts.NewAvlTree[int]()
	tree.InsertSlice([]int{1, 2, 3})
	fmt.Println(tree.DeleteSlice([]int{2, 9}), tree.Slice())
	fmt.Println(tree.DeleteSlice([]int{9}))
	// Output:
	// true [1 3]
	// false
}

func ExampleAvlTree_DeleteFunc() {
	tree := avl_tree_ts.NewAvlTree[int]()
	tree.InsertSlice([]int{1, 2, 3, 4, 5, 6})
	fmt.Println(tree.DeleteFunc(func(v int) bool { return v%2 == 0 }), tree.Slice())
	// Output: true [1 3 5]
}

func ExampleAvlTree_SymmetricDifference() {
	a := avl_tree_ts.NewAvlTree[int]()
	a.InsertSlice([]int{1, 2, 3, 4})
	b := avl_tree_ts.NewAvlTree[int]()
	b.InsertSlice([]int{3, 4, 5, 6})
	dst := avl_tree_ts.NewAvlTree[int]()
	dst.SymmetricDifference(a, b)
	fmt.Println(dst.Slice())
	// Output: [1 2 5 6]
}

func ExampleAvlTree_Floor() {
	tree := avl_tree_ts.NewAvlTree[int]()
	tree.InsertSlice([]int{10, 20, 30})
	if v, found := tree.Floor(25); found {
		fmt.Println(v)
	}
	if _, found := tree.Floor(5); !found {
		fmt.Println("no floor below 5")
	}
	// Output:
	// 20
	// no floor below 5
}

func ExampleAvlTree_Ceil() {
	tree := avl_tree_ts.NewAvlTree[int]()
	tree.InsertSlice([]int{10, 20, 30})
	if v, found := tree.Ceil(25); found {
		fmt.Println(v)
	}
	if _, found := tree.Ceil(35); !found {
		fmt.Println("no ceil above 35")
	}
	// Output:
	// 30
	// no ceil above 35
}
