/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package geo

import "math"

// degToRad is geohash_helper.c's D_R (M_PI/180.0): the same double the
// C constant folds to, so every trigonometric path below matches Redis
// bit-for-bit.
const degToRad = math.Pi / 180.0

// radToDeg converts radians to degrees (rad_deg, the reciprocal path —
// division by the same constant).
func radToDeg(ang float64) float64 { return ang / degToRad }

// prod computes round(x·y) — arithmetically a plain multiply — through
// math.FMA so the compiler cannot contract it with a surrounding add or
// subtract.  The Go spec permits fusing a multiply that feeds an
// add/sub into one rounded op (gc does on arm64), and that fusion is
// not cosmetic: in the C's haversine the differences lon2r-lon1r and
// lat2r-lat1r cancel catastrophically for nearby points, so an
// unrounded product there surfaces its own rounding error and
// Distance(p, p) comes out ~1e-10 instead of 0.  FMA(x, y, 0) is a
// single-rounded product on every platform (hardware where available,
// software elsewhere), which pins the C's two-rounding arithmetic —
// round(x·y) then a rounded add — deterministically.
func prod(x, y float64) float64 { return math.FMA(x, y, 0) }

// latDistance is geohash_helper.c's geohashGetLatDistance: the
// great-circle distance between two points on the same meridian, which
// is R·|Δlat| in radians because asin(sin(x)) collapses to x for
// latitudes inside [-π/2, π/2].  It is the fast path Distance takes for
// equal longitudes, and one half of the BYBOX membership rule (see
// RangesForBox).
func latDistance(lat1, lat2 float64) float64 {
	return EarthRadiusInMeters * math.Abs(prod(degToRad, lat2)-prod(degToRad, lat1))
}

// Distance returns the haversine great-circle distance in meters between
// (lat1, lon1) and (lat2, lon2) — geohashGetDistance, what GEODIST
// divides by the unit factor.  The formula and operation order are the C
// exactly, including the equal-longitude shortcut that skips the
// asin/sqrt machinery; every product feeding an add or subtract is
// pinned through prod so the arithmetic stays the C's two-rounding
// form on every platform (see prod).  What remains platform-dependent
// is only the trigonometry itself — sin/cos/asin may differ by an ulp
// between libms, which through the formula's cancellations shows up as
// ~1e-11 relative on near-coincident points.  Radius is
// EarthRadiusInMeters.
//
// Complexity is O(1).
func Distance(lat1, lon1, lat2, lon2 float64) float64 {
	lon1r := prod(degToRad, lon1)
	lon2r := prod(degToRad, lon2)
	v := math.Sin((lon2r - lon1r) / 2)
	// if v == 0 we can avoid doing expensive math when the longitudes
	// are practically the same — the C shortcut.
	if v == 0.0 {
		return latDistance(lat1, lat2)
	}
	lat1r := prod(degToRad, lat1)
	lat2r := prod(degToRad, lat2)
	u := math.Sin((lat2r - lat1r) / 2)
	a := prod(u, u) + prod(prod(prod(math.Cos(lat1r), math.Cos(lat2r)), v), v)
	return 2.0 * EarthRadiusInMeters * math.Asin(math.Sqrt(a))
}
