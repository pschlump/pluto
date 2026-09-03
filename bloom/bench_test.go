/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bloom

import (
	"encoding/binary"
	"testing"
)

// benchFilter returns a filter at its design load: NewBloom(1M, 0.01)
// with 1M distinct 16-byte elements added (the low 8 bytes enumerate
// the element, so the 5M-offset streams below are disjoint misses).
func benchFilter(b *testing.B) *Bloom {
	b.Helper()
	f := NewBloom(1_000_000, 0.01)
	var key [16]byte
	for i := uint64(0); i < 1_000_000; i++ {
		binary.LittleEndian.PutUint64(key[:8], i)
		f.Add(key[:])
	}
	return f
}

// BenchmarkAdd16 measures Add of a 16-byte element (the fresh-key path:
// every element sets bits at the design load).
func BenchmarkAdd16(b *testing.B) {
	f := NewBloom(1_000_000, 0.01)
	var key [16]byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		binary.LittleEndian.PutUint64(key[:8], uint64(i))
		f.Add(key[:])
	}
}

// BenchmarkMayContainHit measures the membership query on a present
// element (all k probes checked).
func BenchmarkMayContainHit(b *testing.B) {
	f := benchFilter(b)
	var key [16]byte
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		binary.LittleEndian.PutUint64(key[:8], uint64(i)%1_000_000)
		f.MayContain(key[:])
	}
}

// BenchmarkMayContainMiss measures the membership query on an absent
// element (the first clear bit exits early — on average partway through
// the probes).
func BenchmarkMayContainMiss(b *testing.B) {
	f := benchFilter(b)
	var key [16]byte
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		binary.LittleEndian.PutUint64(key[:8], uint64(i)+5_000_000)
		f.MayContain(key[:])
	}
}

// BenchmarkTestAndSet measures the combined answer-and-record.
func BenchmarkTestAndSet(b *testing.B) {
	f := NewBloom(1_000_000, 0.01)
	var key [16]byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		binary.LittleEndian.PutUint64(key[:8], uint64(i))
		f.TestAndSet(key[:])
	}
}

// BenchmarkMerge measures a merge into an already-superset filter (the
// full O(m/64) word scan, no bit writes).
func BenchmarkMerge(b *testing.B) {
	f := benchFilter(b)
	part := NewBloom(1_000_000, 0.01)
	var key [16]byte
	for i := uint64(0); i < 500_000; i++ {
		binary.LittleEndian.PutUint64(key[:8], i)
		part.Add(key[:])
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Merge(part)
	}
}

// BenchmarkHashes measures the two frozen hashes in isolation over the
// 16-byte element of the benches above.
func BenchmarkHashes(b *testing.B) {
	var key [16]byte
	binary.LittleEndian.PutUint64(key[:8], 0x0102030405060708)
	b.Run("murmur2-16", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			murmur2(key[:])
		}
	})
	b.Run("superfast-16", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			superFastHash(key[:])
		}
	})
}
