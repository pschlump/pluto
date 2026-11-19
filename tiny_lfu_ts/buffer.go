/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tiny_lfu_ts

import "sync/atomic"

type paddedCounter struct {
	value atomic.Uint64
	_     [56]byte
}

type ringSlot[T any] struct {
	value T
	ready atomic.Bool
}

// ringBuffer is a bounded MPSC ring with one maintenance consumer. Producers
// claim slots with CAS and publish with a release store. A full or contended
// offer reports false instead of waiting.
type ringBuffer[T any] struct {
	head  paddedCounter
	tail  paddedCounter
	slots []ringSlot[T]
}

func newRingBuffer[T any](capacity int) ringBuffer[T] {
	return ringBuffer[T]{slots: make([]ringSlot[T], capacity)}
}

func (r *ringBuffer[T]) offer(value T) (bool, int) {
	head := r.head.value.Load()
	tail := r.tail.value.Load()
	pending := tail - head
	if int(pending) >= len(r.slots) || !r.tail.value.CompareAndSwap(tail, tail+1) {
		return false, int(pending)
	}
	slot := &r.slots[tail%uint64(len(r.slots))]
	slot.value = value
	slot.ready.Store(true)
	return true, int(pending + 1)
}

func (r *ringBuffer[T]) drain(consume func(T)) {
	head := r.head.value.Load()
	tail := r.tail.value.Load()
	for head < tail {
		slot := &r.slots[head%uint64(len(r.slots))]
		if !slot.ready.Load() {
			break
		}
		value := slot.value
		var zero T
		slot.value = zero
		slot.ready.Store(false)
		consume(value)
		head++
	}
	r.head.value.Store(head)
}

func (r *ringBuffer[T]) pending() int {
	return int(r.tail.value.Load() - r.head.value.Load())
}
