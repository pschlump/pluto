/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package dll_test

import (
	"errors"
	"fmt"

	"github.com/pschlump/charon/dll"
)

// A stack: Push at the head, Peek and Pop from the head.
func Example() {
	stack := dll.NewDll[string]()
	stack.Push("first")
	stack.Push("second")
	stack.Push("third")

	top, err := stack.Peek()
	fmt.Println(top, err)

	v, _ := stack.Pop()
	fmt.Println(v)
	v, _ = stack.Pop()
	fmt.Println(v)

	fmt.Println(stack.Len(), stack.IsEmpty())

	_, _ = stack.Pop() // remove the last element
	if _, err := stack.Peek(); errors.Is(err, dll.ErrEmptyDll) {
		fmt.Println("stack is empty")
	}
	// Output:
	// third <nil>
	// third
	// second
	// 1 false
	// stack is empty
}

// Element types comparable with == need no equality function at all.
func ExampleNewDll() {
	list := dll.NewDll[int]()
	for _, v := range []int{3, 1, 2} {
		list.AppendAtTail(v)
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
func ExampleNewDllFunc() {
	type Task struct {
		ID   int
		Name string
	}

	byID := dll.NewDllFunc(func(a, b Task) bool { return a.ID == b.ID })
	byID.AppendAtTail(Task{ID: 1, Name: "write"})
	byID.AppendAtTail(Task{ID: 2, Name: "review"})
	byID.AppendAtTail(Task{ID: 3, Name: "ship"})

	// A search probe only needs the fields the equality function reads.
	if el, _ := byID.Search(Task{ID: 2}); el != nil {
		fmt.Println(el.GetData().Name)
	}
	// Output:
	// review
}

// A queue: Enqueue at the tail, Pop from the head — first in, first out.
func ExampleDll_Enqueue() {
	queue := dll.NewDll[string]()
	queue.Enqueue("first")
	queue.Enqueue("second")
	queue.Enqueue("third")

	head, _ := queue.Pop()
	next, _ := queue.Pop()
	fmt.Println(head, next)

	tail, _ := queue.PeekTail()
	fmt.Println(tail)
	// Output:
	// first second
	// third
}

// All iterates head to tail with the index; Backward iterates tail to
// head with the index counted from the head.
func ExampleDll_All() {
	list := dll.NewDll[string]()
	for _, s := range []string{"a", "b", "c"} {
		list.AppendAtTail(s)
	}

	for i, v := range list.All() {
		fmt.Println(i, v)
	}
	for i, v := range list.Backward() {
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

// The legacy iterator walks in either direction and reports positions.
func ExampleDll_Front() {
	list := dll.NewDll[string]()
	for _, s := range []string{"a", "b", "c"} {
		list.AppendAtTail(s)
	}

	var fwd []string
	for it := list.Front(); !it.Done(); it.Next() {
		v, _ := it.Value()
		fwd = append(fwd, v)
	}
	var bwd []string
	for it := list.Rear(); !it.Done(); it.Prev() {
		v, _ := it.Value()
		bwd = append(bwd, v)
	}
	fmt.Println(fwd)
	fmt.Println(bwd)
	// Output:
	// [a b c]
	// [c b a]
}

// Search returns the element and its position; DeleteFound removes that
// element in O(1) without a second search.
func ExampleDll_Search() {
	list := dll.NewDll[string]()
	for _, s := range []string{"x", "y", "z"} {
		list.AppendAtTail(s)
	}

	el, pos := list.Search("y")
	fmt.Println(pos)
	if err := list.DeleteFound(el); err == nil {
		fmt.Println("deleted")
	}

	var rest []string
	for _, v := range list.All() {
		rest = append(rest, v)
	}
	fmt.Println(rest)
	// Output:
	// 1
	// deleted
	// [x z]
}

// Reverse flips the list in place with O(1) extra storage.
func ExampleDll_Reverse() {
	list := dll.NewDll[int]()
	for _, v := range []int{1, 2, 3} {
		list.AppendAtTail(v)
	}

	list.Reverse()

	var got []int
	for _, v := range list.All() {
		got = append(got, v)
	}
	fmt.Println(got)
	// Output:
	// [3 2 1]
}

// Concat appends a copy of another list; the source is unchanged.
func ExampleDll_Concat() {
	a := dll.NewDll[int]()
	for _, v := range []int{1, 2} {
		a.AppendAtTail(v)
	}
	b := dll.NewDll[int]()
	for _, v := range []int{3, 4} {
		b.AppendAtTail(v)
	}

	a.Concat(b)

	var got []int
	for _, v := range a.All() {
		got = append(got, v)
	}
	fmt.Println(got, b.Len())
	// Output:
	// [1 2 3 4] 2
}

// Trim keeps the first n elements; TrimTail keeps the last n.
func ExampleDll_Trim() {
	list := dll.NewDll[int]()
	for i := range 5 {
		list.AppendAtTail(i)
	}

	_ = list.Trim(2)

	var got []int
	for _, v := range list.All() {
		got = append(got, v)
	}
	fmt.Println(got)

	for i := range 5 {
		list.AppendAtTail(10 + i)
	}
	_ = list.TrimTail(3)
	got = nil
	for _, v := range list.All() {
		got = append(got, v)
	}
	fmt.Println(got)
	// Output:
	// [0 1]
	// [12 13 14]
}
