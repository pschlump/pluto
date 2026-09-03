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
package hash_tab_ts_test

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/pschlump/pluto/hash_tab_ts"
)

// A basic set-membership table of strings shared between goroutines: no
// methods to implement, the builtin == decides equality, and every
// operation takes the table lock.
func Example() {
	ht := hash_tab_ts.NewHashTab[string](16) // 16 buckets, fixed for the life of the table

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

	byID := hash_tab_ts.NewHashTabFunc(
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

// All yields (bucket position, element) pairs over a snapshot copied under
// the read lock when All is called — it is safe to mutate the table from
// inside the loop.
func ExampleHashTab_All() {
	ht := hash_tab_ts.NewHashTab[int](9)
	for _, v := range []int{10, 20, 30} {
		ht.Insert(v)
	}

	n := 0
	for pos := range ht.All() { // pos is the bucket index
		_ = pos
		ht.Delete(20) // mutating inside the loop is safe (runs once)
		n++
	}
	fmt.Println("iterated:", n, "remaining:", ht.Len())
	// Output:
	// iterated: 3 remaining: 2
}

// Lock plus the Nl-prefixed methods run a multi-step operation atomically
// under one lock hold — here a search-then-delete that no other goroutine
// can interleave with.
func ExampleHashTab_Lock() {
	ht := hash_tab_ts.NewHashTab[string](8)
	ht.Insert("one")
	ht.Insert("two")
	ht.Insert("three")

	ht.Lock() // write lock; use only the Nl* methods while holding it
	if _, found := ht.NlSearch("two"); found {
		ht.NlDelete("two")
	}
	n := ht.NlLen()
	ht.Unlock()

	fmt.Println("len:", n, "two gone:", func() bool {
		_, found := ht.Search("two")
		return !found
	}())
	// Output:
	// len: 2 two gone: true
}

// Walk visits each element in bucket order, passing the bucket position and
// the element to the callback; within a bucket the elements come out newest
// first.  The read lock is held for the whole walk, so the callback must
// not touch the table — use All for that.
func ExampleHashTab_Walk() {
	ht := hash_tab_ts.NewHashTab[string](8)
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

// MarshalJSON encodes the table as a JSON array of its elements.  The array
// is in bucket order (hash-dependent), so this example decodes and sorts it
// before printing.
func ExampleHashTab_MarshalJSON() {
	ht := hash_tab_ts.NewHashTab[string](8)
	ht.Insert("red")
	ht.Insert("green")
	ht.Insert("blue")

	b, err := json.Marshal(ht)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	var elems []string
	if err := json.Unmarshal(b, &elems); err != nil {
		fmt.Println("error:", err)
		return
	}
	sort.Strings(elems)
	fmt.Println(strings.Join(elems, ","))
	// Output:
	// blue,green,red
}

// UnmarshalJSON replaces the contents of the table from a JSON array.  The
// table must be created first, so the equality and hash functions survive.
func ExampleHashTab_UnmarshalJSON() {
	ht := hash_tab_ts.NewHashTab[string](8)
	if err := json.Unmarshal([]byte(`["c","a","b"]`), ht); err != nil {
		fmt.Println("error:", err)
		return
	}
	var elems []string
	for v := range ht.Values() { // bucket order — sort when order matters
		elems = append(elems, v)
	}
	sort.Strings(elems)
	fmt.Println(strings.Join(elems, ","))
	fmt.Println("len:", ht.Len())
	// Output:
	// a,b,c
	// len: 3
}
