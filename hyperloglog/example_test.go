/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hyperloglog_test

import (
	"fmt"

	"github.com/pschlump/pluto/hyperloglog"
)

// ExampleHll counts the distinct visitors of a stream of events with
// 0.81% standard error in 12 KiB of registers.  The estimate is
// deterministic (the hash is a frozen constant), so the exact value is
// a stable example output.
func ExampleHll() {
	h := hyperloglog.NewHll()
	for i := range 100_000 {
		h.Add([]byte(fmt.Sprintf("visitor-%d", i)))
	}
	fmt.Println(h.Count())
	// Output: 99209
}

// ExampleHll_Merge unions two sketches: the merged estimate
// approximates the cardinality of the union of the input sets, the
// PFMERGE operation.
func ExampleHll_Merge() {
	eu, ap := hyperloglog.NewHll(), hyperloglog.NewHll()
	for i := range 40_000 {
		eu.Add([]byte(fmt.Sprintf("user:%d", i)))
		ap.Add([]byte(fmt.Sprintf("user:%d", i+30_000))) // 10k overlap with eu
	}
	eu.Merge(ap) // |eu ∪ ap| = 70k
	fmt.Println(eu.Count())
	// Output: 70057
}

// ExampleHllFromBytes decodes a serialized sketch — hand a Bytes()
// snapshot across a process boundary and keep counting.
func ExampleHllFromBytes() {
	h := hyperloglog.NewHll()
	for i := range 1234 {
		h.Add([]byte(fmt.Sprintf("item-%d", i)))
	}
	decoded, err := hyperloglog.HllFromBytes(h.Bytes())
	if err != nil {
		fmt.Println("decode error:", err)
		return
	}
	fmt.Println(decoded.Count() == h.Count(), decoded.IsEmpty())
	// Output: true false
}
