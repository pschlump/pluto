/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package avl_tree_ts_test

import (
	"fmt"
	"sync"

	"github.com/pschlump/pluto/avl_tree_ts"
)

// Writers from many goroutines share one tree; readers see a consistent
// result once the writers finish.
func Example() {
	tree := avl_tree_ts.NewAvlTree[int]()

	var wg sync.WaitGroup
	for w := range 4 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range 25 {
				tree.Insert(w*100 + i)
			}
		}(w)
	}
	wg.Wait()

	fmt.Println(tree.Len(), tree.IsEmpty())

	min, _ := tree.FindMin()
	max, _ := tree.FindMax()
	median, _ := tree.Index(tree.Len() / 2)
	fmt.Println(min, median, max)
	// Output:
	// 100 false
	// 0 200 324
}

// Sorted input into a plain BST builds a degenerate chain; the AVL tree
// rebalances as it goes, so the depth stays logarithmic.
func ExampleNewAvlTree() {
	tree := avl_tree_ts.NewAvlTree[int]()
	for i := range 7 {
		tree.Insert(i) // 0..6 in ascending order
	}
	fmt.Println(tree.Depth(), tree.Length())
	// Output:
	// 3 7
}

// Structs ordered by a field — the element type implements no interface;
// ordering is a plain function, and Compare builds the field comparison.
func ExampleNewAvlTreeFunc() {
	type Employee struct {
		ID   int
		Name string
	}

	byID := avl_tree_ts.NewAvlTreeFunc(func(a, b Employee) int {
		return avl_tree_ts.Compare(a.ID, b.ID)
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
	fmt.Println(avl_tree_ts.Compare("apple", "banana"))
	fmt.Println(avl_tree_ts.Compare(3.5, 1.5))
	fmt.Println(avl_tree_ts.Compare(7, 7))
	// Output:
	// -1
	// 1
	// 0
}

// All iterates in ascending order; Backward iterates in descending order.
// Both operate on a snapshot taken when they are called.
func ExampleAvlTree_All() {
	tree := avl_tree_ts.NewAvlTree[string]()
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

// Because All iterates a snapshot, mutating the tree from inside the loop
// is safe — the loop still visits every element captured at the start.
func ExampleAvlTree_All_snapshot() {
	tree := avl_tree_ts.NewAvlTree[string]()
	for _, s := range []string{"pear", "fig", "apple"} {
		tree.Insert(s)
	}

	var visited []string
	for v := range tree.All() {
		visited = append(visited, v)
		tree.Delete(v)
	}
	fmt.Println(visited, tree.Len())
	// Output:
	// [apple fig pear] 0
}

// The old-style iterator can be paused and resumed — here only the first
// two elements of the in-order traversal are consumed.
func ExampleAvlTree_Front() {
	tree := avl_tree_ts.NewAvlTree[int]()
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
// Returning false stops the walk.  Note that the callback must not call
// methods on the same tree — the walk holds the tree's read lock.
func ExampleAvlTree_WalkInOrder() {
	tree := avl_tree_ts.NewAvlTree[string]()
	for _, s := range []string{"mango", "apple", "peach", "banana"} {
		tree.Insert(s)
	}

	var visited []string
	tree.WalkInOrder(func(pos, depth int, data string) bool {
		visited = append(visited, fmt.Sprintf("%d:%d:%s", pos, depth, data))
		return len(visited) < 3 // false stops the walk
	})
	for _, v := range visited {
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output:
	// 0:1:apple 1:2:banana 2:0:mango
}

// Index answers rank queries on the in-order sequence: Index(0) is the
// minimum, Index(Len()-1) the maximum, and Index(Len()/2) the median.
func ExampleAvlTree_Index() {
	tree := avl_tree_ts.NewAvlTree[int]()
	for _, v := range []int{50, 20, 80, 10, 30, 70, 90} {
		tree.Insert(v)
	}

	median, _ := tree.Index(tree.Len() / 2)
	fmt.Println(median, tree.Depth())
	// Output:
	// 50 3
}

// DeleteAtHead removes the smallest element — draining the tree this way
// enumerates the elements in ascending order.
func ExampleAvlTree_DeleteAtHead() {
	tree := avl_tree_ts.NewAvlTree[int]()
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
func ExampleAvlTree_Truncate() {
	tree := avl_tree_ts.NewAvlTreeFunc(func(a, b int) int {
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

// Union builds the set union of two trees.  A zero-value destination
// works: it adopts the first operand's comparison function.  The sources
// are snapshotted under their own read locks, so this is race-free even
// while other goroutines read a and b.
func ExampleAvlTree_Union() {
	a := avl_tree_ts.NewAvlTree[int]()
	for _, v := range []int{1, 2, 3} {
		a.Insert(v)
	}
	b := avl_tree_ts.NewAvlTree[int]()
	for _, v := range []int{2, 3, 4} {
		b.Insert(v)
	}

	var dst avl_tree_ts.AvlTree[int] // zero value: adopts a's comparison function
	dst.Union(a, b)

	var got []int
	for v := range dst.All() {
		got = append(got, v)
	}
	fmt.Println(got)
	// Output:
	// [1 2 3 4]
}

// Minus builds the set difference of two trees (yy - zz).
func ExampleAvlTree_Minus() {
	a := avl_tree_ts.NewAvlTree[int]()
	for _, v := range []int{1, 2, 3} {
		a.Insert(v)
	}
	b := avl_tree_ts.NewAvlTree[int]()
	for _, v := range []int{2, 3, 4} {
		b.Insert(v)
	}

	var dst avl_tree_ts.AvlTree[int] // zero value: adopts a's comparison function
	dst.Minus(a, b)

	var got []int
	for v := range dst.All() {
		got = append(got, v)
	}
	fmt.Println(got)
	// Output:
	// [1]
}
