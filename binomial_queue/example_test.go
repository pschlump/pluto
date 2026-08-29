/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package binomial_queue_test

import (
	"fmt"

	"github.com/pschlump/pluto/binomial_queue"
)

// A min-first priority queue: Insert adds, Peek sees the minimum,
// DeleteMin drains in ascending order.
func Example() {
	q := binomial_queue.NewBinomialQueue[int]()
	for _, v := range []int{42, 7, 13, 99, 55} {
		q.Insert(v)
	}

	min, _ := q.Peek()
	fmt.Println("minimum:", min)

	for q.Len() > 0 {
		v, _ := q.DeleteMin()
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output:
	// minimum: 7
	// 7 13 42 55 99
}

// A max-first priority queue: reverse the comparison function and the
// same structure drains in descending order.
func ExampleNewBinomialQueueFunc() {
	maxQ := binomial_queue.NewBinomialQueueFunc(func(a, b int) int {
		return -binomial_queue.Compare(a, b)
	})
	for _, v := range []int{5, 1, 9, 3} {
		maxQ.Insert(v)
	}

	for maxQ.Len() > 0 {
		v, _ := maxQ.DeleteMin()
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output:
	// 9 5 3 1
}

// Merge is the signature binomial-queue operation: one queue absorbs
// another in O(log n) — a binary heap cannot do this.
func ExampleBinomialQueue_Merge() {
	a := binomial_queue.NewBinomialQueue[int]()
	for _, v := range []int{8, 2, 6} {
		a.Insert(v)
	}
	b := binomial_queue.NewBinomialQueue[int]()
	for _, v := range []int{1, 9, 4} {
		b.Insert(v)
	}

	a.Merge(b) // b is now empty
	fmt.Println("merged:", a.Len(), "other:", b.Len())

	for a.Len() > 0 {
		v, _ := a.DeleteMin()
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output:
	// merged: 6 other: 0
	// 1 2 4 6 8 9
}

// A priority queue over structs: the element type implements no
// interface, and the exported Compare helper builds the field ordering.
func ExampleBinomialQueue_Insert() {
	type Task struct {
		Priority int
		Name     string
	}

	pq := binomial_queue.NewBinomialQueueFunc(func(a, b Task) int {
		return binomial_queue.Compare(a.Priority, b.Priority)
	})
	pq.Insert(Task{3, "write"})
	pq.Insert(Task{1, "review"})
	pq.Insert(Task{2, "ship"})

	for pq.Len() > 0 {
		v, _ := pq.DeleteMin()
		fmt.Printf("%d:%s ", v.Priority, v.Name)
	}
	fmt.Println()
	// Output:
	// 1:review 2:ship 3:write
}

// Truncate drops every element in O(1); the queue is immediately
// reusable.
func ExampleBinomialQueue_Truncate() {
	q := binomial_queue.NewBinomialQueue[int]()
	for i := range 5 {
		q.Insert(i)
	}

	q.Truncate()
	fmt.Println(q.Len(), q.IsEmpty())

	q.Insert(9)
	fmt.Println(q.Len())
	// Output:
	// 0 true
	// 1
}
