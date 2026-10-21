/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package g_lib

import (
	"fmt"
	"math"
)

// Point2D is a point in the plane — Sedgwick & Wayne, Algorithms 4th
// ed. §1.2 (edu.princeton.cs.algs4.Point2D), carrying the surface the
// library's geometry work needs (kd_tree, and the §9 geometry batch:
// Graham scan, closest pair, farthest pair).  It is a plain
// comparable value type: no methods mutate, no interfaces, safe to use
// as a map key.
//
// Unlike the Java original, the constructor cannot reject NaN
// coordinates — construct with care.  A Point2D with a NaN coordinate
// never compares equal to anything and has an unspecified distance to
// everything; the kd_tree package panics on inserting one, and the
// geometry package's constructors panic on NaN input for the same
// reason (CCW reports "collinear" on a NaN cross product).
type Point2D struct {
	X, Y float64
}

// String formats the point as "(x, y)" using %v coordinates — the
// algs4 toString shape ("(%f, %f)" widened to Go's default float
// formatting so exact values round-trip).
func (p Point2D) String() string {
	return fmt.Sprintf("(%v, %v)", p.X, p.Y)
}

// DistanceSquaredTo returns the squared Euclidean distance between p and
// q.  Squared distance is the working unit of every nearest-neighbor
// algorithm (no square root until an answer is demanded).
// Complexity is O(1).
func (p Point2D) DistanceSquaredTo(q Point2D) float64 {
	dx := p.X - q.X
	dy := p.Y - q.Y
	return prod(dx, dx) + prod(dy, dy)
}

// DistanceTo returns the Euclidean distance between p and q.
// Complexity is O(1).
func (p Point2D) DistanceTo(q Point2D) float64 {
	return math.Sqrt(p.DistanceSquaredTo(q))
}

// CompareXY orders two points by x-coordinate, breaking ties by
// y-coordinate — algs4 Point2D.X_ORDER, the sort the closest-pair
// divide-and-conquer runs first.
// Complexity is O(1).
func CompareXY(a, b Point2D) int {
	switch {
	case a.X < b.X:
		return -1
	case a.X > b.X:
		return 1
	case a.Y < b.Y:
		return -1
	case a.Y > b.Y:
		return 1
	default:
		return 0
	}
}

// CompareYX orders two points by y-coordinate, breaking ties by
// x-coordinate — algs4 Point2D.Y_ORDER (== compareTo), the order that
// puts the Graham scan base point (lowest y, then lowest x) first.
// Complexity is O(1).
func CompareYX(a, b Point2D) int {
	switch {
	case a.Y < b.Y:
		return -1
	case a.Y > b.Y:
		return 1
	case a.X < b.X:
		return -1
	case a.X > b.X:
		return 1
	default:
		return 0
	}
}

// prod returns round(x·y) — the product pinned to its double-rounded
// value via math.FMA so the compiler cannot contract it into a
// surrounding add or subtract (the Go spec permits FMA fusion, even
// across statements).  This is the geo package's unfused-product
// contract applied to the planar predicates: without the pin,
// CCW(a, b, b) returns the rounding error of the fused multiply on
// arm64 instead of exactly 0 — observed, not hypothetical — and
// every degenerate-turn decision downstream (polar ties, collinear
// pops, antipodal ties) corrupts with it.  The pinned value is also
// identical at every call site, inlined or not.
func prod(x, y float64) float64 { return math.FMA(x, y, 0) }

// Area2 returns twice the signed area of the triangle a-b-c —
// positive if a→b→c is a counterclockwise turn, negative if
// clockwise, 0 if collinear.  It is CCW's raw magnitude (the
// "distance from the edge's line" measure the rotating calipers
// compare) with the same pinned products.
// Complexity is O(1).
func Area2(a, b, c Point2D) float64 {
	return prod(b.X-a.X, c.Y-a.Y) - prod(b.Y-a.Y, c.X-a.X)
}

// CCW is the orientation predicate: +1 if a→b→c is a
// counterclockwise turn, -1 if clockwise, 0 if the three points are
// collinear — the sign of Area2, algs4 Point2D.ccw.  It is the
// workhorse of every planar algorithm: polar order, the Graham
// scan's pop rule, antipodal-pair walking all reduce to it.
//
// The underlying products are pinned unfused (see prod) — do NOT
// rewrite them as a bare dx1*dy2 − dy1*dx2 expression: Go's
// permitted FMA contraction would return the multiply's rounding
// error for degenerate triples (CCW(a, b, b) ≠ 0), the geo
// package's Distance trap all over again.
//
// NaN coordinates poison the product to NaN, and NaN compares false
// against 0, so CCW reports 0 ("collinear") — the geometry package
// rejects NaN input at construction for exactly this reason.
// Complexity is O(1).
func CCW(a, b, c Point2D) int {
	area2 := Area2(a, b, c)
	switch {
	case area2 > 0:
		return 1
	case area2 < 0:
		return -1
	default:
		return 0
	}
}

// PolarOrder returns a comparison function that orders points by the
// polar angle they make with p, counterclockwise from p's positive
// x-direction, smaller angle first — ties (same angle, i.e. points
// collinear with p) comparing equal — algs4 Point2D.polarOrder.
// Sort with it via slices.SortStableFunc — see the geometry package
// for why the stability of the surrounding sort is load-bearing.
//
// The comparator never computes an angle: it decides with the sign of
// the dy components and CCW (cross products are far more precise
// than atan2 differences near 0° and 180°).
// Complexity is O(1) per comparison.
func (p Point2D) PolarOrder() func(a, b Point2D) int {
	return func(q1, q2 Point2D) int {
		dx1 := q1.X - p.X
		dy1 := q1.Y - p.Y
		dx2 := q2.X - p.X
		dy2 := q2.Y - p.Y

		switch {
		case dy1 >= 0 && dy2 < 0:
			return -1 // q1 above p; q2 below
		case dy2 >= 0 && dy1 < 0:
			return 1 // q1 below p; q2 above
		case dy1 == 0 && dy2 == 0: // 3-collinear and horizontal
			switch {
			case dx1 >= 0 && dx2 < 0:
				return -1
			case dx2 >= 0 && dx1 < 0:
				return 1
			default:
				return 0
			}
		default: // both above or both below
			// ccw(p, q1, q2) > 0 means q2 lies counterclockwise of q1
			// around p, i.e. q1 has the smaller angle — negate so q1
			// sorts first.
			return -CCW(p, q1, q2)
		}
	}
}

// RectHV is an axis-aligned closed rectangle — algs4 §1.2 RectHV — given
// by its min and max corners.  Contains and Intersects treat the
// boundary as included, matching the Java original.  The zero value is
// the empty rectangle at the origin (a degenerate point rectangle at
// (0,0)); the kd_tree package uses RectHV{Xmin: -Inf, Ymin: -Inf,
// Xmax: +Inf, Ymax: +Inf} as the whole plane.
//
// A rectangle with Xmin > Xmax or Ymin > Ymax (an "inverted" rectangle)
// is tolerated: it contains no point and its distances are computed by
// the usual per-axis clamping.  NaN bounds make Contains and Intersects
// false; none of the methods panic.
type RectHV struct {
	Xmin, Ymin float64
	Xmax, Ymax float64
}

// Contains reports whether the point p lies in the rectangle, boundary
// included.
// Complexity is O(1).
func (r RectHV) Contains(p Point2D) bool {
	return p.X >= r.Xmin && p.X <= r.Xmax && p.Y >= r.Ymin && p.Y <= r.Ymax
}

// Intersects reports whether the two rectangles intersect, boundaries
// touching counting as intersecting.
// Complexity is O(1).
func (r RectHV) Intersects(o RectHV) bool {
	return r.Xmin <= o.Xmax && o.Xmin <= r.Xmax && r.Ymin <= o.Ymax && o.Ymin <= r.Ymax
}

// DistanceSquaredTo returns the squared Euclidean distance from the
// point p to the closest point of the rectangle (0 when p is inside).
// The distance to a rectangle is the sum of the squared per-axis
// distances to the interval the rectangle spans on that axis.
// Complexity is O(1).
func (r RectHV) DistanceSquaredTo(p Point2D) float64 {
	dx := 0.0
	if p.X < r.Xmin {
		dx = r.Xmin - p.X
	} else if p.X > r.Xmax {
		dx = p.X - r.Xmax
	}
	dy := 0.0
	if p.Y < r.Ymin {
		dy = r.Ymin - p.Y
	} else if p.Y > r.Ymax {
		dy = p.Y - r.Ymax
	}
	return dx*dx + dy*dy
}
