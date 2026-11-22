/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset_test

import (
	"fmt"

	"github.com/pschlump/pluto/multiset"
)

func ExampleHashMultiset() {
	set := multiset.FromSlice([]string{"apple", "banana", "apple"})
	set.Add("banana", 2)
	set.Remove("apple", 1)
	fmt.Println(set.Len(), set.DistinctLen(), set.Count("banana"))
	// Output: 4 2 3
}

func ExampleTreeMultiset() {
	set := multiset.FromTreeSlice([]int{3, 1, 3, 2, 3})
	fmt.Println(set.Slice())
	fmt.Println(set.SubRange(2, true, 3, false).Slice())
	// Output:
	// [1 2 3 3 3]
	// [2]
}
