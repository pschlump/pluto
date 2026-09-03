/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package union_find_test

import (
	"encoding/json"
	"fmt"

	"github.com/pschlump/pluto/union_find"
)

// Sedgwick's classic 10-site trace (algs4 §1.5, tinyUF.txt): eleven
// pairs over the sites 0..9.  A union reports false when the two sites
// are already connected (8-9, 1-0, and 6-7 here); the ten sites end in
// two components, {0,1,2,5,6,7} and {3,4,8,9}.
func Example() {
	uf := union_find.NewUnionFind(10)
	pairs := [][2]int{
		{4, 3}, {3, 8}, {6, 5}, {9, 4}, {2, 1},
		{8, 9}, {5, 0}, {7, 2}, {6, 1}, {1, 0}, {6, 7},
	}
	for _, pq := range pairs {
		if uf.Union(pq[0], pq[1]) {
			fmt.Printf("%d-%d merged\n", pq[0], pq[1])
		} else {
			fmt.Printf("%d-%d already connected\n", pq[0], pq[1])
		}
	}
	fmt.Println(uf.Count(), "components")
	fmt.Println("0 and 4 connected:", uf.Connected(0, 4))
	fmt.Println("0 and 7 connected:", uf.Connected(0, 7))
	// Output:
	// 4-3 merged
	// 3-8 merged
	// 6-5 merged
	// 9-4 merged
	// 2-1 merged
	// 8-9 already connected
	// 5-0 merged
	// 7-2 merged
	// 6-1 merged
	// 1-0 already connected
	// 6-7 already connected
	// 2 components
	// 0 and 4 connected: false
	// 0 and 7 connected: true
}

// A fresh union-find starts with every element in its own singleton
// set: Count() == Len(), and Find returns the element itself.
func ExampleNewUnionFind() {
	uf := union_find.NewUnionFind(5)
	fmt.Println(uf.Len(), uf.Count())

	root, ok := uf.Find(3)
	fmt.Println(root, ok)

	uf.Union(1, 3)
	root, _ = uf.Find(1)
	fmt.Println(root == 3 || root == 1, uf.Count()) // one shared root, one fewer set

	if _, ok := uf.Find(5); !ok { // 5 is out of range for 0..4
		fmt.Println("5 is out of range")
	}
	// Output:
	// 5 5
	// 3 true
	// true 4
	// 5 is out of range
}

// MarshalJSON encodes the partition as a JSON array of arrays — one
// inner array per disjoint set, members ascending, sets ordered by
// their smallest member.
func ExampleUnionFind_MarshalJSON() {
	uf := union_find.NewUnionFind(6)
	uf.Union(4, 3)
	uf.Union(3, 2)
	uf.Union(1, 0)

	b, err := json.Marshal(uf)
	fmt.Println(string(b), err)
	// Output:
	// [[0,1],[2,3,4],[5]] <nil>
}

// UnmarshalJSON replaces the partition from a JSON array of arrays; the
// union-find keeps its size n, and every element 0..n-1 must appear
// exactly once.
func ExampleUnionFind_UnmarshalJSON() {
	uf := union_find.NewUnionFind(6)
	if err := json.Unmarshal([]byte(`[[0,1],[2,3,4],[5]]`), uf); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("0 and 1 connected:", uf.Connected(0, 1))
	fmt.Println("0 and 2 connected:", uf.Connected(0, 2))
	fmt.Println("sets:", uf.Count())
	// Output:
	// 0 and 1 connected: true
	// 0 and 2 connected: false
	// sets: 3
}
