/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_tree

import (
	"iter"
)

// BinaryTreeIter is an old-style (Front/Next/Done/Value) in-order iterator
// over a BinaryTree.  It is more memory efficient than the Walk* functions
// because it manages an explicit stack of only the pending nodes instead of
// using recursion, and unlike Walk* it can be paused and resumed.
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
	cur *BinaryTreeElement[T] // Pointer to the current element.

	// Stack of nodes pending a visit; the top of the stack is next.
	stk []*BinaryTreeElement[T]
}

// -------------------------------------------------------------------------------------------------------

// Front will start at the in-order traversal beginning of the tree for
// iteration over the tree.  The iterator holds pointers into the tree:
// modifying the tree while iterating invalidates it.
func (tt *BinaryTree[T]) Front() (rv *BinaryTreeIter[T]) {
	if tt == nil {
		return &BinaryTreeIter[T]{} // a nil tree iterates as an empty one: Done immediately
	}
	rv = &BinaryTreeIter[T]{}
	// Push the spine of left children so the smallest element is on top.
	for n := tt.root; n != nil; n = n.left {
		rv.stk = append(rv.stk, n)
	}
	rv.Next()
	return
}

// Value returns the current data for this element in the tree, or false if
// the iteration is done.
func (iter *BinaryTreeIter[T]) Value() (item T, found bool) {
	if iter.cur != nil {
		return iter.cur.data, true
	}
	return
}

// Next advances to the next element of the tree in in-order order.
func (iter *BinaryTreeIter[T]) Next() {
	if len(iter.stk) == 0 {
		iter.cur = nil
		return
	}
	n := iter.stk[len(iter.stk)-1]
	iter.stk = iter.stk[:len(iter.stk)-1]
	iter.cur = n
	// Push the spine of left children of the right subtree, if any.
	for r := n.right; r != nil; r = r.left {
		iter.stk = append(iter.stk, r)
	}
}

// Done returns true if the end of the tree has been reached.
func (iter *BinaryTreeIter[T]) Done() bool {
	return iter.cur == nil
}

// -------------------------------------------------------------------------------------------------------

// All returns an iterator that yields every element of the tree in in-order
// (ascending) order.  The tree must not be modified while the returned
// iterator is being consumed — it walks the live nodes.  It is a
// range-over-func iterator:
//
//	for v := range tree.All() {
//		...
//	}
func (tt *BinaryTree[T]) All() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil tree iterates as an empty one
	}
	return func(yield func(T) bool) {
		var stk []*BinaryTreeElement[T]
		n := tt.root
		for n != nil || len(stk) > 0 {
			for n != nil {
				stk = append(stk, n)
				n = n.left
			}
			n = stk[len(stk)-1]
			stk = stk[:len(stk)-1]
			if !yield(n.data) {
				return
			}
			n = n.right
		}
	}
}

// Backward returns an iterator that yields every element of the tree in
// reverse in-order (descending) order.  As with All, the tree must not be
// modified while the iterator is being consumed.  It is a range-over-func
// iterator:
//
//	for v := range tree.Backward() {
//		...
//	}
func (tt *BinaryTree[T]) Backward() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil tree iterates as an empty one
	}
	return func(yield func(T) bool) {
		var stk []*BinaryTreeElement[T]
		n := tt.root
		for n != nil || len(stk) > 0 {
			for n != nil {
				stk = append(stk, n)
				n = n.right
			}
			n = stk[len(stk)-1]
			stk = stk[:len(stk)-1]
			if !yield(n.data) {
				return
			}
			n = n.left
		}
	}
}
