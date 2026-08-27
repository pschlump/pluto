/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package queue_dll_test

import (
	"errors"
	"fmt"

	"github.com/pschlump/pluto/queue_dll"
)

// A FIFO queue on a doubly linked list: elements come out in exactly
// the order they went in, and — unlike the slice-based queue — pushes
// and dequeues never reallocate.  The zero value is ready to use — no
// constructor, no constraints on the element type.
func Example() {
	var q queue_dll.Queue[int]

	for _, job := range []int{10, 20, 30} {
		q.Push(job)
	}

	for !q.IsEmpty() {
		v, _ := q.Dequeue()
		fmt.Println(v)
	}
	// Output:
	// 10
	// 20
	// 30
}

// Push and Enqueue are aliases; both add at the tail.
func ExampleQueue_Push() {
	var q queue_dll.Queue[string]
	q.Push("first")
	q.Enqueue("second")

	for _, v := range q.All() {
		fmt.Println(v)
	}
	// Output:
	// first
	// second
}

// ErrEmptyQueue reports the drained queue; compare with errors.Is.
func ExampleQueue_Dequeue() {
	var q queue_dll.Queue[int]
	if _, err := q.Dequeue(); errors.Is(err, queue_dll.ErrEmptyQueue) {
		fmt.Println("queue is empty")
	}

	q.Push(42)
	v, err := q.Dequeue()
	fmt.Println(v, err)
	// Output:
	// queue is empty
	// 42 <nil>
}

// Peek returns the head — the next element to be dequeued — by value:
// mutating it cannot affect the queue.
func ExampleQueue_Peek() {
	type task struct{ ID int }

	var q queue_dll.Queue[task]
	q.Push(task{ID: 1})
	q.Push(task{ID: 2})

	head, _ := q.Peek()
	head.ID = 99 // mutate the returned value

	next, _ := q.Peek()
	fmt.Println(head.ID != next.ID, next.ID, q.Len())
	// Output:
	// true 1 2
}

// All iterates head to tail (dequeue order); Backward iterates tail to
// head with indexes that match the ones All assigns to the same
// element.  Both walk the live list, so the queue must not be modified
// until the loops finish.
func ExampleQueue_All() {
	var q queue_dll.Queue[string]
	for _, s := range []string{"a", "b", "c"} {
		q.Push(s)
	}

	for i, v := range q.Backward() {
		fmt.Println(i, v)
	}

	// Modify only after the iterators are done.
	for !q.IsEmpty() {
		_, _ = q.Dequeue()
	}
	fmt.Println(q.IsEmpty())
	// Output:
	// 2 c
	// 1 b
	// 0 a
	// true
}

// Truncate drops every element in O(1); the queue is immediately
// reusable.
func ExampleQueue_Truncate() {
	var q queue_dll.Queue[int]
	for i := range 5 {
		q.Push(i)
	}

	q.Truncate()
	fmt.Println(q.Len(), q.IsEmpty())

	q.Push(9)
	fmt.Println(q.Len())
	// Output:
	// 0 true
	// 1
}
