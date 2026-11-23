/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package immutable provides defensive, read-only collection snapshots. Values
// are safe for concurrent reads; referenced payloads are not deeply immutable.
package immutable

import (
	"errors"
	"hash/maphash"
	"iter"

	"github.com/pschlump/pluto/hash_grow"
)

var (
	ErrDuplicateKey   = errors.New("immutable: duplicate map key")
	ErrDuplicateValue = errors.New("immutable: duplicate bimap value")
)

func naturalFunctions[T comparable]() (func(T, T) bool, func(T) uint64) {
	seed := maphash.MakeSeed()
	return func(a, b T) bool { return a == b }, func(value T) uint64 {
		return maphash.Comparable(seed, value)
	}
}

type indexedValue[K any] struct {
	key   K
	index int
}

type hashIndex[K any] struct {
	table *hash_grow.HashTab[indexedValue[K]]
}

func newHashIndex[K any](equal func(K, K) bool, hash func(K) uint64) *hashIndex[K] {
	if equal == nil || hash == nil {
		panic("immutable: nil equality or hash function")
	}
	return &hashIndex[K]{table: hash_grow.NewHashTabFunc(
		func(a, b indexedValue[K]) bool { return equal(a.key, b.key) },
		func(value indexedValue[K]) uint64 { return hash(value.key) },
		16, 0,
	)}
}

func (index *hashIndex[K]) get(key K) (int, bool) {
	if index == nil {
		return 0, false
	}
	entry, found := index.table.Search(indexedValue[K]{key: key})
	return entry.index, found
}

func (index *hashIndex[K]) add(key K, position int) bool {
	if _, found := index.get(key); found {
		return false
	}
	index.table.Insert(indexedValue[K]{key: key, index: position})
	return true
}

func sequence[T any](values []T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}
}

func reverseSequence[T any](values []T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for index := len(values) - 1; index >= 0; index-- {
			if !yield(values[index]) {
				return
			}
		}
	}
}

func copySubslice[T any](values []T) []T {
	if len(values)*4 < cap(values) {
		return append([]T(nil), values...)
	}
	return values
}
