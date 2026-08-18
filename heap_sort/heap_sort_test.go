// Copyright (C) 2021 Philip Schlump. All rights reserved.

package heap_sort

import (
	"fmt"
	"testing"

	"github.com/pschlump/pluto/comparable"
)

// Create a HeapSort type called SomeData
type SomeData struct {
	theValue int // The theValue of the item in the queue.
}

// At compile time verify that this is a correct type/interface setup.
var _ comparable.Comparable = (*SomeData)(nil)

// Compare implements the Compare function to satisfy the interface requirements.
func (aa SomeData) Compare(x comparable.Comparable) int {
	if bb, ok := x.(SomeData); ok {
		return int(aa.theValue) - int(bb.theValue)
	} else if bb, ok := x.(*SomeData); ok {
		return int(aa.theValue) - int((*bb).theValue)
	} else {
		panic(fmt.Sprintf("Passed invalid type %T to a Compare function.", x))
	}
	// return 0 --detects as unreachable in Go 1.23
}

func TestSortUp(t *testing.T) {
	h := NewHeapSort[SomeData]()
	sample := []int{5, 2, 1, 8, 3, 4}
	for _, v := range sample {
		vv := &SomeData{theValue: v}
		h.Insert(vv)
	}

	if h.theHeap.Len() != 6 {
		t.Errorf("Invalid length returned: Expected %d got %d\n", h.theHeap.Len(), 6)
	}
	if h.Len() != 6 {
		t.Errorf("Invalid length returned: Expected %d got %d\n", h.theHeap.Len(), 6)
	}
	if h.Length() != 6 {
		t.Errorf("Invalid length returned: Expected %d got %d\n", h.theHeap.Len(), 6)
	}

	sorted := h.Sort()
	expect := []int{1, 2, 3, 4, 5, 8}
	if len(sorted) != len(expect) || len(sorted) != len(sample) {
		t.Errorf("Invalid length returned: Expected %d got %d, length of sorted data\n", len(sample), len(sorted))
	} else {
		for i, v := range expect {
			if v != sorted[i].theValue {
				t.Errorf("Expected %d got %d at subscript %d\n", v, sorted[i], i)
			}
		}
	}
	// Sort drains the heap.
	if h.Len() != 0 {
		t.Errorf("expected empty after Sort, got length %d", h.Len())
	}
}

func TestSortDown(t *testing.T) {
	h := NewHeapSort[SomeData]()
	sample := []int{5, 2, 1, 8, 3, 4}
	for _, v := range sample {
		vv := &SomeData{theValue: v}
		h.Insert(vv)
	}

	if h.theHeap.Len() != 6 {
		t.Errorf("Invalid length returned: Expected %d got %d\n", h.theHeap.Len(), 6)
	}

	sorted := h.SortDown()
	expect := []int{8, 5, 4, 3, 2, 1}
	if len(sorted) != len(expect) || len(sorted) != len(sample) {
		t.Errorf("Invalid length returned: Expected %d got %d, length of sorted data\n", len(sample), len(sorted))
	} else {
		for i, v := range expect {
			if v != sorted[i].theValue {
				t.Errorf("Expected %d got %d at subscript %d\n", v, sorted[i], i)
			}
		}
	}
	// SortDown drains the heap.
	if h.Len() != 0 {
		t.Errorf("expected empty after SortDown, got length %d", h.Len())
	}
}

func TestInsertArraySort(t *testing.T) {
	h := NewHeapSort[SomeData]()
	sample := []int{42, 5, 2, 99, 1, 8, 3, 4, 77, 23}
	data := make([]*SomeData, 0, len(sample))
	for _, v := range sample {
		data = append(data, &SomeData{theValue: v})
	}
	h.InsertArray(data)

	if h.Len() != len(sample) {
		t.Fatalf("Invalid length: expected %d got %d", len(sample), h.Len())
	}

	sorted := h.Sort()
	expect := []int{1, 2, 3, 4, 5, 8, 23, 42, 77, 99}
	for i, v := range expect {
		if sorted[i].theValue != v {
			t.Errorf("Expected %d got %d at subscript %d", v, sorted[i].theValue, i)
		}
	}
}

func TestInsertArraySortDown(t *testing.T) {
	h := NewHeapSort[SomeData]()
	sample := []int{42, 5, 2, 99, 1, 8, 3, 4, 77, 23}
	data := make([]*SomeData, 0, len(sample))
	for _, v := range sample {
		data = append(data, &SomeData{theValue: v})
	}
	h.InsertArray(data)

	sorted := h.SortDown()
	expect := []int{99, 77, 42, 23, 8, 5, 4, 3, 2, 1}
	for i, v := range expect {
		if sorted[i].theValue != v {
			t.Errorf("Expected %d got %d at subscript %d", v, sorted[i].theValue, i)
		}
	}
}

func TestMixedInsertAndInsertArray(t *testing.T) {
	h := NewHeapSort[SomeData]()
	h.Insert(&SomeData{theValue: 10})
	h.InsertArray([]*SomeData{{theValue: 30}, {theValue: 20}})
	h.Insert(&SomeData{theValue: 5})

	sorted := h.Sort()
	expect := []int{5, 10, 20, 30}
	for i, v := range expect {
		if sorted[i].theValue != v {
			t.Errorf("Expected %d got %d at subscript %d", v, sorted[i].theValue, i)
		}
	}
}

func TestSortEmpty(t *testing.T) {
	h := NewHeapSort[SomeData]()
	if got := h.Sort(); len(got) != 0 {
		t.Errorf("Sort on empty: expected 0 elements, got %d", len(got))
	}
	if got := h.SortDown(); len(got) != 0 {
		t.Errorf("SortDown on empty: expected 0 elements, got %d", len(got))
	}
}

func TestTruncate(t *testing.T) {
	h := NewHeapSort[SomeData]()
	for _, v := range []int{5, 2, 1} {
		h.Insert(&SomeData{theValue: v})
	}
	h.Truncate()
	if h.Len() != 0 || h.Length() != 0 {
		t.Errorf("Truncate: expected length 0, got %d/%d", h.Len(), h.Length())
	}
	// The heap must still be usable after a Truncate.
	h.Insert(&SomeData{theValue: 7})
	sorted := h.Sort()
	if len(sorted) != 1 || sorted[0].theValue != 7 {
		t.Errorf("after Truncate+Insert: expected [7], got %v", sorted)
	}
}

func BenchmarkHeapSort(b *testing.B) {
	vals := make([]*SomeData, b.N)
	for i := range vals {
		vals[i] = &SomeData{theValue: i}
	}
	b.ResetTimer()
	h := NewHeapSort[SomeData]()
	h.InsertArray(vals)
	_ = h.Sort()
}
