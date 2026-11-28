/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset

import (
	"fmt"
	"hash/maphash"
	"iter"
	"sort"
)

// HashMultiset is a hash-backed multiset. Iteration order is unspecified.
// The zero value is empty and read-only.
type HashMultiset[T any] struct {
	tab  *table[T, int]
	eq   func(a, b T) bool
	hash func(a T) uint64
	size int
}

// NewHashMultiset creates an empty multiset using == and maphash.
func NewHashMultiset[T comparable]() *HashMultiset[T] {
	seed := maphash.MakeSeed()
	return newHashMultiset(
		func(a, b T) bool { return a == b },
		func(a T) uint64 { return maphash.Comparable(seed, a) },
	)
}

// NewHashMultisetFunc creates an empty multiset with caller-supplied equality
// and hashing. Equal values must have equal hashes. It panics for nil functions.
func NewHashMultisetFunc[T any](eq func(a, b T) bool, hash func(a T) uint64) *HashMultiset[T] {
	if eq == nil || hash == nil {
		panic("multiset: NewHashMultisetFunc requires non-nil equality and hash functions")
	}
	return newHashMultiset(eq, hash)
}

func newHashMultiset[T any](eq func(a, b T) bool, hash func(a T) uint64) *HashMultiset[T] {
	return &HashMultiset[T]{tab: newTable[T, int](eq, hash), eq: eq, hash: hash}
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
	if m == nil || m.tab == nil {
		panic(fmt.Sprintf("multiset: %s on a nil or zero-value HashMultiset (use NewHashMultiset or NewHashMultisetFunc)", caller))
	}
}

// Eq returns the stored equality function, or nil for a zero-value multiset.
func (m *HashMultiset[T]) Eq() func(a, b T) bool {
	if m == nil {
		return nil
	}
	return m.eq
}

// Hash returns the stored hash function, or nil for a zero-value multiset.
func (m *HashMultiset[T]) Hash() func(a T) uint64 {
	if m == nil {
		return nil
	}
	return m.hash
}

// Count returns the number of occurrences of value.
// Complexity is O(1) expected.
func (m *HashMultiset[T]) Count(value T) int {
	if m == nil {
		return 0
	}
	if count := m.tab.lookup(value); count != nil {
		return *count
	}
	return 0
}

// Add adds occurrences of value and returns its resulting count. Zero is a
// no-op. Negative counts and count or total-size overflow panic.
// Complexity is O(1) expected: one hash and one probe sequence.
func (m *HashMultiset[T]) Add(value T, occurrences int) int {
	checkedCount("Add", occurrences)
	if occurrences == 0 {
		return m.Count(value)
	}
	m.writeContract("Add")
	index, h, found := m.tab.find(value)
	if found {
		s := &m.tab.slots[index]
		checkedAddition(s.val, occurrences)
		checkedAddition(m.size, occurrences)
		s.key = value // a count change stores the latest representative
		s.val += occurrences
		m.size += occurrences
		return s.val
	}
	checkedAddition(m.size, occurrences)
	m.tab.insertAt(index, h, value, occurrences)
	m.size += occurrences
	return occurrences
}

// Remove removes up to occurrences of value and returns its resulting count.
// Removing more than the current count removes the value entirely.
// Complexity is O(1) expected.
func (m *HashMultiset[T]) Remove(value T, occurrences int) int {
	checkedCount("Remove", occurrences)
	if m == nil || m.tab == nil || m.tab.length == 0 {
		return 0
	}
	index, _, found := m.tab.find(value)
	if !found {
		return 0
	}
	s := &m.tab.slots[index]
	if occurrences == 0 {
		return s.val
	}
	if occurrences < s.val {
		s.key = value
		s.val -= occurrences
		m.size -= occurrences
		return s.val
	}
	m.size -= s.val
	m.tab.deleteAt(index)
	return 0
}

// SetCount changes value's count and returns its previous count. A zero count
// removes the value.
// Complexity is O(1) expected.
func (m *HashMultiset[T]) SetCount(value T, count int) int {
	checkedCount("SetCount", count)
	if count == 0 {
		if m == nil || m.tab == nil || m.tab.length == 0 {
			return 0
		}
		index, _, found := m.tab.find(value)
		if !found {
			return 0
		}
		old := m.tab.slots[index].val
		m.size -= old
		m.tab.deleteAt(index)
		return old
	}
	m.writeContract("SetCount")
	index, h, found := m.tab.find(value)
	if found {
		s := &m.tab.slots[index]
		old := s.val
		if count == old {
			return old
		}
		if count > old {
			checkedAddition(m.size, count-old)
		}
		s.key = value
		s.val = count
		m.size += count - old
		return old
	}
	checkedAddition(m.size, count)
	m.tab.insertAt(index, h, value, count)
	m.size += count
	return 0
}

// DeleteAll removes every occurrence of value and returns its previous count.
func (m *HashMultiset[T]) DeleteAll(value T) int {
	return m.SetCount(value, 0)
}

// Contains reports whether value has a positive count.
func (m *HashMultiset[T]) Contains(value T) bool {
	return m.Count(value) != 0
}

// Len returns the total number of occurrences.
func (m *HashMultiset[T]) Len() int {
	if m == nil {
		return 0
	}
	return m.size
}

// DistinctLen returns the number of values with positive counts.
func (m *HashMultiset[T]) DistinctLen() int {
	if m == nil || m.tab == nil {
		return 0
	}
	return m.tab.length
}

// IsEmpty reports whether the multiset has no occurrences.
func (m *HashMultiset[T]) IsEmpty() bool {
	return m.Len() == 0
}

// Clear removes all values while retaining the equality and hash functions.
func (m *HashMultiset[T]) Clear() {
	if m == nil || m.tab == nil {
		return
	}
	m.tab.clear()
	m.size = 0
}

// AddSlice adds one occurrence of each value. The batch is atomic if it panics.
func (m *HashMultiset[T]) AddSlice(values []T) {
	if len(values) == 0 {
		return
	}
	m.writeContract("AddSlice")
	next := m.Clone()
	for _, value := range values {
		next.Add(value, 1)
	}
	*m = *next
}

// AddIter drains seq, then adds one occurrence of each yielded value. Draining
// before mutation makes adding from the receiver's own iterator safe.
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

// RemoveSlice removes up to one occurrence of each value.
func (m *HashMultiset[T]) RemoveSlice(values []T) {
	for _, value := range values {
		m.Remove(value, 1)
	}
}

// RemoveIter drains seq, then removes up to one occurrence of each value.
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

// ContainsOccurrences reports whether every count in other is no greater than
// the corresponding count in m. An empty other is always contained.
func (m *HashMultiset[T]) ContainsOccurrences(other *HashMultiset[T]) bool {
	if other == nil || other.tab == nil {
		return true
	}
	for entry := range other.Entries() {
		if m.Count(entry.Value) < entry.Count {
			return false
		}
	}
	return true
}

// Equal reports count-aware content equality, independent of iteration order.
func (m *HashMultiset[T]) Equal(other *HashMultiset[T]) bool {
	if m.Len() != other.Len() || m.DistinctLen() != other.DistinctLen() {
		return false
	}
	return m.ContainsOccurrences(other)
}

// Clone returns an independent copy with the same equality and hash functions.
func (m *HashMultiset[T]) Clone() *HashMultiset[T] {
	if m == nil {
		return nil
	}
	clone := &HashMultiset[T]{eq: m.eq, hash: m.hash, size: m.size}
	if m.tab != nil {
		clone.tab = m.tab.clone()
	}
	return clone
}

// Entries iterates over distinct value/count entries in unspecified order.
func (m *HashMultiset[T]) Entries() iter.Seq[Entry[T]] {
	if m == nil || m.tab == nil {
		return func(func(Entry[T]) bool) {}
	}
	t := m.tab
	return func(yield func(Entry[T]) bool) {
		for i := 0; i < len(t.slots); i++ {
			if s := &t.slots[i]; s.hash != 0 && !yield(Entry[T]{Value: s.key, Count: s.val}) {
				return
			}
		}
	}
}

// Elements iterates over distinct values in unspecified order.
func (m *HashMultiset[T]) Elements() iter.Seq[T] {
	return func(yield func(T) bool) {
		for entry := range m.Entries() {
			if !yield(entry.Value) {
				return
			}
		}
	}
}

// All iterates over every occurrence in unspecified order. Values with equal
// counts are yielded consecutively; early termination propagates immediately.
func (m *HashMultiset[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for entry := range m.Entries() {
			for range entry.Count {
				if !yield(entry.Value) {
					return
				}
			}
		}
	}
}

// EntriesSlice returns a fresh entry slice in unspecified order.
func (m *HashMultiset[T]) EntriesSlice() []Entry[T] {
	if m == nil || m.tab == nil {
		return nil
	}
	entries := make([]Entry[T], 0, m.tab.length)
	for i := range m.tab.slots {
		if s := &m.tab.slots[i]; s.hash != 0 {
			entries = append(entries, Entry[T]{Value: s.key, Count: s.val})
		}
	}
	return entries
}

// Slice returns a fresh slice containing every occurrence.
func (m *HashMultiset[T]) Slice() []T {
	if m == nil {
		return nil
	}
	values := make([]T, 0, m.Len())
	for value := range m.All() {
		values = append(values, value)
	}
	return values
}

// HighestCountFirst returns a fresh entry slice sorted by decreasing count.
// Ties retain the hash table's unspecified iteration order.
func (m *HashMultiset[T]) HighestCountFirst() []Entry[T] {
	entries := m.EntriesSlice()
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Count > entries[j].Count
	})
	return entries
}

func (m *HashMultiset[T]) adopt(yy, zz *HashMultiset[T]) (func(T, T) bool, func(T) uint64) {
	if m != nil && m.eq != nil && m.hash != nil {
		return m.eq, m.hash
	}
	if yy != nil && yy.eq != nil && yy.hash != nil {
		return yy.eq, yy.hash
	}
	if zz != nil {
		return zz.eq, zz.hash
	}
	return nil, nil
}

func emptyHashLike[T any](eq func(T, T) bool, hash func(T) uint64) *HashMultiset[T] {
	if eq == nil || hash == nil {
		return &HashMultiset[T]{}
	}
	return newHashMultiset(eq, hash)
}

// Union sets m to the count-wise maximum of yy and zz.
func (m *HashMultiset[T]) Union(yy, zz *HashMultiset[T]) {
	if m == nil {
		return
	}
	a, b := yy.EntriesSlice(), zz.EntriesSlice()
	eq, hash := m.adopt(yy, zz)
	next := emptyHashLike(eq, hash)
	for _, entry := range a {
		next.SetCount(entry.Value, entry.Count)
	}
	for _, entry := range b {
		if entry.Count > next.Count(entry.Value) {
			next.SetCount(entry.Value, entry.Count)
		}
	}
	*m = *next
}

// Sum sets m to the count-wise sum of yy and zz. Overflow panics and leaves m
// unchanged.
func (m *HashMultiset[T]) Sum(yy, zz *HashMultiset[T]) {
	if m == nil {
		return
	}
	a, b := yy.EntriesSlice(), zz.EntriesSlice()
	eq, hash := m.adopt(yy, zz)
	next := emptyHashLike(eq, hash)
	for _, entry := range a {
		next.SetCount(entry.Value, entry.Count)
	}
	for _, entry := range b {
		next.Add(entry.Value, entry.Count)
	}
	*m = *next
}

// Intersection sets m to the count-wise minimum of yy and zz.
func (m *HashMultiset[T]) Intersection(yy, zz *HashMultiset[T]) {
	if m == nil {
		return
	}
	a := yy.EntriesSlice()
	eq, hash := m.adopt(yy, zz)
	next := emptyHashLike(eq, hash)
	for _, entry := range a {
		count := min(entry.Count, zz.Count(entry.Value))
		if count > 0 {
			next.SetCount(entry.Value, count)
		}
	}
	*m = *next
}

// Minus sets m to yy's counts minus zz's counts, saturating at zero.
func (m *HashMultiset[T]) Minus(yy, zz *HashMultiset[T]) {
	if m == nil {
		return
	}
	a := yy.EntriesSlice()
	eq, hash := m.adopt(yy, zz)
	next := emptyHashLike(eq, hash)
	for _, entry := range a {
		count := entry.Count - zz.Count(entry.Value)
		if count > 0 {
			next.SetCount(entry.Value, count)
		}
	}
	*m = *next
}
