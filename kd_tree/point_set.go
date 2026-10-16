/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package kd_tree

import (
	"cmp"
	"iter"
	"math"
	"slices"

	"github.com/pschlump/pluto/g_lib"
)

// PointSet is the brute-force point set — algs4 PointSET.java — with
// the same query surface as KdTree answered by linear scans over a
// hash map of the points.  It is the simple fallback for small inputs
// and the reference implementation the KdTree tests cross-check
// against (the algs4 pairing: every nearest-neighbor assignment
// implements both and diffs them).
//
// Costs are the brute-force mirror of the tree's: Insert and Contains
// O(1) expected, Nearest/Range O(n), NearestK O(n log n).  The zero
// value is an empty set ready to use (the map appears on first
// Insert); NewPointSet is convenience only.  Nil tolerance and the
// panic contract match KdTree exactly: reads on a nil *PointSet behave
// as an empty set, and Insert panics on a nil receiver or a NaN
// coordinate, naming the method.
//
// All iterates in map order, which Go randomizes — sort the visited
// points when order matters.
type PointSet struct {
	pts map[g_lib.Point2D]struct{}
}

// NewPointSet returns an empty point set — convenience for callers who
// prefer an explicit constructor; the zero value is equally usable.
func NewPointSet() *PointSet {
	return &PointSet{pts: make(map[g_lib.Point2D]struct{})}
}

// Insert adds the point p and reports whether it was newly added;
// inserting an existing point is a no-op that returns false.
// It panics on a nil *PointSet or a NaN coordinate.
// Complexity is O(1) expected.
func (s *PointSet) Insert(p g_lib.Point2D) bool {
	if s == nil {
		panic("kd_tree: PointSet Insert called on a nil *PointSet (use a PointSet value, not a nil pointer)")
	}
	if math.IsNaN(p.X) || math.IsNaN(p.Y) {
		panic("kd_tree: PointSet Insert called with a NaN coordinate (a NaN point can never be found again)")
	}
	if s.pts == nil {
		s.pts = make(map[g_lib.Point2D]struct{})
	}
	if _, ok := s.pts[p]; ok {
		return false
	}
	s.pts[p] = struct{}{}
	return true
}

// Contains reports whether the point p is in the set.
// Complexity is O(1) expected.
func (s *PointSet) Contains(p g_lib.Point2D) bool {
	if s == nil {
		return false
	}
	_, ok := s.pts[p]
	return ok
}

// Len returns the number of points in the set.
// Complexity is O(1).
func (s *PointSet) Len() int {
	if s == nil {
		return 0
	}
	return len(s.pts)
}

// IsEmpty reports whether the set holds no points.
// Complexity is O(1).
func (s *PointSet) IsEmpty() bool {
	return s.Len() == 0
}

// Nearest returns the point of the set closest to q by squared
// Euclidean distance; not-found on an empty or nil set.  Among exactly
// equidistant points which one is returned is unspecified (map order).
// Complexity is O(n).
func (s *PointSet) Nearest(q g_lib.Point2D) (g_lib.Point2D, bool) {
	if s == nil || len(s.pts) == 0 {
		return g_lib.Point2D{}, false
	}
	var champ g_lib.Point2D
	champD2 := math.Inf(1)
	for p := range s.pts {
		if d2 := q.DistanceSquaredTo(p); d2 < champD2 {
			champ, champD2 = p, d2
		}
	}
	return champ, true
}

// nearestLess orders tagged points by (squared distance, X, Y) — the
// same deterministic order NearestK's output is pinned to.
func nearestLess(a, b kdBest) int {
	if c := cmp.Compare(a.d2, b.d2); c != 0 {
		return c
	}
	if c := cmp.Compare(a.p.X, b.p.X); c != 0 {
		return c
	}
	return cmp.Compare(a.p.Y, b.p.Y)
}

// NearestK returns the k points of the set closest to q, in ascending
// order of squared distance with (X, Y) tie-breaking — the same order
// contract as KdTree.NearestK.  When the set holds fewer than k points
// all of them are returned; k ≤ 0 returns nil.
// Complexity is O(n log n).
func (s *PointSet) NearestK(q g_lib.Point2D, k int) []g_lib.Point2D {
	if s == nil || k <= 0 {
		return nil
	}
	entries := make([]kdBest, 0, len(s.pts))
	for p := range s.pts {
		entries = append(entries, kdBest{p: p, d2: q.DistanceSquaredTo(p)})
	}
	if len(entries) == 0 {
		return nil
	}
	slices.SortFunc(entries, nearestLess)
	if k > len(entries) {
		k = len(entries)
	}
	out := make([]g_lib.Point2D, k)
	for i, e := range entries[:k] {
		out[i] = e.p
	}
	return out
}

// Range returns the points inside the closed rectangle r (boundary
// included), in unspecified order.  A rectangle that matches nothing
// returns nil.
// Complexity is O(n).
func (s *PointSet) Range(r g_lib.RectHV) []g_lib.Point2D {
	if s == nil {
		return nil
	}
	out := make([]g_lib.Point2D, 0, 8)
	for p := range s.pts {
		if r.Contains(p) {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// All returns a range-over-func iterator over every point of the set,
// in map order (randomized by the Go runtime — sort when order
// matters).  A nil *PointSet iterates as an empty one.
// Complexity is O(n) for a full traversal.
func (s *PointSet) All() iter.Seq[g_lib.Point2D] {
	return func(yield func(g_lib.Point2D) bool) {
		if s == nil {
			return
		}
		for p := range s.pts {
			if !yield(p) {
				return
			}
		}
	}
}
