/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package range_map_test

import (
	"fmt"

	"github.com/pschlump/pluto/range_map"
	"github.com/pschlump/pluto/ranges"
)

func ExampleRangeMap() {
	mapped := range_map.NewRangeMap[int, string]()
	mapped.Put(ranges.Closed(1, 10), "outer")
	mapped.Put(ranges.Closed(3, 5), "inner")
	fmt.Println(mapped.Get(2))
	fmt.Println(mapped.Get(4))
	// Output:
	// outer true
	// inner true
}
