/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package set_view_test

import (
	"fmt"

	"github.com/pschlump/pluto/set_view"
	"github.com/pschlump/pluto/tree_set"
)

func ExampleSetView() {
	left := tree_set.FromSlice([]int{1, 2, 3})
	right := tree_set.FromSlice([]int{3, 4})
	view := set_view.SymmetricDifference(left, right)
	for value := range view.All() {
		fmt.Print(value, " ")
	}
	fmt.Println()
	// Output: 1 2 4
}
