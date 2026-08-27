/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package stack_sll_ts_test

import (
	"errors"
	"fmt"
	"sync"

	"github.com/pschlump/pluto/stack_sll_ts"
)

// Writers from many goroutines share one stack; the accounting balances
// once everyone finishes.
func Example() {
	var stk stack_sll_ts.Stack[int]

	const workers = 4
	const perWorker = 25

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range perWorker {
				stk.Push(1)
			}
		})
	}
	wg.Wait()
	fmt.Println(stk.Len(), stk.IsEmpty())

	// The exact interleaving of the writers is nondeterministic; the total
	// drain is not.
	drained := 0
	for {
		if _, err := stk.Pop(); err != nil {
			break
		}
		drained++
	}
	fmt.Println(drained, stk.IsEmpty())
	// Output:
	// 100 false
	// 100 true
}

// A LIFO stack: last in, first out.  The zero value is ready to use —
// no constructor, no constraints on the element type.
func ExampleStack_Push() {
	var stk stack_sll_ts.Stack[string]
	stk.Push("first")
	stk.Push("second")

	top, _ := stk.Peek()
	fmt.Println(top)

	v, _ := stk.Pop()
	fmt.Println(v, stk.Len())
	// Output:
	// second
	// second 1
}

// ErrEmptyStack reports the drained stack; compare with errors.Is.
func ExampleStack_Pop() {
	var stk stack_sll_ts.Stack[int]
	if _, err := stk.Pop(); errors.Is(err, stack_sll_ts.ErrEmptyStack) {
		fmt.Println("stack is empty")
	}

	stk.Push(42)
	v, err := stk.Pop()
	fmt.Println(v, err)
	// Output:
	// stack is empty
	// 42 <nil>
}

// Peek returns the top by value: mutating it cannot affect the stack,
// and it is not invalidated by a later Push or Pop.
func ExampleStack_Peek() {
	type task struct{ ID int }

	var stk stack_sll_ts.Stack[task]
	stk.Push(task{ID: 1})
	stk.Push(task{ID: 2})

	v, _ := stk.Peek()
	v.ID = 99 // mutate the returned value

	next, _ := stk.Peek()
	fmt.Println(next.ID, stk.Len())
	// Output:
	// 2 2
}

// All and Backward iterate over a snapshot taken when they are called,
// so it is safe to mutate the stack from inside the loop.  All numbers
// from 0 at the top; Backward from 0 at the bottom.
func ExampleStack_All() {
	var stk stack_sll_ts.Stack[string]
	for _, s := range []string{"a", "b", "c"} {
		stk.Push(s) // pushed a, then b, then c — the top is c
	}

	for i, v := range stk.All() {
		fmt.Println(i, v)
	}

	// Popping from inside the loop is safe: All walks a snapshot.
	for _, v := range stk.All() {
		_, _ = stk.Pop()
		_ = v
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
	var stk stack_sll_ts.Stack[int]
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
