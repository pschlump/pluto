/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package lru_ts implements the capacity-bounded LRU cache safe for
// concurrent use.  It is the thread-safe twin of
// github.com/pschlump/pluto/lru — the map plus recency-list cache with
// the eviction-veto callback and the soft cap — with the identical API
// guarded by one sync.RWMutex, plus the Lock and Unlock pair and the
// Nl-prefixed (no-lock) methods for compound operations (built for the
// Ultima Redis-clone's allkeys-lru / volatile-lru eviction, one cache
// per shard — request note/06-lru-ts.md).
//
// Concurrency model — which method takes which lock:
//
//	Get        — the WRITE lock.  It looks like a read, but a hit
//	            re-marks the entry most-recently-used, which mutates
//	            the recency list; a read lock would race with every
//	            other Get.
//	Peek, Len, Capacity — the read lock (true reads).
//	Put, Delete, Clear  — the write lock.
//	All / Backward      — the read lock held only while the snapshot
//	                     is materialized; the returned iterator then
//	                     walks the snapshot, so it is safe to mutate
//	                     the cache (even from inside the loop).  This
//	                     differs from the plain package, whose
//	                     iterators walk the live list in O(1) extra
//	                     space.
//
// The eviction-veto callback (NewLruFunc) runs INSIDE Put while the
// write lock is held: the callback must not call any method on the
// same cache — that way lies deadlock — and it must not run for long,
// because it blocks every reader and writer.  There is deliberately
// no no-lock handle passed to the callback: the escape hatch for
// veto-then-act sequences is the caller-held Lock with the Nl* methods
// below.
//
// A nil *Lru and the zero value behave as an empty cache for every
// operation that has a sane answer: Get and Peek report not-found,
// Delete returns false, Len and Capacity are 0, Clear does nothing,
// and the iterators yield nothing; the nil guards run before any lock
// acquisition.  The package panics in exactly three situations, all
// programmer errors that cannot be handled where they occur — each
// message names the method and the fix:
//
//	NewLru / NewLruFunc with capacity < 1 — a cache needs room for at least one entry.
//	Put on a nil cache                    — a nil cache cannot store an entry.
//	Put on a zero-value cache             — no capacity; the message names the constructors.
//
// See the lru package documentation for the cache contracts (the veto
// scan, the soft cap, the eviction order) — this twin changes only the
// concurrency.
//
// Run the tests with -race.
package lru_ts

import (
	"iter"
	"sync"

	"github.com/pschlump/pluto/lru"
)

// kvPair is one snapshot entry for the iterators.
type kvPair[K comparable, V any] struct {
	key   K
	value V
}

// Lru is a capacity-bounded LRU cache guarded by one sync.RWMutex:
// the plain package's cache behind a pointer plus the lock.  Create it
// with NewLru or NewLruFunc; the zero value reads as an empty cache
// but cannot be Put into.  Do not copy an Lru (the mutex must not be
// duplicated) — always use *Lru.
type Lru[K comparable, V any] struct {
	inner *lru.Lru[K, V]
	lock  sync.RWMutex
}

// NewLru creates a cache that holds up to capacity entries; every
// entry is evictable.  It panics if capacity < 1.
// Complexity is O(1).
func NewLru[K comparable, V any](capacity int) *Lru[K, V] {
	return NewLruFunc[K, V](capacity, nil)
}

// NewLruFunc creates a cache that holds up to capacity entries and
// consults evictable before evicting an entry: evictable(k, v) == false
// vetoes the eviction (the scan skips the entry and tries the
// next-older one); a nil evictable means everything is evictable.
// When nothing is evictable the cache temporarily EXCEEDS its capacity
// — the soft cap, see the plain package's docs.  The callback runs
// with this cache's write lock held: it must not call back into the
// same cache (deadlock).  It panics if capacity < 1.
// Complexity is O(1).
func NewLruFunc[K comparable, V any](capacity int, evictable func(K, V) bool) *Lru[K, V] {
	if capacity < 1 {
		panic("lru_ts: NewLruFunc called with capacity < 1: a cache needs room for at least one entry")
	}
	return &Lru[K, V]{inner: lru.NewLruFunc(capacity, evictable)}
}

// Get returns the value stored under key and whether it was found.  A
// hit marks the entry most-recently-used — a mutation — so Get takes
// the WRITE lock, not the read lock (Peek is the read-lock form).
// A nil or zero-value cache reports not-found.
// Complexity is O(1).
func (c *Lru[K, V]) Get(key K) (V, bool) {
	if c == nil {
		var zero V
		return zero, false
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.inner.Get(key) // nil-tolerant: a zero-value inner reports not-found
}

// Peek returns the value stored under key and whether it was found,
// WITHOUT changing the recency order — a true read under the read
// lock.  A nil or zero-value cache reports not-found.
// Complexity is O(1).
func (c *Lru[K, V]) Peek(key K) (V, bool) {
	if c == nil {
		var zero V
		return zero, false
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.inner.Peek(key)
}

// Put inserts or updates the entry for key and marks it
// most-recently-used.  Before an insert, least-recently-used entries
// are evicted (skipping any the veto protects) until the cache is
// under capacity; when nothing is evictable the cache temporarily
// EXCEEDS its capacity — the soft cap, see the plain package's docs.
//
// Put takes the write lock, and the eviction-veto callback (if any)
// runs while it is held: the callback must not call any method on the
// same cache (deadlock).
//
// Put panics on a nil cache and on a zero-value cache (no capacity);
// the message names the constructors.
// Complexity is O(1), except that eviction with a vetoing callback is
// O(scan) in the worst case.
func (c *Lru[K, V]) Put(key K, value V) {
	if c == nil {
		panic("lru_ts: Put on a nil cache: a nil cache cannot store an entry; create it with NewLru or NewLruFunc")
	}
	if c.inner == nil {
		panic("lru_ts: Put on a zero-value cache: no capacity; create the cache with NewLru or NewLruFunc")
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	c.inner.Put(key, value)
}

// Delete removes the entry for key and reports whether it was present,
// under the write lock.  A nil or zero-value cache reports false.
// Complexity is O(1).
func (c *Lru[K, V]) Delete(key K) bool {
	if c == nil {
		return false
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.inner.Delete(key)
}

// Len returns the number of entries in the cache — which may exceed
// Capacity while every entry is veto-protected (the soft cap) — under
// the read lock.  A nil or zero-value cache has length 0.
// Complexity is O(1).
func (c *Lru[K, V]) Len() int {
	if c == nil {
		return 0
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.inner.Len()
}

// Capacity returns the (soft) capacity the cache was created with,
// under the read lock.  A nil or zero-value cache has capacity 0.
// Complexity is O(1).
func (c *Lru[K, V]) Capacity() int {
	if c == nil {
		return 0
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.inner.Capacity()
}

// Clear drops every entry, keeping the capacity and the veto callback,
// under the write lock.  A nil or zero-value cache does nothing.
// Complexity is O(n).
func (c *Lru[K, V]) Clear() {
	if c == nil {
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	c.inner.Clear()
}

// All returns a range-over-func iterator that yields each key/value
// pair of the cache, from the most recently used to the least
// recently used.  The pairs are SNAPSHOT-materialized under the read
// lock when All is called (the eager-snapshot convention — unlike the
// plain package's live-list walk); the iterator then runs lock-free
// over the copy, so it is safe to mutate the cache, even from inside
// the loop — changes made after the call are not visible.
//
//	for k, v := range c.All() {
//		...
//	}
//
// A single-variable range yields the KEY.  A nil or zero-value cache
// iterates as an empty one.
// Complexity is O(n) time and O(n) space for the snapshot.
func (c *Lru[K, V]) All() iter.Seq2[K, V] {
	return c.snapshot(true)
}

// Backward returns a range-over-func iterator that yields each
// key/value pair of the cache, from the least recently used to the
// most recently used — the snapshot form of All; see it for the
// concurrency contract.  A single-variable range yields the KEY.  A
// nil or zero-value cache iterates as an empty one.
// Complexity is O(n) time and O(n) space for the snapshot.
func (c *Lru[K, V]) Backward() iter.Seq2[K, V] {
	return c.snapshot(false)
}

// snapshot materializes the recency order into a slice under the read
// lock (mru first when mruFirst, lru first otherwise) and returns an
// iterator over the copy.
func (c *Lru[K, V]) snapshot(mruFirst bool) iter.Seq2[K, V] {
	if c == nil {
		return func(func(K, V) bool) {}
	}
	c.lock.RLock()
	var pairs []kvPair[K, V]
	if c.inner != nil {
		var src iter.Seq2[K, V]
		if mruFirst {
			src = c.inner.All()
		} else {
			src = c.inner.Backward()
		}
		for k, v := range src {
			pairs = append(pairs, kvPair[K, V]{key: k, value: v})
		}
	}
	c.lock.RUnlock()
	return func(yield func(K, V) bool) {
		for _, p := range pairs {
			if !yield(p.key, p.value) {
				return
			}
		}
	}
}

// Lock takes the real write lock, for compound operations — the Nl*
// methods below run unlocked while it is held.  The canonical compound
// is the atomic batch: Lock, NlPut the batch (each NlPut evicts to
// capacity as it goes), NlLen to observe the result, Unlock.  Calling
// a regular method while the lock is held deadlocks — use the Nl*
// forms inside.  A nil *Lru no-ops.
func (c *Lru[K, V]) Lock() {
	if c == nil {
		return
	}
	c.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  A nil *Lru no-ops.
func (c *Lru[K, V]) Unlock() {
	if c == nil {
		return
	}
	c.lock.Unlock()
}

// NlGet is the no-lock Get (it still re-marks the hit
// most-recently-used) — call it only while holding Lock.
// Complexity is O(1).
func (c *Lru[K, V]) NlGet(key K) (V, bool) { return c.inner.Get(key) }

// NlPeek is the no-lock Peek — call it only while holding Lock.
// Complexity is O(1).
func (c *Lru[K, V]) NlPeek(key K) (V, bool) { return c.inner.Peek(key) }

// NlPut is the no-lock Put, veto callback included — call it only
// while holding Lock (and the callback must still not call back into
// this cache).  Complexity is O(1), O(scan) with a vetoing callback.
func (c *Lru[K, V]) NlPut(key K, value V) { c.inner.Put(key, value) }

// NlDelete is the no-lock Delete — call it only while holding Lock.
// Complexity is O(1).
func (c *Lru[K, V]) NlDelete(key K) bool { return c.inner.Delete(key) }

// NlClear is the no-lock Clear — call it only while holding Lock.
// Complexity is O(n).
func (c *Lru[K, V]) NlClear() { c.inner.Clear() }

// NlLen is the no-lock Len — call it only while holding Lock.
// Complexity is O(1).
func (c *Lru[K, V]) NlLen() int { return c.inner.Len() }

// NlCapacity is the no-lock Capacity — call it only while holding
// Lock.
// Complexity is O(1).
func (c *Lru[K, V]) NlCapacity() int { return c.inner.Capacity() }
