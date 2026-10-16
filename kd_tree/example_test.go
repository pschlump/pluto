/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package kd_tree_test

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/pschlump/pluto/g_lib"
	"github.com/pschlump/pluto/kd_tree"
)

// tracePoints is the hand-traced fixture of kd_tree_test.go: a small
// set of well-spread points in the unit square.
var tracePoints = []g_lib.Point2D{
	{X: 0.7, Y: 0.2}, {X: 0.5, Y: 0.4}, {X: 0.2, Y: 0.3}, {X: 0.4, Y: 0.7}, {X: 0.9, Y: 0.6},
}

func ExampleKdTree_Nearest() {
	tr := kd_tree.NewKdTree()
	for _, p := range tracePoints {
		tr.Insert(p)
	}
	// Where would a new cell tower most annoy people?  The closest
	// existing site answers.
	p, ok := tr.Nearest(g_lib.Point2D{X: 0.85, Y: 0.45})
	fmt.Println(p, ok)
	// Output: (0.9, 0.6) true
}

func ExampleKdTree_NearestK() {
	tr := kd_tree.NewKdTree()
	for _, p := range tracePoints {
		tr.Insert(p)
	}
	// The two nearest sites to a new customer, closest first.
	for _, p := range tr.NearestK(g_lib.Point2D{X: 0.6, Y: 0.5}, 2) {
		fmt.Println(p)
	}
	// Output:
	// (0.5, 0.4)
	// (0.4, 0.7)
}

func ExampleKdTree_Range() {
	tr := kd_tree.NewKdTree()
	for _, p := range tracePoints {
		tr.Insert(p)
	}
	// Every site inside a service rectangle, boundary included; the
	// result is in tree order, so sort it for display.
	pts := tr.Range(g_lib.RectHV{Xmin: 0.2, Ymin: 0.2, Xmax: 0.5, Ymax: 0.4})
	slices.SortFunc(pts, func(a, b g_lib.Point2D) int {
		if c := cmp.Compare(a.X, b.X); c != 0 {
			return c
		}
		return cmp.Compare(a.Y, b.Y)
	})
	for _, p := range pts {
		fmt.Println(p)
	}
	// Output:
	// (0.2, 0.3)
	// (0.5, 0.4)
}

func ExampleKdTree_All() {
	tr := kd_tree.NewKdTree()
	for _, p := range tracePoints {
		tr.Insert(p)
	}
	for p := range tr.All() {
		fmt.Print(p, " ")
	}
	fmt.Println()
	// Output:
	// (0.7, 0.2) (0.5, 0.4) (0.2, 0.3) (0.4, 0.7) (0.9, 0.6)
}

func ExamplePointSet_Nearest() {
	// The brute-force alternative — same queries, linear scans, no
	// tree.  For a handful of points this is all you need.
	ps := kd_tree.NewPointSet()
	for _, p := range tracePoints {
		ps.Insert(p)
	}
	p, ok := ps.Nearest(g_lib.Point2D{X: 0.15, Y: 0.25})
	fmt.Println(p, ok, ps.Len())
	// Output: (0.2, 0.3) true 5
}
