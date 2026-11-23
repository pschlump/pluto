/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package tree_set_ts

import "testing"

func TestNonDestructiveAlgebra(t *testing.T) {
	left, right := FromSlice([]int{1, 2}), FromSlice([]int{2, 3})
	if !UnionOf(left, right).Equal(FromSlice([]int{1, 2, 3})) || !IntersectionOf(left, right).Equal(FromSlice([]int{2})) || !DifferenceOf(left, right).Equal(FromSlice([]int{1})) || !SymmetricDifferenceOf(left, right).Equal(FromSlice([]int{1, 3})) {
		t.Fatal("algebra mismatch")
	}
}
