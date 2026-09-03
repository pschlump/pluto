package hash_tab_dll_ts

// Thorough tests for the thread-safe dll-bucket hash table: nil and
// zero-value behavior, the panic contract with its messages, replacement
// semantics and handle liveness, chain construction and splicing, the fixed
// bucket count, snapshot iterator semantics, compound Lock+Nl operations
// (including the atomic locate-then-splice), a randomized cross-check
// against a map model, encoding/json round trips, and concurrent access.
// TestData and newTestTab are defined in hash_tab_test.go and reused here.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/pschlump/pluto/dll"
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

// checkInvariants verifies the structural invariants of the table: every
// element sits in the bucket its data hashes to, the bucket lists together
// hold exactly Len() elements, Search finds every stored element, and Walk
// and the All iterator agree exactly with the bucket lists.  The dll's own
// fields are not visible from this package, so the buckets are inspected
// through their public All and Len.  Call it after structural changes — in
// single-goroutine tests only (it reads the internals without the lock).
func checkInvariants[T comparable](t *testing.T, ht *HashTab[T]) {
	t.Helper()
	nodes := 0
	var want []T
	for i := range ht.buckets {
		for _, data := range ht.buckets[i].All() {
			if home := ht.bucketOf(ht.hashOf(data)); home != i {
				t.Fatalf("element %v is chained in bucket %d, want bucket %d", data, i, home)
			}
			if _, found := ht.Search(data); !found {
				t.Fatalf("element in bucket %d (%v) not found by Search", i, data)
			}
			nodes++
			want = append(want, data)
		}
	}
	if nodes != ht.Len() {
		t.Fatalf("chained nodes = %d, Len() = %d", nodes, ht.Len())
	}
	total := 0
	for i := range ht.buckets {
		total += ht.buckets[i].Len()
	}
	if total != ht.Len() {
		t.Fatalf("bucket lists hold %d elements, Len() = %d", total, ht.Len())
	}
	var got []T
	for pos, item := range ht.All() {
		if pos < 0 || pos >= ht.size {
			t.Fatalf("All reported out-of-range position %d", pos)
		}
		got = append(got, item)
	}
	if len(got) != len(want) {
		t.Fatalf("All visited %d elements, bucket lists hold %d", len(got), len(want))
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("All position %d: bucket lists hold %v, All reported %v", i, want[i], got[i])
		}
	}
	nWalk := 0
	ht.Walk(func(pos int, data T) bool {
		nWalk++
		return true
	})
	if nWalk != ht.Len() {
		t.Fatalf("Walk visited %d, Len() = %d", nWalk, ht.Len())
	}
}

// foreignElement builds a live element of a standalone dll list, so tests
// can check DeleteFound's behavior when handed an element that did not come
// from the table (the contract says that is undefined on a non-empty table,
// but an empty table reports not-found before any bucket is touched).
func foreignElement(t *testing.T) *dll.DllElement[TestData] {
	t.Helper()
	d := dll.NewDllFunc(func(a, b TestData) bool { return a.S == b.S })
	d.InsertBeforeHead(TestData{S: "foreign"})
	el, pos := d.Search(TestData{S: "foreign"})
	if el == nil || pos < 0 {
		t.Fatalf("Setup failed: could not create a live DllElement")
	}
	return el
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
		if el, found := ht.Search(TestData{S: "x"}); found || el != nil {
			t.Errorf("%s: Search should report not-found, got %v %v", name, el, found)
		}
		if ht.Delete(TestData{S: "x"}) {
			t.Errorf("%s: Delete should return false", name)
		}
		if ht.DeleteFound(nil) {
			t.Errorf("%s: DeleteFound(nil) should return false", name)
		}
		if ht.DeleteFound(foreignElement(t)) {
			t.Errorf("%s: DeleteFound should return false on an empty table", name)
		}
		if ht.Len() != 0 {
			t.Errorf("%s: length should still be 0 after the failed deletes, got %d", name, ht.Len())
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
	if zeroTable.NlDeleteFound(nil) {
		t.Errorf("NlDeleteFound(nil) on zero value should return false")
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
		func() { NewHashTabFunc(nil, validHash, 7) })
	expectPanicMessage(t, "NewHashTabFunc(nil hash)", "nil hash function",
		func() { NewHashTabFunc(validEq, nil, 7) })

	// NlInsert (the Insert body) carries the same zero-value panic.
	zero := &HashTab[TestData]{}
	expectPanicMessage(t, "NlInsert on zero value", "NewHashTab",
		func() { zero.NlInsert(TestData{S: "x"}) })
}

// TestNewHashTabSize verifies that a table with fewer than 5 buckets panics,
// that the minimum size of 5 is accepted, and that every bucket is an
// initialized empty list.
func TestNewHashTabSize(t *testing.T) {
	for _, n := range []int{-1, 0, 1, 4} {
		expectPanic(t, fmt.Sprintf("NewHashTab(%d)", n), func() {
			NewHashTab[TestData](n)
		})
	}

	ht := NewHashTab[TestData](5)
	if ht == nil {
		t.Fatalf("Expected NewHashTab(5) to succeed")
	}
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Errorf("Expected new table to be empty, got length %d", ht.Len())
	}
	if got := len(ht.buckets); got != 5 {
		t.Errorf("Expected 5 buckets, got %d", got)
	}
	for i, b := range ht.buckets {
		if b == nil {
			t.Errorf("Expected bucket %d to be initialized", i)
		} else if b.Len() != 0 {
			t.Errorf("Expected bucket %d to be empty, got length %d", i, b.Len())
		}
	}
}

// TestInsertReplacesExisting verifies that re-inserting an equal key replaces
// the stored value in place (the satellite data changes, the length does not,
// and a previously returned handle observes the new value), including when
// the key is not the head of its chain.
func TestInsertReplacesExisting(t *testing.T) {
	ht := newTestTab(7)
	if added := ht.Insert(TestData{S: "dup", N: 1}); !added {
		t.Errorf("First insert of a key should report added=true")
	}
	el, found := ht.Search(TestData{S: "dup"})
	if !found {
		t.Fatalf("Expected to find the inserted key")
	}
	if added := ht.Insert(TestData{S: "dup", N: 2}); added {
		t.Errorf("Duplicate insert should report added=false (replaced)")
	}
	if ht.Len() != 1 {
		t.Fatalf("Duplicate insert should keep length 1, got %d", ht.Len())
	}
	if got := el.GetData(); got.N != 2 {
		t.Errorf("The handle should observe the replacement, got %v", got)
	}
	if got, found := ht.Search(TestData{S: "dup"}); !found || got.GetData().N != 2 {
		t.Errorf("Search should return the replacement value, got %v found=%v", got.GetData(), found)
	}

	// Force collisions so the replacement happens deep in a chain.
	home := ht.bucketOf(ht.hashOf(TestData{S: "dup"}))
	used := map[string]bool{"dup": true}
	c1 := findKeyWithHome(t, ht, home, used)
	c2 := findKeyWithHome(t, ht, home, used)
	ht.Insert(TestData{S: c1, N: 10})
	ht.Insert(TestData{S: c2, N: 20}) // chains ahead of c1
	elc1, _ := ht.Search(TestData{S: c1})
	ht.Insert(TestData{S: c1, N: 11}) // replace inside the chain
	if elc1.GetData().N != 11 {
		t.Errorf("Deep handle should observe the replacement, got %v", elc1.GetData())
	}
	if got, found := ht.Search(TestData{S: c1}); !found || got.GetData().N != 11 {
		t.Errorf("Replacement in chain should return new value, got %v found=%v", got.GetData(), found)
	}
	if ht.Len() != 3 {
		t.Errorf("Expected length 3 after collision replacements, got %d", ht.Len())
	}
	checkInvariants(t, ht)
}

// findKeyWithHome returns a fresh key whose home bucket is `home`, recording
// it in `used` so keys are never repeated.
func findKeyWithHome(t *testing.T, ht *HashTab[TestData], home int, used map[string]bool) string {
	t.Helper()
	for i := 0; ; i++ {
		s := fmt.Sprintf("w%06d", i)
		if used[s] {
			continue
		}
		if ht.bucketOf(ht.hashOf(TestData{S: s})) == home {
			used[s] = true
			return s
		}
	}
}

// TestChainDeletePositions builds one chain with a constant hash function
// (everything lands in bucket 7 % 5 = 2), verifies that iteration runs from
// the most recently inserted element to the oldest (the bucket lists run
// head-newest to tail-oldest, like hash_tab's chains), and then unlinks the
// middle, the head and the tail of the chain by value.
func TestChainDeletePositions(t *testing.T) {
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

	// Chains run newest-first.
	var order []string
	for pos, item := range ht.All() {
		if pos != 2 {
			t.Fatalf("All reported position %d, want 2", pos)
		}
		order = append(order, item)
	}
	want := []string{"tail-new", "mid", "head-old"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("Chain order = %v, want %v (newest first)", order, want)
		}
	}

	// Unlink the middle.
	if !ht.Delete("mid") {
		t.Fatalf("Expected to delete mid")
	}
	if _, found := ht.Search("mid"); found {
		t.Errorf("mid should be gone")
	}
	if _, found := ht.Search("head-old"); !found {
		t.Errorf("head-old must survive the middle unlink")
	}
	if _, found := ht.Search("tail-new"); !found {
		t.Errorf("tail-new must survive the middle unlink")
	}
	checkInvariants(t, ht)

	// Unlink the head.
	if !ht.Delete("tail-new") {
		t.Fatalf("Expected to delete tail-new (the chain head)")
	}
	if _, found := ht.Search("head-old"); !found {
		t.Errorf("head-old must survive the head unlink")
	}
	checkInvariants(t, ht)

	// Unlink the last node.
	if !ht.Delete("head-old") {
		t.Fatalf("Expected to delete head-old")
	}
	if !ht.IsEmpty() || ht.buckets[2].Len() != 0 {
		t.Errorf("Expected empty table and empty bucket after draining, got length %d", ht.Len())
	}
	// Deleting from the drained chain returns false.
	if ht.Delete("anything") {
		t.Errorf("Delete on a drained chain should return false")
	}
	checkInvariants(t, ht)
}

// TestHashZeroNotSpecial verifies that a hash of 0 is just another hash —
// unlike hash_grow_ts there is no reserved zero marker to remap, because an
// empty bucket is an empty list.
func TestHashZeroNotSpecial(t *testing.T) {
	ht := NewHashTabFunc(
		func(a, b string) bool { return a == b },
		func(s string) uint64 { return 0 }, // every key chains in bucket 0
		5,
	)
	for _, k := range []string{"a", "b", "c"} {
		if !ht.Insert(k) {
			t.Errorf("Expected insert of %q to be an add", k)
		}
	}
	if ht.Len() != 3 {
		t.Fatalf("Expected length 3, got %d", ht.Len())
	}
	if got := ht.buckets[0].Len(); got != 3 {
		t.Fatalf("Expected the zero hash to chain all 3 keys in bucket 0, got %d", got)
	}
	for _, k := range []string{"a", "b", "c"} {
		if _, found := ht.Search(k); !found {
			t.Errorf("Expected to find %q, did not", k)
		}
	}
	// Delete from the middle of the degenerate chain.
	if !ht.Delete("b") {
		t.Fatalf("Expected to delete b")
	}
	if _, found := ht.Search("b"); found {
		t.Errorf("b should be gone")
	}
	if _, found := ht.Search("c"); !found {
		t.Errorf("c must survive the unlink")
	}
	checkInvariants(t, ht)
}

// TestSizeNeverGrows verifies that the bucket count is fixed for the life of
// the table — the table never re-hashes, no matter the load.
func TestSizeNeverGrows(t *testing.T) {
	ht := newTestTab(7)
	for i := range 200 {
		ht.Insert(TestData{S: fmt.Sprintf("g%03d", i)})
	}
	if ht.size != 7 {
		t.Errorf("Expected the size to stay 7, got %d", ht.size)
	}
	if ht.Len() != 200 {
		t.Errorf("Expected length 200, got %d", ht.Len())
	}
	for i := range 200 {
		if _, found := ht.Search(TestData{S: fmt.Sprintf("g%03d", i)}); !found {
			t.Errorf("Expected to find g%03d, did not", i)
		}
	}
	checkInvariants(t, ht)
}

// TestIteratorSnapshotSemantics verifies that All and Values iterate a
// snapshot copied under the read lock when they are called: mutating the
// table afterwards — or from inside the loop body — does not affect the
// iteration and does not race.
func TestIteratorSnapshotSemantics(t *testing.T) {
	ht := newTestTab(16)
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
	ht2 := newTestTab(16)
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
// compound operations — here the package's signature move: an atomic
// NlSearch followed by an O(1) NlDeleteFound splice of the located handle,
// plus a bulk insert, all under one lock hold.
func TestLockNlCompound(t *testing.T) {
	ht := newTestTab(16)
	for i := range 40 {
		ht.Insert(TestData{S: fmt.Sprintf("c%03d", i), N: i})
	}

	ht.Lock()
	if el, found := ht.NlSearch(TestData{S: "c021"}); found {
		if got := el.GetData(); got.N != 21 {
			t.Errorf("NlSearch returned stale satellite %d, want 21", got.N)
		}
		if !ht.NlDeleteFound(el) {
			t.Errorf("NlDeleteFound inside the held lock should succeed")
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
		if el, found := ht.Search(TestData{S: fmt.Sprintf("c%03d", i)}); !found || el.GetData().N != i {
			t.Errorf("Expected to find c%03d with satellite %d, got %v found=%v", i, i, el.GetData(), found)
		}
	}
	checkInvariants(t, ht)
}

// TestRandomizedModel runs a fixed-seed pseudo-random mix of Insert, Delete,
// Search and Search+DeleteFound operations, cross-checking every result
// (including the satellite value of the latest insert) against a map model
// and periodically validating structural invariants.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(12345)) // fixed seed: deterministic run
	ht := newTestTab(7)
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
		case 4: // search, occasionally removing the located element
			wantN, want := model[k]
			el, found := ht.Search(TestData{S: k})
			if found != want {
				t.Fatalf("op %d: Search(%q) found=%v, model says %v", op, k, found, want)
			}
			if found {
				if el.GetData().N != wantN {
					t.Fatalf("op %d: Search(%q) returned stale satellite %d, want %d", op, k, el.GetData().N, wantN)
				}
				if rng.Intn(3) == 0 { // exercise the O(1) splice
					if !ht.DeleteFound(el) {
						t.Fatalf("op %d: DeleteFound of %q failed", op, k)
					}
					delete(model, k)
				}
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

// -------------------------------------------------------------------------------------------------------
// Thread-safety: snapshot iterators and concurrent access
// -------------------------------------------------------------------------------------------------------

// TestConcurrentInsertSearchIterate runs writers inserting disjoint key
// ranges against one shared table while readers search and drain the
// snapshot iterators.  It is primarily a test for the race detector
// (`make race`); the final state must be exactly the union of the ranges.
func TestConcurrentInsertSearchIterate(t *testing.T) {
	ht := newTestTab(16)
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
			el, found := ht.Search(TestData{S: fmt.Sprintf("w%d-%04d", w, i)})
			if !found {
				t.Fatalf("Expected to find w%d-%04d after all writers finished", w, i)
			}
			if el.GetData().N != w*perWriter+i {
				t.Fatalf("w%d-%04d stored satellite %d, want %d", w, i, el.GetData().N, w*perWriter+i)
			}
		}
	}
}

// TestConcurrentDelete fills a table and then deletes disjoint key ranges
// from concurrent goroutines while readers search and iterate.  After the
// wait the table must be empty and every key not-found.  Race-detector
// target (`make race`).
func TestConcurrentDelete(t *testing.T) {
	ht := newTestTab(16)
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

// TestConcurrentCompound mixes compound Lock+Nl sections — the atomic
// NlSearch + NlDeleteFound splice this package exists for — with plain
// locked operations and snapshot iteration.  Each compound goroutine owns a
// disjoint key range, so every splice must succeed.  Race-detector target
// (`make race`).
func TestConcurrentCompound(t *testing.T) {
	ht := newTestTab(16)

	const compounders = 4
	const perCompounder = 100
	cKey := func(c, i int) string { return fmt.Sprintf("c%d-%03d", c, i) }

	// Fill the compound key space up front.
	for c := range compounders {
		for i := range perCompounder {
			ht.Insert(TestData{S: cKey(c, i), N: c*perCompounder + i})
		}
	}

	var wg sync.WaitGroup
	// Compound goroutines: locate each element with NlSearch and splice it
	// out with NlDeleteFound, atomically under one lock hold.
	for c := range compounders {
		wg.Add(1)
		go func(c int) {
			defer wg.Done()
			for i := range perCompounder {
				ht.Lock()
				el, found := ht.NlSearch(TestData{S: cKey(c, i)})
				if !found {
					ht.Unlock()
					t.Errorf("Expected to locate %q inside the held lock", cKey(c, i))
					return
				}
				if !ht.NlDeleteFound(el) {
					ht.Unlock()
					t.Errorf("Expected the O(1) splice of %q to succeed", cKey(c, i))
					return
				}
				ht.Unlock()
			}
		}(c)
	}
	// Plain writers on a disjoint key space, plus snapshot iterators.
	for w := range 2 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for r := range 200 {
				ht.Insert(TestData{S: fmt.Sprintf("y%d-%03d", w, r%50), N: r})
				for range ht.Values() {
				}
			}
		}(w)
	}
	wg.Wait()

	if ht.Len() != 100 { // all c-keys spliced out, 2*50 y-keys remain
		t.Fatalf("Expected length 100, got %d", ht.Len())
	}
	for c := range compounders {
		for i := range perCompounder {
			if _, found := ht.Search(TestData{S: cKey(c, i)}); found {
				t.Fatalf("%q should have been spliced out", cKey(c, i))
			}
		}
	}
	for w := range 2 {
		for r := range 50 {
			if _, found := ht.Search(TestData{S: fmt.Sprintf("y%d-%03d", w, r)}); !found {
				t.Fatalf("y%d-%03d should be present", w, r)
			}
		}
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

// newUpperTab builds a table of upperString with the usual field-style
// equality and hash functions.
func newUpperTab(n int) *HashTab[upperString] {
	return NewHashTabFunc(
		func(a, b upperString) bool { return a == b },
		func(v upperString) uint64 {
			h := fnv.New64a()
			_, _ = h.Write([]byte(v))
			return h.Sum64()
		},
		n,
	)
}

// decodeSet decodes a JSON array of TestData into a key->satellite map, so
// tests compare tables as sets — bucket order in the marshaled array is
// hash-dependent and never asserted.
func decodeSet(t *testing.T, b []byte) map[string]int {
	t.Helper()
	var items []TestData
	if err := json.Unmarshal(b, &items); err != nil {
		t.Fatalf("json.Unmarshal of marshaled table: %v", err)
	}
	set := make(map[string]int, len(items))
	for _, it := range items {
		if _, dup := set[it.S]; dup {
			t.Fatalf("marshaled array holds %q twice", it.S)
		}
		set[it.S] = it.N
	}
	return set
}

// equalSets reports whether two key->satellite maps are identical.
func equalSets(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func TestMarshalJSON(t *testing.T) {
	// The elements marshal as one JSON array; compare as a set because
	// bucket order is hash-dependent.
	ht := newTestTab(7)
	for i, s := range []string{"a", "b", "c"} {
		ht.Insert(TestData{S: s, N: i})
	}
	b, err := json.Marshal(ht)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if got, want := decodeSet(t, b), map[string]int{"a": 0, "b": 1, "c": 2}; !equalSets(got, want) {
		t.Errorf("Expected set %v, got %v from %s", want, got, b)
	}

	// An empty table encodes as [].
	empty := newTestTab(7)
	if b, err := json.Marshal(empty); err != nil || string(b) != "[]" {
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
	custom := newUpperTab(7)
	custom.Insert("x")
	custom.Insert("y")
	b, err = json.Marshal(custom)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var ups []string
	if err := json.Unmarshal(b, &ups); err != nil {
		t.Fatalf("json.Unmarshal of upperString array: %v", err)
	}
	sort.Strings(ups)
	if fmt.Sprint(ups) != "[X Y]" {
		t.Errorf(`Expected [X Y], got %v from %s`, ups, b)
	}

	// Encoding errors pass through unchanged.
	bad := NewHashTab[chan int](8)
	bad.Insert(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a table of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Every decoded element is searchable with its satellite value.
	ht := newTestTab(7)
	if err := json.Unmarshal([]byte(`[{"S":"x","N":1},{"S":"y","N":2}]`), ht); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if ht.Len() != 2 {
		t.Fatalf("Expected length 2, got %d", ht.Len())
	}
	for _, want := range []TestData{{S: "x", N: 1}, {S: "y", N: 2}} {
		if el, found := ht.Search(TestData{S: want.S}); !found || el.GetData() != want {
			t.Errorf("Expected to find %v, got (%v, %v)", want, el, found)
		}
	}
	checkInvariants(t, ht)

	// A round trip rebuilds a structurally sound table and keeps the
	// equality/hash functions (Search works on the rebuilt table).
	full := newTestTab(11)
	for i, s := range []string{"a", "b", "c", "d"} {
		full.Insert(TestData{S: s, N: i})
	}
	b, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestTab(5)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, again)
	if got := decodeSet(t, b); !equalSets(got, map[string]int{"a": 0, "b": 1, "c": 2, "d": 3}) {
		t.Errorf("Unexpected round-trip set %v", got)
	}
	if el, found := again.Search(TestData{S: "c"}); !found || el.GetData().N != 2 {
		t.Errorf("Expected Search to work after unmarshal, got (%v, %v)", el, found)
	}

	// Unmarshaling replaces the contents; it does not append.
	if err := json.Unmarshal([]byte(`[{"S":"only","N":9}]`), full); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if full.Len() != 1 {
		t.Errorf("Expected replacement, got length %d, want 1", full.Len())
	}
	if _, found := full.Search(TestData{S: "a"}); found {
		t.Errorf("Expected the old contents to be replaced.")
	}

	// Equal elements in the array collapse like duplicate Insert calls:
	// the last occurrence wins.
	dup := newTestTab(7)
	if err := json.Unmarshal([]byte(`[{"S":"k","N":1},{"S":"k","N":2}]`), dup); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if dup.Len() != 1 {
		t.Errorf("Expected duplicate keys to collapse to length 1, got %d", dup.Len())
	}
	if el, found := dup.Search(TestData{S: "k"}); !found || el.GetData().N != 2 {
		t.Errorf("Expected the last duplicate to win, got (%v, %v)", el, found)
	}

	// An empty array and null clear the table.
	for _, data := range []string{"[]", "null"} {
		full.Insert(TestData{S: "z"})
		if err := json.Unmarshal([]byte(data), full); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", data, err)
		}
		if !full.IsEmpty() {
			t.Errorf("Expected %s to clear the table.", data)
		}
		checkInvariants(t, full)
	}

	// Element-level unmarshalers are honored.
	custom := newUpperTab(7)
	if err := json.Unmarshal([]byte(`["X","Y"]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var cs []string
	for v := range custom.Values() {
		cs = append(cs, string(v))
	}
	sort.Strings(cs)
	if fmt.Sprint(cs) != "[X Y]" {
		t.Errorf("Expected [X Y], got %v", cs)
	}

	// Decode errors are returned and leave the table untouched.
	keep := newTestTab(7)
	keep.Insert(TestData{S: "keep", N: 1})
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `[{"S":"a"},3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if keep.Len() != 1 {
			t.Errorf("Table changed after the error on %s: length %d", badData, keep.Len())
		}
		if el, found := keep.Search(TestData{S: "keep"}); !found || el.GetData().N != 1 {
			t.Errorf("Table contents changed after the error on %s", badData)
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
	expectPanicMessage(t, "UnmarshalJSON on zero value", "NewHashTab",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })
	expectPanicMessage(t, "UnmarshalJSON on zero value", "UnmarshalJSON",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })

	var nilTable *HashTab[TestData]
	if err := nilTable.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil table to be tolerated, got %v", err)
	}
	expectPanicMessage(t, "UnmarshalJSON on nil table", "UnmarshalJSON called on a nil table",
		func() { _ = nilTable.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })
}

// TestJSONStructField marshals and unmarshals a HashTab nested in a struct
// through the encoding/json package.  The table must be created with
// NewHashTab/NewHashTabFunc before unmarshaling: for a nil *HashTab field
// the json package allocates a zero-value table itself (no equality/hash
// functions), so non-empty data panics with the insert-family message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string           `json:"title"`
		Tags  *HashTab[string] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: NewHashTab[string](8)}
	d.Tags.Insert("ds")
	d.Tags.Insert("go")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	// The tags array is in bucket order — decode and compare as a set.
	var probe struct {
		Title string   `json:"title"`
		Tags  []string `json:"tags"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		t.Fatalf("json.Unmarshal of document: %v", err)
	}
	sort.Strings(probe.Tags)
	if probe.Title != "pluto" || fmt.Sprint(probe.Tags) != "[ds go]" {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created table field.
	out := Doc{Tags: NewHashTab[string](8)}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var tags []string
	for v := range out.Tags.Values() {
		tags = append(tags, v)
	}
	sort.Strings(tags)
	if fmt.Sprint(tags) != "[ds go]" {
		t.Errorf("Expected [ds go], got %v", tags)
	}

	// A nil table field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created table and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: NewHashTab[string](8)}
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
	expectPanicMessage(t, "unmarshal into uncreated table field", "NewHashTab",
		func() {
			var bad Doc
			_ = json.Unmarshal([]byte(`{"title":"x","tags":["a"]}`), &bad)
		})
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling against
// a map reference model at fixed seed; tables compare as sets because the
// marshaled array is in hash-dependent bucket order.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20260903)) // fixed seed: deterministic run
	const ops = 500

	ht := newTestTab(7)
	model := make(map[string]int) // key -> satellite value of the latest insert

	key := func(i int) string { return fmt.Sprintf("j%04d", i) }

	for step := range ops {
		k := key(rng.Intn(150))
		if rng.Intn(4) == 3 { // delete
			ht.Delete(TestData{S: k})
			delete(model, k)
		} else { // insert
			ht.Insert(TestData{S: k, N: step})
			model[k] = step
		}

		// Marshal must hold exactly the model as a set.
		b, err := json.Marshal(ht)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if got := decodeSet(t, b); !equalSets(got, model) {
			t.Fatalf("step %d: marshaled set %v, model %v", step, got, model)
		}

		// Unmarshaling into a fresh table must reproduce the model.
		fresh := newTestTab(7)
		if err := json.Unmarshal(b, fresh); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if fresh.Len() != len(model) {
			t.Fatalf("step %d: round trip length %d, model %d", step, fresh.Len(), len(model))
		}
		for mk, mv := range model {
			if el, found := fresh.Search(TestData{S: mk}); !found || el.GetData().N != mv {
				t.Fatalf("step %d: round trip Search(%q) = (%v, %v), model %d", step, mk, el, found, mv)
			}
		}
	}
	checkInvariants(t, ht)
}

// TestJSONConcurrent hammers MarshalJSON and UnmarshalJSON concurrently
// with writers and a marshaling reader; every output must be a valid JSON
// array.  Run under -race (make race).
func TestJSONConcurrent(t *testing.T) {
	ht := newTestTab(16)

	const workers = 8
	const perWorker = 100

	stop := make(chan struct{})
	var writers sync.WaitGroup
	var readers sync.WaitGroup

	// A marshaling reader: MarshalJSON snapshots under the read lock, so
	// it is safe while the writers replace the contents.
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
				item := TestData{S: fmt.Sprintf("%02d-%03d", w, i), N: i}
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
