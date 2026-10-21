/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package geometry

import (
	"math"
	"slices"

	"github.com/pschlump/pluto/g_lib"
)

// ClosestPair is the closest pair of points — Sedgwick's algs4 §9
// ClosestPair.java: the divide-and-conquer algorithm that sorts by x,
// recurses on the halves, and checks a vertical strip of width 2δ
// around the dividing line in y-order, where a geometric packing
// argument bounds each strip point against at most 7 successors —
// O(n log n) where the brute force is O(n²).  Build with
// NewClosestPair; the object is immutable afterwards.
type ClosestPair struct {
	minDist float64       // the pair's distance, +Inf when there is no pair
	p, q    g_lib.Point2D // the pair, zero values when there is no pair
	found   bool          // whether a pair exists (len(pts) >= 2)
}

// NewClosestPair finds the closest pair among pts.  Coincident
// points are a legal pair at distance 0 (the algs4 early exit).
// Fewer than two points leaves no pair: Pair reports found false and
// Distance reports +Inf — the min over an empty set, the identity the
// algorithm inits with.  The input is never modified.
// It panics if any point has a NaN coordinate.
// Complexity is O(n log n).
func NewClosestPair(pts []g_lib.Point2D) *ClosestPair {
	noNaN(pts, "NewClosestPair")

	c := &ClosestPair{minDist: math.Inf(1)}
	n := len(pts)
	if n <= 1 {
		return c
	}

	// Sort by x-coordinate (breaking ties by y-coordinate).
	pointsByX := slices.Clone(pts)
	slices.SortFunc(pointsByX, g_lib.CompareXY)

	// Check for coincident points — distance 0, nothing can beat it.
	for i := 0; i+1 < n; i++ {
		if pointsByX[i] == pointsByX[i+1] {
			c.p, c.q = pointsByX[i], pointsByX[i+1]
			c.minDist = 0
			c.found = true
			return c
		}
	}

	// pointsByY starts as the x-order and is kept y-sorted by the
	// recursion's merge steps, exactly as in ClosestPair.java.
	pointsByY := slices.Clone(pointsByX)

	c.closest(pointsByX, pointsByY, 0, n-1)
	c.found = true // n >= 2 and no coincident pair: the recursion found one
	return c
}

// closest is the recursion of ClosestPair.java over pointsByX[lo..hi]
// (globally x-sorted) with pointsByY[lo..hi] maintained y-sorted by
// the merge on the way back up.  It returns the closest distance
// within the range and records the global pair whenever one is found
// that beats the running minimum.
// Complexity is O(n log n) for the full range.
func (c *ClosestPair) closest(pointsByX, pointsByY []g_lib.Point2D, lo, hi int) float64 {
	best := math.Inf(1)
	if hi <= lo {
		return best
	}
	mid := lo + (hi-lo)/2
	median := pointsByX[mid]

	// Closest pair with both endpoints in each half.
	best = min(best, c.closest(pointsByX, pointsByY, lo, mid))
	best = min(best, c.closest(pointsByX, pointsByY, mid+1, hi))

	// Merge back so that pointsByY[lo..hi] is sorted by
	// y-coordinate — the mergesort merge of the two y-sorted halves.
	mergeByY(pointsByY, lo, mid, hi)

	// The strip: every point within best of the dividing line, in
	// y-order.  A packing argument puts at most 7 points in any
	// best × 2·best rectangle, so the inner loop is O(1) amortized.
	aux := make([]g_lib.Point2D, hi-lo+1)
	m := 0
	for i := lo; i <= hi; i++ {
		if math.Abs(pointsByY[i].X-median.X) < best {
			aux[m] = pointsByY[i]
			m++
		}
	}
	for i := 0; i < m; i++ {
		for j := i + 1; j < m && (aux[j].Y-aux[i].Y) < best; j++ {
			distance := aux[i].DistanceTo(aux[j])
			if distance < best {
				best = distance
				if distance < c.minDist {
					c.minDist = distance
					c.p, c.q = aux[i], aux[j]
				}
			}
		}
	}
	return best
}

// mergeByY is the stable mergesort merge of the y-sorted ranges
// pointsByY[lo..mid] and pointsByY[mid+1..hi] — the one place the
// algs4 original uses the classic three-while merge.
func mergeByY(a []g_lib.Point2D, lo, mid, hi int) {
	aux := make([]g_lib.Point2D, hi-lo+1)
	i, j := lo, mid+1
	for k := 0; k < len(aux); k++ {
		if i > mid {
			aux[k] = a[j]
			j++
		} else if j > hi {
			aux[k] = a[i]
			i++
		} else if g_lib.CompareYX(a[j], a[i]) < 0 {
			aux[k] = a[j]
			j++
		} else {
			aux[k] = a[i]
			i++
		}
	}
	copy(a[lo:hi+1], aux)
}

// Pair returns the closest pair of points.  found is false when the
// constructor saw fewer than two points (p and q are zero values
// then).  The points are returned by value; the pair's order is
// unspecified.
// Complexity is O(1).
func (c *ClosestPair) Pair() (p, q g_lib.Point2D, found bool) {
	if c == nil || !c.found {
		return
	}
	return c.p, c.q, true
}

// Distance returns the distance between the closest pair, or +Inf
// when there is no pair (fewer than two points) — the minimum over an
// empty set, and the answer for a nil or zero-value object.
// Complexity is O(1).
func (c *ClosestPair) Distance() float64 {
	if c == nil || !c.found {
		return math.Inf(1)
	}
	return c.minDist
}
