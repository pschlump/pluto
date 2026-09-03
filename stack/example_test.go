/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package stack_test

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pschlump/pluto/stack"
)

// A LIFO stack: last in, first out.  The zero value is ready to use —
// no constructor, no constraints on the element type.
func Example() {
	var stk stack.Stack[string]
	stk.Push("first")
	stk.Push("second")
	stk.Push("third")

	top, _ := stk.Peek()
	fmt.Println(top)

	v, _ := stk.Pop()
	fmt.Println(v)
	fmt.Println(stk.Len(), stk.IsEmpty())
	// Output:
	// third
	// third
	// 2 false
}

// The exact LIFO order of a drain.
func ExampleStack_Push() {
	var stk stack.Stack[int]
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
	var stk stack.Stack[int]
	if _, err := stk.Pop(); errors.Is(err, stack.ErrEmptyStack) {
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

	var stk stack.Stack[task]
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
// bottom; Backward iterates from the bottom to the top.  Both reflect
// the live stack.
func ExampleStack_All() {
	var stk stack.Stack[string]
	for _, s := range []string{"a", "b", "c"} {
		stk.Push(s) // pushed a, then b, then c — the top is c
	}

	for i, v := range stk.All() {
		fmt.Println(i, v)
	}
	for i, v := range stk.Backward() {
		fmt.Println(i, v)
	}
	// Output:
	// 0 c
	// 1 b
	// 2 a
	// 0 a
	// 1 b
	// 2 c
}

// Truncate drops every element in O(1); the stack is immediately
// reusable.
func ExampleStack_Truncate() {
	var stk stack.Stack[int]
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

// MarshalJSON encodes the stack as a JSON array of its elements, top to
// bottom (the most recently pushed element is element 0).
func ExampleStack_MarshalJSON() {
	var stk stack.Stack[int]
	stk.Push(1)
	stk.Push(2)
	stk.Push(3)

	b, err := json.Marshal(&stk)
	fmt.Println(string(b), err)
	// Output:
	// [3,2,1] <nil>
}

// UnmarshalJSON replaces the contents of the stack from a JSON array;
// element 0 becomes the new top.
func ExampleStack_UnmarshalJSON() {
	var stk stack.Stack[string]
	if err := json.Unmarshal([]byte(`["c","a"]`), &stk); err != nil {
		fmt.Println("error:", err)
		return
	}
	for i, v := range stk.All() {
		fmt.Println(i, v)
	}
	// Output:
	// 0 c
	// 1 a
}
