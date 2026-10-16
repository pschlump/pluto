/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package kd_tree_ts implements the two-dimensional k-d tree that is
// safe for concurrent use.  It is the thread-safe twin of
// github.com/pschlump/pluto/kd_tree — the algs4 KdTree of orthogonal
// range search and nearest-neighbor queries — with the identical API
// guarded by a sync.RWMutex, plus the Lock and Unlock pair and the
// Nl-prefixed (no-lock) methods for compound operations.  The Point2D
// and RectHV types are aliases of the g_lib package's (as in the plain
// package), so switching between the twins is an import change.
//
// The twin guards a plain kd_tree.KdTree value behind one RWMutex —
// the stream_ts/hash_tab_bt_ts composition pattern rather than a
// re-implementation: the k-d tree stores no comparison function (the
// splitting rule is inherent in the coordinates, the constructor-free
// contract), so there is nothing to adopt, snapshot or duplicate, and
// the Nl methods are simply the plain package's methods.
//
// Concurrency model:
//
// Reads (Len, IsEmpty, Contains, Nearest, NearestK, Range) take the
// read lock and release it before returning, so they run in parallel
// with each other.  Writes (Insert) take the write lock.
//
// All takes an eager snapshot under the read lock: the points are
// collected into a slice while the lock is held and yielded after it is
// released — unlike the plain package's live walk, it is safe to mutate
// the tree — even from inside the loop — and later modifications are
// never visible.  Range and NearestK already return fresh slices, so
// they need no separate snapshot discipline.
//
// The zero value of KdTree is an empty tree ready to use (including
// Insert) — no constructor.  A nil *KdTree behaves as an empty tree for
// every operation except Insert, which panics naming the method; the
// nil guards run before any lock acquisition.  These are the package's
// only panics, with one addition over the house rule: Insert with a NaN
// coordinate panics as in the plain package (a NaN point can never be
// found again).
//
// The brute-force PointSet is deliberately NOT mirrored here: a locked
// linear scan is just a worse hash set, and the twin exists for the
// search structure.  Concurrent callers of PointSet wrap it themselves
// or use this tree.
//
// See the kd_tree package documentation for the data-structure
// contracts (set semantics, no Delete, expected O(log₂ n) on random
// insert order, degenerate-chain worst case) — this twin changes only
// the concurrency.
//
// Run the tests with -race.
package kd_tree_ts

import (
	"iter"
	"math"
	"slices"
	"sync"

	"github.com/pschlump/pluto/g_lib"
	"github.com/pschlump/pluto/kd_tree"
)

// Point2D and RectHV are the g_lib geometry types, aliased so that code
// written against either twin compiles against the other.
type (
	Point2D = g_lib.Point2D
	RectHV  = g_lib.RectHV
)

// NewKdTree returns an empty k-d tree — convenience parity with the
// plain package; the zero value is equally usable.
func NewKdTree() *KdTree {
	return &KdTree{}
}

// KdTree is a set of points in the plane organized by alternating x/y
// splits, safe for concurrent use.  It is the kd_tree package's KdTree
// behind one sync.RWMutex (a plain value guarded by this structure's
// lock — no locks inside the borrowed structure).
//
// The zero value is an empty tree ready to use.
type KdTree struct {
	inner kd_tree.KdTree
	lock  sync.RWMutex
}

// Lock takes the write lock and is the entry point for compound
// operations: Lock, then the Nl-prefixed methods, then Unlock.  Calling
// a regular (locking) method while the lock is held deadlocks.  A no-op
// on a nil *KdTree.
func (t *KdTree) Lock() {
	if t == nil {
		return
	}
	t.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  A no-op on a nil
// *KdTree.
func (t *KdTree) Unlock() {
	if t == nil {
		return
	}
	t.lock.Unlock()
}

// Insert adds the point p and reports whether it was newly added — see
// kd_tree.KdTree.Insert for the set semantics.  It panics on a nil
// *KdTree or a NaN coordinate (before any lock acquisition).
// Complexity is O(log₂ n) expected, O(n) worst (degenerate tree).
func (t *KdTree) Insert(p Point2D) bool {
	if t == nil {
		panic("kd_tree_ts: Insert called on a nil *KdTree (use a KdTree value, not a nil pointer)")
	}
	if math.IsNaN(p.X) || math.IsNaN(p.Y) {
		panic("kd_tree_ts: Insert called with a NaN coordinate (a NaN point can never be found again)")
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.NlInsert(p)
}

// NlInsert is Insert without locking; call it only while holding Lock.
// Complexity is O(log₂ n) expected.
func (t *KdTree) NlInsert(p Point2D) bool {
	return t.inner.Insert(p)
}

// IsEmpty reports whether the tree holds no points.
// Complexity is O(1).
func (t *KdTree) IsEmpty() bool {
	return t.Len() == 0
}

// NlIsEmpty is IsEmpty without locking; call it only while holding Lock.
// Complexity is O(1).
func (t *KdTree) NlIsEmpty() bool {
	return t.inner.IsEmpty()
}

// Len returns the number of points in the tree.
// Complexity is O(1).
func (t *KdTree) Len() int {
	if t == nil {
		return 0
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.NlLen()
}

// NlLen is Len without locking; call it only while holding Lock.
// Complexity is O(1).
func (t *KdTree) NlLen() int {
	return t.inner.Len()
}

// Contains reports whether the point p is in the tree.
// Complexity is O(log₂ n) expected, O(n) worst (degenerate tree).
func (t *KdTree) Contains(p Point2D) bool {
	if t == nil {
		return false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.NlContains(p)
}

// NlContains is Contains without locking; call it only while holding Lock.
// Complexity is O(log₂ n) expected.
func (t *KdTree) NlContains(p Point2D) bool {
	return t.inner.Contains(p)
}

// Nearest returns the point of the tree closest to q — see
// kd_tree.KdTree.Nearest for the tie contract.
// Complexity is O(log₂ n) expected, O(n) worst.
func (t *KdTree) Nearest(q Point2D) (Point2D, bool) {
	if t == nil {
		return Point2D{}, false
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.NlNearest(q)
}

// NlNearest is Nearest without locking; call it only while holding Lock.
// Complexity is O(log₂ n) expected.
func (t *KdTree) NlNearest(q Point2D) (Point2D, bool) {
	return t.inner.Nearest(q)
}

// NearestK returns the k closest points to q in ascending distance
// order — see kd_tree.KdTree.NearestK for the order contract.
// Complexity is O(log₂ n + k log k) expected.
func (t *KdTree) NearestK(q Point2D, k int) []Point2D {
	if t == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.NlNearestK(q, k)
}

// NlNearestK is NearestK without locking; call it only while holding Lock.
// Complexity is O(log₂ n + k log k) expected.
func (t *KdTree) NlNearestK(q Point2D, k int) []Point2D {
	return t.inner.NearestK(q, k)
}

// Range returns the points inside the closed rectangle r, in tree
// order (not sorted) — see kd_tree.KdTree.Range.
// Complexity is O(log₂ n + m) expected.
func (t *KdTree) Range(r RectHV) []Point2D {
	if t == nil {
		return nil
	}
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.NlRange(r)
}

// NlRange is Range without locking; call it only while holding Lock.
// Complexity is O(log₂ n + m) expected.
func (t *KdTree) NlRange(r RectHV) []Point2D {
	return t.inner.Range(r)
}

// All returns a range-over-func iterator over every point of the tree
// in tree (pre)order — see kd_tree.KdTree.All.
//
// Unlike the plain package's live walk, the result is an eager snapshot
// taken under the read lock at call time: it is safe to mutate the tree
// — even from inside the loop — and later modifications are never
// visible.
// Complexity is O(n) space and time for the snapshot.
func (t *KdTree) All() iter.Seq[Point2D] {
	if t == nil {
		return func(func(Point2D) bool) {} // a nil tree iterates as an empty one
	}
	t.lock.RLock()
	snapshot := slices.Collect(t.inner.All())
	t.lock.RUnlock()
	return slices.Values(snapshot)
}
