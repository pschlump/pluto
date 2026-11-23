/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package range_set_test

import (
	"fmt"

	"github.com/pschlump/pluto/range_set"
	"github.com/pschlump/pluto/ranges"
)

func ExampleRangeSet() {
	set := range_set.NewDiscreteRangeSet[int](ranges.SignedDomain[int]())
	set.Add(ranges.Closed(1, 3))
	set.Add(ranges.ClosedOpen(4, 6))
	set.Remove(ranges.Closed(2, 2))
	fmt.Println(set.Len(), set.Contains(1), set.Contains(2), set.Contains(5))
	// Output: 2 true false true
}
