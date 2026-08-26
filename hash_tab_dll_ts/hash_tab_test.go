package hash_tab_dll_ts

// Thread-safe fixed-size hash table with doubly linked list buckets.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"math/rand"
	"strings"
	"testing"
)

// TestData is the standard test element: S is the key that the equality and
// hash functions read; N is satellite data both ignore, used to verify that
// duplicate inserts replace the stored value.
type TestData struct {
	S string
	N int
}

// newTestTab builds a table with deterministic FNV-1a-64 hashing over S, so
// tests can reason about bucket placement.  The default NewHashTab hashes
// with a per-process random maphash seed, which is fine for membership
// assertions but not for placement.
func newTestTab(n int) *HashTab[TestData] {
	return NewHashTabFunc(
		func(a, b TestData) bool { return a.S == b.S },
		func(v TestData) uint64 {
			h := fnv.New64a()
			_, _ = h.Write([]byte(v.S))
			return h.Sum64()
		},
		n,
	)
}

func TestTest1(t *testing.T) {
	ht := newTestTab(7)

	for i := range 40 {
		ht.Insert(TestData{S: fmt.Sprintf("%4d", i)})
	}
	if ht.Len() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}
	for i := range 40 {
		ht.Insert(TestData{S: fmt.Sprintf("%4d", i)})
	}
	if ht.Len() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}

	// Check setup of hash tab
	if ht.IsEmpty() {
		t.Errorf("Expected to not be empty hash-tab, failed.")
	}
	if ht.Len() != 40 || ht.Length() != 40 {
		t.Errorf("Expected length of 40, got %d/%d", ht.Len(), ht.Length())
	}

	// Search - find (a live handle to the stored element)
	el, found := ht.Search(TestData{S: "   8"})
	if !found || el == nil || el.GetData().S != "   8" {
		t.Errorf("Expected to find it, did not (found=%v el=%v)", found, el)
	}

	// DeleteFound - O(1) removal of the located element
	if !ht.DeleteFound(el) {
		t.Errorf("Expected to delete it, did not")
	}
	if ht.Len() != 39 {
		t.Errorf("Expected length of 39, got %d", ht.Len())
	}

	// Search - do not find
	if _, found := ht.Search(TestData{S: "   8"}); found {
		t.Errorf("Expected to NOT find it, did")
	}

	// Insert
	ht.Insert(TestData{S: "abcd"})

	if ht.Length() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}

	// Delete by value (a probe, not the handle)
	if !ht.Delete(TestData{S: "abcd"}) {
		t.Errorf("Expected to delete it, did not")
	}
	if _, found := ht.Search(TestData{S: "abcd"}); found {
		t.Errorf("Expected to NOT find it, did")
	}

	// Truncate
	ht.Truncate()

	if ht.Length() != 0 {
		t.Errorf("Expected length of 0, got %d", ht.Len())
	}
	if !ht.IsEmpty() {
		t.Errorf("Expected empty hash-tab after Truncate.")
	}

	// Search - do not find
	if _, found := ht.Search(TestData{S: "abcd"}); found {
		t.Errorf("Expected to NOT find it, did")
	}
}

func TestTest2(t *testing.T) {
	ht := newTestTab(7)

	for i := range 40 {
		ht.Insert(TestData{S: fmt.Sprintf("%4d", i)})
	}
	if ht.Len() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}
	for i := range 40 {
		ht.Insert(TestData{S: fmt.Sprintf("%4d", i)})
	}
	if ht.Len() != 40 {
		t.Errorf("Expected length of 40, got %d", ht.Len())
	}

	// Delete by a probe value (not the returned stored element).
	if !ht.Delete(TestData{S: "  13"}) {
		t.Errorf("Expected to delete it, did not")
	}
	if !ht.Delete(TestData{S: "   6"}) {
		t.Errorf("Expected to delete it, did not")
	}

	// Search - find
	if _, found := ht.Search(TestData{S: "  38"}); !found {
		t.Errorf("Expected to find it, did not")
	}
}

// TestEmptyTable verifies operations on an empty table.
func TestEmptyTable(t *testing.T) {
	ht := newTestTab(7)
	if !ht.IsEmpty() {
		t.Errorf("Expected empty hash-tab after declaration, failed to get one.")
	}
	if ht.Len() != 0 || ht.Length() != 0 {
		t.Errorf("Expected length of 0, got %d/%d", ht.Len(), ht.Length())
	}
	if _, found := ht.Search(TestData{S: "nope"}); found {
		t.Errorf("Expected not-found from Search on empty table")
	}
	if ht.Delete(TestData{S: "nope"}) {
		t.Errorf("Expected false from Delete on empty table")
	}
	if ht.DeleteFound(nil) {
		t.Errorf("Expected false from DeleteFound(nil) on empty table")
	}
	n := 0
	ht.Walk(func(pos int, data TestData) bool {
		n++
		return true
	})
	if n != 0 {
		t.Errorf("Expected Walk on empty table to call function 0 times, got %d", n)
	}
	for range ht.All() {
		t.Errorf("Expected no elements from All on empty table")
	}
	for range ht.Values() {
		t.Errorf("Expected no elements from Values on empty table")
	}
}

// TestDefaultConstructor exercises NewHashTab with the builtin == equality
// and the randomized maphash hashing.  Bucket positions vary per process, so
// only membership is asserted.
func TestDefaultConstructor(t *testing.T) {
	ht := NewHashTab[string](17)
	for i := range 100 {
		if !ht.Insert(fmt.Sprintf("k%03d", i)) {
			t.Fatalf("Expected Insert of a new key to return true, i=%d", i)
		}
	}
	if ht.Len() != 100 {
		t.Errorf("Expected length 100, got %d", ht.Len())
	}
	// Duplicates replace.
	if ht.Insert("k042") {
		t.Errorf("Expected Insert of a duplicate key to return false")
	}
	if ht.Len() != 100 {
		t.Errorf("Expected length still 100 after duplicate insert, got %d", ht.Len())
	}
	for i := range 100 {
		if el, found := ht.Search(fmt.Sprintf("k%03d", i)); !found || el.GetData() != fmt.Sprintf("k%03d", i) {
			t.Errorf("Expected to find k%03d, got %q found=%v", i, el.GetData(), found)
		}
	}
	if _, found := ht.Search("missing"); found {
		t.Errorf("Expected not-found for a missing key")
	}
	// Struct keys compare with == on the whole value.
	type point struct{ x, y int }
	pt := NewHashTab[point](9)
	pt.Insert(point{1, 2})
	pt.Insert(point{1, 3})
	if pt.Len() != 2 {
		t.Errorf("Expected 2 points, got %d", pt.Len())
	}
	if _, found := pt.Search(point{1, 2}); !found {
		t.Errorf("Expected to find point {1,2}")
	}
	if !pt.Delete(point{1, 2}) || pt.Len() != 1 {
		t.Errorf("Expected delete of point {1,2} to leave 1 element")
	}

	// Two tables hash independently; both must find the same keys.
	a := NewHashTab[int](11)
	b := NewHashTab[int](11)
	for i := range 50 {
		a.Insert(i * 7)
		b.Insert(i * 7)
	}
	for i := range 50 {
		if _, fa := a.Search(i * 7); !fa {
			t.Errorf("Table a did not find %d", i*7)
		}
		if _, fb := b.Search(i * 7); !fb {
			t.Errorf("Table b did not find %d", i*7)
		}
	}
}

// TestHandleLiveness verifies that a handle returned by Search is a live
// node of the bucket list: it observes an Insert replacement of its element
// (the replacement is SetData on the same node) and remains removable via
// DeleteFound afterwards.
func TestHandleLiveness(t *testing.T) {
	ht := newTestTab(7)
	ht.Insert(TestData{S: "dup", N: 1})
	el, found := ht.Search(TestData{S: "dup"})
	if !found {
		t.Fatalf("Expected to find the first insert")
	}
	if got := el.GetData(); got.N != 1 {
		t.Fatalf("Handle holds N=1, got %v", got)
	}

	// A duplicate insert replaces the node's data in place — the old handle
	// stays valid and sees the new value.
	if added := ht.Insert(TestData{S: "dup", N: 2}); added {
		t.Errorf("Duplicate insert should report added=false (replaced)")
	}
	if ht.Len() != 1 {
		t.Fatalf("Duplicate insert should keep length 1, got %d", ht.Len())
	}
	if got := el.GetData(); got.N != 2 {
		t.Errorf("Handle should observe the replacement, got %v", got)
	}
	if got, found := ht.Search(TestData{S: "dup"}); !found || got != el {
		t.Errorf("Search should keep returning the same live node, got %v (want %v)", got, el)
	}

	// The handle is still removable.
	if !ht.DeleteFound(el) {
		t.Errorf("Expected DeleteFound of the replaced element to succeed")
	}
	if !ht.IsEmpty() {
		t.Errorf("Expected empty table, got length %d", ht.Len())
	}
}

// TestDeleteFoundPositions builds one chain with a constant hash function
// (everything lands in bucket 7 % 5 = 2) and splices out the middle, the
// head and the tail of the chain through their handles — each in O(1).
func TestDeleteFoundPositions(t *testing.T) {
	ht := NewHashTabFunc(
		func(a, b string) bool { return a == b },
		func(s string) uint64 { return 7 }, // every key chains in bucket 2
		5,
	)
	for _, k := range []string{"head-old", "mid", "tail-new"} {
		if !ht.Insert(k) {
			t.Fatalf("Expected insert of %q to be an add", k)
		}
	}
	if ht.Len() != 3 {
		t.Fatalf("Expected length 3, got %d", ht.Len())
	}
	if got := ht.buckets[2].Len(); got != 3 {
		t.Fatalf("Expected a 3-element chain in bucket 2, got %d", got)
	}

	// Splice out the middle.
	mid, found := ht.Search("mid")
	if !found {
		t.Fatalf("Expected to find mid")
	}
	if !ht.DeleteFound(mid) {
		t.Fatalf("Expected to delete mid")
	}
	if _, found := ht.Search("mid"); found {
		t.Errorf("mid should be gone")
	}
	if _, found := ht.Search("head-old"); !found {
		t.Errorf("head-old must survive the middle splice")
	}
	if _, found := ht.Search("tail-new"); !found {
		t.Errorf("tail-new must survive the middle splice")
	}
	checkInvariants(t, ht)

	// Splice out the head of the chain.
	head, _ := ht.Search("tail-new") // newest is at the list head
	if !ht.DeleteFound(head) {
		t.Fatalf("Expected to delete tail-new (the chain head)")
	}
	if _, found := ht.Search("head-old"); !found {
		t.Errorf("head-old must survive the head splice")
	}
	checkInvariants(t, ht)

	// Splice out the last element.
	last, _ := ht.Search("head-old")
	if !ht.DeleteFound(last) {
		t.Fatalf("Expected to delete head-old")
	}
	if !ht.IsEmpty() || ht.buckets[2].Len() != 0 {
		t.Errorf("Expected empty table and empty bucket after draining, got length %d", ht.Len())
	}
	// DeleteFound on a drained table returns false.
	if ht.DeleteFound(nil) {
		t.Errorf("DeleteFound on a drained table should return false")
	}
	checkInvariants(t, ht)
}

// TestDeleteNotFound verifies Delete of a missing element on a non-empty
// table.
func TestDeleteNotFound(t *testing.T) {
	ht := newTestTab(7)
	ht.Insert(TestData{S: "a"})
	ht.Insert(TestData{S: "b"})

	if ht.Delete(TestData{S: "not-present"}) {
		t.Errorf("Expected Delete of missing element to return false")
	}
	if ht.Len() != 2 {
		t.Errorf("Expected length unchanged at 2, got %d", ht.Len())
	}
}

func TestDumpOutput(t *testing.T) {
	ht := newTestTab(7)
	// Empty table: header line plus 7 empty-bucket lines.
	buf := new(bytes.Buffer)
	ht.Dump(buf)
	if !strings.HasPrefix(buf.String(), "Elements: 0, mod size:7\n") {
		t.Errorf("Unexpected Dump header for empty table: %q", buf.String())
	}
	if n := strings.Count(buf.String(), "\n"); n != 8 { // header + 7 buckets
		t.Errorf("Expected 8 lines from Dump of empty size-7 table, got %d", n)
	}

	// Non-empty table: every stored element appears in the dump.
	for i := range 20 {
		ht.Insert(TestData{S: fmt.Sprintf("d%03d", i)})
	}
	occupiedBuckets := 0
	for _, bucket := range ht.buckets {
		if !bucket.IsEmpty() {
			occupiedBuckets++
		}
	}
	buf.Reset()
	ht.Dump(buf)
	out := buf.String()
	if !strings.HasPrefix(out, "Elements: 20, mod size:7\n") {
		t.Errorf("Unexpected Dump header: %q", strings.SplitN(out, "\n", 2)[0])
	}
	// header + one line per empty bucket + one line per element
	if want := 1 + (7 - occupiedBuckets) + 20; strings.Count(out, "\n") != want {
		t.Errorf("Expected %d lines from Dump, got %d", want, strings.Count(out, "\n"))
	}
	for i := range 20 {
		if !strings.Contains(out, fmt.Sprintf("d%03d", i)) {
			t.Errorf("Dump output missing element d%03d", i)
		}
	}
	// Element lines carry no hash values — the dll nodes keep none, unlike
	// hash_tab's h= lines.
	occupied, empty := 0, 0
	for _, line := range strings.Split(out, "\n")[1:] {
		if strings.Contains(line, "empty") {
			empty++
		} else if strings.Contains(line, "bucket [") {
			occupied++
		}
	}
	if occupied != 20 || empty != 7-occupiedBuckets {
		t.Errorf("Expected 20 element and %d empty lines, got %d/%d", 7-occupiedBuckets, occupied, empty)
	}
	if strings.Contains(out, "h=") {
		t.Errorf("Dump should not print hash values, got %q", out)
	}
}

// TestNewHashTabPanics verifies that an initial size below 5 panics.
func TestNewHashTabPanics(t *testing.T) {
	expectPanic(t, "NewHashTab(4)", func() { NewHashTab[string](4) })
	expectPanic(t, "NewHashTabFunc(4)", func() {
		NewHashTabFunc(
			func(a, b string) bool { return a == b },
			func(s string) uint64 { return 1 },
			4,
		)
	})
}

// TestTruncateReuse verifies that a table is fully reusable after Truncate
// and that Truncate clears every bucket list (no stale chains).
func TestTruncateReuse(t *testing.T) {
	ht := newTestTab(7)
	for i := range 100 {
		ht.Insert(TestData{S: fmt.Sprintf("%4d", i)})
	}
	ht.Truncate()
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Fatalf("Expected empty table after Truncate, got length %d", ht.Len())
	}
	for i := range ht.buckets {
		if ht.buckets[i].Len() != 0 {
			t.Fatalf("Expected empty bucket after Truncate, bucket %d is not", i)
		}
	}
	checkInvariants(t, ht)
	for i := range 100 {
		ht.Insert(TestData{S: fmt.Sprintf("%4d", i)})
	}
	if ht.Len() != 100 {
		t.Errorf("Expected length of 100, got %d", ht.Len())
	}
	for i := range 100 {
		if _, found := ht.Search(TestData{S: fmt.Sprintf("%4d", i)}); !found {
			t.Errorf("Expected to find %4d after Truncate+re-insert, did not", i)
		}
	}
	checkInvariants(t, ht)
}

// TestOracleVsMap runs a long sequence of pseudo-random Insert/Delete/Search
// operations and compares the results against a map oracle.  This exercises
// the chain splice paths under heavy collision (size 7, keys 0..399).
func TestOracleVsMap(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run
	ht := newTestTab(7)
	oracle := make(map[string]int) // key -> satellite value of the latest insert

	key := func(i int) string { return fmt.Sprintf("%6d", i) }

	for op := range 2500 {
		k := key(rng.Intn(400))
		switch rng.Intn(3) {
		case 0, 1: // insert
			_, wasPresent := oracle[k]
			oracle[k] = op
			if got := ht.Insert(TestData{S: k, N: op}); got == wasPresent {
				t.Fatalf("op %d: Insert(%q) = %v, oracle says previously present = %v", op, k, got, wasPresent)
			}
		case 2: // delete
			_, want := oracle[k]
			if got := ht.Delete(TestData{S: k}); got != want {
				t.Fatalf("op %d: Delete(%q) = %v, oracle says %v", op, k, got, want)
			}
			delete(oracle, k)
		}
		if ht.Len() != len(oracle) {
			t.Fatalf("op %d: Len() = %d, oracle says %d", op, ht.Len(), len(oracle))
		}
		if op%250 == 0 {
			checkInvariants(t, ht)
		}
	}

	// Every key in the oracle must be found with its latest satellite value;
	// a sample of missing keys must not.
	for k, wantN := range oracle {
		el, found := ht.Search(TestData{S: k})
		if !found {
			t.Errorf("Expected to find %q, did not", k)
		} else if el.GetData().N != wantN {
			t.Errorf("Search(%q) returned stale satellite %d, want %d", k, el.GetData().N, wantN)
		}
	}
	for i := 400; i < 600; i++ {
		if _, found := ht.Search(TestData{S: key(i)}); found {
			t.Errorf("Expected to NOT find %q, did", key(i))
		}
	}
	checkInvariants(t, ht)
}

// TestInsertManyThenDeleteAll inserts 2500 values into 7 buckets (long
// chains) and then searches for and deletes every one of them, verifying
// each step.
func TestInsertManyThenDeleteAll(t *testing.T) {
	const N = 2500
	ht := newTestTab(7)
	for i := range N {
		ht.Insert(TestData{S: fmt.Sprintf("%6d", i)})
	}
	if ht.Len() != N {
		t.Fatalf("Expected length of %d, got %d", N, ht.Len())
	}
	// Re-inserting the same values must not grow the element count.
	for i := range N {
		ht.Insert(TestData{S: fmt.Sprintf("%6d", i)})
	}
	if ht.Len() != N {
		t.Fatalf("Expected length of %d after duplicate insert, got %d", N, ht.Len())
	}
	for i := range N {
		if _, found := ht.Search(TestData{S: fmt.Sprintf("%6d", i)}); !found {
			t.Fatalf("Expected to find %6d, did not", i)
		}
	}
	for i := 0; i < N; i += 2 {
		if !ht.Delete(TestData{S: fmt.Sprintf("%6d", i)}) {
			t.Fatalf("Expected to delete %6d, did not", i)
		}
	}
	if ht.Len() != N/2 {
		t.Fatalf("Expected length of %d, got %d", N/2, ht.Len())
	}
	// All the odd values must still be findable after half the table was deleted.
	for i := 1; i < N; i += 2 {
		if _, found := ht.Search(TestData{S: fmt.Sprintf("%6d", i)}); !found {
			t.Fatalf("Expected to find %6d after deletes, did not", i)
		}
	}
	for i := 1; i < N; i += 2 {
		if !ht.Delete(TestData{S: fmt.Sprintf("%6d", i)}) {
			t.Fatalf("Expected to delete %6d, did not", i)
		}
	}
	if !ht.IsEmpty() {
		t.Fatalf("Expected empty table, got length %d", ht.Len())
	}
}

// TestIterators verifies the range-over-func iterators All and Values.
func TestIterators(t *testing.T) {
	ht := newTestTab(7)
	for i := range 40 {
		ht.Insert(TestData{S: fmt.Sprintf("%4d", i)})
	}

	seen := make(map[string]int)
	count := 0
	for pos, item := range ht.All() {
		if pos < 0 || pos >= 7 {
			t.Errorf("Invalid bucket position %d", pos)
		}
		seen[item.S]++
		count++
	}
	if count != 40 {
		t.Errorf("Expected All to yield 40 elements, got %d", count)
	}
	for i := range 40 {
		k := fmt.Sprintf("%4d", i)
		if seen[k] != 1 {
			t.Errorf("Expected to see %q exactly once, saw %d", k, seen[k])
		}
	}

	count = 0
	for range ht.Values() {
		count++
	}
	if count != 40 {
		t.Errorf("Expected Values to yield 40 elements, got %d", count)
	}

	// Early termination of the range loop.
	count = 0
	for range ht.All() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("Expected early break after 1 element, got %d", count)
	}

	// Early termination of the Values range loop.
	count = 0
	for range ht.Values() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("Expected Values early break after 1 element, got %d", count)
	}

	// Empty table yields nothing.
	ht.Truncate()
	for range ht.All() {
		t.Errorf("Expected no elements from All on empty table")
	}
	for range ht.Values() {
		t.Errorf("Expected no elements from Values on empty table")
	}
}

// TestWalk verifies the Walk callback API, including early termination.
func TestWalk(t *testing.T) {
	ht := newTestTab(7)
	for i := range 40 {
		ht.Insert(TestData{S: fmt.Sprintf("%4d", i)})
	}
	n := 0
	b := ht.Walk(func(pos int, data TestData) bool {
		n++
		return true
	})
	if !b || n != 40 {
		t.Errorf("Expected complete walk over 40 elements, got b=%v n=%d", b, n)
	}
	n = 0
	b = ht.Walk(func(pos int, data TestData) bool {
		n++
		return false
	})
	if b || n != 1 {
		t.Errorf("Expected walk to stop after 1 element, got b=%v n=%d", b, n)
	}
}

func BenchmarkInsert(b *testing.B) {
	ht := newTestTab(1024)
	items := make([]TestData, b.N)
	for i := range items {
		items[i] = TestData{S: fmt.Sprintf("%8d", i)}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ht.Insert(items[i])
	}
}

func BenchmarkSearch(b *testing.B) {
	ht := newTestTab(1024)
	const n = 1000
	for i := range n {
		ht.Insert(TestData{S: fmt.Sprintf("%8d", i)})
	}
	probe := TestData{S: fmt.Sprintf("%8d", n/2)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ht.Search(probe)
	}
}

func BenchmarkDelete(b *testing.B) {
	ht := newTestTab(1024)
	const n = 1000
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if ht.IsEmpty() {
			b.StopTimer()
			for j := range n {
				ht.Insert(TestData{S: fmt.Sprintf("%8d", j)})
			}
			b.StartTimer()
		}
		ht.Delete(TestData{S: fmt.Sprintf("%8d", i%n)})
	}
}

// BenchmarkDeleteFound measures the locate-then-splice flow that the dll
// buckets exist for: Search hands back the handle, DeleteFound splices the
// node out in O(1), and the element is re-inserted to keep the table
// population steady.
func BenchmarkDeleteFound(b *testing.B) {
	ht := newTestTab(1024)
	const n = 1000
	for i := range n {
		ht.Insert(TestData{S: fmt.Sprintf("%8d", i)})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if el, found := ht.Search(TestData{S: fmt.Sprintf("%8d", i%n)}); found {
			ht.DeleteFound(el)
		}
		ht.Insert(TestData{S: fmt.Sprintf("%8d", i%n)})
	}
}
