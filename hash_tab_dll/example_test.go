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
package hash_tab_dll_test

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/pschlump/pluto/hash_tab_dll"
)

// A basic set-membership table of strings: no methods to implement, the
// builtin == decides equality.  Search returns a handle to the stored
// element — pass it to DeleteFound for an O(1) removal.
func Example() {
	ht := hash_tab_dll.NewHashTab[string](16) // 16 buckets, fixed for the life of the table

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

	byID := hash_tab_dll.NewHashTabFunc(
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
// operation the doubly linked buckets exist for.
func ExampleHashTab_DeleteFound() {
	ht := hash_tab_dll.NewHashTab[int](9)
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

// All yields (bucket position, element) pairs; a single-variable range
// yields the bucket position, not the element.
func ExampleHashTab_All() {
	ht := hash_tab_dll.NewHashTab[int](9)
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
// Returning false stops the walk (Walk then returns false itself).
func ExampleHashTab_Walk() {
	ht := hash_tab_dll.NewHashTab[string](8)
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

// MarshalJSON encodes the table as a JSON array of its elements, in bucket
// order — which depends on the hash function, so the order is normally not
// fixed.  With a constant hash every element chains in a single bucket,
// newest first, making the output deterministic for this example.
func ExampleHashTab_MarshalJSON() {
	ht := hash_tab_dll.NewHashTabFunc(
		func(a, b string) bool { return a == b },
		func(s string) uint64 { return 7 }, // constant hash: one chain
		5,
	)
	ht.Insert("alpha")
	ht.Insert("beta")

	b, err := json.Marshal(ht)
	fmt.Println(string(b), err)
	// Output:
	// ["beta","alpha"] <nil>
}

// UnmarshalJSON replaces the contents of the table from a JSON array; the
// table keeps its equality and hash functions, so it stays fully usable
// afterwards.  Element order in the table is bucket order (hash
// dependent), so the decoded set is sorted for printing here.
func ExampleHashTab_UnmarshalJSON() {
	ht := hash_tab_dll.NewHashTab[string](8)
	if err := json.Unmarshal([]byte(`["c","a","b"]`), ht); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("len:", ht.Len())

	got := []string{}
	for v := range ht.Values() {
		got = append(got, v)
	}
	sort.Strings(got)
	fmt.Println(got)
	// Output:
	// len: 3
	// [a b c]
}
