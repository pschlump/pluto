/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package stack_ts_test

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/pschlump/charon/stack_ts"
)

// Producers share one stack; every pushed element is popped exactly once.
func Example() {
	var stk stack_ts.Stack[int]

	const producers = 4
	const perProducer = 25
	const total = producers * perProducer

	var wg sync.WaitGroup
	var popped atomic.Int64

	// Consumers pop until the expected total has flowed through.
	for range 2 {
		wg.Go(func() {
			for {
				if _, err := stk.Pop(); err != nil {
					if popped.Load() >= int64(total) {
						return
					}
					continue
				}
				popped.Add(1)
			}
		})
	}

	// Producers push.
	for range producers {
		wg.Go(func() {
			for range perProducer {
				stk.Push(1)
			}
		})
	}
	wg.Wait()

	fmt.Println(popped.Load(), stk.IsEmpty())
	// Output:
	// 100 true
}

// A LIFO stack: last in, first out.  The zero value is ready to use —
// no constructor, no constraints on the element type.
func ExampleStack() {
	var stk stack_ts.Stack[string]
	stk.Push("first")
	stk.Push("second")

	top, _ := stk.Peek()
	fmt.Println(top)

	v, _ := stk.Pop()
	fmt.Println(v)
	fmt.Println(stk.Len(), stk.IsEmpty())
	// Output:
	// second
	// second
	// 1 false
}

// The exact LIFO order of a single-goroutine drain.
func ExampleStack_Push() {
	var stk stack_ts.Stack[int]
	for i := range 5 {
		stk.Push(i)
	}

	var drained []int
	for !stk.IsEmpty() {
		v, _ := stk.Pop()
		drained = append(drained, v)
	}
	fmt.Println(drained)
	// Output:
	// [4 3 2 1 0]
}

// ErrEmptyStack reports the drained stack; compare with errors.Is.
func ExampleStack_Pop() {
	var stk stack_ts.Stack[int]
	if _, err := stk.Pop(); errors.Is(err, stack_ts.ErrEmptyStack) {
		fmt.Println("stack is empty")
	}

	stk.Push(42)
	v, err := stk.Pop()
	fmt.Println(v, err)
	// Output:
	// stack is empty
	// 42 <nil>
}

// Peek returns the top by value: an independent copy taken under the
// read lock — mutating it cannot affect the stack, and it is not
// invalidated by a later Push or Pop.
func ExampleStack_Peek() {
	type task struct{ ID int }

	var stk stack_ts.Stack[task]
	stk.Push(task{ID: 1})
	stk.Push(task{ID: 2})

	v, _ := stk.Peek()
	v.ID = 99 // mutate the returned value

	next, _ := stk.Peek()
	fmt.Println(next.ID, stk.Len())
	// Output:
	// 2 2
}

// All iterates from the top (most recently pushed, index 0) to the
// bottom; Backward iterates from the bottom to the top.  Both walk a
// snapshot, so it is safe to pop from inside the loop.
func ExampleStack_All() {
	var stk stack_ts.Stack[string]
	for _, s := range []string{"a", "b", "c"} {
		stk.Push(s) // pushed a, then b, then c — the top is c
	}

	for i, v := range stk.All() {
		fmt.Println(i, v)
	}

	// Popping from inside the loop is safe: All walks a snapshot.
	for range stk.All() {
		_, _ = stk.Pop()
	}
	fmt.Println(stk.IsEmpty())
	// Output:
	// 0 c
	// 1 b
	// 2 a
	// true
}

// Truncate drops every element in O(1); the stack is immediately
// reusable.
func ExampleStack_Truncate() {
	var stk stack_ts.Stack[int]
	for i := range 5 {
		stk.Push(i)
	}

	stk.Truncate()
	fmt.Println(stk.Len(), stk.IsEmpty())

	stk.Push(9)
	fmt.Println(stk.Len())
	// Output:
	// 0 true
	// 1
}
