/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package cache

import (
	"iter"
	"math"
	"time"
)

func (c *Cache[K, V]) now() time.Time { return c.clock() }

func (c *Cache[K, V]) expired(entry *cacheEntry[K, V], now time.Time) bool {
	if !entry.linked {
		return false
	}
	if c.expireWrite > 0 && now.Sub(entry.writeTime) >= c.expireWrite {
		return true
	}
	return c.expireAccess > 0 && now.Sub(entry.accessTime) >= c.expireAccess
}

func (c *Cache[K, V]) link(entry *cacheEntry[K, V]) {
	if entry.weight > 0 && c.totalWeight > math.MaxInt64-entry.weight {
		panic("cache: total weight overflow")
	}
	entry.linked = true
	entry.prev = c.tail
	if c.tail == nil {
		c.head = entry
	} else {
		c.tail.next = entry
	}
	c.tail = entry
	c.length++
	c.totalWeight += entry.weight
}

func (c *Cache[K, V]) unlink(entry *cacheEntry[K, V]) {
	if !entry.linked {
		return
	}
	if entry.prev == nil {
		c.head = entry.next
	} else {
		entry.prev.next = entry.next
	}
	if entry.next == nil {
		c.tail = entry.prev
	} else {
		entry.next.prev = entry.prev
	}
	entry.prev, entry.next, entry.linked = nil, nil, false
	c.length--
	c.totalWeight -= entry.weight
}

// touchInline keeps length/weight unchanged while moving an entry to the tail.
func (c *Cache[K, V]) touchInline(entry *cacheEntry[K, V], now time.Time) {
	if !entry.linked {
		return
	}
	entry.accessTime = now
	if c.tail == entry {
		return
	}
	if entry.prev == nil {
		c.head = entry.next
	} else {
		entry.prev.next = entry.next
	}
	if entry.next == nil {
		c.tail = entry.prev
	} else {
		entry.next.prev = entry.prev
	}
	entry.prev, entry.next = c.tail, nil
	if c.tail != nil {
		c.tail.next = entry
	}
	c.tail = entry
}

func (c *Cache[K, V]) removeLocked(entry *cacheEntry[K, V], cause RemovalCause, events []Notification[K, V]) []Notification[K, V] {
	current, found := c.tab.Search(&cacheEntry[K, V]{key: entry.key})
	if !found || current != entry {
		return events
	}
	c.tab.Delete(&cacheEntry[K, V]{key: entry.key})
	c.unlink(entry)
	if cause == Size || cause == Expired {
		c.stats.Evictions++
	}
	if c.listener != nil {
		events = append(events, Notification[K, V]{Key: entry.key, Value: entry.value, Cause: cause})
	}
	return events
}

func (c *Cache[K, V]) cleanExpiredLocked(now time.Time, events []Notification[K, V]) []Notification[K, V] {
	entries := []*cacheEntry[K, V]{}
	for entry := range c.tab.Values() {
		if c.expired(entry, now) {
			entries = append(entries, entry)
		}
	}
	for _, entry := range entries {
		events = c.removeLocked(entry, Expired, events)
	}
	return events
}

func (c *Cache[K, V]) evictLocked(events []Notification[K, V]) []Notification[K, V] {
	for c.head != nil && (c.maximumSize >= 0 && c.length > c.maximumSize || c.maximumWeight >= 0 && c.totalWeight > c.maximumWeight) {
		events = c.removeLocked(c.head, Size, events)
	}
	return events
}

func (c *Cache[K, V]) notify(events []Notification[K, V]) {
	if len(events) == 0 || c.listener == nil {
		return
	}
	operation := func() {
		for _, event := range events {
			c.listener(event)
		}
	}
	if c.listenerExecutor != nil {
		c.listenerExecutor(operation)
	} else {
		operation()
	}
}

func (c *Cache[K, V]) weight(key K, value V) int64 {
	weight := c.weigher(key, value)
	if weight < 0 {
		panic("cache: negative entry weight")
	}
	return weight
}

func (c *Cache[K, V]) setLocked(key K, value V, weight int64, now time.Time, events []Notification[K, V]) []Notification[K, V] {
	probe := &cacheEntry[K, V]{key: key}
	if old, found := c.tab.Search(probe); found {
		events = c.removeLocked(old, Replaced, events)
	}
	entry := &cacheEntry[K, V]{key: key, value: value, weight: weight, writeTime: now, accessTime: now}
	c.tab.Insert(entry)
	c.link(entry)
	return c.evictLocked(events)
}

// Put inserts or replaces a value using its static write-time weight.
func (c *Cache[K, V]) Put(key K, value V) {
	weight := c.weight(key, value)
	c.lock.Lock()
	now := c.now()
	events := c.cleanExpiredLocked(now, nil)
	events = c.setLocked(key, value, weight, now, events)
	c.lock.Unlock()
	c.notify(events)
}

// PutIfAbsent atomically stores value only when key has no visible value.
func (c *Cache[K, V]) PutIfAbsent(key K, value V) bool {
	weight := c.weight(key, value)
	c.lock.Lock()
	now := c.now()
	events := c.cleanExpiredLocked(now, nil)
	_, found := c.tab.Search(&cacheEntry[K, V]{key: key})
	stored := !found
	if stored {
		events = c.setLocked(key, value, weight, now, events)
	}
	c.lock.Unlock()
	c.notify(events)
	return stored
}

// Replace atomically replaces a visible value and returns whether key existed.
func (c *Cache[K, V]) Replace(key K, value V) bool {
	weight := c.weight(key, value)
	c.lock.Lock()
	now := c.now()
	events := c.cleanExpiredLocked(now, nil)
	_, found := c.tab.Search(&cacheEntry[K, V]{key: key})
	if found {
		events = c.setLocked(key, value, weight, now, events)
	}
	c.lock.Unlock()
	c.notify(events)
	return found
}

// DeleteValue removes key only when its value compares equal.
func (c *Cache[K, V]) DeleteValue(key K, value V, equal func(V, V) bool) bool {
	if equal == nil {
		panic("cache: nil value equality")
	}
	c.lock.Lock()
	events := c.cleanExpiredLocked(c.now(), nil)
	found := false
	if entry, ok := c.tab.Search(&cacheEntry[K, V]{key: key}); ok && equal(entry.value, value) {
		found = true
		events = c.removeLocked(entry, Explicit, events)
	}
	c.lock.Unlock()
	c.notify(events)
	return found
}

func (c *Cache[K, V]) Invalidate(key K) {
	c.lock.Lock()
	events := []Notification[K, V]{}
	if entry, found := c.tab.Search(&cacheEntry[K, V]{key: key}); found {
		events = c.removeLocked(entry, Explicit, events)
	}
	c.lock.Unlock()
	c.notify(events)
}

// InvalidateKeys atomically invalidates all supplied keys.
func (c *Cache[K, V]) InvalidateKeys(keys []K) {
	c.lock.Lock()
	events := []Notification[K, V]{}
	for _, key := range keys {
		if entry, found := c.tab.Search(&cacheEntry[K, V]{key: key}); found {
			events = c.removeLocked(entry, Explicit, events)
		}
	}
	c.lock.Unlock()
	c.notify(events)
}

func (c *Cache[K, V]) InvalidateAll() {
	c.lock.Lock()
	events := []Notification[K, V]{}
	entries := []*cacheEntry[K, V]{}
	for entry := range c.tab.Values() {
		entries = append(entries, entry)
	}
	for _, entry := range entries {
		events = c.removeLocked(entry, Explicit, events)
	}
	c.lock.Unlock()
	c.notify(events)
}

// CleanUp performs operation-driven expiration and size/weight eviction.
func (c *Cache[K, V]) CleanUp() {
	c.lock.Lock()
	events := c.cleanExpiredLocked(c.now(), nil)
	events = c.evictLocked(events)
	c.lock.Unlock()
	c.notify(events)
}

func (c *Cache[K, V]) Len() int {
	c.lock.Lock()
	events := c.cleanExpiredLocked(c.now(), nil)
	length := c.length
	c.lock.Unlock()
	c.notify(events)
	return length
}
func (c *Cache[K, V]) IsEmpty() bool { return c.Len() == 0 }

// GetIfPresent returns a visible value and records an access and hit/miss stats.
func (c *Cache[K, V]) GetIfPresent(key K) (V, bool) {
	return c.present(key, true, true)
}

// Peek returns a visible value without changing LRU/access time or stats.
func (c *Cache[K, V]) Peek(key K) (V, bool) { return c.present(key, false, false) }

func (c *Cache[K, V]) present(key K, access, record bool) (V, bool) {
	c.lock.Lock()
	now := c.now()
	events := []Notification[K, V]{}
	entry, found := c.tab.Search(&cacheEntry[K, V]{key: key})
	if found && !entry.linked {
		found = false
	}
	if found && c.expired(entry, now) {
		events = c.removeLocked(entry, Expired, events)
		found = false
	}
	if record {
		if found {
			c.stats.Hits++
		} else {
			c.stats.Misses++
		}
	}
	if found && access {
		c.touchInline(entry, now)
	}
	c.lock.Unlock()
	c.notify(events)
	if found {
		return entry.value, true
	}
	return *new(V), false
}

// Snapshot returns fresh entries for visible values in unspecified hash order.
func (c *Cache[K, V]) Snapshot() []Entry[K, V] {
	c.lock.Lock()
	events := c.cleanExpiredLocked(c.now(), nil)
	entries := []Entry[K, V]{}
	for entry := range c.tab.Values() {
		if entry.linked {
			entries = append(entries, Entry[K, V]{Key: entry.key, Value: entry.value})
		}
	}
	c.lock.Unlock()
	c.notify(events)
	return entries
}
func (c *Cache[K, V]) All() iter.Seq[Entry[K, V]] {
	entries := c.Snapshot()
	return func(yield func(Entry[K, V]) bool) {
		for _, entry := range entries {
			if !yield(entry) {
				return
			}
		}
	}
}

func (c *Cache[K, V]) Stats() Stats {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.stats
}
func (c *Cache[K, V]) ClearStats() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.stats = Stats{}
}
