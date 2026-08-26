/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package dqueue_ts_test

import (
	"errors"
	"fmt"
	"sync"

	"github.com/pschlump/charon/dqueue_ts"
)

// Writers from many goroutines share one deque, pushing at both ends;
// the accounting balances once everyone finishes.
func Example() {
	var q dqueue_ts.Deque[int]

	const workers = 4
	const perWorker = 25

	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for range perWorker {
				if w%2 == 0 {
					q.PushFront(1)
				} else {
					q.PushBack(1)
				}
			}
		}(w)
	}
	wg.Wait()
	fmt.Println(q.Len(), q.IsEmpty())

	// The exact interleaving of the writers is nondeterministic; the total
	// drain is not.
	drained := 0
	for {
		if _, err := q.PopFront(); err != nil {
			break
		}
		drained++
	}
	fmt.Println(drained, q.IsEmpty())
	// Output:
	// 100 false
	// 100 true
}

// A double ended queue: push and pop at either end.  The zero value is
// ready to use — no constructor, no constraints on the element type.
func ExampleDeque_PushFront() {
	var q dqueue_ts.Deque[string]
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
	var q dqueue_ts.Deque[int]
	if _, err := q.PopFront(); errors.Is(err, dqueue_ts.ErrEmptyDeque) {
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

	var q dqueue_ts.Deque[task]
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
// walk a snapshot taken when they are called, so it is safe to mutate
// the deque from inside the loop.
func ExampleDeque_All() {
	var q dqueue_ts.Deque[string]
	for _, s := range []string{"a", "b", "c"} {
		q.PushBack(s)
	}

	for i, v := range q.Backward() {
		fmt.Println(i, v)
	}

	// Popping from inside the loop is safe: All walks a snapshot.
	for _, v := range q.All() {
		_, _ = q.PopFront()
		_ = v
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
	var q dqueue_ts.Deque[int]
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
