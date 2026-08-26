/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package avl_tree_ts

import (
	"iter"
	"slices"
)

// AvlTreeIter is an old-style (Front/Value/Next/Done) iterator that walks
// the tree in in-order (sorted) sequence.
//
// The iterator operates on a snapshot of the tree taken when Front is
// called, so it is safe to use concurrently with other tree operations and
// it does not observe later modifications of the tree.
//
// For new code prefer the range-over-func iterators All and Backward.
type AvlTreeIter[T any] struct {
	items []T // snapshot of the tree data in in-order order
	pos   int // index of the current element in items
}

// -------------------------------------------------------------------------------------------------------

// Front will start at the in-order traversal beginning of the tree for
// iteration over the tree.  The snapshot is taken under the read lock, so
// Front is safe to call concurrently with any tree operation.
// Complexity is O(n).
func (tt *AvlTree[T]) Front() (rv *AvlTreeIter[T]) {
	if tt == nil {
		return &AvlTreeIter[T]{} // a nil tree iterates as an empty one
	}
	items, _ := tt.snapshot()
	return &AvlTreeIter[T]{items: items}
}

// Value returns the data of the current element, or false if the iterator
// is done.
// Complexity is O(1).
func (iter *AvlTreeIter[T]) Value() (item T, found bool) {
	if iter.pos < len(iter.items) {
		return iter.items[iter.pos], true
	}
	return
}

// Next advances the iterator to the next element in in-order sequence.
// Complexity is O(1).
func (iter *AvlTreeIter[T]) Next() {
	if iter.pos < len(iter.items) {
		iter.pos++
	}
}

// Done returns true when the iteration has visited every element.
// Complexity is O(1).
func (iter *AvlTreeIter[T]) Done() bool {
	return iter.pos >= len(iter.items)
}

// -------------------------------------------------------------------------------------------------------

// All returns an iterator that yields every element of the tree in in-order
// (sorted) sequence.  It is a range-over-func iterator:
//
//	for v := range tree.All() {
//		...
//	}
//
// The iterator operates on a snapshot of the tree taken when All is
// called, so it is safe to call other tree operations — including from
// inside the loop — and it never observes later modifications.
// Complexity is O(n).
func (tt *AvlTree[T]) All() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil tree iterates as an empty one
	}
	items, _ := tt.snapshot()
	return func(yield func(T) bool) {
		for _, v := range items {
			if !yield(v) {
				return
			}
		}
	}
}

// Backward returns an iterator that yields every element of the tree in
// reverse in-order (descending) sequence.  It is a range-over-func
// iterator:
//
//	for v := range tree.Backward() {
//		...
//	}
//
// The iterator operates on a snapshot of the tree taken when Backward is
// called, so it is safe to call other tree operations — including from
// inside the loop — and it never observes later modifications.
// Complexity is O(n).
func (tt *AvlTree[T]) Backward() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil tree iterates as an empty one
	}
	items, _ := tt.snapshot()
	return func(yield func(T) bool) {
		for _, item := range slices.Backward(items) {
			if !yield(item) {
				return
			}
		}
	}
}
