/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package interval_tree

import "iter"

// overlaps reports whether the two closed intervals intersect:
// each one starts at or before the other one ends.  Touching
// endpoints ([1,2] and [2,3]) overlap, at the point 2.
func (tt *IntervalTree[K, V]) overlaps(a, b Interval[K]) bool {
	return tt.cmp(a.Lo, b.Hi) <= 0 && tt.cmp(b.Lo, a.Hi) <= 0
}

// AnyOverlap returns one stored interval that intersects iv, with its
// value, or found=false if no interval in the tree overlaps iv.  It is
// the CLRS INTERVAL-SEARCH: a single descent that at each step goes
// left only when the left subtree's maxHi reaches the query's Lo (a
// lower maxHi means nothing on the left can overlap) and right
// otherwise — at most one node examined per level.  The query interval
// itself is never validated; an inverted probe returns an unspecified
// not-found-or-found answer rather than panicking.
// Complexity is O(log₂ n).
func (tt *IntervalTree[K, V]) AnyOverlap(iv Interval[K]) (found Interval[K], rv V, ok bool) {
	if tt == nil || tt.root == nil || tt.cmp == nil {
		return
	}
	x := tt.root
	for x != nil {
		if tt.overlaps(x.iv, iv) {
			return x.iv, x.val, true
		}
		if x.left != nil && tt.cmp(x.left.maxHi, iv.Lo) >= 0 {
			x = x.left
		} else {
			x = x.right
		}
	}
	return
}

// Overlaps yields every stored interval that intersects iv, with its
// value, ascending by (Lo, Hi).  It is an in-order walk with the same
// pruning AnyOverlap uses: a subtree whose maxHi is below iv.Lo is
// skipped whole.  The walk is over the live tree — the tree must not be
// modified while the iterator is being consumed (the interval_tree_ts
// twin snapshots instead).  iv is never validated; an inverted probe
// yields an unspecified subset rather than panicking.
// Complexity is O(log₂ n + k) where k is the number of matches.
func (tt *IntervalTree[K, V]) Overlaps(iv Interval[K]) iter.Seq2[Interval[K], V] {
	if tt == nil {
		return func(func(Interval[K], V) bool) {}
	}
	return func(yield func(Interval[K], V) bool) {
		if tt.root == nil || tt.cmp == nil {
			return
		}
		var walk func(n *itNode[K, V]) bool
		walk = func(n *itNode[K, V]) bool {
			if n == nil {
				return true
			}
			if n.left != nil && tt.cmp(n.left.maxHi, iv.Lo) >= 0 {
				if !walk(n.left) {
					return false
				}
			}
			if tt.overlaps(n.iv, iv) {
				if !yield(n.iv, n.val) {
					return false
				}
			}
			if n.right != nil && tt.cmp(n.right.maxHi, iv.Lo) >= 0 {
				if !walk(n.right) {
					return false
				}
			}
			return true
		}
		walk(tt.root)
	}
}

// Stab yields every stored interval that contains the point p — the
// stabbing query — with its value, ascending by (Lo, Hi).  It is
// Overlaps over the degenerate interval [p, p].
// Complexity is O(log₂ n + k) where k is the number of matches.
func (tt *IntervalTree[K, V]) Stab(p K) iter.Seq2[Interval[K], V] {
	return tt.Overlaps(Interval[K]{Lo: p, Hi: p})
}
