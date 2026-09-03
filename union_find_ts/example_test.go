/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package union_find_ts_test

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/pschlump/pluto/union_find_ts"
)

// Sedgwick's classic 10-site trace (algs4 §1.5, tinyUF.txt): eleven
// pairs over the sites 0..9.  A union reports false when the two sites
// are already connected (8-9, 1-0, and 6-7 here); the ten sites end in
// two components, {0,1,2,5,6,7} and {3,4,8,9}.
func Example() {
	uf := union_find_ts.NewUnionFind(10)
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

// One union-find shared between goroutines: each goroutine merges its
// own band of sites, then a final pass joins the bands.  Union, Find,
// and Connected take the write lock (path halving mutates the forest),
// so no extra synchronization is needed around them.
func ExampleNewUnionFind() {
	const n = 100
	uf := union_find_ts.NewUnionFind(n)

	var wg sync.WaitGroup
	for g := range 4 {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 1; i < 25; i++ {
				uf.Union(base, base+i)
			}
		}(g * 25)
	}
	wg.Wait()
	fmt.Println(uf.Count()) // 4 bands of 25

	uf.Union(0, 25)
	uf.Union(0, 50)
	uf.Union(0, 75)
	fmt.Println(uf.Count(), uf.Connected(0, 99))
	// Output:
	// 4
	// 1 true
}

// MarshalJSON encodes the union-find as a JSON array of arrays — one
// array per disjoint set, sets ordered by their smallest member.
func ExampleUnionFind_MarshalJSON() {
	uf := union_find_ts.NewUnionFind(6)
	uf.Union(4, 5)
	uf.Union(0, 1)
	uf.Union(1, 2)

	b, err := json.Marshal(uf)
	fmt.Println(string(b), err)
	// Output:
	// [[0,1,2],[3],[4,5]] <nil>
}

// UnmarshalJSON replaces the partition from a JSON array of arrays; the
// receiver must have the same number of elements as the decoded
// partition.
func ExampleUnionFind_UnmarshalJSON() {
	uf := union_find_ts.NewUnionFind(6)
	if err := json.Unmarshal([]byte(`[[0,1,2],[3],[4,5]]`), uf); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(uf.Count(), "components")
	fmt.Println("0 and 2 connected:", uf.Connected(0, 2))
	fmt.Println("0 and 3 connected:", uf.Connected(0, 3))
	// Output:
	// 3 components
	// 0 and 2 connected: true
	// 0 and 3 connected: false
}
