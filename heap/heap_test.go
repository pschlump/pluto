// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright (C) 2021 Philip Schlump. All rights reserved.

package heap

import (
	"fmt"
	"testing"

	// "github.com/pschlump/godebug"
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
	return 0
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
			t.Errorf("Heap invariant invalidated [%d] = %d > [%d] = %d, compare()=%d", i, *((*hp).data[i]), j1, *((*hp).data[j1]), c)
			return
		}
		hp.verify(t, j1) // Recursivly check each sub-tree
	}
	if j2 < n {
		// if h.Less(j2, i) {																			// PJS
		c := (*(hp.data[j2])).Compare(*(hp.data[i])) // Compare [j2] less than [i]
		if c < 0 {
			// fmt.Printf("%s((Error 2 from verify))%s heap invariant invalidated [%d] = %d > [%d] = %d, compare()=%d\n", MiscLib.ColorRed, MiscLib.ColorReset, i, *((*hp).data[i]), j1, *((*hp).data[j2]), c)
			t.Errorf("heap invariant invalidated [%d] = %d > [%d] = %d, compare()=%d", i, *((*hp).data[i]), j1, *((*hp).data[j2]), c)
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

/*

func TestInit1(t *testing.T) {
	h := new(myHeap)
	for i := 20; i > 0; i-- {
		h.Push(i) // all elements are different
	}
	Init(h)
	h.verify(t, 0)

	for i := 1; h.Len() > 0; i++ {
		x := Pop(h).(int)
		h.verify(t, 0)
		if x != i {
			t.Errorf("%d.th pop got %d; want %d", i, x, i)
		}
	}
}

func Test(t *testing.T) {
	h := new(myHeap)
	h.verify(t, 0)

	for i := 20; i > 10; i-- {
		h.Push(i)
	}
	Init(h)
	h.verify(t, 0)

	for i := 10; i > 0; i-- {
		Push(h, i)
		h.verify(t, 0)
	}

	for i := 1; h.Len() > 0; i++ {
		x := Pop(h).(int)
		if i < 20 {
			Push(h, 20+i)
		}
		h.verify(t, 0)
		if x != i {
			t.Errorf("%d.th pop got %d; want %d", i, x, i)
		}
	}
}

func TestRemove0(t *testing.T) {
	h := new(myHeap)
	for i := 0; i < 10; i++ {
		h.Push(i)
	}
	h.verify(t, 0)

	for h.Len() > 0 {
		i := h.Len() - 1
		x := Remove(h, i).(int)
		if x != i {
			t.Errorf("Remove(%d) got %d; want %d", i, x, i)
		}
		h.verify(t, 0)
	}
}

func TestRemove1(t *testing.T) {
	h := new(myHeap)
	for i := 0; i < 10; i++ {
		h.Push(i)
	}
	h.verify(t, 0)

	for i := 0; h.Len() > 0; i++ {
		x := Remove(h, 0).(int)
		if x != i {
			t.Errorf("Remove(0) got %d; want %d", x, i)
		}
		h.verify(t, 0)
	}
}

func TestRemove2(t *testing.T) {
	N := 10

	h := new(myHeap)
	for i := 0; i < N; i++ {
		h.Push(i)
	}
	h.verify(t, 0)

	m := make(map[int]bool)
	for h.Len() > 0 {
		m[Remove(h, (h.Len()-1)/2).(int)] = true
		h.verify(t, 0)
	}

	if len(m) != N {
		t.Errorf("len(m) = %d; want %d", len(m), N)
	}
	for i := 0; i < len(m); i++ {
		if !m[i] {
			t.Errorf("m[%d] doesn't exist", i)
		}
	}
}

func BenchmarkDup(b *testing.B) {
	const n = 10000
	h := make(myHeap, 0, n)
	for i := 0; i < b.N; i++ {
		for j := 0; j < n; j++ {
			Push(&h, 0) // all elements are the same
		}
		for h.Len() > 0 {
			Pop(&h)
		}
	}
}

func TestFix(t *testing.T) {
	h := new(myHeap)
	h.verify(t, 0)

	for i := 200; i > 0; i -= 10 {
		Push(h, i)
	}
	h.verify(t, 0)

	if (*h)[0] != 10 {
		t.Fatalf("Expected head to be 10, was %d", (*h)[0])
	}
	(*h)[0] = 210
	Fix(h, 0)
	h.verify(t, 0)

	for i := 100; i > 0; i-- {
		elem := rand.Intn(h.Len())
		if i&1 == 0 {
			(*h)[elem] *= 2
		} else {
			(*h)[elem] /= 2
		}
		Fix(h, elem)
		h.verify(t, 0)
	}
}
*/

const db12 = false
