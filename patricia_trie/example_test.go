/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package patricia_trie_test

import (
	"encoding/json"
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

// KeysThatMatch iterates the keys matching a Redis-style glob (the
// KEYS-command matcher: *, ?, [...], \x escaping), in ascending order.
func ExamplePatriciaTrie_KeysThatMatch() {
	var pt patricia_trie.PatriciaTrie[int]
	pt.Insert("user:1000", 0)
	pt.Insert("user:1001", 1)
	pt.Insert("user:2000", 2)
	pt.Insert("session:abc", 3)

	for key, value := range pt.KeysThatMatch("user:1???") {
		fmt.Println(key, value)
	}
	// Output:
	// user:1000 0
	// user:1001 1
}

// LongestPrefixOf returns the longest stored key that is a prefix of
// the query, with its value.
func ExamplePatriciaTrie_LongestPrefixOf() {
	var pt patricia_trie.PatriciaTrie[int]
	pt.Insert("user", 0)
	pt.Insert("user:1000", 1)

	key, value, ok := pt.LongestPrefixOf("user:1000:cart")
	fmt.Println(key, value, ok)

	key, value, ok = pt.LongestPrefixOf("session:abc")
	fmt.Println(key, value, ok)
	// Output:
	// user:1000 1 true
	//  0 false
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

// MarshalJSON encodes the trie as a JSON object mapping each key to its
// value; the json package emits the members in sorted key order, the
// trie's natural iteration order.
func ExamplePatriciaTrie_MarshalJSON() {
	var pt patricia_trie.PatriciaTrie[int]
	pt.Insert("she", 0)
	pt.Insert("by", 1)
	pt.Insert("sea", 2)

	b, err := json.Marshal(&pt)
	fmt.Println(string(b), err)
	// Output:
	// {"by":1,"sea":2,"she":0} <nil>
}

// UnmarshalJSON replaces the contents of the trie from a JSON object.
func ExamplePatriciaTrie_UnmarshalJSON() {
	var pt patricia_trie.PatriciaTrie[int] // the zero value is ready to use
	if err := json.Unmarshal([]byte(`{"she":0,"sea":2}`), &pt); err != nil {
		fmt.Println("error:", err)
		return
	}
	for key, value := range pt.All() {
		fmt.Println(key, value)
	}
	// Output:
	// sea 2
	// she 0
}
