/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package multiset_ts is the mutex-guarded twin of multiset.HashMultiset.
// Compound writes hold one write lock. Iterators and multi-set operations use
// snapshots and never hold two multiset locks at once.
package multiset_ts

import (
	"fmt"
	"hash/maphash"
	"iter"
	"slices"
	"sync"

	"github.com/pschlump/pluto/multiset"
)

// HashMultiset is a thread-safe hash multiset. Its zero value is empty and
// read-only; use a constructor before storing values.
type HashMultiset[T any] struct {
	inner multiset.HashMultiset[T]
	lock  sync.RWMutex
}

// NewHashMultiset creates an empty multiset using == and maphash.
func NewHashMultiset[T comparable]() *HashMultiset[T] {
	seed := maphash.MakeSeed()
	return newHashMultiset(
		func(a, b T) bool { return a == b },
		func(a T) uint64 { return maphash.Comparable(seed, a) },
	)
}

// NewHashMultisetFunc creates an empty multiset with custom equality and hash.
func NewHashMultisetFunc[T any](eq func(a, b T) bool, hash func(a T) uint64) *HashMultiset[T] {
	if eq == nil || hash == nil {
		panic("multiset_ts: NewHashMultisetFunc requires non-nil equality and hash functions")
	}
	return newHashMultiset(eq, hash)
}

func newHashMultiset[T any](eq func(a, b T) bool, hash func(a T) uint64) *HashMultiset[T] {
	return &HashMultiset[T]{inner: *multiset.NewHashMultisetFunc(eq, hash)}
}

// FromSlice creates a multiset containing one occurrence per slice element.
func FromSlice[T comparable](values []T) *HashMultiset[T] {
	result := NewHashMultiset[T]()
	result.AddSlice(values)
	return result
}

// FromSliceFunc is FromSlice with custom equality and hashing.
func FromSliceFunc[T any](eq func(a, b T) bool, hash func(a T) uint64, values []T) *HashMultiset[T] {
	result := NewHashMultisetFunc(eq, hash)
	result.AddSlice(values)
	return result
}

func (m *HashMultiset[T]) writeContract(caller string) {
	if m.inner.Eq() == nil || m.inner.Hash() == nil {
		panic(fmt.Sprintf("multiset_ts: %s on a zero-value HashMultiset (use NewHashMultiset or NewHashMultisetFunc)", caller))
	}
}

// Add adds occurrences and returns the resulting count.
func (m *HashMultiset[T]) Add(value T, occurrences int) int {
	if m == nil {
		var plain *multiset.HashMultiset[T]
		return plain.Add(value, occurrences)
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.Add(value, occurrences)
}

// Remove removes up to occurrences and returns the resulting count.
func (m *HashMultiset[T]) Remove(value T, occurrences int) int {
	if m == nil {
		return 0
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.Remove(value, occurrences)
}

// SetCount sets a value's count and returns its previous count.
func (m *HashMultiset[T]) SetCount(value T, count int) int {
	if m == nil {
		var plain *multiset.HashMultiset[T]
		return plain.SetCount(value, count)
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.SetCount(value, count)
}

// DeleteAll removes value and returns its previous count.
func (m *HashMultiset[T]) DeleteAll(value T) int {
	return m.SetCount(value, 0)
}

// Count returns the number of occurrences of value.
func (m *HashMultiset[T]) Count(value T) int {
	if m == nil {
		return 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.Count(value)
}

// Contains reports whether value has a positive count.
func (m *HashMultiset[T]) Contains(value T) bool {
	return m.Count(value) != 0
}

// Len returns the total occurrence count.
func (m *HashMultiset[T]) Len() int {
	if m == nil {
		return 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.Len()
}

// DistinctLen returns the number of distinct values.
func (m *HashMultiset[T]) DistinctLen() int {
	if m == nil {
		return 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.DistinctLen()
}

// IsEmpty reports whether there are no occurrences.
func (m *HashMultiset[T]) IsEmpty() bool {
	return m.Len() == 0
}

// Clear removes all values atomically.
func (m *HashMultiset[T]) Clear() {
	if m == nil {
		return
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	m.inner.Clear()
}

// AddSlice atomically adds one occurrence of each value.
func (m *HashMultiset[T]) AddSlice(values []T) {
	if len(values) == 0 {
		return
	}
	if m == nil {
		panic("multiset_ts: AddSlice called on a nil HashMultiset")
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	m.writeContract("AddSlice")
	m.inner.AddSlice(values)
}

// AddIter drains seq before atomically adding its values.
func (m *HashMultiset[T]) AddIter(seq iter.Seq[T]) {
	if seq == nil {
		return
	}
	var values []T
	for value := range seq {
		values = append(values, value)
	}
	m.AddSlice(values)
}

// RemoveSlice atomically removes up to one occurrence of each value.
func (m *HashMultiset[T]) RemoveSlice(values []T) {
	if m == nil {
		return
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	m.inner.RemoveSlice(values)
}

// RemoveIter drains seq before removing its values.
func (m *HashMultiset[T]) RemoveIter(seq iter.Seq[T]) {
	if seq == nil {
		return
	}
	var values []T
	for value := range seq {
		values = append(values, value)
	}
	m.RemoveSlice(values)
}

func snapshot[T any](m *HashMultiset[T]) *multiset.HashMultiset[T] {
	if m == nil {
		return nil
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.Clone()
}

// ContainsOccurrences reports whether m contains at least other's counts.
func (m *HashMultiset[T]) ContainsOccurrences(other *HashMultiset[T]) bool {
	otherSnapshot := snapshot(other)
	if m == nil {
		return otherSnapshot.Len() == 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.ContainsOccurrences(otherSnapshot)
}

// Equal reports count-aware content equality without holding two locks.
func (m *HashMultiset[T]) Equal(other *HashMultiset[T]) bool {
	return snapshot(m).Equal(snapshot(other))
}

// Clone returns an independent thread-safe copy.
func (m *HashMultiset[T]) Clone() *HashMultiset[T] {
	plain := snapshot(m)
	if plain == nil {
		return nil
	}
	return &HashMultiset[T]{inner: *plain}
}

// Entries returns a snapshot iterator over distinct entries.
func (m *HashMultiset[T]) Entries() iter.Seq[multiset.Entry[T]] {
	plain := snapshot(m)
	return slices.Values(plain.EntriesSlice())
}

// Elements returns a snapshot iterator over distinct values.
func (m *HashMultiset[T]) Elements() iter.Seq[T] {
	plain := snapshot(m)
	if plain == nil {
		return slices.Values([]T(nil))
	}
	values := make([]T, 0, plain.DistinctLen())
	for value := range plain.Elements() {
		values = append(values, value)
	}
	return slices.Values(values)
}

// All returns a snapshot iterator over every occurrence.
func (m *HashMultiset[T]) All() iter.Seq[T] {
	return slices.Values(snapshot(m).Slice())
}

// EntriesSlice returns a fresh snapshot of distinct entries.
func (m *HashMultiset[T]) EntriesSlice() []multiset.Entry[T] {
	return snapshot(m).EntriesSlice()
}

// Slice returns a fresh snapshot of every occurrence.
func (m *HashMultiset[T]) Slice() []T {
	return snapshot(m).Slice()
}

// HighestCountFirst returns a snapshot sorted by decreasing count.
func (m *HashMultiset[T]) HighestCountFirst() []multiset.Entry[T] {
	return snapshot(m).HighestCountFirst()
}

// Union atomically sets m to the count-wise maximum of yy and zz.
func (m *HashMultiset[T]) Union(yy, zz *HashMultiset[T]) {
	if m == nil {
		return
	}
	a, b := snapshot(yy), snapshot(zz)
	m.lock.Lock()
	defer m.lock.Unlock()
	m.inner.Union(a, b)
}

// Sum atomically sets m to the count-wise sum of yy and zz.
func (m *HashMultiset[T]) Sum(yy, zz *HashMultiset[T]) {
	if m == nil {
		return
	}
	a, b := snapshot(yy), snapshot(zz)
	m.lock.Lock()
	defer m.lock.Unlock()
	m.inner.Sum(a, b)
}

// Intersection atomically sets m to the count-wise minimum of yy and zz.
func (m *HashMultiset[T]) Intersection(yy, zz *HashMultiset[T]) {
	if m == nil {
		return
	}
	a, b := snapshot(yy), snapshot(zz)
	m.lock.Lock()
	defer m.lock.Unlock()
	m.inner.Intersection(a, b)
}

// Minus atomically sets m to yy minus zz, saturating at zero.
func (m *HashMultiset[T]) Minus(yy, zz *HashMultiset[T]) {
	if m == nil {
		return
	}
	a, b := snapshot(yy), snapshot(zz)
	m.lock.Lock()
	defer m.lock.Unlock()
	m.inner.Minus(a, b)
}
