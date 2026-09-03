package hash_grow_ts

// Thorough tests for the thread-safe hash_grow table: nil and zero-value
// behavior, the panic contract with its messages, replacement semantics,
// wrap-around probe chains, growth, the full-table guard, snapshot iterator
// semantics, compound Lock+Nl operations, a randomized cross-check against a
// map model, and concurrent access.  TestData and newTestTab are defined in
// hash_tab_test.go and reused here.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"testing"
)

// expectPanic runs fx and fails the test unless it panics.
func expectPanic(t *testing.T, name string, fx func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
		}
	}()
	fx()
}

// expectPanicMessage additionally checks that the panic message contains
// `want` — the contract says each message names the method and the fix.
func expectPanicMessage(t *testing.T, name, want string, fx func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
			return
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, want) {
			t.Errorf("Expected the %s panic message to contain %q, got %v", name, want, r)
		}
	}()
	fx()
}

// checkInvariants verifies the structural invariants of the table: a bucket
// is occupied exactly when its raw hash is non-zero, the occupied count
// matches Len, Search finds every stored element, and Walk and the
// iterators agree with the buckets.  Call it after structural changes — in
// single-goroutine tests only (it reads the internals without the lock).
func checkInvariants(t *testing.T, ht *HashTab[TestData]) {
	t.Helper()
	var zero TestData
	occupied := 0
	for i := range ht.buckets {
		if ht.originalHash[i] == 0 {
			if ht.buckets[i] != zero {
				t.Fatalf("bucket %d is empty but holds %v", i, ht.buckets[i])
			}
			continue
		}
		occupied++
		if _, found := ht.Search(ht.buckets[i]); !found {
			t.Fatalf("element in bucket %d (%v) not found by Search", i, ht.buckets[i])
		}
	}
	if occupied != ht.Len() {
		t.Fatalf("occupied buckets = %d, Len() = %d", occupied, ht.Len())
	}
	seen := make(map[string]int)
	for pos, item := range ht.All() {
		if pos < 0 || pos >= ht.size {
			t.Fatalf("All reported out-of-range position %d", pos)
		}
		if ht.buckets[pos] != item {
			t.Fatalf("All reported position %d for %v but bucket holds %v", pos, item, ht.buckets[pos])
		}
		seen[item.S]++
	}
	if len(seen) != ht.Len() {
		t.Fatalf("All visited %d distinct elements, Len() = %d", len(seen), ht.Len())
	}
	for k, n := range seen {
		if n != 1 {
			t.Fatalf("key %q seen %d times by All, want 1", k, n)
		}
	}
	nWalk := 0
	ht.Walk(func(pos int, data TestData) bool {
		nWalk++
		return true
	})
	if nWalk != ht.Len() {
		t.Fatalf("Walk visited %d, Len() = %d", nWalk, ht.Len())
	}
}

// TestHashTabNilTolerated verifies that a nil table and the zero value
// behave as empty tables for every operation that has a sane answer.
func TestHashTabNilTolerated(t *testing.T) {
	var nilTable *HashTab[TestData]
	var zeroTable HashTab[TestData]

	for name, ht := range map[string]*HashTab[TestData]{
		"nil table":  nilTable,
		"zero value": &zeroTable,
	} {
		if !ht.IsEmpty() {
			t.Errorf("%s: should be empty", name)
		}
		if ht.Len() != 0 || ht.Length() != 0 {
			t.Errorf("%s: length should be 0, got %d/%d", name, ht.Len(), ht.Length())
		}
		if _, found := ht.Search(TestData{S: "x"}); found {
			t.Errorf("%s: Search should report not-found", name)
		}
		if ht.Delete(TestData{S: "x"}) {
			t.Errorf("%s: Delete should return false", name)
		}
		n := 0
		if !ht.Walk(func(pos int, data TestData) bool {
			n++
			return true
		}) || n != 0 {
			t.Errorf("%s: Walk should complete without calls, got n=%d", name, n)
		}
		for range ht.All() {
			t.Errorf("%s: All should yield nothing", name)
		}
		for range ht.Values() {
			t.Errorf("%s: Values should yield nothing", name)
		}
		buf := new(bytes.Buffer)
		ht.Dump(buf) // must not panic
		if !strings.HasPrefix(buf.String(), "Elements: 0") {
			t.Errorf("%s: unexpected Dump output %q", name, buf.String())
		}
		ht.Truncate() // must not panic
	}

	// Lock/Unlock on a nil table are no-ops, not panics; the nil-guarded
	// reads are safe inside the (no-op) held lock.
	nilTable.Lock()
	if !nilTable.IsEmpty() {
		t.Errorf("IsEmpty on a nil table should be true")
	}
	nilTable.Unlock()

	// The Nl reads tolerate a zero-value table (no functions set).
	if zeroTable.NlLen() != 0 || !zeroTable.NlIsEmpty() {
		t.Errorf("NlLen/NlIsEmpty on zero value should report empty")
	}
	if _, found := zeroTable.NlSearch(TestData{S: "x"}); found {
		t.Errorf("NlSearch on zero value should report not-found")
	}
	if zeroTable.NlDelete(TestData{S: "x"}) {
		t.Errorf("NlDelete on zero value should return false")
	}
}

// TestHashTabNilPanics verifies the panic contract and that each message
// names the method and the fix.
func TestHashTabNilPanics(t *testing.T) {
	var nilTable *HashTab[TestData]
	expectPanicMessage(t, "Insert on nil table", "Insert called on a nil table",
		func() { nilTable.Insert(TestData{S: "x"}) })

	var zeroTable HashTab[TestData]
	expectPanicMessage(t, "Insert on zero value", "NewHashTab",
		func() { zeroTable.Insert(TestData{S: "x"}) })

	validHash := func(v TestData) uint64 { return 7 }
	validEq := func(a, b TestData) bool { return a.S == b.S }
	expectPanicMessage(t, "NewHashTabFunc(nil eq)", "nil equality function",
		func() { NewHashTabFunc(nil, validHash, 7, 0) })
	expectPanicMessage(t, "NewHashTabFunc(nil hash)", "nil hash function",
		func() { NewHashTabFunc(validEq, nil, 7, 0) })

	// NlInsert (the Insert body) carries the same zero-value panic.
	zero := &HashTab[TestData]{}
	expectPanicMessage(t, "NlInsert on zero value", "NewHashTab",
		func() { zero.NlInsert(TestData{S: "x"}) })
}

// TestInsertReplacesExisting verifies that re-inserting an equal key replaces
// the stored value (the satellite data changes) and does not change the
// length, including when the key is not at its home bucket (collision path).
func TestInsertReplacesExisting(t *testing.T) {
	ht := newTestTab(7, 0)
	if added := ht.Insert(TestData{S: "dup", N: 1}); !added {
		t.Errorf("First insert of a key should report added=true")
	}
	if added := ht.Insert(TestData{S: "dup", N: 2}); added {
		t.Errorf("Duplicate insert should report added=false (replaced)")
	}
	if ht.Len() != 1 {
		t.Fatalf("Duplicate insert should keep length 1, got %d", ht.Len())
	}
	got, found := ht.Search(TestData{S: "dup"})
	if !found || got.N != 2 {
		t.Errorf("Search should return the replacement value, got %v found=%v", got, found)
	}

	// Force collisions so the replacement happens away from the home bucket.
	home := int(ht.hashOf(TestData{S: "dup"}) % uint64(sizeOf(t, ht)))
	used := map[string]bool{"dup": true}
	c1 := findKeyWithHome(t, ht, home, used)
	c2 := findKeyWithHome(t, ht, home, used)
	ht.Insert(TestData{S: c1, N: 10})
	ht.Insert(TestData{S: c2, N: 20}) // pushes the chain along
	ht.Insert(TestData{S: c1, N: 11}) // replace inside the probe chain
	if got, found := ht.Search(TestData{S: c1}); !found || got.N != 11 {
		t.Errorf("Replacement in collision chain should return new value, got %v found=%v", got, found)
	}
	if ht.Len() != 3 {
		t.Errorf("Expected length 3 after collision replacements, got %d", ht.Len())
	}
	checkInvariants(t, ht)
}

// findKeyWithHome returns a fresh key whose home bucket is the home bucket of
// the table `ht`, recording it in `used` so keys are never repeated.
func findKeyWithHome(t *testing.T, ht *HashTab[TestData], home int, used map[string]bool) string {
	t.Helper()
	size := sizeOf(t, ht)
	for i := 0; ; i++ {
		s := fmt.Sprintf("w%06d", i)
		if used[s] {
			continue
		}
		if int(ht.hashOf(TestData{S: s})%uint64(size)) == home {
			used[s] = true
			return s
		}
	}
}

// TestDeleteWrapAround builds a probe chain that wraps past the end of the
// table (buckets 4, 0, 1 in a size-5 table) and then deletes from the middle
// of the chain, exercising the backward-shift deletion across the wrap point
// and both arms of inProbeRange.
func TestDeleteWrapAround(t *testing.T) {
	const size = 5
	// Saturation 1.0 keeps the table from growing while length <= size.
	ht := newTestTab(size, 1.0)
	used := map[string]bool{}
	d := findKeyWithHome(t, ht, 0, used) // lands in bucket 0
	a := findKeyWithHome(t, ht, 4, used) // lands in bucket 4
	b := findKeyWithHome(t, ht, 4, used) // collides: 4 and 0 taken -> bucket 1

	ht.Insert(TestData{S: d})
	ht.Insert(TestData{S: a})
	ht.Insert(TestData{S: b})
	if ht.Len() != 3 {
		t.Fatalf("Expected length 3, got %d", ht.Len())
	}
	if ht.originalHash[4] == 0 || ht.originalHash[0] == 0 || ht.originalHash[1] == 0 {
		t.Fatalf("Expected chain across buckets 4,0,1; got %v", ht.buckets)
	}

	// Delete the middle of the chain (bucket 4).  The element in bucket 1
	// (home 4) must be shifted back into bucket 4 across the wrap point,
	// while the element in bucket 0 (home 0) must stay put.
	if !ht.Delete(TestData{S: a}) {
		t.Fatalf("Expected to delete %q", a)
	}
	if ht.Len() != 2 {
		t.Errorf("Expected length 2 after delete, got %d", ht.Len())
	}
	if _, found := ht.Search(TestData{S: b}); !found {
		t.Errorf("Element %q shifted across wrap point must still be found", b)
	}
	if _, found := ht.Search(TestData{S: d}); !found {
		t.Errorf("Element %q at its home bucket must still be found", d)
	}
	if ht.buckets[4].S != b {
		t.Errorf("Expected %q to be shifted into bucket 4, got %v", b, ht.buckets[4])
	}

	// A missing key with home bucket 4 must probe across the wrap and stop
	// at the first truly empty slot without finding anything.
	missing := findKeyWithHome(t, ht, 4, used)
	if _, found := ht.Search(TestData{S: missing}); found {
		t.Errorf("Missing key with wrapped probe should not be found")
	}
	if ht.Delete(TestData{S: missing}) {
		t.Errorf("Delete of missing key with wrapped probe should return false")
	}

	// Delete the rest; the table must drain cleanly.
	if !ht.Delete(TestData{S: b}) || !ht.Delete(TestData{S: d}) {
		t.Errorf("Expected to delete remaining elements")
	}
	if !ht.IsEmpty() {
		t.Errorf("Table should be empty, got length %d", ht.Len())
	}
	checkInvariants(t, ht)
}

// TestInProbeRange directly covers both arms of the cyclic interval test
// used by backward-shift deletion.
func TestInProbeRange(t *testing.T) {
	tests := []struct {
		gap, hf, home int
		want          bool
	}{
		// gap < hf: in range iff gap < home <= hf.
		{2, 5, 3, true},
		{2, 5, 5, true},
		{2, 5, 2, false},
		{2, 5, 6, false},
		// gap > hf (wrap): in range iff home > gap or home <= hf.
		{5, 2, 7, true},
		{5, 2, 1, true},
		{5, 2, 4, false},
	}
	for _, tc := range tests {
		if got := inProbeRange(tc.gap, tc.hf, tc.home); got != tc.want {
			t.Errorf("inProbeRange(%d, %d, %d) = %v, want %v", tc.gap, tc.hf, tc.home, got, tc.want)
		}
	}
}

// TestGrowthDoubles verifies that the table doubles when the load factor
// passes the saturation threshold and that all elements survive re-hashing.
func TestGrowthDoubles(t *testing.T) {
	ht := newTestTab(5, 0.5)
	if sizeOf(t, ht) != 5 {
		t.Fatalf("Expected initial size 5, got %d", sizeOf(t, ht))
	}
	keys := []string{"g0", "g1", "g2", "g3", "g4", "g5"}
	for i, k := range keys {
		ht.Insert(TestData{S: k})
		size := sizeOf(t, ht)
		switch {
		case i < 2 && size != 5: // 1/5, 2/5 stay below 0.5
			t.Fatalf("After %d inserts size should be 5, got %d", i+1, size)
		case i == 2 && size != 10: // 3/5 = 0.6 > 0.5 -> grow
			t.Fatalf("After 3 inserts size should be 10, got %d", size)
		case i >= 3 && i < 5 && size != 10:
			t.Fatalf("After %d inserts size should be 10, got %d", i+1, size)
		case i == 5 && size != 20: // 6/10 = 0.6 > 0.5 -> grow
			t.Fatalf("After 6 inserts size should be 20, got %d", size)
		}
	}
	if ht.Len() != len(keys) {
		t.Errorf("Expected length %d, got %d", len(keys), ht.Len())
	}
	for _, k := range keys {
		if _, found := ht.Search(TestData{S: k}); !found {
			t.Errorf("Expected to find %q after growth, did not", k)
		}
	}
	checkInvariants(t, ht)
}

// TestSaturationDefault verifies that 0, negative and NaN saturations all
// select the documented default of 0.5.
func TestSaturationDefault(t *testing.T) {
	for _, sat := range []float64{0, -1, math.NaN()} {
		ht := newTestTab(5, sat)
		if ht.saturationThreshold != 0.5 {
			t.Errorf("Saturation %v should select the default 0.5, got %v", sat, ht.saturationThreshold)
		}
	}
}

// TestFullTableGrows verifies the forced growth when a saturation of 1.0 or
// more has deferred growth until the table is completely full: the table
// doubles so the insert lands and probes always terminate.
func TestFullTableGrows(t *testing.T) {
	for _, saturation := range []float64{1.0, 2.0} {
		ht := newTestTab(5, saturation)
		for i := range 5 {
			ht.Insert(TestData{S: fmt.Sprintf("f%d", i)})
		}
		if sizeOf(t, ht) != 5 || ht.Len() != 5 {
			t.Fatalf("saturation %v: expected a full size-5 table, got size %d length %d", saturation, sizeOf(t, ht), ht.Len())
		}
		// A missing key on a completely full table must terminate not-found.
		if _, found := ht.Search(TestData{S: "no-such-key"}); found {
			t.Errorf("saturation %v: missing key must not be found", saturation)
		}
		// The 6th insert forces growth instead of being dropped.
		if !ht.Insert(TestData{S: "f5"}) {
			t.Errorf("saturation %v: the 6th insert should be an add", saturation)
		}
		if sizeOf(t, ht) != 10 {
			t.Errorf("saturation %v: expected forced growth to size 10, got %d", saturation, sizeOf(t, ht))
		}
		for i := range 6 {
			if _, found := ht.Search(TestData{S: fmt.Sprintf("f%d", i)}); !found {
				t.Errorf("saturation %v: expected to find f%d after forced growth", saturation, i)
			}
		}
		checkInvariants(t, ht)
	}
}

// TestHashZeroRemapped verifies that a hash function returning 0 (the
// reserved empty-slot marker) is remapped consistently on every path.
func TestHashZeroRemapped(t *testing.T) {
	ht := NewHashTabFunc(
		func(a, b string) bool { return a == b },
		func(s string) uint64 { return 0 }, // every key collides at bucket 1
		5, 0,
	)
	for _, k := range []string{"a", "b", "c"} {
		if !ht.Insert(k) {
			t.Errorf("Expected insert of %q to be an add", k)
		}
	}
	if ht.Len() != 3 {
		t.Fatalf("Expected length 3, got %d", ht.Len())
	}
	for _, k := range []string{"a", "b", "c"} {
		if _, found := ht.Search(k); !found {
			t.Errorf("Expected to find %q, did not", k)
		}
	}
	// Delete from the middle of the degenerate probe chain.
	if !ht.Delete("b") {
		t.Fatalf("Expected to delete b")
	}
	if _, found := ht.Search("b"); found {
		t.Errorf("b should be gone")
	}
	if _, found := ht.Search("c"); !found {
		t.Errorf("c must survive the backward shift out of the chain")
	}
	// The internal remap must have been applied on the insert path too.
	if ht.originalHash[1] != 1 {
		t.Errorf("hash 0 should be stored as 1, got %d", ht.originalHash[1])
	}
}

// TestIteratorSnapshotSemantics verifies that All and Values iterate a
// snapshot copied under the read lock when they are called: mutating the
// table afterwards — or from inside the loop body — does not affect the
// iteration and does not race.
func TestIteratorSnapshotSemantics(t *testing.T) {
	ht := newTestTab(16, 0)
	for i := range 30 {
		ht.Insert(TestData{S: fmt.Sprintf("s%03d", i)})
	}

	all := ht.All()     // snapshot copied now
	vals := ht.Values() // snapshot copied now
	ht.Truncate()       // ... and the table emptied after that
	n := 0
	for range all {
		n++
	}
	if n != 30 {
		t.Errorf("All should yield the 30 elements captured at call time, got %d", n)
	}
	n = 0
	for range vals {
		n++
	}
	if n != 30 {
		t.Errorf("Values should yield the 30 elements captured at call time, got %d", n)
	}

	// Mutating the table from inside the loop body is safe.
	ht2 := newTestTab(16, 0)
	for i := range 20 {
		ht2.Insert(TestData{S: fmt.Sprintf("t%03d", i)})
	}
	n = 0
	for _, v := range ht2.All() {
		ht2.Delete(v) // delete each yielded element inside the loop
		n++
	}
	if n != 20 {
		t.Errorf("Expected to iterate 20 elements while deleting them, got %d", n)
	}
	if ht2.Len() != 0 {
		t.Errorf("Expected an empty table after deleting every yielded element, got %d", ht2.Len())
	}
}

// TestLockNlCompound verifies the Lock/Unlock + Nl* escape hatch for
// compound operations: a search followed by a delete (or a bulk insert)
// runs atomically under one lock hold.
func TestLockNlCompound(t *testing.T) {
	ht := newTestTab(16, 0)
	for i := range 40 {
		ht.Insert(TestData{S: fmt.Sprintf("c%03d", i), N: i})
	}

	ht.Lock()
	if v, found := ht.NlSearch(TestData{S: "c021"}); found {
		if v.N != 21 {
			t.Errorf("NlSearch returned stale satellite %d, want 21", v.N)
		}
		if !ht.NlDelete(TestData{S: "c021"}) {
			t.Errorf("NlDelete inside the held lock should succeed")
		}
	} else {
		t.Errorf("NlSearch should have found c021")
	}
	if ht.NlLen() != 39 || ht.NlIsEmpty() {
		t.Errorf("NlLen/NlIsEmpty should report 39/false, got %d/%v", ht.NlLen(), ht.NlIsEmpty())
	}
	// A bulk insert under the same single lock hold.
	for i := 100; i < 150; i++ {
		ht.NlInsert(TestData{S: fmt.Sprintf("c%03d", i), N: i})
	}
	ht.Unlock()

	if ht.Len() != 89 {
		t.Fatalf("Expected length 89 after the compound section, got %d", ht.Len())
	}
	if _, found := ht.Search(TestData{S: "c021"}); found {
		t.Errorf("c021 should be gone")
	}
	for i := 100; i < 150; i++ {
		if v, found := ht.Search(TestData{S: fmt.Sprintf("c%03d", i)}); !found || v.N != i {
			t.Errorf("Expected to find c%03d with satellite %d, got %v found=%v", i, i, v, found)
		}
	}
	checkInvariants(t, ht)
}

// TestRandomizedModel runs a fixed-seed pseudo-random mix of Insert, Delete
// and Search operations, cross-checking every result (including the
// satellite value of the latest insert) against a map model and periodically
// validating structural invariants.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(12345)) // fixed seed: deterministic run
	ht := newTestTab(7, 0)
	model := make(map[string]int) // key -> satellite value of the latest insert

	key := func(i int) string { return fmt.Sprintf("r%05d", i) }

	for op := range 4000 {
		k := key(rng.Intn(300))
		switch rng.Intn(5) {
		case 0, 1, 2: // insert (weighted heavier)
			_, wasPresent := model[k]
			model[k] = op
			if got := ht.Insert(TestData{S: k, N: op}); got == wasPresent {
				t.Fatalf("op %d: Insert(%q) = %v, model says previously present = %v", op, k, got, wasPresent)
			}
		case 3: // delete
			_, want := model[k]
			if got := ht.Delete(TestData{S: k}); got != want {
				t.Fatalf("op %d: Delete(%q) = %v, model says %v", op, k, got, want)
			}
			delete(model, k)
		case 4: // search
			wantN, want := model[k]
			it, found := ht.Search(TestData{S: k})
			if found != want {
				t.Fatalf("op %d: Search(%q) found=%v, model says %v", op, k, found, want)
			}
			if found && it.N != wantN {
				t.Fatalf("op %d: Search(%q) returned stale satellite %d, want %d", op, k, it.N, wantN)
			}
		}
		if ht.Len() != len(model) {
			t.Fatalf("op %d: Len() = %d, model has %d", op, ht.Len(), len(model))
		}
		if op%250 == 0 {
			checkInvariants(t, ht)
		}
	}
	checkInvariants(t, ht)

	// Drain the table through the iterators and confirm it empties cleanly.
	toDelete := make([]string, 0, len(model))
	for item := range ht.Values() {
		toDelete = append(toDelete, item.S)
	}
	for _, k := range toDelete {
		if !ht.Delete(TestData{S: k}) {
			t.Fatalf("Expected to delete %q during drain", k)
		}
	}
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Fatalf("Table should be empty after drain, got length %d", ht.Len())
	}
	checkInvariants(t, ht)
}

// TestConcurrentInsertSearchIterate runs writers inserting disjoint key
// ranges against one shared table while readers search and drain the
// snapshot iterators.  It is primarily a test for the race detector
// (`make race`); the final state must be exactly the union of the ranges.
func TestConcurrentInsertSearchIterate(t *testing.T) {
	ht := newTestTab(16, 0)
	const writers = 8
	const perWriter = 200
	const total = writers * perWriter

	var wg sync.WaitGroup
	for w := range writers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range perWriter {
				ht.Insert(TestData{S: fmt.Sprintf("w%d-%04d", w, i), N: w*perWriter + i})
			}
		}(w)
	}

	// Readers run until the writers finish.  Searches may hit or miss
	// (writers are in flight); the iterator must yield a consistent snapshot
	// of at most the total number of elements.
	stop := make(chan struct{})
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				ht.Search(TestData{S: "w3-0042"})
				n := 0
				for range ht.All() { // snapshot; safe alongside the writers
					n++
				}
				if n > total {
					t.Errorf("All yielded %d elements, more than the %d ever inserted", n, total)
					return
				}
			}
		})
	}

	wg.Wait()
	close(stop)
	readers.Wait()

	if ht.Len() != total {
		t.Fatalf("Expected length %d, got %d", total, ht.Len())
	}
	for w := range writers {
		for i := range perWriter {
			v, found := ht.Search(TestData{S: fmt.Sprintf("w%d-%04d", w, i)})
			if !found {
				t.Fatalf("Expected to find w%d-%04d after all writers finished", w, i)
			}
			if v.N != w*perWriter+i {
				t.Fatalf("w%d-%04d stored satellite %d, want %d", w, i, v.N, w*perWriter+i)
			}
		}
	}
}

// TestConcurrentDelete fills a table and then deletes disjoint key ranges
// from concurrent goroutines while readers search and iterate.  After the
// wait the table must be empty and every key not-found.  Race-detector
// target (`make race`).
func TestConcurrentDelete(t *testing.T) {
	ht := newTestTab(16, 0)
	const deleters = 8
	const perDeleter = 100
	key := func(d, i int) string { return fmt.Sprintf("d%d-%04d", d, i) }

	for d := range deleters {
		for i := range perDeleter {
			ht.Insert(TestData{S: key(d, i)})
		}
	}

	var wg sync.WaitGroup
	for d := range deleters {
		wg.Add(1)
		go func(d int) {
			defer wg.Done()
			for i := range perDeleter {
				if !ht.Delete(TestData{S: key(d, i)}) {
					t.Errorf("Expected to delete %q", key(d, i))
					return
				}
			}
		}(d)
	}

	stop := make(chan struct{})
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				ht.Search(TestData{S: key(3, 42)}) // found or not: both legal mid-flight
				for range ht.Values() {            // snapshot; safe alongside the deleters
				}
			}
		})
	}

	wg.Wait()
	close(stop)
	readers.Wait()

	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Fatalf("Expected an empty table after all deleters finished, got length %d", ht.Len())
	}
	for d := range deleters {
		for i := range perDeleter {
			if _, found := ht.Search(TestData{S: key(d, i)}); found {
				t.Fatalf("%q should be gone", key(d, i))
			}
		}
	}
}

// TestConcurrentCompound mixes compound Lock+Nl sections with plain locked
// operations: writers do read-modify-write under Lock while other goroutines
// insert and iterate.  Race-detector target (`make race`).
func TestConcurrentCompound(t *testing.T) {
	ht := newTestTab(16, 0)

	const writers = 4
	const rounds = 200

	var wg sync.WaitGroup
	// Compound writers: atomically replace xNNN's satellite (an insert that
	// reports "added" becomes the insert, one that reports "replaced" bumps).
	for w := range writers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for r := range rounds {
				k := TestData{S: fmt.Sprintf("x%03d", r%50), N: w}
				ht.Lock()
				ht.NlInsert(k)
				ht.Unlock()
			}
		}(w)
	}
	// Plain writers on a disjoint key space, plus snapshot iterators.
	for w := range 2 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for r := range rounds {
				ht.Insert(TestData{S: fmt.Sprintf("y%d-%03d", w, r%50), N: r})
				for range ht.Values() {
				}
			}
		}(w)
	}
	wg.Wait()

	if ht.Len() != 150 { // 50 x-keys + 2*50 y-keys
		t.Fatalf("Expected length 150, got %d", ht.Len())
	}
	checkInvariants(t, ht)
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// upperString is a string with its own JSON representation, to verify
// that element-level marshalers are honored through the table.
type upperString string

func (u upperString) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strings.ToUpper(string(u)) + `"`), nil
}

func (u *upperString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*u = upperString(s)
	return nil
}

// sortedElements decodes a marshaled table and sorts by key, so tests can
// compare contents without depending on bucket order (which varies with the
// hash seed and the growth history).
func sortedElements(t *testing.T, b []byte) []TestData {
	t.Helper()
	var items []TestData
	if err := json.Unmarshal(b, &items); err != nil {
		t.Fatalf("the table's JSON did not decode as an array: %v", err)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].S < items[j].S })
	return items
}

func TestMarshalJSON(t *testing.T) {
	// The elements are all present, as a JSON array (bucket order varies).
	ht := newTestTab(16, 0)
	for _, s := range []string{"c", "a", "b"} {
		ht.Insert(TestData{S: s})
	}
	b, err := json.Marshal(ht)
	if err != nil {
		t.Fatalf("json.Marshal(table): %v", err)
	}
	if got := fmt.Sprint(sortedElements(t, b)); got != "[{a 0} {b 0} {c 0}]" {
		t.Errorf("Expected [{a 0} {b 0} {c 0}], got %s (raw %s)", got, b)
	}

	// An empty table encodes as [].
	if b, err := json.Marshal(newTestTab(16, 0)); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty table, got (%s, %v)", b, err)
	}

	// A zero-value table is a tolerated read: [].
	var zero HashTab[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value table, got (%s, %v)", b, err)
	}

	// A direct call on a nil table encodes as []; json.Marshal on a nil
	// *HashTab never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilTable *HashTab[int]
	if b, err := nilTable.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-table call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTable); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil table, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := NewHashTab[upperString](16, 0)
	custom.Insert("x")
	custom.Insert("y")
	b, err = json.Marshal(custom)
	if err != nil {
		t.Fatalf("json.Marshal(custom): %v", err)
	}
	var elems []string
	if err := json.Unmarshal(b, &elems); err != nil {
		t.Fatalf("element-level JSON did not decode: %v", err)
	}
	sort.Strings(elems)
	if fmt.Sprint(elems) != "[X Y]" {
		t.Errorf(`Expected [X Y], got %v (raw %s)`, elems, b)
	}

	// Encoding errors pass through unchanged.
	bad := NewHashTab[chan int](16, 0)
	bad.Insert(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a table of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Every decoded element is stored and searchable.
	ht := NewHashTab[string](16, 0)
	if err := json.Unmarshal([]byte(`["c","a"]`), ht); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if ht.Len() != 2 {
		t.Errorf("Len = %d, want 2", ht.Len())
	}
	for _, want := range []string{"c", "a"} {
		if _, found := ht.Search(want); !found {
			t.Errorf("%q not found after unmarshal", want)
		}
	}

	// A round trip rebuilds a structurally sound table and keeps the
	// equality/hash functions (Search works on the rebuilt table).
	items := newTestTab(16, 0)
	for i, s := range []string{"a", "b", "c"} {
		items.Insert(TestData{S: s, N: i})
	}
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestTab(16, 0)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, again)
	for i, s := range []string{"a", "b", "c"} {
		if v, found := again.Search(TestData{S: s}); !found || v.N != i {
			t.Errorf("Search(%q) = (%v, %v) after round trip, want N=%d", s, v, found, i)
		}
	}

	// Unmarshaling replaces the contents; it does not add to them.
	if err := json.Unmarshal([]byte(`["x"]`), ht); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := ht.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}
	if _, found := ht.Search("c"); found {
		t.Errorf("old element \"c\" should be gone after replacement")
	}

	// An empty array and null clear the table.
	full := newTestTab(16, 0)
	full.Insert(TestData{S: "z"})
	if err := json.Unmarshal([]byte("[]"), full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the table.")
	}
	full.Insert(TestData{S: "z"})
	if err := json.Unmarshal([]byte("null"), full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the table.")
	}
	checkInvariants(t, full)

	// A duplicate within the data settles as the last occurrence, as with
	// repeated Insert.
	dup := newTestTab(16, 0)
	if err := json.Unmarshal([]byte(`[{"S":"k","N":1},{"S":"k","N":2}]`), dup); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, found := dup.Search(TestData{S: "k"}); !found || v.N != 2 || dup.Len() != 1 {
		t.Errorf("duplicate decode: (%v, %v) Len=%d, want N=2 found Len=1", v, found, dup.Len())
	}

	// Element-level unmarshalers are honored.
	custom := NewHashTab[upperString](16, 0)
	if err := json.Unmarshal([]byte(`["X","Y"]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	for _, want := range []upperString{"X", "Y"} {
		if _, found := custom.Search(want); !found {
			t.Errorf("%q not found after unmarshal", want)
		}
	}

	// Decode errors are returned and leave the table untouched.
	keep := newTestTab(16, 0)
	keep.Insert(TestData{S: "keep", N: 9})
	for _, badData := range []string{"[1,", `[{"S":3}]`, `{"S":"a"}`, "7", `[{"S":"a"},3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if keep.Len() != 1 {
			t.Errorf("Table changed after the error on %s: Len = %d", badData, keep.Len())
		}
		if v, found := keep.Search(TestData{S: "keep"}); !found || v.N != 9 {
			t.Errorf("element lost after the error on %s: (%v, %v)", badData, v, found)
		}
	}
	checkInvariants(t, keep)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil or zero-value table panics with a
// message naming the method and the fix, while [] and null — which store
// nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero HashTab[TestData]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value table to be tolerated, got %v", data, err)
		}
	}
	expectPanicMessage(t, "UnmarshalJSON on zero-value table", "no equality/hash functions",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })
	expectPanicMessage(t, "UnmarshalJSON on zero-value table names the constructors", "NewHashTab",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })

	var nilTable *HashTab[TestData]
	if err := nilTable.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil table to be tolerated, got %v", err)
	}
	expectPanicMessage(t, "UnmarshalJSON on nil table", "hash_grow_ts: UnmarshalJSON called on a nil table",
		func() { _ = nilTable.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })
}

// TestJSONStructField marshals and unmarshals a HashTab nested in a struct
// through the encoding/json package.  The table must be created with
// NewHashTab/NewHashTabFunc before unmarshaling: for a nil *HashTab field the
// json package allocates a zero-value table itself (no equality/hash
// functions), so non-empty data panics with the insert-family message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string           `json:"title"`
		Tags  *HashTab[string] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: NewHashTab[string](16, 0)}
	d.Tags.Insert("ds")
	d.Tags.Insert("go")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var raw struct {
		Title string   `json:"title"`
		Tags  []string `json:"tags"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("document did not decode: %v", err)
	}
	sort.Strings(raw.Tags)
	if raw.Title != "pluto" || fmt.Sprint(raw.Tags) != "[ds go]" {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created table field.
	var out Doc
	out.Tags = NewHashTab[string](16, 0)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if out.Tags.Len() != 2 {
		t.Errorf("Expected 2 tags, got %d", out.Tags.Len())
	}
	for _, want := range []string{"ds", "go"} {
		if _, found := out.Tags.Search(want); !found {
			t.Errorf("tag %q not found after unmarshal", want)
		}
	}

	// A nil table field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created table and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: NewHashTab[string](16, 0)}
	clearDoc.Tags.Insert("gone")
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the table.")
	}

	// Non-empty data into a nil *HashTab field: the json package allocates
	// a zero-value table, and the insert contract panics through
	// json.Unmarshal (it does not recover panics).
	expectPanicMessage(t, "unmarshal into an uncreated table field", "NewHashTab", func() {
		var bad Doc
		_ = json.Unmarshal([]byte(`{"title":"x","tags":["a"]}`), &bad)
	})
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling against a
// map reference model at fixed seed: the table's JSON decoded into a fresh
// table must carry exactly the model's contents.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20260902))
	const ops = 400

	ht := newTestTab(16, 0)
	model := map[string]int{}

	for step := 0; step < ops; step++ {
		k := fmt.Sprintf("%d", rng.Intn(200))
		if rng.Intn(10) < 7 { // insert
			ht.Insert(TestData{S: k, N: step})
			model[k] = step
		} else { // delete
			ht.Delete(TestData{S: k})
			delete(model, k)
		}

		if step%50 == 49 {
			b, err := json.Marshal(ht)
			if err != nil {
				t.Fatalf("step %d: %v", step, err)
			}
			fresh := newTestTab(16, 0)
			if err := json.Unmarshal(b, fresh); err != nil {
				t.Fatalf("step %d: %v", step, err)
			}
			if fresh.Len() != len(model) {
				t.Fatalf("step %d: round-tripped Len = %d, model has %d", step, fresh.Len(), len(model))
			}
			for k, n := range model {
				if v, found := fresh.Search(TestData{S: k}); !found || v.N != n {
					t.Fatalf("step %d: round trip disagrees on %q: found=%v v=%v model N=%d", step, k, found, v, n)
				}
			}
		}
	}
	checkInvariants(t, ht)
}

// TestJSONConcurrent hammers MarshalJSON and UnmarshalJSON concurrently with
// a marshaling reader; every output must be a valid JSON array.  Run under
// -race (make race).
func TestJSONConcurrent(t *testing.T) {
	ht := newTestTab(16, 0)

	const workers = 8
	const perWorker = 100

	stop := make(chan struct{})
	var writers sync.WaitGroup
	var readers sync.WaitGroup

	// A marshaling reader: MarshalJSON snapshots under the read lock, so it
	// is safe while the writers replace the contents.
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			b, err := ht.MarshalJSON()
			if err != nil {
				t.Errorf("MarshalJSON: %v", err)
				return
			}
			var probe []TestData
			if err := json.Unmarshal(b, &probe); err != nil {
				t.Errorf("MarshalJSON produced invalid JSON %s: %v", b, err)
				return
			}
		}
	}()

	// Concurrent replacers: each replaces the whole contents with one
	// element of its own.
	for w := range workers {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			for i := range perWorker {
				item := TestData{S: fmt.Sprintf("%02d-%03d", w, i)}
				b, err := json.Marshal([]TestData{item})
				if err != nil {
					t.Errorf("worker %d: %v", w, err)
					return
				}
				if err := ht.UnmarshalJSON(b); err != nil {
					t.Errorf("worker %d: UnmarshalJSON: %v", w, err)
					return
				}
			}
		}(w)
	}

	writers.Wait()
	close(stop)
	readers.Wait()
	checkInvariants(t, ht)
}
