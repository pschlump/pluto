/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package b_tree

import (
	"iter"
)

// All returns a range-over-func iterator that yields the position and
// value of every element of the tree in ascending (sorted) order:
//
//	for i, item := range tree.All() { ... }
//
// Note that a single-variable range yields the INDEX, not the value; use
// the two-variable form to get the elements.  The tree must not be
// modified while the iterator is being consumed — it walks the live
// nodes.
// Complexity is O(n).
func (tt *BTree[T]) All() iter.Seq2[int, T] {
	if tt == nil {
		return func(func(int, T) bool) {} // a nil tree iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		if tt.root == nil {
			return // an empty tree iterates as an empty one
		}
		pos := 0
		var walk func(n *BTreeNode[T]) bool
		walk = func(n *BTreeNode[T]) bool {
			for i, k := range n.keys {
				if n.children != nil && !walk(n.children[i]) {
					return false
				}
				if !yield(pos, k) {
					return false
				}
				pos++
			}
			if n.children != nil {
				return walk(n.children[len(n.keys)])
			}
			return true
		}
		walk(tt.root)
	}
}

// Backward returns a range-over-func iterator that yields the position
// and value of every element of the tree in descending (reverse sorted)
// order:
//
//	for i, item := range tree.Backward() { ... }
//
// As with All, a single-variable range yields the INDEX.  The tree must
// not be modified while the iterator is being consumed.
// Complexity is O(n).
func (tt *BTree[T]) Backward() iter.Seq2[int, T] {
	if tt == nil {
		return func(func(int, T) bool) {} // a nil tree iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		if tt.root == nil {
			return // an empty tree iterates as an empty one
		}
		pos := 0
		var walk func(n *BTreeNode[T]) bool
		walk = func(n *BTreeNode[T]) bool {
			for i := len(n.keys) - 1; i >= 0; i-- {
				if n.children != nil && !walk(n.children[i+1]) {
					return false
				}
				if !yield(pos, n.keys[i]) {
					return false
				}
				pos++
			}
			if n.children != nil {
				return walk(n.children[0])
			}
			return true
		}
		walk(tt.root)
	}
}
