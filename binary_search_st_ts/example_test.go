/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_search_st_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/binary_search_st_ts"
)

func Example() {
	st := binary_search_st_ts.NewBinarySearchST[string, int]()
	st.Put("banana", 2)
	st.Put("apple", 1)
	st.Put("cherry", 3)
	st.Put("apple", 5) // a duplicate key replaces the value

	fmt.Println(st.Len())
	v, found := st.Get("apple")
	fmt.Println(v, found)
	fmt.Println(st.String())
	// Output:
	// 3
	// 5 true
	// [apple:5 banana:2 cherry:3]
}

func ExampleBinarySearchST_orderedRiders() {
	st := binary_search_st_ts.NewBinarySearchST[string, int]()
	for _, p := range []struct {
		k string
		v int
	}{{"a", 1}, {"e", 5}, {"i", 9}, {"o", 15}, {"u", 21}} {
		st.Put(p.k, p.v)
	}

	k, v, _ := st.Floor("h")   // greatest key <= h
	rank, _ := st.Rank("o")    // position among the stored keys
	k2, v2, _ := st.AtIndex(1) // rank's inverse
	fmt.Println(k, v, rank, k2, v2, st.CountRange("c", "p"))
	// Output:
	// e 5 3 e 5 3
}

func ExampleNewBinarySearchSTFunc() {
	type user struct {
		name string
		age  int
	}
	st := binary_search_st_ts.NewBinarySearchSTFunc[user, string](func(a, b user) int {
		return a.age - b.age // order by age; ties keep insertion order
	})
	st.Put(user{name: "ana", age: 30}, "ana's record")
	st.Put(user{name: "bo", age: 20}, "bo's record")
	k, _, _ := st.FindMin()
	fmt.Println(k.name)
	// Output:
	// bo
}

func ExampleBinarySearchST_iterators() {
	st := binary_search_st_ts.NewBinarySearchST[string, int]()
	for i, k := range []string{"pear", "fig", "kiwi"} {
		st.Put(k, i)
	}

	// All walks a SNAPSHOT: mutating the table inside the loop is safe.
	for k, v := range st.All() {
		fmt.Println(k, v)
		st.Delete(k)
	}
	fmt.Println(st.IsEmpty())
	// Output:
	// fig 1
	// kiwi 2
	// pear 0
	// true
}

func ExampleBinarySearchST_lock() {
	st := binary_search_st_ts.NewBinarySearchST[string, int]()

	// The canonical compound: the check-and-insert pair is atomic.
	st.Lock()
	if _, found := st.NlGet("ticket-1"); !found {
		st.NlPut("ticket-1", 42)
	}
	st.Unlock()

	v, _ := st.Get("ticket-1")
	fmt.Println(v)
	// Output:
	// 42
}
