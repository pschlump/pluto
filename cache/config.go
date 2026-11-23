/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package cache implements concurrent loading, expiring, refreshing caches with
// LRU size/weight eviction. No background goroutine is created per cache.
package cache

import (
	"context"
	"hash/maphash"
	"sync"
	"time"

	"github.com/pschlump/pluto/hash_grow"
)

// Loader computes a value for a missing key.
type Loader[K any, V any] func(context.Context, K) (V, error)

// BulkLoader computes entries for missing keys.
type BulkLoader[K any, V any] func(context.Context, []K) ([]Entry[K, V], error)

// Entry is one key/value snapshot.
type Entry[K any, V any] struct {
	Key   K `json:"key"`
	Value V `json:"value"`
}

// RemovalCause identifies why a value left the cache.
type RemovalCause string

const (
	Explicit    RemovalCause = "explicit"
	Replaced    RemovalCause = "replaced"
	Expired     RemovalCause = "expired"
	Size        RemovalCause = "size"
	LoaderError RemovalCause = "loader-error"
)

// Notification is delivered after the internal lock has been released.
type Notification[K any, V any] struct {
	Key   K
	Value V
	Cause RemovalCause
}

// Stats is an immutable statistics snapshot.
type Stats struct {
	Hits          uint64
	Misses        uint64
	Loads         uint64
	LoadSuccesses uint64
	LoadFailures  uint64
	TotalLoadTime time.Duration
	Evictions     uint64
}

type cacheEntry[K any, V any] struct {
	key        K
	value      V
	weight     int64
	writeTime  time.Time
	accessTime time.Time
	prev, next *cacheEntry[K, V]
	linked     bool
	call       *loadCall[V]
}

// Option configures a cache before publication.
type Option[K any, V any] func(*Cache[K, V])

func WithMaximumSize[K any, V any](size int) Option[K, V] {
	return func(c *Cache[K, V]) {
		if size < 0 {
			panic("cache: negative maximum size")
		}
		c.maximumSize = size
	}
}
func WithMaximumWeight[K any, V any](weight int64) Option[K, V] {
	return func(c *Cache[K, V]) {
		if weight < 0 {
			panic("cache: negative maximum weight")
		}
		c.maximumWeight = weight
	}
}
func WithWeigher[K any, V any](weigher func(K, V) int64) Option[K, V] {
	return func(c *Cache[K, V]) {
		if weigher == nil {
			panic("cache: nil weigher")
		}
		c.weigher = weigher
	}
}
func WithExpireAfterWrite[K any, V any](duration time.Duration) Option[K, V] {
	return func(c *Cache[K, V]) {
		if duration < 0 {
			panic("cache: negative expiration")
		}
		c.expireWrite = duration
	}
}
func WithExpireAfterAccess[K any, V any](duration time.Duration) Option[K, V] {
	return func(c *Cache[K, V]) {
		if duration < 0 {
			panic("cache: negative expiration")
		}
		c.expireAccess = duration
	}
}
func WithRefreshAfterWrite[K any, V any](duration time.Duration) Option[K, V] {
	return func(c *Cache[K, V]) {
		if duration < 0 {
			panic("cache: negative refresh")
		}
		c.refreshWrite = duration
	}
}
func WithClock[K any, V any](clock func() time.Time) Option[K, V] {
	return func(c *Cache[K, V]) {
		if clock == nil {
			panic("cache: nil clock")
		}
		c.clock = clock
	}
}
func WithRemovalListener[K any, V any](listener func(Notification[K, V])) Option[K, V] {
	return func(c *Cache[K, V]) { c.listener = listener }
}
func WithExecutor[K any, V any](executor func(func())) Option[K, V] {
	return func(c *Cache[K, V]) {
		if executor == nil {
			panic("cache: nil executor")
		}
		c.executor = executor
	}
}
func WithListenerExecutor[K any, V any](executor func(func())) Option[K, V] {
	return func(c *Cache[K, V]) { c.listenerExecutor = executor }
}
func WithBulkLoader[K any, V any](loader BulkLoader[K, V]) Option[K, V] {
	return func(c *Cache[K, V]) { c.bulkLoader = loader }
}

// Cache is safe for concurrent use. Its zero value is not usable; use New or
// NewFunc. Callback loaders, listeners, weighers, and hash/equality functions
// must not mutate configuration after construction.
type Cache[K any, V any] struct {
	lock             sync.Mutex
	tab              *hash_grow.HashTab[*cacheEntry[K, V]]
	equal            func(K, K) bool
	hash             func(K) uint64
	weigher          func(K, V) int64
	maximumSize      int
	maximumWeight    int64
	expireWrite      time.Duration
	expireAccess     time.Duration
	refreshWrite     time.Duration
	clock            func() time.Time
	loader           Loader[K, V]
	bulkLoader       BulkLoader[K, V]
	executor         func(func())
	listener         func(Notification[K, V])
	listenerExecutor func(func())
	head, tail       *cacheEntry[K, V]
	length           int
	totalWeight      int64
	stats            Stats
}

func New[K comparable, V any](options ...Option[K, V]) *Cache[K, V] {
	seed := maphash.MakeSeed()
	return NewFunc(func(a, b K) bool { return a == b }, func(key K) uint64 { return maphash.Comparable(seed, key) }, options...)
}

func NewFunc[K any, V any](equal func(K, K) bool, hash func(K) uint64, options ...Option[K, V]) *Cache[K, V] {
	if equal == nil || hash == nil {
		panic("cache: NewFunc requires non-nil key functions")
	}
	result := &Cache[K, V]{
		equal: equal, hash: hash, maximumSize: -1, maximumWeight: -1,
		weigher: func(K, V) int64 { return 1 }, clock: time.Now,
		executor: func(operation func()) { go operation() },
	}
	result.tab = hash_grow.NewHashTabFunc(
		func(a, b *cacheEntry[K, V]) bool { return equal(a.key, b.key) },
		func(entry *cacheEntry[K, V]) uint64 { return hash(entry.key) }, 16, 0,
	)
	for _, option := range options {
		if option != nil {
			option(result)
		}
	}
	return result
}

// NewLoading creates a cache with a default loader.
func NewLoading[K comparable, V any](loader Loader[K, V], options ...Option[K, V]) *LoadingCache[K, V] {
	value := New(options...)
	value.loader = loader
	if loader == nil {
		panic("cache: nil loader")
	}
	return &LoadingCache[K, V]{cache: value}
}

func NewLoadingFunc[K any, V any](equal func(K, K) bool, hash func(K) uint64, loader Loader[K, V], options ...Option[K, V]) *LoadingCache[K, V] {
	value := NewFunc(equal, hash, options...)
	value.loader = loader
	if loader == nil {
		panic("cache: nil loader")
	}
	return &LoadingCache[K, V]{cache: value}
}

// LoadingCache exposes Cache operations and loader-based Get/GetAll.
type LoadingCache[K any, V any] struct{ cache *Cache[K, V] }

// Cache returns the underlying cache for direct operations.
func (c *LoadingCache[K, V]) Cache() *Cache[K, V] {
	if c == nil {
		return nil
	}
	return c.cache
}
