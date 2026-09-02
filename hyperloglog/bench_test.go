/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hyperloglog

import (
	"encoding/binary"
	"testing"
)

// benchFill returns a sketch with n distinct elements.
func benchFill(n uint64) *Hll {
	h := NewHll()
	var key [8]byte
	for i := uint64(0); i < n; i++ {
		binary.LittleEndian.PutUint64(key[:], i)
		h.Add(key[:])
	}
	return h
}

// BenchmarkAdd8 measures Add of an 8-byte element with a fresh key each
// iteration (the changed-register path).  Target from the request note:
// ≥ 10M ops/s single goroutine.
func BenchmarkAdd8(b *testing.B) {
	h := NewHll()
	var key [8]byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		binary.LittleEndian.PutUint64(key[:], uint64(i))
		h.Add(key[:])
	}
}

// BenchmarkAdd64 measures Add of a 64-byte element (two full stripes
// plus tail in the hash).
func BenchmarkAdd64(b *testing.B) {
	h := NewHll()
	key := make([]byte, 64)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		binary.LittleEndian.PutUint64(key[:], uint64(i))
		h.Add(key)
	}
}

// BenchmarkCountCached measures Count on an unchanged sketch: the first
// call after a mutation pays the O(m) estimate, the rest read the cache.
func BenchmarkCountCached(b *testing.B) {
	h := benchFill(100_000)
	h.Count()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		h.Count()
	}
}

// BenchmarkCountUncached measures the full O(m) estimate: the cache is
// invalidated before every call.  (Invalidating through Add instead
// would silently degrade to the cached path once the registers
// saturate and fresh elements stop raising any rank.)
func BenchmarkCountUncached(b *testing.B) {
	h := benchFill(100_000)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		h.valid.Store(false)
		h.Count()
	}
}

// BenchmarkMerge measures a merge into an already-superset sketch (the
// full O(m) scan, no register writes).
func BenchmarkMerge(b *testing.B) {
	h := benchFill(100_000)
	part := benchFill(50_000)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		h.Merge(part)
	}
}
