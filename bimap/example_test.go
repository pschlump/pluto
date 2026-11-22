/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bimap_test

import (
	"fmt"

	"github.com/pschlump/pluto/bimap"
)

func ExampleHashBiMap() {
	ids := bimap.NewHashBiMap[int, string]()
	_ = ids.Put(1, "alice")
	_ = ids.Put(2, "bob")
	name, _ := ids.GetValue(1)
	id, _ := ids.GetKey("bob")
	fmt.Println(name, id)
	ids.Inverse().ForcePut("alice", 3)
	fmt.Println(ids.ContainsKey(1), ids.ContainsKey(3))
	// Output:
	// alice 2
	// false true
}
