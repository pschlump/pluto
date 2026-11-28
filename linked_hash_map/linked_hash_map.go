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

// slot is one open-addressing table cell. The full key hash is cached so that
// probes reject mismatches without loading the node and growth never rehashes.
// An empty cell has a nil node.
type slot[K any, V any] struct {
	hash uint64
	node *node[K, V]
}

// Map is an insertion-ordered hash map. Its zero value is empty for reads and
// must be initialized by a constructor before mutation.
//
// A New map indexes keys through a Go built-in map (native). A NewFunc map
// uses a linear-probing table of (hash, node) cells with backward-shift
// deletion (no tombstones), allocated lazily on the first insertion. Either
// way the insertion order lives in the doubly linked node list.
type Map[K any, V any] struct {
	native *nativeIndex[K, V] // non-nil for New: Go map index, slots unused
	slots  []slot[K, V]
	head   *node[K, V]
	tail   *node[K, V]
	equal  func(K, K) bool
	hash   func(K) uint64
	length int
	shift  uint8 // 64 - log2(len(slots)); home cell = (hash*fibMul) >> shift
}

const (
	fibMul       = 0x9E3779B97F4A7C15 // 2^64 / golden ratio: spreads weak caller hashes
	minTableBits = 3
)

// nativeIndex is the key index of a map built by New: closures over a Go
// built-in map[K]*node, which needs the comparable constraint that Map's own
// type parameter cannot carry. Keys are passed by value through ordinary
// function values; nothing is boxed into an interface.
type nativeIndex[K any, V any] struct {
	get   func(K) *node[K, V]
	put   func(K, *node[K, V])
	del   func(K)
	clear func()
	fresh func() *nativeIndex[K, V]
}

func newNativeIndex[K comparable, V any]() *nativeIndex[K, V] {
	index := make(map[K]*node[K, V])
	return &nativeIndex[K, V]{
		get:   func(key K) *node[K, V] { return index[key] },
		put:   func(key K, found *node[K, V]) { index[key] = found },
		del:   func(key K) { delete(index, key) },
		clear: func() { clear(index) },
		fresh: newNativeIndex[K, V],
	}
}

// New constructs an empty map using == and maphash.
func New[K comparable, V any]() *Map[K, V] {
	seed := maphash.MakeSeed()
	m := NewFunc[K, V](
		func(a, b K) bool { return a == b },
		func(value K) uint64 { return maphash.Comparable(seed, value) },
	)
	m.native = newNativeIndex[K, V]()
	return m
}

// NewFunc constructs an empty map with matching key equality/hash functions.
func NewFunc[K any, V any](equal func(K, K) bool, hash func(K) uint64) *Map[K, V] {
	if equal == nil || hash == nil {
		panic("linked_hash_map: NewFunc requires non-nil equality and hash functions")
	}
	return &Map[K, V]{equal: equal, hash: hash}
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
	if m == nil || m.hash == nil {
		panic(fmt.Sprintf("linked_hash_map: %s on a nil or zero-value Map; use New or NewFunc", caller))
	}
}

// lookup returns the cell index holding key, or -1. hash must be m.hash(key).
func (m *Map[K, V]) lookup(key K, hash uint64) int {
	slots := m.slots
	if len(slots) == 0 {
		return -1
	}
	mask := len(slots) - 1
	index := int((hash * fibMul) >> m.shift)
	for {
		cell := &slots[index&mask]
		if cell.node == nil {
			return -1
		}
		if cell.hash == hash && m.equal(cell.node.entry.Key, key) {
			return index & mask
		}
		index++
	}
}

// find returns key's node or nil.
func (m *Map[K, V]) find(key K) *node[K, V] {
	if m == nil || m.hash == nil || m.length == 0 {
		return nil
	}
	if m.native != nil {
		return m.native.get(key)
	}
	index := m.lookup(key, m.hash(key))
	if index < 0 {
		return nil
	}
	return m.slots[index].node
}

// place stores a cell known to be absent into the first free cell of its probe
// sequence. The table must have a free cell.
func (m *Map[K, V]) place(cell slot[K, V]) {
	slots := m.slots
	mask := len(slots) - 1
	index := int((cell.hash * fibMul) >> m.shift)
	for slots[index&mask].node != nil {
		index++
	}
	slots[index&mask] = cell
}

// resize rebuilds the index with 2^bits cells using the cached hashes.
func (m *Map[K, V]) resize(bits uint8) {
	old := m.slots
	m.slots = make([]slot[K, V], 1<<bits)
	m.shift = 64 - bits
	for _, cell := range old {
		if cell.node != nil {
			m.place(cell)
		}
	}
}

// removeAt deletes cell index by backward-shifting the rest of its cluster.
func (m *Map[K, V]) removeAt(index int) {
	slots := m.slots
	mask := len(slots) - 1
	hole := index
	for next := (hole + 1) & mask; slots[next].node != nil; next = (next + 1) & mask {
		home := int((slots[next].hash * fibMul) >> m.shift)
		// Move next into the hole unless its home lies cyclically in (hole, next].
		if (next-home)&mask >= (next-hole)&mask {
			slots[hole] = slots[next]
			hole = next
		}
	}
	slots[hole] = slot[K, V]{}
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
//
// Complexity is expected O(1).
func (m *Map[K, V]) Get(key K) (V, bool) {
	found := m.find(key)
	if found == nil {
		var zero V
		return zero, false
	}
	return found.entry.Value, true
}

// ContainsKey reports whether key is present.
//
// Complexity is expected O(1).
func (m *Map[K, V]) ContainsKey(key K) bool { return m.find(key) != nil }

// Put inserts or updates key. Updating retains the first key representation and
// insertion position. It returns the previous value and presence flag.
//
// Complexity is expected O(1) (amortized over table growth).
func (m *Map[K, V]) Put(key K, value V) (V, bool) {
	m.writeContract("Put")
	if m.native != nil {
		if found := m.native.get(key); found != nil {
			old := found.entry.Value
			found.entry.Value = value
			return old, true
		}
		m.native.put(key, m.link(key, value))
		var zero V
		return zero, false
	}
	hash := m.hash(key)
	if index := m.lookup(key, hash); index >= 0 {
		found := m.slots[index].node
		old := found.entry.Value
		found.entry.Value = value
		return old, true
	}
	// Keep the load factor at or below 3/4.
	if (m.length+1)*4 > len(m.slots)*3 {
		bits := uint8(minTableBits)
		if len(m.slots) != 0 {
			bits = 64 - m.shift + 1
		}
		m.resize(bits)
	}
	m.place(slot[K, V]{hash: hash, node: m.link(key, value)})
	var zero V
	return zero, false
}

// link appends a new node to the insertion-order list and counts it.
func (m *Map[K, V]) link(key K, value V) *node[K, V] {
	found := &node[K, V]{entry: Entry[K, V]{Key: key, Value: value}, prev: m.tail}
	if m.tail == nil {
		m.head = found
	} else {
		m.tail.next = found
	}
	m.tail = found
	m.length++
	return found
}

// Delete removes key and returns its former value and presence flag.
//
// Complexity is expected O(1).
func (m *Map[K, V]) Delete(key K) (V, bool) {
	var zero V
	if m == nil || m.hash == nil || m.length == 0 {
		return zero, false
	}
	var found *node[K, V]
	if m.native != nil {
		if found = m.native.get(key); found == nil {
			return zero, false
		}
		m.native.del(key)
	} else {
		index := m.lookup(key, m.hash(key))
		if index < 0 {
			return zero, false
		}
		found = m.slots[index].node
		m.removeAt(index)
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
//
// Complexity is O(n) in the index capacity.
func (m *Map[K, V]) Clear() {
	if m == nil || m.hash == nil {
		return
	}
	if m.native != nil {
		m.native.clear()
	}
	clear(m.slots)
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
		if m == nil {
			return
		}
		for current := m.head; current != nil; current = current.next {
			if !yield(current.entry.Key, current.entry.Value) {
				return
			}
		}
	}
}

// Keys iterates over keys in first-insertion order.
func (m *Map[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		if m == nil {
			return
		}
		for current := m.head; current != nil; current = current.next {
			if !yield(current.entry.Key) {
				return
			}
		}
	}
}

// Values iterates over values in first-insertion key order.
func (m *Map[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		if m == nil {
			return
		}
		for current := m.head; current != nil; current = current.next {
			if !yield(current.entry.Value) {
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
	for current := m.head; current != nil; current = current.next {
		entries = append(entries, current.entry)
	}
	return entries
}

// Clone returns an independent map with the same functions, index kind and
// iteration order.
//
// Complexity is expected O(n).
func (m *Map[K, V]) Clone() *Map[K, V] {
	if m == nil {
		return nil
	}
	if m.hash == nil {
		return &Map[K, V]{}
	}
	clone := m.empty()
	for current := m.head; current != nil; current = current.next {
		clone.Put(current.entry.Key, current.entry.Value)
	}
	return clone
}

// empty returns a new empty map with the same functions and index kind.
func (m *Map[K, V]) empty() *Map[K, V] {
	result := NewFunc[K, V](m.equal, m.hash)
	if m.native != nil {
		result.native = m.native.fresh()
	}
	return result
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
