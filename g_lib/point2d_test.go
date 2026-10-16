/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package g_lib

import (
	"math"
	"testing"
)

func TestPoint2DDistance(t *testing.T) {
	p, q := Point2D{X: 0, Y: 0}, Point2D{X: 3, Y: 4}
	if d2 := p.DistanceSquaredTo(q); d2 != 25 {
		t.Fatalf("DistanceSquaredTo(3-4-5) = %v, want 25", d2)
	}
	if d := p.DistanceTo(q); d != 5 {
		t.Fatalf("DistanceTo(3-4-5) = %v, want 5", d)
	}
	// Exact identities.
	if d := p.DistanceTo(p); d != 0 {
		t.Fatalf("DistanceTo self = %v, want 0", d)
	}
	if p.DistanceTo(q) != q.DistanceTo(p) {
		t.Fatalf("distance is not symmetric")
	}
}

func TestPoint2DStringComparable(t *testing.T) {
	// Point2D must stay usable as a map key (the kd_tree contract).
	m := map[Point2D]int{{X: 1, Y: 2}: 3}
	if m[Point2D{X: 1, Y: 2}] != 3 || len(m) != 1 {
		t.Fatalf("Point2D map-key equality broken")
	}
	if got := (Point2D{X: 0.5, Y: -1.25}).String(); got != "(0.5, -1.25)" {
		t.Fatalf("String = %q", got)
	}
}

func TestRectHVContainsIntersects(t *testing.T) {
	r := RectHV{Xmin: 0, Ymin: 0, Xmax: 1, Ymax: 1}
	for _, p := range []Point2D{{0, 0}, {1, 1}, {0.5, 1}, {1, 0.5}, {0.25, 0.75}} {
		if !r.Contains(p) {
			t.Fatalf("Contains(%v) = false, boundary is inclusive", p)
		}
	}
	for _, p := range []Point2D{{-0.0001, 0}, {1.0001, 1}, {0.5, 1.5}, {2, -1}} {
		if r.Contains(p) {
			t.Fatalf("Contains(%v) = true", p)
		}
	}

	inner := RectHV{Xmin: 0.25, Ymin: 0.25, Xmax: 0.75, Ymax: 0.75}
	touching := RectHV{Xmin: 1, Ymin: 0, Xmax: 2, Ymax: 1} // shares edge x=1
	disjoint := RectHV{Xmin: 1.5, Ymin: 1.5, Xmax: 2, Ymax: 2}
	if !r.Intersects(inner) || !inner.Intersects(r) {
		t.Fatalf("contained rectangle does not intersect")
	}
	if !r.Intersects(touching) || !touching.Intersects(r) {
		t.Fatalf("edge-touching rectangles do not intersect")
	}
	if r.Intersects(disjoint) || disjoint.Intersects(r) {
		t.Fatalf("disjoint rectangles intersect")
	}
}

func TestRectHVDistanceSquared(t *testing.T) {
	r := RectHV{Xmin: 0, Ymin: 0, Xmax: 2, Ymax: 2}
	cases := []struct {
		p    Point2D
		want float64
	}{
		{p: Point2D{X: 1, Y: 1}, want: 0},   // inside
		{p: Point2D{X: 1, Y: 2}, want: 0},   // on the boundary
		{p: Point2D{X: 3, Y: 1}, want: 1},   // 1 past the right edge
		{p: Point2D{X: 1, Y: 5}, want: 9},   // 3 past the top edge
		{p: Point2D{X: 3, Y: 4}, want: 5},   // corner: 1² + 2²
		{p: Point2D{X: -2, Y: -2}, want: 8}, // corner: 2² + 2²
	}
	for _, tc := range cases {
		if got := r.DistanceSquaredTo(tc.p); got != tc.want {
			t.Fatalf("DistanceSquaredTo(%v) = %v, want %v", tc.p, got, tc.want)
		}
	}
}

func TestRectHVEmptyAndNaN(t *testing.T) {
	empty := RectHV{Xmin: 2, Ymin: 2, Xmax: 1, Ymax: 1} // inverted
	if empty.Contains(Point2D{X: 1.5, Y: 1.5}) {
		t.Fatalf("inverted rectangle contains a point")
	}
	// No method panics on NaN.
	nanR := RectHV{Xmin: math.NaN(), Ymin: 0, Xmax: 1, Ymax: 1}
	if nanR.Contains(Point2D{X: 0.5, Y: 0.5}) {
		t.Fatalf("NaN rectangle contains a point")
	}
	if nanR.Intersects(RectHV{Xmin: 0, Ymin: 0, Xmax: 1, Ymax: 1}) {
		t.Fatalf("NaN rectangle intersects")
	}
	nanP := Point2D{X: math.NaN(), Y: 0}
	d2 := nanP.DistanceSquaredTo(Point2D{})
	// NaN distances are NaN: they do not panic and do not order
	// (both < and > are false against any number).
	if d2 < 1 || d2 > 1 {
		t.Fatalf("NaN distance ordered against a number")
	}
}

func TestRectHVWholePlane(t *testing.T) {
	plane := RectHV{Xmin: math.Inf(-1), Ymin: math.Inf(-1), Xmax: math.Inf(1), Ymax: math.Inf(1)}
	if !plane.Contains(Point2D{X: -1e308, Y: 1e308}) {
		t.Fatalf("the whole plane misses a finite point")
	}
	if plane.DistanceSquaredTo(Point2D{X: -1e308, Y: 1e308}) != 0 {
		t.Fatalf("distance inside the whole plane is not 0")
	}
	if !plane.Intersects(RectHV{Xmin: 0, Ymin: 0, Xmax: 0, Ymax: 0}) {
		t.Fatalf("the whole plane misses a point rectangle")
	}
}
