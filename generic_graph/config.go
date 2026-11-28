/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package generic_graph provides directed and undirected graphs over arbitrary
// node values, valued edges, unique-edge networks, lazy traversal, snapshots,
// graph transformations, and adapters to Pluto's integer-vertex algorithms.
package generic_graph

import (
	"errors"
	"hash/maphash"
	"iter"
	"maps"

	"github.com/pschlump/pluto/hash_grow"
)

var (
	ErrUnknownNode           = errors.New("generic_graph: unknown node")
	ErrDuplicateNode         = errors.New("generic_graph: duplicate node")
	ErrDuplicateEdge         = errors.New("generic_graph: duplicate edge")
	ErrUnknownEdge           = errors.New("generic_graph: unknown edge")
	ErrSelfLoop              = errors.New("generic_graph: self-loop is disallowed")
	ErrParallelEdge          = errors.New("generic_graph: parallel edge is disallowed")
	ErrInconsistentEndpoints = errors.New("generic_graph: inconsistent endpoints")
)

// IterationOrder controls node, adjacency, and edge iteration.
type IterationOrder uint8

const (
	Unordered IterationOrder = iota
	InsertionOrder
	SortedOrder
)

// Config controls Graph and ValueGraph construction. Func construction requires
// Equal and Hash; SortedOrder additionally requires Less.
type Config[N any] struct {
	Directed       bool
	AllowSelfLoops bool
	ExpectedNodes  int
	Order          IterationOrder
	Equal          func(N, N) bool
	Hash           func(N) uint64
	Less           func(N, N) bool

	// native is set only by the natural (comparable) constructors: it builds a
	// node index on Go's built-in map, whose == semantics are exactly the
	// natural Equal/Hash pair. Func configurations leave it nil and use the
	// Equal/Hash-driven hash_grow index.
	native func(expected int) nativeIndex[N]
}

// NetworkConfig additionally controls unique edge identity and parallel edges.
type NetworkConfig[N any, E any] struct {
	Config[N]
	AllowParallelEdges bool
	ExpectedEdges      int
	EqualEdge          func(E, E) bool
	HashEdge           func(E) uint64
	EdgeLess           func(E, E) bool

	nativeEdge func(expected int) nativeIndex[E] // see Config.native
}

// EndpointPair identifies an edge. Source/Target retain insertion orientation
// even for an undirected graph.
type EndpointPair[N any] struct {
	Source N
	Target N
}

type indexedValue[T any] struct {
	value  T
	id     int
	active bool
}

// indexedSet assigns each distinct value a stable ID (never reused) and finds
// it again through either a native Go map (natural configurations) or a
// hash_grow table driven by the configured Equal/Hash functions.
type indexedSet[T any] struct {
	table   *hash_grow.HashTab[indexedValue[T]] // nil when native is used
	native  nativeIndex[T]
	entries []indexedValue[T]
	active  int
}

// nativeIndex is a value-to-ID index over a map[T]int captured by closures,
// letting the type-parameter-free indexedSet[T any] use a comparable map
// without boxing values in an interface.
type nativeIndex[T any] struct {
	get   func(T) (int, bool)
	put   func(T, int)
	del   func(T)
	clone func() nativeIndex[T]
}

func newNativeIndex[T comparable](expected int) nativeIndex[T] {
	return wrapNativeIndex(make(map[T]int, expected))
}

func wrapNativeIndex[T comparable](index map[T]int) nativeIndex[T] {
	return nativeIndex[T]{
		get: func(value T) (int, bool) {
			id, found := index[value]
			return id, found
		},
		put:   func(value T, id int) { index[value] = id },
		del:   func(value T) { delete(index, value) },
		clone: func() nativeIndex[T] { return wrapNativeIndex(maps.Clone(index)) },
	}
}

func newIndexedSet[T any](equal func(T, T) bool, hash func(T) uint64, expected int, native func(int) nativeIndex[T]) *indexedSet[T] {
	if equal == nil || hash == nil || expected < 0 {
		panic("generic_graph: invalid equality/hash functions or negative expected capacity")
	}
	if native != nil {
		return &indexedSet[T]{native: native(expected), entries: make([]indexedValue[T], 0, expected)}
	}
	return &indexedSet[T]{table: hash_grow.NewHashTabFunc(
		func(a, b indexedValue[T]) bool { return equal(a.value, b.value) },
		func(value indexedValue[T]) uint64 { return hash(value.value) },
		max(16, expected), 0,
	), entries: make([]indexedValue[T], 0, expected)}
}

func (s *indexedSet[T]) lookup(value T) (int, bool) {
	if s == nil {
		return 0, false
	}
	if s.table == nil {
		return s.native.get(value)
	}
	entry, found := s.table.Search(indexedValue[T]{value: value})
	return entry.id, found
}

func (s *indexedSet[T]) add(value T) (int, bool) {
	if _, found := s.lookup(value); found {
		return 0, false
	}
	id := len(s.entries)
	entry := indexedValue[T]{value: value, id: id, active: true}
	s.entries = append(s.entries, entry)
	if s.table == nil {
		s.native.put(value, id)
	} else {
		s.table.Insert(entry)
	}
	s.active++
	return id, true
}

func (s *indexedSet[T]) remove(value T) bool {
	id, found := s.lookup(value)
	if !found {
		return false
	}
	if s.table == nil {
		s.native.del(value)
	} else {
		s.table.Delete(indexedValue[T]{value: value})
	}
	s.entries[id].value = *new(T)
	s.entries[id].active = false
	s.active--
	return true
}

func (s *indexedSet[T]) hasID(id int) bool {
	return s != nil && id >= 0 && id < len(s.entries) && s.entries[id].active
}

func (s *indexedSet[T]) copy() *indexedSet[T] {
	if s == nil {
		return nil
	}
	result := &indexedSet[T]{entries: append([]indexedValue[T](nil), s.entries...), active: s.active}
	if s.table == nil {
		result.native = s.native.clone()
	} else {
		result.table = s.table.Clone()
	}
	return result
}

func naturalConfig[N comparable](directed, loops bool) Config[N] {
	seed := maphash.MakeSeed()
	return Config[N]{Directed: directed, AllowSelfLoops: loops, Order: InsertionOrder,
		Equal:  func(a, b N) bool { return a == b },
		Hash:   func(value N) uint64 { return maphash.Comparable(seed, value) },
		native: newNativeIndex[N]}
}

func naturalNetworkConfig[N comparable, E comparable](directed, loops, parallel bool) NetworkConfig[N, E] {
	seed := maphash.MakeSeed()
	return NetworkConfig[N, E]{Config: naturalConfig[N](directed, loops), AllowParallelEdges: parallel,
		EqualEdge:  func(a, b E) bool { return a == b },
		HashEdge:   func(value E) uint64 { return maphash.Comparable(seed, value) },
		nativeEdge: newNativeIndex[E]}
}

func validateConfig[N any](config Config[N]) {
	if config.Order > SortedOrder || config.ExpectedNodes < 0 {
		panic("generic_graph: invalid iteration order or expected capacity")
	}
	if config.Equal == nil || config.Hash == nil || (config.Order == SortedOrder && config.Less == nil) {
		panic("generic_graph: Func configuration requires Equal/Hash and SortedOrder requires Less")
	}
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
