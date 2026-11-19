/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tiny_lfu

import (
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/pschlump/pluto/lru"
)

// The paper's synthetic evaluation uses Zipf 0.9 over a large key domain and a
// frequency sample ten times the cache size.
func TestPaperZipfOutperformsLRU(t *testing.T) {
	const capacity = 128
	tiny := NewTinyLfu[int, int](capacity)
	tiny.sketch = NewFrequencySketchWithHash(capacity, func(key int) uint64 {
		return uint64(int64(key))
	})
	plainLRU := lru.NewLru[int, int](capacity)
	random := rand.New(rand.NewSource(42))
	zipf := zipfCDF(0.9, 10000)
	lruHits := uint64(0)
	for range 50000 {
		key := sort.SearchFloat64s(zipf, random.Float64())
		request(tiny, key)
		if _, ok := plainLRU.Get(key); !ok {
			plainLRU.Put(key, key)
		} else {
			lruHits++
		}
	}
	tinyHits := tiny.Stats().Hits
	if tinyHits <= lruHits {
		t.Errorf("TinyLFU hits = %d, LRU hits = %d; want frequency admission to improve the Zipf workload", tinyHits, lruHits)
	}
	checkCacheInvariants(t, tiny)
}

func zipfCDF(exponent float64, n int) []float64 {
	cdf := make([]float64, n)
	total := 0.0
	for i := 1; i <= n; i++ {
		total += 1 / math.Pow(float64(i), exponent)
	}
	cumulative := 0.0
	for i := 1; i <= n; i++ {
		cumulative += 1 / math.Pow(float64(i), exponent)
		cdf[i-1] = cumulative / total
	}
	cdf[n-1] = 1
	return cdf
}

// W-TinyLFU's frequency filter keeps the hot main-cache set resident through
// a one-pass scan; the single LRU window entry remains vulnerable by design.
func TestPaperScanDoesNotPolluteCache(t *testing.T) {
	const hotKeys = 32
	c := NewTinyLfu[int, int](hotKeys)
	c.sketch = NewFrequencySketchWithHash(hotKeys, func(key int) uint64 {
		return uint64(int64(key))
	})
	for range 20 {
		for key := range hotKeys {
			request(c, key)
		}
	}
	for key := 1000; key < 1256; key++ {
		request(c, key)
	}
	hits := 0
	for key := range hotKeys {
		if _, ok := c.Get(key); ok {
			hits++
		}
	}
	if hits < hotKeys-1 {
		t.Errorf("hot keys retained after scan = %d, want at least %d", hits, hotKeys-1)
	}
	checkCacheInvariants(t, c)
}

// Periodic halving lets a new workload displace keys that were popular in an
// earlier phase, matching the paper's freshness mechanism.
func TestPaperAgingAdaptsToNewHotSet(t *testing.T) {
	c := NewTinyLfu[int, int](16)
	c.sketch = NewFrequencySketchWithHash(16, func(key int) uint64 {
		return uint64(int64(key))
	})
	for range 2000 {
		for key := range 8 {
			request(c, key)
		}
	}
	for range 4000 {
		for key := 100; key < 108; key++ {
			request(c, key)
		}
	}
	for key := 100; key < 108; key++ {
		if _, ok := c.Peek(key); !ok {
			t.Errorf("new hot key %d is not resident after the aging epochs", key)
		}
	}
	checkCacheInvariants(t, c)
}
