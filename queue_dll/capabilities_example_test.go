/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Example tests for the set-convenience capabilities (capabilities.go).
// The queue has no constructor — the zero value is ready to use.

package queue_dll_test

import (
	"fmt"

	"github.com/pschlump/pluto/queue_dll"
)

func ExampleQueue_Slice() {
	var q queue_dll.Queue[int]
	q.Push(1)
	q.Push(2)
	q.Push(3)
	fmt.Println(q.Slice())
	// Output: [1 2 3]
}

func ExampleQueue_String() {
	var q queue_dll.Queue[string]
	q.Push("alpha")
	q.Push("beta")
	fmt.Println(q.String())
	// Output: [alpha beta]
}

func ExampleQueue_StringFunc() {
	var q queue_dll.Queue[int]
	q.Push(5)
	q.Push(8)
	fmt.Println(q.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleQueue_Equal() {
	a := &queue_dll.Queue[int]{}
	a.Push(1)
	a.Push(2)
	b := &queue_dll.Queue[int]{}
	b.Push(1)
	b.Push(2)
	c := &queue_dll.Queue[int]{}
	c.Push(2)
	c.Push(1)
	// Equal is order-sensitive: [1 2] != [2 1].
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleQueue_Clone() {
	var q queue_dll.Queue[string]
	q.Push("apple")
	dup := q.Clone()
	dup.Push("pear")
	fmt.Println(q.Slice(), dup.Slice())
	// Output: [apple] [apple pear]
}
