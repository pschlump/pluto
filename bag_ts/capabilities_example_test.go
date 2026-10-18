/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Example tests for the set-convenience capabilities (capabilities.go).
// The bag has no constructor — the zero value is ready to use.  The
// examples are single-goroutine; run the tests with -race for the
// concurrency guarantees.

package bag_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/bag_ts"
)

func ExampleBag_Slice() {
	var b bag_ts.Bag[int]
	b.Add(1)
	b.Add(2)
	b.Add(3)
	fmt.Println(b.Slice()) // insertion order: 1 was added first
	// Output: [1 2 3]
}

func ExampleBag_String() {
	var b bag_ts.Bag[string]
	b.Add("alpha")
	b.Add("beta")
	fmt.Println(b.String())
	// Output: [alpha beta]
}

func ExampleBag_StringFunc() {
	var b bag_ts.Bag[int]
	b.Add(5)
	b.Add(8)
	fmt.Println(b.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleBag_Equal() {
	a := &bag_ts.Bag[int]{}
	a.Add(1)
	a.Add(2)
	b := &bag_ts.Bag[int]{}
	b.Add(1)
	b.Add(2)
	c := &bag_ts.Bag[int]{}
	c.Add(2)
	c.Add(1)
	// Equal is order-sensitive: a and b added 1 then 2, c added 2 then 1
	// — the same multiset in a different insertion order.
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleBag_Clone() {
	var b bag_ts.Bag[string]
	b.Add("apple")
	dup := b.Clone()
	dup.Add("pear")
	fmt.Println(b.Slice(), dup.Slice())
	// Output: [apple] [apple pear]
}
