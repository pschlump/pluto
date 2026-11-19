/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package tiny_lfu_ts is the thread-safe twin of
// github.com/pschlump/pluto/tiny_lfu. A sharded concurrent hash table provides
// immediate value visibility, while bounded access buffers and batched policy
// maintenance implement Caffeine-style asynchronous W-TinyLFU updates.
//
// Get and Peek read the sharded table without a cache-wide lock. Get records
// exact statistics and offers a best-effort policy-read event. Put and Delete
// update the value table immediately and enqueue reliable policy events. A
// full write buffer falls back to synchronous maintenance. Frequency, Stats,
// All, Clear, and caller-held Lock drain pending maintenance first.
//
// A weighted cache's weigher runs before the value table is changed. Eviction
// vetoes run during policy maintenance. Neither callback may call back into
// the same cache. Do not copy a TinyLfu after first use. Run tests with -race.
package tiny_lfu_ts

import (
	"hash/maphash"
	"iter"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/pschlump/pluto/sharded_hash_ts"
	"github.com/pschlump/pluto/tiny_lfu"
)

const (
	readBufferSize  = 16
	writeBufferSize = 1024
)

// Stats is the plain package's cache statistics snapshot.
type Stats = tiny_lfu.Stats

type cacheRecord[K comparable, V any] struct {
	key    K
	value  V
	weight int
	id     uint64
}

type writeEvent[K comparable, V any] struct {
	record cacheRecord[K, V]
	delete bool
}

type keyValue[K comparable, V any] struct {
	key   K
	value V
}

// TinyLfu is a buffered, thread-safe W-TinyLFU cache. The zero value reads as
// empty but cannot be written to.
type TinyLfu[K comparable, V any] struct {
	data   *sharded_hash_ts.ShardedHash[cacheRecord[K, V]]
	policy *tiny_lfu.TinyLfu[K, cacheRecord[K, V]]

	capacity  int
	weigher   func(K, V) int
	evictable func(K, V) bool

	readBuffers []ringBuffer[K]
	writeBuffer ringBuffer[writeEvent[K, V]]

	hash   func(K) uint64
	nextID atomic.Uint64

	readDropped []paddedCounter
	misses      atomic.Uint64
	size        atomic.Int64

	workerActive atomic.Bool
	maintenance  sync.Mutex
	mutationGate sync.RWMutex
}

// NewTinyLfu creates an entry-count-bounded cache with the default geometry.
// It panics if capacity < 1.
// Complexity is O(stripes + sketch size) for initialization.
func NewTinyLfu[K comparable, V any](capacity int) *TinyLfu[K, V] {
	if capacity < 1 {
		panic("tiny_lfu_ts: NewTinyLfu called with capacity < 1")
	}
	return newTinyLfu[K, V](capacity, 0, 0, false, nil, nil)
}

// NewTinyLfuFunc creates an entry-count-bounded cache with an eviction veto.
// The callback runs during policy maintenance and must not call back into the
// cache. If no entry is evictable, capacity remains a soft cap.
// Complexity is O(stripes + sketch size) for initialization.
func NewTinyLfuFunc[K comparable, V any](capacity int, evictable func(K, V) bool) *TinyLfu[K, V] {
	if capacity < 1 {
		panic("tiny_lfu_ts: NewTinyLfuFunc called with capacity < 1")
	}
	return newTinyLfu[K, V](capacity, 0, 0, false, nil, evictable)
}

// NewTinyLfuWithSegments creates an entry-count-bounded cache with explicit
// window and protected capacities. It panics on invalid segment geometry.
// Complexity is O(stripes + sketch size) for initialization.
func NewTinyLfuWithSegments[K comparable, V any](capacity, window, protected int) *TinyLfu[K, V] {
	if capacity < 1 || window < 0 || protected < 0 || window > capacity-protected {
		panic("tiny_lfu_ts: invalid W-TinyLFU capacity or segment sizes")
	}
	return newTinyLfu[K, V](capacity, window, protected, true, nil, nil)
}

// NewTinyLfuWeighted creates a weight-bounded cache using weigher. The weigher
// runs outside the value-table locks and must not call back into the cache. It
// panics if maximumWeight < 1 or weigher is nil.
// Complexity is O(stripes + sketch size) for initialization.
func NewTinyLfuWeighted[K comparable, V any](maximumWeight int, weigher func(K, V) int) *TinyLfu[K, V] {
	if maximumWeight < 1 || weigher == nil {
		panic("tiny_lfu_ts: NewTinyLfuWeighted requires positive capacity and a non-nil weigher")
	}
	return newTinyLfu[K, V](maximumWeight, 0, 0, false, weigher, nil)
}

func newTinyLfu[K comparable, V any](capacity, window, protected int, explicitSegments bool,
	weigher func(K, V) int, evictable func(K, V) bool) *TinyLfu[K, V] {
	processors := max(runtime.GOMAXPROCS(0), 1)
	c := &TinyLfu[K, V]{
		capacity:    capacity,
		weigher:     weigher,
		evictable:   evictable,
		writeBuffer: newRingBuffer[writeEvent[K, V]](writeBufferSize),
		readBuffers: make([]ringBuffer[K], processors),
		readDropped: make([]paddedCounter, processors),
	}
	for i := range c.readBuffers {
		c.readBuffers[i] = newRingBuffer[K](readBufferSize)
	}

	seed := maphash.MakeSeed()
	c.hash = func(key K) uint64 { return maphash.Comparable(seed, key) }
	c.data = sharded_hash_ts.NewShardedHashFunc(
		func(a, b cacheRecord[K, V]) bool { return a.key == b.key },
		func(record cacheRecord[K, V]) uint64 { return c.hash(record.key) },
		16*processors, 16, 0,
	)

	var policyWeigher func(K, cacheRecord[K, V]) int
	if weigher != nil {
		policyWeigher = func(_ K, record cacheRecord[K, V]) int { return record.weight }
	}
	var policyEvictable func(K, cacheRecord[K, V]) bool
	if evictable != nil {
		policyEvictable = func(_ K, record cacheRecord[K, V]) bool {
			return evictable(record.key, record.value)
		}
	}
	if explicitSegments {
		c.policy = tiny_lfu.NewTinyLfuWithSegmentsAndHooks(capacity, window, protected,
			policyWeigher, policyEvictable, c.evicted)
	} else {
		c.policy = tiny_lfu.NewTinyLfuWithHooks(capacity,
			policyWeigher, policyEvictable, c.evicted)
	}
	return c
}

// Get returns the current value stored under key. A hit updates exact hit
// statistics and offers a best-effort policy-read event; a miss updates exact
// miss statistics.
// Complexity is O(1) average.
func (c *TinyLfu[K, V]) Get(key K) (V, bool) {
	if c == nil || c.data == nil {
		var zero V
		return zero, false
	}
	record, ok := c.data.Search(cacheRecord[K, V]{key: key})
	if !ok {
		c.misses.Add(1)
		var zero V
		return zero, false
	}
	stripe := record.id % uint64(len(c.readBuffers))
	c.offerRead(key, stripe)
	return record.value, true
}

// Peek returns the current value without updating policy or statistics.
// Complexity is O(1) average.
func (c *TinyLfu[K, V]) Peek(key K) (V, bool) {
	if c == nil || c.data == nil {
		var zero V
		return zero, false
	}
	record, ok := c.data.Search(cacheRecord[K, V]{key: key})
	if !ok {
		var zero V
		return zero, false
	}
	return record.value, true
}

// Put inserts or replaces a value with immediate visibility, then queues its
// policy update. It panics on a nil or zero-value cache or a negative weight.
// Complexity is amortized O(1); a full policy-write buffer may synchronously
// drain pending maintenance.
func (c *TinyLfu[K, V]) Put(key K, value V) {
	if c == nil {
		panic("tiny_lfu_ts: Put on a nil cache: create it with NewTinyLfu")
	}
	if c.data == nil {
		panic("tiny_lfu_ts: Put on a zero-value cache: create it with NewTinyLfu")
	}
	weight := 1
	if c.weigher != nil {
		weight = c.weigher(key, value)
		if weight < 0 {
			panic("tiny_lfu_ts: weigher returned a negative weight")
		}
	}
	record := cacheRecord[K, V]{key: key, value: value, weight: weight, id: c.nextID.Add(1)}
	c.mutationGate.RLock()
	fallback := c.storeBuffered(record)
	c.mutationGate.RUnlock()
	if fallback {
		c.writeFallback(writeEvent[K, V]{record: record})
	}
}

// Delete removes key with immediate visibility and queues its policy removal.
// It reports whether key was present.
// Complexity is O(1) average, plus possible synchronous maintenance when the
// policy-write buffer is full.
func (c *TinyLfu[K, V]) Delete(key K) bool {
	if c == nil || c.data == nil {
		return false
	}
	record := cacheRecord[K, V]{key: key}
	c.mutationGate.RLock()
	found, weight, fallback := c.deleteBuffered(record)
	c.mutationGate.RUnlock()
	if fallback {
		c.writeFallback(writeEvent[K, V]{record: record, delete: true})
	}
	if found {
		c.size.Add(-int64(weight))
	}
	return found
}

// Len returns the number of currently visible entries.
// Complexity is O(1).
func (c *TinyLfu[K, V]) Len() int {
	if c == nil || c.data == nil {
		return 0
	}
	return c.data.Len()
}

// Capacity returns the configured entry or weight bound.
// Complexity is O(1).
func (c *TinyLfu[K, V]) Capacity() int {
	if c == nil {
		return 0
	}
	return c.capacity
}

// Size returns the current total entry weight, or Len for an unweighted cache.
// Policy maintenance may transiently leave Size above Capacity when eviction
// vetoes require the soft cap.
// Complexity is O(1).
func (c *TinyLfu[K, V]) Size() int {
	if c == nil {
		return 0
	}
	return int(c.size.Load())
}

// Frequency drains pending maintenance and returns key's estimated frequency.
// Complexity is O(1) plus the pending maintenance work.
func (c *TinyLfu[K, V]) Frequency(key K) int {
	if c == nil || c.data == nil {
		return 0
	}
	c.maintenance.Lock()
	c.drainLocked()
	frequency := c.policy.Frequency(key)
	c.maintenance.Unlock()
	return frequency
}

// Stats drains policy maintenance and returns a statistics snapshot. Hits and
// misses are exact; buffered read events do not affect those counters.
// Complexity is O(1) plus the pending maintenance work.
func (c *TinyLfu[K, V]) Stats() Stats {
	if c == nil || c.data == nil {
		return Stats{}
	}
	c.maintenance.Lock()
	c.drainLocked()
	stats := c.policy.Stats()
	c.maintenance.Unlock()
	for i := range c.readBuffers {
		stats.Hits += c.readBuffers[i].tail.value.Load() + c.readDropped[i].value.Load()
	}
	stats.Misses = c.misses.Load()
	return stats
}

// Clear removes all currently visible entries, drains prior events, and clears
// policy residency while retaining configuration, frequency history, and
// statistics. Concurrent writes are excluded until Clear returns.
// Complexity is O(n) plus pending maintenance.
func (c *TinyLfu[K, V]) Clear() {
	if c == nil || c.data == nil {
		return
	}
	c.mutationGate.Lock()
	c.data.Truncate()
	c.size.Store(0)
	c.maintenance.Lock()
	c.drainLocked()
	c.policy.Clear()
	c.maintenance.Unlock()
	c.mutationGate.Unlock()
}

// All drains maintenance and returns an iterator over a snapshot in policy
// eviction order. Iteration runs without locks and may safely mutate the cache.
// Complexity is O(n) time and space plus pending maintenance.
func (c *TinyLfu[K, V]) All() iter.Seq2[K, V] {
	if c == nil || c.data == nil {
		return func(func(K, V) bool) {}
	}
	c.mutationGate.RLock()
	c.maintenance.Lock()
	c.drainLocked()
	pairs := make([]keyValue[K, V], 0, c.policy.Len())
	for key, record := range c.policy.All() {
		pairs = append(pairs, keyValue[K, V]{key: key, value: record.value})
	}
	c.maintenance.Unlock()
	c.mutationGate.RUnlock()
	return func(yield func(K, V) bool) {
		for _, pair := range pairs {
			if !yield(pair.key, pair.value) {
				return
			}
		}
	}
}

// Lock excludes Put, Delete, Clear, and All, drains pending maintenance, and
// holds the compound-operation gate until Unlock. Lookups may proceed. Use the
// Nl-prefixed methods inside; regular methods that need the gate deadlock.
func (c *TinyLfu[K, V]) Lock() {
	if c == nil {
		return
	}
	c.mutationGate.Lock()
	if c.data != nil {
		c.maintenance.Lock()
		c.drainLocked()
		c.maintenance.Unlock()
	}
}

// Unlock releases the compound-operation gate. A nil receiver is a no-op.
func (c *TinyLfu[K, V]) Unlock() {
	if c == nil {
		return
	}
	c.mutationGate.Unlock()
}

// NlGet is Get without compound-gate locking; call it only while holding Lock.
func (c *TinyLfu[K, V]) NlGet(key K) (V, bool) { return c.Get(key) }

// NlPeek is Peek without compound-gate locking; call it only while holding Lock.
func (c *TinyLfu[K, V]) NlPeek(key K) (V, bool) { return c.Peek(key) }

// NlPut applies Put immediately, including policy maintenance, while holding
// Lock.
func (c *TinyLfu[K, V]) NlPut(key K, value V) {
	if c == nil || c.data == nil {
		panic("tiny_lfu_ts: NlPut on a nil or zero-value cache")
	}
	weight := 1
	if c.weigher != nil {
		weight = c.weigher(key, value)
		if weight < 0 {
			panic("tiny_lfu_ts: weigher returned a negative weight")
		}
	}
	record := cacheRecord[K, V]{key: key, value: value, weight: weight, id: c.nextID.Add(1)}
	c.storeDirect(record)
}

// NlDelete applies Delete immediately, including policy maintenance, while
// holding Lock.
func (c *TinyLfu[K, V]) NlDelete(key K) bool {
	if c == nil || c.data == nil {
		return false
	}
	record := cacheRecord[K, V]{key: key}
	found, weight := c.deleteDirect(record)
	if found {
		c.size.Add(-int64(weight))
	}
	return found
}

// NlLen is Len without compound-gate locking; call it only while holding Lock.
func (c *TinyLfu[K, V]) NlLen() int { return c.Len() }

// NlCapacity is Capacity without compound-gate locking.
func (c *TinyLfu[K, V]) NlCapacity() int { return c.Capacity() }

// NlSize is Size without compound-gate locking; call it only while holding Lock.
func (c *TinyLfu[K, V]) NlSize() int { return c.Size() }

// NlFrequency is Frequency without compound-gate locking.
func (c *TinyLfu[K, V]) NlFrequency(key K) int { return c.Frequency(key) }

// NlStats is Stats without compound-gate locking.
func (c *TinyLfu[K, V]) NlStats() Stats { return c.Stats() }

// NlClear applies Clear immediately while holding Lock.
func (c *TinyLfu[K, V]) NlClear() {
	if c == nil || c.data == nil {
		return
	}
	c.data.Truncate()
	c.size.Store(0)
	c.maintenance.Lock()
	c.policy.Clear()
	c.maintenance.Unlock()
}

func (c *TinyLfu[K, V]) storeBuffered(record cacheRecord[K, V]) bool {
	lock := c.data.KeyMutex(record)
	lock.Lock()
	prior, found := c.data.NlSearch(record)
	c.data.NlInsert(record)
	fallback := !c.offerWrite(writeEvent[K, V]{record: record})
	lock.Unlock()
	if found {
		c.size.Add(int64(record.weight - prior.weight))
	} else {
		c.size.Add(int64(record.weight))
	}
	return fallback
}

func (c *TinyLfu[K, V]) deleteBuffered(record cacheRecord[K, V]) (bool, int, bool) {
	lock := c.data.KeyMutex(record)
	lock.Lock()
	prior, found := c.data.NlSearch(record)
	fallback := false
	if found {
		c.data.NlDelete(record)
		fallback = !c.offerWrite(writeEvent[K, V]{record: record, delete: true})
	}
	lock.Unlock()
	if !found {
		return false, 0, false
	}
	return true, prior.weight, fallback
}

func (c *TinyLfu[K, V]) storeDirect(record cacheRecord[K, V]) {
	lock := c.data.KeyMutex(record)
	lock.Lock()
	prior, found := c.data.NlSearch(record)
	c.data.NlInsert(record)
	lock.Unlock()
	if found {
		c.size.Add(int64(record.weight - prior.weight))
	} else {
		c.size.Add(int64(record.weight))
	}
	c.maintenance.Lock()
	c.policy.Put(record.key, record)
	c.maintenance.Unlock()
}

func (c *TinyLfu[K, V]) deleteDirect(record cacheRecord[K, V]) (bool, int) {
	lock := c.data.KeyMutex(record)
	lock.Lock()
	prior, found := c.data.NlSearch(record)
	if found {
		c.data.NlDelete(record)
	}
	lock.Unlock()
	if found {
		c.maintenance.Lock()
		c.policy.Delete(record.key)
		c.maintenance.Unlock()
		return true, prior.weight
	}
	return false, 0
}

func (c *TinyLfu[K, V]) evicted(_ K, record cacheRecord[K, V]) {
	lock := c.data.KeyMutex(record)
	lock.Lock()
	current, found := c.data.NlSearch(record)
	if found && current.id == record.id {
		c.data.NlDelete(record)
		c.size.Add(-int64(record.weight))
	}
	lock.Unlock()
}

func (c *TinyLfu[K, V]) offerRead(key K, stripe uint64) {
	accepted, pending := c.readBuffers[stripe].offer(key)
	if !accepted {
		c.readDropped[stripe].value.Add(1)
		c.scheduleMaintenance()
	} else if pending >= readBufferSize/2 {
		c.scheduleMaintenance()
	}
}

func (c *TinyLfu[K, V]) offerWrite(event writeEvent[K, V]) bool {
	accepted, _ := c.writeBuffer.offer(event)
	if accepted {
		c.scheduleMaintenance()
	}
	return accepted
}

func (c *TinyLfu[K, V]) writeFallback(event writeEvent[K, V]) {
	c.maintenance.Lock()
	c.drainLocked()
	c.applyWrite(event)
	c.maintenance.Unlock()
	c.scheduleMaintenance()
}

func (c *TinyLfu[K, V]) scheduleMaintenance() {
	if c.workerActive.CompareAndSwap(false, true) {
		go c.drainWorker()
	}
}

func (c *TinyLfu[K, V]) drainWorker() {
	for {
		c.maintenance.Lock()
		c.drainLocked()
		c.maintenance.Unlock()
		if !c.hasPending() {
			c.workerActive.Store(false)
			if c.hasPending() && c.workerActive.CompareAndSwap(false, true) {
				continue
			}
			return
		}
	}
}

func (c *TinyLfu[K, V]) hasPending() bool {
	if c.writeBuffer.pending() != 0 {
		return true
	}
	for i := range c.readBuffers {
		if c.readBuffers[i].pending() != 0 {
			return true
		}
	}
	return false
}

func (c *TinyLfu[K, V]) drainLocked() {
	for i := range c.readBuffers {
		c.readBuffers[i].drain(func(key K) { c.policy.RecordAccess(key) })
	}
	c.writeBuffer.drain(c.applyWrite)
}

func (c *TinyLfu[K, V]) applyWrite(event writeEvent[K, V]) {
	current, found := c.data.Search(event.record)
	if event.delete {
		if !found {
			c.policy.Delete(event.record.key)
		}
		return
	}
	if found && current.id == event.record.id {
		c.policy.Put(event.record.key, event.record)
	}
}
