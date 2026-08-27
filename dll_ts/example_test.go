/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package dll_ts_test

import (
	"fmt"
	"sync"

	"github.com/pschlump/pluto/dll_ts"
)

// Writers from many goroutines share one queue; readers see a consistent
// result once the writers finish.
func Example() {
	queue := dll_ts.NewDll[string]()

	var wg sync.WaitGroup
	for w := range 4 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range 25 {
				queue.Enqueue(fmt.Sprintf("%d-%02d", w, i))
			}
		}(w)
	}
	wg.Wait()

	fmt.Println(queue.Len(), queue.IsEmpty())

	// The exact interleaving of the writers is nondeterministic; the total
	// drain is not.
	drained := 0
	for {
		if _, err := queue.Pop(); err != nil {
			break
		}
		drained++
	}
	fmt.Println(drained, queue.IsEmpty())
	// Output:
	// 100 false
	// 100 true
}

// A stack: Push at the head, Peek and Pop from the head.
func ExampleNewDll() {
	stack := dll_ts.NewDll[int]()
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
func ExampleNewDllFunc() {
	type Task struct {
		ID   int
		Name string
	}

	byID := dll_ts.NewDllFunc(func(a, b Task) bool { return a.ID == b.ID })
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

// All iterates head to tail with the index; Backward iterates tail to
// head.  Both operate on a snapshot taken when they are called.
func ExampleDll_All() {
	list := dll_ts.NewDll[string]()
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

// Because All iterates a snapshot, mutating the list from inside the loop
// is safe — the loop still visits every element captured at the start.
func ExampleDll_All_snapshot() {
	list := dll_ts.NewDll[string]()
	for _, s := range []string{"a", "b", "c"} {
		list.AppendAtTail(s)
	}

	var visited []string
	for _, v := range list.All() {
		visited = append(visited, v)
		_ = list.Delete(v)
	}
	fmt.Println(visited, list.Len())
	// Output:
	// [a b c] 0
}

// The legacy iterator walks the live list, in either direction.  Each
// method takes the list's read lock for the duration of that call, so it
// is race-free under concurrent use.
func ExampleDll_Front() {
	list := dll_ts.NewDll[string]()
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
	list := dll_ts.NewDll[int]()
	for _, v := range []int{10, 20, 30} {
		list.AppendAtTail(v)
	}

	el, pos := list.Search(20)
	fmt.Println(pos)
	if err := list.DeleteFound(el); err == nil {
		fmt.Println("deleted")
	}

	var rest []int
	for _, v := range list.All() {
		rest = append(rest, v)
	}
	fmt.Println(rest)
	// Output:
	// 1
	// deleted
	// [10 30]
}

// Reverse flips the list in place with O(1) extra storage.
func ExampleDll_Reverse() {
	list := dll_ts.NewDll[int]()
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

// Concat appends a copy of another list; the source is snapshotted under
// its own lock, so this is race-free and the source is unchanged.
func ExampleDll_Concat() {
	a := dll_ts.NewDll[int]()
	for _, v := range []int{1, 2} {
		a.AppendAtTail(v)
	}
	b := dll_ts.NewDll[int]()
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
	list := dll_ts.NewDll[int]()
	for i := range 5 {
		list.AppendAtTail(i)
	}

	_ = list.Trim(2)

	var got []int
	for _, v := range list.All() {
		got = append(got, v)
	}
	fmt.Println(got)
	// Output:
	// [0 1]
}
