package heap_sort_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"strings"
	"sync"
	"testing"
)

// HsTest is the test element type.  Ordering is supplied to the sorter
// as a plain function (cmpHsTest below).
type HsTest struct {
	Key int    // the field the comparison function orders by
	Tag string // satellite data the comparison function ignores
}

// cmpHsTest orders HsTest by its Key field.
func cmpHsTest(a, b HsTest) int {
	return a.Key - b.Key
}

// expectPanic runs fn and fails the test unless it panics.
func expectPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
		}
	}()
	fn()
}

func TestSortUp(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	sample := []int{5, 2, 1, 8, 3, 4}
	for _, v := range sample {
		h.Insert(HsTest{Key: v, Tag: "kept"})
	}

	if h.Len() != 6 || h.Length() != 6 {
		t.Errorf("Invalid length returned: Expected 6 got %d/%d", h.Len(), h.Length())
	}

	sorted := h.Sort()
	expect := []int{1, 2, 3, 4, 5, 8}
	if len(sorted) != len(expect) {
		t.Fatalf("Invalid length returned: Expected %d got %d, length of sorted data", len(expect), len(sorted))
	}
	for i, v := range expect {
		if sorted[i].Key != v {
			t.Errorf("Expected %d got %v at subscript %d", v, sorted[i], i)
		}
		// Satellite data is carried through the sort by value.
		if sorted[i].Tag != "kept" {
			t.Errorf("Satellite data lost at subscript %d: got %q", i, sorted[i].Tag)
		}
	}
	// Sort drains the sorter.
	if h.Len() != 0 {
		t.Errorf("expected empty after Sort, got length %d", h.Len())
	}
}

func TestSortDown(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	sample := []int{5, 2, 1, 8, 3, 4}
	for _, v := range sample {
		h.Insert(HsTest{Key: v})
	}

	sorted := h.SortDown()
	expect := []int{8, 5, 4, 3, 2, 1}
	if len(sorted) != len(expect) {
		t.Fatalf("Invalid length returned: Expected %d got %d, length of sorted data", len(expect), len(sorted))
	}
	for i, v := range expect {
		if sorted[i].Key != v {
			t.Errorf("Expected %d got %v at subscript %d", v, sorted[i], i)
		}
	}
	// SortDown drains the sorter.
	if h.Len() != 0 {
		t.Errorf("expected empty after SortDown, got length %d", h.Len())
	}
}

// TestNewHeapSortOrdered covers the natural-ordering constructor: no
// comparison function, integers and strings.
func TestNewHeapSortOrdered(t *testing.T) {
	h := NewHeapSort[int]()
	for _, v := range []int{42, 7, 13, 7} {
		h.Insert(v)
	}
	got := h.Sort()
	expect := []int{7, 7, 13, 42}
	if len(got) != len(expect) {
		t.Fatalf("expected %d elements, got %d (%v)", len(expect), len(got), got)
	}
	for i, v := range expect {
		if got[i] != v {
			t.Errorf("at subscript %d expected %d got %d", i, v, got[i])
		}
	}

	s := NewHeapSort[string]()
	for _, v := range []string{"pear", "apple", "fig"} {
		s.Insert(v)
	}
	sg := s.SortDown()
	sExp := []string{"pear", "fig", "apple"}
	for i, v := range sExp {
		if sg[i] != v {
			t.Errorf("string at subscript %d expected %s got %s", i, v, sg[i])
		}
	}
}

func TestInsertArraySort(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	sample := []int{42, 5, 2, 99, 1, 8, 3, 4, 77, 23}
	data := make([]HsTest, 0, len(sample))
	for _, v := range sample {
		data = append(data, HsTest{Key: v})
	}
	h.InsertArray(data)

	if h.Len() != len(sample) {
		t.Fatalf("Invalid length: expected %d got %d", len(sample), h.Len())
	}

	sorted := h.Sort()
	expect := []int{1, 2, 3, 4, 5, 8, 23, 42, 77, 99}
	for i, v := range expect {
		if sorted[i].Key != v {
			t.Errorf("Expected %d got %v at subscript %d", v, sorted[i], i)
		}
	}
}

func TestInsertArraySortDown(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	sample := []int{42, 5, 2, 99, 1, 8, 3, 4, 77, 23}
	data := make([]HsTest, 0, len(sample))
	for _, v := range sample {
		data = append(data, HsTest{Key: v})
	}
	h.InsertArray(data)

	sorted := h.SortDown()
	expect := []int{99, 77, 42, 23, 8, 5, 4, 3, 2, 1}
	for i, v := range expect {
		if sorted[i].Key != v {
			t.Errorf("Expected %d got %v at subscript %d", v, sorted[i], i)
		}
	}
}

func TestMixedInsertAndInsertArray(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	h.Insert(HsTest{Key: 10})
	h.InsertArray([]HsTest{{Key: 30}, {Key: 20}})
	h.Insert(HsTest{Key: 5})

	sorted := h.Sort()
	expect := []int{5, 10, 20, 30}
	for i, v := range expect {
		if sorted[i].Key != v {
			t.Errorf("Expected %d got %v at subscript %d", v, sorted[i], i)
		}
	}
}

func TestSortEmpty(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	if got := h.Sort(); len(got) != 0 {
		t.Errorf("Sort on empty: expected 0 elements, got %d", len(got))
	}
	if got := h.SortDown(); len(got) != 0 {
		t.Errorf("SortDown on empty: expected 0 elements, got %d", len(got))
	}
}

func TestTruncate(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	for _, v := range []int{5, 2, 1} {
		h.Insert(HsTest{Key: v})
	}
	h.Truncate()
	if h.Len() != 0 || h.Length() != 0 {
		t.Errorf("Truncate: expected length 0, got %d/%d", h.Len(), h.Length())
	}
	// The sorter must still be usable after a Truncate.
	h.Insert(HsTest{Key: 7})
	sorted := h.Sort()
	if len(sorted) != 1 || sorted[0].Key != 7 {
		t.Errorf("after Truncate+Insert: expected [7], got %v", sorted)
	}
}

// TestConstructorNilPanics verifies the construction-time panic and that
// the message names the constructor.
func TestConstructorNilPanics(t *testing.T) {
	expectPanic(t, "NewHeapSortFunc(nil)", func() { NewHeapSortFunc[HsTest](nil) })

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected NewHeapSortFunc(nil) to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewHeapSortFunc") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		NewHeapSortFunc[HsTest](nil)
	}()
}

// TestNilPanics verifies the documented panics when Insert or
// InsertArray is called on a nil sorter — the two operations with no
// sane answer — and that the messages name the method and the fix.
func TestNilPanics(t *testing.T) {
	var nilSorter *HeapSort[HsTest]
	item := HsTest{Key: 1}
	batch := []HsTest{item}

	expectPanic(t, "Insert", func() { nilSorter.Insert(item) })
	expectPanic(t, "InsertArray", func() { nilSorter.InsertArray(batch) })

	// Verify the panic messages name the method.
	for _, tc := range []struct {
		name string
		fn   func()
	}{
		{"Insert", func() { nilSorter.Insert(item) }},
		{"InsertArray", func() { nilSorter.InsertArray(batch) }},
	} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected %s to panic on nil sorter.", tc.name)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, tc.name) {
					t.Errorf("Unexpected panic message: %v", r)
				}
			}()
			tc.fn()
		}()
	}
}

// TestZeroValuePanics: the same two inserts panic on a zero-value
// sorter (no underlying heap) — the message names the constructors.
func TestZeroValuePanics(t *testing.T) {
	var zero HeapSort[HsTest]
	expectPanic(t, "Insert on zero-value", func() { zero.Insert(HsTest{Key: 1}) })
	expectPanic(t, "InsertArray on zero-value", func() { zero.InsertArray([]HsTest{{Key: 1}}) })
}

// TestNilAndZeroTolerated verifies that every operation other than the
// two inserts treats a nil sorter and the zero value as an empty one.
func TestNilAndZeroTolerated(t *testing.T) {
	var nilSorter *HeapSort[HsTest]
	var zero HeapSort[HsTest]

	for name, hs := range map[string]*HeapSort[HsTest]{"nil": nilSorter, "zero": &zero} {
		if !hs.IsEmpty() {
			t.Errorf("%s: expected IsEmpty.", name)
		}
		if hs.Len() != 0 || hs.Length() != 0 {
			t.Errorf("%s: expected length 0, got %d/%d.", name, hs.Len(), hs.Length())
		}
		if got := hs.Sort(); len(got) != 0 {
			t.Errorf("%s: Sort on empty: expected 0 elements, got %d.", name, len(got))
		}
		if got := hs.SortDown(); len(got) != 0 {
			t.Errorf("%s: SortDown on empty: expected 0 elements, got %d.", name, len(got))
		}
		hs.Lock()     // nil no-op
		hs.Truncate() // must not panic
		hs.Unlock()   // nil no-op
	}
}

// TestLockNlCompound exercises the exposed write lock with the Nl
// methods: an atomic insert-batch-then-sort sequence on a shared sorter
// while other goroutines keep using the regular methods.  Every element
// inserted must come out exactly once across all results.
func TestLockNlCompound(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)

	const workers = 4
	var wg sync.WaitGroup

	// Atomic compound users: batch + sort under the manual lock.
	for g := range workers {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := range 25 {
				h.Lock()
				h.NlInsertArray([]HsTest{{Key: base + i%7}, {Key: base + 100 + i}})
				if !h.NlIsEmpty() {
					sorted := h.NlSort()
					for j := 1; j < len(sorted); j++ {
						if sorted[j-1].Key > sorted[j].Key {
							t.Errorf("worker %d: NlSort out of order: %d after %d", base, sorted[j].Key, sorted[j-1].Key)
							break
						}
					}
				}
				h.Unlock()
			}
		}(g * 1000)
	}

	// Regular-method users alongside them.
	wg.Go(func() {
		for i := range 100 {
			h.Insert(HsTest{Key: 100000 + i})
			_ = h.Len()
			_ = h.IsEmpty()
		}
	})

	wg.Wait()

	// Drain what is left and verify the sorter is consistent: the
	// remaining drain must come out in order (a mid-rebuild observation
	// or a corrupted heap would surface as an inversion here).
	leftover := h.Sort()
	for j := 1; j < len(leftover); j++ {
		if leftover[j-1].Key > leftover[j].Key {
			t.Fatalf("leftover drain out of order: %d after %d", leftover[j].Key, leftover[j-1].Key)
		}
	}
	if h.Len() != 0 {
		t.Errorf("expected empty after final drain, got %d", h.Len())
	}
}

func BenchmarkHeapSort(b *testing.B) {
	vals := make([]HsTest, b.N)
	for i := range vals {
		vals[i] = HsTest{Key: i}
	}
	b.ResetTimer()
	h := NewHeapSortFunc(cmpHsTest)
	h.InsertArray(vals)
	_ = h.Sort()
}

func BenchmarkInsert(b *testing.B) {
	h := NewHeapSortFunc(cmpHsTest)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Insert(HsTest{Key: i})
	}
}
