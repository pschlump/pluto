/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tiny_lfu

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/pschlump/pluto/lru"
)

// BenchmarkDiskPolicyHitRate compares block-cache hit rates on B+tree-like and
// hash-index-like access paths. Each case replays identical requests to both
// policies and reports hits as a percentage.
func BenchmarkDiskPolicyHitRate(b *testing.B) {
	for _, test := range []struct {
		name     string
		capacity int
		access   func() []int
	}{
		{name: "btreeZipf", capacity: 64, access: diskZipfTrace},
		{name: "hashZipf", capacity: 64, access: diskHashZipfTrace},
		{name: "btreeSequential", capacity: 64, access: diskSequentialTrace},
		{name: "hotSetPlusScan", capacity: 64, access: diskHotSetScanTrace},
		{name: "residentLoop", capacity: 64, access: diskResidentLoopTrace},
	} {
		b.Run(test.name, func(b *testing.B) {
			for range b.N {
				trace := test.access()
				lruHits := replayLRU(trace, test.capacity)
				tinyHits := replayTiny(trace, test.capacity)
				b.ReportMetric(100*float64(lruHits)/float64(len(trace)), "lru-hit-%")
				b.ReportMetric(100*float64(tinyHits)/float64(len(trace)), "tiny-hit-%")
			}
		})
	}
}

func diskZipfTrace() []int {
	return diskSkewedTrace(false)
}

func diskHashZipfTrace() []int {
	return diskSkewedTrace(true)
}

func diskSkewedTrace(hashIndex bool) []int {
	const operations = 200000
	random := rand.New(rand.NewSource(42))
	cdf := zipfCDF(0.9, 4096)
	trace := make([]int, 0, operations*3)
	for range operations {
		item := sort.SearchFloat64s(cdf, random.Float64())
		if hashIndex {
			trace = append(trace, item/63, 1000+item)
		} else {
			trace = append(trace, 0, 1+item/1024, 1000+item)
		}
	}
	return trace
}

func diskSequentialTrace() []int {
	const operations = 200000
	trace := make([]int, 0, operations*3)
	for i := range operations {
		item := i % 4096
		trace = append(trace, 0, 1+item/1024, 1000+item)
	}
	return trace
}

func diskHotSetScanTrace() []int {
	const operations = 200000
	trace := make([]int, 0, operations)
	for i := range operations {
		if i%4 == 0 {
			trace = append(trace, i/4%4096+1000)
		} else {
			trace = append(trace, i%16)
		}
	}
	return trace
}

func diskResidentLoopTrace() []int {
	const operations = 200000
	trace := make([]int, operations)
	for i := range trace {
		trace[i] = i % 48
	}
	return trace
}

func replayLRU(trace []int, capacity int) int {
	cache := lru.NewLru[int, int](capacity)
	hits := 0
	for _, key := range trace {
		if _, ok := cache.Get(key); ok {
			hits++
		} else {
			cache.Put(key, key)
		}
	}
	return hits
}

func replayTiny(trace []int, capacity int) int {
	cache := NewTinyLfu[int, int](capacity)
	cache.sketch = NewFrequencySketchWithHash(capacity, func(key int) uint64 {
		return uint64(int64(key))
	})
	hits := 0
	for _, key := range trace {
		if _, ok := cache.Get(key); ok {
			hits++
		} else {
			cache.Put(key, key)
		}
	}
	return hits
}
