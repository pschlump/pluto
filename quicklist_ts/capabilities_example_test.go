/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Example tests for the set-convenience capabilities (capabilities.go).
// The examples are single-goroutine; run the tests with -race for the
// concurrency guarantees.

package quicklist_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/quicklist_ts"
)

func ExampleQuickList_Slice() {
	q := quicklist_ts.NewQuickList[int]()
	q.PushTail(1)
	q.PushTail(2)
	q.PushTail(3)
	fmt.Println(q.Slice())
	// Output: [1 2 3]
}

func ExampleQuickList_String() {
	q := quicklist_ts.NewQuickList[string]()
	q.PushTail("alpha")
	q.PushTail("beta")
	fmt.Println(q.String())
	// Output: [alpha beta]
}

func ExampleQuickList_StringFunc() {
	q := quicklist_ts.NewQuickList[int]()
	q.PushTail(5)
	q.PushTail(8)
	fmt.Println(q.StringFunc(func(v int) string { return fmt.Sprintf("#%d", v) }))
	// Output: [#5 #8]
}

func ExampleQuickList_Equal() {
	a := quicklist_ts.NewQuickList[int]()
	a.PushTail(1)
	a.PushTail(2)
	b := quicklist_ts.NewQuickList[int]()
	b.PushTail(1)
	b.PushTail(2)
	c := quicklist_ts.NewQuickList[int]()
	c.PushHead(1)
	c.PushHead(2) // c holds [2 1]
	// Equal is order-sensitive: [1 2] != [2 1].
	fmt.Println(a.Equal(b), a.Equal(c))
	// Output: true false
}

func ExampleQuickList_Clone() {
	q := quicklist_ts.NewQuickList[string]()
	q.PushTail("apple")
	dup := q.Clone()
	dup.PushTail("pear")
	fmt.Println(q.Slice(), dup.Slice())
	// Output: [apple] [apple pear]
}
