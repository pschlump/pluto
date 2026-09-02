/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lru_ts_test

import (
	"testing"

	"github.com/pschlump/pluto/lru"
	"github.com/pschlump/pluto/lru_ts"
)

// The uncontended benchmarks mirror the plain package's, plus the
// plain package itself inlined for the lock-overhead comparison (run
// with -benchtime for stable numbers; the README reports measured
// deltas).  The contended forms use RunParallel — `go test -bench .
// -cpu 8` runs them with exactly 8 goroutines.

// BenchmarkGetHit measures a hit on a 1000-entry cache (the write
// lock — Get re-marks recency).
func BenchmarkGetHit(b *testing.B) {
	c := lru_ts.NewLru[int, int](1000)
	for i := 0; i < 1000; i++ {
		c.Put(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(i % 1000)
	}
}

// BenchmarkPlainGetHit is BenchmarkGetHit against the plain package —
// the difference is the twin's lock overhead.
func BenchmarkPlainGetHit(b *testing.B) {
	c := lru.NewLru[int, int](1000)
	for i := 0; i < 1000; i++ {
		c.Put(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(i % 1000)
	}
}

// BenchmarkPut measures half updates, half inserts with eviction.
func BenchmarkPut(b *testing.B) {
	c := lru_ts.NewLru[int, int](1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Put(i%2000, i)
	}
}

// BenchmarkPlainPut is BenchmarkPut against the plain package.
func BenchmarkPlainPut(b *testing.B) {
	c := lru.NewLru[int, int](1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Put(i%2000, i)
	}
}

// BenchmarkGetMiss measures the not-found path (the write lock is
// taken even though nothing is re-marked — see the package doc).
func BenchmarkGetMiss(b *testing.B) {
	c := lru_ts.NewLru[int, int](1000)
	for i := 0; i < 1000; i++ {
		c.Put(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(1000 + i%1000)
	}
}

// BenchmarkGetHitContended is the shared-cache read path under
// RunParallel — goroutine count = GOMAXPROCS (use -cpu 8 for the
// note's 8-goroutine shape).  Every Get takes the write lock, so this
// is the pessimistic contention number.
func BenchmarkGetHitContended(b *testing.B) {
	c := lru_ts.NewLru[int, int](1000)
	for i := 0; i < 1000; i++ {
		c.Put(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Get(i % 1000)
			i++
		}
	})
}

// BenchmarkPutContended is the shared-cache write path under
// RunParallel.
func BenchmarkPutContended(b *testing.B) {
	c := lru_ts.NewLru[int, int](1000)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Put(i%2000, i)
			i++
		}
	})
}

// BenchmarkPeekContended is the read-lock path under RunParallel —
// Peek is the true read, so readers scale where Get cannot.
func BenchmarkPeekContended(b *testing.B) {
	c := lru_ts.NewLru[int, int](1000)
	for i := 0; i < 1000; i++ {
		c.Put(i, i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Peek(i % 1000)
			i++
		}
	})
}
