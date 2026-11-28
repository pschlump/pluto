/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package bimap implements a bidirectional hash map. Keys and values are unique;
// every forward association has exactly one reverse association. Inverse returns
// a coupled view sharing both indexes, not an independent copy.
package bimap

import (
	"errors"
	"fmt"
	"hash/maphash"
	"iter"

	"github.com/pschlump/pluto/hash_grow"
)

// ErrValueConflict reports that Put's value already belongs to another key.
var ErrValueConflict = errors.New("bimap: value already associated with another key")

// Association is one key/value pair.
type Association[K any, V any] struct {
	Key   K `json:"key"`
	Value V `json:"value"`
}

type direction[K any, V any] struct {
	forward   *hash_grow.HashTab[Association[K, V]]
	reverse   *hash_grow.HashTab[Association[V, K]]
	keyEq     func(K, K) bool
	keyHash   func(K) uint64
	valueEq   func(V, V) bool
	valueHash func(V) uint64
}

// HashBiMap is a bidirectional map with unique keys and values. Its zero value
// is empty and read-only; use a constructor before mutation.
type HashBiMap[K any, V any] struct {
	direction *direction[K, V]
	inverse   *HashBiMap[V, K]
}

// NewHashBiMap creates an empty bimap using == and maphash for both directions.
func NewHashBiMap[K comparable, V comparable]() *HashBiMap[K, V] {
	keyEq, keyHash := naturalFunctions[K]()
	valueEq, valueHash := naturalFunctions[V]()
	return NewHashBiMapFunc(keyEq, keyHash, valueEq, valueHash)
}

// NewHashBiMapFunc creates an empty bimap with custom equality/hash functions.
// Each equality/hash pair must agree.
func NewHashBiMapFunc[K any, V any](keyEq func(K, K) bool, keyHash func(K) uint64, valueEq func(V, V) bool, valueHash func(V) uint64) *HashBiMap[K, V] {
	if keyEq == nil || keyHash == nil || valueEq == nil || valueHash == nil {
		panic("bimap: NewHashBiMapFunc requires non-nil equality and hash functions")
	}
	forward := hash_grow.NewHashTabFunc(
		func(a, b Association[K, V]) bool { return keyEq(a.Key, b.Key) },
		func(a Association[K, V]) uint64 { return keyHash(a.Key) },
		16,
		0,
	)
	reverse := hash_grow.NewHashTabFunc(
		func(a, b Association[V, K]) bool { return valueEq(a.Key, b.Key) },
		func(a Association[V, K]) uint64 { return valueHash(a.Key) },
		16,
		0,
	)
	forwardDirection := &direction[K, V]{
		forward: forward, reverse: reverse,
		keyEq: keyEq, keyHash: keyHash, valueEq: valueEq, valueHash: valueHash,
	}
	reverseDirection := &direction[V, K]{
		forward: reverse, reverse: forward,
		keyEq: valueEq, keyHash: valueHash, valueEq: keyEq, valueHash: keyHash,
	}
	result := &HashBiMap[K, V]{direction: forwardDirection}
	inverse := &HashBiMap[V, K]{direction: reverseDirection}
	result.inverse = inverse
	inverse.inverse = result
	return result
}

func naturalFunctions[T comparable]() (func(T, T) bool, func(T) uint64) {
	seed := maphash.MakeSeed()
	return func(a, b T) bool { return a == b }, func(value T) uint64 {
		return maphash.Comparable(seed, value)
	}
}

// KeyEq and KeyHash return the forward key functions, or nil for a zero value.
func (m *HashBiMap[K, V]) KeyEq() func(K, K) bool {
	if m == nil || m.direction == nil {
		return nil
	}
	return m.direction.keyEq
}

func (m *HashBiMap[K, V]) KeyHash() func(K) uint64 {
	if m == nil || m.direction == nil {
		return nil
	}
	return m.direction.keyHash
}

// ValueEq and ValueHash return the reverse key functions, or nil for a zero value.
func (m *HashBiMap[K, V]) ValueEq() func(V, V) bool {
	if m == nil || m.direction == nil {
		return nil
	}
	return m.direction.valueEq
}

func (m *HashBiMap[K, V]) ValueHash() func(V) uint64 {
	if m == nil || m.direction == nil {
		return nil
	}
	return m.direction.valueHash
}

func (m *HashBiMap[K, V]) writeContract(caller string) {
	if m == nil || m.direction == nil {
		panic(fmt.Sprintf("bimap: %s on a nil or zero-value HashBiMap; use a constructor", caller))
	}
}

// Put inserts or updates key. It returns ErrValueConflict and makes no change
// when value belongs to a different key.
func (m *HashBiMap[K, V]) Put(key K, value V) error {
	m.writeContract("Put")
	return put(m.direction, key, value, false)
}

// ForcePut inserts or updates key, removing any conflicting old association.
func (m *HashBiMap[K, V]) ForcePut(key K, value V) {
	m.writeContract("ForcePut")
	_ = put(m.direction, key, value, true)
}

// put writes key<->value. The two tables are always mutual inverses, so the
// reverse lookup alone decides the no-op and conflict cases: reverse[value] ==
// key implies forward[key] == value. The forward write is a single Upsert that
// also returns the value it displaced, so a Put costs three probes (reverse
// lookup, forward upsert, reverse insert) plus any conflict deletes, not four.
func put[K any, V any](direction *direction[K, V], key K, value V, force bool) error {
	oldReverse, valueFound := direction.reverse.Search(Association[V, K]{Key: value})
	if valueFound && direction.keyEq(oldReverse.Value, key) {
		return nil // key already maps to value (and value back to key)
	}
	if valueFound && !force {
		return ErrValueConflict
	}
	if valueFound {
		direction.forward.Delete(Association[K, V]{Key: oldReverse.Value})
	}
	oldForward, keyFound := direction.forward.Upsert(Association[K, V]{Key: key, Value: value})
	if keyFound && !direction.valueEq(oldForward.Value, value) {
		direction.reverse.Delete(Association[V, K]{Key: oldForward.Value})
	}
	direction.reverse.Insert(Association[V, K]{Key: value, Value: key})
	return nil
}

// GetValue returns the value associated with key.
func (m *HashBiMap[K, V]) GetValue(key K) (V, bool) {
	if m == nil || m.direction == nil {
		var zero V
		return zero, false
	}
	entry, found := m.direction.forward.Search(Association[K, V]{Key: key})
	if !found {
		var zero V
		return zero, false
	}
	return entry.Value, true
}

// GetKey returns the key associated with value.
func (m *HashBiMap[K, V]) GetKey(value V) (K, bool) {
	if m == nil || m.direction == nil {
		var zero K
		return zero, false
	}
	entry, found := m.direction.reverse.Search(Association[V, K]{Key: value})
	if !found {
		var zero K
		return zero, false
	}
	return entry.Value, true
}

// DeleteKey removes key and returns its former value.
func (m *HashBiMap[K, V]) DeleteKey(key K) (V, bool) {
	if m == nil || m.direction == nil {
		var zero V
		return zero, false
	}
	entry, found := m.direction.forward.Remove(Association[K, V]{Key: key})
	if !found {
		var zero V
		return zero, false
	}
	m.direction.reverse.Delete(Association[V, K]{Key: entry.Value})
	return entry.Value, true
}

// DeleteValue removes value and returns its former key.
func (m *HashBiMap[K, V]) DeleteValue(value V) (K, bool) {
	inverse := m.Inverse()
	if inverse == nil {
		var zero K
		return zero, false
	}
	return inverse.DeleteKey(value)
}

// ContainsKey reports whether key is present.
func (m *HashBiMap[K, V]) ContainsKey(key K) bool {
	_, found := m.GetValue(key)
	return found
}

// ContainsValue reports whether value is present.
func (m *HashBiMap[K, V]) ContainsValue(value V) bool {
	_, found := m.GetKey(value)
	return found
}

// Len returns the number of associations.
func (m *HashBiMap[K, V]) Len() int {
	if m == nil || m.direction == nil {
		return 0
	}
	return m.direction.forward.Len()
}

// IsEmpty reports whether there are no associations.
func (m *HashBiMap[K, V]) IsEmpty() bool { return m.Len() == 0 }

// Clear removes all associations while retaining equality/hash functions.
func (m *HashBiMap[K, V]) Clear() {
	if m == nil || m.direction == nil {
		return
	}
	m.direction.forward.Truncate()
	m.direction.reverse.Truncate()
}

// Inverse returns the coupled reverse view. Inverse().Inverse() returns the
// original pointer. A nil or zero-value bimap returns nil.
func (m *HashBiMap[K, V]) Inverse() *HashBiMap[V, K] {
	if m == nil || m.direction == nil {
		return nil
	}
	return m.inverse
}

// Entries iterates over forward associations in unspecified hash order.
func (m *HashBiMap[K, V]) Entries() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if m == nil || m.direction == nil {
			return
		}
		for entry := range m.direction.forward.Values() {
			if !yield(entry.Key, entry.Value) {
				return
			}
		}
	}
}

// Keys iterates over keys in unspecified order.
func (m *HashBiMap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for key := range m.Entries() {
			if !yield(key) {
				return
			}
		}
	}
}

// Values iterates over values in the same order as Entries.
func (m *HashBiMap[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, value := range m.Entries() {
			if !yield(value) {
				return
			}
		}
	}
}

// EntriesSlice returns a fresh association snapshot.
func (m *HashBiMap[K, V]) EntriesSlice() []Association[K, V] {
	if m == nil {
		return nil
	}
	entries := make([]Association[K, V], 0, m.Len())
	for key, value := range m.Entries() {
		entries = append(entries, Association[K, V]{Key: key, Value: value})
	}
	return entries
}

// Clone returns an independent bimap with the same functions and associations.
func (m *HashBiMap[K, V]) Clone() *HashBiMap[K, V] {
	if m == nil {
		return nil
	}
	if m.direction == nil {
		return &HashBiMap[K, V]{}
	}
	clone := NewHashBiMapFunc(m.direction.keyEq, m.direction.keyHash, m.direction.valueEq, m.direction.valueHash)
	for key, value := range m.Entries() {
		clone.ForcePut(key, value)
	}
	return clone
}

// Equal reports bidirectional content equality. Both maps must use consistent
// equality and hash functions.
func (m *HashBiMap[K, V]) Equal(other *HashBiMap[K, V]) bool {
	if m.Len() != other.Len() {
		return false
	}
	if m == nil || m.direction == nil {
		return true
	}
	for key, value := range m.Entries() {
		otherValue, found := other.GetValue(key)
		if !found || !m.direction.valueEq(value, otherValue) {
			return false
		}
	}
	return true
}
