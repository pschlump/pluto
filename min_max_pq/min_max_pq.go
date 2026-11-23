/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package min_max_pq implements a double-ended priority queue as a single-array
// min-max heap. Minimum and maximum peeks are O(1); insertions and removals at
// either end are O(log n).
package min_max_pq

import (
	"cmp"
	"fmt"
	"iter"
	"math/bits"
	"slices"
)

// MinMaxPriorityQueue is a generic double-ended priority queue. Its zero value
// is empty and read-only; use a constructor before storing values.
type MinMaxPriorityQueue[T any] struct {
	data        []T
	compare     func(T, T) int
	maximumSize int
}

// NewMinMaxPriorityQueue creates an unbounded queue using natural ordering.
func NewMinMaxPriorityQueue[T cmp.Ordered]() *MinMaxPriorityQueue[T] {
	return NewMinMaxPriorityQueueFunc(cmp.Compare[T])
}

// NewMinMaxPriorityQueueFunc creates an unbounded queue using compare.
func NewMinMaxPriorityQueueFunc[T any](compare func(T, T) int) *MinMaxPriorityQueue[T] {
	if compare == nil {
		panic("min_max_pq: NewMinMaxPriorityQueueFunc called with a nil comparison function")
	}
	return &MinMaxPriorityQueue[T]{compare: compare, maximumSize: -1}
}

// NewBoundedMinMaxPriorityQueue creates a queue that retains at most
// maximumSize of its least elements. The bound must be positive.
func NewBoundedMinMaxPriorityQueue[T cmp.Ordered](maximumSize int) *MinMaxPriorityQueue[T] {
	return NewBoundedMinMaxPriorityQueueFunc(maximumSize, cmp.Compare[T])
}

// NewBoundedMinMaxPriorityQueueFunc is NewBoundedMinMaxPriorityQueue with a
// caller-supplied comparison function.
func NewBoundedMinMaxPriorityQueueFunc[T any](maximumSize int, compare func(T, T) int) *MinMaxPriorityQueue[T] {
	if maximumSize <= 0 {
		panic("min_max_pq: bounded queue requires a positive maximum size")
	}
	if compare == nil {
		panic("min_max_pq: NewBoundedMinMaxPriorityQueueFunc called with a nil comparison function")
	}
	return &MinMaxPriorityQueue[T]{compare: compare, maximumSize: maximumSize}
}

func (q *MinMaxPriorityQueue[T]) writeContract(caller string) {
	if q == nil || q.compare == nil {
		panic(fmt.Sprintf("min_max_pq: %s on a nil or zero-value MinMaxPriorityQueue; use a constructor", caller))
	}
}

// MaximumSize returns the configured bound and true, or zero and false when
// the queue is unbounded.
func (q *MinMaxPriorityQueue[T]) MaximumSize() (int, bool) {
	if q == nil || q.maximumSize <= 0 {
		return 0, false
	}
	return q.maximumSize, true
}

// Push inserts value. At a full bound, the greatest of the old contents and
// value is evicted; an incoming value equal to the current maximum is discarded.
func (q *MinMaxPriorityQueue[T]) Push(value T) {
	q.writeContract("Push")
	if q.maximumSize > 0 && len(q.data) == q.maximumSize {
		maximum := q.data[q.maxIndex()]
		if q.compare(value, maximum) >= 0 {
			return
		}
		_, _ = q.PopMax()
	}
	q.data = append(q.data, value)
	q.bubbleUp(len(q.data) - 1)
}

// PushAll inserts values in order.
func (q *MinMaxPriorityQueue[T]) PushAll(values []T) {
	if len(values) == 0 {
		return
	}
	q.writeContract("PushAll")
	for _, value := range values {
		q.Push(value)
	}
}

// PeekMin returns the least element without removing it.
func (q *MinMaxPriorityQueue[T]) PeekMin() (T, bool) {
	if q == nil || len(q.data) == 0 {
		var zero T
		return zero, false
	}
	return q.data[0], true
}

// PeekMax returns the greatest element without removing it.
func (q *MinMaxPriorityQueue[T]) PeekMax() (T, bool) {
	if q == nil || len(q.data) == 0 {
		var zero T
		return zero, false
	}
	return q.data[q.maxIndex()], true
}

// PopMin removes and returns the least element.
func (q *MinMaxPriorityQueue[T]) PopMin() (T, bool) {
	if q == nil || len(q.data) == 0 {
		var zero T
		return zero, false
	}
	return q.removeAt(0), true
}

// PopMax removes and returns the greatest element.
func (q *MinMaxPriorityQueue[T]) PopMax() (T, bool) {
	if q == nil || len(q.data) == 0 {
		var zero T
		return zero, false
	}
	return q.removeAt(q.maxIndex()), true
}

func (q *MinMaxPriorityQueue[T]) maxIndex() int {
	if len(q.data) < 3 {
		return len(q.data) - 1
	}
	if q.compare(q.data[1], q.data[2]) >= 0 {
		return 1
	}
	return 2
}

func isMinLevel(index int) bool {
	return (bits.Len(uint(index+1))-1)%2 == 0
}

func (q *MinMaxPriorityQueue[T]) bubbleUp(index int) {
	if index == 0 {
		return
	}
	parent := (index - 1) / 2
	if isMinLevel(index) {
		if q.compare(q.data[index], q.data[parent]) > 0 {
			q.data[index], q.data[parent] = q.data[parent], q.data[index]
			q.bubbleUpMax(parent)
			return
		}
		q.bubbleUpMin(index)
	} else {
		if q.compare(q.data[index], q.data[parent]) < 0 {
			q.data[index], q.data[parent] = q.data[parent], q.data[index]
			q.bubbleUpMin(parent)
			return
		}
		q.bubbleUpMax(index)
	}
}

func (q *MinMaxPriorityQueue[T]) bubbleUpMin(index int) {
	for index > 2 {
		grandparent := ((index-1)/2 - 1) / 2
		if q.compare(q.data[index], q.data[grandparent]) >= 0 {
			return
		}
		q.data[index], q.data[grandparent] = q.data[grandparent], q.data[index]
		index = grandparent
	}
}

func (q *MinMaxPriorityQueue[T]) bubbleUpMax(index int) {
	for index > 2 {
		grandparent := ((index-1)/2 - 1) / 2
		if q.compare(q.data[index], q.data[grandparent]) <= 0 {
			return
		}
		q.data[index], q.data[grandparent] = q.data[grandparent], q.data[index]
		index = grandparent
	}
}

func (q *MinMaxPriorityQueue[T]) removeAt(index int) T {
	last := len(q.data) - 1
	removed := q.data[index]
	if index < last {
		q.data[index] = q.data[last]
	}
	var zero T
	q.data[last] = zero
	q.data = q.data[:last]
	if last == 0 {
		q.data = nil
	}
	if index < last {
		q.bubbleUp(index)
		q.trickleDown(index)
	}
	return removed
}

func (q *MinMaxPriorityQueue[T]) bestDescendant(index int) (int, bool) {
	best := -1
	grandchild := false
	better := func(candidate int, isGrandchild bool) {
		if best < 0 || q.compare(q.data[candidate], q.data[best]) < 0 == isMinLevel(index) {
			best = candidate
			grandchild = isGrandchild
		}
	}
	for child := 2*index + 1; child < min(len(q.data), 2*index+3); child++ {
		better(child, false)
	}
	for descendant := 4*index + 3; descendant < min(len(q.data), 4*index+7); descendant++ {
		better(descendant, true)
	}
	return best, grandchild
}

func (q *MinMaxPriorityQueue[T]) trickleDown(index int) {
	if isMinLevel(index) {
		q.trickleDownMin(index)
	} else {
		q.trickleDownMax(index)
	}
}

func (q *MinMaxPriorityQueue[T]) trickleDownMin(index int) {
	best, grandchild := q.bestDescendant(index)
	if best < 0 {
		return
	}
	if !grandchild {
		if q.compare(q.data[best], q.data[index]) < 0 {
			q.data[index], q.data[best] = q.data[best], q.data[index]
		}
		return
	}
	if q.compare(q.data[best], q.data[index]) >= 0 {
		return
	}
	q.data[index], q.data[best] = q.data[best], q.data[index]
	parent := (best - 1) / 2
	if q.compare(q.data[best], q.data[parent]) > 0 {
		q.data[best], q.data[parent] = q.data[parent], q.data[best]
	}
	q.trickleDownMin(best)
}

func (q *MinMaxPriorityQueue[T]) trickleDownMax(index int) {
	best, grandchild := q.bestDescendant(index)
	if best < 0 {
		return
	}
	if !grandchild {
		if q.compare(q.data[best], q.data[index]) > 0 {
			q.data[index], q.data[best] = q.data[best], q.data[index]
		}
		return
	}
	if q.compare(q.data[best], q.data[index]) <= 0 {
		return
	}
	q.data[index], q.data[best] = q.data[best], q.data[index]
	parent := (best - 1) / 2
	if q.compare(q.data[best], q.data[parent]) < 0 {
		q.data[best], q.data[parent] = q.data[parent], q.data[best]
	}
	q.trickleDownMax(best)
}

// Len returns the number of elements.
func (q *MinMaxPriorityQueue[T]) Len() int {
	if q == nil {
		return 0
	}
	return len(q.data)
}

// IsEmpty reports whether the queue has no elements.
func (q *MinMaxPriorityQueue[T]) IsEmpty() bool { return q.Len() == 0 }

// Clear removes all elements and releases their storage.
func (q *MinMaxPriorityQueue[T]) Clear() {
	if q == nil {
		return
	}
	var zero T
	for index := range q.data {
		q.data[index] = zero
	}
	q.data = nil
}

// Slice returns a fresh slice in internal breadth-first heap order, not sorted
// order. PopMin or PopMax provide ordered removal.
func (q *MinMaxPriorityQueue[T]) Slice() []T {
	if q == nil {
		return nil
	}
	return append([]T{}, q.data...)
}

// All iterates in internal breadth-first heap order. Do not mutate while
// iterating; early termination propagates immediately.
func (q *MinMaxPriorityQueue[T]) All() iter.Seq[T] {
	return slices.Values(q.Slice())
}

// Clone returns an independent queue with the same comparator, bound, and
// internal layout.
func (q *MinMaxPriorityQueue[T]) Clone() *MinMaxPriorityQueue[T] {
	if q == nil {
		return nil
	}
	return &MinMaxPriorityQueue[T]{data: append([]T(nil), q.data...), compare: q.compare, maximumSize: q.maximumSize}
}

// Equal reports multiset equality using the receiver's comparator. Both queues
// must use consistent comparators; their internal layouts need not match.
func (q *MinMaxPriorityQueue[T]) Equal(other *MinMaxPriorityQueue[T]) bool {
	if q.Len() != other.Len() {
		return false
	}
	if q.Len() == 0 {
		return true
	}
	left, right := q.Slice(), other.Slice()
	slices.SortFunc(left, q.compare)
	slices.SortFunc(right, q.compare)
	for index := range left {
		if q.compare(left[index], right[index]) != 0 {
			return false
		}
	}
	return true
}
