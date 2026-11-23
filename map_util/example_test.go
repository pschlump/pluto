/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package map_util_test

import (
	"fmt"

	"github.com/pschlump/pluto/map_util"
)

func ExampleUniqueIndex() {
	indexed, err := map_util.UniqueIndex([]string{"a", "bb"}, func(value string) int { return len(value) })
	if err != nil {
		panic(err)
	}
	fmt.Println(indexed.Get(2))
	// Output: bb true
}
