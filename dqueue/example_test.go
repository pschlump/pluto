/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package dqueue_test

import (
	"errors"
	"fmt"

	"github.com/pschlump/pluto/dqueue"
)

// A double ended queue used both ways: PushBack+PopFront is a FIFO,
// PushBack+PopBack is a stack.  The zero value is ready to use — no
// constructor, no constraints on the element type.
func Example() {
	var q dqueue.Deque[int]

	// FIFO: the first element in is the first element out.
	q.PushBack(1)
	q.PushBack(2)
	q.PushBack(3)
	for !q.IsEmpty() {
		v, _ := q.PopFront()
		fmt.Println("fifo:", v)
	}

	// The same deque as a LIFO stack, from the other end.
	q.PushBack(1)
	q.PushBack(2)
	v, _ := q.PopBack()
	fmt.Println("lifo:", v, q.Len())
	// Output:
	// fifo: 1
	// fifo: 2
	// fifo: 3
	// lifo: 2 1
}

// PushFront and PushBack insert at opposite ends; All iterates front to
// back.
func ExampleDeque_PushFront() {
	var q dqueue.Deque[string]
	q.PushBack("b")
	q.PushFront("a") // a goes on the front, ahead of b
	q.PushBack("c")

	for _, v := range q.All() {
		fmt.Println(v)
	}
	// Output:
	// a
	// b
	// c
}

// ErrEmptyDeque reports the drained deque; compare with errors.Is.
func ExampleDeque_PopFront() {
	var q dqueue.Deque[int]
	if _, err := q.PopFront(); errors.Is(err, dqueue.ErrEmptyDeque) {
		fmt.Println("deque is empty")
	}

	q.PushBack(42)
	v, err := q.PopBack()
	fmt.Println(v, err)
	// Output:
	// deque is empty
	// 42 <nil>
}

// PeekFront and PeekBack return values by value: mutating them cannot
// affect the deque, and they are not invalidated by a later push or pop.
func ExampleDeque_PeekFront() {
	type task struct{ ID int }

	var q dqueue.Deque[task]
	q.PushBack(task{ID: 1})
	q.PushBack(task{ID: 2})

	front, _ := q.PeekFront()
	back, _ := q.PeekBack()
	front.ID = 99 // mutate the returned value

	next, _ := q.PeekFront()
	fmt.Println(front.ID != next.ID, back.ID, q.Len())
	// Output:
	// true 2 2
}

// All iterates front to back; Backward iterates back to front with
// indexes that match the ones All assigns to the same element.  Both
// walk the live list, so the deque must not be modified until the loops
// finish — the dqueue_ts twin snapshots instead and may be mutated from
// inside the loop.
func ExampleDeque_All() {
	var q dqueue.Deque[string]
	for _, s := range []string{"a", "b", "c"} {
		q.PushBack(s)
	}

	for i, v := range q.Backward() {
		fmt.Println(i, v)
	}

	// Modify only after the iterators are done.
	for !q.IsEmpty() {
		_, _ = q.PopFront()
	}
	fmt.Println(q.IsEmpty())
	// Output:
	// 2 c
	// 1 b
	// 0 a
	// true
}

// Truncate drops every element in O(1); the deque is immediately
// reusable.
func ExampleDeque_Truncate() {
	var q dqueue.Deque[int]
	for i := range 5 {
		q.PushBack(i)
	}

	q.Truncate()
	fmt.Println(q.Len(), q.IsEmpty())

	q.PushFront(9)
	fmt.Println(q.Len())
	// Output:
	// 0 true
	// 1
}
