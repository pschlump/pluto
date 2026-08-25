// Copyright (C) 2021 Philip Schlump. All rights reserved.

package heap_sort

import (
	"math/rand"
	"sort"
	"testing"
)

// extract returns the theValue fields of a sorted result as a plain int slice.
func extract(sorted []*SomeData) (rv []int) {
	rv = make([]int, 0, len(sorted))
	for _, v := range sorted {
		rv = append(rv, v.theValue)
	}
	return
}

// checkOrder verifies got against expect element by element.
func checkOrder(t *testing.T, name string, got []int, expect []int) {
	t.Helper()
	if len(got) != len(expect) {
		t.Fatalf("%s: expected %d elements, got %d (%v)", name, len(expect), len(got), got)
	}
	for i, v := range expect {
		if got[i] != v {
			t.Errorf("%s: at subscript %d expected %d got %d", name, i, v, got[i])
		}
	}
}

func TestSingleElement(t *testing.T) {
	h := NewHeapSort[SomeData]()
	h.Insert(&SomeData{theValue: 42})
	if h.Len() != 1 {
		t.Fatalf("expected length 1, got %d", h.Len())
	}
	checkOrder(t, "Sort of single element", extract(h.Sort()), []int{42})
	if h.Len() != 0 {
		t.Errorf("expected empty after Sort, got length %d", h.Len())
	}

	h.Insert(&SomeData{theValue: -7})
	checkOrder(t, "SortDown of single element", extract(h.SortDown()), []int{-7})
	if h.Len() != 0 {
		t.Errorf("expected empty after SortDown, got length %d", h.Len())
	}
}

func TestDuplicates(t *testing.T) {
	h := NewHeapSort[SomeData]()
	for _, v := range []int{3, 1, 3, 2, 1, 3, 2} {
		h.Insert(&SomeData{theValue: v})
	}
	checkOrder(t, "Sort with duplicates", extract(h.Sort()), []int{1, 1, 2, 2, 3, 3, 3})

	h.InsertArray([]*SomeData{{theValue: 5}, {theValue: 5}, {theValue: 5}})
	checkOrder(t, "SortDown with duplicates", extract(h.SortDown()), []int{5, 5, 5})
}

func TestRepeatedSortIsIdempotent(t *testing.T) {
	h := NewHeapSort[SomeData]()
	if got := h.Sort(); len(got) != 0 {
		t.Errorf("first Sort on empty: expected 0 elements, got %d", len(got))
	}
	// A second Sort on an empty heap must also be empty, not panic.
	if got := h.Sort(); len(got) != 0 {
		t.Errorf("second Sort on empty: expected 0 elements, got %d", len(got))
	}
	if got := h.SortDown(); len(got) != 0 {
		t.Errorf("SortDown after Sort: expected 0 elements, got %d", len(got))
	}
}

func TestLenLengthConsistency(t *testing.T) {
	h := NewHeapSort[SomeData]()
	for n := 0; n < 5; n++ {
		if h.Len() != n || h.Length() != n {
			t.Errorf("before insert %d: Len=%d Length=%d, expected %d", n, h.Len(), h.Length(), n)
		}
		h.Insert(&SomeData{theValue: n})
	}
	h.Truncate()
	if h.Len() != 0 || h.Length() != 0 {
		t.Errorf("after Truncate: Len=%d Length=%d, expected 0", h.Len(), h.Length())
	}
}

func TestInsertArrayMultipleBatches(t *testing.T) {
	h := NewHeapSort[SomeData]()
	// InsertArray on an empty heap.
	h.InsertArray([]*SomeData{{theValue: 9}, {theValue: 4}})
	// InsertArray on a non-empty heap must re-heapify the combined data.
	h.InsertArray([]*SomeData{{theValue: 1}, {theValue: 7}, {theValue: 2}})
	if h.Len() != 5 {
		t.Fatalf("expected length 5, got %d", h.Len())
	}
	checkOrder(t, "Sort after two InsertArray batches", extract(h.Sort()), []int{1, 2, 4, 7, 9})
}

func TestInsertArrayEmpty(t *testing.T) {
	h := NewHeapSort[SomeData]()
	h.Insert(&SomeData{theValue: 3})
	h.InsertArray(nil)
	h.InsertArray([]*SomeData{})
	if h.Len() != 1 {
		t.Fatalf("expected length 1 after empty InsertArray, got %d", h.Len())
	}
	checkOrder(t, "Sort after empty InsertArray", extract(h.Sort()), []int{3})
}

func TestSortedAndReverseSortedInput(t *testing.T) {
	asc := []*SomeData{}
	desc := []*SomeData{}
	for i := 0; i < 20; i++ {
		asc = append(asc, &SomeData{theValue: i})
		desc = append(desc, &SomeData{theValue: 19 - i})
	}

	h := NewHeapSort[SomeData]()
	h.InsertArray(asc)
	got := extract(h.Sort())
	if !sort.IntsAreSorted(got) {
		t.Errorf("Sort of already-sorted input is not sorted: %v", got)
	}

	h.InsertArray(desc)
	got = extract(h.Sort())
	if !sort.IntsAreSorted(got) {
		t.Errorf("Sort of reverse-sorted input is not sorted: %v", got)
	}
	gotD := extract(h.SortDown())
	if len(gotD) != 0 {
		t.Errorf("expected empty after drain, got %v", gotD)
	}

	h.InsertArray(desc)
	got = extract(h.SortDown())
	if !sort.IsSorted(sort.Reverse(sort.IntSlice(got))) {
		t.Errorf("SortDown of reverse-sorted input is not descending: %v", got)
	}
}

// TestRandomizedProperty inserts random values (with duplicates) through a
// random mix of Insert and InsertArray, then cross-checks Sort and SortDown
// against a reference sorted slice.  Fixed seed for reproducibility.
func TestRandomizedProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	for round := 0; round < 20; round++ {
		h := NewHeapSort[SomeData]()
		var reference []int

		ops := 50 + rng.Intn(200)
		for i := 0; i < ops; i++ {
			switch rng.Intn(3) {
			case 0: // single Insert
				v := rng.Intn(1000)
				h.Insert(&SomeData{theValue: v})
				reference = append(reference, v)
			case 1: // small InsertArray batch
				m := 1 + rng.Intn(10)
				batch := make([]*SomeData, 0, m)
				for j := 0; j < m; j++ {
					v := rng.Intn(1000)
					batch = append(batch, &SomeData{theValue: v})
					reference = append(reference, v)
				}
				h.InsertArray(batch)
			default: // Insert followed by a length sanity check
				v := rng.Intn(1000)
				h.Insert(&SomeData{theValue: v})
				reference = append(reference, v)
				if h.Len() != len(reference) {
					t.Fatalf("round %d op %d: Len()=%d, reference has %d", round, i, h.Len(), len(reference))
				}
			}
		}

		if h.Len() != len(reference) || h.Length() != len(reference) {
			t.Fatalf("round %d: Len=%d Length=%d, expected %d", round, h.Len(), h.Length(), len(reference))
		}

		sort.Ints(reference)

		if round%2 == 0 {
			got := extract(h.Sort())
			checkOrder(t, "randomized Sort", got, reference)
		} else {
			got := extract(h.SortDown())
			// Reverse reference into descending order.
			for i, j := 0, len(reference)-1; i < j; i, j = i+1, j-1 {
				reference[i], reference[j] = reference[j], reference[i]
			}
			checkOrder(t, "randomized SortDown", got, reference)
		}

		// The heap must be drained and reusable after sorting.
		if h.Len() != 0 {
			t.Fatalf("round %d: expected empty after sort, got %d", round, h.Len())
		}
		h.Insert(&SomeData{theValue: rng.Intn(1000)})
		if h.Len() != 1 {
			t.Fatalf("round %d: heap not reusable after sort, Len=%d", round, h.Len())
		}
		h.Truncate()
		if h.Len() != 0 {
			t.Fatalf("round %d: Truncate failed, Len=%d", round, h.Len())
		}
	}
}
