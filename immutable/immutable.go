/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package immutable provides defensive, read-only collection snapshots. Values
// are safe for concurrent reads; referenced payloads are not deeply immutable.
package immutable

import (
	"errors"
	"hash/maphash"
	"iter"
)

var (
	ErrDuplicateKey   = errors.New("immutable: duplicate map key")
	ErrDuplicateValue = errors.New("immutable: duplicate bimap value")
)

func naturalFunctions[T comparable]() (func(T, T) bool, func(T) uint64) {
	seed := maphash.MakeSeed()
	return func(a, b T) bool { return a == b }, func(value T) uint64 {
		return maphash.Comparable(seed, value)
	}
}

// hashIndex maps each distinct key to its position in the owning value's
// backing slice. It is written only while a constructor runs and is read-only
// afterwards, so concurrent lookups need no locks.
//
// Natural constructors (comparable keys, builtin ==) back it with a builtin Go
// map: the runtime's specialised string and integer hashers beat any table
// driven through func values. Func constructors back it with funcTable, a
// purpose-built open-addressing table that stores each key's hash so probes
// compare hashes before calling the caller's equality function and growth
// never re-hashes. Both are reached through the same closures, so get stays
// small enough to inline into every lookup method.
type hashIndex[K any] struct {
	// lookup reports key's position. Expected O(1).
	lookup func(key K) (int, bool)
	// getOrAdd returns the position of an equal key already present (found
	// true), or records key at position (found false). Constructors only.
	getOrAdd func(key K, position int) (int, bool)
	// addUnique records key at position and reports false when an equal key
	// was already present. On false the stored position may have been
	// overwritten, so only constructors that abort on a duplicate (Map,
	// BiMap) may call it.
	addUnique func(key K, position int) bool
}

// newNaturalIndex returns a builtin-map index sized for hint distinct keys.
func newNaturalIndex[K comparable](hint int) *hashIndex[K] {
	positions := make(map[K]int, hint)
	return &hashIndex[K]{
		lookup: func(key K) (int, bool) {
			position, found := positions[key]
			return position, found
		},
		getOrAdd: func(key K, position int) (int, bool) {
			if existing, found := positions[key]; found {
				return existing, true
			}
			positions[key] = position
			return position, false
		},
		addUnique: func(key K, position int) bool {
			// One map operation: a duplicate leaves the length unchanged.
			before := len(positions)
			positions[key] = position
			return len(positions) > before
		},
	}
}

func newHashIndex[K any](equal func(K, K) bool, hash func(K) uint64) *hashIndex[K] {
	return newHashIndexSized(equal, hash, 0)
}

// newHashIndexSized returns a func-mode index presized for hint distinct keys.
func newHashIndexSized[K any](equal func(K, K) bool, hash func(K) uint64, hint int) *hashIndex[K] {
	if equal == nil || hash == nil {
		panic("immutable: nil equality or hash function")
	}
	size := 8
	for size < hint*2 && size < maxInt/4 {
		size <<= 1
	}
	table := &funcTable[K]{slots: make([]indexSlot[K], size), equal: equal, hash: hash}
	return &hashIndex[K]{
		lookup:   table.lookup,
		getOrAdd: table.getOrAdd,
		addUnique: func(key K, position int) bool {
			_, found := table.getOrAdd(key, position)
			return !found
		},
	}
}

// get reports key's position; a nil index holds nothing. Expected O(1).
func (index *hashIndex[K]) get(key K) (int, bool) {
	if index == nil {
		return 0, false
	}
	return index.lookup(key)
}

// add records key at position unless an equal key is already present, in
// which case the existing position is kept. Constructors only.
func (index *hashIndex[K]) add(key K, position int) bool {
	_, found := index.getOrAdd(key, position)
	return !found
}

// funcTable is the func-mode linear-probing table: power-of-two length, load
// at most 1/2, no deletes.
type funcTable[K any] struct {
	slots []indexSlot[K]
	count int
	equal func(K, K) bool
	hash  func(K) uint64
}

// indexSlot is one funcTable slot. position is stored plus one so the zero
// slot means empty.
type indexSlot[K any] struct {
	key      K
	hash     uint64
	position int
}

func (table *funcTable[K]) lookup(key K) (int, bool) {
	hash := table.hash(key)
	mask := uint64(len(table.slots) - 1)
	for slot := hash & mask; ; slot = (slot + 1) & mask {
		candidate := &table.slots[slot]
		if candidate.position == 0 {
			return 0, false
		}
		if candidate.hash == hash && table.equal(candidate.key, key) {
			return candidate.position - 1, true
		}
	}
}

func (table *funcTable[K]) getOrAdd(key K, position int) (int, bool) {
	hash := table.hash(key)
	mask := uint64(len(table.slots) - 1)
	slot := hash & mask
	for ; table.slots[slot].position != 0; slot = (slot + 1) & mask {
		candidate := &table.slots[slot]
		if candidate.hash == hash && table.equal(candidate.key, key) {
			return candidate.position - 1, true
		}
	}
	table.slots[slot] = indexSlot[K]{key: key, hash: hash, position: position + 1}
	table.count++
	if table.count*2 > len(table.slots) {
		table.grow()
	}
	return position, false
}

// grow doubles the table, reusing the stored hashes.
func (table *funcTable[K]) grow() {
	old := table.slots
	table.slots = make([]indexSlot[K], len(old)*2)
	mask := uint64(len(table.slots) - 1)
	for _, entry := range old {
		if entry.position == 0 {
			continue
		}
		slot := entry.hash & mask
		for table.slots[slot].position != 0 {
			slot = (slot + 1) & mask
		}
		table.slots[slot] = entry
	}
}

func sequence[T any](values []T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}
}

func reverseSequence[T any](values []T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for index := len(values) - 1; index >= 0; index-- {
			if !yield(values[index]) {
				return
			}
		}
	}
}

func copySubslice[T any](values []T) []T {
	if len(values)*4 < cap(values) {
		return append([]T(nil), values...)
	}
	return values
}
