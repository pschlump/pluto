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

// fibonacciMultiplier spreads key hashes before the shard index is taken from
// the top bits, so weak caller hashes (small integers) still spread.
const fibonacciMultiplier = 0x9E3779B97F4A7C15

func (c *Cache[K, V]) now() time.Time { return c.clock() }

// stamp reads the clock only when entries carry timestamps (expiration or
// refresh is configured); otherwise timestamps are never read and the zero
// time is returned.
func (c *Cache[K, V]) stamp() time.Time {
	if c.timed {
		return c.clock()
	}
	return time.Time{}
}

// locate hashes key once and returns its shard and hash.
func (c *Cache[K, V]) locate(key K) (*cacheShard[K, V], uint64) {
	h := c.hash(key)
	return c.shards[(h*fibonacciMultiplier)>>c.shift], h
}

// lockAll locks every shard in index order; it is the only code path that
// holds more than one shard lock, so the fixed order cannot deadlock.
func (c *Cache[K, V]) lockAll() {
	for _, shard := range c.shards {
		shard.lock.Lock()
	}
}

func (c *Cache[K, V]) unlockAll() {
	for _, shard := range c.shards {
		shard.lock.Unlock()
	}
}

func (c *Cache[K, V]) expired(entry *cacheEntry[K, V], now time.Time) bool {
	if !entry.linked {
		return false
	}
	if c.expireWrite > 0 && now.Sub(entry.writeTime) >= c.expireWrite {
		return true
	}
	return c.expireAccess > 0 && now.Sub(entry.accessTime) >= c.expireAccess
}

func (s *cacheShard[K, V]) lookup(key K, h uint64) (*cacheEntry[K, V], bool) {
	slot, found := s.tab.Search(cacheSlot[K, V]{key: key, hash: h})
	return slot.entry, found
}

// link appends entry to the tail of the access list (and the write list when
// expire-after-write is configured).
func (s *cacheShard[K, V]) link(entry *cacheEntry[K, V]) {
	if entry.weight > 0 && s.totalWeight > math.MaxInt64-entry.weight {
		panic("cache: total weight overflow")
	}
	entry.linked = true
	entry.prev = s.tail
	if s.tail == nil {
		s.head = entry
	} else {
		s.tail.next = entry
	}
	s.tail = entry
	if s.c.expireWrite > 0 {
		s.linkWrite(entry)
	}
	s.length++
	s.totalWeight += entry.weight
}

func (s *cacheShard[K, V]) linkWrite(entry *cacheEntry[K, V]) {
	entry.wprev = s.wtail
	if s.wtail == nil {
		s.whead = entry
	} else {
		s.wtail.wnext = entry
	}
	s.wtail = entry
}

func (s *cacheShard[K, V]) unlinkAccess(entry *cacheEntry[K, V]) {
	if entry.prev == nil {
		s.head = entry.next
	} else {
		entry.prev.next = entry.next
	}
	if entry.next == nil {
		s.tail = entry.prev
	} else {
		entry.next.prev = entry.prev
	}
	entry.prev, entry.next = nil, nil
}

func (s *cacheShard[K, V]) unlinkWrite(entry *cacheEntry[K, V]) {
	if entry.wprev == nil {
		s.whead = entry.wnext
	} else {
		entry.wprev.wnext = entry.wnext
	}
	if entry.wnext == nil {
		s.wtail = entry.wprev
	} else {
		entry.wnext.wprev = entry.wprev
	}
	entry.wprev, entry.wnext = nil, nil
}

func (s *cacheShard[K, V]) unlink(entry *cacheEntry[K, V]) {
	if !entry.linked {
		return
	}
	s.unlinkAccess(entry)
	if s.c.expireWrite > 0 {
		s.unlinkWrite(entry)
	}
	entry.linked = false
	s.length--
	s.totalWeight -= entry.weight
}

// touchInline keeps length/weight unchanged while moving an entry to the tail
// of the access list. The access time is recorded only when something reads it.
func (s *cacheShard[K, V]) touchInline(entry *cacheEntry[K, V], now time.Time) {
	if !entry.linked {
		return
	}
	if s.c.expireAccess > 0 {
		entry.accessTime = now
	}
	if s.tail == entry {
		return
	}
	s.unlinkAccess(entry)
	entry.prev = s.tail
	s.tail.next = entry // tail != nil: entry is linked and is not the tail
	s.tail = entry
}

// rewriteInline records an in-place write (refresh completion): both
// timestamps advance and the entry moves to the tail of both lists.
func (s *cacheShard[K, V]) rewriteInline(entry *cacheEntry[K, V], now time.Time) {
	entry.writeTime = now
	s.touchInline(entry, now)
	entry.accessTime = now
	if s.c.expireWrite > 0 && s.wtail != entry {
		s.unlinkWrite(entry)
		s.linkWrite(entry)
	}
}

func (s *cacheShard[K, V]) removeLocked(entry *cacheEntry[K, V], cause RemovalCause, events []Notification[K, V]) []Notification[K, V] {
	current, found := s.lookup(entry.key, entry.hash)
	if !found || current != entry {
		return events
	}
	s.tab.Delete(cacheSlot[K, V]{key: entry.key, hash: entry.hash})
	s.unlink(entry)
	if cause == Size || cause == Expired {
		s.stats.Evictions++
	}
	if s.c.listener != nil {
		events = append(events, Notification[K, V]{Key: entry.key, Value: entry.value, Cause: cause})
	}
	return events
}

// cleanExpiredLocked drains expired entries from the fronts of the time-ordered
// lists: O(expired), not O(n). Every timestamp is taken under the shard lock,
// so (for a monotonic clock) each list is ordered by its timestamp and the
// first live entry ends the drain.
func (s *cacheShard[K, V]) cleanExpiredLocked(now time.Time, events []Notification[K, V]) []Notification[K, V] {
	c := s.c
	if c.expireWrite > 0 {
		for entry := s.whead; entry != nil && now.Sub(entry.writeTime) >= c.expireWrite; entry = s.whead {
			events = s.removeLocked(entry, Expired, events)
			if s.whead == entry {
				panic("cache: internal error: linked entry missing from its table")
			}
		}
	}
	if c.expireAccess > 0 {
		for entry := s.head; entry != nil && now.Sub(entry.accessTime) >= c.expireAccess; entry = s.head {
			events = s.removeLocked(entry, Expired, events)
			if s.head == entry {
				panic("cache: internal error: linked entry missing from its table")
			}
		}
	}
	return events
}

// cleanLocked is the operation-driven expiration hook: free when nothing expires.
func (s *cacheShard[K, V]) cleanLocked(events []Notification[K, V]) []Notification[K, V] {
	if !s.c.expiring {
		return events
	}
	return s.cleanExpiredLocked(s.c.clock(), events)
}

func (s *cacheShard[K, V]) evictLocked(events []Notification[K, V]) []Notification[K, V] {
	for s.head != nil && (s.maximumSize >= 0 && s.length > s.maximumSize || s.maximumWeight >= 0 && s.totalWeight > s.maximumWeight) {
		events = s.removeLocked(s.head, Size, events)
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

func (s *cacheShard[K, V]) setLocked(key K, h uint64, value V, weight int64, now time.Time, events []Notification[K, V]) []Notification[K, V] {
	if old, found := s.lookup(key, h); found {
		events = s.removeLocked(old, Replaced, events)
	}
	entry := &cacheEntry[K, V]{key: key, value: value, hash: h, weight: weight, writeTime: now, accessTime: now}
	s.tab.Insert(cacheSlot[K, V]{key: key, hash: h, entry: entry})
	s.link(entry)
	return s.evictLocked(events)
}

// Put inserts or replaces a value using its static write-time weight.
// Complexity is O(1) average plus O(expired + evicted) in the key's shard.
func (c *Cache[K, V]) Put(key K, value V) {
	weight := c.weight(key, value)
	s, h := c.locate(key)
	s.lock.Lock()
	now := c.stamp()
	events := s.cleanLocked(nil)
	events = s.setLocked(key, h, value, weight, now, events)
	s.lock.Unlock()
	c.notify(events)
}

// PutIfAbsent atomically stores value only when key has no visible value.
// Complexity is O(1) average plus O(expired + evicted) in the key's shard.
func (c *Cache[K, V]) PutIfAbsent(key K, value V) bool {
	weight := c.weight(key, value)
	s, h := c.locate(key)
	s.lock.Lock()
	now := c.stamp()
	events := s.cleanLocked(nil)
	_, found := s.lookup(key, h)
	stored := !found
	if stored {
		events = s.setLocked(key, h, value, weight, now, events)
	}
	s.lock.Unlock()
	c.notify(events)
	return stored
}

// Replace atomically replaces a visible value and returns whether key existed.
// Complexity is O(1) average plus O(expired + evicted) in the key's shard.
func (c *Cache[K, V]) Replace(key K, value V) bool {
	weight := c.weight(key, value)
	s, h := c.locate(key)
	s.lock.Lock()
	now := c.stamp()
	events := s.cleanLocked(nil)
	_, found := s.lookup(key, h)
	if found {
		events = s.setLocked(key, h, value, weight, now, events)
	}
	s.lock.Unlock()
	c.notify(events)
	return found
}

// DeleteValue removes key only when its value compares equal.
// Complexity is O(1) average plus O(expired) in the key's shard.
func (c *Cache[K, V]) DeleteValue(key K, value V, equal func(V, V) bool) bool {
	if equal == nil {
		panic("cache: nil value equality")
	}
	s, h := c.locate(key)
	s.lock.Lock()
	events := s.cleanLocked(nil)
	found := false
	if entry, ok := s.lookup(key, h); ok && equal(entry.value, value) {
		found = true
		events = s.removeLocked(entry, Explicit, events)
	}
	s.lock.Unlock()
	c.notify(events)
	return found
}

// Invalidate removes key. An entry that had already expired is reported as
// Expired, not Explicit (expiration in the shard runs first, as in Guava).
// Complexity is O(1) average plus O(expired) in the key's shard.
func (c *Cache[K, V]) Invalidate(key K) {
	s, h := c.locate(key)
	s.lock.Lock()
	events := s.cleanLocked(nil)
	if entry, found := s.lookup(key, h); found {
		events = s.removeLocked(entry, Explicit, events)
	}
	s.lock.Unlock()
	c.notify(events)
}

// InvalidateKeys atomically invalidates all supplied keys (all shards are
// locked for the duration).
// Complexity is O(len(keys)) average plus O(shards + expired).
func (c *Cache[K, V]) InvalidateKeys(keys []K) {
	c.lockAll()
	var events []Notification[K, V]
	for _, shard := range c.shards {
		events = shard.cleanLocked(events)
	}
	for _, key := range keys {
		s, h := c.locate(key)
		if entry, found := s.lookup(key, h); found {
			events = s.removeLocked(entry, Explicit, events)
		}
	}
	c.unlockAll()
	c.notify(events)
}

// InvalidateAll atomically removes every entry.
// Complexity is O(n + shards).
func (c *Cache[K, V]) InvalidateAll() {
	c.lockAll()
	var events []Notification[K, V]
	for _, shard := range c.shards {
		events = shard.cleanLocked(events)
		entries := make([]*cacheEntry[K, V], 0, shard.tab.Len())
		for slot := range shard.tab.Values() {
			entries = append(entries, slot.entry)
		}
		for _, entry := range entries {
			events = shard.removeLocked(entry, Explicit, events)
		}
	}
	c.unlockAll()
	c.notify(events)
}

// CleanUp performs operation-driven expiration and size/weight eviction in
// every shard, one shard at a time.
// Complexity is O(shards + expired + evicted).
func (c *Cache[K, V]) CleanUp() {
	var events []Notification[K, V]
	for _, shard := range c.shards {
		shard.lock.Lock()
		events = shard.cleanLocked(events)
		events = shard.evictLocked(events)
		shard.lock.Unlock()
	}
	c.notify(events)
}

// Len returns the number of visible entries, as one atomic snapshot.
// Complexity is O(shards + expired).
func (c *Cache[K, V]) Len() int {
	c.lockAll()
	var events []Notification[K, V]
	length := 0
	for _, shard := range c.shards {
		events = shard.cleanLocked(events)
		length += shard.length
	}
	c.unlockAll()
	c.notify(events)
	return length
}
func (c *Cache[K, V]) IsEmpty() bool { return c.Len() == 0 }

// GetIfPresent returns a visible value and records an access and hit/miss stats.
// Complexity is O(1) average.
func (c *Cache[K, V]) GetIfPresent(key K) (V, bool) {
	return c.present(key, true, true)
}

// Peek returns a visible value without changing LRU/access time or stats.
// Complexity is O(1) average.
func (c *Cache[K, V]) Peek(key K) (V, bool) { return c.present(key, false, false) }

func (c *Cache[K, V]) present(key K, access, record bool) (V, bool) {
	s, h := c.locate(key)
	s.lock.Lock()
	var events []Notification[K, V]
	var now time.Time
	entry, found := s.lookup(key, h)
	if found && !entry.linked {
		found = false
	}
	if found && c.expiring {
		now = c.clock()
		if c.expired(entry, now) {
			events = s.removeLocked(entry, Expired, events)
			found = false
		}
	}
	if record {
		if found {
			s.stats.Hits++
		} else {
			s.stats.Misses++
		}
	}
	var value V
	if found {
		value = entry.value
		if access {
			s.touchInline(entry, now)
		}
	}
	s.lock.Unlock()
	c.notify(events)
	return value, found
}

// Snapshot returns fresh entries for visible values in unspecified hash order,
// taken atomically across all shards.
// Complexity is O(n + shards).
func (c *Cache[K, V]) Snapshot() []Entry[K, V] {
	c.lockAll()
	var events []Notification[K, V]
	entries := []Entry[K, V]{}
	for _, shard := range c.shards {
		events = shard.cleanLocked(events)
		for slot := range shard.tab.Values() {
			if slot.entry.linked {
				entries = append(entries, Entry[K, V]{Key: slot.key, Value: slot.entry.value})
			}
		}
	}
	c.unlockAll()
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

// Stats returns an atomic snapshot of the counters summed across shards.
// Complexity is O(shards).
func (c *Cache[K, V]) Stats() Stats {
	c.lockAll()
	defer c.unlockAll()
	var total Stats
	for _, shard := range c.shards {
		total.Hits += shard.stats.Hits
		total.Misses += shard.stats.Misses
		total.Loads += shard.stats.Loads
		total.LoadSuccesses += shard.stats.LoadSuccesses
		total.LoadFailures += shard.stats.LoadFailures
		total.TotalLoadTime += shard.stats.TotalLoadTime
		total.Evictions += shard.stats.Evictions
	}
	return total
}

// ClearStats resets every shard's counters atomically.
// Complexity is O(shards).
func (c *Cache[K, V]) ClearStats() {
	c.lockAll()
	defer c.unlockAll()
	for _, shard := range c.shards {
		shard.stats = Stats{}
	}
}
