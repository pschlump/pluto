/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Examples for the set-convenience capabilities (capabilities.go): the
// string-set ergonomics layered on the trie.
package patricia_trie_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/patricia_trie_ts"
)

// Keys, Values and String materialize the trie as a sorted key set.
func ExamplePatriciaTrie_Keys() {
	var tr patricia_trie_ts.PatriciaTrie[int]
	for _, k := range []string{"she", "sells", "sea", "shells"} {
		tr.Insert(k, len(k))
	}
	fmt.Println(tr.Keys())
	fmt.Println(tr.Values()) // aligned with Keys, by key order
	fmt.Println(tr.String()) // the values are not rendered
	// Output:
	// [sea sells she shells]
	// [3 5 3 6]
	// [sea sells she shells]
}

// StringFunc renders each key with a caller format function.
func ExamplePatriciaTrie_StringFunc() {
	var tr patricia_trie_ts.PatriciaTrie[int]
	for _, k := range []string{"b", "a", "c"} {
		tr.Insert(k, 0)
	}
	fmt.Println(tr.StringFunc(func(key string) string {
		return "<" + key + ">"
	}))
	// Output:
	// [<a> <b> <c>]
}

// Equal compares KEY SETS only — values are not compared.
func ExamplePatriciaTrie_Equal() {
	var a, b patricia_trie_ts.PatriciaTrie[int]
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
func ExamplePatriciaTrie_Clone() {
	var tr patricia_trie_ts.PatriciaTrie[int]
	tr.Insert("she", 1)
	tr.Insert("shells", 2)

	dup := tr.Clone()
	dup.Insert("extra", 3)
	dup.Delete("she")

	fmt.Println(tr.Keys())
	fmt.Println(dup.Keys())
	// Output:
	// [she shells]
	// [extra shells]
}

// The key batches: ContainsSlice, ContainsAny and DeleteSlice.
func ExamplePatriciaTrie_containsBatches() {
	var tr patricia_trie_ts.PatriciaTrie[int]
	for _, k := range []string{"she", "sells", "sea", "shells"} {
		tr.Insert(k, len(k))
	}

	fmt.Println(tr.ContainsSlice([]string{"she", "sea"}))
	fmt.Println(tr.ContainsSlice([]string{"she", "shell"})) // a prefix is not a key
	fmt.Println(tr.ContainsAny([]string{"shell", "shells"}))

	changed := tr.DeleteSlice([]string{"she", "nope", "sea"})
	fmt.Println(changed, tr.Keys())
	// Output:
	// true
	// false
	// true
	// true [sells shells]
}
