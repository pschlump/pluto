/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package tiny_lfu implements the TinyLFU admission policy and the W-TinyLFU
// cache from "TinyLFU: A Highly Efficient Cache Admission Policy" by Gil
// Einziger, Roy Friedman, and Ben Manes.
//
// New entries reside in an LRU admission window. Entries leaving the window
// compete for admission to a segmented-LRU main cache by comparing their
// estimated frequencies in an aging Count-Min sketch. The main cache has a
// probation segment for new residents and a protected segment for entries hit
// again. This combines scan resistance, frequency history, and recency.
//
// The implementation follows Caffeine's FrequencySketch and
// WindowTinyLfuPolicy, without Caffeine's concurrency buffers or adaptive
// window sizing. It is not safe for concurrent use; tiny_lfu_ts is the
// mutex-guarded twin.
package tiny_lfu

import "iter"

// Stats contains cache activity counters. Hits and Misses count Get calls;
// Put and Peek do not affect them. Evictions includes both rejected admission
// candidates and displaced residents.
type Stats struct {
	Hits       uint64
	Misses     uint64
	Evictions  uint64
	Admissions uint64
	Rejections uint64
}

type region uint8

const (
	windowRegion region = iota
	probationRegion
	protectedRegion
)

type entry[K comparable, V any] struct {
	key    K
	value  V
	weight int
	region region
	prev   *entry[K, V]
	next   *entry[K, V]
}

// TinyLfu is a capacity-bounded W-TinyLFU cache. Use NewTinyLfu or one of the
// other constructors; the zero value reads as empty but cannot be written to.
// With an eviction veto, capacity is a soft cap until an entry is evictable.
type TinyLfu[K comparable, V any] struct {
	data map[K]*entry[K, V]

	window    entry[K, V]
	probation entry[K, V]
	protected entry[K, V]

	maximumSize   int
	maxWindow     int
	maxProtected  int
	sizeWindow    int
	sizeProtected int
	sizeData      int

	sketch    *FrequencySketch[K]
	weigher   func(K, V) int
	evictable func(K, V) bool
	onEvict   func(K, V)
	stats     Stats
}

// NewTinyLfu creates an entry-count-bounded cache. The admission window
// receives one percent of the capacity, rounded up, and the protected segment
// receives 80 percent of the remaining capacity. It panics if capacity < 1.
func NewTinyLfu[K comparable, V any](capacity int) *TinyLfu[K, V] {
	maximumWindow, maximumProtected := defaultSegmentSizes(capacity)
	return newTinyLfu[K, V](capacity, maximumWindow, maximumProtected, nil)
}

// NewTinyLfuFunc creates an entry-count-bounded cache that consults evictable
// before removing a resident. A false answer vetoes that eviction. If no entry
// is evictable, the cache temporarily exceeds capacity until a later Put can
// make progress. A nil callback makes every entry evictable.
// Complexity is O(1) plus the sketch allocation.
func NewTinyLfuFunc[K comparable, V any](capacity int, evictable func(K, V) bool) *TinyLfu[K, V] {
	c := NewTinyLfu[K, V](capacity)
	c.evictable = evictable
	return c
}

// NewTinyLfuWithHooks creates a cache with optional weighing, eviction-veto,
// and eviction-notification callbacks. onEvict runs synchronously after an
// entry is removed by eviction, but not after Delete or Clear. A nil weigher
// uses unit weights; nil evictable and onEvict callbacks are no-ops.
// Complexity is O(1) plus the sketch allocation.
func NewTinyLfuWithHooks[K comparable, V any](capacity int, weigher func(K, V) int,
	evictable func(K, V) bool, onEvict func(K, V)) *TinyLfu[K, V] {
	window, protected := defaultSegmentSizes(capacity)
	return NewTinyLfuWithSegmentsAndHooks(capacity, window, protected, weigher, evictable, onEvict)
}

// NewTinyLfuWithSegmentsAndHooks is NewTinyLfuWithHooks with explicit segment
// capacities.
// Complexity is O(1) plus the sketch allocation.
func NewTinyLfuWithSegmentsAndHooks[K comparable, V any](capacity, window, protected int,
	weigher func(K, V) int, evictable func(K, V) bool, onEvict func(K, V)) *TinyLfu[K, V] {
	c := newTinyLfu(capacity, window, protected, weigher)
	c.evictable = evictable
	c.onEvict = onEvict
	return c
}

// NewTinyLfuWithSegments creates an entry-count-bounded cache with explicit
// window and protected capacities. The probation segment receives the
// remaining capacity. It panics if capacity < 1, either segment is negative,
// or window+protected exceeds capacity.
func NewTinyLfuWithSegments[K comparable, V any](capacity, window, protected int) *TinyLfu[K, V] {
	return newTinyLfu[K, V](capacity, window, protected, nil)
}

// NewTinyLfuWeighted creates a weight-bounded cache using weigher to compute
// each value's weight. Entries heavier than maximumWeight are not retained.
// It panics if maximumWeight < 1, weigher is nil, or weigher returns a
// negative weight.
func NewTinyLfuWeighted[K comparable, V any](maximumWeight int, weigher func(K, V) int) *TinyLfu[K, V] {
	if weigher == nil {
		panic("tiny_lfu: NewTinyLfuWeighted called with a nil weigher")
	}
	maximumWindow, maximumProtected := defaultSegmentSizes(maximumWeight)
	return newTinyLfu[K, V](maximumWeight, maximumWindow, maximumProtected, weigher)
}

func defaultSegmentSizes(capacity int) (window, protected int) {
	window = capacity / 100
	if capacity%100 != 0 {
		window++
	}
	main := capacity - window
	return window, main/5*4 + main%5*4/5
}

func newTinyLfu[K comparable, V any](capacity, window, protected int, weigher func(K, V) int) *TinyLfu[K, V] {
	if capacity < 1 {
		panic("tiny_lfu: NewTinyLfu called with capacity < 1: a cache needs room for at least one entry")
	}
	if window < 0 || protected < 0 || window > capacity-protected {
		panic("tiny_lfu: invalid W-TinyLFU segment sizes: window and protected must be nonnegative and fit within capacity")
	}
	c := &TinyLfu[K, V]{
		data:         make(map[K]*entry[K, V]),
		maximumSize:  capacity,
		maxWindow:    window,
		maxProtected: protected,
		sketch:       NewFrequencySketch[K](capacity),
		weigher:      weigher,
	}
	c.window.prev = &c.window
	c.window.next = &c.window
	c.probation.prev = &c.probation
	c.probation.next = &c.probation
	c.protected.prev = &c.protected
	c.protected.next = &c.protected
	return c
}

// Get returns the value stored under key and whether it was found. A hit
// records an access and updates recency. A miss only updates Misses; call Put
// after loading the value to record the access in the frequency sketch.
func (c *TinyLfu[K, V]) Get(key K) (V, bool) {
	var zero V
	if c == nil || c.data == nil {
		return zero, false
	}
	node, ok := c.data[key]
	if !ok {
		c.stats.Misses++
		return zero, false
	}
	c.stats.Hits++
	c.recordHit(node)
	return node.value, true
}

// RecordAccess records an access without changing Get statistics. It updates
// recency when resident and frequency history in all cases, and reports
// whether key is resident. It is intended for buffered maintenance that has
// already accounted for the access.
// Complexity is amortized O(1); an aging reset is O(sketch size).
func (c *TinyLfu[K, V]) RecordAccess(key K) bool {
	if c == nil || c.data == nil {
		return false
	}
	node, ok := c.data[key]
	if !ok {
		c.sketch.Increment(key)
		c.evict()
		return false
	}
	c.recordHit(node)
	return true
}

// Peek returns the value stored under key without recording an access,
// changing recency, or affecting statistics.
func (c *TinyLfu[K, V]) Peek(key K) (V, bool) {
	var zero V
	if c == nil || c.data == nil {
		return zero, false
	}
	node, ok := c.data[key]
	if !ok {
		return zero, false
	}
	return node.value, true
}

// Put inserts or updates an entry and records one access. New entries enter
// the admission window. Put panics on a nil or zero-value cache, or when a
// weighted cache's weigher returns a negative weight.
func (c *TinyLfu[K, V]) Put(key K, value V) {
	if c == nil {
		panic("tiny_lfu: Put on a nil cache: create it with NewTinyLfu")
	}
	if c.data == nil {
		panic("tiny_lfu: Put on a zero-value cache: create it with NewTinyLfu")
	}
	weight := c.weight(key, value)
	if node, ok := c.data[key]; ok {
		delta := weight - node.weight
		if node.region == windowRegion {
			c.sizeWindow += delta
		} else if node.region == protectedRegion {
			c.sizeProtected += delta
		}
		c.sizeData += delta
		node.value = value
		node.weight = weight
		c.recordHit(node)
		return
	}

	c.sketch.Increment(key)
	node := &entry[K, V]{key: key, value: value, weight: weight, region: windowRegion}
	node.appendToTail(&c.window)
	c.data[key] = node
	c.sizeWindow += weight
	c.sizeData += weight
	c.evict()
}

func (c *TinyLfu[K, V]) weight(key K, value V) int {
	if c.weigher == nil {
		return 1
	}
	weight := c.weigher(key, value)
	if weight < 0 {
		panic("tiny_lfu: weigher returned a negative weight")
	}
	return weight
}

func (c *TinyLfu[K, V]) recordHit(node *entry[K, V]) {
	c.sketch.Increment(node.key)
	switch node.region {
	case windowRegion:
		node.moveToTail(&c.window)
	case probationRegion:
		if node.weight > c.maxProtected {
			node.moveToTail(&c.probation)
			break
		}
		node.remove()
		node.region = protectedRegion
		node.appendToTail(&c.protected)
		c.sizeProtected += node.weight
		c.demoteProtected()
	case protectedRegion:
		node.moveToTail(&c.protected)
		c.demoteProtected()
	}
	c.evict()
}

func (c *TinyLfu[K, V]) demoteProtected() {
	for c.sizeProtected > c.maxProtected {
		node := c.protected.next
		node.remove()
		node.region = probationRegion
		node.appendToTail(&c.probation)
		c.sizeProtected -= node.weight
	}
}

func (c *TinyLfu[K, V]) evict() {
	if c.weigher != nil && c.sizeData >= c.maximumSize>>1 {
		c.sketch.EnsureCapacity(len(c.data))
	}
	c.evictFromMain(c.evictFromWindow())
}

func (c *TinyLfu[K, V]) evictFromWindow() *entry[K, V] {
	var first *entry[K, V]
	for c.sizeWindow > c.maxWindow {
		candidate := c.window.next
		candidate.remove()
		candidate.region = probationRegion
		candidate.appendToTail(&c.probation)
		c.sizeWindow -= candidate.weight
		if first == nil {
			first = candidate
		}
	}
	return first
}

func (c *TinyLfu[K, V]) evictFromMain(candidate *entry[K, V]) {
	searchedWindow := false
	victimHead := &c.probation
	victim := firstEntry(victimHead)
	for c.sizeData > c.maximumSize {
		if candidate == nil && !searchedWindow {
			candidate = firstEntry(&c.window)
			searchedWindow = true
		}
		if candidate == nil && victim == nil {
			switch victimHead {
			case &c.probation:
				victimHead = &c.protected
				victim = firstEntry(victimHead)
				continue
			case &c.protected:
				victimHead = &c.window
				victim = firstEntry(victimHead)
				continue
			default:
				return
			}
		}
		if candidate != nil && !c.canEvict(candidate) {
			candidate = c.successor(candidate)
			continue
		}
		if victim != nil && !c.canEvict(victim) {
			victim = c.successor(victim)
			continue
		}
		if victim == nil {
			evict := candidate
			candidate = c.successor(candidate)
			c.evictEntry(evict)
			continue
		}
		if candidate == nil {
			evict := victim
			victim = c.successor(victim)
			c.evictEntry(evict)
			continue
		}
		if candidate == victim {
			victim = c.successor(victim)
			c.evictEntry(candidate)
			candidate = nil
			continue
		}
		if candidate.weight > c.maximumSize {
			evict := candidate
			candidate = c.successor(candidate)
			c.evictEntry(evict)
			continue
		}
		if c.admit(candidate.key, victim.key) {
			evict := victim
			victim = c.successor(victim)
			c.evictEntry(evict)
			candidate = c.successor(candidate)
		} else {
			evict := candidate
			candidate = c.successor(candidate)
			c.evictEntry(evict)
		}
	}
}

func (c *TinyLfu[K, V]) admit(candidate, victim K) bool {
	if c.sketch.Frequency(candidate) > c.sketch.Frequency(victim) {
		c.stats.Admissions++
		return true
	}
	c.stats.Rejections++
	return false
}

func (c *TinyLfu[K, V]) canEvict(node *entry[K, V]) bool {
	return c.evictable == nil || c.evictable(node.key, node.value)
}

func (c *TinyLfu[K, V]) evictEntry(node *entry[K, V]) {
	if node.region == windowRegion {
		c.sizeWindow -= node.weight
	} else if node.region == protectedRegion {
		c.sizeProtected -= node.weight
	}
	c.sizeData -= node.weight
	delete(c.data, node.key)
	node.remove()
	c.stats.Evictions++
	if c.onEvict != nil {
		c.onEvict(node.key, node.value)
	}
}

func firstEntry[K comparable, V any](head *entry[K, V]) *entry[K, V] {
	if head.next == head {
		return nil
	}
	return head.next
}

func (c *TinyLfu[K, V]) successor(node *entry[K, V]) *entry[K, V] {
	head := &c.protected
	if node.region == windowRegion {
		head = &c.window
	} else if node.region == probationRegion {
		head = &c.probation
	}
	if node.next == head {
		return nil
	}
	return node.next
}

func (node *entry[K, V]) moveToTail(head *entry[K, V]) {
	node.remove()
	node.appendToTail(head)
}

func (node *entry[K, V]) appendToTail(head *entry[K, V]) {
	tail := head.prev
	head.prev = node
	tail.next = node
	node.next = head
	node.prev = tail
}

func (node *entry[K, V]) remove() {
	node.prev.next = node.next
	node.next.prev = node.prev
	node.next = nil
	node.prev = nil
}

// Delete removes key without recording an eviction and reports whether it was
// present. Its frequency history remains in the sketch.
func (c *TinyLfu[K, V]) Delete(key K) bool {
	if c == nil || c.data == nil {
		return false
	}
	node, ok := c.data[key]
	if !ok {
		return false
	}
	if node.region == windowRegion {
		c.sizeWindow -= node.weight
	} else if node.region == protectedRegion {
		c.sizeProtected -= node.weight
	}
	c.sizeData -= node.weight
	delete(c.data, key)
	node.remove()
	return true
}

// Len returns the number of resident entries.
func (c *TinyLfu[K, V]) Len() int {
	if c == nil {
		return 0
	}
	return len(c.data)
}

// Capacity returns the entry or weight bound supplied to the constructor.
func (c *TinyLfu[K, V]) Capacity() int {
	if c == nil {
		return 0
	}
	return c.maximumSize
}

// Size returns the total resident weight, or Len for an unweighted cache.
func (c *TinyLfu[K, V]) Size() int {
	if c == nil {
		return 0
	}
	return c.sizeData
}

// Frequency returns key's estimated recent access frequency from 0 through
// 15, including when key is not currently resident.
func (c *TinyLfu[K, V]) Frequency(key K) int {
	if c == nil {
		return 0
	}
	return c.sketch.Frequency(key)
}

// Stats returns a snapshot of the cache's activity counters.
func (c *TinyLfu[K, V]) Stats() Stats {
	if c == nil {
		return Stats{}
	}
	return c.stats
}

// Clear removes every resident entry while retaining capacity, frequency
// history, and statistics.
func (c *TinyLfu[K, V]) Clear() {
	if c == nil || c.data == nil {
		return
	}
	clear(c.data)
	c.window.prev = &c.window
	c.window.next = &c.window
	c.probation.prev = &c.probation
	c.probation.next = &c.probation
	c.protected.prev = &c.protected
	c.protected.next = &c.protected
	c.sizeWindow = 0
	c.sizeProtected = 0
	c.sizeData = 0
}

// All returns an iterator over every resident key/value pair in eviction order:
// window, probation, then protected, each from least to most recently used. It
// walks live lists and must not mutate the cache during iteration. A nil or
// zero-value cache yields nothing.
// Complexity is O(1) per yielded entry.
func (c *TinyLfu[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if c == nil || c.data == nil {
			return
		}
		for _, head := range []*entry[K, V]{&c.window, &c.probation, &c.protected} {
			for node := head.next; node != head; node = node.next {
				if !yield(node.key, node.value) {
					return
				}
			}
		}
	}
}

// Lock and Unlock are no-ops kept so code written against the tiny_lfu_ts
// twin compiles unchanged. The plain package is not safe for concurrent use.
func (c *TinyLfu[K, V]) Lock() {}

// Unlock is the other half of the no-op lock pair; see Lock.
func (c *TinyLfu[K, V]) Unlock() {}
