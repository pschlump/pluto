package avl_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"iter"

	"github.com/pschlump/pluto/comparable"
)

// AvlTreeIter is an old-style (Front/Value/Next/Done) iterator that walks the
// tree in in-order (sorted) sequence.
//
// To remain race-free the iterator operates on a snapshot of the tree data
// taken under the read lock when Front is called; concurrent modifications of
// the tree after that point are not reflected in the iteration.  For new code
// prefer the range-over-func iterators All and Backward.
type AvlTreeIter[T comparable.Comparable] struct {
	items []*T // Snapshot of the tree data in in-order sequence.
	pos   int  // Index of the current element.
}

// -------------------------------------------------------------------------------------------------------

// Front returns an iterator positioned at the first (smallest) element of a
// snapshot of the tree in in-order sequence.  On an empty tree the returned
// iterator is immediately Done.
// Complexity is O(n).
func (tt *AvlTree[T]) Front() (rv *AvlTreeIter[T]) {
	return &AvlTreeIter[T]{
		items: tt.toSlice(),
	}
}

// Value returns the data of the current element, or nil if the iterator is
// done.
// Complexity is O(1).
func (iter *AvlTreeIter[T]) Value() *T {
	if iter.pos < len(iter.items) {
		return iter.items[iter.pos]
	}
	return nil
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

// All returns a Go 1.23 range-over-func iterator that visits every element of
// a snapshot of the tree in in-order (sorted) sequence:
//
//	for item := range tree.All() { ... }
//
// The snapshot is taken under the read lock when iteration starts, so the
// loop is race-free and never holds the lock while the body runs.
// Complexity is O(n).
func (tt *AvlTree[T]) All() iter.Seq[*T] {
	return func(yield func(*T) bool) {
		for _, d := range tt.toSlice() {
			if !yield(d) {
				return
			}
		}
	}
}

// Backward returns a Go 1.23 range-over-func iterator that visits every
// element of a snapshot of the tree in reverse in-order (descending)
// sequence:
//
//	for item := range tree.Backward() { ... }
//
// The snapshot is taken under the read lock when iteration starts, so the
// loop is race-free and never holds the lock while the body runs.
// Complexity is O(n).
func (tt *AvlTree[T]) Backward() iter.Seq[*T] {
	return func(yield func(*T) bool) {
		items := tt.toSlice()
		for i := len(items) - 1; i >= 0; i-- {
			if !yield(items[i]) {
				return
			}
		}
	}
}

/* vim: set noai ts=4 sw=4: */
