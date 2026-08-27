/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
//
// The default NewHashTab hashing uses a per-process random seed, so examples
// that iterate a table sort their output first — slot order is never
// asserted.
package cuckoo_test

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/pschlump/pluto/cuckoo"
)

// A basic set-membership table of strings: no methods to implement, the
// builtin == decides equality.
func Example() {
	ht := cuckoo.NewHashTab[string](16, 0, 0) // size 16, default thresholds 0.85/0.20

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

// Structs compared and hashed by a field — the element type implements no
// interface; equality and hashing are plain functions.
func ExampleNewHashTabFunc() {
	type User struct {
		ID   int
		Name string
	}

	byID := cuckoo.NewHashTabFunc(
		func(a, b User) bool { return a.ID == b.ID },
		func(u User) uint64 {
			h := fnv.New64a()
			_, _ = fmt.Fprint(h, u.ID)
			return h.Sum64()
		},
		8, 0, 0, // rounds up to the 256 minimum; default thresholds 0.85/0.20
	)

	byID.Insert(User{ID: 1, Name: "write"})
	byID.Insert(User{ID: 2, Name: "review"})
	byID.Insert(User{ID: 1, Name: "write it again"}) // same ID: replaces

	// A search probe only needs the fields the functions read.
	if u, found := byID.Search(User{ID: 2}); found {
		fmt.Println("found user:", u.Name)
	}

	// Iteration is in slot order (hash-dependent) — sort when order matters.
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

// Capacity and Saturation report the table size and its load factor; the
// table doubles when the saturation passes the grow threshold and halves
// when deletions take it below the shrink threshold.
func ExampleHashTab_Saturation() {
	ht := cuckoo.NewHashTab[int](8, 0, 0) // requests round up — capacity starts at 256
	for i := range 128 {
		ht.Insert(i)
	}
	fmt.Printf("len=%d capacity=%d saturation=%.3f\n", ht.Len(), ht.Capacity(), ht.Saturation())
	// Output:
	// len=128 capacity=256 saturation=0.500
}

// All yields (slot position, element) pairs; a single-variable range yields
// the slot position, not the element.
func ExampleHashTab_All() {
	ht := cuckoo.NewHashTab[int](9, 0, 0)
	for _, v := range []int{10, 20, 30} {
		ht.Insert(v)
	}

	positions := []int{}
	for pos := range ht.All() { // pos is the slot index
		positions = append(positions, pos)
	}
	sort.Ints(positions)
	fmt.Println("element count:", ht.Len(), "distinct slots:", len(positions))
	// Output:
	// element count: 3 distinct slots: 3
}

// Walk visits each element in slot order, passing the slot position and the
// element to the callback; the closure captures the caller's state.
// Returning false stops the walk (Walk then returns false itself).
func ExampleHashTab_Walk() {
	ht := cuckoo.NewHashTab[string](8, 0, 0)
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
