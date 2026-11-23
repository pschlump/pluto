/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package map_util provides immutable indexing and map-difference utilities.
package map_util

import "github.com/pschlump/pluto/immutable"

// UniqueIndex indexes values by unique keys and returns an immutable map.
// Duplicate keys fail without publishing a partial result.
func UniqueIndex[K comparable, V any](values []V, key func(V) K) (*immutable.Map[K, V], error) {
	equal, hash := naturalFunctions[K]()
	return UniqueIndexFunc(equal, hash, values, key)
}

// UniqueIndexFunc is UniqueIndex with custom key equality and hashing.
func UniqueIndexFunc[K any, V any](equal func(K, K) bool, hash func(K) uint64, values []V, key func(V) K) (*immutable.Map[K, V], error) {
	if key == nil {
		panic("map_util: nil key function")
	}
	entries := make([]immutable.Entry[K, V], len(values))
	for index, value := range values {
		entries[index] = immutable.Entry[K, V]{Key: key(value), Value: value}
	}
	return immutable.NewMapFunc(equal, hash, entries)
}

// ValueDifference records the left and right values for a shared unequal key.
type ValueDifference[V any] struct {
	Left  V
	Right V
}

// MapDifference contains four independent maps partitioning both inputs.
type MapDifference[K comparable, V any] struct {
	Common    map[K]V
	Differing map[K]ValueDifference[V]
	LeftOnly  map[K]V
	RightOnly map[K]V
}

func Difference[K comparable, V comparable](left, right map[K]V) MapDifference[K, V] {
	return DifferenceFunc(left, right, func(a, b V) bool { return a == b })
}

// DifferenceFunc compares values with equal and supports non-comparable V.
func DifferenceFunc[K comparable, V any](left, right map[K]V, equal func(V, V) bool) MapDifference[K, V] {
	if equal == nil {
		panic("map_util: nil value equality")
	}
	result := MapDifference[K, V]{
		Common: map[K]V{}, Differing: map[K]ValueDifference[V]{}, LeftOnly: map[K]V{}, RightOnly: map[K]V{},
	}
	for key, leftValue := range left {
		rightValue, found := right[key]
		if !found {
			result.LeftOnly[key] = leftValue
			continue
		}
		if equal(leftValue, rightValue) {
			result.Common[key] = leftValue
		} else {
			result.Differing[key] = ValueDifference[V]{Left: leftValue, Right: rightValue}
		}
	}
	for key, value := range right {
		if _, found := left[key]; !found {
			result.RightOnly[key] = value
		}
	}
	return result
}
