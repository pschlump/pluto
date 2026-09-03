/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package lru_test

import (
	"encoding/json"
	"fmt"

	"github.com/pschlump/pluto/lru"
)

func ExampleLru() {
	c := lru.NewLru[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a")    // a hit marks "a" most-recently-used
	c.Put("c", 3) // so "b" is the eviction victim, not "a"
	first := true
	for k, v := range c.All() { // most to least recently used
		if !first {
			fmt.Print(" ")
		}
		first = false
		fmt.Printf("%s=%d", k, v)
	}
	fmt.Println()
	if _, ok := c.Get("b"); !ok {
		fmt.Println("b was evicted")
	}
	// Output:
	// c=3 a=1
	// b was evicted
}

// Peek looks without changing the recency order; Get re-marks.
func ExampleLru_Peek() {
	c := lru.NewLru[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Peek("a")   // no reorder: "a" stays least-recently-used
	c.Put("c", 3) // so "a" is evicted
	_, aOK := c.Get("a")
	fmt.Println("a cached:", aOK)
	// Output:
	// a cached: false
}

// NewLruFunc takes a veto callback: an entry the callback rejects is
// skipped during eviction (the next-older entry goes instead), and when
// nothing is evictable the cache temporarily exceeds its capacity — the
// soft cap.
func ExampleNewLruFunc() {
	c := lru.NewLruFunc(2, func(k string, _ int) bool {
		return k != "pinned" // "pinned" vetoes its own eviction
	})
	c.Put("pinned", 0)
	c.Put("x", 1)
	c.Put("y", 2) // "pinned" is LRU but vetoed: "x" is evicted instead
	c.Put("z", 3) // now "pinned" and "y" — "y" goes
	first := true
	for k := range c.All() {
		if !first {
			fmt.Print(" ")
		}
		first = false
		fmt.Print(k)
	}
	fmt.Println()
	// Output:
	// z pinned
}

// MarshalJSON encodes the cache as a JSON array of {"k":key,"v":value}
// objects, most recently used first.
func ExampleLru_MarshalJSON() {
	c := lru.NewLru[string, int](3)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)

	b, err := json.Marshal(c)
	fmt.Println(string(b), err)
	// Output:
	// [{"k":"c","v":3},{"k":"b","v":2},{"k":"a","v":1}] <nil>
}

// UnmarshalJSON replaces the contents of the cache from a JSON array of
// {"k":key,"v":value} objects, recreating the encoded recency order —
// pair 0 becomes the most recently used entry.
func ExampleLru_UnmarshalJSON() {
	c := lru.NewLru[string, int](3)
	if err := json.Unmarshal([]byte(`[{"k":"c","v":3},{"k":"a","v":1}]`), c); err != nil {
		fmt.Println("error:", err)
		return
	}
	for k, v := range c.All() {
		fmt.Println(k, v)
	}
	// Output:
	// c 3
	// a 1
}
