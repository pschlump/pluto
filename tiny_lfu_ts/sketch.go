/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tiny_lfu_ts

import (
	"sync"

	"github.com/pschlump/pluto/tiny_lfu"
)

// FrequencySketch is an aging Count-Min sketch guarded by one sync.RWMutex.
// The zero value tolerates reads and can be initialized with EnsureCapacity.
// Do not copy a FrequencySketch after first use.
type FrequencySketch[K comparable] struct {
	inner *tiny_lfu.FrequencySketch[K]
	lock  sync.RWMutex
}

// NewFrequencySketch creates a sketch sized for maximumSize cache entries. It
// panics if maximumSize is negative.
// Complexity is O(sketch size) for the allocation.
func NewFrequencySketch[K comparable](maximumSize int) *FrequencySketch[K] {
	if maximumSize < 0 {
		panic("tiny_lfu_ts: NewFrequencySketch called with a negative maximum size")
	}
	return &FrequencySketch[K]{inner: tiny_lfu.NewFrequencySketch[K](maximumSize)}
}

// NewFrequencySketchWithHash creates a sketch using hash for key hashing. It
// panics if maximumSize is negative or hash is nil.
// Complexity is O(sketch size) for the allocation.
func NewFrequencySketchWithHash[K comparable](maximumSize int, hash func(K) uint64) *FrequencySketch[K] {
	if maximumSize < 0 || hash == nil {
		panic("tiny_lfu_ts: NewFrequencySketchWithHash requires nonnegative capacity and a non-nil hash")
	}
	return &FrequencySketch[K]{inner: tiny_lfu.NewFrequencySketchWithHash(maximumSize, hash)}
}

// EnsureCapacity sizes the sketch under the write lock. Growing forgets all
// counts; retaining an oversized table preserves counts and retunes aging.
// Complexity is O(sketch size) when growing, O(1) otherwise.
func (s *FrequencySketch[K]) EnsureCapacity(maximumSize int) {
	if s == nil {
		panic("tiny_lfu_ts: EnsureCapacity on a nil FrequencySketch")
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.init()
	s.inner.EnsureCapacity(maximumSize)
}

// Frequency returns key's estimated frequency under the read lock. A nil or
// uninitialized sketch returns 0.
// Complexity is O(1).
func (s *FrequencySketch[K]) Frequency(key K) int {
	if s == nil {
		return 0
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.inner.Frequency(key)
}

// Increment records one access under the write lock. A nil or uninitialized
// sketch does nothing.
// Complexity is amortized O(1); an aging reset is O(sketch size).
func (s *FrequencySketch[K]) Increment(key K) {
	if s == nil {
		return
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.inner.Increment(key)
}

// Lock takes the write lock for a compound operation. Use Nl-prefixed methods
// before Unlock. A nil receiver is a no-op.
func (s *FrequencySketch[K]) Lock() {
	if s == nil {
		return
	}
	s.lock.Lock()
}

// Unlock releases the write lock taken by Lock. A nil receiver is a no-op.
func (s *FrequencySketch[K]) Unlock() {
	if s == nil {
		return
	}
	s.lock.Unlock()
}

func (s *FrequencySketch[K]) init() {
	if s.inner == nil {
		s.inner = &tiny_lfu.FrequencySketch[K]{}
	}
}

// NlEnsureCapacity is EnsureCapacity without locking; call it only while
// holding Lock.
func (s *FrequencySketch[K]) NlEnsureCapacity(maximumSize int) {
	s.init()
	s.inner.EnsureCapacity(maximumSize)
}

// NlFrequency is Frequency without locking; call it only while holding Lock.
func (s *FrequencySketch[K]) NlFrequency(key K) int { return s.inner.Frequency(key) }

// NlIncrement is Increment without locking; call it only while holding Lock.
func (s *FrequencySketch[K]) NlIncrement(key K) { s.inner.Increment(key) }
