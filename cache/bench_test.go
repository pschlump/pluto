/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package cache

import (
	"context"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pschlump/pluto/internal/guava_compare"
	"github.com/pschlump/pluto/lru_ts"
	"github.com/pschlump/pluto/tiny_lfu_ts"
)

var cacheBenchmarkResult int
var cacheBenchmarkAtomic atomic.Int64

func applyCacheOperation(value *Cache[string, string], operation guava_compare.CacheOperation) {
	if operation.Op == "put" {
		value.Put(operation.Key, operation.Value)
	} else if _, found := value.GetIfPresent(operation.Key); found {
		cacheBenchmarkResult++
	}
}

func BenchmarkCacheHit(b *testing.B) {
	corpus := readCacheCorpus(b)
	value := New[string, string]()
	for _, operation := range corpus.Operations {
		value.Put(operation.Key, operation.Value)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		applyCacheOperation(value, corpus.Operations[index%len(corpus.Operations)])
	}
}

func BenchmarkCacheEvictingWrite(b *testing.B) {
	corpus := readCacheCorpus(b)
	value := New[string, string](WithMaximumSize[string, string](guava_compare.CacheBenchmarkEvictionCapacity))
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		operation := corpus.Operations[index%len(corpus.Operations)]
		value.Put(operation.Key, operation.Value)
	}
}

func BenchmarkCacheMissLoading(b *testing.B) {
	corpus := readCacheCorpus(b)
	value := NewLoading(func(_ context.Context, key string) (string, error) { return key + "!", nil })
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		key := corpus.Operations[index%len(corpus.Operations)].Key
		value.Invalidate(key)
		if _, err := value.Get(context.Background(), key); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCacheExpiration(b *testing.B) {
	corpus := readCacheCorpus(b)
	clock := newFakeClock()
	value := NewLoading(func(_ context.Context, key string) (string, error) { return key + "!", nil },
		WithExpireAfterWrite[string, string](time.Nanosecond), WithClock[string, string](clock.Now))
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		clock.Advance(2 * time.Nanosecond)
		if _, err := value.Get(context.Background(), corpus.Operations[index%len(corpus.Operations)].Key); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCacheRefresh(b *testing.B) {
	corpus := readCacheCorpus(b)
	clock := newFakeClock()
	value := NewLoading(func(_ context.Context, key string) (string, error) { return key + "!", nil },
		WithRefreshAfterWrite[string, string](time.Nanosecond), WithClock[string, string](clock.Now),
		WithExecutor[string, string](func(operation func()) { operation() }))
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		clock.Advance(2 * time.Nanosecond)
		if _, err := value.Get(context.Background(), corpus.Operations[index%len(corpus.Operations)].Key); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCacheLRUHit(b *testing.B) {
	corpus := readCacheCorpus(b)
	value := lru_ts.NewLru[string, string](corpus.Capacity)
	for _, operation := range corpus.Operations {
		value.Put(operation.Key, operation.Value)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		operation := corpus.Operations[index%len(corpus.Operations)]
		if operation.Op == "put" {
			value.Put(operation.Key, operation.Value)
		} else if _, found := value.Get(operation.Key); found {
			cacheBenchmarkResult++
		}
	}
}

func BenchmarkCacheTinyLFUHit(b *testing.B) {
	corpus := readCacheCorpus(b)
	value := tiny_lfu_ts.NewTinyLfu[string, string](corpus.Capacity)
	for _, operation := range corpus.Operations {
		value.Put(operation.Key, operation.Value)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		operation := corpus.Operations[index%len(corpus.Operations)]
		if operation.Op == "put" {
			value.Put(operation.Key, operation.Value)
		} else if _, found := value.Get(operation.Key); found {
			cacheBenchmarkResult++
		}
	}
}

func benchmarkCacheKeys(b *testing.B, keys []string) {
	b.Helper()
	corpus := readCacheCorpus(b)
	value := New[string, string]()
	for _, operation := range corpus.Operations {
		value.Put(operation.Key, operation.Value)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, found := value.GetIfPresent(keys[index%len(keys)]); found {
			cacheBenchmarkResult++
		}
	}
}

func BenchmarkCacheHitZipf(b *testing.B)  { benchmarkCacheKeys(b, readCacheCorpus(b).ZipfKeys) }
func BenchmarkCacheHitScan(b *testing.B)  { benchmarkCacheKeys(b, readCacheCorpus(b).ScanKeys) }
func BenchmarkCacheHitPhase(b *testing.B) { benchmarkCacheKeys(b, readCacheCorpus(b).PhaseKeys) }

func benchmarkCacheConcurrentHit(b *testing.B, threads int) {
	corpus := readCacheCorpus(b)
	value := New[string, string]()
	for _, operation := range corpus.Operations {
		value.Put(operation.Key, operation.Value)
	}
	previous := runtime.GOMAXPROCS(threads)
	defer runtime.GOMAXPROCS(previous)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pointer *testing.PB) {
		index := 0
		for pointer.Next() {
			if _, found := value.GetIfPresent(corpus.Operations[index%len(corpus.Operations)].Key); found {
				cacheBenchmarkAtomic.Add(1)
			}
			index++
		}
	})
}

func BenchmarkCacheConcurrentHit8(b *testing.B)  { benchmarkCacheConcurrentHit(b, 8) }
func BenchmarkCacheConcurrentHit16(b *testing.B) { benchmarkCacheConcurrentHit(b, 16) }

func BenchmarkCacheCoalescedMiss8(b *testing.B) {
	value := NewLoading(func(_ context.Context, key string) (string, error) { return key + "!", nil })
	previous := runtime.GOMAXPROCS(8)
	defer runtime.GOMAXPROCS(previous)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pointer *testing.PB) {
		for pointer.Next() {
			value.Invalidate("shared")
			if _, err := value.Get(context.Background(), "shared"); err != nil {
				panic(err)
			}
		}
	})
	stats := value.Stats()
	b.ReportMetric(float64(stats.Loads), "loads")
	if stats.Hits+stats.Misses > 0 {
		b.ReportMetric(float64(stats.Hits)/float64(stats.Hits+stats.Misses), "hit-rate")
	}
}

func readCacheCorpus(b *testing.B) guava_compare.CacheCorpus {
	b.Helper()
	corpus, err := guava_compare.ReadCacheCorpus(filepath.Join("..", "internal", "guava_compare", "testdata", "cache.corpus.json"))
	if err != nil {
		b.Fatal(err)
	}
	return corpus
}
