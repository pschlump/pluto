package bag_ts_test

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"slices"
	"sync"

	"github.com/pschlump/pluto/bag_ts"
)

func Example() {
	var b bag_ts.Bag[int]
	var wg sync.WaitGroup

	// Two goroutines collect into the same bag.
	for g := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 3 {
				b.Add(g*10 + i)
			}
		}()
	}
	wg.Wait()

	fmt.Println(b.Len()) // 6 — every concurrent Add landed exactly once

	// The snapshot iterator is safe to use while others mutate; the
	// cross-goroutine order is not deterministic, so sort before
	// printing.
	var vals []int
	for _, v := range b.All() {
		vals = append(vals, v)
	}
	slices.Sort(vals)
	fmt.Println(vals)
	// Output:
	// 6
	// [0 1 2 10 11 12]
}

func ExampleBag_Add() {
	var b bag_ts.Bag[string]
	b.Add("dash")
	b.Add("dot")
	b.Add("dash")
	fmt.Println(b.Length(), b.IsEmpty())
	// Output:
	// 3 false
}

func ExampleBag_All() {
	var b bag_ts.Bag[int]
	b.Add(1)
	b.Add(2)
	b.Add(3)

	// All iterates the call-time snapshot in insertion order: element 0
	// is the first added.
	for i, v := range b.All() {
		fmt.Println(i, v)
	}
	// Output:
	// 0 1
	// 1 2
	// 2 3
}

func ExampleBag_Backward() {
	var b bag_ts.Bag[int]
	b.Add(1)
	b.Add(2)
	b.Add(3)

	// Backward iterates the call-time snapshot from the most recently
	// added; the indexes count down from Length()-1.
	for i, v := range b.Backward() {
		fmt.Println(i, v)
	}
	// Output:
	// 2 3
	// 1 2
	// 0 1
}

func ExampleBag_Truncate() {
	var b bag_ts.Bag[int]
	b.Add(1)
	b.Add(2)
	b.Truncate()
	fmt.Println(b.Length(), b.IsEmpty())
	// Output:
	// 0 true
}
