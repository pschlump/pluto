package cuckoo_ts_test

// Checked examples for the set-convenience capabilities (capabilities.go).
// Mirrored from cuckoo's example file.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"sort"

	"github.com/pschlump/pluto/cuckoo_ts"
)

// Contains is the membership predicate — the found flag of Search.
func ExampleHashTab_Contains() {
	ht := cuckoo_ts.NewHashTab[string](8, 0, 0)
	ht.Insert("alpha")
	fmt.Println(ht.Contains("alpha"), ht.Contains("beta"))
	// Output:
	// true false
}

// Slice materializes the elements into a fresh slice.  The order is
// bucket order (seeded randomly per process), so sort when a fixed
// order matters.
func ExampleHashTab_Slice() {
	ht := cuckoo_ts.NewHashTab[int](8, 0, 0)
	ht.InsertSlice([]int{42, 7, 13})
	s := ht.Slice()
	sort.Ints(s)
	fmt.Println(s)
	// Output:
	// [7 13 42]
}

// String renders the elements deterministically: each formatted with
// fmt.Sprint, the strings sorted — the random bucket order never shows,
// so two tables with the same elements always print the same thing.
func ExampleHashTab_String() {
	ht := cuckoo_ts.NewHashTab[string](8, 0, 0)
	ht.InsertSlice([]string{"pear", "apple", "fig"})
	fmt.Println(ht.String())
	// Output:
	// [apple fig pear]
}

// StringFunc is String with a caller supplied per-element format.
func ExampleHashTab_StringFunc() {
	ht := cuckoo_ts.NewHashTab[int](8, 0, 0)
	ht.InsertSlice([]int{7, 42})
	fmt.Println(ht.StringFunc(func(v int) string { return fmt.Sprintf("%02d", v) }))
	// Output:
	// [07 42]
}

// Equal compares membership, never iteration order; a nil table equals
// an empty one.  Both tables must use consistent equality/hash
// functions.
func ExampleHashTab_Equal() {
	a := cuckoo_ts.NewHashTab[int](8, 0, 0)
	a.InsertSlice([]int{1, 2, 3})
	b := cuckoo_ts.NewHashTab[int](5, 0, 0)
	b.InsertSlice([]int{3, 1, 2})
	fmt.Println(a.Equal(b), a.Equal(nil))
	// Output:
	// true false
}

// Clone is a deep copy — same bucket count, same equality/hash
// functions, fresh nodes; mutating either table never affects the other.
func ExampleHashTab_Clone() {
	ht := cuckoo_ts.NewHashTab[string](8, 0, 0)
	ht.InsertSlice([]string{"a", "b"})
	dup := ht.Clone()
	dup.Insert("c")
	fmt.Println(ht.Len(), dup.Len(), ht.Equal(dup))
	// Output:
	// 2 3 false
}

// ContainsSlice reports whether every item is present.
func ExampleHashTab_ContainsSlice() {
	ht := cuckoo_ts.NewHashTab[int](8, 0, 0)
	ht.InsertSlice([]int{1, 2, 3})
	fmt.Println(ht.ContainsSlice([]int{1, 3}), ht.ContainsSlice([]int{1, 9}))
	// Output:
	// true false
}

// ContainsAny reports whether at least one item is present.
func ExampleHashTab_ContainsAny() {
	ht := cuckoo_ts.NewHashTab[int](8, 0, 0)
	ht.InsertSlice([]int{1, 2, 3})
	fmt.Println(ht.ContainsAny([]int{9, 2}), ht.ContainsAny([]int{9, 8}))
	// Output:
	// true false
}

// InsertSlice inserts each item and reports whether any was new.
func ExampleHashTab_InsertSlice() {
	ht := cuckoo_ts.NewHashTab[int](8, 0, 0)
	fmt.Println(ht.InsertSlice([]int{1, 2}), ht.InsertSlice([]int{2, 3}), ht.Len())
	// Output:
	// true true 3
}

// InsertIter is the batch insert over a range-over-func iterator: any
// container's Values() feeds it, with no interface between.
func ExampleHashTab_InsertIter() {
	src := cuckoo_ts.NewHashTab[string](8, 0, 0)
	src.InsertSlice([]string{"a", "b"})
	dst := cuckoo_ts.NewHashTab[string](8, 0, 0)
	dst.Insert("z")
	fmt.Println(dst.InsertIter(src.Values()), dst.Len(), dst.Contains("b"))
	// Output:
	// true 3 true
}

// DeleteSlice deletes each item and reports whether any was present.
func ExampleHashTab_DeleteSlice() {
	ht := cuckoo_ts.NewHashTab[int](8, 0, 0)
	ht.InsertSlice([]int{1, 2, 3})
	fmt.Println(ht.DeleteSlice([]int{2, 9}), ht.DeleteSlice([]int{9}), ht.Len())
	// Output:
	// true false 2
}

// DeleteFunc bulk-deletes by predicate; the matches are collected first,
// then deleted, so the walk is never corrupted.
func ExampleHashTab_DeleteFunc() {
	ht := cuckoo_ts.NewHashTab[int](8, 0, 0)
	ht.InsertSlice([]int{1, 2, 3, 4, 5, 6})
	fmt.Println(ht.DeleteFunc(func(v int) bool { return v%2 == 0 }), ht.Len(), ht.ContainsSlice([]int{1, 3, 5}))
	// Output:
	// true 3 true
}
