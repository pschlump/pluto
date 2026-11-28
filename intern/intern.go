/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package intern provides strong concurrent canonicalization of generic values.
package intern

import (
	"hash/maphash"
	"sync"
)

// slot is one open-addressing table cell for NewFunc interners. The full hash
// is cached so probes reject mismatches without calling equal and growth never
// rehashes. A hash of exactly 0 is stored as 1, so hash 0 marks an empty cell.
type slot[T any] struct {
	hash  uint64
	value T
}

// nativeIndex is the index of an interner built by New: closures over a Go
// built-in map[T]T, which needs the comparable constraint that Interner's own
// type parameter cannot carry. Values are passed by value through ordinary
// function values; nothing is boxed into an interface.
type nativeIndex[T any] struct {
	get   func(T) (T, bool)
	put   func(T)
	clear func()
}

func newNativeIndex[T comparable]() *nativeIndex[T] {
	index := make(map[T]T)
	return &nativeIndex[T]{
		get: func(value T) (T, bool) {
			canonical, found := index[value]
			return canonical, found
		},
		put:   func(value T) { index[value] = value },
		clear: func() { clear(index) },
	}
}

// Interner stores one canonical representative for each equality class. Values
// are strongly retained until Clear or until the Interner becomes unreachable.
//
// New interners index through a built-in map (native); NewFunc interners use a
// linear-probing table of (hash, value) cells with a load factor of at most
// 3/4. Lookups run under the read lock; only a miss takes the write lock and
// re-checks before inserting.
type Interner[T any] struct {
	mu     sync.RWMutex
	native *nativeIndex[T]
	slots  []slot[T]
	equal  func(T, T) bool
	hash   func(T) uint64
	length int
	shift  uint8 // 64 - log2(len(slots)); home cell = (hash*fibMul) >> shift
}

const (
	fibMul       = 0x9E3779B97F4A7C15 // 2^64 / golden ratio: spreads weak caller hashes
	minTableBits = 3
)

// New constructs an Interner using == and maphash.
func New[T comparable]() *Interner[T] {
	seed := maphash.MakeSeed()
	i := NewFunc(
		func(a, b T) bool { return a == b },
		func(value T) uint64 { return maphash.Comparable(seed, value) },
	)
	i.native = newNativeIndex[T]()
	return i
}

// NewFunc constructs an Interner with matching equality and hash functions.
func NewFunc[T any](equal func(T, T) bool, hash func(T) uint64) *Interner[T] {
	if equal == nil || hash == nil {
		panic("intern: NewFunc requires non-nil equality and hash functions")
	}
	return &Interner[T]{equal: equal, hash: hash}
}

// cellHash is the cached form of a caller hash: 0 is reserved for empty cells.
func cellHash(hash uint64) uint64 {
	if hash == 0 {
		return 1
	}
	return hash
}

// lookup returns the cell index holding a value equal to value, or -1. The
// caller holds the lock (read or write) and hash is cellHash(i.hash(value)).
func (i *Interner[T]) lookup(value T, hash uint64) int {
	slots := i.slots
	if len(slots) == 0 {
		return -1
	}
	mask := len(slots) - 1
	index := int((hash * fibMul) >> i.shift)
	for {
		cell := &slots[index&mask]
		if cell.hash == 0 {
			return -1
		}
		if cell.hash == hash && i.equal(cell.value, value) {
			return index & mask
		}
		index++
	}
}

// place stores an absent cell in the first free cell of its probe sequence.
func (i *Interner[T]) place(cell slot[T]) {
	slots := i.slots
	mask := len(slots) - 1
	index := int((cell.hash * fibMul) >> i.shift)
	for slots[index&mask].hash != 0 {
		index++
	}
	slots[index&mask] = cell
}

// resize rebuilds the table with 2^bits cells using the cached hashes.
func (i *Interner[T]) resize(bits uint8) {
	old := i.slots
	i.slots = make([]slot[T], 1<<bits)
	i.shift = 64 - bits
	for _, cell := range old {
		if cell.hash != 0 {
			i.place(cell)
		}
	}
}

// Intern returns the first stored value equal to value. The canonical value is
// never replaced by a later equal value.
//
// Complexity is expected O(1).
func (i *Interner[T]) Intern(value T) T {
	if i == nil || i.hash == nil {
		panic("intern: Intern on a nil or zero-value Interner; use New or NewFunc")
	}
	if i.native != nil {
		i.mu.RLock()
		canonical, found := i.native.get(value)
		i.mu.RUnlock()
		if found {
			return canonical
		}
		return i.internNativeMiss(value)
	}
	hash := cellHash(i.hash(value))
	i.mu.RLock()
	index := i.lookup(value, hash)
	var canonical T
	if index >= 0 {
		canonical = i.slots[index].value
	}
	i.mu.RUnlock()
	if index >= 0 {
		return canonical
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if index = i.lookup(value, hash); index >= 0 {
		return i.slots[index].value
	}
	if (i.length+1)*4 > len(i.slots)*3 {
		bits := uint8(minTableBits)
		if len(i.slots) != 0 {
			bits = 64 - i.shift + 1
		}
		i.resize(bits)
	}
	i.place(slot[T]{hash: hash, value: value})
	i.length++
	return value
}

// internNativeMiss is the write-locked re-check and insert after a New
// interner's read-locked lookup missed.
func (i *Interner[T]) internNativeMiss(value T) T {
	i.mu.Lock()
	defer i.mu.Unlock()
	if canonical, found := i.native.get(value); found {
		return canonical
	}
	i.native.put(value)
	i.length++
	return value
}

// Contains reports whether an equal canonical value is stored.
//
// Complexity is expected O(1).
func (i *Interner[T]) Contains(value T) bool {
	if i == nil || i.hash == nil {
		return false
	}
	if i.native != nil {
		i.mu.RLock()
		_, found := i.native.get(value)
		i.mu.RUnlock()
		return found
	}
	hash := cellHash(i.hash(value))
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.lookup(value, hash) >= 0
}

// Len returns the number of canonical values.
//
// Complexity is O(1).
func (i *Interner[T]) Len() int {
	if i == nil {
		return 0
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.length
}

// IsEmpty reports whether no canonical values are stored.
func (i *Interner[T]) IsEmpty() bool { return i.Len() == 0 }

// Clear releases every canonical value and retains the constructor functions.
//
// Complexity is O(n) in the table capacity.
func (i *Interner[T]) Clear() {
	if i == nil || i.hash == nil {
		return
	}
	i.mu.Lock()
	if i.native != nil {
		i.native.clear()
	}
	clear(i.slots)
	i.length = 0
	i.mu.Unlock()
}
