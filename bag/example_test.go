package bag_test

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"encoding/json"
	"fmt"

	"github.com/pschlump/pluto/bag"
)

func Example() {
	var b bag.Bag[string]
	b.Add("dash")
	b.Add("dot")
	b.Add("dash")

	fmt.Println(b.Len())
	for _, v := range b.All() {
		fmt.Println(v)
	}
	// Output:
	// 3
	// dash
	// dot
	// dash
}

func ExampleBag_Add() {
	var b bag.Bag[int]
	for _, v := range []int{10, 20, 30} {
		b.Add(v)
	}
	fmt.Println(b.Length(), b.IsEmpty())
	// Output:
	// 3 false
}

func ExampleBag_All() {
	var b bag.Bag[int]
	b.Add(1)
	b.Add(2)
	b.Add(3)

	// All iterates in insertion order: element 0 is the first added.
	for i, v := range b.All() {
		fmt.Println(i, v)
	}
	// Output:
	// 0 1
	// 1 2
	// 2 3
}

func ExampleBag_Backward() {
	var b bag.Bag[int]
	b.Add(1)
	b.Add(2)
	b.Add(3)

	// Backward iterates from the most recently added; the indexes count
	// down from Length()-1 so they match the index All assigns.
	for i, v := range b.Backward() {
		fmt.Println(i, v)
	}
	// Output:
	// 2 3
	// 1 2
	// 0 1
}

func ExampleBag_Truncate() {
	var b bag.Bag[int]
	b.Add(1)
	b.Add(2)
	b.Truncate()
	fmt.Println(b.Length(), b.IsEmpty())
	// Output:
	// 0 true
}

func ExampleBag_MarshalJSON() {
	var b bag.Bag[string]
	b.Add("a")
	b.Add("b")
	blob, err := json.Marshal(&b)
	fmt.Println(string(blob), err)
	// Output:
	// ["a","b"] <nil>
}

func ExampleBag_UnmarshalJSON() {
	var b bag.Bag[int]
	if err := json.Unmarshal([]byte(`[3,1,2]`), &b); err != nil {
		fmt.Println("error:", err)
		return
	}
	for _, v := range b.All() {
		fmt.Println(v)
	}
	// Output:
	// 3
	// 1
	// 2
}
