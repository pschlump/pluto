package interval_tree_test

// Example tests for the set-convenience capabilities.  The menu is the
// map-shaped, reduced form (no batch inserters — a map-shaped batch
// insert would need key/value pairs; callers loop).

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"

	"github.com/pschlump/pluto/interval_tree"
)

func ExampleIntervalTree_Contains() {
	tree := interval_tree.NewIntervalTree[int, string]()
	tree.Insert(interval_tree.Interval[int]{Lo: 10, Hi: 20}, "meeting")
	fmt.Println(tree.Contains(interval_tree.Interval[int]{Lo: 10, Hi: 20}),
		tree.Contains(interval_tree.Interval[int]{Lo: 11, Hi: 19}))
	// Output: true false
}

func ExampleIntervalTree_Slice() {
	tree := interval_tree.NewIntervalTree[int, string]()
	tree.Insert(interval_tree.Interval[int]{Lo: 20, Hi: 25}, "c")
	tree.Insert(interval_tree.Interval[int]{Lo: 10, Hi: 12}, "a")
	tree.Insert(interval_tree.Interval[int]{Lo: 10, Hi: 11}, "b")
	fmt.Println(tree.Slice())
	// Output: [{10 11} {10 12} {20 25}]
}

func ExampleIntervalTree_String() {
	tree := interval_tree.NewIntervalTree[int, string]()
	tree.Insert(interval_tree.Interval[int]{Lo: 1, Hi: 5}, "a")
	tree.Insert(interval_tree.Interval[int]{Lo: 2, Hi: 7}, "b")
	fmt.Println(tree.String())
	// Output: [1..5 2..7]
}

func ExampleIntervalTree_StringFunc() {
	tree := interval_tree.NewIntervalTree[int, string]()
	tree.Insert(interval_tree.Interval[int]{Lo: 1, Hi: 5}, "a")
	fmt.Println(tree.StringFunc(func(iv interval_tree.Interval[int]) string {
		return fmt.Sprintf("[%d,%d]", iv.Lo, iv.Hi)
	}))
	// Output: [[1,5]]
}

func ExampleIntervalTree_Equal() {
	a := interval_tree.NewIntervalTree[int, string]()
	a.Insert(interval_tree.Interval[int]{Lo: 1, Hi: 2}, "one")
	b := interval_tree.NewIntervalTree[int, string]()
	b.Insert(interval_tree.Interval[int]{Lo: 1, Hi: 2}, "uno") // different value
	c := interval_tree.NewIntervalTree[int, string]()
	c.Insert(interval_tree.Interval[int]{Lo: 9, Hi: 9}, "nine")
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleIntervalTree_Clone() {
	tree := interval_tree.NewIntervalTree[int, string]()
	tree.Insert(interval_tree.Interval[int]{Lo: 1, Hi: 5}, "a")
	dup := tree.Clone()
	dup.Insert(interval_tree.Interval[int]{Lo: 8, Hi: 9}, "b")
	fmt.Println(tree.Slice(), dup.Slice())
	// Output: [{1 5}] [{1 5} {8 9}]
}

func ExampleIntervalTree_ContainsSlice() {
	tree := interval_tree.NewIntervalTree[int, string]()
	tree.Insert(interval_tree.Interval[int]{Lo: 1, Hi: 2}, "a")
	tree.Insert(interval_tree.Interval[int]{Lo: 3, Hi: 4}, "b")
	fmt.Println(tree.ContainsSlice([]interval_tree.Interval[int]{{Lo: 1, Hi: 2}, {Lo: 3, Hi: 4}}),
		tree.ContainsSlice([]interval_tree.Interval[int]{{Lo: 1, Hi: 2}, {Lo: 9, Hi: 9}}))
	// Output: true false
}

func ExampleIntervalTree_ContainsAny() {
	tree := interval_tree.NewIntervalTree[int, string]()
	tree.Insert(interval_tree.Interval[int]{Lo: 1, Hi: 2}, "a")
	tree.Insert(interval_tree.Interval[int]{Lo: 3, Hi: 4}, "b")
	fmt.Println(tree.ContainsAny([]interval_tree.Interval[int]{{Lo: 9, Hi: 9}, {Lo: 3, Hi: 4}}),
		tree.ContainsAny([]interval_tree.Interval[int]{{Lo: 9, Hi: 9}, {Lo: 8, Hi: 8}}))
	// Output: true false
}

func ExampleIntervalTree_DeleteSlice() {
	tree := interval_tree.NewIntervalTree[int, string]()
	tree.Insert(interval_tree.Interval[int]{Lo: 1, Hi: 2}, "a")
	tree.Insert(interval_tree.Interval[int]{Lo: 3, Hi: 4}, "b")
	tree.Insert(interval_tree.Interval[int]{Lo: 5, Hi: 6}, "c")
	fmt.Println(tree.DeleteSlice([]interval_tree.Interval[int]{{Lo: 3, Hi: 4}, {Lo: 9, Hi: 9}}), tree.Slice())
	fmt.Println(tree.DeleteSlice([]interval_tree.Interval[int]{{Lo: 9, Hi: 9}}))
	// Output:
	// true [{1 2} {5 6}]
	// false
}

func ExampleIntervalTree_DeleteFunc() {
	tree := interval_tree.NewIntervalTree[int, string]()
	tree.Insert(interval_tree.Interval[int]{Lo: 1, Hi: 5}, "keep")
	tree.Insert(interval_tree.Interval[int]{Lo: 2, Hi: 6}, "drop")
	tree.Insert(interval_tree.Interval[int]{Lo: 8, Hi: 9}, "keep")
	fmt.Println(tree.DeleteFunc(func(_ interval_tree.Interval[int], val string) bool { return val == "drop" }), tree.Slice())
	// Output: true [{1 5} {8 9}]
}
