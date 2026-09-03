/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package quicklist_ts_test

import (
	"encoding/json"
	"fmt"

	"github.com/pschlump/pluto/quicklist"
	"github.com/pschlump/pluto/quicklist_ts"
)

// The thread-safe segmented deque has the same API as the plain
// package, plus Lock/Unlock with the Nl* methods for compound
// operations.
func Example() {
	q := quicklist_ts.NewQuickList[string]()
	q.PushTail("b")
	q.PushHead("a")
	q.PushTail("c")

	fmt.Println(q.Len())
	v, _ := q.At(-1)
	fmt.Println(v)
	// Output:
	// 3
	// c
}

// Lock takes the write lock for a compound operation; the Nl* methods
// run unlocked while it is held.
func ExampleQuickList_Lock() {
	q := quicklist_ts.NewQuickList[int]()
	for i := 1; i <= 4; i++ {
		q.PushTail(i)
	}

	// Atomically double every element.
	q.Lock()
	for i := 0; i < q.NlLen(); i++ {
		v, _ := q.NlAt(i)
		q.NlSet(i, 2*v)
	}
	q.Unlock()

	for _, v := range q.All() {
		fmt.Println(v)
	}
	// Output:
	// 2
	// 4
	// 6
	// 8
}

// Options come from the plain package and pass straight through.
func ExampleNewQuickList() {
	q := quicklist_ts.NewQuickList(
		quicklist.WithSegmentFill[string](64),
		quicklist.WithCompression[string](
			quicklist.LZWCodec(), 1,
			quicklist.EncodeStringSegment, quicklist.DecodeStringSegment))
	for i := 0; i < 300; i++ {
		q.PushTail("entry")
	}
	fmt.Println(q.Len())
	// Output:
	// 300
}

// MarshalJSON encodes the list as a JSON array of its elements, head to
// tail.
func ExampleQuickList_MarshalJSON() {
	q := quicklist_ts.NewQuickList[int]()
	q.PushTail(3)
	q.PushTail(1)
	q.PushTail(2)

	b, err := json.Marshal(q)
	fmt.Println(string(b), err)
	// Output:
	// [3,1,2] <nil>
}

// UnmarshalJSON replaces the contents of the list from a JSON array;
// element 0 becomes the new head.
func ExampleQuickList_UnmarshalJSON() {
	q := quicklist_ts.NewQuickList[string]()
	if err := json.Unmarshal([]byte(`["c","a"]`), q); err != nil {
		fmt.Println("error:", err)
		return
	}
	for i, v := range q.All() {
		fmt.Println(i, v)
	}
	// Output:
	// 0 c
	// 1 a
}
