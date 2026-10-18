/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lru_test

import (
	"fmt"

	"github.com/pschlump/pluto/lru"
)

// The rendering is the recency order — keys only, most recently used
// first; a Get reorders it.
func ExampleLru_String() {
	c := lru.NewLru[string, int](8)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	fmt.Println(c.String())
	c.Get("a") // promotes a to most recently used
	fmt.Println(c.String())
	// Output:
	// [c b a]
	// [a c b]
}

// StringFunc renders the same recency order with a caller-supplied
// per-key format.
func ExampleLru_StringFunc() {
	c := lru.NewLru[int, string](8)
	c.Put(1, "one")
	c.Put(2, "two")
	fmt.Println(c.StringFunc(func(key int) string { return fmt.Sprintf("key=%d", key) }))
	// Output:
	// [key=2 key=1]
}
