/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Example tests for the set-convenience capabilities (capabilities.go).
// The list needs its constructor — the zero value has no equality
// function, so writes on it panic.

package sll_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/sll_ts"
)

func ExampleSll_Slice() {
	list := sll_ts.NewSll[string]()
	list.InsertAfterTail("a")
	list.InsertAfterTail("b")
	list.InsertBeforeHead("c") // c is the new head
	fmt.Println(list.Slice())
	// Output: [c a b]
}

func ExampleSll_String() {
	list := sll_ts.NewSll[string]()
	list.InsertAfterTail("alpha")
	list.InsertAfterTail("beta")
	fmt.Println(list.String())
	// Output: [alpha beta]
}

func ExampleSll_StringFunc() {
	list := sll_ts.NewSll[int]()
	list.InsertAfterTail(5)
	list.InsertAfterTail(8)
	fmt.Println(list.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleSll_Equal() {
	a := sll_ts.NewSll[int]()
	a.InsertAfterTail(1)
	a.InsertAfterTail(2)
	b := sll_ts.NewSll[int]()
	b.InsertAfterTail(1)
	b.InsertAfterTail(2)
	c := sll_ts.NewSll[int]()
	c.InsertAfterTail(2)
	c.InsertAfterTail(1)
	// Equal is order-sensitive: [1 2] != [2 1].
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleSll_Clone() {
	list := sll_ts.NewSll[string]()
	list.InsertAfterTail("apple")
	dup := list.Clone()
	dup.InsertAfterTail("pear")
	fmt.Println(list.Slice(), dup.Slice())
	// Output: [apple] [apple pear]
}
