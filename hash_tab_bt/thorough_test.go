package hash_tab_bt

// Thorough tests for the tree-bucket hash table: nil and zero-value
// behavior, the panic contract with its messages, replacement semantics,
// in-order bucket visits, unlinking through the tree deletes, the fixed
// bucket count, and a randomized cross-check against a map model.
// TestData and newTestTab are defined in hash_tab_test.go and reused here.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
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

// checkInvariants verifies the structural invariants of the table: every
// element sits in the bucket its hash reduces to, each bucket tree is
// visited in ascending in-order, the element count matches Len, Search
// finds every stored element, and the All iterator agrees exactly with the
// bucket trees.  Call it after structural changes.
func checkInvariants[T any](t *testing.T, ht *HashTab[T]) {
	t.Helper()
	nodes := 0
	var want []T
	for i := range ht.buckets {
		var prev *T
		for data := range ht.buckets[i].All() {
			if home := ht.bucketOf(ht.hashOf(data)); home != i {
				t.Fatalf("element %v hashes to bucket %d but is stored in bucket %d", data, home, i)
			}
			if prev != nil && ht.cmp(*prev, data) >= 0 {
				t.Fatalf("bucket %d not visited in ascending in-order: %v then %v", i, *prev, data)
			}
			p := data
			prev = &p
			if _, found := ht.Search(data); !found {
				t.Fatalf("element in bucket %d (%v) not found by Search", i, data)
			}
			nodes++
			want = append(want, data)
		}
	}
	if nodes != ht.Len() {
		t.Fatalf("bucket tree elements = %d, Len() = %d", nodes, ht.Len())
	}
	var got []T
	for pos, item := range ht.All() {
		if pos < 0 || pos >= ht.size {
			t.Fatalf("All reported out-of-range position %d", pos)
		}
		got = append(got, item)
	}
	if len(got) != len(want) {
		t.Fatalf("All visited %d elements, bucket trees hold %d", len(got), len(want))
	}
	for i := range want {
		if ht.cmp(want[i], got[i]) != 0 {
			t.Fatalf("All position %d: bucket trees hold %v, All reported %v", i, want[i], got[i])
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

// TestHashTabNilTolerated verifies that a nil table and the zero value
// behave as empty tables for every operation that has a sane answer.
func TestHashTabNilTolerated(t *testing.T) {
	for name, ht := range map[string]*HashTab[TestData]{
		"nil table":  nil,
		"zero value": {},
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
	validCmp := func(a, b TestData) int { return strings.Compare(a.S, b.S) }
	expectPanicMessage(t, "NewHashTabFunc(nil cmp)", "nil comparison function",
		func() { NewHashTabFunc(nil, validHash, 7) })
	expectPanicMessage(t, "NewHashTabFunc(nil hash)", "nil hash function",
		func() { NewHashTabFunc(validCmp, nil, 7) })
}

// TestInsertReplacesExisting verifies that re-inserting an equal key replaces
// the stored value (the satellite data changes) and does not change the
// length, including when the node is deep in its bucket tree.
func TestInsertReplacesExisting(t *testing.T) {
	ht := newTestTab(7)
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

	// Force collisions so the replacement happens deep in a bucket tree.
	home := ht.bucketOf(ht.hashOf(TestData{S: "dup"}))
	used := map[string]bool{"dup": true}
	c1 := findKeyWithHome(t, ht, home, used)
	c2 := findKeyWithHome(t, ht, home, used)
	ht.Insert(TestData{S: c1, N: 10})
	ht.Insert(TestData{S: c2, N: 20})
	ht.Insert(TestData{S: c1, N: 11}) // replace inside the tree
	if got, found := ht.Search(TestData{S: c1}); !found || got.N != 11 {
		t.Errorf("Replacement in tree should return new value, got %v found=%v", got, found)
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

// TestTreeBucketDeletePositions builds one bucket tree with a constant hash
// function (everything lands in bucket 7 % 5 = 2), verifies that iteration
// runs in the tree's in-order — ascending per the comparison function, not
// newest-first like hash_tab's chains — and then removes the two-children
// root, the minimum and the maximum of the tree.
func TestTreeBucketDeletePositions(t *testing.T) {
	ht := NewHashTabFunc(
		func(a, b string) int { return strings.Compare(a, b) },
		func(s string) uint64 { return 7 }, // every key lands in bucket 2
		5,
	)
	// Insertion order chosen so the tree root has two children: "mid" is
	// the root, "head-old" its left child, "tail-new" its right child.
	for _, k := range []string{"mid", "head-old", "tail-new"} {
		if !ht.Insert(k) {
			t.Fatalf("Expected insert of %q to be an add", k)
		}
	}
	if ht.Len() != 3 {
		t.Fatalf("Expected length 3, got %d", ht.Len())
	}
	if ht.buckets[2].IsEmpty() || ht.buckets[2].Len() != 3 {
		t.Fatalf("Expected a 3-element tree in bucket 2")
	}

	// Trees visit in-order: ascending, oldest/newest irrelevant.
	var order []string
	for pos, item := range ht.All() {
		if pos != 2 {
			t.Fatalf("All reported position %d, want 2", pos)
		}
		order = append(order, item)
	}
	want := []string{"head-old", "mid", "tail-new"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("In-order bucket visit = %v, want %v (ascending)", order, want)
		}
	}

	// Remove the two-children root (the in-order successor is promoted).
	if !ht.Delete("mid") {
		t.Fatalf("Expected to delete mid")
	}
	if _, found := ht.Search("mid"); found {
		t.Errorf("mid should be gone")
	}
	if _, found := ht.Search("head-old"); !found {
		t.Errorf("head-old must survive the root delete")
	}
	if _, found := ht.Search("tail-new"); !found {
		t.Errorf("tail-new must survive the root delete")
	}
	checkInvariants(t, ht)

	// Remove the minimum.
	if !ht.Delete("head-old") {
		t.Fatalf("Expected to delete head-old")
	}
	if _, found := ht.Search("tail-new"); !found {
		t.Errorf("tail-new must survive the minimum delete")
	}
	checkInvariants(t, ht)

	// Remove the last element — the bucket tree empties but the tree
	// itself stays (unlike hash_tab's nil-able chains).
	if !ht.Delete("tail-new") {
		t.Fatalf("Expected to delete tail-new")
	}
	if !ht.IsEmpty() || !ht.buckets[2].IsEmpty() {
		t.Errorf("Expected empty table and empty bucket tree after draining, got length %d", ht.Len())
	}
	// Deleting from the drained tree returns false.
	if ht.Delete("anything") {
		t.Errorf("Delete on a drained bucket should return false")
	}
	checkInvariants(t, ht)
}

// TestHashZeroNotSpecial verifies that a hash of 0 is just another hash —
// unlike hash_grow there is no reserved zero marker to remap, because an
// empty bucket is an empty tree.  (The tree nodes keep no raw hash, so
// unlike hash_tab's chains there is nothing to inspect — placement is
// asserted through All.)
func TestHashZeroNotSpecial(t *testing.T) {
	ht := NewHashTabFunc(
		func(a, b string) int { return strings.Compare(a, b) },
		func(s string) uint64 { return 0 }, // every key lands in bucket 0
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
	if ht.buckets[0].IsEmpty() {
		t.Fatalf("Expected the zero hash to land in bucket 0")
	}
	for _, k := range []string{"a", "b", "c"} {
		if _, found := ht.Search(k); !found {
			t.Errorf("Expected to find %q, did not", k)
		}
	}
	// Delete from the middle of the single tree.
	if !ht.Delete("b") {
		t.Fatalf("Expected to delete b")
	}
	if _, found := ht.Search("b"); found {
		t.Errorf("b should be gone")
	}
	if _, found := ht.Search("c"); !found {
		t.Errorf("c must survive the delete")
	}
	for pos := range ht.All() {
		if pos != 0 {
			t.Fatalf("All reported position %d, want 0", pos)
		}
	}
	checkInvariants(t, ht)
}

// TestSizeNeverGrows verifies that the bucket count is fixed for the life of
// the table — the tree-bucket table never re-hashes, no matter the load.
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

// TestRandomizedModel runs a fixed-seed pseudo-random mix of Insert, Delete
// and Search operations, cross-checking every result (including the
// satellite value of the latest insert) against a map model and periodically
// validating structural invariants.
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
