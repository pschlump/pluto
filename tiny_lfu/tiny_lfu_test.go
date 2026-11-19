/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tiny_lfu

import (
	"reflect"
	"testing"
)

func request(c *TinyLfu[int, int], key int) {
	requestValue(c, key, key)
}

func requestValue(c *TinyLfu[int, int], key, value int) {
	if _, ok := c.Get(key); !ok {
		c.Put(key, value)
	}
}

func requireStats(t *testing.T, c *TinyLfu[int, int], hits, misses, evictions uint64) {
	t.Helper()
	stats := c.Stats()
	if stats.Hits != hits || stats.Misses != misses || stats.Evictions != evictions {
		t.Errorf("Stats = %+v, want hits=%d misses=%d evictions=%d", stats, hits, misses, evictions)
	}
}

func requireHitMiss(t *testing.T, c *TinyLfu[int, int], hits, misses uint64) {
	t.Helper()
	stats := c.Stats()
	if stats.Hits != hits || stats.Misses != misses {
		t.Errorf("Hits/Misses = %d/%d, want %d/%d", stats.Hits, stats.Misses, hits, misses)
	}
}

func checkCacheInvariants(t *testing.T, c *TinyLfu[int, int]) {
	t.Helper()
	seen := make(map[*entry[int, int]]region)
	weight := make(map[region]int)
	for _, head := range []*entry[int, int]{&c.window, &c.probation, &c.protected} {
		previous := head
		for node := head.next; node != head; node = node.next {
			if node == nil {
				t.Fatal("nil node in non-sentinel list position")
			}
			if node.prev != previous {
				t.Fatalf("broken prev link for key %d", node.key)
			}
			if _, duplicate := seen[node]; duplicate {
				t.Fatalf("key %d occurs in multiple regions", node.key)
			}
			seen[node] = node.region
			weight[node.region] += node.weight
			previous = node
		}
		if head.prev != previous {
			t.Fatal("list tail does not point to the last entry")
		}
	}
	if len(seen) != len(c.data) {
		t.Errorf("linked entries = %d, map entries = %d", len(seen), len(c.data))
	}
	total := 0
	for key, node := range c.data {
		if _, ok := seen[node]; !ok {
			t.Errorf("key %d is mapped but not linked", key)
		}
		total += node.weight
	}
	if c.sizeWindow != weight[windowRegion] || c.sizeProtected != weight[protectedRegion] || c.sizeData != total {
		t.Errorf("weights = window %d/%d protected %d/%d total %d/%d",
			c.sizeWindow, weight[windowRegion], c.sizeProtected, weight[protectedRegion], c.sizeData, total)
	}
	if c.sizeData > c.maximumSize {
		t.Errorf("Size = %d exceeds Capacity = %d", c.sizeData, c.maximumSize)
	}
}

func TestTinyLfuGetPutDeleteClear(t *testing.T) {
	c := NewTinyLfu[string, int](3)
	if _, ok := c.Get("missing"); ok {
		t.Error("Get on an empty cache found a value")
	}
	c.Put("a", 1)
	c.Put("b", 2)
	if got, ok := c.Get("a"); !ok || got != 1 {
		t.Errorf("Get(a) = (%d, %v), want (1, true)", got, ok)
	}
	c.Put("a", 10)
	if got, ok := c.Peek("a"); !ok || got != 10 {
		t.Errorf("Peek(a) after update = (%d, %v), want (10, true)", got, ok)
	}
	if c.Len() != 2 || c.Size() != 2 || c.Capacity() != 3 {
		t.Errorf("Len/Size/Capacity = %d/%d/%d, want 2/2/3", c.Len(), c.Size(), c.Capacity())
	}
	if !c.Delete("a") || c.Delete("a") {
		t.Error("Delete did not report present then absent")
	}
	c.Clear()
	if c.Len() != 0 || c.Capacity() != 3 {
		t.Errorf("Len/Capacity after Clear = %d/%d, want 0/3", c.Len(), c.Capacity())
	}
	c.Put("z", 26)
	if got, ok := c.Get("z"); !ok || got != 26 {
		t.Error("cache was not reusable after Clear")
	}
	stats := c.Stats()
	if stats.Hits != 2 || stats.Misses != 1 {
		t.Errorf("Hits/Misses = %d/%d, want 2/1", stats.Hits, stats.Misses)
	}
}

func TestTinyLfuNilAndZeroValue(t *testing.T) {
	var zero TinyLfu[int, int]
	var nilCache *TinyLfu[int, int]
	for _, c := range []*TinyLfu[int, int]{&zero, nilCache} {
		if _, ok := c.Get(1); ok {
			t.Error("Get found a value in an empty cache")
		}
		if _, ok := c.Peek(1); ok {
			t.Error("Peek found a value in an empty cache")
		}
		if c.Delete(1) || c.Len() != 0 || c.Size() != 0 || c.Capacity() != 0 || c.Frequency(1) != 0 {
			t.Error("empty cache returned nonzero state")
		}
		c.Clear()
		if c.Stats() != (Stats{}) {
			t.Error("empty cache returned nonzero stats")
		}
	}
	requirePanic(t, func() { zero.Put(1, 1) })
	requirePanic(t, func() { nilCache.Put(1, 1) })
	requirePanic(t, func() { NewTinyLfu[int, int](0) })
	requirePanic(t, func() { NewTinyLfuWeighted[int, int](1, nil) })
}

func TestTinyLfuDefaultSegments(t *testing.T) {
	for _, test := range []struct {
		capacity, window, protected int
	}{
		{1, 1, 0},
		{3, 1, 1},
		{100, 1, 79},
		{101, 2, 79},
	} {
		c := NewTinyLfu[int, int](test.capacity)
		if c.maxWindow != test.window || c.maxProtected != test.protected {
			t.Errorf("capacity %d: window/protected = %d/%d, want %d/%d",
				test.capacity, c.maxWindow, c.maxProtected, test.window, test.protected)
		}
	}
}

func TestTinyLfuAdmissionPrefersFrequencyAndVictimOnTie(t *testing.T) {
	c := NewTinyLfuWithSegments[int, int](2, 0, 0)
	if c.admit(3, 1) {
		t.Error("candidate won a frequency tie")
	}
	c.sketch.Increment(3)
	if !c.admit(3, 1) {
		t.Error("more frequent candidate was rejected")
	}
	for range 15 {
		c.sketch.Increment(1)
		if c.admit(3, 1) {
			t.Error("candidate beat a more frequent victim")
		}
	}
}

func TestTinyLfuEvictionOrderFromCaffeine(t *testing.T) {
	c := NewTinyLfu[int, int](10)
	for i := range 10 {
		c.Put(i, -i)
	}
	want := []int{9, 0, 1, 2, 3, 4, 5, 6, 7, 8}
	if got := evictionOrder(c); !reflect.DeepEqual(got, want) {
		t.Fatalf("initial eviction order = %v, want %v", got, want)
	}
	for _, key := range []int{0, 1, 2} {
		if _, ok := c.Get(key); !ok {
			t.Fatalf("Get(%d) missed", key)
		}
	}
	want = []int{9, 3, 4, 5, 6, 7, 8, 0, 1, 2}
	if got := evictionOrder(c); !reflect.DeepEqual(got, want) {
		t.Fatalf("reordered eviction order = %v, want %v", got, want)
	}
	for _, key := range []int{10, 11, 12} {
		c.Put(key, key)
	}
	want = []int{12, 3, 4, 5, 6, 7, 8, 0, 1, 2}
	if got := evictionOrder(c); !reflect.DeepEqual(got, want) {
		t.Fatalf("eviction order after scan = %v, want %v", got, want)
	}
	for _, key := range []int{6, 7, 8} {
		c.Get(key)
	}
	for _, key := range []int{13, 14, 15} {
		c.Put(key, key)
	}
	want = []int{15, 3, 4, 5, 0, 1, 2, 6, 7, 8}
	if got := evictionOrder(c); !reflect.DeepEqual(got, want) {
		t.Fatalf("final eviction order = %v, want %v", got, want)
	}
	if got := c.Stats().Evictions; got != 6 {
		t.Errorf("Evictions = %d, want 6", got)
	}
	checkCacheInvariants(t, c)
}

func evictionOrder(c *TinyLfu[int, int]) []int {
	var keys []int
	for _, head := range []*entry[int, int]{&c.window, &c.probation, &c.protected} {
		for node := head.next; node != head; node = node.next {
			keys = append(keys, node.key)
		}
	}
	return keys
}

func TestWeightedOversizedEntryIsDiscarded(t *testing.T) {
	c := NewTinyLfuWeighted[int, int](100, func(_, value int) int { return value })
	requestValue(c, 1, 101)
	requestValue(c, 1, 101)
	checkCacheInvariants(t, c)
	requireStats(t, c, 0, 2, 2)
}

func TestWeightedOversizedCandidateDoesNotDisplaceResidents(t *testing.T) {
	c := NewTinyLfuWeighted[int, int](100, func(_, value int) int { return value })
	for key := 1; key <= 9; key++ {
		requestValue(c, key, 10)
		requestValue(c, key, 10)
	}
	misses := c.Stats().Misses
	requestValue(c, 100, 101)
	checkCacheInvariants(t, c)
	if got := c.Stats().Evictions; got != 1 {
		t.Errorf("Evictions after oversized candidate = %d, want 1", got)
	}
	requestValue(c, 100, 101)
	if got := c.Stats().Misses; got != misses+2 {
		t.Errorf("Misses = %d, want %d", got, misses+2)
	}
}

func TestWeightedOversizedPromotionKeepsProtectedIntact(t *testing.T) {
	c := NewTinyLfuWeighted[int, int](100, func(_, value int) int { return value })
	for key := 1; key <= 2; key++ {
		requestValue(c, key, 10)
		requestValue(c, key, 10)
	}
	requestValue(c, 9, 80)
	requestValue(c, 9, 80)
	requestValue(c, 20, 10)
	requestValue(c, 1, 10)
	checkCacheInvariants(t, c)
	requireStats(t, c, 4, 4, 1)
}

func TestWeightedUpdateEvictsBackUnderCapacity(t *testing.T) {
	c := NewTinyLfuWeighted[int, int](100, func(_, value int) int { return value })
	for key := 1; key <= 6; key++ {
		requestValue(c, key, 15)
		requestValue(c, key, 15)
	}
	c.Put(1, 60)
	checkCacheInvariants(t, c)
	if c.Stats().Evictions < 1 {
		t.Error("growing a resident did not evict back under capacity")
	}
}

func TestUnweightedCacheIgnoresValueAsWeight(t *testing.T) {
	c := NewTinyLfu[int, int](3)
	for key := 1; key <= 3; key++ {
		requestValue(c, key, 1000)
	}
	for key := 1; key <= 3; key++ {
		requestValue(c, key, 1000)
	}
	checkCacheInvariants(t, c)
	requireStats(t, c, 3, 3, 0)
}

func TestExplicitSegmentSizesWindow(t *testing.T) {
	smallWindow := NewTinyLfuWithSegments[int, int](3, 1, 0)
	largeWindow := NewTinyLfuWithSegments[int, int](3, 3, 0)
	for _, key := range []int{1, 2, 3, 4, 1} {
		request(smallWindow, key)
		request(largeWindow, key)
	}
	checkCacheInvariants(t, smallWindow)
	checkCacheInvariants(t, largeWindow)
	requireStats(t, smallWindow, 1, 4, 1)
	requireStats(t, largeWindow, 0, 5, 2)
}

func TestExplicitSegmentSizesProtected(t *testing.T) {
	withoutProtected := NewTinyLfuWithSegments[int, int](3, 1, 0)
	withProtected := NewTinyLfuWithSegments[int, int](3, 1, 1)
	for _, key := range []int{1, 2, 3, 1, 4, 5, 1} {
		for _, c := range []*TinyLfu[int, int]{withoutProtected, withProtected} {
			if _, ok := c.Get(key); !ok {
				if c.sizeData == c.maximumSize && c.window.next != &c.window {
					clear(c.sketch.table)
					c.sketch.size = 0
					c.sketch.Increment(c.window.next.key)
				}
				c.Put(key, key)
			}
		}
	}
	checkCacheInvariants(t, withoutProtected)
	checkCacheInvariants(t, withProtected)
	requireHitMiss(t, withoutProtected, 1, 6)
	requireHitMiss(t, withProtected, 2, 5)
}

func TestExplicitSegmentSizesRejectInvalidGeometry(t *testing.T) {
	for _, sizes := range [][3]int{{0, 0, 0}, {3, -1, 0}, {3, 0, -1}, {3, 2, 2}} {
		requirePanic(t, func() {
			NewTinyLfuWithSegments[int, int](sizes[0], sizes[1], sizes[2])
		})
	}
}

func TestWeightedCacheRejectsNegativeWeight(t *testing.T) {
	c := NewTinyLfuWeighted[int, int](10, func(_, _ int) int { return -1 })
	requirePanic(t, func() { c.Put(1, 1) })
}

func TestEvictionVetoAndSoftCap(t *testing.T) {
	vetoAll := true
	c := NewTinyLfuFunc[int, int](2, func(_, _ int) bool { return !vetoAll })
	c.Put(1, 1)
	c.Put(2, 2)
	c.Put(3, 3)
	if c.Len() != 3 || c.Size() != 3 {
		t.Fatalf("Len/Size with all entries vetoed = %d/%d, want 3/3", c.Len(), c.Size())
	}
	vetoAll = false
	c.Put(4, 4)
	if c.Len() != 2 || c.Size() != 2 {
		t.Fatalf("Len/Size after veto lifted = %d/%d, want 2/2", c.Len(), c.Size())
	}
	checkCacheInvariants(t, c)
}

func TestEvictionVetoKeepsPinnedResident(t *testing.T) {
	c := NewTinyLfuFunc[int, int](2, func(key, _ int) bool { return key != 1 })
	c.Put(1, 1)
	c.Put(2, 2)
	c.Get(1)
	c.sketch.Increment(2)
	c.Put(3, 3)
	if _, ok := c.Peek(1); !ok {
		t.Error("vetoed resident was evicted")
	}
	checkCacheInvariants(t, c)
}

func TestAllIteration(t *testing.T) {
	c := NewTinyLfu[int, string](4)
	for _, key := range []int{1, 2, 3, 4} {
		c.Put(key, string(rune('a'+key-1)))
	}
	seen := make(map[int]string)
	for key, value := range c.All() {
		seen[key] = value
	}
	if !reflect.DeepEqual(seen, map[int]string{1: "a", 2: "b", 3: "c", 4: "d"}) {
		t.Errorf("All contents = %v", seen)
	}
	count := 0
	for range c.All() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("early-stop iteration yielded %d entries, want 1", count)
	}
	var zero TinyLfu[int, int]
	for range zero.All() {
		t.Error("zero-value All yielded an entry")
	}
}
