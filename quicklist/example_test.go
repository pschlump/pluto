/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package quicklist_test

import (
	"fmt"

	"github.com/pschlump/pluto/quicklist"
)

// A segmented deque used like a Redis list: push at both ends, index
// from either side (negative indexes count back from the tail), and
// iterate a range.
func Example() {
	q := quicklist.NewQuickList[string]()
	q.PushTail("b")
	q.PushTail("c")
	q.PushHead("a")

	fmt.Println(q.Len())
	v, _ := q.At(-1)
	fmt.Println(v)
	for i, v := range q.Range(0, 1) {
		fmt.Println(i, v)
	}
	// Output:
	// 3
	// c
	// 0 a
	// 1 b
}

// MoveHeadToTail is RPOPLPUSH/LMOVE: the head of src lands on the tail
// of dst.
func ExampleMoveHeadToTail() {
	src := quicklist.NewQuickList[int]()
	dst := quicklist.NewQuickList[int]()
	src.PushTail(1)
	src.PushTail(2)
	dst.PushTail(9)

	v, ok := quicklist.MoveHeadToTail(src, dst)
	fmt.Println(v, ok)
	for _, v := range dst.All() {
		fmt.Println(v)
	}
	// Output:
	// 1 true
	// 9
	// 1
}

// WithCompression stores interior segments compressed — transparent to
// every operation — while the first and last depth segments stay plain.
func ExampleWithCompression() {
	q := quicklist.NewQuickList(
		quicklist.WithSegmentFill[string](4),
		quicklist.WithCompression[string](
			quicklist.LZWCodec(), 1,
			quicklist.EncodeStringSegment, quicklist.DecodeStringSegment))
	for i := 0; i < 10; i++ {
		q.PushTail(fmt.Sprintf("item-%d", i))
	}
	q.Set(5, "changed")
	v, _ := q.At(5)
	fmt.Println(v)
	fmt.Println(q.Len())
	// Output:
	// changed
	// 10
}
