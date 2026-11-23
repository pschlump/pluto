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

type entry[T any] struct {
	value T
}

// Interner stores one canonical representative for each equality class. Values
// are strongly retained until Clear or until the Interner becomes unreachable.
type Interner[T any] struct {
	mu      sync.RWMutex
	entries map[uint64][]entry[T]
	equal   func(T, T) bool
	hash    func(T) uint64
	length  int
}

// New constructs an Interner using == and maphash.
func New[T comparable]() *Interner[T] {
	seed := maphash.MakeSeed()
	return NewFunc(
		func(a, b T) bool { return a == b },
		func(value T) uint64 { return maphash.Comparable(seed, value) },
	)
}

// NewFunc constructs an Interner with matching equality and hash functions.
func NewFunc[T any](equal func(T, T) bool, hash func(T) uint64) *Interner[T] {
	if equal == nil || hash == nil {
		panic("intern: NewFunc requires non-nil equality and hash functions")
	}
	return &Interner[T]{entries: make(map[uint64][]entry[T]), equal: equal, hash: hash}
}

// Intern returns the first stored value equal to value. The canonical value is
// never replaced by a later equal value.
func (i *Interner[T]) Intern(value T) T {
	if i == nil || i.entries == nil {
		panic("intern: Intern on a nil or zero-value Interner; use New or NewFunc")
	}
	hash := i.hash(value)
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, candidate := range i.entries[hash] {
		if i.equal(candidate.value, value) {
			return candidate.value
		}
	}
	i.entries[hash] = append(i.entries[hash], entry[T]{value: value})
	i.length++
	return value
}

// Contains reports whether an equal canonical value is stored.
func (i *Interner[T]) Contains(value T) bool {
	if i == nil || i.entries == nil {
		return false
	}
	hash := i.hash(value)
	i.mu.RLock()
	defer i.mu.RUnlock()
	for _, candidate := range i.entries[hash] {
		if i.equal(candidate.value, value) {
			return true
		}
	}
	return false
}

// Len returns the number of canonical values.
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
func (i *Interner[T]) Clear() {
	if i == nil || i.entries == nil {
		return
	}
	i.mu.Lock()
	clear(i.entries)
	i.length = 0
	i.mu.Unlock()
}
