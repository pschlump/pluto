/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package avl_tree

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/pschlump/pluto/ranges"
)

func TestRangeOfSeededModel(t *testing.T) {
	random := rand.New(rand.NewSource(1729))
	tree := NewAvlTree[int]()
	for _, value := range random.Perm(100) {
		tree.Insert(value)
	}
	for step := 0; step < 2000; step++ {
		low, high := random.Intn(105)-2, random.Intn(105)-2
		if low > high {
			low, high = high, low
		}
		var query ranges.Range[int]
		switch random.Intn(8) {
		case 0:
			query = ranges.Closed(low, high)
		case 1:
			query = ranges.ClosedOpen(low, high)
		case 2:
			query = ranges.OpenClosed(low, high)
		case 3:
			if low == high {
				query = ranges.ClosedOpen(low, high)
			} else {
				query = ranges.Open(low, high)
			}
		case 4:
			query = ranges.AtLeast(low)
		case 5:
			query = ranges.GreaterThan(low)
		case 6:
			query = ranges.AtMost(high)
		case 7:
			query = ranges.LessThan(high)
		}
		got, want := []int{}, []int{}
		for value := range tree.RangeOf(query) {
			got = append(got, value)
		}
		for value := range tree.All() {
			if query.Contains(value) {
				want = append(want, value)
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("step %d RangeOf=%v want=%v", step, got, want)
		}
	}
}
