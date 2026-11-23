/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package tree_set_ts

import (
	"reflect"
	"testing"

	"github.com/pschlump/pluto/ranges"
)

func TestRangeNavigation(t *testing.T) {
	set := FromSlice([]int{1, 3, 5, 7})
	if got, _ := set.Lower(3); got != 1 {
		t.Fatal(got)
	}
	if got, _ := set.Higher(3); got != 5 {
		t.Fatal(got)
	}
	sequence := set.RangeOf(ranges.Open(1, 7))
	set.Insert(4)
	got := []int{}
	for value := range sequence {
		got = append(got, value)
	}
	if !reflect.DeepEqual(got, []int{3, 5}) {
		t.Fatal("range was not a snapshot", got)
	}
	got = []int{}
	for value := range set.Range(1, 5) {
		got = append(got, value)
	}
	if !reflect.DeepEqual(got, []int{1, 3, 4, 5}) {
		t.Fatal(got)
	}
}
