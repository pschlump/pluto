/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package evicting_queue_ts is the mutex-guarded twin of evicting_queue.
// Bulk writes are atomic and all iterators are snapshots.
package evicting_queue_ts

import (
	"iter"
	"sync"

	"github.com/pschlump/pluto/evicting_queue"
)

// Queue is a thread-safe fixed-capacity evicting FIFO queue.
type Queue[T any] struct {
	lock  sync.RWMutex
	plain *evicting_queue.Queue[T]
}

// NewQueue creates an empty queue. Capacity zero accepts and discards writes.
func NewQueue[T any](capacity int) *Queue[T] {
	return &Queue[T]{plain: evicting_queue.NewQueue[T](capacity)}
}

func (q *Queue[T]) writeContract(caller string) {
	if q == nil || q.plain == nil {
		panic("evicting_queue_ts: " + caller + " on a nil or zero-value Queue; use NewQueue")
	}
}

func (q *Queue[T]) Capacity() int {
	if q == nil || q.plain == nil {
		return 0
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.plain.Capacity()
}

func (q *Queue[T]) RemainingCapacity() int {
	if q == nil || q.plain == nil {
		return 0
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.plain.RemainingCapacity()
}

func (q *Queue[T]) Len() int {
	if q == nil || q.plain == nil {
		return 0
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.plain.Len()
}

func (q *Queue[T]) IsEmpty() bool { return q.Len() == 0 }

func (q *Queue[T]) Push(value T) {
	q.writeContract("Push")
	q.lock.Lock()
	defer q.lock.Unlock()
	q.plain.Push(value)
}

// PushAll atomically appends all values, retaining the newest values that fit.
func (q *Queue[T]) PushAll(values []T) {
	if len(values) == 0 {
		return
	}
	q.writeContract("PushAll")
	q.lock.Lock()
	defer q.lock.Unlock()
	q.plain.PushAll(values)
}

func (q *Queue[T]) Pop() (T, bool) {
	if q == nil || q.plain == nil {
		var zero T
		return zero, false
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.plain.Pop()
}

func (q *Queue[T]) Peek() (T, bool) {
	if q == nil || q.plain == nil {
		var zero T
		return zero, false
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.plain.Peek()
}

func (q *Queue[T]) Clear() {
	if q == nil || q.plain == nil {
		return
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	q.plain.Clear()
}

func (q *Queue[T]) Slice() []T {
	if q == nil || q.plain == nil {
		return nil
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.plain.Slice()
}

// All returns an oldest-to-newest snapshot iterator.
func (q *Queue[T]) All() iter.Seq[T] {
	values := q.Slice()
	return func(yield func(T) bool) {
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}
}

func (q *Queue[T]) Clone() *Queue[T] {
	if q == nil || q.plain == nil {
		return nil
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return &Queue[T]{plain: q.plain.Clone()}
}

// Equal snapshots each queue separately and never holds two queue locks.
func (q *Queue[T]) Equal(other *Queue[T]) bool {
	left, right := q.Slice(), other.Slice()
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if any(left[index]) != any(right[index]) {
			return false
		}
	}
	return true
}
