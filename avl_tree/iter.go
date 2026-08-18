package avl_tree

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"iter"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/stack"
)

// AvlTreeIter is an old-style (Front/Value/Next/Done) iterator that walks the
// tree in in-order (sorted) sequence.
//
// It is more memory efficient than the Walk* functions because it manages an
// explicit stack of at most Depth() nodes instead of traversing recursively.
//
// It is not safe to modify the tree while an iterator is in use.  For new
// code prefer the range-over-func iterators All and Backward.
type AvlTreeIter[T comparable.Comparable] struct {
	cur *AvlTreeElement[T]              // Pointer to the current element.
	stk stack.Stack[*AvlTreeElement[T]] // Stack of nodes with unvisited right subtrees.
}

// -------------------------------------------------------------------------------------------------------

// Front returns an iterator positioned at the first (smallest, leftmost) node
// of the tree in in-order sequence.  On an empty tree the returned iterator
// is immediately Done.
// Complexity is O(log2(n)).
func (tt *AvlTree[T]) Front() (rv *AvlTreeIter[T]) {
	rv = &AvlTreeIter[T]{}
	cur := tt.root
	for cur != nil && cur.left != nil {
		rv.stk.Push(cur)
		cur = cur.left
	}
	rv.cur = cur
	return
}

// Value returns the data of the current element, or nil if the iterator is
// done.
// Complexity is O(1).
func (iter *AvlTreeIter[T]) Value() *T {
	if iter.cur != nil {
		return iter.cur.data
	}
	return nil
}

// Next advances the iterator to the next element in in-order sequence.
// Complexity is O(1) amortized.
func (iter *AvlTreeIter[T]) Next() {
	if iter.cur == nil {
		return
	}
	if n := iter.cur.right; n != nil {
		// Descend to the leftmost node of the right subtree.
		for n.left != nil {
			iter.stk.Push(n)
			n = n.left
		}
		iter.cur = n
		return
	}
	// Pop back up to the nearest unvisited ancestor.
	if p, err := iter.stk.Pop(); err == nil {
		iter.cur = p
	} else {
		iter.cur = nil
	}
}

// Done returns true when the iteration has visited every element.
// Complexity is O(1).
func (iter *AvlTreeIter[T]) Done() bool {
	return iter.cur == nil
}

// -------------------------------------------------------------------------------------------------------

// All returns a Go 1.23 range-over-func iterator that visits every element of
// the tree in in-order (sorted) sequence:
//
//	for item := range tree.All() { ... }
//
// Complexity is O(n).
func (tt *AvlTree[T]) All() iter.Seq[*T] {
	return func(yield func(*T) bool) {
		var walk func(cur *AvlTreeElement[T]) bool
		walk = func(cur *AvlTreeElement[T]) bool {
			if cur == nil {
				return true
			}
			return walk(cur.left) && yield(cur.data) && walk(cur.right)
		}
		walk(tt.root)
	}
}

// Backward returns a Go 1.23 range-over-func iterator that visits every
// element of the tree in reverse in-order (descending) sequence:
//
//	for item := range tree.Backward() { ... }
//
// Complexity is O(n).
func (tt *AvlTree[T]) Backward() iter.Seq[*T] {
	return func(yield func(*T) bool) {
		var walk func(cur *AvlTreeElement[T]) bool
		walk = func(cur *AvlTreeElement[T]) bool {
			if cur == nil {
				return true
			}
			return walk(cur.right) && yield(cur.data) && walk(cur.left)
		}
		walk(tt.root)
	}
}

/* vim: set noai ts=4 sw=4: */
