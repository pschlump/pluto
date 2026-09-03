/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package sll_ts_test

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/pschlump/pluto/sll_ts"
)

// Writers from many goroutines share one stack; readers see a consistent
// result once the writers finish.
func Example() {
	stack := sll_ts.NewSll[int]()

	var wg sync.WaitGroup
	for w := range 4 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range 25 {
				stack.Push(w*100 + i)
			}
		}(w)
	}
	wg.Wait()

	fmt.Println(stack.Len(), stack.IsEmpty())

	// The exact interleaving of the writers is nondeterministic; the total
	// drain is not.
	drained := 0
	for {
		if _, err := stack.Pop(); err != nil {
			break
		}
		drained++
	}
	fmt.Println(drained, stack.IsEmpty())
	// Output:
	// 100 false
	// 100 true
}

// A stack: Push at the head, Peek and Pop from the head.
func ExampleNewSll() {
	stack := sll_ts.NewSll[int]()
	stack.Push(1)
	stack.Push(2)
	stack.Push(3)

	top, _ := stack.Peek()
	fmt.Println(top)

	v, _ := stack.Pop()
	fmt.Println(v, stack.Len())
	// Output:
	// 3
	// 3 2
}

// Structs compared by a field — the element type implements no interface;
// equality is a plain function.
func ExampleNewSllFunc() {
	type Task struct {
		ID   int
		Name string
	}

	byID := sll_ts.NewSllFunc(func(a, b Task) bool { return a.ID == b.ID })
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
	queue := sll_ts.NewSll[string]()
	queue.InsertAfterTail("first")
	queue.InsertAfterTail("second")
	queue.InsertAfterTail("third")

	head, _ := queue.Pop()
	next, _ := queue.Pop()
	fmt.Println(head, next)
	// Output:
	// first second
}

// IterateOver yields index/value pairs from head to tail over a snapshot
// taken when it is called.
func ExampleSll_IterateOver() {
	list := sll_ts.NewSll[string]()
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

// Because IterateOver walks a snapshot, mutating the list from inside the
// loop is safe — the loop still visits every element captured at the
// start.
func ExampleSll_IterateOver_snapshot() {
	list := sll_ts.NewSll[string]()
	for _, s := range []string{"a", "b", "c"} {
		list.InsertAfterTail(s)
	}

	var visited []string
	for _, v := range list.IterateOver() {
		visited = append(visited, v)
		_ = list.Delete(v)
	}
	fmt.Println(visited, list.Len())
	// Output:
	// [a b c] 0
}

// The cursor iterator walks the live list.  Each method takes the list's
// read lock for the duration of that call, so it is race-free under
// concurrent use.
func ExampleSll_Front() {
	list := sll_ts.NewSll[string]()
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
	list := sll_ts.NewSll[int]()
	for _, v := range []int{10, 20, 30} {
		list.InsertAfterTail(v)
	}

	el, pos := list.Search(20)
	fmt.Println(pos)
	if err := list.DeleteFound(el); err == nil {
		fmt.Println("deleted")
	}

	var rest []int
	for _, v := range list.IterateOver() {
		rest = append(rest, v)
	}
	fmt.Println(rest)
	// Output:
	// 1
	// deleted
	// [10 30]
}

// Reverse flips the list in place with O(1) extra storage.
func ExampleSll_Reverse() {
	list := sll_ts.NewSll[int]()
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

// MarshalJSON encodes the list as a JSON array of its elements, head to
// tail.  It is safe to call concurrently with any list operation.
func ExampleSll_MarshalJSON() {
	list := sll_ts.NewSll[int]()
	list.InsertAfterTail(3)
	list.InsertAfterTail(1)
	list.InsertAfterTail(2)

	b, err := json.Marshal(list)
	fmt.Println(string(b), err)
	// Output:
	// [3,1,2] <nil>
}

// UnmarshalJSON replaces the contents of the list from a JSON array;
// element 0 becomes the new head.
func ExampleSll_UnmarshalJSON() {
	list := sll_ts.NewSll[string]()
	if err := json.Unmarshal([]byte(`["c","a"]`), list); err != nil {
		fmt.Println("error:", err)
		return
	}
	for i, v := range list.IterateOver() {
		fmt.Println(i, v)
	}
	// Output:
	// 0 c
	// 1 a
}
