/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import "iter"

// Reader is implemented by every mutable multimap and its thread-safe twin.
type Reader[K any, V any] interface {
	Entries() iter.Seq2[K, V]
}

// Index groups values by keyFunc, preserving input order within each key.
func Index[K comparable, V comparable](values []V, keyFunc func(V) K) *ListMultimap[K, V] {
	if keyFunc == nil {
		panic("multimap: Index called with a nil key function")
	}
	result := NewListMultimap[K, V]()
	for _, value := range values {
		result.Put(keyFunc(value), value)
	}
	return result
}

// InvertFrom adds the reverse associations of source to destination and returns
// destination. A nil destination creates a new list multimap. Source entries are
// snapshotted first, so source and destination may alias when K and V match.
func InvertFrom[K comparable, V comparable](source Reader[K, V], destination *ListMultimap[V, K]) *ListMultimap[V, K] {
	if source == nil {
		return destination
	}
	if destination == nil {
		destination = NewListMultimap[V, K]()
	}
	var associations []Association[K, V]
	for key, value := range source.Entries() {
		associations = append(associations, Association[K, V]{Key: key, Value: value})
	}
	for _, association := range associations {
		destination.Put(association.Value, association.Key)
	}
	return destination
}

// ForMap copies each map entry into a list multimap. Map iteration order is
// unspecified, so associations for different keys have unspecified order.
func ForMap[K comparable, V comparable](values map[K]V) *ListMultimap[K, V] {
	result := NewListMultimap[K, V]()
	for key, value := range values {
		result.Put(key, value)
	}
	return result
}
