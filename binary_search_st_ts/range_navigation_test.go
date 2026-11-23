/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package binary_search_st_ts

import (
	"reflect"
	"testing"

	"github.com/pschlump/pluto/ranges"
)

func TestRangeNavigation(t *testing.T) {
	table := NewBinarySearchST[int, string]()
	for _, key := range []int{1, 3, 5, 7} {
		table.Put(key, string(rune('a'+key)))
	}
	if got, _, _ := table.Lower(3); got != 1 {
		t.Fatal(got)
	}
	if got, _, _ := table.Higher(3); got != 5 {
		t.Fatal(got)
	}
	sequence := table.RangeOf(ranges.OpenClosed(1, 5))
	table.Put(4, "four")
	got := []int{}
	for key := range sequence {
		got = append(got, key)
	}
	if !reflect.DeepEqual(got, []int{3, 5}) {
		t.Fatal("range was not snapshot", got)
	}
}
