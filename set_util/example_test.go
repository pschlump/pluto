/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package set_util_test

import (
	"fmt"

	"github.com/pschlump/pluto/set_util"
)

func ExampleCartesian() {
	for tuple := range set_util.Cartesian([]string{"a", "b"}, []string{"1", "2"}) {
		fmt.Println(tuple)
	}
	// Output:
	// [a 1]
	// [a 2]
	// [b 1]
	// [b 2]
}

func ExamplePowerSet() {
	for subset := range set_util.PowerSet([]int{1, 2}) {
		fmt.Println(subset)
	}
	// Output:
	// []
	// [1]
	// [2]
	// [1 2]
}
