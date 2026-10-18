/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu_test

import (
	"fmt"

	"github.com/pschlump/pluto/lfu"
)

// The keys render sorted — the backing table's slot order is random
// per table, the rendering is deterministic.
func ExampleLfu_String() {
	l := lfu.NewLfu[string](lfu.DefaultLogFactor, lfu.DefaultDecayTime)
	l.Add("pear")
	l.Add("fig")
	l.Add("apple")
	fmt.Println(l.String())
	// Output:
	// [apple fig pear]
}

// StringFunc renders the same sorted keys with a caller-supplied
// format; the frequency state stays one call away (Counter).
func ExampleLfu_StringFunc() {
	l := lfu.NewLfu[int](lfu.DefaultLogFactor, lfu.DefaultDecayTime)
	for _, k := range []int{5, 1, 3} {
		l.Add(k)
	}
	fmt.Println(l.StringFunc(func(key int) string { return fmt.Sprintf("k%d", key) }))
	// Output:
	// [k1 k3 k5]
}
