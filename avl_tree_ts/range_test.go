/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package avl_tree_ts

import (
	"reflect"
	"testing"

	"github.com/pschlump/pluto/ranges"
)

func TestRangeOfSnapshot(t *testing.T) {
	tree := NewAvlTree[int]()
	for _, value := range []int{1, 3, 5, 7} {
		tree.Insert(value)
	}
	if got, _ := tree.Lower(3); got != 1 {
		t.Fatal(got)
	}
	if got, _ := tree.Higher(3); got != 5 {
		t.Fatal(got)
	}
	sequence := tree.RangeOf(ranges.Open(1, 7))
	tree.Insert(4)
	got := []int{}
	for value := range sequence {
		got = append(got, value)
	}
	if !reflect.DeepEqual(got, []int{3, 5}) {
		t.Fatal(got)
	}
}
