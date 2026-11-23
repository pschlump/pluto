/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package min_max_pq_ts is the mutex-guarded twin of min_max_pq. Compound
// writes are atomic and iterators, clones, and equality use snapshots.
package min_max_pq_ts

import (
	"cmp"
	"iter"
	"slices"
	"sync"

	"github.com/pschlump/pluto/min_max_pq"
)

// MinMaxPriorityQueue is a thread-safe double-ended priority queue.
type MinMaxPriorityQueue[T any] struct {
	lock  sync.RWMutex
	plain *min_max_pq.MinMaxPriorityQueue[T]
}

func NewMinMaxPriorityQueue[T cmp.Ordered]() *MinMaxPriorityQueue[T] {
	return &MinMaxPriorityQueue[T]{plain: min_max_pq.NewMinMaxPriorityQueue[T]()}
}

func NewMinMaxPriorityQueueFunc[T any](compare func(T, T) int) *MinMaxPriorityQueue[T] {
	return &MinMaxPriorityQueue[T]{plain: min_max_pq.NewMinMaxPriorityQueueFunc(compare)}
}

func NewBoundedMinMaxPriorityQueue[T cmp.Ordered](maximumSize int) *MinMaxPriorityQueue[T] {
	return &MinMaxPriorityQueue[T]{plain: min_max_pq.NewBoundedMinMaxPriorityQueue[T](maximumSize)}
}

func NewBoundedMinMaxPriorityQueueFunc[T any](maximumSize int, compare func(T, T) int) *MinMaxPriorityQueue[T] {
	return &MinMaxPriorityQueue[T]{plain: min_max_pq.NewBoundedMinMaxPriorityQueueFunc(maximumSize, compare)}
}

func (q *MinMaxPriorityQueue[T]) writeContract(caller string) {
	if q == nil || q.plain == nil {
		panic("min_max_pq_ts: " + caller + " on a nil or zero-value MinMaxPriorityQueue; use a constructor")
	}
}

func (q *MinMaxPriorityQueue[T]) MaximumSize() (int, bool) {
	if q == nil || q.plain == nil {
		return 0, false
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.plain.MaximumSize()
}

func (q *MinMaxPriorityQueue[T]) Push(value T) {
	q.writeContract("Push")
	q.lock.Lock()
	defer q.lock.Unlock()
	q.plain.Push(value)
}

// PushAll atomically inserts all values in order.
func (q *MinMaxPriorityQueue[T]) PushAll(values []T) {
	if len(values) == 0 {
		return
	}
	q.writeContract("PushAll")
	q.lock.Lock()
	defer q.lock.Unlock()
	q.plain.PushAll(values)
}

func (q *MinMaxPriorityQueue[T]) PeekMin() (T, bool) {
	if q == nil || q.plain == nil {
		var zero T
		return zero, false
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.plain.PeekMin()
}

func (q *MinMaxPriorityQueue[T]) PeekMax() (T, bool) {
	if q == nil || q.plain == nil {
		var zero T
		return zero, false
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.plain.PeekMax()
}

func (q *MinMaxPriorityQueue[T]) PopMin() (T, bool) {
	if q == nil || q.plain == nil {
		var zero T
		return zero, false
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.plain.PopMin()
}

func (q *MinMaxPriorityQueue[T]) PopMax() (T, bool) {
	if q == nil || q.plain == nil {
		var zero T
		return zero, false
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.plain.PopMax()
}

func (q *MinMaxPriorityQueue[T]) Len() int {
	if q == nil || q.plain == nil {
		return 0
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.plain.Len()
}

func (q *MinMaxPriorityQueue[T]) IsEmpty() bool { return q.Len() == 0 }

func (q *MinMaxPriorityQueue[T]) Clear() {
	if q == nil || q.plain == nil {
		return
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	q.plain.Clear()
}

func (q *MinMaxPriorityQueue[T]) Slice() []T {
	if q == nil || q.plain == nil {
		return nil
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.plain.Slice()
}

// All returns a snapshot iterator in internal breadth-first heap order.
func (q *MinMaxPriorityQueue[T]) All() iter.Seq[T] {
	return slices.Values(q.Slice())
}

func snapshot[T any](queue *MinMaxPriorityQueue[T]) *min_max_pq.MinMaxPriorityQueue[T] {
	if queue == nil || queue.plain == nil {
		return nil
	}
	queue.lock.RLock()
	defer queue.lock.RUnlock()
	return queue.plain.Clone()
}

func (q *MinMaxPriorityQueue[T]) Clone() *MinMaxPriorityQueue[T] {
	plain := snapshot(q)
	if plain == nil {
		return nil
	}
	return &MinMaxPriorityQueue[T]{plain: plain}
}

// Equal snapshots each operand separately and never holds two queue locks.
func (q *MinMaxPriorityQueue[T]) Equal(other *MinMaxPriorityQueue[T]) bool {
	return snapshot(q).Equal(snapshot(other))
}
