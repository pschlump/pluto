/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package tst_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/tst_ts"
)

// A ternary search trie maps string keys to values.  The zero value is
// ready to use — there are no constructors.
func Example() {
	var tt tst_ts.Tst[int]
	tt.Insert("shells", 3)
	tt.Insert("she", 0)
	tt.Insert("sea", 6)
	tt.Insert("shore", 7)
	tt.Insert("by", 4)

	fmt.Println("length:", tt.Length())

	if v, ok := tt.Search("sea"); ok {
		fmt.Println("sea ->", v)
	}
	if _, ok := tt.Search("seas"); !ok {
		fmt.Println("seas: not found")
	}

	// All visits the keys in ascending order.
	for key, value := range tt.All() {
		fmt.Println(key, value)
	}
	// Output:
	// length: 5
	// sea -> 6
	// seas: not found
	// by 4
	// sea 6
	// she 0
	// shells 3
	// shore 7
}

// Insert returns true when a key is added and false when an existing
// value is replaced.  The empty key is rejected: Insert("") returns
// false and changes nothing.
func ExampleTst_Insert() {
	var tt tst_ts.Tst[string]

	fmt.Println(tt.Insert("go", "gopher")) // added
	fmt.Println(tt.Insert("go", "golang")) // replaced
	fmt.Println(tt.Insert("", "nothing"))  // rejected
	fmt.Println(tt.Length())

	v, _ := tt.Search("go")
	fmt.Println(v)
	// Output:
	// true
	// false
	// false
	// 1
	// golang
}

// Delete removes a key and prunes the dead branches it leaves behind.
func ExampleTst_Delete() {
	var tt tst_ts.Tst[int]
	tt.Insert("she", 0)
	tt.Insert("shells", 3)
	tt.Insert("sea", 6)

	fmt.Println(tt.Delete("sells")) // a missing key reports false
	fmt.Println(tt.Delete("she"))
	fmt.Println(tt.Contains("she"), tt.Contains("shells"))
	fmt.Println(tt.Length())
	// Output:
	// false
	// true
	// false true
	// 2
}

// LongestPrefixOf finds the longest key of the trie that is a prefix of
// the query — the classic URL-routing / IP-lookup query.
func ExampleTst_LongestPrefixOf() {
	var tt tst_ts.Tst[int]
	tt.Insert("she", 0)
	tt.Insert("shell", 1)
	tt.Insert("shells", 2)
	tt.Insert("sea", 3)

	fmt.Println(tt.LongestPrefixOf("shellsort"))
	fmt.Println(tt.LongestPrefixOf("shelters"))
	fmt.Println(tt.LongestPrefixOf("seashore"))
	fmt.Printf("%q\n", tt.LongestPrefixOf("xyz"))
	// Output:
	// shells
	// she
	// sea
	// ""
}

// KeysWithPrefix returns the keys beginning with a prefix, in ascending
// order.
func ExampleTst_KeysWithPrefix() {
	var tt tst_ts.Tst[int]
	for i, key := range []string{"she", "shells", "sea", "shore", "the"} {
		tt.Insert(key, i)
	}
	for _, key := range tt.KeysWithPrefix("sh") {
		fmt.Println(key)
	}
	// Output:
	// she
	// shells
	// shore
}

// KeysThatMatch matches a pattern where '.' stands for any single byte.
func ExampleTst_KeysThatMatch() {
	var tt tst_ts.Tst[int]
	for i, key := range []string{"she", "the", "sea", "by"} {
		tt.Insert(key, i)
	}
	for _, key := range tt.KeysThatMatch(".he") {
		fmt.Println(key)
	}
	// Output:
	// she
	// the
}

// Lock and Unlock expose the write lock for compound multi-step
// operations: inside the critical section use only the Nl-prefixed
// (no-lock) methods.
func ExampleTst_Lock() {
	var tt tst_ts.Tst[int]
	tt.Insert("sea", 6)

	// Atomically search-and-replace under one lock hold.
	tt.Lock()
	if v, found := tt.NlSearch("sea"); found {
		tt.NlInsert("sea", v+1)
	}
	tt.Unlock()

	v, _ := tt.Search("sea")
	fmt.Println(v)
	// Output:
	// 7
}
