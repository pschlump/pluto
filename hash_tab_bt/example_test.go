/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
//
// The default NewHashTab hashing uses a per-process random seed, so examples
// that iterate a table sort their output first — bucket order is never
// asserted.
package hash_tab_bt_test

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/pschlump/charon/hash_tab_bt"
)

// A basic set-membership table of strings: no methods to implement, the
// builtin < and > decide the bucket tree ordering.
func Example() {
	ht := hash_tab_bt.NewHashTab[string](16) // 16 buckets, fixed for the life of the table

	ht.Insert("alpha")
	ht.Insert("beta")
	replaced := ht.Insert("alpha") // duplicate: replaces, returns false

	if v, found := ht.Search("alpha"); found {
		fmt.Println("found", v)
	}
	fmt.Println("replaced:", replaced, "len:", ht.Len())

	ht.Delete("alpha")
	fmt.Println("after delete:", ht.Len(), ht.IsEmpty())
	// Output:
	// found alpha
	// replaced: false len: 2
	// after delete: 1 false
}

// Structs ordered and hashed by a field — the element type implements no
// interface; the ordering and the hash are plain functions.
func ExampleNewHashTabFunc() {
	type User struct {
		ID   int
		Name string
	}

	byID := hash_tab_bt.NewHashTabFunc(
		func(a, b User) int { return a.ID - b.ID },
		func(u User) uint64 {
			h := fnv.New64a()
			_, _ = fmt.Fprint(h, u.ID)
			return h.Sum64()
		},
		8, // 8 buckets
	)

	byID.Insert(User{ID: 1, Name: "write"})
	byID.Insert(User{ID: 2, Name: "review"})
	byID.Insert(User{ID: 1, Name: "write it again"}) // same ID: replaces

	// A search probe only needs the fields the functions read.
	if u, found := byID.Search(User{ID: 2}); found {
		fmt.Println("found user:", u.Name)
	}

	// Iteration is in bucket order (hash-dependent) — sort when order matters.
	names := []string{}
	for u := range byID.Values() {
		names = append(names, u.Name)
	}
	sort.Strings(names)
	fmt.Println(strings.Join(names, ","))
	// Output:
	// found user: review
	// review,write it again
}

// All yields (bucket position, element) pairs; a single-variable range
// yields the bucket position, not the element.
func ExampleHashTab_All() {
	ht := hash_tab_bt.NewHashTab[int](9)
	for _, v := range []int{10, 20, 30} {
		ht.Insert(v)
	}

	positions := []int{}
	for pos := range ht.All() { // pos is the bucket index
		positions = append(positions, pos)
	}
	sort.Ints(positions)
	fmt.Println("element count:", ht.Len(), "distinct buckets:", len(positions))
	// Output:
	// element count: 3 distinct buckets: 3
}

// Walk visits each element in bucket order, passing the bucket position and
// the element to the callback; within a bucket the elements come out
// ascending per the comparison function.  The closure captures the caller's
// state.  Returning false stops the walk (Walk then returns false itself).
func ExampleHashTab_Walk() {
	ht := hash_tab_bt.NewHashTab[string](8)
	ht.Insert("one")
	ht.Insert("two")
	ht.Insert("three")

	total := 0 // captured by the closure — no userData parameter needed
	completed := ht.Walk(func(pos int, data string) bool {
		total += len(data)
		return true // return false to stop the walk
	})
	fmt.Println("total:", total, "completed:", completed)
	// Output:
	// total: 11 completed: true
}
