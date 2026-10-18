package skip_list_ts_test

// Example tests for the set-convenience capabilities.  The same examples
// (package name aside) run against the plain skip_list — the twins must
// behave identically.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"

	"github.com/pschlump/pluto/skip_list_ts"
)

func ExampleSkipList_Contains() {
	list := skip_list_ts.NewSkipList[string]()
	list.Insert("alpha")
	list.Insert("beta")
	fmt.Println(list.Contains("alpha"), list.Contains("gamma"))
	// Output: true false
}

func ExampleSkipList_Slice() {
	list := skip_list_ts.NewSkipList[string]()
	list.Insert("pear")
	list.Insert("apple")
	list.Insert("fig")
	fmt.Println(list.Slice())
	// Output: [apple fig pear]
}

func ExampleSkipList_String() {
	list := skip_list_ts.NewSkipList[int]()
	list.Insert(5)
	list.Insert(2)
	list.Insert(8)
	fmt.Println(list.String())
	// Output: [2 5 8]
}

func ExampleSkipList_StringFunc() {
	list := skip_list_ts.NewSkipList[int]()
	list.Insert(5)
	list.Insert(8)
	fmt.Println(list.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleSkipList_Equal() {
	a := skip_list_ts.NewSkipList[string]()
	a.Insert("apple")
	a.Insert("fig")
	b := skip_list_ts.NewSkipList[string]()
	b.Insert("fig")
	b.Insert("apple")
	c := skip_list_ts.NewSkipList[string]()
	c.Insert("fig")
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleSkipList_Clone() {
	list := skip_list_ts.NewSkipList[string]()
	list.Insert("apple")
	dup := list.Clone()
	dup.Insert("pear")
	fmt.Println(list.Slice(), dup.Slice())
	// Output: [apple] [apple pear]
}

func ExampleSkipList_ContainsSlice() {
	list := skip_list_ts.NewSkipList[int]()
	list.InsertSlice([]int{1, 2, 3})
	fmt.Println(list.ContainsSlice([]int{1, 3}), list.ContainsSlice([]int{1, 9}))
	// Output: true false
}

func ExampleSkipList_ContainsAny() {
	list := skip_list_ts.NewSkipList[int]()
	list.InsertSlice([]int{1, 2, 3})
	fmt.Println(list.ContainsAny([]int{9, 2}), list.ContainsAny([]int{9, 8}))
	// Output: true false
}

func ExampleSkipList_InsertSlice() {
	list := skip_list_ts.NewSkipList[int]()
	fmt.Println(list.InsertSlice([]int{3, 1, 2}), list.Slice())
	fmt.Println(list.InsertSlice([]int{2, 3}))
	// Output:
	// true [1 2 3]
	// false
}

func ExampleSkipList_InsertIter() {
	other := skip_list_ts.NewSkipList[string]()
	other.Insert("fig")
	other.Insert("apple")
	list := skip_list_ts.NewSkipList[string]()
	// All is an iter.Seq snapshot here, so it feeds InsertIter directly.
	fmt.Println(list.InsertIter(other.All()), list.Slice())
	// Output: true [apple fig]
}

func ExampleSkipList_DeleteSlice() {
	list := skip_list_ts.NewSkipList[int]()
	list.InsertSlice([]int{1, 2, 3})
	fmt.Println(list.DeleteSlice([]int{2, 9}), list.Slice())
	fmt.Println(list.DeleteSlice([]int{9}))
	// Output:
	// true [1 3]
	// false
}

func ExampleSkipList_DeleteFunc() {
	list := skip_list_ts.NewSkipList[int]()
	list.InsertSlice([]int{1, 2, 3, 4, 5, 6})
	fmt.Println(list.DeleteFunc(func(v int) bool { return v%2 == 0 }), list.Slice())
	// Output: true [1 3 5]
}
