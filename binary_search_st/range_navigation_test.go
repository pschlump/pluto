/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package binary_search_st

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
	if got, _, found := table.Lower(3); !found || got != 1 {
		t.Fatal(got, found)
	}
	if got, _, found := table.Higher(3); !found || got != 5 {
		t.Fatal(got, found)
	}
	if _, _, found := table.Lower(1); found {
		t.Fatal("lower minimum")
	}
	if _, _, found := table.Higher(7); found {
		t.Fatal("higher maximum")
	}
	got := []int{}
	for key := range table.RangeOf(ranges.OpenClosed(1, 5)) {
		got = append(got, key)
	}
	if !reflect.DeepEqual(got, []int{3, 5}) {
		t.Fatal(got)
	}
	got = []int{}
	for key := range table.RangeOf(ranges.AtLeast(5)) {
		got = append(got, key)
	}
	if !reflect.DeepEqual(got, []int{5, 7}) {
		t.Fatal(got)
	}
}
