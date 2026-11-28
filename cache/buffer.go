/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package cache

import "sync/atomic"

// readBufferSize is the per-shard read-buffer capacity (a power of two). A
// reader that leaves at least readDrainThreshold events pending tries to drain.
const (
	readBufferSize     = 64
	readDrainThreshold = readBufferSize / 2
)

type readSlot[K any, V any] struct {
	entry *cacheEntry[K, V] // written before seq is published
	at    int64             // access time; written before seq is published
	seq   atomic.Uint64     // ticket+1 once the slot for ticket is published
}

// readBuffer is a bounded multi-producer ring with one consumer at a time (the
// shard-lock holder). A producer claims ticket n by CAS on tail, fills slot
// n%size with plain writes, then publishes it by storing seq = n+1, a release.
// The consumer accepts slot n only when seq == n+1, so a stale slot from the
// previous lap is never mistaken for a new event. No atomic pointer stores
// appear anywhere, so the hot path carries no out-of-line write-barrier call.
// Unlike tiny_lfu_ts's buffers it is LOSSLESS: a full ring makes the producer
// take the shard lock instead of dropping the event, so hit statistics and
// LRU/access order stay exact.
type readBuffer[K any, V any] struct {
	head  atomic.Uint64
	_     [56]byte
	tail  atomic.Uint64
	_     [56]byte
	slots [readBufferSize]readSlot[K, V]
	_     [64]byte // keeps the next shard's lock off the last slot's cache line
}

// offer appends one read event. It reports false only when the ring is full,
// and returns the number of pending events after the append.
func (r *readBuffer[K, V]) offer(entry *cacheEntry[K, V], at int64) (bool, uint64) {
	for {
		tail := r.tail.Load()
		pending := tail - r.head.Load()
		if pending >= readBufferSize {
			return false, pending
		}
		if r.tail.CompareAndSwap(tail, tail+1) {
			slot := &r.slots[tail&(readBufferSize-1)]
			slot.entry = entry
			slot.at = at
			slot.seq.Store(tail + 1)
			return true, pending + 1
		}
	}
}

// drain consumes published events in order. The caller holds the shard lock.
func (r *readBuffer[K, V]) drain(consume func(*cacheEntry[K, V], int64)) {
	head := r.head.Load()
	tail := r.tail.Load()
	for ; head < tail; head++ {
		slot := &r.slots[head&(readBufferSize-1)]
		if slot.seq.Load() != head+1 {
			break // claimed but not yet published; the next drain resumes here
		}
		entry, at := slot.entry, slot.at
		slot.entry = nil // plain write: the next producer for this slot waits on head
		consume(entry, at)
	}
	r.head.Store(head)
}
