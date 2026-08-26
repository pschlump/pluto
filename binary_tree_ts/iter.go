/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_tree_ts

import (
	"iter"
	"slices"
)

// BinaryTreeIter is an old-style (Front/Next/Done/Value) in-order iterator
// over a BinaryTree.
//
// The iterator operates on a snapshot of the tree taken when Front is
// called, so it is safe to use concurrently with other tree operations and
// it does not observe later modifications of the tree.
//
// The main benefit is that it can be used to make cleaner code:
//
//	for it := tree.Front(); !it.Done(); it.Next() {
//		v, found := it.Value()
//		...
//	}
//
// For most new code the All and Backward range-over-func iterators are a
// simpler choice.
type BinaryTreeIter[T any] struct {
	items []T // snapshot of the tree data in in-order order
	pos   int // index of the current element in items
}

// -------------------------------------------------------------------------------------------------------

// Front will start at the in-order traversal beginning of the tree for
// iteration over the tree.  The snapshot is taken under the read lock, so
// Front is safe to call concurrently with any tree operation.
func (tt *BinaryTree[T]) Front() (rv *BinaryTreeIter[T]) {
	if tt == nil {
		return &BinaryTreeIter[T]{} // a nil tree iterates as an empty one
	}
	rv = &BinaryTreeIter[T]{
		items: tt.snapshotInOrder(),
	}
	return
}

// Value returns the current data for this element in the tree, or false if
// the iteration is done.
func (iter *BinaryTreeIter[T]) Value() (item T, found bool) {
	if iter.pos < len(iter.items) {
		return iter.items[iter.pos], true
	}
	return
}

// Next advances to the next element of the tree in in-order order.
func (iter *BinaryTreeIter[T]) Next() {
	if iter.pos < len(iter.items) {
		iter.pos++
	}
}

// Done returns true if the end of the tree has been reached.
func (iter *BinaryTreeIter[T]) Done() bool {
	return iter.pos >= len(iter.items)
}

// -------------------------------------------------------------------------------------------------------

// snapshotInOrder returns the data of every element in in-order order.
// The caller must NOT hold the lock.
func (tt *BinaryTree[T]) snapshotInOrder() []T {
	tt.lock.RLock()
	defer tt.lock.RUnlock()

	items := make([]T, 0, tt.length)
	var stk []*BinaryTreeElement[T]
	n := tt.root
	for n != nil || len(stk) > 0 {
		for n != nil {
			stk = append(stk, n)
			n = n.left
		}
		n = stk[len(stk)-1]
		stk = stk[:len(stk)-1]
		items = append(items, n.data)
		n = n.right
	}
	return items
}

// All returns an iterator that yields every element of the tree in in-order
// (ascending) order.  It is a range-over-func iterator:
//
//	for v := range tree.All() {
//		...
//	}
//
// The iterator operates on a snapshot of the tree taken when All is
// called, so it is safe to call other tree operations — including from
// inside the loop — and it never observes later modifications.
func (tt *BinaryTree[T]) All() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil tree iterates as an empty one
	}
	items := tt.snapshotInOrder()
	return func(yield func(T) bool) {
		for _, v := range items {
			if !yield(v) {
				return
			}
		}
	}
}

// Backward returns an iterator that yields every element of the tree in
// reverse in-order (descending) order.  It is a range-over-func iterator:
//
//	for v := range tree.Backward() {
//		...
//	}
//
// The iterator operates on a snapshot of the tree taken when Backward is
// called, so it is safe to call other tree operations — including from
// inside the loop — and it never observes later modifications.
func (tt *BinaryTree[T]) Backward() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil tree iterates as an empty one
	}
	items := tt.snapshotInOrder()
	return func(yield func(T) bool) {
		for _, item := range slices.Backward(items) {
			if !yield(item) {
				return
			}
		}
	}
}
