package heap_sort_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand/v2"
	"slices"
	"sync"
	"sync/atomic"
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

// checkAscending fails the test if got is not in ascending order.
func checkAscending(t *testing.T, name string, got []int) {
	t.Helper()
	if !slices.IsSorted(got) {
		t.Errorf("%s: result not in ascending order: %v", name, got)
	}
}

// checkDescending fails the test if got is not in descending order.
func checkDescending(t *testing.T, name string, got []int) {
	t.Helper()
	if !slices.IsSortedFunc(got, func(a, b int) int { return b - a }) {
		t.Errorf("%s: result not in descending order: %v", name, got)
	}
}

// containsAll reports whether every element of expect appears in got in
// order (both are expected to be ascending, expect without duplicates).
func containsAll(got, expect []int) bool {
	j := 0
	for _, v := range got {
		if j < len(expect) && v == expect[j] {
			j++
		}
	}
	return j == len(expect)
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

// TestNlSortsOnEmpty verifies the Nl drain pair under the manual lock on
// an empty sorter: empty slices, no panic.
func TestNlSortsOnEmpty(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	h.Lock()
	if !h.NlIsEmpty() || h.NlLen() != 0 {
		t.Errorf("expected NlIsEmpty/NlLen 0, got %v/%d", h.NlIsEmpty(), h.NlLen())
	}
	if got := h.NlSort(); len(got) != 0 {
		t.Errorf("NlSort on empty: expected 0 elements, got %d", len(got))
	}
	if got := h.NlSortDown(); len(got) != 0 {
		t.Errorf("NlSortDown on empty: expected 0 elements, got %d", len(got))
	}
	h.Unlock()
}

// TestNlCompoundNonEmpty verifies the Nl insert/drain pair under the manual
// lock on a non-empty sorter: NlInsert lands under the same hold of the
// lock, NlSort drains ascending, and NlSortDown drains descending.
func TestNlCompoundNonEmpty(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)
	h.InsertArray([]HsTest{{Key: 3}, {Key: 1}})

	h.Lock()
	h.NlInsert(HsTest{Key: 2})
	if h.NlLen() != 3 {
		t.Errorf("expected NlLen 3 after NlInsert, got %d", h.NlLen())
	}
	got := extract(h.NlSort())
	h.Unlock()
	checkOrder(t, "NlSort after NlInsert", got, []int{1, 2, 3})

	// NlSortDown drains in descending order.
	h.InsertArray([]HsTest{{Key: 5}, {Key: 4}})
	h.Lock()
	gotD := extract(h.NlSortDown())
	h.Unlock()
	checkOrder(t, "NlSortDown", gotD, []int{5, 4})
	if h.Len() != 0 {
		t.Errorf("expected sorter drained empty, got Len %d", h.Len())
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

// -------------------------------------------------------------------------------------------------------
// Concurrency: run with -race.
// -------------------------------------------------------------------------------------------------------

// TestConcurrentInsertSort hammers the sorter from both sides: writers
// feed it with a mix of Insert and InsertArray (the append+rebuild
// compound), drainers pull everything out with Sort and SortDown, and
// observers poll the size queries.  The accounting afterwards proves the
// two compound properties: every result slice is itself in order (a
// torn InsertArray would corrupt the heap invariant and surface as an
// inversion), and every one of the distinct inserted keys comes out
// exactly once across all results plus the final drain (Sort's
// whole-drain atomicity loses nothing and duplicates nothing).
func TestConcurrentInsertSort(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)

	const writers = 4
	const perWriter = 300
	const drainers = 2

	var writerWG, helperWG sync.WaitGroup
	var done atomic.Bool

	// results[i] collects the keys drainer i pulled out; index writers+i
	// for the drainers.  Written only by its goroutine, read after
	// helperWG.Wait().
	results := make([][]int, writers+drainers)

	// Drainers: atomically drain everything present, repeatedly, one via
	// Sort and one via SortDown.
	for d := range drainers {
		helperWG.Add(1)
		go func(d int) {
			defer helperWG.Done()
			var out []int
			for !done.Load() {
				if d == 0 {
					sorted := extract(h.Sort())
					checkAscending(t, "concurrent Sort", sorted)
					out = append(out, sorted...)
				} else {
					sorted := extract(h.SortDown())
					checkDescending(t, "concurrent SortDown", sorted)
					out = append(out, sorted...)
				}
			}
			results[writers+d] = out
		}(d)
	}

	// Observers: size queries only.
	for range 2 {
		helperWG.Add(1)
		go func() {
			defer helperWG.Done()
			for !done.Load() {
				_ = h.Len()
				_ = h.Length()
				_ = h.IsEmpty()
			}
		}()
	}

	// Writers: a mix of single inserts and small batches, all keys
	// distinct across writers.
	for g := range writers {
		writerWG.Add(1)
		go func(g int) {
			defer writerWG.Done()
			base := g * 100000
			for i := range perWriter {
				if i%7 == 3 {
					h.InsertArray([]HsTest{{Key: base + i}, {Key: base + 50000 + i}})
				} else {
					h.Insert(HsTest{Key: base + i})
				}
			}
		}(g)
	}

	writerWG.Wait()
	done.Store(true)
	helperWG.Wait()

	// Final drain after all goroutines have stopped.
	leftover := extract(h.Sort())
	checkAscending(t, "final drain", leftover)
	if h.Len() != 0 {
		t.Errorf("expected empty after final drain, got %d", h.Len())
	}

	// Accounting: each of the distinct keys must appear exactly once.
	counts := make(map[int]int)
	total := 0
	for _, out := range results {
		for _, k := range out {
			counts[k]++
			total++
		}
	}
	for _, k := range leftover {
		counts[k]++
		total++
	}
	// Each writer inserted perWriter keys plus one extra key (base+50000+i)
	// for every i where i%7==3.
	extra := 0
	for i := range perWriter {
		if i%7 == 3 {
			extra++
		}
	}
	expectTotal := writers*perWriter + writers*extra
	if total != expectTotal {
		t.Errorf("accounting: expected %d elements pulled out, got %d", expectTotal, total)
	}
	if len(counts) != total {
		t.Errorf("accounting: expected %d distinct keys, got %d (a key came out twice)", total, len(counts))
	}
	for k, n := range counts {
		if n != 1 {
			t.Errorf("accounting: key %d appeared %d times, expected exactly 1", k, n)
		}
	}
}

// TestConcurrentCompoundSort runs insert-batch-then-sort as one atomic
// sequence through the exposed write lock while regular users keep
// mutating the same sorter.  Every element must still come out exactly
// once across all results.
func TestConcurrentCompoundSort(t *testing.T) {
	h := NewHeapSortFunc(cmpHsTest)

	const workers = 4
	const rounds = 50
	const perRound = 5

	var wg sync.WaitGroup
	results := make([][]int, workers+1)

	// Compound users: Lock + NlInsertArray + NlSort + Unlock is atomic,
	// so each NlSort result is in order and contains exactly that
	// worker's batch — no element can be lost inside the sequence.
	// (Elements the regular user below inserted between this worker's
	// rounds may ride along in the drain.)
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			var out []int
			for r := range rounds {
				base := w*100000 + r*100
				batch := make([]HsTest, 0, perRound)
				expect := make([]int, 0, perRound)
				for i := range perRound {
					k := base + i
					batch = append(batch, HsTest{Key: k})
					expect = append(expect, k)
				}
				h.Lock()
				h.NlInsertArray(batch)
				got := extract(h.NlSort())
				h.Unlock()
				checkAscending(t, "compound NlSort batch", got)
				if !containsAll(got, expect) {
					t.Errorf("worker %d round %d: NlSort result %v does not contain the whole batch %v", w, r, got, expect)
				}
				out = append(out, got...)
			}
			results[w] = out
		}(w)
	}

	// A regular-method user alongside: insert bursts + occasional drains.
	wg.Add(1)
	go func() {
		defer wg.Done()
		var out []int
		for i := range rounds * perRound {
			h.Insert(HsTest{Key: 900000 + i})
			if i%37 == 36 {
				out = append(out, extract(h.Sort())...)
			}
		}
		results[workers] = out
	}()

	wg.Wait()

	// Drain the leftovers and account for every element exactly once.
	leftover := extract(h.Sort())
	checkAscending(t, "compound final drain", leftover)

	counts := make(map[int]int)
	total := 0
	for _, out := range results {
		for _, k := range out {
			counts[k]++
			total++
		}
	}
	for _, k := range leftover {
		counts[k]++
		total++
	}
	expectTotal := workers*rounds*perRound + rounds*perRound
	if total != expectTotal {
		t.Errorf("accounting: expected %d elements pulled out, got %d", expectTotal, total)
	}
	if len(counts) != total {
		t.Errorf("accounting: expected %d distinct keys, got %d", total, len(counts))
	}
	for k, n := range counts {
		if n != 1 {
			t.Errorf("accounting: key %d appeared %d times, expected exactly 1", k, n)
		}
	}
}
