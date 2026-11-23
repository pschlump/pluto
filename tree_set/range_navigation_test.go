/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package tree_set

import (
	"reflect"
	"testing"

	"github.com/pschlump/pluto/ranges"
)

func TestRangeNavigation(t *testing.T) {
	set := FromSlice([]int{1, 3, 5, 7})
	if got, found := set.Lower(3); !found || got != 1 {
		t.Fatal(got, found)
	}
	if got, found := set.Higher(3); !found || got != 5 {
		t.Fatal(got, found)
	}
	if _, found := set.Lower(1); found {
		t.Fatal("lower minimum")
	}
	if _, found := set.Higher(7); found {
		t.Fatal("higher maximum")
	}
	tests := []struct {
		value ranges.Range[int]
		want  []int
	}{
		{ranges.Open(1, 5), []int{3}},
		{ranges.Closed(1, 5), []int{1, 3, 5}},
		{ranges.ClosedOpen(3, 7), []int{3, 5}},
		{ranges.OpenClosed(1, 3), []int{3}},
		{ranges.AtLeast(5), []int{5, 7}},
		{ranges.LessThan(3), []int{1}},
		{ranges.All[int](), []int{1, 3, 5, 7}},
		{ranges.ClosedOpen(3, 3), []int{}},
	}
	for _, test := range tests {
		got := []int{}
		for value := range set.RangeOf(test.value) {
			got = append(got, value)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Fatalf("RangeOf got=%v want=%v", got, test.want)
		}
	}
	got := []int{}
	for value := range set.Range(1, 5) {
		got = append(got, value)
	}
	if !reflect.DeepEqual(got, []int{1, 3, 5}) {
		t.Fatal(got)
	}
}
