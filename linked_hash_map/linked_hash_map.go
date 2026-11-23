/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package linked_hash_map implements a hash map with first-insertion iteration.
package linked_hash_map

import (
	"fmt"
	"hash/maphash"
	"iter"
)

// Entry is one key/value association.
type Entry[K any, V any] struct {
	Key   K `json:"key"`
	Value V `json:"value"`
}

type node[K any, V any] struct {
	entry      Entry[K, V]
	prev, next *node[K, V]
}

// Map is an insertion-ordered hash map. Its zero value is empty for reads and
// must be initialized by a constructor before mutation.
type Map[K any, V any] struct {
	buckets map[uint64][]*node[K, V]
	head    *node[K, V]
	tail    *node[K, V]
	equal   func(K, K) bool
	hash    func(K) uint64
	length  int
}

// New constructs an empty map using == and maphash.
func New[K comparable, V any]() *Map[K, V] {
	seed := maphash.MakeSeed()
	return NewFunc[K, V](
		func(a, b K) bool { return a == b },
		func(value K) uint64 { return maphash.Comparable(seed, value) },
	)
}

// NewFunc constructs an empty map with matching key equality/hash functions.
func NewFunc[K any, V any](equal func(K, K) bool, hash func(K) uint64) *Map[K, V] {
	if equal == nil || hash == nil {
		panic("linked_hash_map: NewFunc requires non-nil equality and hash functions")
	}
	return &Map[K, V]{buckets: make(map[uint64][]*node[K, V]), equal: equal, hash: hash}
}

// FromEntries constructs a map and puts entries in slice order.
func FromEntries[K comparable, V any](entries []Entry[K, V]) *Map[K, V] {
	result := New[K, V]()
	for _, entry := range entries {
		result.Put(entry.Key, entry.Value)
	}
	return result
}

func (m *Map[K, V]) writeContract(caller string) {
	if m == nil || m.buckets == nil {
		panic(fmt.Sprintf("linked_hash_map: %s on a nil or zero-value Map; use New or NewFunc", caller))
	}
}

func (m *Map[K, V]) find(key K) *node[K, V] {
	if m == nil || m.buckets == nil {
		return nil
	}
	for _, candidate := range m.buckets[m.hash(key)] {
		if m.equal(candidate.entry.Key, key) {
			return candidate
		}
	}
	return nil
}

// Eq returns the stored key equality function.
func (m *Map[K, V]) Eq() func(K, K) bool {
	if m == nil {
		return nil
	}
	return m.equal
}

// Hash returns the stored key hash function.
func (m *Map[K, V]) Hash() func(K) uint64 {
	if m == nil {
		return nil
	}
	return m.hash
}

// Get returns key's value and whether it is present.
func (m *Map[K, V]) Get(key K) (V, bool) {
	found := m.find(key)
	if found == nil {
		var zero V
		return zero, false
	}
	return found.entry.Value, true
}

// ContainsKey reports whether key is present.
func (m *Map[K, V]) ContainsKey(key K) bool { return m.find(key) != nil }

// Put inserts or updates key. Updating retains the first key representation and
// insertion position. It returns the previous value and presence flag.
func (m *Map[K, V]) Put(key K, value V) (V, bool) {
	m.writeContract("Put")
	if found := m.find(key); found != nil {
		old := found.entry.Value
		found.entry.Value = value
		return old, true
	}
	hash := m.hash(key)
	found := &node[K, V]{entry: Entry[K, V]{Key: key, Value: value}, prev: m.tail}
	if m.tail == nil {
		m.head = found
	} else {
		m.tail.next = found
	}
	m.tail = found
	m.buckets[hash] = append(m.buckets[hash], found)
	m.length++
	var zero V
	return zero, false
}

// Delete removes key and returns its former value and presence flag.
func (m *Map[K, V]) Delete(key K) (V, bool) {
	found := m.find(key)
	if found == nil {
		var zero V
		return zero, false
	}
	if found.prev == nil {
		m.head = found.next
	} else {
		found.prev.next = found.next
	}
	if found.next == nil {
		m.tail = found.prev
	} else {
		found.next.prev = found.prev
	}
	hash := m.hash(key)
	bucket := m.buckets[hash]
	for index, candidate := range bucket {
		if candidate == found {
			bucket = append(bucket[:index], bucket[index+1:]...)
			break
		}
	}
	if len(bucket) == 0 {
		delete(m.buckets, hash)
	} else {
		m.buckets[hash] = bucket
	}
	m.length--
	found.prev, found.next = nil, nil
	return found.entry.Value, true
}

// Len returns the number of associations.
func (m *Map[K, V]) Len() int {
	if m == nil {
		return 0
	}
	return m.length
}

// IsEmpty reports whether the map has no associations.
func (m *Map[K, V]) IsEmpty() bool { return m.Len() == 0 }

// Clear removes every association while retaining constructor functions.
func (m *Map[K, V]) Clear() {
	if m == nil || m.buckets == nil {
		return
	}
	clear(m.buckets)
	m.head, m.tail, m.length = nil, nil, 0
}

// Entries iterates in first-insertion order. Do not mutate a plain map during
// iteration. Entry values are copies and cannot update the map.
func (m *Map[K, V]) Entries() iter.Seq[Entry[K, V]] {
	return func(yield func(Entry[K, V]) bool) {
		if m == nil {
			return
		}
		for current := m.head; current != nil; current = current.next {
			if !yield(current.entry) {
				return
			}
		}
	}
}

// All is the key/value iterator form of Entries.
func (m *Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for entry := range m.Entries() {
			if !yield(entry.Key, entry.Value) {
				return
			}
		}
	}
}

// Keys iterates over keys in first-insertion order.
func (m *Map[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for entry := range m.Entries() {
			if !yield(entry.Key) {
				return
			}
		}
	}
}

// Values iterates over values in first-insertion key order.
func (m *Map[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for entry := range m.Entries() {
			if !yield(entry.Value) {
				return
			}
		}
	}
}

// EntriesSlice returns an independent insertion-ordered snapshot.
func (m *Map[K, V]) EntriesSlice() []Entry[K, V] {
	if m == nil {
		return nil
	}
	entries := make([]Entry[K, V], 0, m.length)
	for entry := range m.Entries() {
		entries = append(entries, entry)
	}
	return entries
}

// Clone returns an independent map with the same functions and iteration order.
func (m *Map[K, V]) Clone() *Map[K, V] {
	if m == nil {
		return nil
	}
	if m.buckets == nil {
		return &Map[K, V]{}
	}
	clone := NewFunc[K, V](m.equal, m.hash)
	for entry := range m.Entries() {
		clone.Put(entry.Key, entry.Value)
	}
	return clone
}

// Equal reports key/value content equality using equalValue for values. Key
// iteration order does not affect equality.
func (m *Map[K, V]) Equal(other *Map[K, V], equalValue func(V, V) bool) bool {
	if equalValue == nil || m.Len() != other.Len() {
		return false
	}
	for entry := range m.Entries() {
		value, found := other.Get(entry.Key)
		if !found || !equalValue(entry.Value, value) {
			return false
		}
	}
	return true
}
