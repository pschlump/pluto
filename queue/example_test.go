/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package queue_test

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pschlump/pluto/queue"
)

// A FIFO queue: first in, first out.
func Example() {
	var q queue.Queue[string]
	q.Enqueue("first")
	q.Enqueue("second")
	q.Enqueue("third")

	head, _ := q.Dequeue()
	next, _ := q.Dequeue()
	fmt.Println(head, next)
	fmt.Println(q.Len(), q.IsEmpty())
	// Output:
	// first second
	// 1 false
}

// The element type needs no constraints at all — no ordering, no
// equality — and the zero value is ready to use without a constructor.
func ExampleQueue_Push() {
	var q queue.Queue[int]
	for i := range 5 {
		q.Push(i)
	}

	for !q.IsEmpty() {
		v, _ := q.Dequeue()
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output:
	// 0 1 2 3 4
}

// Peek looks at the head without removing it; Pop removes the head
// without returning it.
func ExampleQueue_Peek() {
	var q queue.Queue[int]
	q.Push(1)
	q.Push(2)

	v, _ := q.Peek()
	fmt.Println(v, q.Len())

	if err := q.Pop(); err != nil {
		fmt.Println("pop failed")
	}
	v, _ = q.Peek()
	fmt.Println(v, q.Len())
	// Output:
	// 1 2
	// 2 1
}

// ErrEmptyQueue reports the drained queue; compare with errors.Is.
func ExampleQueue_Dequeue() {
	var q queue.Queue[int]
	if _, err := q.Dequeue(); errors.Is(err, queue.ErrEmptyQueue) {
		fmt.Println("queue is empty")
	}

	q.Push(42)
	v, err := q.Dequeue()
	fmt.Println(v, err)
	// Output:
	// queue is empty
	// 42 <nil>
}

// Dequeue returns the element by value: mutating it cannot affect the
// queue.
func ExampleQueue_Dequeue_copy() {
	type task struct {
		ID int
	}

	var q queue.Queue[task]
	q.Push(task{ID: 1})
	q.Push(task{ID: 2})

	v, _ := q.Dequeue()
	v.ID = 99 // mutate the returned value

	next, _ := q.Dequeue()
	fmt.Println(next.ID)
	// Output:
	// 2
}

// All iterates head to tail with the index; Backward iterates tail to
// head.
func ExampleQueue_All() {
	var q queue.Queue[string]
	for _, s := range []string{"a", "b", "c"} {
		q.Push(s)
	}

	for i, v := range q.All() {
		fmt.Println(i, v)
	}
	for i, v := range q.Backward() {
		fmt.Println(i, v)
	}
	// Output:
	// 0 a
	// 1 b
	// 2 c
	// 2 c
	// 1 b
	// 0 a
}

// Truncate drops every element in O(1); the queue is immediately
// reusable.
func ExampleQueue_Truncate() {
	var q queue.Queue[int]
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

// MarshalJSON encodes the queue as a JSON array of its elements, head to
// tail.
func ExampleQueue_MarshalJSON() {
	var q queue.Queue[int]
	q.Push(3)
	q.Push(1)
	q.Push(2)

	b, err := json.Marshal(&q)
	fmt.Println(string(b), err)
	// Output:
	// [3,1,2] <nil>
}

// UnmarshalJSON replaces the contents of the queue from a JSON array;
// element 0 becomes the new head.
func ExampleQueue_UnmarshalJSON() {
	var q queue.Queue[string]
	if err := json.Unmarshal([]byte(`["c","a"]`), &q); err != nil {
		fmt.Println("error:", err)
		return
	}
	for i, v := range q.All() {
		fmt.Println(i, v)
	}
	// Output:
	// 0 c
	// 1 a
}
