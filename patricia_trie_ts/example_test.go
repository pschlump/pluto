/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package patricia_trie_ts_test

import (
	"encoding/json"
	"fmt"

	"github.com/pschlump/pluto/patricia_trie_ts"
)

// The classic string-table keys as a tiny symbol table: insert, search,
// replace, delete, then iterate in ascending key order.
func Example() {
	var pt patricia_trie_ts.PatriciaTrie[int] // the zero value is ready to use

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
	var pt patricia_trie_ts.PatriciaTrie[int]
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
	var pt patricia_trie_ts.PatriciaTrie[int]
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
	var nilTrie *patricia_trie_ts.PatriciaTrie[int]
	fmt.Println(nilTrie.Length())       // a nil trie reads as empty
	fmt.Println(nilTrie.Contains("go")) //

	defer func() {
		fmt.Println("recovered:", recover())
	}()
	nilTrie.Insert("go", 1)
	// Output:
	// 0
	// false
	// recovered: patricia_trie_ts: Insert called on a nil PatriciaTrie
}

// KeysThatMatch iterates the keys matching a Redis-style glob (the
// KEYS-command matcher: *, ?, [...], \x escaping), in ascending order,
// over a call-time snapshot — mutating the trie from inside the loop is
// safe.
func ExamplePatriciaTrie_KeysThatMatch() {
	var pt patricia_trie_ts.PatriciaTrie[int]
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
	var pt patricia_trie_ts.PatriciaTrie[int]
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

// Lock + Nl methods run a compound sequence atomically under one lock
// hold — here an atomic search-then-delete.
func ExamplePatriciaTrie_Lock() {
	var pt patricia_trie_ts.PatriciaTrie[int]
	pt.Insert("user:1000", 1)
	pt.Insert("user:1001", 2)

	pt.Lock()
	if _, found := pt.NlSearch("user:1000"); found {
		pt.NlDelete("user:1000") // remove only if the search saw it — atomically
	}
	pt.Unlock()

	fmt.Println(pt.Len(), pt.Contains("user:1000"))
	// Output:
	// 1 false
}

// MarshalJSON encodes the trie as a JSON object from key to value; the
// object keys come out in ascending order, the trie's natural iteration
// order.
func ExamplePatriciaTrie_MarshalJSON() {
	var pt patricia_trie_ts.PatriciaTrie[int]
	pt.Insert("shells", 0)
	pt.Insert("sea", 1)
	pt.Insert("she", 2)

	b, err := json.Marshal(&pt)
	fmt.Println(string(b), err)
	// Output:
	// {"sea":1,"she":2,"shells":0} <nil>
}

// UnmarshalJSON replaces the contents of the trie from a JSON object.
func ExamplePatriciaTrie_UnmarshalJSON() {
	var pt patricia_trie_ts.PatriciaTrie[int] // the zero value is ready to use
	if err := json.Unmarshal([]byte(`{"she":2,"sea":1}`), &pt); err != nil {
		fmt.Println("error:", err)
		return
	}
	for key, value := range pt.All() {
		fmt.Println(key, value)
	}
	// Output:
	// sea 1
	// she 2
}
