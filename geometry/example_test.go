/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package geometry_test

import (
	"fmt"

	"github.com/pschlump/pluto/g_lib"
	"github.com/pschlump/pluto/geometry"
)

// Convex hull: the corner vertices in counterclockwise order,
// starting from the lowest point — the point on the bottom edge is
// not a vertex (corner-only convention).
func ExampleGrahamScan() {
	pts := []g_lib.Point2D{
		{X: 1, Y: 1}, {X: 2, Y: 0}, {X: 2, Y: 2}, {X: 0, Y: 2}, {X: 0, Y: 0}, {X: 1, Y: 0},
	}
	hull := geometry.NewGrahamScan(pts).Hull()
	for _, p := range hull {
		fmt.Print(p, " ")
	}
	fmt.Println()
	// Output:
	// (0, 0) (2, 0) (2, 2) (0, 2)
}

// Closest pair of points, O(n log n) instead of the O(n²) sweep.
func ExampleClosestPair() {
	pts := []g_lib.Point2D{
		{X: 0, Y: 0}, {X: 10, Y: 10}, {X: 1, Y: 1}, {X: 10, Y: 0}, {X: 3, Y: 4},
	}
	p, q, found := geometry.NewClosestPair(pts).Pair()
	if found {
		fmt.Println(p, q)
	}
	// Output:
	// (0, 0) (1, 1)
}

// Farthest pair — the diameter, a pair of hull vertices found by
// rotating calipers.
func ExampleFarthestPair() {
	pts := []g_lib.Point2D{
		{X: 0, Y: 0}, {X: 6, Y: 0}, {X: 6, Y: 8}, {X: 0, Y: 8}, {X: 3, Y: 4},
	}
	p, q, _ := geometry.NewFarthestPair(pts).Pair()
	fmt.Println(p, q, geometry.NewFarthestPair(pts).Distance())
	// Output:
	// (0, 0) (6, 8) 10
}
