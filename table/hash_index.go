/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package table

import (
	"iter"
	"math/bits"
)

// hashSlot is one open-addressing slot. hash == 0 marks an empty slot; a key
// whose hash is exactly 0 is stored under 1 (see fixHash).
type hashSlot[K any, V any] struct {
	hash  uint64
	key   K
	value V
}

// hashIndex is the private key/value index behind HashTable, ArrayTable and
// ImmutableTable. It is a linear-probing table with backward-shift deletion
// (no tombstones), a power-of-two slot array and a maximum load of 3/4. Each
// slot caches its key's hash, so a probe calls equal only on a full hash match
// and growth never re-hashes a key. The table stores the caller's equality and
// hash functions directly: one indirect call each per probe, with no adapter
// closure in between.
//
// The zero slot array is legal (an index with no entries); the first insert
// allocates minHashSlots slots.
type hashIndex[K any, V any] struct {
	slots []hashSlot[K, V]
	count int
	shift uint8 // 64 - log2(len(slots)); home slot = (hash * fib) >> shift
	equal func(K, K) bool
	hash  func(K) uint64
}

const (
	minHashSlots = 8
	fibonacci64  = 0x9E3779B97F4A7C15
)

func newHashIndex[K any, V any](equal func(K, K) bool, hash func(K) uint64) *hashIndex[K, V] {
	return &hashIndex[K, V]{equal: equal, hash: hash}
}

// fixHash remaps a hash of exactly 0, which marks an empty slot.
func fixHash(hash uint64) uint64 {
	if hash == 0 {
		return 1
	}
	return hash
}

// home spreads the cached hash over the slot array with a Fibonacci multiply,
// so weak caller hashes (identity integers, sequential ids) still distribute.
func (index *hashIndex[K, V]) home(hash uint64) int {
	return int((hash * fibonacci64) >> index.shift)
}

// slotsFor returns the power-of-two slot count that holds n entries at a load
// of at most 3/4.
func slotsFor(n int) int {
	size := minHashSlots
	for size*3 < n*4 {
		size <<= 1
	}
	return size
}

// allocate replaces the slot array with an empty array of size slots.
func (index *hashIndex[K, V]) allocate(size int) {
	index.slots = make([]hashSlot[K, V], size)
	index.shift = uint8(64 - bits.TrailingZeros(uint(size)))
}

// reserve makes room for n entries without further growth. It is only called
// on an empty index.
func (index *hashIndex[K, V]) reserve(n int) {
	if n > 0 {
		index.allocate(slotsFor(n))
	}
}

// find returns the slot holding key, or the empty slot that ends its probe
// chain, and whether key was found. It requires a non-empty slot array.
func (index *hashIndex[K, V]) find(hash uint64, key K) (int, bool) {
	mask := len(index.slots) - 1
	for position := index.home(hash); ; position = (position + 1) & mask {
		slot := &index.slots[position]
		if slot.hash == 0 {
			return position, false
		}
		if slot.hash == hash && index.equal(slot.key, key) {
			return position, true
		}
	}
}

// lookup returns the slot position of key. Complexity is expected O(1).
func (index *hashIndex[K, V]) lookup(key K) (int, bool) {
	if index == nil || index.count == 0 {
		return 0, false
	}
	return index.find(fixHash(index.hash(key)), key)
}

// get returns the value stored for key. Complexity is expected O(1).
func (index *hashIndex[K, V]) get(key K) (V, bool) {
	position, found := index.lookup(key)
	if !found {
		var zero V
		return zero, false
	}
	return index.slots[position].value, true
}

// put inserts or replaces key's entry (both the stored key and its value, as an
// equal key replaces the whole entry) and returns the previous value. The key
// is hashed once. Complexity is expected amortized O(1).
func (index *hashIndex[K, V]) put(key K, value V) (V, bool) {
	hash := fixHash(index.hash(key))
	position, found := index.findOrAllocate(hash, key)
	if found {
		slot := &index.slots[position]
		old := slot.value
		slot.key, slot.value = key, value
		return old, true
	}
	index.insertAt(position, hash, key, value)
	var zero V
	return zero, false
}

// findOrAllocate is find for a write path: it allocates the first slot array
// of an empty index so the returned position is always usable by insertAt.
func (index *hashIndex[K, V]) findOrAllocate(hash uint64, key K) (int, bool) {
	if index.slots == nil {
		index.allocate(minHashSlots)
	}
	return index.find(hash, key)
}

// insertAt stores a new entry at position, the empty slot that a failed find
// for (hash, key) returned, growing first when the entry would exceed the 3/4
// load (growth relocates the target slot). It returns the entry's final slot.
func (index *hashIndex[K, V]) insertAt(position int, hash uint64, key K, value V) int {
	if (index.count+1)*4 > len(index.slots)*3 {
		index.grow()
		position = index.emptySlot(hash)
	}
	index.slots[position] = hashSlot[K, V]{hash: hash, key: key, value: value}
	index.count++
	return position
}

// insertUnique stores an entry whose key is known to be absent, under its
// already-fixed hash. No equality or hash call is made. Bulk builders
// (Transpose, Clone) use it with hashes cached in the source index.
func (index *hashIndex[K, V]) insertUnique(hash uint64, key K, value V) {
	if index.slots == nil {
		index.allocate(minHashSlots)
	} else if (index.count+1)*4 > len(index.slots)*3 {
		index.grow()
	}
	index.slots[index.emptySlot(hash)] = hashSlot[K, V]{hash: hash, key: key, value: value}
	index.count++
}

// emptySlot returns the first empty slot on hash's probe chain.
func (index *hashIndex[K, V]) emptySlot(hash uint64) int {
	mask := len(index.slots) - 1
	position := index.home(hash)
	for index.slots[position].hash != 0 {
		position = (position + 1) & mask
	}
	return position
}

// grow doubles the slot array, moving entries by their cached hashes.
func (index *hashIndex[K, V]) grow() {
	old := index.slots
	index.allocate(len(old) * 2)
	for position := range old {
		if old[position].hash != 0 {
			index.slots[index.emptySlot(old[position].hash)] = old[position]
		}
	}
}

// delete removes key's entry and returns its value. The slot array never
// shrinks. Complexity is expected O(1).
func (index *hashIndex[K, V]) delete(key K) (V, bool) {
	position, found := index.lookup(key)
	if !found {
		var zero V
		return zero, false
	}
	old := index.slots[position].value
	index.removeAt(position)
	return old, true
}

// removeAt empties a slot with backward-shift deletion: every later entry of
// the cluster whose home is not cyclically inside (hole, current] moves back
// into the hole, so lookups never need tombstones.
func (index *hashIndex[K, V]) removeAt(hole int) {
	mask := len(index.slots) - 1
	for current := (hole + 1) & mask; index.slots[current].hash != 0; current = (current + 1) & mask {
		home := index.home(index.slots[current].hash)
		// The entry may move into the hole when its home lies at or before
		// the hole along the probe direction, i.e. outside (hole, current].
		if (current-home)&mask >= (current-hole)&mask {
			index.slots[hole] = index.slots[current]
			hole = current
		}
	}
	index.slots[hole] = hashSlot[K, V]{}
	index.count--
}

func (index *hashIndex[K, V]) len() int {
	if index == nil {
		return 0
	}
	return index.count
}

// clear removes every entry and releases the slot array.
func (index *hashIndex[K, V]) clear() {
	index.slots = nil
	index.shift = 0
	index.count = 0
}

// clone returns an independent index with the same functions and entries.
// The slot array is copied verbatim: no key is re-hashed. Values are copied by
// assignment.
func (index *hashIndex[K, V]) clone() *hashIndex[K, V] {
	result := &hashIndex[K, V]{count: index.count, shift: index.shift, equal: index.equal, hash: index.hash}
	if index.slots != nil {
		result.slots = append([]hashSlot[K, V](nil), index.slots...)
	}
	return result
}

// all iterates the entries in slot order. Do not mutate while iterating.
func (index *hashIndex[K, V]) all() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if index == nil {
			return
		}
		for position := range index.slots {
			slot := &index.slots[position]
			if slot.hash != 0 && !yield(slot.key, slot.value) {
				return
			}
		}
	}
}
