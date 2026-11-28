/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lfu_ts

import "testing"

// BenchmarkTouch measures one locked access of a hot key at Redis's
// default settings.
func BenchmarkTouch(b *testing.B) {
	l := NewLfu[string](DefaultLogFactor, DefaultDecayTime)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Touch("hot-key")
	}
}

// BenchmarkTouchSpread touches 10k distinct present keys.
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

// BenchmarkTouchAbsent is the first-ever Touch of a key on a steady
// 2048-key table (touch an absent key, delete it again — no growth).
func BenchmarkTouchAbsent(b *testing.B) {
	l := NewLfu[int](DefaultLogFactor, DefaultDecayTime)
	for i := 0; i < 2048; i++ {
		l.Touch(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k := 2048 + i&2047
		l.Touch(k)
		l.Delete(k)
	}
}
