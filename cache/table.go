/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package cache

import "sync/atomic"

// The read table is a per-shard open-addressing hash table that readers probe
// without any lock while writers mutate it under the shard lock.
//
// Slot protocol. A slot's hash is 0 (empty: ends a probe chain), 1 (tombstone:
// probing continues), or a key hash remapped to be ≥ 2. Insert stores the
// entry pointer and then the hash. Delete stores the tombstone hash and then a
// nil entry. A reader loads the hash, then the entry, and compares the entry's
// own immutable hash and key, so a slot reused between the two loads can only
// produce a false miss, never a wrong hit.
//
// Growth and tombstone purges build a new table and publish it with one atomic
// pointer store. The old table is never written again, so a reader still
// probing it sees a frozen past state. Readers therefore treat the table as a
// hint. An entry found there is returned only if its own atomic state is
// entryLive, and every not-found, dead, loading or expired outcome is retried
// under the shard lock (which is exact). Occupied slots never exceed half the
// table, so every probe chain ends at an empty slot.

const (
	slotEmpty     uint64 = 0
	slotTombstone uint64 = 1
	minTableSize         = 16
)

// fixHash remaps the two reserved slot markers out of the key-hash space.
func fixHash(h uint64) uint64 {
	if h < 2 {
		h += 2
	}
	return h
}

type tableSlot[K any, V any] struct {
	hash  atomic.Uint64
	entry atomic.Pointer[cacheEntry[K, V]]
}

type readTable[K any, V any] struct {
	mask  uint64
	slots []tableSlot[K, V]
}

func newReadTable[K any, V any](size int) *readTable[K, V] {
	return &readTable[K, V]{mask: uint64(size - 1), slots: make([]tableSlot[K, V], size)}
}

// find returns the entry stored for key, or nil. It is safe without the lock;
// lock-free callers must still check the entry's state.
// Complexity is O(1) average.
func (s *cacheShard[K, V]) find(key K, h uint64) *cacheEntry[K, V] {
	t := s.table.Load()
	equal := s.c.equal
	for i := h & t.mask; ; i = (i + 1) & t.mask {
		slotHash := t.slots[i].hash.Load()
		if slotHash == slotEmpty {
			return nil
		}
		if slotHash == h {
			if entry := t.slots[i].entry.Load(); entry != nil && entry.hash == h && equal(entry.key, key) {
				return entry
			}
		}
	}
}

// insertLocked stores entry, whose key must be absent.
// Complexity is O(1) average; rebuilds are amortized O(1).
func (s *cacheShard[K, V]) insertLocked(entry *cacheEntry[K, V]) {
	t := s.table.Load()
	if (s.used+1)*2 > len(t.slots) {
		size := len(t.slots)
		for (s.live+1)*4 > size {
			size *= 2
		}
		t = s.rebuildLocked(size)
	}
	for i := entry.hash & t.mask; ; i = (i + 1) & t.mask {
		slotHash := t.slots[i].hash.Load()
		if slotHash == slotEmpty || slotHash == slotTombstone {
			if slotHash == slotEmpty {
				s.used++
			}
			s.live++
			t.slots[i].entry.Store(entry)
			t.slots[i].hash.Store(entry.hash)
			return
		}
	}
}

// slotOfLocked returns the slot holding exactly this entry pointer, or nil.
func (s *cacheShard[K, V]) slotOfLocked(entry *cacheEntry[K, V]) *tableSlot[K, V] {
	t := s.table.Load()
	for i := entry.hash & t.mask; ; i = (i + 1) & t.mask {
		slotHash := t.slots[i].hash.Load()
		if slotHash == slotEmpty {
			return nil
		}
		if slotHash == entry.hash && t.slots[i].entry.Load() == entry {
			return &t.slots[i]
		}
	}
}

// deleteLocked removes exactly this entry pointer and reports whether it was
// the stored entry for its key.
// Complexity is O(1) average.
func (s *cacheShard[K, V]) deleteLocked(entry *cacheEntry[K, V]) bool {
	slot := s.slotOfLocked(entry)
	if slot == nil {
		return false
	}
	slot.hash.Store(slotTombstone)
	slot.entry.Store(nil)
	s.live--
	return true
}

// swapLocked replaces old with replacement (same key) in place.
func (s *cacheShard[K, V]) swapLocked(old, replacement *cacheEntry[K, V]) {
	s.slotOfLocked(old).entry.Store(replacement)
}

// rebuildLocked copies the live slots into a fresh table of the given size,
// dropping tombstones, and publishes it.
func (s *cacheShard[K, V]) rebuildLocked(size int) *readTable[K, V] {
	old := s.table.Load()
	t := newReadTable[K, V](size)
	for index := range old.slots {
		slotHash := old.slots[index].hash.Load()
		if slotHash == slotEmpty || slotHash == slotTombstone {
			continue
		}
		entry := old.slots[index].entry.Load()
		for i := slotHash & t.mask; ; i = (i + 1) & t.mask {
			if t.slots[i].hash.Load() == slotEmpty {
				t.slots[i].entry.Store(entry)
				t.slots[i].hash.Store(slotHash)
				break
			}
		}
	}
	s.used = s.live
	s.table.Store(t)
	return t
}

// entriesLocked returns every stored entry (live and loading placeholders).
// Complexity is O(table size).
func (s *cacheShard[K, V]) entriesLocked() []*cacheEntry[K, V] {
	t := s.table.Load()
	entries := make([]*cacheEntry[K, V], 0, s.live)
	for index := range t.slots {
		if t.slots[index].hash.Load() >= 2 {
			entries = append(entries, t.slots[index].entry.Load())
		}
	}
	return entries
}
