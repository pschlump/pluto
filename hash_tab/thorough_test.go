package hash_tab

// Thorough tests for the chained hash table: nil and zero-value behavior,
// the panic contract with its messages, replacement semantics, chain
// construction and unlinking, the fixed bucket count, and a randomized
// cross-check against a map model.  TestData and newTestTab are defined in
// hash_tab_test.go and reused here.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"cmp"
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
// node sits in the bucket its stored hash reduces to, the node count
// matches Len, Search finds every stored element, and Walk and the All
// iterator agree exactly with the chains.  Call it after structural changes.
func checkInvariants[T comparable](t *testing.T, ht *HashTab[T]) {
	t.Helper()
	nodes := 0
	var want []T
	for i := range ht.buckets {
		for node := ht.buckets[i]; node != nil; node = node.next {
			if int(node.hash%uint64(ht.size)) != i {
				t.Fatalf("element %v with hash %d is chained in bucket %d, want bucket %d", node.data, node.hash, i, node.hash%uint64(ht.size))
			}
			if _, found := ht.Search(node.data); !found {
				t.Fatalf("element in bucket %d (%v) not found by Search", i, node.data)
			}
			nodes++
			want = append(want, node.data)
		}
	}
	if nodes != ht.Len() {
		t.Fatalf("chained nodes = %d, Len() = %d", nodes, ht.Len())
	}
	var got []T
	for pos, item := range ht.All() {
		if pos < 0 || pos >= ht.size {
			t.Fatalf("All reported out-of-range position %d", pos)
		}
		got = append(got, item)
	}
	if len(got) != len(want) {
		t.Fatalf("All visited %d elements, chains hold %d", len(got), len(want))
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("All position %d: chains hold %v, All reported %v", i, want[i], got[i])
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
	validEq := func(a, b TestData) bool { return a.S == b.S }
	expectPanicMessage(t, "NewHashTabFunc(nil eq)", "nil equality function",
		func() { NewHashTabFunc(nil, validHash, 7) })
	expectPanicMessage(t, "NewHashTabFunc(nil hash)", "nil hash function",
		func() { NewHashTabFunc(validEq, nil, 7) })
}

// TestInsertReplacesExisting verifies that re-inserting an equal key replaces
// the stored value (the satellite data changes) and does not change the
// length, including when the key is not the head of its chain.
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

	// Force collisions so the replacement happens deep in a chain.
	home := ht.bucketOf(ht.hashOf(TestData{S: "dup"}))
	used := map[string]bool{"dup": true}
	c1 := findKeyWithHome(t, ht, home, used)
	c2 := findKeyWithHome(t, ht, home, used)
	ht.Insert(TestData{S: c1, N: 10})
	ht.Insert(TestData{S: c2, N: 20}) // chains ahead of c1
	ht.Insert(TestData{S: c1, N: 11}) // replace inside the chain
	if got, found := ht.Search(TestData{S: c1}); !found || got.N != 11 {
		t.Errorf("Replacement in chain should return new value, got %v found=%v", got, found)
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
// the most recently inserted element to the oldest, and then unlinks the
// middle, the head and the tail of the chain, splicing each node out in a
// single pass.
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
	if ht.buckets[2] == nil || ht.buckets[2].next == nil {
		t.Fatalf("Expected a 3-node chain in bucket 2")
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
	if !ht.IsEmpty() || ht.buckets[2] != nil {
		t.Errorf("Expected empty table and nil chain after draining, got length %d", ht.Len())
	}
	// Deleting from the drained chain returns false.
	if ht.Delete("anything") {
		t.Errorf("Delete on a drained chain should return false")
	}
	checkInvariants(t, ht)
}

// TestHashZeroNotSpecial verifies that a hash of 0 is just another hash —
// unlike hash_grow there is no reserved zero marker to remap, because an
// empty bucket is a nil chain.
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
	if ht.buckets[0] == nil {
		t.Fatalf("Expected the zero hash to chain in bucket 0")
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
	if ht.buckets[0].hash != 0 {
		t.Errorf("hash 0 should be stored as 0, got %d", ht.buckets[0].hash)
	}
	checkInvariants(t, ht)
}

// TestSizeNeverGrows verifies that the bucket count is fixed for the life of
// the table — the chained table never re-hashes, no matter the load.
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

// sortedJSONElements decodes b as a JSON array of T and returns the elements
// sorted — the table marshals in bucket order, which varies from process to
// process, so multi-element assertions compare sets, not positions.
func sortedJSONElements[T cmp.Ordered](t *testing.T, b []byte) []T {
	t.Helper()
	var items []T
	if err := json.Unmarshal(b, &items); err != nil {
		t.Fatalf("json.Unmarshal of the marshaled table: %v", err)
	}
	sort.Slice(items, func(i, j int) bool { return items[i] < items[j] })
	return items
}

// oneChainTab returns a table whose every element chains in bucket 2 (a
// constant hash into 5 buckets), so the marshaled order is deterministic:
// newest element first.
func oneChainTab(items ...string) *HashTab[string] {
	ht := NewHashTabFunc(
		func(a, b string) bool { return a == b },
		func(s string) uint64 { return 7 },
		5,
	)
	for _, s := range items {
		ht.Insert(s)
	}
	return ht
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output with a forced single chain: chains run from the
	// most recently inserted element to the oldest.
	ht := oneChainTab("a", "b", "c")
	b, err := json.Marshal(ht)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `["c","b","a"]` {
		t.Errorf(`Expected ["c","b","a"], got %s`, b)
	}

	// Struct elements use their normal JSON encoding (order-independent).
	items := newTestTab(7)
	for _, s := range []string{"a", "b"} {
		items.Insert(TestData{S: s})
	}
	if b, err := json.Marshal(items); err != nil {
		t.Fatalf("json.Marshal: %v", err)
	} else {
		var decoded []TestData
		if err := json.Unmarshal(b, &decoded); err != nil {
			t.Fatalf("json.Unmarshal of the marshaled table: %v", err)
		}
		sort.Slice(decoded, func(i, j int) bool { return decoded[i].S < decoded[j].S })
		if fmt.Sprint(decoded) != "[{a 0} {b 0}]" {
			t.Errorf("Expected [{a 0} {b 0}], got %v", decoded)
		}
	}

	// An empty table encodes as [].
	if b, err := json.Marshal(NewHashTab[int](7)); err != nil || string(b) != "[]" {
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
	var nilTab *HashTab[int]
	if b, err := nilTab.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-table call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTab); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil table, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := NewHashTab[upperString](7)
	custom.Insert("x")
	custom.Insert("y")
	got := sortedJSONElements[upperString](t, mustMarshal(t, custom))
	if fmt.Sprint(got) != "[X Y]" {
		t.Errorf("Expected [X Y], got %v", got)
	}

	// Encoding errors pass through unchanged.
	bad := NewHashTab[chan int](7)
	bad.Insert(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a table of channels.")
	}
}

func mustMarshal[T any](t *testing.T, ht *HashTab[T]) []byte {
	t.Helper()
	b, err := json.Marshal(ht)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

func TestUnmarshalJSON(t *testing.T) {
	// Every decoded element is inserted; the equality/hash functions are
	// kept, so Search works on the rebuilt table.
	ht := NewHashTab[int](11)
	if err := json.Unmarshal([]byte("[3,1,2]"), ht); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if ht.Len() != 3 {
		t.Fatalf("Expected length 3 after unmarshal, got %d", ht.Len())
	}
	for _, v := range []int{3, 1, 2} {
		if _, found := ht.Search(v); !found {
			t.Errorf("Expected to find %d after unmarshal", v)
		}
	}
	checkInvariants(t, ht)

	// A round trip preserves the set exactly, including satellite data.
	items := newTestTab(7)
	for _, s := range []string{"a", "b", "c"} {
		items.Insert(TestData{S: s, N: len(s)})
	}
	b := mustMarshal(t, items)
	again := newTestTab(7)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkInvariants(t, again)
	if again.Len() != 3 {
		t.Fatalf("Expected length 3 after round trip, got %d", again.Len())
	}
	for item := range items.Values() {
		got, found := again.Search(item)
		if !found || got != item {
			t.Errorf("Expected %v after round trip, got (%v, %v)", item, got, found)
		}
	}

	// Duplicate elements in the array collapse: the later insert replaces,
	// exactly as Insert does.
	dup := newTestTab(7)
	if err := json.Unmarshal([]byte(`[{"S":"k","N":1},{"S":"k","N":2}]`), dup); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if dup.Len() != 1 {
		t.Fatalf("Expected duplicates to collapse to length 1, got %d", dup.Len())
	}
	if got, found := dup.Search(TestData{S: "k"}); !found || got.N != 2 {
		t.Errorf("Expected the later duplicate to win, got (%v, %v)", got, found)
	}

	// Unmarshaling replaces the contents; it does not add to them.
	if err := json.Unmarshal([]byte("[7]"), ht); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := ht.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}
	if _, found := ht.Search(7); !found {
		t.Errorf("Expected to find 7 after replacement")
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
	custom := NewHashTab[upperString](7)
	if err := json.Unmarshal([]byte(`["X","Y"]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	for _, v := range []upperString{"X", "Y"} {
		if _, found := custom.Search(v); !found {
			t.Errorf("Expected to find %q after unmarshal", v)
		}
	}

	// Decode errors are returned and leave the table untouched.
	keep := newTestTab(7)
	keep.Insert(TestData{S: "keep"})
	for _, badData := range []string{"[1,", `[{"S":"x"}`, `{"S":"a"}`, "7", `[{"S":"a"},3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got, want := keep.Len(), 1; got != want {
			t.Errorf("Table length changed after the error on %s: got %d, want %d", badData, got, want)
		}
		if got, found := keep.Search(TestData{S: "keep"}); !found || got.S != "keep" {
			t.Errorf("Table contents changed after the error on %s: (%v, %v)", badData, got, found)
		}
	}
	checkInvariants(t, keep)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil or zero-value table panics with a
// message naming the method and the fix, while [] and null — which
// store nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero HashTab[TestData]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value table to be tolerated, got %v", data, err)
		}
	}
	expectPanicMessage(t, "UnmarshalJSON with elements on a zero-value table", "NewHashTab",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })
	expectPanicMessage(t, "UnmarshalJSON with elements on a zero-value table", "UnmarshalJSON",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })

	var nilTab *HashTab[TestData]
	if err := nilTab.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil table to be tolerated, got %v", err)
	}
	expectPanicMessage(t, "UnmarshalJSON with elements on a nil table", "nil table",
		func() { _ = nilTab.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })
	expectPanicMessage(t, "UnmarshalJSON with elements on a nil table", "UnmarshalJSON",
		func() { _ = nilTab.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })
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

	d := Doc{Title: "pluto", Tags: NewHashTab[string](7)}
	d.Tags.Insert("ds") // a single element keeps the document order deterministic

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"pluto","tags":["ds"]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created table field.
	var out Doc
	out.Tags = NewHashTab[string](7)
	if err := json.Unmarshal([]byte(`{"title":"pluto","tags":["ds","go"]}`), &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if out.Tags.Len() != 2 {
		t.Fatalf("Expected 2 tags, got %d", out.Tags.Len())
	}
	for _, tag := range []string{"ds", "go"} {
		if _, found := out.Tags.Search(tag); !found {
			t.Errorf("Expected to find tag %q", tag)
		}
	}

	// A nil table field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created table and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: NewHashTab[string](7)}
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

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling against
// a map reference model at fixed seed.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20260902)) // fixed seed: deterministic run
	const ops = 500

	ht := NewHashTab[int](13)
	model := make(map[int]bool)

	for step := range ops {
		v := rng.Intn(150)
		if rng.Intn(4) == 0 {
			ht.Delete(v)
			delete(model, v)
		} else {
			ht.Insert(v)
			model[v] = true
		}

		// Marshal must hold exactly the model's elements (in bucket order,
		// so compare as sets).
		got := sortedJSONElements[int](t, mustMarshal(t, ht))
		want := make([]int, 0, len(model))
		for k := range model {
			want = append(want, k)
		}
		sort.Ints(want)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("step %d: marshaled %v, model %v", step, got, want)
		}

		// Unmarshaling the document into a fresh table must reproduce the
		// model's membership.
		fresh := NewHashTab[int](13)
		if err := json.Unmarshal(mustMarshal(t, ht), fresh); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if fresh.Len() != len(model) {
			t.Fatalf("step %d: fresh length %d, model %d", step, fresh.Len(), len(model))
		}
		for k := range model {
			if _, found := fresh.Search(k); !found {
				t.Fatalf("step %d: fresh table missing %d", step, k)
			}
		}
	}
	checkInvariants(t, ht)
}
