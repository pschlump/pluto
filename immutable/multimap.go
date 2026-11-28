/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import (
	"iter"

	"github.com/pschlump/pluto/multimap"
)

// Multimap is an immutable list multimap. Keys retain first occurrence order and
// each key's values retain input order, including duplicates.
type Multimap[K any, V any] struct {
	keys       []K
	values     [][]V
	total      int
	index      *hashIndex[K]
	valueEqual func(V, V) bool
	keyEqual   func(K, K) bool
	keyHash    func(K) uint64
}

func NewMultimap[K comparable, V comparable](entries []multimap.Association[K, V]) *Multimap[K, V] {
	keyEqual, keyHash := naturalFunctions[K]()
	return newMultimap(keyEqual, keyHash, func(a, b V) bool { return a == b }, newNaturalIndex[K](0), entries)
}

func NewMultimapFunc[K any, V any](keyEqual func(K, K) bool, keyHash func(K) uint64, valueEqual func(V, V) bool, entries []multimap.Association[K, V]) *Multimap[K, V] {
	if valueEqual == nil {
		panic("immutable: NewMultimapFunc requires non-nil value equality")
	}
	return newMultimap(keyEqual, keyHash, valueEqual, newHashIndex(keyEqual, keyHash), entries)
}

// newMultimap groups entries in two passes: the first assigns each entry its
// key position and counts values per key, the second copies every value into
// one shared backing array carved into per-key, full-capacity groups. Groups
// are never exposed (Get and AsMap copy), so sharing one array is safe.
func newMultimap[K any, V any](keyEqual func(K, K) bool, keyHash func(K) uint64, valueEqual func(V, V) bool, index *hashIndex[K], entries []multimap.Association[K, V]) *Multimap[K, V] {
	result := &Multimap[K, V]{index: index, valueEqual: valueEqual, keyEqual: keyEqual, keyHash: keyHash}
	if len(entries) == 0 {
		return result
	}
	positions := make([]int, len(entries))
	var counts []int
	for entryIndex, entry := range entries {
		if result.total == maxInt {
			panic("immutable: multimap association count overflow")
		}
		position, found := index.getOrAdd(entry.Key, len(result.keys))
		if !found {
			result.keys = append(result.keys, entry.Key)
			counts = append(counts, 0)
		}
		positions[entryIndex] = position
		counts[position]++
		result.total++
	}
	flat := make([]V, len(entries))
	result.values = make([][]V, len(result.keys))
	start := 0
	for position, count := range counts {
		result.values[position] = flat[start : start : start+count]
		start += count
	}
	for entryIndex, entry := range entries {
		position := positions[entryIndex]
		result.values[position] = append(result.values[position], entry.Value)
	}
	return result
}

// group returns key's internal value group without copying. Internal only:
// the group must never escape to a caller. Expected O(1).
func (m *Multimap[K, V]) group(key K) ([]V, bool) {
	if m == nil {
		return nil, false
	}
	position, found := m.index.get(key)
	if !found {
		return nil, false
	}
	return m.values[position], true
}

func CopyOfMultimap[K any, V any](value *Multimap[K, V]) *Multimap[K, V] { return value }

func FreezeListMultimap[K any, V any](source *multimap.ListMultimap[K, V]) *Multimap[K, V] {
	if source == nil {
		return nil
	}
	if source.KeyEq() == nil {
		return &Multimap[K, V]{valueEqual: source.ValueEq()}
	}
	return NewMultimapFunc(source.KeyEq(), source.KeyHash(), source.ValueEq(), source.EntriesSlice())
}

func FreezeSetMultimap[K any, V any](source *multimap.SetMultimap[K, V]) *Multimap[K, V] {
	if source == nil {
		return nil
	}
	if source.KeyEq() == nil {
		return &Multimap[K, V]{valueEqual: source.ValueEq()}
	}
	return NewMultimapFunc(source.KeyEq(), source.KeyHash(), source.ValueEq(), source.EntriesSlice())
}

func FreezeSortedSetMultimap[K any, V any](source *multimap.SortedSetMultimap[K, V]) *Multimap[K, V] {
	if source == nil {
		return nil
	}
	if source.KeyEq() == nil {
		return &Multimap[K, V]{valueEqual: source.ValueEq()}
	}
	return NewMultimapFunc(source.KeyEq(), source.KeyHash(), source.ValueEq(), source.EntriesSlice())
}

func (m *Multimap[K, V]) Len() int {
	if m == nil {
		return 0
	}
	return m.total
}

func (m *Multimap[K, V]) KeyLen() int {
	if m == nil {
		return 0
	}
	return len(m.keys)
}

func (m *Multimap[K, V]) IsEmpty() bool { return m.Len() == 0 }

// Get returns a fresh copy of key's values in input order; the caller owns it.
// Complexity is expected O(1) lookup plus O(values for key) for the copy.
func (m *Multimap[K, V]) Get(key K) ([]V, bool) {
	values, found := m.group(key)
	if !found {
		return nil, false
	}
	return append([]V{}, values...), true
}

// ValuesOf iterates key's values in input order, including duplicates, without
// copying them. It is the allocation-free alternative to Get for callers that
// only read the values: Get must copy because it hands the caller a slice it
// owns, while ValuesOf yields values one at a time from the shared group, so
// the multimap stays immutable. An absent key, a nil multimap, and the zero
// value yield nothing.
//
// The lookup runs inside the returned function, each time iteration starts.
// That keeps ValuesOf a one-line wrapper the compiler can inline into a range
// loop, which is what makes it allocation-free. With the lookup outside the
// closure, ValuesOf is too large to inline and costs 3 allocations per call.
// The multimap is immutable, so when the lookup happens is not observable.
// Complexity is expected O(1) lookup plus O(values for key) to iterate.
func (m *Multimap[K, V]) ValuesOf(key K) iter.Seq[V] {
	return func(yield func(V) bool) {
		values, _ := m.group(key)
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}
}

// ValueCount returns how many values key has, counting duplicates, without
// copying them; an absent key, a nil multimap, and the zero value report 0.
// It is the allocation-free replacement for len(Get(key)), and the same work
// as Guava's get(key).size().
// Complexity is expected O(1).
func (m *Multimap[K, V]) ValueCount(key K) int {
	values, _ := m.group(key)
	return len(values)
}

func (m *Multimap[K, V]) ContainsKey(key K) bool {
	_, found := m.group(key)
	return found
}

func (m *Multimap[K, V]) ContainsEntry(key K, value V) bool {
	values, found := m.group(key)
	if !found {
		return false
	}
	for _, candidate := range values {
		if m.valueEqual(candidate, value) {
			return true
		}
	}
	return false
}

func (m *Multimap[K, V]) ContainsValue(value V) bool {
	for candidate := range m.Values() {
		if m.valueEqual(candidate, value) {
			return true
		}
	}
	return false
}

func (m *Multimap[K, V]) Entries() iter.Seq[multimap.Association[K, V]] {
	return func(yield func(multimap.Association[K, V]) bool) {
		if m == nil {
			return
		}
		for position, key := range m.keys {
			for _, value := range m.values[position] {
				if !yield(multimap.Association[K, V]{Key: key, Value: value}) {
					return
				}
			}
		}
	}
}

func (m *Multimap[K, V]) EntriesSlice() []multimap.Association[K, V] {
	entries := []multimap.Association[K, V]{}
	for entry := range m.Entries() {
		entries = append(entries, entry)
	}
	return entries
}

func (m *Multimap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		if m == nil {
			return
		}
		for position, key := range m.keys {
			for range m.values[position] {
				if !yield(key) {
					return
				}
			}
		}
	}
}

func (m *Multimap[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		if m == nil {
			return
		}
		for _, values := range m.values {
			for _, value := range values {
				if !yield(value) {
					return
				}
			}
		}
	}
}

func (m *Multimap[K, V]) KeySet() iter.Seq[K] {
	var values []K
	if m != nil {
		values = m.keys
	}
	return sequence(values)
}

// AsMap yields each key and a fresh value slice snapshot.
func (m *Multimap[K, V]) AsMap() iter.Seq2[K, []V] {
	return func(yield func(K, []V) bool) {
		if m == nil {
			return
		}
		for position, key := range m.keys {
			if !yield(key, append([]V{}, m.values[position]...)) {
				return
			}
		}
	}
}

func (m *Multimap[K, V]) Equal(other *Multimap[K, V]) bool {
	if m.Len() != other.Len() || m.KeyLen() != other.KeyLen() {
		return false
	}
	for position, key := range m.keys {
		values, found := other.group(key)
		if !found || len(values) != len(m.values[position]) {
			return false
		}
		for index, value := range m.values[position] {
			if !m.valueEqual(value, values[index]) {
				return false
			}
		}
	}
	return true
}

// MultimapBuilder incrementally constructs independent Multimap snapshots.
type MultimapBuilder[K any, V any] struct {
	entries    []multimap.Association[K, V]
	keyEqual   func(K, K) bool
	keyHash    func(K) uint64
	valueEqual func(V, V) bool
	natural    func(hint int) *hashIndex[K] // set by NewMultimapBuilder only
}

func NewMultimapBuilder[K comparable, V comparable]() *MultimapBuilder[K, V] {
	keyEqual, keyHash := naturalFunctions[K]()
	builder := NewMultimapBuilderFunc(keyEqual, keyHash, func(a, b V) bool { return a == b })
	builder.natural = newNaturalIndex[K]
	return builder
}

func NewMultimapBuilderFunc[K any, V any](keyEqual func(K, K) bool, keyHash func(K) uint64, valueEqual func(V, V) bool) *MultimapBuilder[K, V] {
	if keyEqual == nil || keyHash == nil || valueEqual == nil {
		panic("immutable: NewMultimapBuilderFunc requires non-nil functions")
	}
	return &MultimapBuilder[K, V]{keyEqual: keyEqual, keyHash: keyHash, valueEqual: valueEqual}
}

func (b *MultimapBuilder[K, V]) Put(key K, value V) {
	b.entries = append(b.entries, multimap.Association[K, V]{Key: key, Value: value})
}

func (b *MultimapBuilder[K, V]) PutAll(entries []multimap.Association[K, V]) {
	b.entries = append(b.entries, entries...)
}

func (b *MultimapBuilder[K, V]) Build() *Multimap[K, V] {
	if b == nil {
		return nil
	}
	if b.natural != nil {
		return newMultimap(b.keyEqual, b.keyHash, b.valueEqual, b.natural(0), b.entries)
	}
	return NewMultimapFunc(b.keyEqual, b.keyHash, b.valueEqual, b.entries)
}
