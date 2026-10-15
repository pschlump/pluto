/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package interval_tree_ts

// overlaps reports whether the two closed intervals intersect: each
// one starts at or before the other one ends.  Touching endpoints
// ([1,2] and [2,3]) overlap, at the point 2.
func (tt *IntervalTree[K, V]) overlaps(a, b Interval[K]) bool {
	return tt.cmp(a.Lo, b.Hi) <= 0 && tt.cmp(b.Lo, a.Hi) <= 0
}

// AnyOverlap returns one stored interval that intersects iv, with its
// value, or found=false if no interval in the tree overlaps iv — the
// CLRS INTERVAL-SEARCH, a single pruned descent.  For an atomic
// overlap-check-then-book use Lock + NlAnyOverlap + NlInsert.
// Complexity is O(log₂ n).
func (tt *IntervalTree[K, V]) AnyOverlap(iv Interval[K]) (found Interval[K], rv V, ok bool) {
	if tt == nil || tt.cmp == nil {
		return
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.nlAnyOverlap(iv)
}

// NlAnyOverlap is the no-lock form of AnyOverlap, for compound
// operations run under Lock.
func (tt *IntervalTree[K, V]) NlAnyOverlap(iv Interval[K]) (found Interval[K], rv V, ok bool) {
	if tt == nil || tt.cmp == nil {
		return
	}
	return tt.nlAnyOverlap(iv)
}

// nlAnyOverlap is the lock-free search body; the caller holds a lock.
func (tt *IntervalTree[K, V]) nlAnyOverlap(iv Interval[K]) (found Interval[K], rv V, ok bool) {
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

// nlOverlaps collects the pairs intersecting iv, ascending by (Lo, Hi);
// the caller holds a lock.  Shared by Overlaps and Stab snapshots.
func (tt *IntervalTree[K, V]) nlOverlaps(iv Interval[K]) []ivPair[K, V] {
	var out []ivPair[K, V]
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
			out = append(out, ivPair[K, V]{n.iv, n.val})
		}
		if n.right != nil && tt.cmp(n.right.maxHi, iv.Lo) >= 0 {
			if !walk(n.right) {
				return false
			}
		}
		return true
	}
	walk(tt.root)
	return out
}

// Overlaps returns an iterator over every stored interval that
// intersects iv, with its value, ascending by (Lo, Hi).  The iteration
// is over an EAGER SNAPSHOT of just the matches, taken under the read
// lock at call time — unlike the plain package's live walk it is safe
// to mutate the tree from inside the loop, at O(k) snapshot cost.
// Complexity is O(log₂ n + k) expected for the snapshot (see the plain
// package's README for the pruning caveat), then O(1) per element.
func (tt *IntervalTree[K, V]) Overlaps(iv Interval[K]) func(yield func(Interval[K], V) bool) {
	if tt == nil || tt.cmp == nil {
		return emptySeq[K, V]
	}
	tt.lock.RLock()
	snap := tt.nlOverlaps(iv)
	tt.lock.RUnlock()
	return pairSeq(snap)
}

// Stab returns an iterator over every stored interval that contains the
// point p — the stabbing query — with its value, ascending by (Lo, Hi).
// It is Overlaps over the degenerate interval [p, p], with the same
// snapshot semantics.
// Complexity is O(log₂ n + k) expected for the snapshot.
func (tt *IntervalTree[K, V]) Stab(p K) func(yield func(Interval[K], V) bool) {
	return tt.Overlaps(Interval[K]{Lo: p, Hi: p})
}
