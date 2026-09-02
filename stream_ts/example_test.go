/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package stream_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/stream_ts"
)

// The shared stream: appends from many goroutines in the auto-sequence
// form (the "next seq" resolution runs under the lock, so the
// strictly-increasing rule holds without coordination), then a snapshot
// range read that is safe to take while writers keep writing.
func Example() {
	var s stream_ts.Stream // the zero value is ready to use

	for i := range 5 {
		_, _ = s.Add(stream_ts.ID{Ms: 100, Seq: stream_ts.AutoSeq}, [][2]string{{"n", fmt.Sprintf("%d", i)}})
	}

	// The compound-operation surface: read a batch and acknowledge it
	// atomically, with no other goroutine able to deliver the same
	// entries in between.
	_ = s.CreateGroup("workers", stream_ts.MinID)
	s.Lock()
	delivered := s.NlReadGroup("workers", "alice", stream_ts.MinID, 3)
	acked := s.NlAck("workers", stream_ts.ID{Ms: 100}, stream_ts.ID{Ms: 100, Seq: 1})
	s.Unlock()
	fmt.Println("delivered:", len(delivered), "acked:", acked)

	// The snapshot iterator sees the state at call time.
	for e := range s.Range(stream_ts.MinID, stream_ts.MaxID, 2) {
		fmt.Println(e.ID, e.Fields[0][1])
	}
	count, _, _, _ := s.Pending("workers")
	fmt.Println("pending:", count)

	// Output:
	// delivered: 3 acked: 2
	// 100-0 0
	// 100-1 1
	// pending: 1
}
