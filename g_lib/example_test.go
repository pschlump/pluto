/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.  Map iteration
// order is nondeterministic, so every example that prints map keys
// sorts them first.
package g_lib_test

import (
	"fmt"

	"github.com/pschlump/pluto/g_lib"
)

func ExampleMin() {
	fmt.Println(g_lib.Min(3, 4), g_lib.Min("pear", "apple"))
	// Output:
	// 3 apple
}

func ExampleMax() {
	fmt.Println(g_lib.Max(3, 4), g_lib.Max(2.5, 2.4))
	// Output:
	// 4 2.5
}

func ExampleMinArray() {
	fmt.Println(g_lib.MinArray([]int{5, 2, 9, 2}), g_lib.MinArray([]int{}))
	// Output:
	// 2 0
}

func ExampleMaxArray() {
	fmt.Println(g_lib.MaxArray([]string{"pear", "apple", "zebra"}))
	// Output:
	// zebra
}

func ExampleAbs() {
	fmt.Println(g_lib.Abs(-12), g_lib.Abs(3.5), g_lib.Abs(0))
	// Output:
	// 12 3.5 0
}

func ExamplePow() {
	fmt.Println(g_lib.Pow(2, 10), g_lib.Pow(3, 0), g_lib.Pow(1.5, 2), g_lib.Pow(5, -1))
	// Output:
	// 1024 1 2.25 1
}

// IfTrue is a ternary-style selector: the Go analog of cond ? a : b.
func ExampleIfTrue() {
	name := "pluto"
	fmt.Println(g_lib.IfTrue(name == "pluto", "right repo", "wrong repo"))
	// Output:
	// right repo
}

func ExampleInArray() {
	fmt.Println(g_lib.InArray(42, []int{1, 42, 7}), g_lib.InArray(43, []int{1, 42, 7}))
	// Output:
	// true false
}

func ExampleLocationInArray() {
	fmt.Println(g_lib.LocationInArray("b", []string{"a", "b", "c", "b"}))
	fmt.Println(g_lib.LocationInArray("z", []string{"a", "b", "c"}))
	// Output:
	// 1
	// -1
}

func ExampleKeysForStringMap() {
	m := map[string]int{"b": 2, "a": 1}
	keys := g_lib.KeysForStringMap(m) // unspecified order
	g_lib.SortSlice(keys)
	fmt.Println(keys)
	// Output:
	// [a b]
}

func ExampleSortedKeysForStringMap() {
	m := map[string]int{"z": 26, "a": 1, "m": 13}
	fmt.Println(g_lib.SortedKeysForStringMap(m))
	// Output:
	// [a m z]
}

func ExampleGetMapKeys() {
	m := map[int]string{3: "c", 1: "a", 2: "b"}
	keys := g_lib.GetMapKeys(m) // unspecified order
	g_lib.SortSlice(keys)
	fmt.Println(keys)
	// Output:
	// [1 2 3]
}

func ExampleSortSlice() {
	s := []string{"pear", "apple", "zebra", "fig"}
	g_lib.SortSlice(s)
	fmt.Println(s)
	// Output:
	// [apple fig pear zebra]
}

func ExampleEqualSlice() {
	fmt.Println(g_lib.EqualSlice([]int{1, 2, 3}, []int{1, 2, 3}))
	fmt.Println(g_lib.EqualSlice([]int{1, 2, 3}, []int{1, 2}))
	// Output:
	// true
	// false
}

func ExampleRemoveAt() {
	s := []int{1, 2, 3, 4}
	fmt.Println(g_lib.RemoveAt(s, 1), s) // input is never modified
	fmt.Println(g_lib.RemoveAt(s, 9))    // out of range: unchanged
	// Output:
	// [1 3 4] [1 2 3 4]
	// [1 2 3 4]
}

// Remove compares with reflect.DeepEqual, so it works for element
// types that are not comparable.
func ExampleRemove() {
	haystack := [][]int{{1, 2}, {3}, {1, 2}}
	fmt.Println(g_lib.Remove(haystack, []int{1, 2}))
	// Output:
	// [[3]]
}

func ExampleRemoveComparable() {
	fmt.Println(g_lib.RemoveComparable([]string{"a", "b", "a", "c"}, "a"))
	// Output:
	// [b c]
}

func ExampleUnique() {
	fmt.Println(g_lib.Unique([]string{"a", "b", "a", "c", "b"}))
	// Output:
	// [a b c]
}

// ToBoolMap turns a slice into a set for O(1) membership tests.
func ExampleToBoolMap() {
	set := g_lib.ToBoolMap([]string{"x", "y", "x"})
	fmt.Println(len(set), set["x"], set["y"], set["z"])
	// Output:
	// 2 true true false
}

// Ptr returns a pointer to a copy of the value — handy for literals,
// where &42 is not allowed.
func ExamplePtr() {
	p := g_lib.Ptr(42)
	fmt.Println(*p)
	// Output:
	// 42
}
