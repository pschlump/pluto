/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package kd_tree is the two-dimensional k-d tree of Sedgwick & Wayne,
// Algorithms, 4th ed. — the booksite's "5. Nearest Neighbor Search"
// chapter (KdTree.java, with the brute-force PointSET.java shipped
// alongside as PointSet) over the g_lib Point2D/RectHV geometry value
// types.  A k-d tree is a binary search tree over points that alternates
// the splitting coordinate by depth: the root partitions the plane by x
// (points with smaller x go left, the rest right), its children
// partition by y, and so on, so that every node owns an axis-aligned
// rectangle of the plane — the property that prunes both range and
// nearest-neighbor searches to the rectangles that can matter.
//
//	Insert(p)            — add p (set semantics; duplicates no-op).	O(log₂ n) expected
//	Contains(p)          — exact-match search.					O(log₂ n) expected
//	Nearest(q)           — closest point to q.					O(log₂ n) expected
//	NearestK(q, k)       — the k closest points to q.			O(log₂ n + k log k) expected
//	Range(rect)          — all points inside a rectangle.		O(log₂ n + m) expected
//	All                  — iterate in tree (pre)order.			O(n)
//
// Expected costs hold for randomly ordered insertions; the tree is not
// self-balancing (like binary_tree), and monotone adversarial insertion
// orders — e.g. points marching up a diagonal — legitimately build a
// depth-n chain with O(n) operations (the binary_tree sorted-input
// precedent; asserted in TestDegenerateChain, don't "fix" it).
//
// The package follows the tries' constructor-free contract: the
// splitting rule is inherent in the coordinates — there is no comparison
// function to store — so the zero value of KdTree is an empty tree ready
// to use (including Insert), and NewKdTree is convenience only.  Points
// are stored and returned by value.
//
// A nil *KdTree behaves as an empty tree for every operation except
// Insert.  The package panics in exactly two situations, both on Insert
// and both naming the method: a nil tree (cannot store a point) and a
// NaN coordinate (a NaN point compares false against everything, so it
// could never be found again — the interval_tree inverted-interval
// precedent of rejecting at write what reads could never answer).
// ±Inf coordinates are accepted; queries with NaN coordinates never
// panic and return unspecified not-found/empty answers.
//
// Delete is deliberately absent, as in algs4's KdTree: deletion in a
// k-d tree requires re-partitioning (or rebuilding) the affected
// subtree, and the classic structure ships without it.  Truncate-like
// rebuilds are the caller's job (allocate a fresh tree).
//
// The structure is not safe for concurrent use; the mutex-guarded twin
// kd_tree_ts has the same interface.
package kd_tree

import (
	"math"

	"github.com/pschlump/pluto/g_lib"
)

// wholePlane is the rectangle every inserted point lives in — the k-d
// tree's equivalent of a root that spans the whole key space.  algs4's
// assignment version fixes the unit square because its inputs are
// confined to it; this library takes arbitrary coordinates, so the root
// rectangle is the whole plane.
var wholePlane = g_lib.RectHV{Xmin: math.Inf(-1), Ymin: math.Inf(-1), Xmax: math.Inf(1), Ymax: math.Inf(1)}

// kdNode is one node of the tree: the point and the two subtrees it
// partitions.  Unlike the algs4 node it carries no RectHV — each
// rectangle is derived from the ancestor splits during the descent
// (four floats passed down the recursion instead of four stored per
// node).
type kdNode struct {
	p           g_lib.Point2D
	left, right *kdNode
}

// KdTree is a set of points in the plane organized by alternating
// x/y splits.  The zero value is an empty tree ready to use.
type KdTree struct {
	root *kdNode
	n    int
}

// NewKdTree returns an empty k-d tree — convenience for callers who
// prefer an explicit constructor; the zero value is equally usable.
func NewKdTree() *KdTree {
	return &KdTree{}
}

// Insert adds the point p and reports whether it was newly added.
// Inserting a point already in the tree is a no-op that returns false —
// a k-d tree is a set (the trees' duplicates-replace convention with
// nothing to replace).  It panics on a nil *KdTree or a NaN coordinate.
// Complexity is O(log₂ n) expected, O(n) worst (degenerate tree).
func (t *KdTree) Insert(p g_lib.Point2D) bool {
	if t == nil {
		panic("kd_tree: Insert called on a nil *KdTree (use a KdTree value, not a nil pointer)")
	}
	if math.IsNaN(p.X) || math.IsNaN(p.Y) {
		panic("kd_tree: Insert called with a NaN coordinate (a NaN point can never be found again)")
	}
	added := false
	t.root, added = insertNode(t.root, p, true)
	if added {
		t.n++
	}
	return added
}

// insertNode inserts p below h, splitting on x at vertical levels and y
// at horizontal ones; strict less goes left/bottom, ties and greater go
// right/top.  A duplicate is caught by the == check along its own
// insertion path.
func insertNode(h *kdNode, p g_lib.Point2D, vertical bool) (*kdNode, bool) {
	if h == nil {
		return &kdNode{p: p}, true
	}
	if h.p == p {
		return h, false
	}
	goLeft := p.X < h.p.X
	if !vertical {
		goLeft = p.Y < h.p.Y
	}
	var added bool
	if goLeft {
		h.left, added = insertNode(h.left, p, !vertical)
	} else {
		h.right, added = insertNode(h.right, p, !vertical)
	}
	return h, added
}

// Contains reports whether the point p is in the tree.
// Complexity is O(log₂ n) expected, O(n) worst (degenerate tree).
func (t *KdTree) Contains(p g_lib.Point2D) bool {
	if t == nil {
		return false
	}
	n := t.root
	vertical := true
	for n != nil {
		if n.p == p {
			return true
		}
		if vertical {
			if p.X < n.p.X {
				n = n.left
			} else {
				n = n.right
			}
		} else {
			if p.Y < n.p.Y {
				n = n.left
			} else {
				n = n.right
			}
		}
		vertical = !vertical
	}
	return false
}

// Len returns the number of points in the tree.
// Complexity is O(1).
func (t *KdTree) Len() int {
	if t == nil {
		return 0
	}
	return t.n
}

// IsEmpty reports whether the tree holds no points.
// Complexity is O(1).
func (t *KdTree) IsEmpty() bool {
	return t.Len() == 0
}
