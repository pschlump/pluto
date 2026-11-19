/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tiny_lfu_test

import (
	"runtime"
	"testing"

	"github.com/pschlump/pluto/lru"
	"github.com/pschlump/pluto/tiny_lfu"
	"github.com/pschlump/pluto/tiny_lfu_ts"
)

const (
	benchmarkCapacity = 32768
	benchmarkKeys     = 2 * benchmarkCapacity
	memoryEntries     = 25000
	memoryCapacity    = 50000
)

type benchmarkCache interface {
	Get(int) (bool, bool)
	Put(int, bool)
	Clear()
}

var benchmarkHit bool

// BenchmarkCaffeineComparison mirrors testdata/CaffeineBenchmark.java. The
// single-threaded cases use tiny_lfu and the eight-thread cases tiny_lfu_ts.
func BenchmarkCaffeineComparison(b *testing.B) {
	for _, test := range []struct {
		name       string
		threadSafe bool
		concurrent bool
		evicting   bool
		mixed      bool
		write      bool
	}{
		{name: "get1"},
		{name: "putUpdate1", write: true},
		{name: "get8", threadSafe: true, concurrent: true},
		{name: "putUpdate8", threadSafe: true, concurrent: true, write: true},
		{name: "mixed8", threadSafe: true, concurrent: true, mixed: true},
		{name: "putEvict1", evicting: true, write: true},
	} {
		b.Run(test.name, func(b *testing.B) {
			keys := make([]int, benchmarkKeys)
			for i := range keys {
				keys[i] = i
			}
			var cache benchmarkCache
			if test.threadSafe {
				cache = tiny_lfu_ts.NewTinyLfu[int, bool](benchmarkCapacity)
			} else {
				cache = tiny_lfu.NewTinyLfu[int, bool](benchmarkCapacity)
			}
			for i := range benchmarkCapacity {
				cache.Put(i, true)
			}

			operation := func(cache benchmarkCache, key int) {
				_, benchmarkHit = cache.Get(key)
			}
			if test.write {
				operation = func(cache benchmarkCache, key int) {
					cache.Put(key, true)
				}
			}
			if test.mixed {
				operation = func(cache benchmarkCache, key int) {
					if key%20 == 0 {
						cache.Put(key, true)
					} else {
						_, benchmarkHit = cache.Get(key)
					}
				}
			}

			b.ResetTimer()
			if test.concurrent {
				previous := runtime.GOMAXPROCS(8)
				defer runtime.GOMAXPROCS(previous)
				b.RunParallel(func(pb *testing.PB) {
					index := 0
					for pb.Next() {
						keyIndex := index
						index++
						if test.evicting {
							keyIndex &= benchmarkKeys - 1
						} else {
							keyIndex &= benchmarkCapacity - 1
						}
						operation(cache, keys[keyIndex])
					}
				})
				return
			}
			for i := range b.N {
				keyIndex := i
				if test.evicting {
					keyIndex &= benchmarkKeys - 1
				} else {
					keyIndex &= benchmarkCapacity - 1
				}
				operation(cache, keys[keyIndex])
			}
		})
	}
}

// BenchmarkMemoryOverhead reports retained cache overhead after Clear, matching
// Caffeine's MemoryBenchmark methodology. Integer keys and values have no
// per-object allocation in Go, so no application-object subtraction is needed.
func BenchmarkMemoryOverhead(b *testing.B) {
	for _, implementation := range []struct {
		name string
		new  func() benchmarkCache
	}{
		{name: "lru", new: func() benchmarkCache { return lru.NewLru[int, bool](memoryCapacity) }},
		{name: "tiny_lfu", new: func() benchmarkCache { return tiny_lfu.NewTinyLfu[int, bool](memoryCapacity) }},
		{name: "tiny_lfu_ts", new: func() benchmarkCache { return tiny_lfu_ts.NewTinyLfu[int, bool](memoryCapacity) }},
	} {
		b.Run(implementation.name, func(b *testing.B) {
			for range b.N {
				cache := implementation.new()
				for i := range memoryEntries {
					cache.Put(i, true)
				}
				runtime.GC()
				var populated runtime.MemStats
				runtime.ReadMemStats(&populated)
				cache.Clear()
				runtime.GC()
				var base runtime.MemStats
				runtime.ReadMemStats(&base)
				overhead := int64(populated.HeapAlloc) - int64(base.HeapAlloc)
				b.ReportMetric(float64(base.HeapAlloc), "base-bytes")
				b.ReportMetric(float64(overhead)/memoryEntries, "overhead-bytes/entry")
				runtime.KeepAlive(cache)
			}
		})
	}
}
