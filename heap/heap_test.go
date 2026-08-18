// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright (C) 2021 Philip Schlump. All rights reserved.

package heap

import (
	"fmt"
	"testing"

	// "github.com/pschlump/dbgo"
	// "github.com/pschlump/MiscLib"
	"github.com/pschlump/pluto/comparable"
)

// Create a "heap of int" type called myHeap
type myHeap int

// At compile time verify that this is a correct type/interface setup.
var _ comparable.Comparable = (*myHeap)(nil)

// Compare implements the Compare function to satisfy the interface requirements.
func (aa myHeap) Compare(x comparable.Comparable) int {
	if bb, ok := x.(myHeap); ok {
		return int(aa) - int(bb)
	} else if bb, ok := x.(*myHeap); ok {
		return int(aa) - int(*bb)
	} else {
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
	// return 0
}

func TestNewHeap(t *testing.T) {
	x := NewHeap[myHeap]()
	_ = x
}

func TestSetpAndPop(t *testing.T) {
	h := NewHeap[myHeap]()
	h.verify(t, 0)

	for i := 20; i > 10; i-- {
		hv := myHeap(i)
		h.Push(&hv)
	}
	h.verify(t, 0)

	h.Truncate() // Empty the Heap

	if h.Length() != 0 { // Verify it is empty.
		t.Errorf("Invalid length, expected 0, got %d", h.Length())
	}

	// Test with 20 0's in the heap.
	for i := 20; i > 0; i-- {
		hv := myHeap(0)
		h.Push(&hv) // all elements are the same
	}
	h.verify(t, 0)

	for i := 1; h.Length() > 0; i++ {
		if x0 := h.Pop(); x0 != nil {
			x := int(*x0)
			h.verify(t, 0)
			if x != 0 {
				t.Errorf("%d.th Pop() got %d; expected %d", i, x, 0)
			}
		}
	}
}

func TestSearch(t *testing.T) {
	h := NewHeap[myHeap]()
	for i := 20; i > 10; i-- {
		hv := myHeap(i)
		h.Push(&hv)
	}
	h.verify(t, 0)

	if db12 {
		h.printAsTree()
	}

	hv := myHeap(12)
	v, i, _ := h.Search(&hv)
	if db12 {
		fmt.Printf("v=%+v pos %d\n", *v, i)
	}

	for i := 11; i < 20; i++ {
		hv := myHeap(i)
		val, pos, err := h.Search(&hv)
		if err != nil {
			t.Errorf("Got err")
		} else if val != nil && int(*val) != i {
			t.Errorf("Got err, expected %d got %d, location=%d", i, int(*val), pos)
		}
	}

}

// verify checks that the heap is a heap - that it is properly ordered.
func (hp *Heap[T]) verify(t *testing.T, i int) {
	t.Helper() // set line number to line of caller of 'verify()'
	n := hp.Length()
	j1 := 2*i + 1
	j2 := 2*i + 2
	if j1 < n {
		// if h.Less(j1, i) {																			// PJS
		c := (*(hp.data[j1])).Compare(*(hp.data[i])) // Compare [j1] less than [i]
		if c < 0 {
			// fmt.Printf("%s((Error 1 from Verify))%s Heap invariant invalidated [%d] = %d > [%d] = %d, compare()=%d\n", MiscLib.ColorRed, MiscLib.ColorReset, i, *((*hp).data[i]), j1, *((*hp).data[j1]), c)
			t.Errorf("Heap invariant invalidated [%d] = %v > [%d] = %v, compare()=%d", i, *((*hp).data[i]), j1, *((*hp).data[j1]), c)
			return
		}
		hp.verify(t, j1) // Recursivly check each sub-tree
	}
	if j2 < n {
		// if h.Less(j2, i) {																			// PJS
		c := (*(hp.data[j2])).Compare(*(hp.data[i])) // Compare [j2] less than [i]
		if c < 0 {
			// fmt.Printf("%s((Error 2 from verify))%s heap invariant invalidated [%d] = %d > [%d] = %d, compare()=%d\n", MiscLib.ColorRed, MiscLib.ColorReset, i, *((*hp).data[i]), j1, *((*hp).data[j2]), c)
			t.Errorf("heap invariant invalidated [%d] = %v > [%d] = %v, compare()=%d", i, *((*hp).data[i]), j1, *((*hp).data[j2]), c)
			return
		}
		hp.verify(t, j2) // Recursivly check each sub-tree
	}
}

func TestWithDifferentElements(t *testing.T) {
	h := NewHeap[myHeap]()

	expect := make(map[int]bool)
	for i := 800; i > 0; i-- {
		hv := myHeap(i)
		h.Push(&hv)
		expect[i] = false
	}
	if db10 {
		h.printAsJSON()
		h.printAsTree()
	}
	h.verify(t, 0)

	// fmt.Printf ( "\n--------------------------- Top of Pop() Test --------------------------- \n\n" )
	for i := 1; h.Length() > 0; i++ {
		if x0 := h.Pop(); x0 != nil {
			x := int(*x0)
			// h.printAsTree()
			h.verify(t, 0)
			expect[x] = true
			if x != i {
				// if x < i {
				t.Errorf("%d.th Pop() got %d; expected >= %d", i, x, i)
				// }
			}
		}
	}

	for k, v := range expect {
		if !v {
			t.Errorf("missing %d\n", k)
		}
	}
}

func TestPopPeekOnEmpty(t *testing.T) {
	h := NewHeap[myHeap]()
	if got := h.Pop(); got != nil {
		t.Errorf("Pop on empty heap: expected nil, got %v", *got)
	}
	if got := h.Peek(); got != nil {
		t.Errorf("Peek on empty heap: expected nil, got %v", *got)
	}
	if h.Len() != 0 || h.Length() != 0 {
		t.Errorf("empty heap: expected length 0, got %d/%d", h.Len(), h.Length())
	}
}

func TestPeek(t *testing.T) {
	h := NewHeap[myHeap]()
	for i := 20; i > 10; i-- {
		hv := myHeap(i)
		h.Push(&hv)
	}
	h.verify(t, 0)
	// Peek must return the minimum without removing it.
	for i := 0; i < 3; i++ {
		got := h.Peek()
		if got == nil || int(*got) != 11 {
			t.Fatalf("Peek: expected 11, got %v", got)
		}
		if h.Len() != 10 {
			t.Fatalf("Peek changed length: expected 10, got %d", h.Len())
		}
	}
}

func TestDelete(t *testing.T) {
	h := NewHeap[myHeap]()
	for i := 30; i >= 1; i-- {
		hv := myHeap(i)
		h.Push(&hv)
	}
	h.verify(t, 0)

	// Delete the last element (index == Len()-1 edge case).
	last := h.GetValue(h.Len() - 1)
	got := h.Delete(h.Len() - 1)
	if got != last {
		t.Errorf("Delete(last): expected %v, got %v", *last, *got)
	}
	if h.Len() != 29 {
		t.Fatalf("Delete: expected length 29, got %d", h.Len())
	}
	h.verify(t, 0)

	// Delete an interior element found via Search.
	hv := myHeap(15)
	_, pos, err := h.Search(&hv)
	if err != nil || pos < 0 {
		t.Fatalf("Search for 15 failed: pos=%d err=%v", pos, err)
	}
	got = h.Delete(pos)
	if got == nil || int(*got) != 15 {
		t.Fatalf("Delete(%d): expected 15, got %v", pos, got)
	}
	h.verify(t, 0)
	if _, pos, _ := h.Search(&hv); pos != -1 {
		t.Errorf("15 should be gone after Delete, found at pos %d", pos)
	}

	// Delete the remaining elements one at a time from the root.
	prev := -1
	for h.Len() > 0 {
		x := h.Delete(0)
		if x == nil {
			t.Fatal("Delete(0) returned nil on a non-empty heap")
		}
		if int(*x) < prev {
			t.Errorf("Delete(0) out of order: got %d after %d", int(*x), prev)
		}
		prev = int(*x)
		h.verify(t, 0)
	}
	if h.Len() != 0 {
		t.Errorf("expected empty heap, got length %d", h.Len())
	}
}

func TestDeleteOutOfRangePanics(t *testing.T) {
	h := NewHeap[myHeap]()
	hv := myHeap(1)
	h.Push(&hv)
	for _, idx := range []int{-1, 1, 100} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Delete(%d): expected panic, got none", idx)
				}
			}()
			h.Delete(idx)
		}()
	}
}

func TestFixAndSetValue(t *testing.T) {
	h := NewHeap[myHeap]()
	for i := 200; i > 0; i -= 10 {
		hv := myHeap(i)
		h.Push(&hv)
	}
	h.verify(t, 0)

	if got := h.Peek(); got == nil || int(*got) != 10 {
		t.Fatalf("expected head to be 10, got %v", got)
	}

	// Replace the minimum with the largest value; it must sink to a leaf.
	nv := myHeap(210)
	h.SetValue(0, &nv)
	h.verify(t, 0)
	if got := h.Peek(); got == nil || int(*got) != 20 {
		t.Fatalf("after SetValue: expected head 20, got %v", got)
	}
	if got := h.GetValue(0); got == nil || int(*got) != 20 {
		t.Fatalf("GetValue(0): expected 20, got %v", got)
	}

	// Replace an interior element with a new minimum; it must rise to the top.
	nv2 := myHeap(1)
	h.Fix(5, &nv2)
	h.verify(t, 0)
	if got := h.Peek(); got == nil || int(*got) != 1 {
		t.Fatalf("after Fix: expected head 1, got %v", got)
	}
}

func TestAppendHeapAndHeapify(t *testing.T) {
	h := NewHeap[myHeap]()
	data := make([]*myHeap, 0, 500)
	for i := 500; i >= 1; i-- {
		hv := myHeap(i)
		data = append(data, &hv)
	}
	h.AppendHeap(data)
	for i := h.Len()/2 - 1; i >= 0; i-- {
		h.Heapify(h.Len(), i)
	}
	h.verify(t, 0)

	for i := 1; h.Len() > 0; i++ {
		x := h.Pop()
		if x == nil {
			t.Fatalf("%d.th Pop() returned nil", i)
		}
		if int(*x) != i {
			t.Fatalf("%d.th Pop() got %d; expected %d", i, int(*x), i)
		}
	}
}

func TestAllIterator(t *testing.T) {
	h := NewHeap[myHeap]()
	seen := make(map[int]bool)
	for i := 1; i <= 50; i++ {
		hv := myHeap(i)
		h.Push(&hv)
		seen[i] = false
	}

	count := 0
	for v := range h.All() {
		if v == nil {
			t.Fatal("All yielded a nil element")
		}
		if _, ok := seen[int(*v)]; !ok {
			t.Fatalf("All yielded unexpected value %d", int(*v))
		}
		if seen[int(*v)] {
			t.Fatalf("All yielded %d twice", int(*v))
		}
		seen[int(*v)] = true
		count++
	}
	if count != 50 {
		t.Errorf("All yielded %d elements, expected 50", count)
	}

	// Early break must stop the iteration.
	count = 0
	for range h.All() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("All with break yielded %d elements, expected 1", count)
	}

	// Empty heap yields nothing.
	empty := NewHeap[myHeap]()
	for range empty.All() {
		t.Error("All on empty heap yielded an element")
	}
}

func BenchmarkHeapPush(b *testing.B) {
	h := NewHeap[myHeap]()
	hv := myHeap(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Push(&hv) // all elements are the same
	}
}

func BenchmarkHeapPushPop(b *testing.B) {
	vals := make([]*myHeap, b.N)
	for i := range vals {
		v := myHeap(i)
		vals[i] = &v
	}
	h := NewHeap[myHeap]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Push(vals[i])
	}
	for i := 0; i < b.N; i++ {
		h.Pop()
	}
}

func BenchmarkHeapSearch(b *testing.B) {
	const n = 10000
	h := NewHeap[myHeap]()
	for i := 0; i < n; i++ {
		hv := myHeap(i)
		h.Push(&hv)
	}
	needle := myHeap(n - 1) // worst case: found at the end
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = h.Search(&needle)
	}
}

const db12 = false
