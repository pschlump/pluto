/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package ranges_test

import (
	"fmt"

	"github.com/pschlump/pluto/ranges"
)

func ExampleRange() {
	left := ranges.ClosedOpen(1, 4)
	right := ranges.Open(2, 8)
	fmt.Println(left.IsConnected(right), left.Intersection(right).ContainsAll([]int{3}))
	// Output: true true
}

func ExampleContiguousSet() {
	set := ranges.NewContiguousSet(ranges.OpenClosed(1, 5), ranges.SignedDomain[int]())
	size, _ := set.Len()
	fmt.Println(size)
	for value := range set.All() {
		fmt.Print(value, " ")
	}
	fmt.Println()
	// Output:
	// 4
	// 2 3 4 5
}
