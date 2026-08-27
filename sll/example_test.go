/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package sll_test

import (
	"errors"
	"fmt"

	"github.com/pschlump/pluto/sll"
)

// A stack: Push at the head, Peek and Pop from the head.
func Example() {
	stack := sll.NewSll[string]()
	stack.Push("first")
	stack.Push("second")
	stack.Push("third")

	top, _ := stack.Peek()
	fmt.Println(top)

	v, _ := stack.Pop()
	fmt.Println(v)
	fmt.Println(stack.Len(), stack.IsEmpty())

	_, _ = stack.Pop() // "second"
	_, _ = stack.Pop() // "first" — the stack is now empty
	if _, err := stack.Peek(); errors.Is(err, sll.ErrEmptySll) {
		fmt.Println("stack is empty")
	}
	// Output:
	// third
	// third
	// 2 false
	// stack is empty
}

// Element types comparable with == need no equality function at all.
func ExampleNewSll() {
	list := sll.NewSll[int]()
	for _, v := range []int{3, 1, 2} {
		list.InsertAfterTail(v)
	}

	if _, pos := list.Search(2); pos == 2 {
		fmt.Println("found 2 at position 2")
	}
	if err := list.Delete(1); err == nil {
		fmt.Println("deleted 1")
	}
	fmt.Println(list.Len())
	// Output:
	// found 2 at position 2
	// deleted 1
	// 2
}

// Structs compared by a field — the element type implements no interface;
// equality is a plain function.
func ExampleNewSllFunc() {
	type Task struct {
		ID   int
		Name string
	}

	byID := sll.NewSllFunc(func(a, b Task) bool { return a.ID == b.ID })
	byID.InsertAfterTail(Task{ID: 1, Name: "write"})
	byID.InsertAfterTail(Task{ID: 2, Name: "review"})
	byID.InsertAfterTail(Task{ID: 3, Name: "ship"})

	// A search probe only needs the fields the equality function reads.
	if el, _ := byID.Search(Task{ID: 2}); el != nil {
		fmt.Println(el.GetData().Name)
	}
	// Output:
	// review
}

// InsertAfterTail appends at the tail — combined with Pop from the head
// this is first in, first out.
func ExampleSll_InsertAfterTail() {
	queue := sll.NewSll[string]()
	queue.InsertAfterTail("first")
	queue.InsertAfterTail("second")
	queue.InsertAfterTail("third")

	head, _ := queue.Pop()
	next, _ := queue.Pop()
	fmt.Println(head, next)
	// Output:
	// first second
}

// IterateOver yields index/value pairs from head to tail.
func ExampleSll_IterateOver() {
	list := sll.NewSll[string]()
	for _, s := range []string{"a", "b", "c"} {
		list.InsertAfterTail(s)
	}

	for i, v := range list.IterateOver() {
		fmt.Println(i, v)
	}
	// Output:
	// 0 a
	// 1 b
	// 2 c
}

// The cursor iterator reports positions as it walks.
func ExampleSll_Front() {
	list := sll.NewSll[string]()
	for _, s := range []string{"a", "b", "c"} {
		list.InsertAfterTail(s)
	}

	var fwd []string
	for it := list.Front(); !it.Done(); it.Next() {
		v, _ := it.Value()
		fwd = append(fwd, v)
	}
	fmt.Println(fwd)
	// Output:
	// [a b c]
}

// Search returns the element and its position; DeleteFound removes the
// element with that data.
func ExampleSll_Search() {
	list := sll.NewSll[string]()
	for _, s := range []string{"x", "y", "z"} {
		list.InsertAfterTail(s)
	}

	el, pos := list.Search("y")
	fmt.Println(pos)
	if err := list.DeleteFound(el); err == nil {
		fmt.Println("deleted")
	}

	var rest []string
	for _, v := range list.IterateOver() {
		rest = append(rest, v)
	}
	fmt.Println(rest)
	// Output:
	// 1
	// deleted
	// [x z]
}

// Reverse flips the list in place with O(1) extra storage.
func ExampleSll_Reverse() {
	list := sll.NewSll[int]()
	for _, v := range []int{1, 2, 3} {
		list.InsertAfterTail(v)
	}

	list.Reverse()

	var got []int
	for _, v := range list.IterateOver() {
		got = append(got, v)
	}
	fmt.Println(got)
	// Output:
	// [3 2 1]
}
