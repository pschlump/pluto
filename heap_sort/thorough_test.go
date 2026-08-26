package heap_sort

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// extract returns the Key fields of a sorted result as a plain int slice.
func extract(sorted []HsTest) (rv []int) {
	rv = make([]int, 0, len(sorted))
	for _, v := range sorted {
		rv = append(rv, v.Key)
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
	h := NewHeapSortFunc(cmpHsTest)
	h.Insert(HsTest{Key: 42})
	if h.Len() != 1 {
		t.Fatalf("expected length 1, got %d", h.Len())
	}
	checkOrder(t, "Sort of single element", extract(h.Sort()), []int{42})
	if h.Len() != 0 {
		t.Errorf("expected empty after Sort, got length %d", h.Len())
	}

	h.Insert(HsTest{Key: -7})
	checkOrder(t, "SortDown of single element", extract(h.SortDown()), []int{-7})
	if h.Len() != 0 {
		t.Errorf("expected empty after SortDown, got length %d", h.Len())
	}
}

func TestDuplicates(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	for _, v := range []int{3, 1, 3, 2, 1, 3, 2} {
		h.Insert(HsTest{Key: v})
	}
	checkOrder(t, "Sort with duplicates", extract(h.Sort()), []int{1, 1, 2, 2, 3, 3, 3})

	h.InsertArray([]HsTest{{Key: 5}, {Key: 5}, {Key: 5}})
	checkOrder(t, "SortDown with duplicates", extract(h.SortDown()), []int{5, 5, 5})
}

func TestRepeatedSortIsIdempotent(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	if got := h.Sort(); len(got) != 0 {
		t.Errorf("first Sort on empty: expected 0 elements, got %d", len(got))
	}
	// A second Sort on an empty sorter must also be empty, not panic.
	if got := h.Sort(); len(got) != 0 {
		t.Errorf("second Sort on empty: expected 0 elements, got %d", len(got))
	}
	if got := h.SortDown(); len(got) != 0 {
		t.Errorf("SortDown after Sort: expected 0 elements, got %d", len(got))
	}
}

func TestLenLengthConsistency(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	for n := range 5 {
		if h.Len() != n || h.Length() != n {
			t.Errorf("before insert %d: Len=%d Length=%d, expected %d", n, h.Len(), h.Length(), n)
		}
		h.Insert(HsTest{Key: n})
	}
	h.Truncate()
	if h.Len() != 0 || h.Length() != 0 {
		t.Errorf("after Truncate: Len=%d Length=%d, expected 0", h.Len(), h.Length())
	}
}

func TestInsertArrayMultipleBatches(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	// InsertArray on an empty sorter.
	h.InsertArray([]HsTest{{Key: 9}, {Key: 4}})
	// InsertArray on a non-empty sorter must re-heapify the combined data.
	h.InsertArray([]HsTest{{Key: 1}, {Key: 7}, {Key: 2}})
	if h.Len() != 5 {
		t.Fatalf("expected length 5, got %d", h.Len())
	}
	checkOrder(t, "Sort after two InsertArray batches", extract(h.Sort()), []int{1, 2, 4, 7, 9})
}

func TestInsertArrayEmpty(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	h.Insert(HsTest{Key: 3})
	h.InsertArray(nil)
	h.InsertArray([]HsTest{})
	if h.Len() != 1 {
		t.Fatalf("expected length 1 after empty InsertArray, got %d", h.Len())
	}
	checkOrder(t, "Sort after empty InsertArray", extract(h.Sort()), []int{3})
}

// TestInsertArrayDoesNotAlias verifies that the batch is copied into
// the sorter: mutating the caller's slice afterwards cannot corrupt
// the sort.
func TestInsertArrayDoesNotAlias(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	batch := []HsTest{{Key: 3}, {Key: 1}, {Key: 2}}
	h.InsertArray(batch)
	for i := range batch {
		batch[i] = HsTest{Key: 999}
	}
	checkOrder(t, "Sort after caller mutates batch", extract(h.Sort()), []int{1, 2, 3})
}

func TestSortedAndReverseSortedInput(t *testing.T) {
	asc := []HsTest{}
	desc := []HsTest{}
	for i := range 20 {
		asc = append(asc, HsTest{Key: i})
		desc = append(desc, HsTest{Key: 19 - i})
	}

	h := NewHeapSortFunc(cmpHsTest)
	h.InsertArray(asc)
	got := extract(h.Sort())
	if !slices.IsSorted(got) {
		t.Errorf("Sort of already-sorted input is not sorted: %v", got)
	}

	h.InsertArray(desc)
	got = extract(h.Sort())
	if !slices.IsSorted(got) {
		t.Errorf("Sort of reverse-sorted input is not sorted: %v", got)
	}
	gotD := extract(h.SortDown())
	if len(gotD) != 0 {
		t.Errorf("expected empty after drain, got %v", gotD)
	}

	h.InsertArray(desc)
	gotD = extract(h.SortDown())
	if !slices.IsSortedFunc(gotD, func(a, b int) int { return b - a }) {
		t.Errorf("SortDown of reverse-sorted input is not descending: %v", gotD)
	}
}

// TestRandomizedProperty inserts random values (with duplicates) through a
// random mix of Insert and InsertArray, then cross-checks Sort and SortDown
// against a reference sorted slice.  Fixed PCG seed for reproducibility.
func TestRandomizedProperty(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))

	for round := range 20 {
		h := NewHeapSortFunc(cmpHsTest)
		var reference []int

		ops := 50 + rng.IntN(200)
		for i := range ops {
			switch rng.IntN(3) {
			case 0: // single Insert
				v := rng.IntN(1000)
				h.Insert(HsTest{Key: v})
				reference = append(reference, v)
			case 1: // small InsertArray batch
				m := 1 + rng.IntN(10)
				batch := make([]HsTest, 0, m)
				for range m {
					v := rng.IntN(1000)
					batch = append(batch, HsTest{Key: v})
					reference = append(reference, v)
				}
				h.InsertArray(batch)
			default: // Insert followed by a length sanity check
				v := rng.IntN(1000)
				h.Insert(HsTest{Key: v})
				reference = append(reference, v)
				if h.Len() != len(reference) {
					t.Fatalf("round %d op %d: Len()=%d, reference has %d", round, i, h.Len(), len(reference))
				}
			}
		}

		if h.Len() != len(reference) || h.Length() != len(reference) {
			t.Fatalf("round %d: Len=%d Length=%d, expected %d", round, h.Len(), h.Length(), len(reference))
		}

		slices.Sort(reference)

		if round%2 == 0 {
			got := extract(h.Sort())
			checkOrder(t, "randomized Sort", got, reference)
		} else {
			got := extract(h.SortDown())
			// Reverse reference into descending order.
			slices.Reverse(reference)
			checkOrder(t, "randomized SortDown", got, reference)
		}

		// The sorter must be drained and reusable after sorting.
		if h.Len() != 0 {
			t.Fatalf("round %d: expected empty after sort, got %d", round, h.Len())
		}
		h.Insert(HsTest{Key: rng.IntN(1000)})
		if h.Len() != 1 {
			t.Fatalf("round %d: sorter not reusable after sort, Len=%d", round, h.Len())
		}
		h.Truncate()
		if h.Len() != 0 {
			t.Fatalf("round %d: Truncate failed, Len=%d", round, h.Len())
		}
	}
}
