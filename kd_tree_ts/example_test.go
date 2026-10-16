/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package kd_tree_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/g_lib"
	"github.com/pschlump/pluto/kd_tree_ts"
)

func ExampleKdTree() {
	tr := kd_tree_ts.NewKdTree()
	for _, p := range []g_lib.Point2D{
		{X: 0.7, Y: 0.2}, {X: 0.5, Y: 0.4}, {X: 0.2, Y: 0.3},
	} {
		tr.Insert(p)
	}
	p, ok := tr.Nearest(g_lib.Point2D{X: 0.4, Y: 0.35})
	fmt.Println(p, ok, tr.Len())
	// Output: (0.5, 0.4) true 3
}

// ExampleKdTree_Lock is the canonical compound: accept a new site only
// when it is spaced away from every existing one — the nearest-neighbor
// check and the insert run as one atomic section under Lock, so a
// concurrent writer cannot slip a site into the gap between them.
func ExampleKdTree_Lock() {
	tr := kd_tree_ts.NewKdTree()
	for _, p := range []g_lib.Point2D{
		{X: 0.7, Y: 0.2}, {X: 0.5, Y: 0.4}, {X: 0.2, Y: 0.3}, {X: 0.4, Y: 0.7}, {X: 0.9, Y: 0.6},
	} {
		tr.Insert(p)
	}

	insertIfFar := func(p g_lib.Point2D, minDist float64) bool {
		tr.Lock()
		defer tr.Unlock()
		if near, ok := tr.NlNearest(p); ok {
			if dx, dy := near.X-p.X, near.Y-p.Y; dx*dx+dy*dy < minDist*minDist {
				return false
			}
		}
		return tr.NlInsert(p)
	}

	fmt.Println(insertIfFar(g_lib.Point2D{X: 0.05, Y: 0.05}, 0.2)) // nearest is 0.23 away: accepted
	fmt.Println(insertIfFar(g_lib.Point2D{X: 0.26, Y: 0.28}, 0.2)) // nearest is 0.06 away: rejected
	fmt.Println(tr.Len())
	// Output:
	// true
	// false
	// 6
}
