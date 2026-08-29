/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package lru implements a generic capacity-bounded LRU (least recently
// used) cache: a map for O(1) lookups plus a doubly linked list that
// tracks recency, most-recently-used at the front.
//
// A cache is created with NewLru (every entry evictable) or
// NewLruFunc, which takes an evictable callback that may VETO the
// eviction of an entry: when eviction is needed the vetoed entries are
// skipped and the next-older entry is tried.  If nothing is evictable
// the cache temporarily EXCEEDS its capacity — a soft cap, not a stall
// (b_tree_disk_ts relies on this: a dirty or pinned block must never be
// evicted, so the block cache briefly grows past the cap instead of
// deadlocking on a flush).
//
// Basic operations on an Lru:
//
//	Get — look up a key; a hit marks the entry most-recently-used.
//	Peek — look up a key without changing the recency order.
//	Put — insert or update; updates mark the entry most-recently-used,
//	      and inserts evict least-recently-used entries (subject to the
//	      veto) down to the capacity first.
//	Delete — remove a key.
//	Len / Capacity — size queries.
//	Clear — drop every entry, keeping the capacity and the veto.
//	All / Backward — range-over-func iterators (MRU→LRU / LRU→MRU).
//
// A nil *Lru and the zero value both behave as an empty cache for every
// operation that has a sane answer: Get and Peek report not-found,
// Delete returns false, Len and Capacity are 0, Clear does nothing, and
// the iterators yield nothing.
//
// The package panics in exactly three situations, all programmer errors
// that cannot be handled where they occur:
//
//	NewLru / NewLruFunc with capacity < 1 — a cache needs room for at least one entry.
//	Put on a nil cache                    — a nil cache cannot store an entry.
//	Put on a zero-value cache             — no capacity; the message names the constructors.
//
// Lru is not safe for concurrent use; callers that share a cache across
// goroutines must guard it with their own mutex (b_tree_disk_ts holds
// its store lock around every cache operation).  There is no _ts twin —
// wrap the cache in the caller's lock, the same way queue_dll points
// shared-FIFO callers at their own guarding.
package lru

import "container/list"

// lruEntry is one cached key/value pair, held in the recency list.
type lruEntry[K comparable, V any] struct {
	key   K
	value V
}

// Lru is a capacity-bounded LRU cache of key/value pairs.  Use NewLru
// or NewLruFunc to create one; the zero value reads as an empty cache
// but cannot be Put into.
type Lru[K comparable, V any] struct {
	capacity int
	ll       *list.List // of lruEntry[K,V], front = most recently used
	byKey    map[K]*list.Element

	// evictable reports whether an entry may be evicted; a false answer
	// vetoes the eviction and the scan moves on to the next-older entry.
	// A nil evictable means everything is evictable.
	evictable func(K, V) bool
}

// NewLru creates a cache that holds up to capacity entries; every entry
// is evictable.  It panics if capacity < 1.
// Complexity is O(1).
func NewLru[K comparable, V any](capacity int) *Lru[K, V] {
	return NewLruFunc[K, V](capacity, nil)
}

// NewLruFunc creates a cache that holds up to capacity entries and
// consults evictable before evicting an entry: evictable(k, v) == false
// vetoes the eviction (the scan skips the entry and tries the
// next-older one).  A nil evictable means everything is evictable.  It
// panics if capacity < 1.
// Complexity is O(1).
func NewLruFunc[K comparable, V any](capacity int, evictable func(K, V) bool) *Lru[K, V] {
	if capacity < 1 {
		panic("lru: NewLruFunc called with capacity < 1: a cache needs room for at least one entry")
	}
	return &Lru[K, V]{
		capacity:  capacity,
		ll:        list.New(),
		byKey:     make(map[K]*list.Element),
		evictable: evictable,
	}
}

// Get returns the value stored under key and whether it was found.  A
// hit marks the entry most-recently-used.  A nil cache reports
// not-found.
// Complexity is O(1).
func (c *Lru[K, V]) Get(key K) (V, bool) {
	var zero V
	if c == nil || c.byKey == nil {
		return zero, false
	}
	if e, ok := c.byKey[key]; ok {
		c.ll.MoveToFront(e)
		return e.Value.(lruEntry[K, V]).value, true
	}
	return zero, false
}

// Peek returns the value stored under key and whether it was found,
// WITHOUT changing the recency order.  A nil cache reports not-found.
// Complexity is O(1).
func (c *Lru[K, V]) Peek(key K) (V, bool) {
	var zero V
	if c == nil || c.byKey == nil {
		return zero, false
	}
	if e, ok := c.byKey[key]; ok {
		return e.Value.(lruEntry[K, V]).value, true
	}
	return zero, false
}

// Put inserts or updates the entry for key and marks it
// most-recently-used.  Before an insert, least-recently-used entries
// are evicted (skipping any the veto protects) until the cache is under
// capacity; when nothing is evictable the cache temporarily EXCEEDS its
// capacity — a soft cap, see the package doc.
//
// Put panics on a nil cache and on a zero-value cache (no capacity);
// the message names the constructors.
// Complexity is O(1), except that eviction with a vetoing callback is
// O(scan) in the worst case.
func (c *Lru[K, V]) Put(key K, value V) {
	if c == nil {
		panic("lru: Put on a nil cache: a nil cache cannot store an entry; create it with NewLru or NewLruFunc")
	}
	if c.byKey == nil {
		panic("lru: Put on a zero-value cache: no capacity; create the cache with NewLru or NewLruFunc")
	}
	if e, ok := c.byKey[key]; ok {
		e.Value = lruEntry[K, V]{key: key, value: value}
		c.ll.MoveToFront(e)
		return
	}
	for c.ll.Len() >= c.capacity {
		if !c.evictOne() {
			break // nothing evictable: soft cap, the cache grows past capacity
		}
	}
	c.byKey[key] = c.ll.PushFront(lruEntry[K, V]{key: key, value: value})
}

// evictOne drops the least-recently-used entry the veto allows and
// reports whether one was evicted.
func (c *Lru[K, V]) evictOne() bool {
	for e := c.ll.Back(); e != nil; e = e.Prev() {
		ent := e.Value.(lruEntry[K, V])
		if c.evictable != nil && !c.evictable(ent.key, ent.value) {
			continue // vetoed: try the next-older entry
		}
		delete(c.byKey, ent.key)
		c.ll.Remove(e)
		return true
	}
	return false
}

// Delete removes the entry for key and reports whether it was present.
// A nil cache reports false.
// Complexity is O(1).
func (c *Lru[K, V]) Delete(key K) bool {
	if c == nil || c.byKey == nil {
		return false
	}
	if e, ok := c.byKey[key]; ok {
		delete(c.byKey, key)
		c.ll.Remove(e)
		return true
	}
	return false
}

// Len returns the number of entries in the cache — which may exceed
// Capacity while every entry is veto-protected (the soft cap).  A nil
// cache has length 0.
// Complexity is O(1).
func (c *Lru[K, V]) Len() int {
	if c == nil || c.ll == nil {
		return 0
	}
	return c.ll.Len()
}

// Capacity returns the (soft) capacity the cache was created with.  A
// nil or zero-value cache has capacity 0.
// Complexity is O(1).
func (c *Lru[K, V]) Capacity() int {
	if c == nil {
		return 0
	}
	return c.capacity
}

// Clear drops every entry, keeping the capacity and the veto callback.
// A nil cache stays nil.
// Complexity is O(n).
func (c *Lru[K, V]) Clear() {
	if c == nil || c.byKey == nil {
		return
	}
	c.ll.Init()
	clear(c.byKey)
}
