/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package cache

import (
	"iter"
	"math"
)

// fibonacciMultiplier spreads key hashes before the shard index is taken from
// the top bits, so weak caller hashes (small integers) still spread.
const fibonacciMultiplier = 0x9E3779B97F4A7C15

// nanos reads the clock as nanoseconds since the cache epoch.
func (c *Cache[K, V]) nanos() int64 { return int64(c.clock().Sub(c.epoch)) }

// stamp reads the clock only when entries carry timestamps (expiration or
// refresh is configured); otherwise timestamps are never read and 0 is returned.
func (c *Cache[K, V]) stamp() int64 {
	if c.timed {
		return c.nanos()
	}
	return 0
}

// locate hashes key once and returns its shard and (slot-safe) hash.
func (c *Cache[K, V]) locate(key K) (*cacheShard[K, V], uint64) {
	h := fixHash(c.hash(key))
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

// expiredAt applies the expiration rule to a live entry's timestamps. It is
// safe without the lock.
func (c *Cache[K, V]) expiredAt(entry *cacheEntry[K, V], now int64) bool {
	if c.expireWrite > 0 && now-entry.writeTime >= int64(c.expireWrite) {
		return true
	}
	return c.expireAccess > 0 && now-entry.accessTime.Load() >= int64(c.expireAccess)
}

func (c *Cache[K, V]) expired(entry *cacheEntry[K, V], now int64) bool {
	return entry.linked && c.expiredAt(entry, now)
}

// clampAccess keeps the access list sorted by access time. An access time
// captured outside the lock can trail the tail's by a few nanoseconds (a read
// event published after a concurrent drain), and is raised to it.
func (s *cacheShard[K, V]) clampAccess(entry *cacheEntry[K, V], at int64) int64 {
	if s.tail != nil && s.tail != entry {
		if t := s.tail.accessTime.Load(); t > at {
			return t
		}
	}
	return at
}

// link appends entry to the tail of the access list (and the write list when
// expire-after-write is configured) and publishes it to readers.
func (s *cacheShard[K, V]) link(entry *cacheEntry[K, V]) {
	if entry.weight > 0 && s.totalWeight > math.MaxInt64-entry.weight {
		panic("cache: total weight overflow")
	}
	if s.c.expireAccess > 0 {
		entry.accessTime.Store(s.clampAccess(entry, entry.accessTime.Load()))
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
	entry.state.Store(entryLive) // release: value/weight/writeTime are visible to readers
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

// touchInline applies one access: the entry moves to the tail of the access
// list, and its access time advances when something reads it.
func (s *cacheShard[K, V]) touchInline(entry *cacheEntry[K, V], at int64) {
	if !entry.linked {
		return
	}
	if s.c.expireAccess > 0 {
		entry.accessTime.Store(s.clampAccess(entry, at))
	}
	if s.tail == entry {
		return
	}
	s.unlinkAccess(entry)
	entry.prev = s.tail
	s.tail.next = entry // tail != nil: entry is linked and is not the tail
	s.tail = entry
}

// replaceLocked publishes replacement (same key, new value, timestamps already
// set) in place of the live entry old, as a write: the new entry goes to the
// tail of both lists. Entries are immutable once readable, so a refresh swaps
// entries rather than mutating the value under concurrent readers.
func (s *cacheShard[K, V]) replaceLocked(old, replacement *cacheEntry[K, V]) {
	s.swapLocked(old, replacement)
	s.unlink(old)
	old.state.Store(entryDead)
	s.link(replacement)
}

func (s *cacheShard[K, V]) removeLocked(entry *cacheEntry[K, V], cause RemovalCause, events []Notification[K, V]) []Notification[K, V] {
	if !s.deleteLocked(entry) {
		return events
	}
	s.unlink(entry)
	entry.state.Store(entryDead)
	if cause == Size || cause == Expired {
		s.stats.Evictions++
	}
	if s.c.listener != nil {
		events = append(events, Notification[K, V]{Key: entry.key, Value: entry.value, Cause: cause})
	}
	return events
}

// drainReadsLocked applies buffered lock-free reads: each event is one hit and
// one LRU/access-time touch.
func (s *cacheShard[K, V]) drainReadsLocked() {
	s.reads.drain(s.applyRead)
}

func (s *cacheShard[K, V]) applyRead(entry *cacheEntry[K, V], at int64) {
	s.stats.Hits++
	s.touchInline(entry, at)
}

// recordRead buffers a lock-free hit. Past the drain threshold the reader
// drains if the lock is free. When the ring is full it takes the lock, so no
// event is ever lost.
//
// When access order is never read (no size/weight bound and no
// expire-after-access), there is nothing to replay, so the hit is just
// counted. Caffeine disables its read buffer in the same case.
func (s *cacheShard[K, V]) recordRead(entry *cacheEntry[K, V], at int64) {
	if !s.c.ordered {
		s.hits.Add(1)
		return
	}
	accepted, pending := s.reads.offer(entry, at)
	if !accepted {
		s.lock.Lock()
		s.drainReadsLocked()
		s.applyRead(entry, at)
		s.lock.Unlock()
		return
	}
	if pending >= readDrainThreshold && s.lock.TryLock() {
		s.drainReadsLocked()
		s.lock.Unlock()
	}
}

// cleanExpiredLocked drains expired entries from the fronts of the time-ordered
// lists: O(expired), not O(n). Write times are taken under the shard lock, and
// access times are clamped to the tail's (clampAccess), so each list is sorted
// by its timestamp (for a monotonic clock) and the first live entry ends the
// drain.
func (s *cacheShard[K, V]) cleanExpiredLocked(now int64, events []Notification[K, V]) []Notification[K, V] {
	c := s.c
	if c.expireWrite > 0 {
		for entry := s.whead; entry != nil && now-entry.writeTime >= int64(c.expireWrite); entry = s.whead {
			events = s.removeLocked(entry, Expired, events)
			if s.whead == entry {
				panic("cache: internal error: linked entry missing from its table")
			}
		}
	}
	if c.expireAccess > 0 {
		for entry := s.head; entry != nil && now-entry.accessTime.Load() >= int64(c.expireAccess); entry = s.head {
			events = s.removeLocked(entry, Expired, events)
			if s.head == entry {
				panic("cache: internal error: linked entry missing from its table")
			}
		}
	}
	return events
}

// cleanLocked is the start of every locked operation. It applies buffered
// reads first, so LRU order, access times and hit counts are current. Then it
// runs operation-driven expiration, which costs nothing when nothing expires.
func (s *cacheShard[K, V]) cleanLocked(events []Notification[K, V]) []Notification[K, V] {
	s.drainReadsLocked()
	if !s.c.expiring {
		return events
	}
	return s.cleanExpiredLocked(s.c.nanos(), events)
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

func (s *cacheShard[K, V]) setLocked(key K, h uint64, value V, weight int64, now int64, events []Notification[K, V]) []Notification[K, V] {
	if old := s.find(key, h); old != nil {
		events = s.removeLocked(old, Replaced, events)
	}
	entry := &cacheEntry[K, V]{key: key, value: value, hash: h, weight: weight, writeTime: now}
	entry.accessTime.Store(now)
	s.insertLocked(entry)
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
	stored := s.find(key, h) == nil
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
	found := s.find(key, h) != nil
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
	if entry := s.find(key, h); entry != nil && equal(entry.value, value) {
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
	if entry := s.find(key, h); entry != nil {
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
		if entry := s.find(key, h); entry != nil {
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
		for _, entry := range shard.entriesLocked() {
			events = shard.removeLocked(entry, Explicit, events)
		}
	}
	c.unlockAll()
	c.notify(events)
}

// CleanUp applies buffered reads, then performs operation-driven expiration and
// size/weight eviction in every shard, one shard at a time.
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
// A hit takes no lock: it probes the shard's lock-free table and buffers the
// access. Misses and expired entries take the shard lock.
// Complexity is O(1) average.
func (c *Cache[K, V]) GetIfPresent(key K) (V, bool) {
	s, h := c.locate(key)
	if entry := s.find(key, h); entry != nil && entry.state.Load() == entryLive {
		var now int64
		if c.expiring {
			now = c.nanos()
		}
		if !c.expiredAt(entry, now) {
			value := entry.value
			s.recordRead(entry, now)
			return value, true
		}
	}
	return c.presentLocked(s, key, h, true)
}

// Peek returns a visible value without changing LRU/access time or stats.
// A hit takes no lock.
// Complexity is O(1) average.
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	s, h := c.locate(key)
	if entry := s.find(key, h); entry != nil && entry.state.Load() == entryLive {
		if !c.expiring || !c.expiredAt(entry, c.nanos()) {
			return entry.value, true
		}
	}
	return c.presentLocked(s, key, h, false)
}

// presentLocked is the exact slow path for GetIfPresent (record) and Peek. It
// applies buffered reads first, because an access time that looked expired
// to the lock-free check may be newer in the buffer.
func (c *Cache[K, V]) presentLocked(s *cacheShard[K, V], key K, h uint64, record bool) (V, bool) {
	s.lock.Lock()
	s.drainReadsLocked()
	var events []Notification[K, V]
	var now int64
	entry := s.find(key, h)
	found := entry != nil && entry.linked
	if found && c.expiring {
		now = c.nanos()
		if c.expiredAt(entry, now) {
			events = s.removeLocked(entry, Expired, events)
			found = false
		}
	}
	var value V
	if found {
		value = entry.value
		if record {
			s.applyRead(entry, now)
		}
	} else if record {
		s.stats.Misses++
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
		for _, entry := range shard.entriesLocked() {
			if entry.linked {
				entries = append(entries, Entry[K, V]{Key: entry.key, Value: entry.value})
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

// Stats returns an atomic snapshot of the counters summed across shards,
// including every buffered hit.
// Complexity is O(shards).
func (c *Cache[K, V]) Stats() Stats {
	c.lockAll()
	defer c.unlockAll()
	var total Stats
	for _, shard := range c.shards {
		shard.drainReadsLocked()
		total.Hits += shard.stats.Hits + shard.hits.Load()
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
		shard.drainReadsLocked()
		shard.stats = Stats{}
		shard.hits.Store(0)
	}
}
