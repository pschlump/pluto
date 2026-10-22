/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_search_st_test

import (
	"fmt"

	"github.com/pschlump/pluto/binary_search_st"
)

func Example() {
	// A frequency counter, the algs4 §3.1 client: the value is replaced
	// on every repeat Put, so the count grows in place.
	st := binary_search_st.NewBinarySearchST[string, int]()
	for _, w := range []string{"it", "was", "the", "best", "of", "times", "it", "was"} {
		count, _ := st.Get(w)
		st.Put(w, count+1)
	}
	fmt.Println(st.Len(), st.String())
	// Output: 6 [best:1 it:2 of:1 the:1 times:1 was:2]
}

func ExampleBinarySearchST_orderedRiders() {
	st := binary_search_st.NewBinarySearchST[string, int]()
	for i, k := range []string{"d", "b", "f", "a", "c", "e"} {
		st.Put(k, i)
	}
	rank, _ := st.Rank("d")
	_, minVal, _ := st.FindMin()
	_, ceilVal, _ := st.Ceil("bb")
	fmt.Println(rank, minVal, ceilVal)
	for k, v := range st.Range("b", "e") {
		fmt.Print(k, ":", v, " ")
	}
	fmt.Println()
	// Output:
	// 3 3 4
	// b:1 c:4 d:0 e:5
}

func ExampleNewBinarySearchSTFunc() {
	type point struct{ x, y int }
	st := binary_search_st.NewBinarySearchSTFunc[point, string](func(a, b point) int {
		switch {
		case a.x != b.x:
			if a.x < b.x {
				return -1
			}
			return 1
		case a.y != b.y:
			if a.y < b.y {
				return -1
			}
			return 1
		default:
			return 0
		}
	})
	st.Put(point{2, 3}, "two-three")
	st.Put(point{1, 5}, "one-five")
	v, found := st.Get(point{2, 3})
	fmt.Println(found, v)
	// Output: true two-three
}
