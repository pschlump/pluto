/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu

import "testing"

// BenchmarkTouch measures one access of a hot key at Redis's default
// settings — the per-command cost of allkeys-lfu bookkeeping.
func BenchmarkTouch(b *testing.B) {
	l := NewLfu[string](DefaultLogFactor, DefaultDecayTime)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Touch("hot-key")
	}
}

// BenchmarkTouchSpread touches 10k distinct keys so the hash_grow
// backing store runs with realistic cache pressure and occasional
// growth behind the steady-state probes.
func BenchmarkTouchSpread(b *testing.B) {
	l := NewLfu[int](DefaultLogFactor, DefaultDecayTime)
	for i := 0; i < 10_000; i++ {
		l.Touch(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Touch(i % 10_000)
	}
}

// BenchmarkCounter is the OBJECT FREQ read.
func BenchmarkCounter(b *testing.B) {
	l := NewLfu[string](DefaultLogFactor, DefaultDecayTime)
	l.Touch("hot-key")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Counter("hot-key")
	}
}

// BenchmarkAdd is the key-creation path (SET on a new key).
func BenchmarkAdd(b *testing.B) {
	l := NewLfu[int](DefaultLogFactor, DefaultDecayTime)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Add(i)
	}
}

// BenchmarkMorrisIncr is the standalone counter, including the global
// rand draw.
func BenchmarkMorrisIncr(b *testing.B) {
	c := NewMorrisCounter()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Incr(DefaultLogFactor)
	}
}
