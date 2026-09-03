package hash_tab_dll

// Thorough tests for the dll-bucket hash table: nil and zero-value
// behavior, the panic contract with its messages, replacement semantics and
// handle liveness, chain construction and splicing, the fixed bucket count,
// and a randomized cross-check against a map model.  TestData and newTestTab
// are defined in hash_tab_test.go and reused here.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
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
// through their public All and Len.  Call it after structural changes.
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
// unlike hash_grow there is no reserved zero marker to remap, because an
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

// keysOf returns the set of keys currently in the table, for
// order-independent comparison (bucket order is hash dependent).
func keysOf(ht *HashTab[TestData]) map[string]TestData {
	got := make(map[string]TestData, ht.Len())
	for item := range ht.Values() {
		got[item.S] = item
	}
	return got
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output with a constant hash: every element chains in one
	// bucket, newest first (the bucket lists run head-newest to
	// tail-oldest).
	oneBucket := NewHashTabFunc(
		func(a, b string) bool { return a == b },
		func(s string) uint64 { return 7 },
		5,
	)
	for _, v := range []string{"a", "b", "c"} {
		oneBucket.Insert(v)
	}
	b, err := json.Marshal(oneBucket)
	if err != nil {
		t.Fatalf("json.Marshal(oneBucket): %v", err)
	}
	if string(b) != `["c","b","a"]` {
		t.Errorf(`Expected ["c","b","a"], got %s`, b)
	}

	// Struct elements use their normal JSON encoding.
	structs := NewHashTabFunc(
		func(a, b TestData) bool { return a.S == b.S },
		func(v TestData) uint64 { return 7 },
		5,
	)
	structs.Insert(TestData{S: "a", N: 1})
	structs.Insert(TestData{S: "b", N: 2})
	if b, err := json.Marshal(structs); err != nil || string(b) != `[{"S":"b","N":2},{"S":"a","N":1}]` {
		t.Errorf(`Expected [{"S":"b","N":2},{"S":"a","N":1}], got (%s, %v)`, b, err)
	}

	// A general table marshals as the set of its elements (order varies).
	tab := newTestTab(7)
	for _, s := range []string{"x", "y", "z"} {
		tab.Insert(TestData{S: s, N: len(s)})
	}
	var decoded []TestData
	b, err = json.Marshal(tab)
	if err != nil {
		t.Fatalf("json.Marshal(tab): %v", err)
	}
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("json.Unmarshal of the table encoding: %v", err)
	}
	if len(decoded) != 3 {
		t.Errorf("Expected 3 elements in the encoding, got %d", len(decoded))
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
	custom := NewHashTab[upperString](7)
	custom.Insert("x")
	custom.Insert("y")
	var customDecoded []string
	cb, err := json.Marshal(custom)
	if err != nil {
		t.Fatalf("json.Marshal(custom): %v", err)
	}
	if err := json.Unmarshal(cb, &customDecoded); err != nil {
		t.Fatalf("json.Unmarshal of the custom encoding: %v", err)
	}
	upper := map[string]bool{}
	for _, s := range customDecoded {
		upper[s] = true
	}
	if len(customDecoded) != 2 || !upper["X"] || !upper["Y"] {
		t.Errorf(`Expected the custom encoding to hold "X" and "Y", got %s`, cb)
	}

	// Encoding errors pass through unchanged.
	bad := NewHashTab[chan int](7)
	bad.Insert(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a table of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded elements replace the contents; Search works on the rebuilt
	// table (the equality/hash functions are kept).
	tab := newTestTab(7)
	tab.Insert(TestData{S: "stale", N: -1})
	if err := json.Unmarshal([]byte(`[{"S":"a","N":1},{"S":"b","N":2},{"S":"c","N":3}]`), tab); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := tab.Len(), 3; got != want {
		t.Errorf("Expected replacement with 3 elements, got length %d", got)
	}
	if _, found := tab.Search(TestData{S: "stale"}); found {
		t.Errorf("Unmarshal should replace the contents, stale is still present")
	}
	for _, want := range []TestData{{S: "a", N: 1}, {S: "b", N: 2}, {S: "c", N: 3}} {
		el, found := tab.Search(TestData{S: want.S})
		if !found || el.GetData() != want {
			t.Errorf("Expected Search to find %v after unmarshal, got found=%v", want, found)
		}
	}
	checkInvariants(t, tab)

	// A round trip preserves the set.
	full := newTestTab(7)
	for i := range 25 {
		full.Insert(TestData{S: fmt.Sprintf("rt%03d", i), N: i})
	}
	b, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestTab(7)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(keysOf(again)), fmt.Sprint(keysOf(full)); got != want {
		t.Errorf("Round trip mismatch: got %s, want %s", got, want)
	}
	checkInvariants(t, again)

	// Elements equal under the equality function collapse; the last one in
	// the array wins (the Insert replacement rule).
	dup := newTestTab(7)
	if err := json.Unmarshal([]byte(`[{"S":"k","N":1},{"S":"k","N":2}]`), dup); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if el, found := dup.Search(TestData{S: "k"}); !found || dup.Len() != 1 || el.GetData().N != 2 {
		t.Errorf("Expected the duplicate to collapse to N=2, got len=%d found=%v", dup.Len(), found)
	}

	// An empty array and null clear the table.
	clearTab := newTestTab(7)
	clearTab.Insert(TestData{S: "z"})
	if err := json.Unmarshal([]byte("[]"), clearTab); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !clearTab.IsEmpty() {
		t.Errorf("Expected [] to clear the table.")
	}
	clearTab.Insert(TestData{S: "z"})
	if err := json.Unmarshal([]byte("null"), clearTab); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !clearTab.IsEmpty() {
		t.Errorf("Expected null to clear the table.")
	}
	checkInvariants(t, clearTab)

	// Element-level unmarshalers are honored.
	custom := NewHashTab[upperString](7)
	if err := json.Unmarshal([]byte(`["X","Y"]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if _, found := custom.Search("X"); !found {
		t.Errorf(`Expected to find "X" after unmarshal`)
	}
	if _, found := custom.Search("Y"); !found {
		t.Errorf(`Expected to find "Y" after unmarshal`)
	}

	// Decode errors are returned and leave the table untouched.
	keep := newTestTab(7)
	keep.Insert(TestData{S: "keep"})
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `[{"S":"a"},3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if el, found := keep.Search(TestData{S: "keep"}); !found || keep.Len() != 1 || el.GetData().S != "keep" {
			t.Errorf("Table changed after the error on %s", badData)
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
	expectPanicMessage(t, "UnmarshalJSON on zero value", "UnmarshalJSON",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })
	expectPanicMessage(t, "UnmarshalJSON on zero value", "NewHashTab",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })

	var nilTable *HashTab[TestData]
	if err := nilTable.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil table to be tolerated, got %v", err)
	}
	expectPanicMessage(t, "UnmarshalJSON on nil table", "UnmarshalJSON",
		func() { _ = nilTable.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })
	expectPanicMessage(t, "UnmarshalJSON on nil table", "nil table",
		func() { _ = nilTable.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })
}

// TestJSONStructField marshals and unmarshals a HashTab nested in a struct
// through the encoding/json package.  The table must be created with
// NewHashTab/NewHashTabFunc before unmarshaling: for a nil *HashTab field
// the json package allocates a zero-value table itself (no equality/hash
// functions), so non-empty data panics with the insert-family message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string             `json:"title"`
		Tags  *HashTab[TestData] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: newTestTab(7)}
	d.Tags.Insert(TestData{S: "ds"})
	d.Tags.Insert(TestData{S: "go"})

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var raw struct {
		Title string     `json:"title"`
		Tags  []TestData `json:"tags"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("json.Unmarshal of the document: %v", err)
	}
	if raw.Title != "pluto" || len(raw.Tags) != 2 {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created table field.
	var out Doc
	out.Tags = newTestTab(7)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if _, found := out.Tags.Search(TestData{S: "ds"}); !found {
		t.Errorf(`Expected to find "ds" after unmarshal`)
	}
	if _, found := out.Tags.Search(TestData{S: "go"}); !found {
		t.Errorf(`Expected to find "go" after unmarshal`)
	}

	// A nil table field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created table and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: newTestTab(7)}
	clearDoc.Tags.Insert(TestData{S: "gone"})
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the table.")
	}

	// Non-empty data into a nil *HashTab field: the json package allocates
	// a zero-value table, and the insert contract panics through
	// json.Unmarshal (it does not recover panics).
	expectPanicMessage(t, "Unmarshal into an uncreated table field", "NewHashTab",
		func() {
			var bad Doc
			_ = json.Unmarshal([]byte(`{"title":"x","tags":[{"S":"a"}]}`), &bad)
		})
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling against
// a map reference model at fixed seed: after every operation the table is
// marshaled, decoded into a fresh table, and the fresh table must hold
// exactly the model's key set with the latest satellite values.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(12345)) // fixed seed: deterministic run
	ht := newTestTab(7)
	model := make(map[string]int) // key -> satellite value of the latest insert

	key := func(i int) string { return fmt.Sprintf("j%05d", i) }

	for op := range 2000 {
		k := key(rng.Intn(300))
		switch rng.Intn(4) {
		case 0, 1, 2: // insert (weighted heavier)
			model[k] = op
			ht.Insert(TestData{S: k, N: op})
		case 3: // delete
			delete(model, k)
			ht.Delete(TestData{S: k})
		}

		data, err := json.Marshal(ht)
		if err != nil {
			t.Fatalf("op %d: json.Marshal: %v", op, err)
		}
		fresh := newTestTab(7)
		if err := json.Unmarshal(data, fresh); err != nil {
			t.Fatalf("op %d: json.Unmarshal: %v", op, err)
		}
		if fresh.Len() != len(model) {
			t.Fatalf("op %d: round-tripped table has %d elements, model has %d", op, fresh.Len(), len(model))
		}
		for mk, mn := range model {
			el, found := fresh.Search(TestData{S: mk})
			if !found {
				t.Fatalf("op %d: round-tripped table missing %q", op, mk)
			}
			if el.GetData().N != mn {
				t.Fatalf("op %d: round-tripped %q has satellite %d, want %d", op, mk, el.GetData().N, mn)
			}
		}
	}
	checkInvariants(t, ht)
}
