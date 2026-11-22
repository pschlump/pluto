/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap_test

import (
	"fmt"

	"github.com/pschlump/pluto/multimap"
)

func ExampleListMultimap() {
	values := multimap.NewListMultimap[string, int]()
	values.Put("a", 1)
	values.Put("a", 1)
	values.Put("a", 2)
	list, _ := values.Get("a")
	fmt.Println(list, values.Len(), values.KeyLen())
	// Output: [1 1 2] 3 1
}

func ExampleSortedSetMultimap() {
	values := multimap.NewSortedSetMultimap[string, int]()
	values.PutAll("a", []int{3, 1, 3, 2})
	list, _ := values.Get("a")
	fmt.Println(list)
	// Output: [1 2 3]
}
