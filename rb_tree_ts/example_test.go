/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package rb_tree_ts_test

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/pschlump/pluto/rb_tree_ts"
)

// Writers from many goroutines share one tree; readers see a consistent
// result once the writers finish.
func Example() {
	tree := rb_tree_ts.NewRbTree[int]()

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
	fmt.Println(min, max)
	// Output:
	// 100 false
	// 0 324
}

// Sorted input into a plain BST builds a degenerate chain; the red-black
// tree recolors and rotates as it goes, so the depth stays logarithmic.
func ExampleNewRbTree() {
	tree := rb_tree_ts.NewRbTree[int]()
	for i := range 7 {
		tree.Insert(i) // 0..6 in ascending order
	}
	fmt.Println(tree.Depth(), tree.Length())
	// Output:
	// 4 7
}

// Structs ordered by a field — the element type implements no interface;
// ordering is a plain function, and Compare builds the field comparison.
func ExampleNewRbTreeFunc() {
	type Employee struct {
		ID   int
		Name string
	}

	byID := rb_tree_ts.NewRbTreeFunc(func(a, b Employee) int {
		return rb_tree_ts.Compare(a.ID, b.ID)
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
	fmt.Println(rb_tree_ts.Compare("apple", "banana"))
	fmt.Println(rb_tree_ts.Compare(3.5, 1.5))
	fmt.Println(rb_tree_ts.Compare(7, 7))
	// Output:
	// -1
	// 1
	// 0
}

// All iterates in ascending order; Backward iterates in descending order.
// Both operate on a snapshot taken when they are called.
func ExampleRbTree_All() {
	tree := rb_tree_ts.NewRbTree[string]()
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
func ExampleRbTree_All_snapshot() {
	tree := rb_tree_ts.NewRbTree[string]()
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

// DeleteAtHead removes the smallest element — draining the tree this way
// enumerates the elements in ascending order.
func ExampleRbTree_DeleteAtHead() {
	tree := rb_tree_ts.NewRbTree[int]()
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
	tree := rb_tree_ts.NewRbTreeFunc(func(a, b int) int {
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

// MarshalJSON encodes the tree as a JSON array of its elements in
// in-order (sorted) sequence, whatever the insertion order was.
func ExampleRbTree_MarshalJSON() {
	tree := rb_tree_ts.NewRbTree[int]()
	tree.Insert(3)
	tree.Insert(1)
	tree.Insert(2)

	b, err := json.Marshal(tree)
	fmt.Println(string(b), err)
	// Output:
	// [1,2,3] <nil>
}

// UnmarshalJSON replaces the contents of the tree from a JSON array;
// the elements land in in-order (sorted) sequence.
func ExampleRbTree_UnmarshalJSON() {
	tree := rb_tree_ts.NewRbTree[string]()
	if err := json.Unmarshal([]byte(`["pear","apple","fig"]`), tree); err != nil {
		fmt.Println("error:", err)
		return
	}
	i := 0
	for v := range tree.All() {
		fmt.Println(i, v)
		i++
	}
	// Output:
	// 0 apple
	// 1 fig
	// 2 pear
}
