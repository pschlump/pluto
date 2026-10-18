/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Example tests for the set-convenience capabilities (capabilities.go).
// The stack has no constructor — the zero value is ready to use.  The
// examples are single-goroutine; run the tests with -race for the
// concurrency guarantees.

package stack_sll_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/stack_sll_ts"
)

func ExampleStack_Slice() {
	var ns stack_sll_ts.Stack[int]
	ns.Push(1)
	ns.Push(2)
	ns.Push(3)
	fmt.Println(ns.Slice()) // top down: 3 was pushed last
	// Output: [3 2 1]
}

func ExampleStack_String() {
	var ns stack_sll_ts.Stack[string]
	ns.Push("alpha")
	ns.Push("beta")
	fmt.Println(ns.String())
	// Output: [beta alpha]
}

func ExampleStack_StringFunc() {
	var ns stack_sll_ts.Stack[int]
	ns.Push(5)
	ns.Push(8)
	fmt.Println(ns.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#8 #5]
}

func ExampleStack_Equal() {
	a := &stack_sll_ts.Stack[int]{}
	a.Push(1)
	a.Push(2)
	b := &stack_sll_ts.Stack[int]{}
	b.Push(1)
	b.Push(2)
	c := &stack_sll_ts.Stack[int]{}
	c.Push(2)
	c.Push(1)
	// Equal is order-sensitive: b has 2 on top of 1, c has 1 on top of 2.
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleStack_Clone() {
	var ns stack_sll_ts.Stack[string]
	ns.Push("apple")
	dup := ns.Clone()
	dup.Push("pear")
	fmt.Println(ns.Slice(), dup.Slice())
	// Output: [apple] [pear apple]
}
