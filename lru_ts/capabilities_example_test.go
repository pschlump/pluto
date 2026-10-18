/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lru_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/lru_ts"
)

// The rendering matches the plain package: keys only, most recently
// used first, snapshotted under the read lock.
func ExampleLru_String() {
	c := lru_ts.NewLru[string, int](8)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	fmt.Println(c.String())
	c.Get("b")
	fmt.Println(c.String())
	// Output:
	// [c b a]
	// [b c a]
}

// StringFunc renders the snapshot with a caller-supplied per-key
// format; the callback runs outside the lock.
func ExampleLru_StringFunc() {
	c := lru_ts.NewLru[int, int](8)
	c.Put(1, 10)
	c.Put(2, 20)
	fmt.Println(c.StringFunc(func(key int) string { return fmt.Sprintf("#%d", key) }))
	// Output:
	// [#2 #1]
}
