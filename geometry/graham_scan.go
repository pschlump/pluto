/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package geometry

import (
	"slices"

	"github.com/pschlump/pluto/g_lib"
)

// GrahamScan is the convex hull of a set of points — Sedgwick's algs4
// §9 GrahamScan.java: sort by polar angle around the base point (the
// lowest, then leftmost), then walk the sorted points keeping only
// left (counterclockwise) turns on a stack.  The hull that remains is
// the corner vertices in counterclockwise order.  Build with
// NewGrahamScan; the object is immutable afterwards.
type GrahamScan struct {
	hull []g_lib.Point2D // corner vertices, ccw, hull[0] the lowest point
}

// NewGrahamScan computes the convex hull of pts — the smallest convex
// polygon containing every point, reported corner-only: points that
// lie ON a hull edge are not vertices of the result (the algs4
// convention; Hull has no three collinear vertices for any input with
// a two-dimensional hull).
//
// Degenerate inputs: an empty input gives an empty hull; a single
// point gives that point; a fully collinear input gives its two
// endpoints (lowest and highest in the y-then-x order); duplicate
// points collapse.  The input is never modified — the scan works on a
// private copy.
// It panics if any point has a NaN coordinate.
// Complexity is O(n log n) for the sort, O(n) for the scan.
func NewGrahamScan(pts []g_lib.Point2D) *GrahamScan {
	noNaN(pts, "NewGrahamScan")

	g := &GrahamScan{}
	n := len(pts)
	if n == 0 {
		return g
	}
	points := slices.Clone(pts)

	// Preprocess so that points[0] has the lowest y-coordinate, ties
	// broken by the lowest x-coordinate.
	slices.SortFunc(points, g_lib.CompareYX)

	// Sort by polar angle with respect to the base point points[0].
	//
	// STABLE sort, load-bearing: points collinear with the base
	// compare equal under polar order, and the all-collinear input
	// relies on those ties keeping their y order — the farthest point
	// of the run must land at points[n-1] (Java's Arrays.sort is a
	// stable mergesort, so the original has this guarantee for free;
	// Go's slices.SortFunc is NOT stable and produces a wrong
	// degenerate hull here).
	slices.SortStableFunc(points[1:], points[0].PolarOrder())

	g.hull = append(g.hull, points[0]) // points[0] is the first extreme point

	// Find k1, the index of the first point not equal to points[0].
	k1 := 1
	for ; k1 < n && points[k1] == points[0]; k1++ {
	}
	if k1 == n {
		return g // all points equal
	}

	// Find k2, the index of the first point not collinear with
	// points[0] and points[k1]; the last of the collinear run,
	// points[k2-1], is the second extreme point.
	k2 := k1 + 1
	for ; k2 < n && g_lib.CCW(points[0], points[k1], points[k2]) == 0; k2++ {
	}
	g.hull = append(g.hull, points[k2-1])

	// The Graham scan proper: pop while the top of the stack does not
	// make a counterclockwise turn with the new point — clockwise and
	// collinear turns both pop, which is what keeps the hull
	// corner-only.  The base point is never popped in exact arithmetic
	// (every scan point turns counterclockwise with the first-angle
	// group's survivor); the len >= 2 guard makes that a checked
	// invariant rather than an assumption, because nearly-collinear
	// float noise can invert CCW's sign and cascade the pops below
	// the base point — where the algs4 original underflows its stack
	// and throws.  The package's contract has no panic for that
	// input, only the NaN class.
	for i := k2; i < n; i++ {
		top := g.hull[len(g.hull)-1]
		g.hull = g.hull[:len(g.hull)-1]
		for len(g.hull) >= 2 && g_lib.CCW(g.hull[len(g.hull)-1], top, points[i]) <= 0 {
			top = g.hull[len(g.hull)-1]
			g.hull = g.hull[:len(g.hull)-1]
		}
		g.hull = append(g.hull, top, points[i])
	}
	return g
}

// Hull returns the vertices of the convex hull in counterclockwise
// order, starting from the lowest point (lowest y, ties broken by
// lowest x) — corner vertices only.  Each call returns a fresh slice,
// so callers may keep or mutate it.  An empty input (or a nil
// GrahamScan) gives an empty, non-nil slice; a fully collinear input
// gives its two endpoints; a single point gives that point.
// Complexity is O(h) where h is the number of hull vertices.
func (g *GrahamScan) Hull() []g_lib.Point2D {
	if g == nil || len(g.hull) == 0 {
		return []g_lib.Point2D{}
	}
	return slices.Clone(g.hull)
}

// Len returns the number of hull vertices — 0 for an empty input, 1
// for a single point or all-equal points, 2 for fully collinear
// input, and at least 3 otherwise.
// Complexity is O(1).
func (g *GrahamScan) Len() int {
	if g == nil {
		return 0
	}
	return len(g.hull)
}
