/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset

import (
	"fmt"
	"iter"
	"sort"

	"github.com/pschlump/pluto/linked_hash_map"
)

// LinkedHashMultiset is a hash multiset whose distinct values iterate in first
// insertion order. Removing a value entirely and reinserting it appends it.
type LinkedHashMultiset[T any] struct {
	counts *linked_hash_map.Map[T, int]
	size   int
}

func NewLinkedHashMultiset[T comparable]() *LinkedHashMultiset[T] {
	return &LinkedHashMultiset[T]{counts: linked_hash_map.New[T, int]()}
}

func NewLinkedHashMultisetFunc[T any](eq func(T, T) bool, hash func(T) uint64) *LinkedHashMultiset[T] {
	return &LinkedHashMultiset[T]{counts: linked_hash_map.NewFunc[T, int](eq, hash)}
}

func FromSliceLinked[T comparable](values []T) *LinkedHashMultiset[T] {
	result := NewLinkedHashMultiset[T]()
	result.AddSlice(values)
	return result
}

func FromSliceLinkedFunc[T any](eq func(T, T) bool, hash func(T) uint64, values []T) *LinkedHashMultiset[T] {
	result := NewLinkedHashMultisetFunc(eq, hash)
	result.AddSlice(values)
	return result
}

func (m *LinkedHashMultiset[T]) writeContract(caller string) {
	if m == nil || m.counts == nil {
		panic(fmt.Sprintf("multiset: %s on a nil or zero-value LinkedHashMultiset; use a constructor", caller))
	}
}

func (m *LinkedHashMultiset[T]) Eq() func(T, T) bool {
	if m == nil || m.counts == nil {
		return nil
	}
	return m.counts.Eq()
}

func (m *LinkedHashMultiset[T]) Hash() func(T) uint64 {
	if m == nil || m.counts == nil {
		return nil
	}
	return m.counts.Hash()
}

func (m *LinkedHashMultiset[T]) Count(value T) int {
	if m == nil || m.counts == nil {
		return 0
	}
	count, _ := m.counts.Get(value)
	return count
}

func (m *LinkedHashMultiset[T]) Add(value T, occurrences int) int {
	checkedCount("Add", occurrences)
	old := m.Count(value)
	if occurrences == 0 {
		return old
	}
	m.writeContract("Add")
	checkedAddition(old, occurrences)
	checkedAddition(m.size, occurrences)
	m.counts.Put(value, old+occurrences)
	m.size += occurrences
	return old + occurrences
}

func (m *LinkedHashMultiset[T]) Remove(value T, occurrences int) int {
	checkedCount("Remove", occurrences)
	old := m.Count(value)
	if occurrences == 0 || old == 0 {
		return old
	}
	if occurrences >= old {
		m.counts.Delete(value)
		m.size -= old
		return 0
	}
	m.counts.Put(value, old-occurrences)
	m.size -= occurrences
	return old - occurrences
}

func (m *LinkedHashMultiset[T]) SetCount(value T, count int) int {
	checkedCount("SetCount", count)
	old := m.Count(value)
	if old == count {
		return old
	}
	if count == 0 {
		m.counts.Delete(value)
		m.size -= old
		return old
	}
	m.writeContract("SetCount")
	if count > old {
		checkedAddition(m.size, count-old)
	}
	m.counts.Put(value, count)
	m.size += count - old
	return old
}

func (m *LinkedHashMultiset[T]) DeleteAll(value T) int { return m.SetCount(value, 0) }
func (m *LinkedHashMultiset[T]) Contains(value T) bool { return m.Count(value) != 0 }
func (m *LinkedHashMultiset[T]) Len() int {
	if m == nil {
		return 0
	}
	return m.size
}
func (m *LinkedHashMultiset[T]) DistinctLen() int {
	if m == nil || m.counts == nil {
		return 0
	}
	return m.counts.Len()
}
func (m *LinkedHashMultiset[T]) IsEmpty() bool { return m.Len() == 0 }
func (m *LinkedHashMultiset[T]) Clear() {
	if m == nil || m.counts == nil {
		return
	}
	m.counts.Clear()
	m.size = 0
}

func (m *LinkedHashMultiset[T]) AddSlice(values []T) {
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

func (m *LinkedHashMultiset[T]) AddIter(seq iter.Seq[T]) {
	if seq == nil {
		return
	}
	values := []T{}
	for value := range seq {
		values = append(values, value)
	}
	m.AddSlice(values)
}

func (m *LinkedHashMultiset[T]) RemoveSlice(values []T) {
	for _, value := range values {
		m.Remove(value, 1)
	}
}

func (m *LinkedHashMultiset[T]) RemoveIter(seq iter.Seq[T]) {
	if seq == nil {
		return
	}
	values := []T{}
	for value := range seq {
		values = append(values, value)
	}
	m.RemoveSlice(values)
}

func (m *LinkedHashMultiset[T]) ContainsOccurrences(other *LinkedHashMultiset[T]) bool {
	if other == nil {
		return true
	}
	for entry := range other.Entries() {
		if m.Count(entry.Value) < entry.Count {
			return false
		}
	}
	return true
}

func (m *LinkedHashMultiset[T]) Equal(other *LinkedHashMultiset[T]) bool {
	return m.Len() == other.Len() && m.DistinctLen() == other.DistinctLen() && m.ContainsOccurrences(other)
}

func (m *LinkedHashMultiset[T]) Clone() *LinkedHashMultiset[T] {
	if m == nil {
		return nil
	}
	if m.counts == nil {
		return &LinkedHashMultiset[T]{}
	}
	return &LinkedHashMultiset[T]{counts: m.counts.Clone(), size: m.size}
}

// Entries yields distinct entries in first-insertion order.
func (m *LinkedHashMultiset[T]) Entries() iter.Seq[Entry[T]] {
	return func(yield func(Entry[T]) bool) {
		if m == nil || m.counts == nil {
			return
		}
		for value, count := range m.counts.All() {
			if !yield(Entry[T]{Value: value, Count: count}) {
				return
			}
		}
	}
}

func (m *LinkedHashMultiset[T]) Elements() iter.Seq[T] {
	return func(yield func(T) bool) {
		for entry := range m.Entries() {
			if !yield(entry.Value) {
				return
			}
		}
	}
}

func (m *LinkedHashMultiset[T]) All() iter.Seq[T] {
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

func (m *LinkedHashMultiset[T]) EntriesSlice() []Entry[T] {
	if m == nil {
		return nil
	}
	entries := make([]Entry[T], 0, m.DistinctLen())
	for entry := range m.Entries() {
		entries = append(entries, entry)
	}
	return entries
}

func (m *LinkedHashMultiset[T]) Slice() []T {
	if m == nil {
		return nil
	}
	values := make([]T, 0, m.Len())
	for value := range m.All() {
		values = append(values, value)
	}
	return values
}

// HighestCountFirst sorts by decreasing count and retains insertion order for ties.
func (m *LinkedHashMultiset[T]) HighestCountFirst() []Entry[T] {
	entries := m.EntriesSlice()
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Count > entries[j].Count })
	return entries
}

func (m *LinkedHashMultiset[T]) adopt(left, right *LinkedHashMultiset[T]) (func(T, T) bool, func(T) uint64) {
	if eq, hash := m.Eq(), m.Hash(); eq != nil && hash != nil {
		return eq, hash
	}
	if eq, hash := left.Eq(), left.Hash(); eq != nil && hash != nil {
		return eq, hash
	}
	return right.Eq(), right.Hash()
}

func newLinkedLike[T any](eq func(T, T) bool, hash func(T) uint64) *LinkedHashMultiset[T] {
	if eq == nil || hash == nil {
		return &LinkedHashMultiset[T]{}
	}
	return NewLinkedHashMultisetFunc(eq, hash)
}

func (m *LinkedHashMultiset[T]) Union(left, right *LinkedHashMultiset[T]) {
	if m == nil {
		return
	}
	a, b := left.EntriesSlice(), right.EntriesSlice()
	next := newLinkedLike(m.adopt(left, right))
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

func (m *LinkedHashMultiset[T]) Sum(left, right *LinkedHashMultiset[T]) {
	if m == nil {
		return
	}
	a, b := left.EntriesSlice(), right.EntriesSlice()
	next := newLinkedLike(m.adopt(left, right))
	for _, entry := range a {
		next.SetCount(entry.Value, entry.Count)
	}
	for _, entry := range b {
		next.Add(entry.Value, entry.Count)
	}
	*m = *next
}

func (m *LinkedHashMultiset[T]) Intersection(left, right *LinkedHashMultiset[T]) {
	if m == nil {
		return
	}
	a := left.EntriesSlice()
	next := newLinkedLike(m.adopt(left, right))
	for _, entry := range a {
		if count := min(entry.Count, right.Count(entry.Value)); count > 0 {
			next.SetCount(entry.Value, count)
		}
	}
	*m = *next
}

func (m *LinkedHashMultiset[T]) Minus(left, right *LinkedHashMultiset[T]) {
	if m == nil {
		return
	}
	a := left.EntriesSlice()
	next := newLinkedLike(m.adopt(left, right))
	for _, entry := range a {
		if count := entry.Count - right.Count(entry.Value); count > 0 {
			next.SetCount(entry.Value, count)
		}
	}
	*m = *next
}
