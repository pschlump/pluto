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
// ed. §1.2 (edu.princeton.cs.algs4.Point2D), stripped to the surface the
// library's geometry work needs (the kd_tree package today, the §9
// geometry batch — Graham scan, closest pair — later).  It is a plain
// comparable value type: no methods mutate, no interfaces, safe to use
// as a map key.
//
// Unlike the Java original, the constructor cannot reject NaN
// coordinates — construct with care.  A Point2D with a NaN coordinate
// never compares equal to anything and has an unspecified distance to
// everything; the kd_tree package panics on inserting one.
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
	return dx*dx + dy*dy
}

// DistanceTo returns the Euclidean distance between p and q.
// Complexity is O(1).
func (p Point2D) DistanceTo(q Point2D) float64 {
	return math.Sqrt(p.DistanceSquaredTo(q))
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
