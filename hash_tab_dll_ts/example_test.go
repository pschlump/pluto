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
package hash_tab_dll_ts_test

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/pschlump/pluto/hash_tab_dll_ts"
)

// A basic set-membership table of strings shared by goroutines: no methods
// to implement, the builtin == decides equality.  Search returns a handle
// to the stored element — pass it to DeleteFound for an O(1) removal.
func Example() {
	ht := hash_tab_dll_ts.NewHashTab[string](16) // 16 buckets, fixed for the life of the table

	ht.Insert("alpha")
	ht.Insert("beta")
	replaced := ht.Insert("alpha") // duplicate: replaces, returns false

	if el, found := ht.Search("alpha"); found {
		fmt.Println("found", el.GetData())
	}
	fmt.Println("replaced:", replaced, "len:", ht.Len())

	if el, found := ht.Search("beta"); found {
		ht.DeleteFound(el) // O(1): splice out the located element
	}
	fmt.Println("after DeleteFound:", ht.Len(), ht.IsEmpty())
	// Output:
	// found alpha
	// replaced: false len: 2
	// after DeleteFound: 1 false
}

// Structs compared and hashed by a field — the element type implements no
// interface; equality and hashing are plain functions.
func ExampleNewHashTabFunc() {
	type User struct {
		ID   int
		Name string
	}

	byID := hash_tab_dll_ts.NewHashTabFunc(
		func(a, b User) bool { return a.ID == b.ID },
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
	if el, found := byID.Search(User{ID: 2}); found {
		fmt.Println("found user:", el.GetData().Name)
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

// Search locates an element and returns a live handle to it; DeleteFound
// splices that element out through its prev/next pointers in O(1) — the
// operation the doubly linked buckets exist for.  When other goroutines
// write to the table, use Lock + NlSearch + NlDeleteFound instead (see the
// Lock example) so the handle cannot go stale between the locate and the
// splice.
func ExampleHashTab_DeleteFound() {
	ht := hash_tab_dll_ts.NewHashTab[int](9)
	for _, v := range []int{10, 20, 30} {
		ht.Insert(v)
	}

	el, found := ht.Search(20)
	fmt.Println("found:", found)
	fmt.Println("deleted:", ht.DeleteFound(el))
	_, still := ht.Search(20)
	fmt.Println("still there:", still)
	// Output:
	// found: true
	// deleted: true
	// still there: false
}

// Lock takes the write lock for a compound sequence of Nl-prefixed
// operations.  This is the race-free form of the package's signature move —
// locate an element and splice it out in O(1), atomically — when the table
// is shared between goroutines and a handle returned by the standalone
// Search could go stale before its use.
func ExampleHashTab_Lock() {
	ht := hash_tab_dll_ts.NewHashTab[int](9)
	for v := range 20 {
		ht.Insert(v)
	}

	removed := 0
	ht.Lock()
	for v := range 10 {
		if el, found := ht.NlSearch(v); found {
			if ht.NlDeleteFound(el) { // O(1) splice, still under the lock
				removed++
			}
		}
	}
	n := ht.NlLen()
	ht.Unlock()

	fmt.Println("removed:", removed, "left:", n, "len:", ht.Len())
	// Output:
	// removed: 10 left: 10 len: 10
}

// All yields (bucket position, element) pairs over a snapshot copied when
// All is called; a single-variable range yields the bucket position, not
// the element.
func ExampleHashTab_All() {
	ht := hash_tab_dll_ts.NewHashTab[int](9)
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
// the element to the callback; the closure captures the caller's state.
// Returning false stops the walk (Walk then returns false itself).  The
// walk holds the read lock — do not call table methods from the callback.
func ExampleHashTab_Walk() {
	ht := hash_tab_dll_ts.NewHashTab[string](8)
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
