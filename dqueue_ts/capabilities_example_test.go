/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Example tests for the set-convenience capabilities (capabilities.go).
// The deque has no constructor — the zero value is ready to use.  The
// examples are single-goroutine; run the tests with -race for the
// concurrency guarantees.

package dqueue_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/dqueue_ts"
)

func ExampleDeque_Slice() {
	var q dqueue_ts.Deque[int]
	q.PushBack(1)
	q.PushBack(2)
	q.PushFront(0)
	fmt.Println(q.Slice())
	// Output: [0 1 2]
}

func ExampleDeque_String() {
	var q dqueue_ts.Deque[string]
	q.PushBack("alpha")
	q.PushBack("beta")
	fmt.Println(q.String())
	// Output: [alpha beta]
}

func ExampleDeque_StringFunc() {
	var q dqueue_ts.Deque[int]
	q.PushBack(5)
	q.PushBack(8)
	fmt.Println(q.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleDeque_Equal() {
	a := &dqueue_ts.Deque[int]{}
	a.PushBack(1)
	a.PushBack(2)
	b := &dqueue_ts.Deque[int]{}
	b.PushBack(1)
	b.PushBack(2)
	c := &dqueue_ts.Deque[int]{}
	c.PushBack(2)
	c.PushBack(1)
	// Equal is order-sensitive: [1 2] != [2 1].
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleDeque_Clone() {
	var q dqueue_ts.Deque[string]
	q.PushBack("apple")
	dup := q.Clone()
	dup.PushBack("pear")
	fmt.Println(q.Slice(), dup.Slice())
	// Output: [apple] [apple pear]
}
