/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tiny_lfu_ts_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pschlump/pluto/tiny_lfu_ts"
)

func expectPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("operation did not panic")
		}
	}()
	fn()
}

func TestCacheSemantics(t *testing.T) {
	c := tiny_lfu_ts.NewTinyLfuWithSegments[string, int](3, 1, 1)
	if _, ok := c.Get("missing"); ok {
		t.Error("empty cache returned a value")
	}
	c.Put("a", 1)
	c.Put("b", 2)
	if got, ok := c.Get("a"); !ok || got != 1 {
		t.Errorf("Get(a) = (%d, %v), want (1, true)", got, ok)
	}
	c.Put("a", 10)
	if got, ok := c.Peek("a"); !ok || got != 10 {
		t.Errorf("Peek(a) = (%d, %v), want (10, true)", got, ok)
	}
	if c.Len() != 2 || c.Size() != 2 || c.Capacity() != 3 || c.Frequency("a") < 2 {
		t.Errorf("unexpected cache state: Len=%d Size=%d Capacity=%d Frequency=%d",
			c.Len(), c.Size(), c.Capacity(), c.Frequency("a"))
	}
	if !c.Delete("a") || c.Delete("a") {
		t.Error("Delete did not report present then absent")
	}
	c.Clear()
	if c.Len() != 0 || c.Capacity() != 3 {
		t.Errorf("Len/Capacity after Clear = %d/%d, want 0/3", c.Len(), c.Capacity())
	}
	stats := c.Stats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Errorf("Hits/Misses = %d/%d, want 1/1", stats.Hits, stats.Misses)
	}
}

func TestWeightedCache(t *testing.T) {
	c := tiny_lfu_ts.NewTinyLfuWeighted[int, int](10, func(_, value int) int { return value })
	c.Put(1, 6)
	c.Put(2, 6)
	c.Stats()
	if _, ok := c.Peek(2); ok {
		t.Error("new candidate displaced a same-weight resident on a tie")
	}
	if c.Size() > c.Capacity() {
		t.Errorf("Size = %d exceeds Capacity = %d", c.Size(), c.Capacity())
	}
	c.Put(3, 11)
	c.Stats()
	if _, ok := c.Peek(3); ok || c.Size() > 10 {
		t.Error("oversized entry was retained")
	}
}

func TestNilAndZeroValues(t *testing.T) {
	var cache tiny_lfu_ts.TinyLfu[int, int]
	var nilCache *tiny_lfu_ts.TinyLfu[int, int]
	for _, c := range []*tiny_lfu_ts.TinyLfu[int, int]{&cache, nilCache} {
		if _, ok := c.Get(1); ok {
			t.Error("empty cache Get found a value")
		}
		if _, ok := c.Peek(1); ok {
			t.Error("empty cache Peek found a value")
		}
		if c.Delete(1) || c.Len() != 0 || c.Size() != 0 || c.Capacity() != 0 || c.Frequency(1) != 0 {
			t.Error("empty cache returned nonzero state")
		}
		c.Clear()
		c.Lock()
		c.Unlock()
		if c.Stats() != (tiny_lfu_ts.Stats{}) {
			t.Error("empty cache returned nonzero stats")
		}
	}
	expectPanic(t, func() { cache.Put(1, 1) })
	expectPanic(t, func() { nilCache.Put(1, 1) })
	expectPanic(t, func() { tiny_lfu_ts.NewTinyLfu[int, int](0) })
	expectPanic(t, func() { tiny_lfu_ts.NewTinyLfuWithSegments[int, int](3, 2, 2) })
	expectPanic(t, func() { tiny_lfu_ts.NewTinyLfuWeighted[int, int](1, nil) })

	var sketch tiny_lfu_ts.FrequencySketch[int]
	var nilSketch *tiny_lfu_ts.FrequencySketch[int]
	sketch.Increment(1)
	if sketch.Frequency(1) != 0 || nilSketch.Frequency(1) != 0 {
		t.Error("uninitialized sketch returned a nonzero frequency")
	}
	nilSketch.Increment(1)
	sketch.Lock()
	sketch.NlEnsureCapacity(10)
	sketch.NlIncrement(1)
	if got := sketch.NlFrequency(1); got != 1 {
		t.Errorf("NlFrequency = %d, want 1", got)
	}
	sketch.Unlock()
	nilSketch.Lock()
	nilSketch.Unlock()
	expectPanic(t, func() { nilSketch.EnsureCapacity(1) })
	expectPanic(t, func() { tiny_lfu_ts.NewFrequencySketch[int](-1) })
	expectPanic(t, func() { tiny_lfu_ts.NewFrequencySketchWithHash[int](1, nil) })
}

func TestLockNlCompound(t *testing.T) {
	c := tiny_lfu_ts.NewTinyLfu[int, int](2)
	c.Lock()
	c.NlPut(1, 10)
	c.NlPut(2, 20)
	if _, ok := c.NlGet(1); !ok {
		c.Unlock()
		t.Fatal("NlGet did not find NlPut's entry")
	}
	c.NlDelete(2)
	got := c.NlLen()
	capacity := c.NlCapacity()
	size := c.NlSize()
	frequency := c.NlFrequency(1)
	stats := c.NlStats()
	c.Unlock()
	if got != 1 || capacity != 2 || size != 1 || frequency < 2 || stats.Hits != 1 {
		t.Errorf("compound result = len %d capacity %d size %d frequency %d stats %+v",
			got, capacity, size, frequency, stats)
	}
	c.Lock()
	c.NlClear()
	c.Unlock()
	if c.Len() != 0 {
		t.Error("NlClear did not clear the cache")
	}
}

func TestEvictionVetoAndSnapshotIteration(t *testing.T) {
	var vetoAll atomic.Bool
	vetoAll.Store(true)
	c := tiny_lfu_ts.NewTinyLfuFunc[int, int](2, func(_, _ int) bool { return !vetoAll.Load() })
	c.Put(1, 1)
	c.Put(2, 2)
	c.Put(3, 3)
	if c.Len() != 3 {
		t.Fatalf("Len with every entry vetoed = %d, want 3", c.Len())
	}
	vetoAll.Store(false)
	c.Put(4, 4)
	c.Stats()
	if c.Len() != 2 {
		t.Fatalf("Len after veto lifted = %d, want 2", c.Len())
	}
	for range c.All() {
		c.Put(9, 9)
	}
	if _, ok := c.Peek(9); !ok {
		t.Error("mutation during snapshot iteration was not visible afterward")
	}
}

func TestConcurrentSameKey(t *testing.T) {
	c := tiny_lfu_ts.NewTinyLfu[int, int](8)
	const workers = 8
	const perWorker = 500
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perWorker {
				c.Put(1, 42)
				if value, ok := c.Get(1); !ok || value != 42 {
					t.Errorf("Get(1) = (%d, %v), want (42, true)", value, ok)
					return
				}
				c.Peek(1)
				c.Frequency(1)
			}
		}()
	}
	wg.Wait()
	stats := c.Stats()
	if stats.Hits != workers*perWorker || stats.Misses != 0 {
		t.Errorf("Hits/Misses = %d/%d, want %d/0", stats.Hits, stats.Misses, workers*perWorker)
	}
	if c.Len() != 1 || c.Frequency(1) != 15 {
		t.Errorf("Len/Frequency = %d/%d, want 1/15", c.Len(), c.Frequency(1))
	}
}

func TestConcurrentMixedOperations(t *testing.T) {
	c := tiny_lfu_ts.NewTinyLfu[int, int](64)
	const workers = 8
	const perWorker = 1000
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWorker {
				key := (worker*31 + i) % 256
				switch i % 6 {
				case 0:
					c.Put(key, i)
				case 1:
					c.Get(key)
				case 2:
					c.Peek(key)
				case 3:
					c.Delete(key)
				case 4:
					c.Frequency(key)
				case 5:
					c.Stats()
				}
			}
		}()
	}
	wg.Wait()
	c.Stats()
	if c.Len() > 64 || c.Size() != c.Len() {
		t.Errorf("final Len/Size = %d/%d, want equal values at most 64", c.Len(), c.Size())
	}
}

func TestConcurrentWeightedOperations(t *testing.T) {
	c := tiny_lfu_ts.NewTinyLfuWeighted[int, int](32, func(_, value int) int { return value })
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 500 {
				key := (worker*17 + i) % 128
				c.Put(key, 1+i%4)
				c.Get(key)
			}
		}()
	}
	wg.Wait()
	c.Stats()
	if c.Size() > 32 {
		t.Errorf("final Size = %d, want at most 32", c.Size())
	}
}

func TestConcurrentSketch(t *testing.T) {
	sketch := tiny_lfu_ts.NewFrequencySketch[int](100)
	const workers = 8
	const perWorker = 1000
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWorker {
				sketch.Increment(worker)
				sketch.Frequency(i % workers)
			}
		}()
	}
	wg.Wait()
	for worker := range workers {
		if got := sketch.Frequency(worker); got != 15 {
			t.Errorf("Frequency(%d) = %d, want saturated count 15", worker, got)
		}
	}
}

func TestWriteBufferOverflowKeepsLatestValue(t *testing.T) {
	c := tiny_lfu_ts.NewTinyLfu[int, int](32)
	for value := range 5000 {
		c.Put(42, value)
	}
	if got, ok := c.Peek(42); !ok || got != 4999 {
		t.Fatalf("Peek after write overflow = (%d, %v), want (4999, true)", got, ok)
	}
	stats := c.Stats()
	if c.Len() != 1 || c.Size() != 1 || c.Frequency(42) < 1 || c.Frequency(42) > 15 {
		t.Errorf("drained state = Len %d Size %d Frequency %d", c.Len(), c.Size(), c.Frequency(42))
	}
	if stats.Evictions != 0 {
		t.Errorf("same-key writes caused %d evictions, want 0", stats.Evictions)
	}
}

func TestStaleEventsCannotResurrectDeletedKey(t *testing.T) {
	c := tiny_lfu_ts.NewTinyLfu[int, int](8)
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 1000 {
				c.Put(42, worker*1000+i)
				if i%3 == 0 {
					c.Delete(42)
				}
			}
		}()
	}
	wg.Wait()
	c.Delete(42)
	c.Stats()
	if _, ok := c.Peek(42); ok {
		t.Error("a stale queued write resurrected the deleted key")
	}
}

func TestClearExcludesConcurrentWriters(t *testing.T) {
	c := tiny_lfu_ts.NewTinyLfu[int, int](32)
	var wg sync.WaitGroup
	for worker := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 1000 {
				c.Put(worker*1000+i, i)
			}
		}()
	}
	wg.Wait()
	c.Clear()
	c.Stats()
	if c.Len() != 0 || c.Size() != 0 {
		t.Errorf("Clear left Len=%d Size=%d, want 0/0", c.Len(), c.Size())
	}
	for range c.All() {
		t.Error("All yielded an entry after Clear")
	}
}
