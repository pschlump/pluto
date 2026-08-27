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
package hash_grow_ts_test

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/pschlump/pluto/hash_grow_ts"
)

// A thread-safe set-membership table of strings, safe to share between
// goroutines.
func Example() {
	ht := hash_grow_ts.NewHashTab[string](16, 0) // initial size 16, default saturation 0.5

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

	byID := hash_grow_ts.NewHashTabFunc(
		func(a, b User) bool { return a.ID == b.ID },
		func(u User) uint64 {
			h := fnv.New64a()
			_, _ = fmt.Fprint(h, u.ID)
			return h.Sum64()
		},
		8, 0, // initial size 8, default saturation 0.5
	)

	byID.Insert(User{ID: 1, Name: "write"})
	byID.Insert(User{ID: 2, Name: "review"})
	byID.Insert(User{ID: 1, Name: "write it again"}) // same ID: replaces

	if u, found := byID.Search(User{ID: 2}); found {
		fmt.Println("found user:", u.Name)
	}

	// Iteration is over a snapshot taken when Values is called — safe to
	// call other table methods from the loop body.  Bucket order is
	// hash-dependent, so sort when order matters.
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

// Lock plus the Nl-prefixed methods run a compound multi-step operation
// atomically — here a search followed by a delete that no other goroutine
// can slip a mutation between.
func ExampleHashTab_Lock() {
	ht := hash_grow_ts.NewHashTab[string](8, 0)
	ht.Insert("one")
	ht.Insert("two")

	ht.Lock()
	if _, found := ht.NlSearch("one"); found {
		ht.NlDelete("one")
	}
	n := ht.NlLen()
	ht.Unlock()

	fmt.Println("deleted one,", n, "elements remained, len now", ht.Len())
	// Output:
	// deleted one, 1 elements remained, len now 1
}
