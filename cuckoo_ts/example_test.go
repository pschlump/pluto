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
package cuckoo_ts_test

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/pschlump/pluto/cuckoo_ts"
)

// A basic set-membership table of strings shared between goroutines: no
// methods to implement, the builtin == decides equality, every operation is
// guarded by the table's lock.
func Example() {
	ht := cuckoo_ts.NewHashTab[string](16, 0, 0) // size 16, default thresholds 0.85/0.20

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

	byID := cuckoo_ts.NewHashTabFunc(
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

	// Iteration is over a snapshot taken when Values is called, so the loop
	// body may safely touch the table; sort when order matters.
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

// Lock plus the Nl-prefixed methods make a multi-step operation atomic —
// here a search-then-delete that must run as one step against other
// goroutines.
func ExampleHashTab_Lock() {
	ht := cuckoo_ts.NewHashTab[string](8, 0, 0)
	ht.Insert("job-42")

	ht.Lock()
	if _, found := ht.NlSearch("job-42"); found {
		ht.NlDelete("job-42") // no other goroutine can slip in between
	}
	ht.Unlock()

	fmt.Println("len:", ht.Len())
	// Output:
	// len: 0
}
