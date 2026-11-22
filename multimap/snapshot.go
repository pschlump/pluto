/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"iter"
	"slices"
)

// MapSnapshotEntry is one key and its independent value snapshot.
type MapSnapshotEntry[K any, V any] struct {
	Key    K
	Values []V
}

// MapSnapshot is an immutable-by-convention snapshot returned by AsMap. Get and
// Entries copy value slices, so callers cannot mutate the snapshot or source.
// It is not a live, write-through view.
type MapSnapshot[K any, V any] struct {
	eq      func(K, K) bool
	entries []MapSnapshotEntry[K, V]
	size    int
}

func (c *core[K, V, C]) AsMap() MapSnapshot[K, V] {
	if c == nil || c.tab == nil {
		return MapSnapshot[K, V]{}
	}
	result := MapSnapshot[K, V]{eq: c.eq, size: c.Len()}
	for entry := range c.tab.Values() {
		result.entries = append(result.entries, MapSnapshotEntry[K, V]{
			Key:    entry.Key,
			Values: c.ops.values(entry.Collection),
		})
	}
	return result
}

// Get returns a copy of key's value collection.
func (m MapSnapshot[K, V]) Get(key K) ([]V, bool) {
	for _, entry := range m.entries {
		if m.eq(entry.Key, key) {
			return slices.Clone(entry.Values), true
		}
	}
	return nil, false
}

// Len returns the number of associations in the snapshot.
func (m MapSnapshot[K, V]) Len() int { return m.size }

// KeyLen returns the number of distinct keys in the snapshot.
func (m MapSnapshot[K, V]) KeyLen() int { return len(m.entries) }

// Keys iterates over distinct keys in snapshot order.
func (m MapSnapshot[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for _, entry := range m.entries {
			if !yield(entry.Key) {
				return
			}
		}
	}
}

// Entries iterates over key/list snapshots. Every Values slice is copied.
func (m MapSnapshot[K, V]) Entries() iter.Seq[MapSnapshotEntry[K, V]] {
	return func(yield func(MapSnapshotEntry[K, V]) bool) {
		for _, entry := range m.entries {
			entry.Values = slices.Clone(entry.Values)
			if !yield(entry) {
				return
			}
		}
	}
}
