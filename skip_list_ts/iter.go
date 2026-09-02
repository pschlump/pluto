/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Range-over-func iterators for the thread-safe skip list.  To remain
// race-free both iterators operate on a snapshot of the list data taken
// under the read lock when they are called; modifications of the list
// after that point are not reflected in the iteration, and the lock is
// never held while the loop body runs.  Both support early exit via
// break.

package skip_list_ts

import (
	"iter"
	"slices"
)

// All returns an iterator over the items in a snapshot of the list in
// ascending sequence.  It can be used directly in a for/range loop:
//
//	for v := range list.All() {
//		fmt.Println(v)
//	}
//
// The snapshot is taken when All is called, so it is safe to call other
// list operations — including from inside the loop — and the iterator
// never observes later modifications.
// Complexity is O(n).
func (tt *SkipList[T]) All() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil list iterates as an empty one
	}
	items := tt.toSlice()
	return func(yield func(T) bool) {
		for _, v := range items {
			if !yield(v) {
				return
			}
		}
	}
}

// Backward returns an iterator over the items in a snapshot of the list
// in descending sequence.  It can be used directly in a for/range loop:
//
//	for v := range list.Backward() {
//		fmt.Println(v)
//	}
//
// The snapshot is taken when Backward is called, so it is safe to call
// other list operations — including from inside the loop — and the
// iterator never observes later modifications.
// Complexity is O(n).
func (tt *SkipList[T]) Backward() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil list iterates as an empty one
	}
	items := tt.toSlice()
	return func(yield func(T) bool) {
		for _, item := range slices.Backward(items) {
			if !yield(item) {
				return
			}
		}
	}
}

// rangeEntry pairs an element with its global 0-based rank for the snapshot
// range iterators.
type rangeEntry[T any] struct {
	rank int
	data T
}

// snapshotRange materializes the elements x with lo <= x <= hi together with
// their global 0-based ranks, under the read lock.  A range with lo > hi
// yields nil.  The caller must NOT hold the lock.
// Complexity is O(log₂ n + m).
func (tt *SkipList[T]) snapshotRange(lo, hi T) []rangeEntry[T] {
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if tt.isEmpty() || tt.cmp(lo, hi) > 0 {
		return nil
	}
	// Descend to the first node >= lo, tracking its 0-based rank.
	rank := 0
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil && tt.cmp(cur.forward[i].data, lo) < 0 {
			rank += cur.span[i]
			cur = cur.forward[i]
		}
	}
	var items []rangeEntry[T]
	for cur = cur.forward[0]; cur != nil && tt.cmp(cur.data, hi) <= 0; cur = cur.forward[0] {
		items = append(items, rangeEntry[T]{rank: rank, data: cur.data})
		rank++
	}
	return items
}

// Range returns an iterator over the elements x with lo <= x <= hi, in
// ascending sequence, yielding each element's (index, value) pair — index is
// the element's global 0-based rank in the whole list:
//
//	for i, v := range list.Range(lo, hi) {
//		fmt.Println(i, v)
//	}
//
// The range is materialized under the read lock when Range is called — only
// the m elements of the range are copied, not the whole list — so it is safe
// to call other list operations, including from inside the loop.  A range
// with lo > hi iterates as empty.  Early exit via break is supported.
// Complexity is O(log₂ n + m).
func (tt *SkipList[T]) Range(lo, hi T) iter.Seq2[int, T] {
	if tt == nil {
		return func(func(int, T) bool) {} // a nil list iterates as an empty one
	}
	items := tt.snapshotRange(lo, hi)
	return func(yield func(int, T) bool) {
		for _, e := range items {
			if !yield(e.rank, e.data) {
				return
			}
		}
	}
}

// RangeBackward is Range in descending sequence: it iterates the elements x
// with lo <= x <= hi from largest to smallest.  The indexes are the same
// global 0-based ranks Range assigns — they count down from the rank of the
// largest element in the range to the rank of the smallest:
//
//	for i, v := range list.RangeBackward(lo, hi) {
//		fmt.Println(i, v)
//	}
//
// Like Range it operates on a snapshot taken when it is called (O(m) space
// for the m elements of the range), so it is safe to call other list
// operations, including from inside the loop.
// Complexity is O(log₂ n + m).
func (tt *SkipList[T]) RangeBackward(lo, hi T) iter.Seq2[int, T] {
	if tt == nil {
		return func(func(int, T) bool) {} // a nil list iterates as an empty one
	}
	items := tt.snapshotRange(lo, hi)
	return func(yield func(int, T) bool) {
		for i := range items {
			e := items[len(items)-1-i]
			if !yield(e.rank, e.data) {
				return
			}
		}
	}
}
