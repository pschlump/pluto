/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package skip_list_ts

import (
	"reflect"
	"testing"

	"github.com/pschlump/pluto/ranges"
)

func TestRangeNavigation(t *testing.T) {
	list := NewSkipList[int]()
	for _, value := range []int{1, 3, 5, 7} {
		list.Insert(value)
	}
	if got, _ := list.Lower(3); got != 1 {
		t.Fatal(got)
	}
	if got, _ := list.Higher(3); got != 5 {
		t.Fatal(got)
	}
	sequence := list.RangeOf(ranges.OpenClosed(1, 5))
	list.Insert(4)
	got, ranks := []int{}, []int{}
	for rank, value := range sequence {
		ranks = append(ranks, rank)
		got = append(got, value)
	}
	if !reflect.DeepEqual(got, []int{3, 5}) || !reflect.DeepEqual(ranks, []int{1, 2}) {
		t.Fatal(ranks, got)
	}
}
