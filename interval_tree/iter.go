/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package interval_tree

import "iter"

// successor returns the node holding the next-larger interval after n,
// or nil at the end of the tree.  Parent pointers make this O(1) amortized
// with no auxiliary stack.
func successor[K any, V any](n *itNode[K, V]) *itNode[K, V] {
	if n.right != nil {
		return minNode(n.right)
	}
	for n.parent != nil && n == n.parent.right {
		n = n.parent
	}
	return n.parent
}

// predecessor returns the node holding the next-smaller interval before
// n, or nil at the start of the tree.
func predecessor[K any, V any](n *itNode[K, V]) *itNode[K, V] {
	if n.left != nil {
		return maxNode(n.left)
	}
	for n.parent != nil && n == n.parent.left {
		n = n.parent
	}
	return n.parent
}

// All returns an iterator (https://go.dev/ref/spec#RangeOverFunc) over
// the tree's intervals and values in ascending (Lo, Hi) order.  In a
// one-variable range the interval takes the first slot:
//
//	for iv := range tree.All() { ... }        // iv is the Interval
//	for iv, val := range tree.All() { ... }
//
// The walk is over the live tree and needs no extra memory; the tree
// must not be modified while the iterator is being consumed (the
// interval_tree_ts twin returns a snapshot instead).  A nil tree or an
// empty tree yields nothing.
// Complexity is O(n) over the whole tree, O(1) extra space.
func (tt *IntervalTree[K, V]) All() iter.Seq2[Interval[K], V] {
	var empty bool
	if tt == nil {
		empty = true
	}
	return func(yield func(Interval[K], V) bool) {
		if empty {
			return
		}
		for n := minNode(tt.root); n != nil; n = successor(n) {
			if !yield(n.iv, n.val) {
				return
			}
		}
	}
}

// Backward returns an iterator over the tree's intervals and values in
// descending (Lo, Hi) order — All reversed, with the same one-variable
// and live-tree caveats.
// Complexity is O(n) over the whole tree, O(1) extra space.
func (tt *IntervalTree[K, V]) Backward() iter.Seq2[Interval[K], V] {
	var empty bool
	if tt == nil {
		empty = true
	}
	return func(yield func(Interval[K], V) bool) {
		if empty {
			return
		}
		for n := maxNode(tt.root); n != nil; n = predecessor(n) {
			if !yield(n.iv, n.val) {
				return
			}
		}
	}
}
