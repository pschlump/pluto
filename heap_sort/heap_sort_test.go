package heap_sort

import (
	"fmt"
	"testing"

	// "github.com/pschlump/dbgo"
	// "github.com/pschlump/MiscLib"
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
	return 0
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
	return
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
	return
}
