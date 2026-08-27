/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package queue_ts_test

import (
	"errors"
	"fmt"
	"sync"

	"github.com/pschlump/pluto/queue_ts"
)

// Producers and consumers share one queue; the accounting balances once
// everyone finishes.
func Example() {
	var q queue_ts.Queue[int]

	const producers = 4
	const perProducer = 25
	const total = producers * perProducer

	var wg sync.WaitGroup
	var received int64
	var mu sync.Mutex

	// Consumers drain until the expected total has flowed through.
	for range 2 {
		wg.Go(func() {
			for {
				v, err := q.Dequeue()
				if err != nil {
					mu.Lock()
					done := received == int64(total)
					mu.Unlock()
					if done {
						return
					}
					continue
				}
				mu.Lock()
				if v > 0 {
					received++
				}
				mu.Unlock()
			}
		})
	}

	// Producers enqueue.
	for range producers {
		wg.Go(func() {
			for range perProducer {
				q.Enqueue(1)
			}
		})
	}
	wg.Wait()

	fmt.Println(received, q.IsEmpty())
	// Output:
	// 100 true
}

// A FIFO queue: first in, first out.  The zero value is ready to use —
// no constructor, no constraints on the element type.
func ExampleQueue() {
	var q queue_ts.Queue[string]
	q.Enqueue("first")
	q.Enqueue("second")

	head, _ := q.Dequeue()
	fmt.Println(head, q.Len())
	// Output:
	// first 1
}

// Peek returns the head by value — an independent copy taken under the
// read lock.
func ExampleQueue_Peek() {
	var q queue_ts.Queue[int]
	q.Push(1)
	q.Push(2)

	v, _ := q.Peek()
	fmt.Println(v, q.Len())
	// Output:
	// 1 2
}

// ErrEmptyQueue reports the drained queue; compare with errors.Is.
func ExampleQueue_Dequeue() {
	var q queue_ts.Queue[int]
	if _, err := q.Dequeue(); errors.Is(err, queue_ts.ErrEmptyQueue) {
		fmt.Println("queue is empty")
	}

	q.Push(42)
	v, err := q.Dequeue()
	fmt.Println(v, err)
	// Output:
	// queue is empty
	// 42 <nil>
}

// All and Backward iterate over a snapshot taken when they are called,
// so it is safe to mutate the queue from inside the loop.
func ExampleQueue_All() {
	var q queue_ts.Queue[string]
	for _, s := range []string{"a", "b", "c"} {
		q.Push(s)
	}

	for i, v := range q.All() {
		fmt.Println(i, v)
	}

	// Deleting from inside the loop is safe: All walks a snapshot.
	for _, v := range q.All() {
		_ = q.Pop()
		_ = v
	}
	fmt.Println(q.IsEmpty())
	// Output:
	// 0 a
	// 1 b
	// 2 c
	// true
}

// Truncate drops every element in O(1); the queue is immediately
// reusable.
func ExampleQueue_Truncate() {
	var q queue_ts.Queue[int]
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
