package multiway_pq_test

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"

	"github.com/pschlump/pluto/multiway_pq"
)

// Example demonstrates the basic min-heap over an ordered type.
func Example() {
	h := multiway_pq.NewDHeap[int](4) // 4 children per node
	for _, v := range []int{42, 7, 13, 99, 0} {
		h.Push(v)
	}

	v, _ := h.Peek()
	fmt.Println("min:", v)

	for {
		v, found := h.Pop()
		if !found {
			break
		}
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output:
	// min: 0
	// 0 7 13 42 99
}

// ExampleNewDHeapFunc demonstrates a max-heap via a reversed comparison
// function.
func ExampleNewDHeapFunc() {
	maxHeap := multiway_pq.NewDHeapFunc(4, func(a, b int) int { return -multiway_pq.Compare(a, b) })
	for _, v := range []int{5, 1, 9, 3} {
		maxHeap.Push(v)
	}
	for {
		v, found := maxHeap.Pop()
		if !found {
			break
		}
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output: 9 5 3 1
}

// ExampleDHeap_AppendHeap demonstrates the bulk load: AppendHeap leaves
// a non-heap state, the bottom-up rebuild loop restores it in O(n).
func ExampleDHeap_AppendHeap() {
	h := multiway_pq.NewDHeap[int](4)
	h.AppendHeap([]int{9, 5, 2, 7, 1, 8})
	for i := (h.Len() - 2) / h.D(); i >= 0; i-- {
		h.Heapify(h.Len(), i)
	}
	for {
		v, found := h.Pop()
		if !found {
			break
		}
		fmt.Print(v, " ")
	}
	fmt.Println()
	// Output: 1 2 5 7 8 9
}

// Example_mergeKSortedSlices is the Multiway.java pattern: merge k
// sorted streams by keeping the head of each stream in a heap — the
// classic use of a heap over k pointers (Sedgwick §2.4 Multiway).  Each
// element carries its stream id as satellite data the comparator
// ignores.
func Example_mergeKSortedSlices() {
	streams := [][]int{
		{1, 4, 7, 10},
		{2, 5, 8},
		{3, 6, 9, 12, 15},
	}

	type head struct {
		v    int // the value — what the comparator reads
		from int // which stream it came from
	}
	h := multiway_pq.NewDHeapFunc(len(streams), func(a, b head) int {
		return multiway_pq.Compare(a.v, b.v)
	})

	next := make([]int, len(streams)) // per-stream cursor
	for i, s := range streams {
		if len(s) > 0 {
			h.Push(head{v: s[0], from: i})
			next[i] = 1
		}
	}

	var merged []int
	for {
		p, found := h.Pop()
		if !found {
			break
		}
		merged = append(merged, p.v)
		if next[p.from] < len(streams[p.from]) {
			h.Push(head{v: streams[p.from][next[p.from]], from: p.from})
			next[p.from]++
		}
	}
	fmt.Println(merged)
	// Output: [1 2 3 4 5 6 7 8 9 10 12 15]
}

// ExampleDHeap_Fix demonstrates replacing an element in place — cheaper
// than a Delete followed by a Push.
func ExampleDHeap_Fix() {
	h := multiway_pq.NewDHeap[int](4)
	for _, v := range []int{10, 20, 30} {
		h.Push(v)
	}

	// Demote the minimum: it sinks, the new minimum surfaces.
	h.Fix(0, 99)
	v, _ := h.Peek()
	fmt.Println("min after demotion:", v)

	// Promote a leaf slot to the front.
	h.Fix(h.Len()-1, 1)
	v, _ = h.Peek()
	fmt.Println("min after promotion:", v)
	// Output:
	// min after demotion: 20
	// min after promotion: 1
}
