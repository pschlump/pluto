// Copyright (C) Philip Schlump, 2012-2026.
// BSD 3 Clause Licensed.
package fib_heap_test

import (
	"fmt"

	"github.com/pschlump/pluto/fib_heap"
)

func Example() {
	q := fib_heap.NewIndexFibonacciPQ[string](4)
	q.Insert(0, "delta")
	q.Insert(1, "alpha")
	q.Insert(2, "charlie")

	k, v, _ := q.Peek()
	fmt.Println(k, v)

	q.DecreaseKey(0, "aaa") // now index 0 holds the minimum
	k, v, _ = q.Peek()
	fmt.Println(k, v)

	q.Insert(3, "bravo")
	for k, v := range q.All() { // priority order, snapshot
		fmt.Println(k, v)
	}
	// Output:
	// 1 alpha
	// 0 aaa
	// 0 aaa
	// 1 alpha
	// 3 bravo
	// 2 charlie
}

func ExampleIndexFibonacciPQ_Change() {
	q := fib_heap.NewIndexFibonacciPQ[int](4)
	for i, v := range []int{10, 20, 30, 40} {
		q.Insert(i, v)
	}

	q.Change(0, 99) // increase: index 0 moves to the back of the order
	q.Change(1, 5)  // decrease: index 1 becomes the minimum

	for {
		k, v, ok := q.Pop()
		if !ok {
			break
		}
		fmt.Println(k, v)
	}
	// Output:
	// 1 5
	// 2 30
	// 3 40
	// 0 99
}

func ExampleNewIndexFibonacciPQFunc() {
	type task struct {
		priority int
		name     string
	}
	q := fib_heap.NewIndexFibonacciPQFunc(2, func(a, b task) int {
		return fib_heap.Compare(a.priority, b.priority)
	})
	q.Insert(0, task{3, "write"})
	q.Insert(1, task{1, "review"})
	for _, v := range q.All() {
		fmt.Println(v.name)
	}
	// Output:
	// review
	// write
}

func ExampleIndexFibonacciPQ_DecreaseKey() {
	// The Dijkstra shape: distances decrease as shorter paths are found.
	q := fib_heap.NewIndexFibonacciPQ[int](3)
	q.Insert(0, 7) // tentative distances to three vertices
	q.Insert(1, 5)
	q.Insert(2, 8)

	if q.DecreaseKey(2, 4) { // a shorter path to vertex 2 was found
		k, v, _ := q.Peek()
		fmt.Println("minimum is now", k, v)
	}
	fmt.Println(q.DecreaseKey(1, 9)) // not a decrease: refused, nothing changes
	// Output:
	// minimum is now 2 4
	// false
}
