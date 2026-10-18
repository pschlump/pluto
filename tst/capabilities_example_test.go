/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Examples for the set-convenience capabilities (capabilities.go): the
// string-set ergonomics layered on the ternary search trie.
package tst_test

import (
	"fmt"

	"github.com/pschlump/pluto/tst"
)

// Keys, Values and String materialize the trie as a sorted key set.
func ExampleTst_Keys() {
	var tt tst.Tst[int]
	for _, k := range []string{"she", "sells", "sea", "shells"} {
		tt.Insert(k, len(k))
	}
	fmt.Println(tt.Keys())
	fmt.Println(tt.Values()) // aligned with Keys, by key order
	fmt.Println(tt.String()) // the values are not rendered
	// Output:
	// [sea sells she shells]
	// [3 5 3 6]
	// [sea sells she shells]
}

// StringFunc renders each key with a caller format function.
func ExampleTst_StringFunc() {
	var tt tst.Tst[int]
	for _, k := range []string{"b", "a", "c"} {
		tt.Insert(k, 0)
	}
	fmt.Println(tt.StringFunc(func(key string) string {
		return "<" + key + ">"
	}))
	// Output:
	// [<a> <b> <c>]
}

// Equal compares KEY SETS only — values are not compared.
func ExampleTst_Equal() {
	var a, b tst.Tst[int]
	a.Insert("sea", 1)
	a.Insert("shells", 2)
	b.Insert("shells", 99)   // different value
	b.Insert("sea", 98)      // different value
	fmt.Println(a.Equal(&b)) // same keys -> equal

	b.Insert("extra", 0)
	fmt.Println(a.Equal(&b)) // different key sets
	fmt.Println(a.Equal(&a)) // a trie equals itself
	// Output:
	// true
	// false
	// true
}

// Clone is a deep copy — mutating either trie leaves the other alone.
func ExampleTst_Clone() {
	var tt tst.Tst[int]
	tt.Insert("she", 1)
	tt.Insert("shells", 2)

	dup := tt.Clone()
	dup.Insert("extra", 3)
	dup.Delete("she")

	fmt.Println(tt.Keys())
	fmt.Println(dup.Keys())
	// Output:
	// [she shells]
	// [extra shells]
}

// The key batches: ContainsSlice, ContainsAny and DeleteSlice.
func ExampleTst_containsBatches() {
	var tt tst.Tst[int]
	for _, k := range []string{"she", "sells", "sea", "shells"} {
		tt.Insert(k, len(k))
	}

	fmt.Println(tt.ContainsSlice([]string{"she", "sea"}))
	fmt.Println(tt.ContainsSlice([]string{"she", "shell"})) // a prefix is not a key
	fmt.Println(tt.ContainsAny([]string{"shell", "shells"}))

	changed := tt.DeleteSlice([]string{"she", "nope", "sea"})
	fmt.Println(changed, tt.Keys())
	// Output:
	// true
	// false
	// true
	// true [sells shells]
}
