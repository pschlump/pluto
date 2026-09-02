/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package stream_test

import (
	"fmt"

	"github.com/pschlump/pluto/stream"
)

// The append-oriented log: auto-sequence XADD-style adds, a bounded
// range walk, and the trim.
func Example() {
	var s stream.Stream // the zero value is ready to use

	for i := range 5 {
		_, _ = s.Add(stream.ID{Ms: 100, Seq: stream.AutoSeq}, [][2]string{{"sensor", fmt.Sprintf("%d", i*7)}})
	}
	fmt.Println("len:", s.Len())
	first, _ := s.FirstID()
	fmt.Println("first:", first)
	fmt.Println("last:", s.LastID())

	// The middle of the log, oldest first (the XRANGE form).
	for e := range s.Range(stream.ID{Ms: 100, Seq: 1}, stream.ID{Ms: 100, Seq: 2}, 0) {
		fmt.Println(e.ID, e.Fields[0][1])
	}

	// The newest first (the XREVRANGE form — note the (end, start) order).
	for e := range s.RevRange(stream.MaxID, stream.MinID, 2) {
		fmt.Println(e.ID)
	}

	evicted := s.TrimMaxLen(2)
	fmt.Println("evicted:", evicted, "kept:", s.Len())
	fmt.Println("last unchanged:", s.LastID())

	// Output:
	// len: 5
	// first: 100-0
	// last: 100-4
	// 100-1 7
	// 100-2 14
	// 100-4
	// 100-3
	// evicted: 3 kept: 2
	// last unchanged: 100-4
}

// Consumer groups: deliver entries to a consumer, inspect the pending
// list, acknowledge, and reclaim abandoned work.
func ExampleStream_ReadGroup() {
	var s stream.Stream
	for i := range 4 {
		_, _ = s.Add(stream.ID{Ms: 1, Seq: uint64(i)}, [][2]string{{"job", fmt.Sprintf("j%d", i)}})
	}
	_ = s.CreateGroup("workers", stream.MinID) // deliver from the start

	// The ">" read (after == MinID): never-delivered entries, up to 3.
	for _, e := range s.ReadGroup("workers", "alice", stream.MinID, 3) {
		fmt.Println("alice got", e.ID)
	}
	for _, e := range s.ReadGroup("workers", "bob", stream.MinID, 3) {
		fmt.Println("bob got", e.ID)
	}

	count, min, max, per := s.Pending("workers")
	fmt.Println("pending:", count, min, max, per["alice"], per["bob"])

	// Alice finishes two jobs.
	fmt.Println("acked:", s.Ack("workers", stream.ID{Ms: 1}, stream.ID{Ms: 1, Seq: 2}))

	// Bob abandons the rest: a zero min-idle AutoClaim hands everything
	// still pending over to carol, reporting IDs whose entries are gone.
	entries, next, deleted := s.AutoClaim("workers", "carol", 0, stream.MinID, 10)
	for _, e := range entries {
		fmt.Println("carol claimed", e.ID)
	}
	fmt.Println("next:", next, "deleted:", len(deleted))

	// Output:
	// alice got 1-0
	// alice got 1-1
	// alice got 1-2
	// bob got 1-3
	// pending: 4 1-0 1-3 3 1
	// acked: 2
	// carol claimed 1-1
	// carol claimed 1-3
	// next: 0-0 deleted: 0
}

// Parsing and printing IDs, including the "ms-*" auto-sequence request
// form.
func ExampleParseID() {
	for _, in := range []string{"1234-56", "1234", "1234-*"} {
		id, err := stream.ParseID(in)
		fmt.Println(in, "->", id, err)
	}

	// Output:
	// 1234-56 -> 1234-56 <nil>
	// 1234 -> 1234-0 <nil>
	// 1234-* -> 1234-* <nil>
}
