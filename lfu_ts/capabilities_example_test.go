/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/lfu_ts"
)

// The rendering matches the plain package: the keys, sorted.
func ExampleLfu_String() {
	l := lfu_ts.NewLfu[string](lfu_ts.DefaultLogFactor, lfu_ts.DefaultDecayTime)
	l.Add("pear")
	l.Add("fig")
	l.Add("apple")
	fmt.Println(l.String())
	// Output:
	// [apple fig pear]
}

// StringFunc renders the same sorted keys with a caller-supplied
// format; the callback runs outside the lock.
func ExampleLfu_StringFunc() {
	l := lfu_ts.NewLfu[int](lfu_ts.DefaultLogFactor, lfu_ts.DefaultDecayTime)
	for _, k := range []int{5, 1, 3} {
		l.Add(k)
	}
	fmt.Println(l.StringFunc(func(key int) string { return fmt.Sprintf("#%d", key) }))
	// Output:
	// [#1 #3 #5]
}
