/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package cache

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	lock sync.Mutex
	now  time.Time
}

func newFakeClock() *fakeClock      { return &fakeClock{now: time.Unix(0, 0)} }
func (c *fakeClock) Now() time.Time { c.lock.Lock(); defer c.lock.Unlock(); return c.now }
func (c *fakeClock) Advance(duration time.Duration) {
	c.lock.Lock()
	c.now = c.now.Add(duration)
	c.lock.Unlock()
}

func TestCacheLRUWeightAndNotifications(t *testing.T) {
	events := []Notification[string, int]{}
	value := New[string, int](
		WithMaximumSize[string, int](2),
		WithRemovalListener[string, int](func(event Notification[string, int]) {
			events = append(events, event)
		}),
	)
	value.Put("a", 1)
	value.Put("b", 2)
	if got, found := value.GetIfPresent("a"); !found || got != 1 {
		t.Fatal(got, found)
	}
	value.Put("c", 3)
	if _, found := value.Peek("b"); found {
		t.Fatal("least-recently-used entry not evicted")
	}
	if value.Len() != 2 {
		t.Fatal(value.Len())
	}
	value.Put("a", 9)
	value.Invalidate("c")
	causes := []RemovalCause{}
	for _, event := range events {
		causes = append(causes, event.Cause)
	}
	if !reflect.DeepEqual(causes, []RemovalCause{Size, Replaced, Explicit}) {
		t.Fatal(causes)
	}

	weighted := New[string, string](
		WithMaximumWeight[string, string](3),
		WithWeigher[string, string](func(_ string, value string) int64 { return int64(len(value)) }),
	)
	weighted.Put("a", "aa")
	weighted.Put("b", "bb")
	if weighted.Len() != 1 {
		t.Fatal("weight eviction", weighted.Snapshot())
	}
	if got, _ := weighted.Peek("b"); got != "bb" {
		t.Fatal(got)
	}
}

func TestCacheExpirationBoundaries(t *testing.T) {
	clock := newFakeClock()
	write := New[string, int](WithExpireAfterWrite[string, int](10), WithClock[string, int](clock.Now))
	write.Put("a", 1)
	clock.Advance(9)
	if _, found := write.Peek("a"); !found {
		t.Fatal("expired before boundary")
	}
	clock.Advance(1)
	if _, found := write.Peek("a"); found {
		t.Fatal("not expired at boundary")
	}

	access := New[string, int](WithExpireAfterAccess[string, int](10), WithClock[string, int](clock.Now))
	access.Put("a", 1)
	clock.Advance(9)
	if _, found := access.GetIfPresent("a"); !found {
		t.Fatal("access entry expired early")
	}
	clock.Advance(9)
	if _, found := access.Peek("a"); !found {
		t.Fatal("Peek changed access time")
	}
	clock.Advance(1)
	if _, found := access.Peek("a"); found {
		t.Fatal("access expiration boundary")
	}
}

func TestCacheAtomicOperationsZeroValuesAndListenerReentry(t *testing.T) {
	value := New[string, int]()
	if !value.PutIfAbsent("a", 0) || value.PutIfAbsent("a", 2) {
		t.Fatal("PutIfAbsent")
	}
	if got, found := value.GetIfPresent("a"); !found || got != 0 {
		t.Fatal(got, found)
	}
	if !value.Replace("a", 3) || value.Replace("missing", 4) {
		t.Fatal("Replace")
	}
	if value.DeleteValue("a", 2, func(a, b int) bool { return a == b }) {
		t.Fatal("deleted unequal value")
	}
	if !value.DeleteValue("a", 3, func(a, b int) bool { return a == b }) {
		t.Fatal("DeleteValue")
	}

	var reentrant *Cache[string, int]
	listenerCalls := 0
	reentrant = New[string, int](WithRemovalListener[string, int](func(Notification[string, int]) {
		listenerCalls++
		reentrant.Len()
	}))
	reentrant.Put("a", 1)
	reentrant.InvalidateAll()
	if listenerCalls != 1 || !reentrant.IsEmpty() {
		t.Fatal(listenerCalls)
	}
}

func TestCacheStatsAndSnapshot(t *testing.T) {
	value := New[string, int]()
	value.Put("a", 1)
	value.GetIfPresent("a")
	value.GetIfPresent("missing")
	stats := value.Stats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Fatal(stats)
	}
	entries := value.Snapshot()
	if len(entries) != 1 || entries[0].Key != "a" || entries[0].Value != 1 {
		t.Fatal(entries)
	}
	entries[0].Value = 99
	if got, _ := value.Peek("a"); got != 1 {
		t.Fatal("snapshot aliases cache")
	}
	value.ClearStats()
	if value.Stats() != (Stats{}) {
		t.Fatal("ClearStats")
	}
}

func TestCacheZeroCapacity(t *testing.T) {
	value := New[string, int](WithMaximumSize[string, int](0))
	value.Put("a", 1)
	if value.Len() != 0 {
		t.Fatal("zero-capacity cache retained value")
	}
}
