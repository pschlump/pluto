/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package trie_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/trie_ts"
)

// The algs4 §5.4 sample keys as a tiny symbol table: insert, search,
// replace, delete, then iterate in ascending key order — safe to share
// between goroutines.
func Example() {
	var tr trie_ts.Trie[int] // the zero value is ready to use

	tr.Insert("she", 0)
	tr.Insert("sells", 1)
	tr.Insert("sea", 2)
	tr.Insert("shells", 3)
	tr.Insert("by", 4)
	tr.Insert("the", 5)
	tr.Insert("shore", 6)

	if v, ok := tr.Search("sea"); ok {
		fmt.Println("sea ->", v)
	}
	fmt.Println("shells present:", tr.Contains("shells"))

	tr.Insert("sea", 7) // a duplicate replaces the value
	v, _ := tr.Search("sea")
	fmt.Println("sea now ->", v)

	tr.Delete("she")
	fmt.Println("she present after delete:", tr.Contains("she"))

	for key, value := range tr.All() {
		fmt.Println(key, value)
	}
	// Output:
	// sea -> 2
	// shells present: true
	// sea now -> 7
	// she present after delete: false
	// by 4
	// sea 7
	// sells 1
	// shells 3
	// shore 6
	// the 5
}

// LongestPrefixOf finds the longest key that is a prefix of the query —
// the classic use is IP routing's longest-prefix match.
func ExampleTrie_LongestPrefixOf() {
	var tr trie_ts.Trie[int]
	tr.Insert("she", 0)
	tr.Insert("shells", 1)
	tr.Insert("by", 2)

	fmt.Println(tr.LongestPrefixOf("shellsort"))
	fmt.Println(tr.LongestPrefixOf("shelter"))
	fmt.Printf("%q\n", tr.LongestPrefixOf("xylophone"))
	// Output:
	// shells
	// she
	// ""
}

// KeysWithPrefix collects keys sharing a prefix, in ascending order.
// The result is an eager snapshot taken under the read lock.
func ExampleTrie_KeysWithPrefix() {
	var tr trie_ts.Trie[int]
	tr.Insert("sea", 0)
	tr.Insert("sells", 1)
	tr.Insert("she", 2)
	tr.Insert("shells", 3)
	tr.Insert("shore", 4)
	tr.Insert("the", 5)

	fmt.Println(tr.KeysWithPrefix("sh"))
	fmt.Println(tr.KeysWithPrefix("zebra"))
	// Output:
	// [she shells shore]
	// []
}

// KeysThatMatch matches a pattern where '.' stands for any one byte.
func ExampleTrie_KeysThatMatch() {
	var tr trie_ts.Trie[int]
	tr.Insert("she", 0)
	tr.Insert("sells", 1)
	tr.Insert("sea", 2)
	tr.Insert("shells", 3)
	tr.Insert("by", 4)
	tr.Insert("the", 5)
	tr.Insert("shore", 6)

	fmt.Println(tr.KeysThatMatch(".he"))
	fmt.Println(tr.KeysThatMatch("s.."))
	// Output:
	// [she the]
	// [sea she]
}

// All walks a snapshot, so it is safe to delete from inside the loop.
func ExampleTrie_All() {
	var tr trie_ts.Trie[int]
	tr.Insert("a", 1)
	tr.Insert("b", 2)
	tr.Insert("c", 3)

	// Deleting each yielded key inside the loop is safe: All iterates a
	// snapshot collected when it was called.
	for key := range tr.All() {
		tr.Delete(key)
	}
	fmt.Println(tr.IsEmpty())
	// Output:
	// true
}

// Lock plus the Nl-prefixed methods run a multi-step operation
// atomically under one lock hold — here a search-then-delete.
func ExampleTrie_Lock() {
	var tr trie_ts.Trie[int]
	tr.Insert("go", 1)
	tr.Insert("gone", 2)

	tr.Lock()
	if v, found := tr.NlSearch("go"); found {
		fmt.Println("removing go ->", v)
		tr.NlDelete("go")
	}
	tr.Unlock()

	fmt.Println(tr.Contains("go"), tr.Contains("gone"))
	// Output:
	// removing go -> 1
	// false true
}

// Insert on a nil *Trie panics — the package's only panic.  Every other
// operation tolerates a nil trie as an empty one.
func ExampleTrie_Insert_panic() {
	var nilTrie *trie_ts.Trie[int]
	fmt.Println(nilTrie.Length())       // a nil trie reads as empty
	fmt.Println(nilTrie.Contains("go")) //

	defer func() {
		fmt.Println("recovered:", recover())
	}()
	nilTrie.Insert("go", 1)
	// Output:
	// 0
	// false
	// recovered: trie_ts: Insert called on a nil Trie
}
