/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap_ts

import (
	"github.com/pschlump/pluto/multimap"
)

// Index groups values by keyFunc, preserving input order within each key.
func Index[K comparable, V comparable](values []V, keyFunc func(V) K) *ListMultimap[K, V] {
	if keyFunc == nil {
		panic("multimap_ts: Index called with a nil key function")
	}
	result := NewListMultimap[K, V]()
	for _, value := range values {
		result.Put(keyFunc(value), value)
	}
	return result
}

// InvertFrom snapshots source and adds its reverse associations to destination.
func InvertFrom[K comparable, V comparable](source multimap.Reader[K, V], destination *ListMultimap[V, K]) *ListMultimap[V, K] {
	if source == nil {
		return destination
	}
	if destination == nil {
		destination = NewListMultimap[V, K]()
	}
	var associations []multimap.Association[K, V]
	for key, value := range source.Entries() {
		associations = append(associations, multimap.Association[K, V]{Key: key, Value: value})
	}
	for _, association := range associations {
		destination.Put(association.Value, association.Key)
	}
	return destination
}

// ForMap copies map associations into a list multimap.
func ForMap[K comparable, V comparable](values map[K]V) *ListMultimap[K, V] {
	result := NewListMultimap[K, V]()
	for key, value := range values {
		result.Put(key, value)
	}
	return result
}
