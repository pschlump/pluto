/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Range-over-func iterators for the skip list.  All walks the level-0
// chain directly; Backward collects the chain into a slice first, because
// skip list nodes have no back pointers.  Both support early exit via
// break.

package skip_list

import (
	"iter"
	"slices"
)

// All returns an iterator over the items in the list in ascending
// sequence.  It can be used directly in a for/range loop:
//
//	for v := range list.All() {
//		fmt.Println(v)
//	}
//
// The list must not be modified while the iterator is being consumed — it
// walks the live nodes.
// Complexity is O(n) time, O(1) space.
func (tt *SkipList[T]) All() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil list iterates as an empty one
	}
	return func(yield func(T) bool) {
		if tt.IsEmpty() {
			return
		}
		for cur := tt.head.forward[0]; cur != nil; cur = cur.forward[0] {
			if !yield(cur.data) {
				return
			}
		}
	}
}

// Backward returns an iterator over the items in the list in descending
// sequence.  It can be used directly in a for/range loop:
//
//	for v := range list.Backward() {
//		fmt.Println(v)
//	}
//
// The items are collected into a slice first, because skip list nodes
// have no back pointers.  The list must not be modified while the
// iterator is being consumed.
// Complexity is O(n) time and O(n) space.
func (tt *SkipList[T]) Backward() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil list iterates as an empty one
	}
	return func(yield func(T) bool) {
		if tt.IsEmpty() {
			return
		}
		var items []T
		for cur := tt.head.forward[0]; cur != nil; cur = cur.forward[0] {
			items = append(items, cur.data)
		}
		for _, item := range slices.Backward(items) {
			if !yield(item) {
				return
			}
		}
	}
}

// firstInRange descends to the first node with data >= lo and returns it
// together with its 0-based rank.  nil is returned when every element sorts
// before lo.  The caller is responsible for the list being non-empty.
func (tt *SkipList[T]) firstInRange(lo T) (*SkipListNode[T], int) {
	rank := 0
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil && tt.cmp(cur.forward[i].data, lo) < 0 {
			rank += cur.span[i]
			cur = cur.forward[i]
		}
	}
	return cur.forward[0], rank
}

// Range returns an iterator over the elements x with lo <= x <= hi, in
// ascending sequence, yielding each element's (index, value) pair — index is
// the element's global 0-based rank in the whole list, exactly the index
// All() would assign:
//
//	for i, v := range list.Range(lo, hi) {
//		fmt.Println(i, v)
//	}
//
// A range with lo > hi iterates as empty.  Like All, the iterator walks the
// live nodes: the list must not be modified while it is being consumed, and
// it supports early exit via break.
// Complexity is O(log₂ n + m) time, O(1) space, where m is the number of
// elements in the range.
func (tt *SkipList[T]) Range(lo, hi T) iter.Seq2[int, T] {
	if tt == nil {
		return func(func(int, T) bool) {} // a nil list iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		if tt.IsEmpty() || tt.cmp(lo, hi) > 0 {
			return
		}
		cur, rank := tt.firstInRange(lo)
		for cur != nil && tt.cmp(cur.data, hi) <= 0 {
			if !yield(rank, cur.data) {
				return
			}
			rank++
			cur = cur.forward[0]
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
// Like Backward, the elements are collected into a slice first, because
// skip list nodes have no back pointers; the slice holds only the m elements
// of the range, not the whole list.  A range with lo > hi iterates as empty.
// The list must not be modified while the iterator is being consumed.
// Complexity is O(log₂ n + m) time and O(m) space.
func (tt *SkipList[T]) RangeBackward(lo, hi T) iter.Seq2[int, T] {
	if tt == nil {
		return func(func(int, T) bool) {} // a nil list iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		if tt.IsEmpty() || tt.cmp(lo, hi) > 0 {
			return
		}
		first, base := tt.firstInRange(lo)
		if first == nil {
			return
		}
		var items []T
		cur := first
		for cur != nil && tt.cmp(cur.data, hi) <= 0 {
			items = append(items, cur.data)
			cur = cur.forward[0]
		}
		for i := range items {
			if !yield(base+len(items)-1-i, items[len(items)-1-i]) {
				return
			}
		}
	}
}
