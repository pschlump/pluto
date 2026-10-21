/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package geometry implements the §9 computational-geometry query
// objects of Sedgwick & Wayne, Algorithms 4th ed. — GrahamScan
// (convex hull, algs4 GrahamScan.java), ClosestPair (closest pair of
// points by divide-and-conquer, algs4 ClosestPair.java) and
// FarthestPair (farthest pair by convex hull + rotating calipers,
// algs4 FarthestPair.java) — over the plain g_lib.Point2D value type.
// It is not the geo package (Redis geohash bit compatibility); the
// names share three letters and nothing else.
//
// All three are constructor-built immutable query objects in the
// substring/suffix_array shape: one constructor runs the algorithm on
// a private copy of the points (the input is never modified), and the
// query methods are O(1) or O(h) afterwards.  Nothing mutates after
// construction, so concurrent reads are safe and there is no _ts
// twin.
//
// The element type is concrete, not a parameter: Point2D is a float64
// pair, and these algorithms are about its arithmetic (the union_find
// and geo precedent for non-generic packages).  Integer coordinates
// within ±2²⁶ make CCW exact; general floats are subject to the usual
// degenerate-case caveats — the orientation predicate CCW in g_lib is
// pinned to the exact algs4 expression form and must not be
// refactored.
//
// Hull orientation: GrahamScan.Hull returns the hull's corner
// vertices only (collinear points on a hull edge are NOT vertices) in
// counterclockwise order starting from the lowest point (lowest y,
// ties broken by lowest x).  A fully collinear input degenerates to
// its two endpoints; n ≤ 1 degenerates to the input itself.
//
// Robustness: with integer coordinates (|values| < 2²⁶) every
// predicate here is exact and the results are the mathematical hull
// and pairs.  With general floats, NEARLY-collinear triples make
// CCW's sign rounding noise — the corner-only guarantee can admit a
// near-collinear vertex, and a "same" pair query may pick a
// different pair of the same extremal distance.  The algorithms
// still terminate on every NaN-free input (the scan cannot pop past
// its base vertex — the algs4 original underflows its stack in this
// regime); exact answers are the caller's to arrange by scaling to
// integers when they matter.
//
// The package panics in exactly one situation class, a programmer
// error with no sane fallback: a NaN coordinate anywhere in the input
// to any of the three constructors (the kd_tree precedent).  A NaN
// poisons CCW's cross product into a false "collinear", corrupting
// every ordering decision downstream — there is no sane hull or pair
// to report.  Every query on a nil or zero-value object behaves as
// the empty case: an empty hull, no pair, the distance identity.
package geometry

import (
	"math"

	"github.com/pschlump/pluto/g_lib"
)

// noNaN panics when any point carries a NaN coordinate — the single
// panic class of the package.  who in the message is the constructor
// the caller used.
func noNaN(pts []g_lib.Point2D, who string) {
	for _, p := range pts {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) {
			panic("geometry: " + who + " called with input containing a NaN coordinate")
		}
	}
}
