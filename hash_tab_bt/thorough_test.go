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
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
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

// newUpperTab builds a table of upperString with a constant hash, so
// every element lands in one bucket and iteration is the tree's in-order
// — ascending — which gives deterministic JSON output.
func newUpperTab() *HashTab[upperString] {
	return NewHashTabFunc(
		func(a, b upperString) int { return strings.Compare(string(a), string(b)) },
		func(u upperString) uint64 { return 7 },
		5,
	)
}

// sortedKeys returns the keys in the table in ascending order, for
// assertions that must not depend on bucket order.
func sortedKeys(ht *HashTab[TestData]) []string {
	keys := []string{}
	for item := range ht.Values() {
		keys = append(keys, item.S)
	}
	sort.Strings(keys)
	return keys
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output with a constant hash: one bucket, ascending.
	ht := NewHashTabFunc(
		func(a, b string) int { return strings.Compare(a, b) },
		func(s string) uint64 { return 7 },
		5,
	)
	for _, v := range []string{"beta", "alpha", "gamma"} {
		ht.Insert(v)
	}
	b, err := json.Marshal(ht)
	if err != nil {
		t.Fatalf("json.Marshal(ht): %v", err)
	}
	if string(b) != `["alpha","beta","gamma"]` {
		t.Errorf(`Expected ["alpha","beta","gamma"], got %s`, b)
	}

	// Struct elements use their normal JSON encoding; the bucket order of
	// an FNV-hashed table is not asserted, only the contents.
	items := newTestTab(7)
	for _, s := range []string{"a", "b"} {
		items.Insert(TestData{S: s})
	}
	b, err = json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal(items): %v", err)
	}
	var round []TestData
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("json.Unmarshal of the table encoding: %v", err)
	}
	if fmt.Sprint(round) != "[{a 0} {b 0}]" && fmt.Sprint(round) != "[{b 0} {a 0}]" {
		t.Errorf("Unexpected encoding of struct elements: %s", b)
	}

	// An empty table encodes as [].
	if b, err := json.Marshal(newTestTab(7)); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty table, got (%s, %v)", b, err)
	}

	// A zero-value table is a tolerated read: [].
	var zero HashTab[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value table, got (%s, %v)", b, err)
	}

	// A direct call on a nil table encodes as []; json.Marshal on a nil
	// *HashTab never reaches the method — the json package writes null
	// for nil pointers itself.
	var nilTable *HashTab[int]
	if b, err := nilTable.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-table call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTable); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil table, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := newUpperTab()
	custom.Insert("x")
	custom.Insert("y")
	if b, err := json.Marshal(custom); err != nil || string(b) != `["X","Y"]` {
		t.Errorf(`Expected ["X","Y"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad := NewHashTabFunc(
		func(a, b chan int) int { return 0 },
		func(c chan int) uint64 { return 0 },
		5,
	)
	bad.Insert(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a table of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded elements land wherever the hash function puts them; the
	// table holds exactly the decoded set.
	ht := newTestTab(7)
	if err := json.Unmarshal([]byte(`[{"S":"c"},{"S":"a"},{"S":"b"}]`), ht); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(sortedKeys(ht)), "[a b c]"; got != want {
		t.Errorf("Expected %s after unmarshal, got %s", want, got)
	}
	checkInvariants(t, ht)

	// A round trip rebuilds a structurally sound table and keeps the
	// comparison/hash functions (Search works on the rebuilt table).
	items := newTestTab(7)
	for i, s := range []string{"a", "b", "c"} {
		items.Insert(TestData{S: s, N: i})
	}
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestTab(7)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, again)
	if got, want := fmt.Sprint(sortedKeys(again)), "[a b c]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}
	if it, found := again.Search(TestData{S: "b"}); !found || it.N != 1 {
		t.Errorf("Expected Search to find {b 1} after unmarshal, got (%v, %v)", it, found)
	}

	// Duplicates in the array collapse like duplicate inserts: the last
	// one wins.
	dup := newTestTab(7)
	if err := json.Unmarshal([]byte(`[{"S":"x","N":1},{"S":"x","N":2}]`), dup); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if it, found := dup.Search(TestData{S: "x"}); !found || it.N != 2 || dup.Len() != 1 {
		t.Errorf("Expected duplicate to collapse to {x 2}, got (%v, %v) len %d", it, found, dup.Len())
	}

	// Unmarshaling replaces the contents; it does not add to them.
	if err := json.Unmarshal([]byte(`[{"S":"only"}]`), ht); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := ht.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}

	// An empty array and null clear the table.
	full := newTestTab(7)
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

	// Element-level unmarshalers are honored.
	custom := newUpperTab()
	if err := json.Unmarshal([]byte(`["X","Y"]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var cs []string
	for v := range custom.Values() {
		cs = append(cs, string(v))
	}
	if fmt.Sprint(cs) != "[X Y]" {
		t.Errorf("Expected [X Y], got %v", cs)
	}

	// Decode errors are returned and leave the table untouched.
	keep := newTestTab(7)
	keep.Insert(TestData{S: "keep", N: 9})
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `[{"S":"a"},3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got, want := fmt.Sprint(sortedKeys(keep)), "[keep]"; got != want {
			t.Errorf("Table changed after the error on %s: %s", badData, got)
		}
		if it, found := keep.Search(TestData{S: "keep"}); !found || it.N != 9 {
			t.Errorf("Satellite data changed after the error on %s", badData)
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
	expectPanicMessage(t, "UnmarshalJSON with elements on zero value", "NewHashTab",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })

	var nilTable *HashTab[TestData]
	if err := nilTable.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil table to be tolerated, got %v", err)
	}
	expectPanicMessage(t, "UnmarshalJSON with elements on nil table", "nil table",
		func() { _ = nilTable.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })
}

// TestJSONStructField marshals and unmarshals a HashTab nested in a struct
// through the encoding/json package.  The table must be created with
// NewHashTab/NewHashTabFunc before unmarshaling: for a nil *HashTab field
// the json package allocates a zero-value table itself (no comparison/hash
// functions), so non-empty data panics with the insert-family message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string                `json:"title"`
		Tags  *HashTab[upperString] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: newUpperTab()}
	d.Tags.Insert("ds")
	d.Tags.Insert("go")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"pluto","tags":["DS","GO"]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created table field.
	var out Doc
	out.Tags = newUpperTab()
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var tags []string
	for v := range out.Tags.Values() {
		tags = append(tags, string(v))
	}
	if fmt.Sprint(tags) != "[DS GO]" {
		t.Errorf("Expected [DS GO], got %v", tags)
	}

	// A nil table field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created table and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: newUpperTab()}
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
	expectPanicMessage(t, "unmarshal into an uncreated table field", "NewHashTab",
		func() {
			var bad Doc
			_ = json.Unmarshal([]byte(`{"title":"x","tags":["a"]}`), &bad)
		})
}

// TestJSONRandomizedModel cross-checks a marshal/unmarshal round trip
// against a map model built at a fixed seed.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20260903)) // fixed seed: deterministic run

	ht := newTestTab(7)
	model := make(map[string]int)
	key := func(i int) string { return fmt.Sprintf("j%05d", i) }

	for op := range 1000 {
		k := key(rng.Intn(150))
		switch rng.Intn(3) {
		case 0, 1: // insert
			model[k] = op
			ht.Insert(TestData{S: k, N: op})
		case 2: // delete
			delete(model, k)
			ht.Delete(TestData{S: k})
		}
	}

	// Round trip: the rebuilt table must hold exactly the model.
	b, err := json.Marshal(ht)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestTab(7)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, again)
	if got, want := again.Len(), len(model); got != want {
		t.Fatalf("Round trip length = %d, model has %d", got, want)
	}
	for k, wantN := range model {
		it, found := again.Search(TestData{S: k})
		if !found || it.N != wantN {
			t.Errorf("Round trip: Search(%q) = (%v, %v), want N=%d", k, it, found, wantN)
		}
	}

	// Marshal of an emptied table is [], and unmarshaling it into a full
	// table clears it.
	ht.Truncate()
	if b, err := json.Marshal(ht); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a truncated table, got (%s, %v)", b, err)
	}
	if err := json.Unmarshal([]byte("[]"), again); err != nil || !again.IsEmpty() {
		t.Errorf("Expected [] to clear the round-tripped table, got (%v, empty=%v)", err, again.IsEmpty())
	}
}
