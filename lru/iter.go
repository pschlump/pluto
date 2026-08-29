/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Range-over-func iterators.  Both walk the live recency list — All
// from the front (most recently used), Backward from the back (least
// recently used) — so they cost O(n) time with O(1) extra space and no
// snapshot copy.  The price is the usual one for a plain package: the
// cache must not be modified while an iterator is running, and note
// that Get counts as a modification (it re-marks the hit entry).

package lru

import "iter"

// All returns a range-over-func iterator that yields each key/value
// pair of the cache, from the most recently used to the least recently
// used.
//
//	for k, v := range c.All() {
//		...
//	}
//
// A single-variable range yields the KEY.  The cache must not be
// modified while the iterator is running.  A nil cache iterates as an
// empty one.
// Complexity is O(n).
func (c *Lru[K, V]) All() iter.Seq2[K, V] {
	if c == nil || c.ll == nil {
		return func(func(K, V) bool) {} // a nil cache iterates as an empty one
	}
	return func(yield func(K, V) bool) {
		for e := c.ll.Front(); e != nil; e = e.Next() {
			ent := e.Value.(lruEntry[K, V])
			if !yield(ent.key, ent.value) {
				return
			}
		}
	}
}

// Backward returns a range-over-func iterator that yields each
// key/value pair of the cache, from the least recently used to the most
// recently used.
//
//	for k, v := range c.Backward() {
//		...
//	}
//
// A single-variable range yields the KEY.  The cache must not be
// modified while the iterator is running.  A nil cache iterates as an
// empty one.
// Complexity is O(n).
func (c *Lru[K, V]) Backward() iter.Seq2[K, V] {
	if c == nil || c.ll == nil {
		return func(func(K, V) bool) {} // a nil cache iterates as an empty one
	}
	return func(yield func(K, V) bool) {
		for e := c.ll.Back(); e != nil; e = e.Prev() {
			ent := e.Value.(lruEntry[K, V])
			if !yield(ent.key, ent.value) {
				return
			}
		}
	}
}
