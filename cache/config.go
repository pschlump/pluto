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
	"sync/atomic"
	"time"
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

// Entry states, published atomically for lock-free readers.
const (
	entryLoading uint32 = iota // in-flight load placeholder: invisible to reads
	entryLive                  // linked and readable
	entryDead                  // removed or replaced: readers must retry under the lock
)

// cacheEntry is one stored value. Linked entries sit on their shard's
// access-order list (prev/next: LRU order, and expire-after-access order) and,
// when expire-after-write is configured, on the write-order list
// (wprev/wnext). Unlinked entries are in-flight load placeholders.
//
// Lock-free readers may touch only key, hash, state, value, writeTime and
// accessTime. key and hash never change. value, weight and writeTime are
// written before state becomes entryLive and never change afterwards (refresh
// publishes a new entry instead), so the atomic state load orders them.
// accessTime is atomic. Everything else is guarded by the shard lock.
// Timestamps are nanoseconds since the cache's epoch.
type cacheEntry[K any, V any] struct {
	key          K
	value        V
	hash         uint64
	weight       int64
	writeTime    int64
	accessTime   atomic.Int64
	state        atomic.Uint32
	prev, next   *cacheEntry[K, V]
	wprev, wnext *cacheEntry[K, V]
	linked       bool
	call         *loadCall[V]
}

// cacheShard is one independently locked segment: its own table, LRU and
// write-order lists, read buffer, size/weight budget, and statistics. Reads
// go through the lock-free table and record accesses in reads; everything
// else holds lock.
type cacheShard[K any, V any] struct {
	lock          sync.Mutex
	c             *Cache[K, V]
	table         atomic.Pointer[readTable[K, V]]
	used, live    int               // occupied (live+tombstone) and live table slots
	head, tail    *cacheEntry[K, V] // access order; head is least recent
	whead, wtail  *cacheEntry[K, V] // write order; used only with expire-after-write
	length        int
	totalWeight   int64
	maximumSize   int
	maximumWeight int64
	stats         Stats
	hits          atomic.Uint64 // lock-free hits when reads are unordered
	_             [56]byte
	reads         readBuffer[K, V]
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

// WithConcurrencyLevel sets the target number of independently locked shards
// (default 16). The shard count is the smallest power of two at least level,
// reduced for small bounded caches so every shard keeps a useful budget (the
// Guava rule: doubling stops once shards*20 exceeds the bound). Size and weight
// limits, LRU order, and expiration cleanup are per shard. Level 1 gives a
// single exact global LRU.
func WithConcurrencyLevel[K any, V any](level int) Option[K, V] {
	return func(c *Cache[K, V]) {
		if level < 1 {
			panic("cache: concurrency level must be at least 1")
		}
		c.concurrency = level
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
	shards           []*cacheShard[K, V]
	shift            uint // 64 - log2(len(shards)); 64 for a single shard
	equal            func(K, K) bool
	hash             func(K) uint64
	weigher          func(K, V) int64
	concurrency      int
	maximumSize      int
	maximumWeight    int64
	expireWrite      time.Duration
	expireAccess     time.Duration
	refreshWrite     time.Duration
	expiring         bool // expireWrite > 0 || expireAccess > 0
	timed            bool // expiring || refreshWrite > 0: entries need timestamps
	ordered          bool // reads must update LRU order: size/weight bound or expire-after-access
	clock            func() time.Time
	epoch            time.Time // timestamps are int64 nanoseconds since epoch
	loader           Loader[K, V]
	bulkLoader       BulkLoader[K, V]
	executor         func(func())
	listener         func(Notification[K, V])
	listenerExecutor func(func())
}

// defaultConcurrency is the default target shard count.
const defaultConcurrency = 16

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
		concurrency: defaultConcurrency,
		weigher:     func(K, V) int64 { return 1 }, clock: time.Now,
		executor: func(operation func()) { go operation() },
	}
	for _, option := range options {
		if option != nil {
			option(result)
		}
	}
	result.expiring = result.expireWrite > 0 || result.expireAccess > 0
	result.timed = result.expiring || result.refreshWrite > 0
	result.ordered = result.maximumSize >= 0 || result.maximumWeight >= 0 || result.expireAccess > 0
	result.epoch = result.clock()
	result.buildShards()
	return result
}

// buildShards sizes and allocates the shards once options are applied.
func (c *Cache[K, V]) buildShards() {
	bound := int64(-1)
	if c.maximumSize >= 0 {
		bound = int64(c.maximumSize)
	}
	if c.maximumWeight >= 0 && (bound < 0 || c.maximumWeight < bound) {
		bound = c.maximumWeight
	}
	count, bits := 1, uint(0)
	for count < c.concurrency && count < 1<<16 && (bound < 0 || int64(count)*20 <= bound) {
		count <<= 1
		bits++
	}
	c.shift = 64 - bits
	c.shards = make([]*cacheShard[K, V], count)
	for index := range c.shards {
		shard := &cacheShard[K, V]{c: c, maximumSize: -1, maximumWeight: -1}
		if c.maximumSize >= 0 {
			shard.maximumSize = c.maximumSize / count
			if index < c.maximumSize%count {
				shard.maximumSize++
			}
		}
		if c.maximumWeight >= 0 {
			shard.maximumWeight = c.maximumWeight / int64(count)
			if int64(index) < c.maximumWeight%int64(count) {
				shard.maximumWeight++
			}
		}
		shard.table.Store(newReadTable[K, V](minTableSize))
		c.shards[index] = shard
	}
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
