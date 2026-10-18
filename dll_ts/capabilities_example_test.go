/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Example tests for the set-convenience capabilities (capabilities.go).
// The list needs its constructor — the zero value has no equality
// function, so writes on it panic.

package dll_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/dll_ts"
)

func ExampleDll_Slice() {
	list := dll_ts.NewDll[string]()
	list.AppendAtTail("a")
	list.AppendAtTail("b")
	list.Push("c") // Push prepends: c is the new head
	fmt.Println(list.Slice())
	// Output: [c a b]
}

func ExampleDll_String() {
	list := dll_ts.NewDll[string]()
	list.AppendAtTail("alpha")
	list.AppendAtTail("beta")
	fmt.Println(list.String())
	// Output: [alpha beta]
}

func ExampleDll_StringFunc() {
	list := dll_ts.NewDll[int]()
	list.AppendAtTail(5)
	list.AppendAtTail(8)
	fmt.Println(list.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleDll_Equal() {
	a := dll_ts.NewDll[int]()
	a.AppendAtTail(1)
	a.AppendAtTail(2)
	b := dll_ts.NewDll[int]()
	b.AppendAtTail(1)
	b.AppendAtTail(2)
	c := dll_ts.NewDll[int]()
	c.AppendAtTail(2)
	c.AppendAtTail(1)
	// Equal is order-sensitive: [1 2] != [2 1].
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleDll_Clone() {
	list := dll_ts.NewDll[string]()
	list.AppendAtTail("apple")
	dup := list.Clone()
	dup.AppendAtTail("pear")
	fmt.Println(list.Slice(), dup.Slice())
	// Output: [apple] [apple pear]
}
