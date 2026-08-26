/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package avl_tree

import (
	"iter"
)

// AvlTreeIter is an old-style (Front/Value/Next/Done) iterator that walks
// the tree in in-order (sorted) sequence.
//
// It is more memory efficient than the Walk* functions because it manages
// an explicit stack of at most Depth() nodes instead of traversing
// recursively.
//
// The iterator holds pointers into the tree: modifying the tree while
// iterating invalidates it.  For new code prefer the range-over-func
// iterators All and Backward.
type AvlTreeIter[T any] struct {
	cur *AvlTreeElement[T]   // Pointer to the current element.
	stk []*AvlTreeElement[T] // Stack of nodes with unvisited right subtrees.
}

// -------------------------------------------------------------------------------------------------------

// Front returns an iterator positioned at the first (smallest, leftmost)
// node of the tree in in-order sequence.  On an empty tree the returned
// iterator is immediately Done.
// Complexity is O(log₂ n).
func (tt *AvlTree[T]) Front() (rv *AvlTreeIter[T]) {
	if tt == nil {
		return &AvlTreeIter[T]{} // a nil tree iterates as an empty one
	}
	rv = &AvlTreeIter[T]{}
	cur := tt.root
	for cur != nil && cur.left != nil {
		rv.stk = append(rv.stk, cur)
		cur = cur.left
	}
	rv.cur = cur
	return
}

// Value returns the data of the current element, or false if the iterator
// is done.
// Complexity is O(1).
func (iter *AvlTreeIter[T]) Value() (item T, found bool) {
	if iter.cur != nil {
		return iter.cur.data, true
	}
	return
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
			iter.stk = append(iter.stk, n)
			n = n.left
		}
		iter.cur = n
		return
	}
	// Pop back up to the nearest unvisited ancestor.
	if len(iter.stk) > 0 {
		iter.cur = iter.stk[len(iter.stk)-1]
		iter.stk = iter.stk[:len(iter.stk)-1]
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

// All returns a range-over-func iterator that visits every element of the
// tree in in-order (sorted) sequence:
//
//	for item := range tree.All() { ... }
//
// The tree must not be modified while the iterator is being consumed — it
// walks the live nodes.
// Complexity is O(n).
func (tt *AvlTree[T]) All() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil tree iterates as an empty one
	}
	return func(yield func(T) bool) {
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

// Backward returns a range-over-func iterator that visits every element of
// the tree in reverse in-order (descending) sequence:
//
//	for item := range tree.Backward() { ... }
//
// The tree must not be modified while the iterator is being consumed — it
// walks the live nodes.
// Complexity is O(n).
func (tt *AvlTree[T]) Backward() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil tree iterates as an empty one
	}
	return func(yield func(T) bool) {
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
