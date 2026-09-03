/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bloom_test

import (
	"fmt"

	"github.com/pschlump/pluto/bloom"
)

// ExampleBloom is the core use: size for the workload, add, and ask
// membership — never a false negative, ~1% false positives at the
// design load.  The outputs are deterministic (the hashes are frozen
// constants), so they are stable example values.
func ExampleBloom() {
	f := bloom.NewBloom(1_000, 0.01) // ~1k elements at ~1% false positives
	for i := range 1_000 {
		f.Add([]byte(fmt.Sprintf("visitor-%d", i)))
	}
	fmt.Println(f.MayContain([]byte("visitor-500")))
	fmt.Println(f.MayContain([]byte("visitor-999999")))
	fmt.Printf("%.3f %d\n", f.Saturation(), f.Count()) // fill ratio and distinct estimate
	// Output:
	// true
	// false
	// 0.520 1006
}

// ExampleBloom_TestAndSet deduplicates a stream in one pass — the
// original 2016 library's operation: false marks the first sighting
// (and records it), true a (probable) repeat.
func ExampleBloom_TestAndSet() {
	f := bloom.NewBloom(100, 0.01)
	stream := []string{"a", "b", "a", "c", "b", "d", "a"}
	firstSeen := 0
	for _, s := range stream {
		if !f.TestAndSet([]byte(s)) {
			firstSeen++
		}
	}
	fmt.Println(firstSeen, "distinct in", len(stream), "events")
	// Output: 4 distinct in 7 events
}

// ExampleBloom_Merge unions two same-shape filters — the lossless
// combination for sharded or accumulated sets (added histories sum).
func ExampleBloom_Merge() {
	eu, ap := bloom.NewBloom(10_000, 0.01), bloom.NewBloom(10_000, 0.01)
	for i := range 4_000 {
		eu.Add([]byte(fmt.Sprintf("user:%d", i)))
	}
	for i := range 3_000 {
		ap.Add([]byte(fmt.Sprintf("user:%d", i+3_500))) // 500 overlap
	}
	eu.Merge(ap) // |eu ∪ ap| = 6500
	fmt.Println(eu.Count(), eu.Added())
	// Output: 6506 7000
}

// ExampleBloomFromBytes carries a filter across a process boundary and
// keeps querying — the serialized form is frozen (versionless), so a
// filter written by an older build of the same library decodes here.
func ExampleBloomFromBytes() {
	f := bloom.NewBloom(100, 0.01)
	for i := range 100 {
		f.Add([]byte(fmt.Sprintf("item-%d", i)))
	}
	decoded, err := bloom.BloomFromBytes(f.Bytes())
	if err != nil {
		fmt.Println("decode error:", err)
		return
	}
	fmt.Println(decoded.MayContain([]byte("item-42")), decoded.IsEmpty(), decoded.Added())
	// Output: true false 100
}
