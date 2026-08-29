/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package patricia_trie_test

import (
	"fmt"

	"github.com/pschlump/pluto/patricia_trie"
)

// The classic string-table keys as a tiny symbol table: insert, search,
// replace, delete, then iterate in ascending key order.
func Example() {
	var pt patricia_trie.PatriciaTrie[int] // the zero value is ready to use

	pt.Insert("she", 0)
	pt.Insert("sells", 1)
	pt.Insert("sea", 2)
	pt.Insert("shells", 3)
	pt.Insert("by", 4)
	pt.Insert("the", 5)
	pt.Insert("shore", 6)

	if v, ok := pt.Search("sea"); ok {
		fmt.Println("sea ->", v)
	}
	fmt.Println("shells present:", pt.Contains("shells"))

	pt.Insert("sea", 7) // a duplicate replaces the value
	v, _ := pt.Search("sea")
	fmt.Println("sea now ->", v)

	pt.Delete("she")
	fmt.Println("she present after delete:", pt.Contains("she"))

	for key, value := range pt.All() {
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

// KeysWithPrefix collects keys sharing a prefix, in ascending order.
func ExamplePatriciaTrie_KeysWithPrefix() {
	var pt patricia_trie.PatriciaTrie[int]
	pt.Insert("sea", 0)
	pt.Insert("sells", 1)
	pt.Insert("she", 2)
	pt.Insert("shells", 3)
	pt.Insert("shore", 4)
	pt.Insert("the", 5)

	fmt.Println(pt.KeysWithPrefix("sh"))
	fmt.Println(pt.KeysWithPrefix("zebra"))
	// Output:
	// [she shells shore]
	// []
}

// Backward iterates in descending key order.
func ExamplePatriciaTrie_Backward() {
	var pt patricia_trie.PatriciaTrie[int]
	pt.Insert("sea", 0)
	pt.Insert("sells", 1)
	pt.Insert("she", 2)

	for key := range pt.Backward() {
		fmt.Println(key)
	}
	// Output:
	// she
	// sells
	// sea
}

// Insert on a nil *PatriciaTrie panics — the package's only panic.
// Every other operation tolerates a nil trie as an empty one.
func ExamplePatriciaTrie_Insert_panic() {
	var nilTrie *patricia_trie.PatriciaTrie[int]
	fmt.Println(nilTrie.Length())       // a nil trie reads as empty
	fmt.Println(nilTrie.Contains("go")) //

	defer func() {
		fmt.Println("recovered:", recover())
	}()
	nilTrie.Insert("go", 1)
	// Output:
	// 0
	// false
	// recovered: patricia_trie: Insert called on a nil PatriciaTrie
}
