/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package set_view

import (
	"reflect"
	"testing"

	"github.com/pschlump/pluto/tree_set"
)

func TestLazyAlgebraAndMutation(t *testing.T) {
	left := tree_set.FromSlice([]int{1, 2, 3})
	right := tree_set.FromSlice([]int{3, 4})
	tests := []struct {
		view *SetView[int]
		want []int
	}{
		{Union(left, right), []int{1, 2, 3, 4}},
		{Intersection(left, right), []int{3}},
		{Difference(left, right), []int{1, 2}},
		{SymmetricDifference(left, right), []int{1, 2, 4}},
	}
	for _, test := range tests {
		got := []int{}
		for value := range test.view.All() {
			got = append(got, value)
		}
		if !reflect.DeepEqual(got, test.want) || test.view.Len() != len(test.want) {
			t.Fatalf("got=%v want=%v", got, test.want)
		}
	}
	view := Difference(left, right)
	right.Insert(2)
	if view.Contains(2) || view.Len() != 1 {
		t.Fatal("view did not reflect mutation")
	}
	seen := 0
	for range Union(left, right).All() {
		seen++
		break
	}
	if seen != 1 {
		t.Fatal("early stop")
	}
}
