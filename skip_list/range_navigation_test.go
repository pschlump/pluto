/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package skip_list

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
	if got, found := list.Lower(3); !found || got != 1 {
		t.Fatal(got, found)
	}
	if got, found := list.Higher(3); !found || got != 5 {
		t.Fatal(got, found)
	}
	if _, found := list.Lower(1); found {
		t.Fatal("lower minimum")
	}
	if _, found := list.Higher(7); found {
		t.Fatal("higher maximum")
	}
	gotRanks, got := []int{}, []int{}
	for rank, value := range list.RangeOf(ranges.OpenClosed(1, 5)) {
		gotRanks = append(gotRanks, rank)
		got = append(got, value)
	}
	if !reflect.DeepEqual(got, []int{3, 5}) || !reflect.DeepEqual(gotRanks, []int{1, 2}) {
		t.Fatal(gotRanks, got)
	}
	got = []int{}
	for _, value := range list.RangeOf(ranges.AtLeast(5)) {
		got = append(got, value)
	}
	if !reflect.DeepEqual(got, []int{5, 7}) {
		t.Fatal(got)
	}
}
