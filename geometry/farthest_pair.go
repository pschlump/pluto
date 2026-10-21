/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package geometry

import (
	"math"

	"github.com/pschlump/pluto/g_lib"
)

// FarthestPair is the farthest pair of points — Sedgwick's algs4 §9
// FarthestPair.java: the diameter of the point set.  The farthest
// pair is always a pair of convex hull vertices, so the algorithm
// runs the Graham scan and then walks the hull's antipodal vertex
// pairs with the rotating-calipers technique (each hull edge visits
// its farthest vertex; the diameter is among the pairs so visited) —
// O(h) after the O(n log n) hull.  Build with NewFarthestPair; the
// object is immutable afterwards.
type FarthestPair struct {
	maxDist float64       // the pair's distance, 0 when there is no pair
	p, q    g_lib.Point2D // the pair, zero values when there is no pair
	found   bool          // whether a pair exists (len(pts) >= 2)
}

// NewFarthestPair finds the farthest pair among pts — a pair of hull
// vertices, reported in the hull's counterclockwise order.  Fewer
// than two points (or a single distinct point among duplicates)
// leaves no pair: Pair reports found false and Distance reports 0 —
// the max over an empty set.  The input is never modified.
// It panics if any point has a NaN coordinate.
// Complexity is O(n log n) for the hull plus O(h) for the calipers.
func NewFarthestPair(pts []g_lib.Point2D) *FarthestPair {
	noNaN(pts, "NewFarthestPair")

	f := &FarthestPair{}
	hull := NewGrahamScan(pts).Hull() // ccw corner vertices
	m := len(hull)
	if m < 2 {
		return f // 0 or 1 distinct point: no pair
	}
	f.found = true

	// best tracks the running maximum in squared distance (the sqrt is
	// monotone, so it is taken once at the end).
	best := 0.0
	check := func(a, b g_lib.Point2D) {
		if d := a.DistanceSquaredTo(b); d > best {
			best = d
			f.p, f.q = a, b
		}
	}

	// Degenerate hulls: a segment (fully collinear input) or a hull
	// of two vertices — its endpoints are the pair.
	if m == 2 {
		check(hull[0], hull[1])
		f.maxDist = math.Sqrt(best)
		return f
	}

	// Rotating calipers: for each hull edge (v[i] → v[i+1]) advance j
	// while the next vertex is farther from the edge's line — the
	// vertex j that stops the walk is antipodal to that edge, and the
	// diameter pair is among the (v[i], v[j]) / (v[i+1], v[j]) pairs
	// visited.  On an area tie (parallel opposite edges, e.g. a
	// rectangle) both stopping vertices are antipodal, so the tie
	// pairs are checked too.
	j := 1
	for i := 0; i < m; i++ {
		i1 := (i + 1) % m
		j1 := (j + 1) % m
		// Advance j while the next vertex is farther from this edge's
		// line than j is (twice the triangle area is the measure).
		for g_lib.Area2(hull[i], hull[i1], hull[j1]) > g_lib.Area2(hull[i], hull[i1], hull[j]) {
			j = j1
			j1 = (j1 + 1) % m
		}
		check(hull[i], hull[j])
		check(hull[i1], hull[j])
		if g_lib.Area2(hull[i], hull[i1], hull[j1]) == g_lib.Area2(hull[i], hull[i1], hull[j]) {
			// Parallel opposite edge: j1 is antipodal as well.
			check(hull[i], hull[j1])
			check(hull[i1], hull[j1])
		}
	}

	f.maxDist = math.Sqrt(best)
	return f
}

// Pair returns the farthest pair of points — two hull vertices in
// counterclockwise hull order.  found is false when the constructor
// saw fewer than two distinct points (p and q are zero values then).
// Complexity is O(1).
func (f *FarthestPair) Pair() (p, q g_lib.Point2D, found bool) {
	if f == nil || !f.found {
		return
	}
	return f.p, f.q, true
}

// Distance returns the distance between the farthest pair, or 0 when
// there is no pair (fewer than two distinct points) — the maximum
// over an empty set, and the answer for a nil or zero-value object.
// Complexity is O(1).
func (f *FarthestPair) Distance() float64 {
	if f == nil || !f.found {
		return 0
	}
	return f.maxDist
}
