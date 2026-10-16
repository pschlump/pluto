/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package kd_tree

import (
	"cmp"
	"math"
	"slices"

	"github.com/pschlump/pluto/g_lib"
	"github.com/pschlump/pluto/heap"
)

// splitRects partitions the rectangle rect at node n's splitting
// coordinate into the left/bottom and right/top child rectangles — the
// two halves of the plane the node hands to its subtrees.
func splitRects(n *kdNode, rect g_lib.RectHV, vertical bool) (lo, hi g_lib.RectHV) {
	if vertical {
		lo = g_lib.RectHV{Xmin: rect.Xmin, Ymin: rect.Ymin, Xmax: math.Min(rect.Xmax, n.p.X), Ymax: rect.Ymax}
		hi = g_lib.RectHV{Xmin: math.Max(rect.Xmin, n.p.X), Ymin: rect.Ymin, Xmax: rect.Xmax, Ymax: rect.Ymax}
		return lo, hi
	}
	lo = g_lib.RectHV{Xmin: rect.Xmin, Ymin: rect.Ymin, Xmax: rect.Xmax, Ymax: math.Min(rect.Ymax, n.p.Y)}
	hi = g_lib.RectHV{Xmin: rect.Xmin, Ymin: math.Max(rect.Ymin, n.p.Y), Xmax: rect.Xmax, Ymax: rect.Ymax}
	return lo, hi
}

// Nearest returns the point of the tree closest to q, measuring squared
// Euclidean distance.  The search descends toward q's side of each
// split first, then explores the other side only when its rectangle
// could still hold something closer than the champion found so far (the
// oriented-distance prune of the algs4 KdTree).  An empty or nil tree
// reports not-found; a query that is itself in the tree returns it.
//
// When several points are exactly equidistant from q, which one is
// returned is unspecified (the first one the descent meets wins).
// Complexity is O(log₂ n) expected, O(n) worst (degenerate tree).
func (t *KdTree) Nearest(q g_lib.Point2D) (g_lib.Point2D, bool) {
	if t == nil || t.root == nil {
		return g_lib.Point2D{}, false
	}
	champ := t.root.p
	champD2 := q.DistanceSquaredTo(t.root.p)
	var visit func(n *kdNode, rect g_lib.RectHV, vertical bool)
	visit = func(n *kdNode, rect g_lib.RectHV, vertical bool) {
		if n == nil {
			return
		}
		if rect.DistanceSquaredTo(q) > champD2 {
			return // this whole rectangle is farther than the champion
		}
		if d2 := q.DistanceSquaredTo(n.p); d2 < champD2 {
			champ, champD2 = n.p, d2
		}
		goLeft := q.X < n.p.X
		if !vertical {
			goLeft = q.Y < n.p.Y
		}
		lo, hi := splitRects(n, rect, vertical)
		if goLeft {
			visit(n.left, lo, !vertical)  // near side first: better champions
			visit(n.right, hi, !vertical) // prune against the tightened champion
		} else {
			visit(n.right, hi, !vertical)
			visit(n.left, lo, !vertical)
		}
	}
	visit(t.root, wholePlane, true)
	return champ, true
}

// kdBest is a point tagged with its squared distance from a query — the
// satellite payload of NearestK's bounded max-heap.
type kdBest struct {
	p  g_lib.Point2D
	d2 float64
}

// maxByDistance orders kdBest entries as a max-heap on squared distance
// (a reversed comparator, the heap package's max-heap idiom), so the
// heap's Peek is the worst of the k best found so far.
func maxByDistance(a, b kdBest) int {
	return cmp.Compare(b.d2, a.d2)
}

// NearestK returns the k points of the tree closest to q, in ascending
// order of squared distance (ties broken by X then Y, so the output is
// deterministic given the tree's contents).  When the tree holds fewer
// than k points all of them are returned; k ≤ 0 returns nil.
//
// The search is Nearest with the single champion replaced by a bounded
// max-heap of the k best (composed from the pluto heap package): a
// subtree is pruned only once k candidates are held and its rectangle
// lies strictly farther than the worst of them.  When more than k
// points are exactly equidistant, which of them make the cut is
// unspecified.
// Complexity is O(log₂ n + k log k) expected, O(n log k) worst.
func (t *KdTree) NearestK(q g_lib.Point2D, k int) []g_lib.Point2D {
	if t == nil || t.root == nil || k <= 0 {
		return nil
	}
	best := heap.NewHeapFunc(maxByDistance)
	var visit func(n *kdNode, rect g_lib.RectHV, vertical bool)
	visit = func(n *kdNode, rect g_lib.RectHV, vertical bool) {
		if n == nil {
			return
		}
		full := best.Len() == k
		if full {
			if worst, ok := best.Peek(); ok && rect.DistanceSquaredTo(q) > worst.d2 {
				return
			}
		}
		d2 := q.DistanceSquaredTo(n.p)
		if !full {
			best.Push(kdBest{p: n.p, d2: d2})
		} else if worst, ok := best.Peek(); ok && d2 < worst.d2 {
			best.Pop()
			best.Push(kdBest{p: n.p, d2: d2})
		}
		goLeft := q.X < n.p.X
		if !vertical {
			goLeft = q.Y < n.p.Y
		}
		lo, hi := splitRects(n, rect, vertical)
		if goLeft {
			visit(n.left, lo, !vertical)
			visit(n.right, hi, !vertical)
		} else {
			visit(n.right, hi, !vertical)
			visit(n.left, lo, !vertical)
		}
	}
	visit(t.root, wholePlane, true)

	entries := make([]kdBest, 0, best.Len())
	for {
		e, ok := best.Pop()
		if !ok {
			break
		}
		entries = append(entries, e)
	}
	// The heap pops worst-first; a (d2, X, Y) sort yields the ascending
	// order with the tie order pinned.
	slices.SortFunc(entries, func(a, b kdBest) int {
		if c := cmp.Compare(a.d2, b.d2); c != 0 {
			return c
		}
		if c := cmp.Compare(a.p.X, b.p.X); c != 0 {
			return c
		}
		return cmp.Compare(a.p.Y, b.p.Y)
	})
	out := make([]g_lib.Point2D, len(entries))
	for i, e := range entries {
		out[i] = e.p
	}
	return out
}

// Range returns the points inside the closed rectangle r (boundary
// included), in tree order — not sorted; sort the slice if order
// matters.  Subtrees whose rectangle does not intersect r are never
// visited.  An empty or nil tree, or a rectangle that matches nothing,
// returns nil.
// Complexity is O(log₂ n + m) expected, O(n) worst.
func (t *KdTree) Range(r g_lib.RectHV) []g_lib.Point2D {
	if t == nil || t.root == nil {
		return nil
	}
	out := make([]g_lib.Point2D, 0, 8)
	var visit func(n *kdNode, rect g_lib.RectHV, vertical bool)
	visit = func(n *kdNode, rect g_lib.RectHV, vertical bool) {
		if n == nil || !r.Intersects(rect) {
			return
		}
		if r.Contains(n.p) {
			out = append(out, n.p)
		}
		lo, hi := splitRects(n, rect, vertical)
		visit(n.left, lo, !vertical)
		visit(n.right, hi, !vertical)
	}
	visit(t.root, wholePlane, true)
	if len(out) == 0 {
		return nil
	}
	return out
}
