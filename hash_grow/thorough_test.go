package hash_grow

// Thorough tests for the hash_grow table: nil and zero-value behavior, the
// panic contract with its messages, replacement semantics, wrap-around probe
// chains, growth, the full-table guard, JSON marshaling/unmarshaling, and a
// randomized cross-check against a map model.  TestData and newTestTab are
// defined in hash_tab_test.go and reused here.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
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

// checkInvariants verifies the structural invariants of the table: a bucket
// is occupied exactly when its raw hash is non-zero, the occupied count
// matches Len, Search finds every stored element, and Walk and the
// iterators agree with the buckets.  Call it after structural changes.
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
		func() { NewHashTabFunc(nil, validHash, 7, 0) })
	expectPanicMessage(t, "NewHashTabFunc(nil hash)", "nil hash function",
		func() { NewHashTabFunc(validEq, nil, 7, 0) })
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
	home := int(ht.hashOf(TestData{S: "dup"}) % uint64(ht.size))
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
	for i := 0; ; i++ {
		s := fmt.Sprintf("w%06d", i)
		if used[s] {
			continue
		}
		if int(ht.hashOf(TestData{S: s})%uint64(ht.size)) == home {
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
	if ht.size != 5 {
		t.Fatalf("Expected initial size 5, got %d", ht.size)
	}
	keys := []string{"g0", "g1", "g2", "g3", "g4", "g5"}
	for i, k := range keys {
		ht.Insert(TestData{S: k})
		switch {
		case i < 2 && ht.size != 5: // 1/5, 2/5 stay below 0.5
			t.Fatalf("After %d inserts size should be 5, got %d", i+1, ht.size)
		case i == 2 && ht.size != 10: // 3/5 = 0.6 > 0.5 -> grow
			t.Fatalf("After 3 inserts size should be 10, got %d", ht.size)
		case i >= 3 && i < 5 && ht.size != 10:
			t.Fatalf("After %d inserts size should be 10, got %d", i+1, ht.size)
		case i == 5 && ht.size != 20: // 6/10 = 0.6 > 0.5 -> grow
			t.Fatalf("After 6 inserts size should be 20, got %d", ht.size)
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
		if ht.size != 5 || ht.Len() != 5 {
			t.Fatalf("saturation %v: expected a full size-5 table, got size %d length %d", saturation, ht.size, ht.Len())
		}
		// A missing key on a completely full table must terminate not-found.
		if _, found := ht.Search(TestData{S: "no-such-key"}); found {
			t.Errorf("saturation %v: missing key must not be found", saturation)
		}
		// The 6th insert forces growth instead of being dropped.
		if !ht.Insert(TestData{S: "f5"}) {
			t.Errorf("saturation %v: the 6th insert should be an add", saturation)
		}
		if ht.size != 10 {
			t.Errorf("saturation %v: expected forced growth to size 10, got %d", saturation, ht.size)
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

// newUpperTab builds an upperString table with deterministic hashing.
func newUpperTab() *HashTab[upperString] {
	return NewHashTabFunc(
		func(a, b upperString) bool { return a == b },
		func(v upperString) uint64 {
			h := fnv.New64a()
			_, _ = h.Write([]byte(v))
			return h.Sum64()
		},
		7, 0,
	)
}

// sortedValues returns the table's elements as a sorted slice of strings,
// for order-independent comparison (bucket order is never asserted).
func sortedValues[T any](ht *HashTab[T]) []string {
	got := []string{}
	for v := range ht.Values() {
		got = append(got, fmt.Sprint(v))
	}
	sort.Strings(got)
	return got
}

func TestMarshalJSON(t *testing.T) {
	// The elements encode as a JSON array; membership is asserted, not
	// bucket order.
	ht := newTestTab(7, 0)
	ht.Insert(TestData{S: "a", N: 1})
	ht.Insert(TestData{S: "b", N: 2})
	b, err := json.Marshal(ht)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var got []TestData
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("json.Unmarshal of the array: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Expected 2 elements in the array, got %d (%s)", len(got), b)
	}
	seen := map[string]int{}
	for _, v := range got {
		seen[v.S] = v.N
	}
	if seen["a"] != 1 || seen["b"] != 2 {
		t.Errorf("Unexpected array contents: %s", b)
	}

	// A single element has an exact encoding (struct fields S and N).
	one := newTestTab(7, 0)
	one.Insert(TestData{S: "a", N: 1})
	if b, err := json.Marshal(one); err != nil || string(b) != `[{"S":"a","N":1}]` {
		t.Errorf(`Expected [{"S":"a","N":1}], got (%s, %v)`, b, err)
	}

	// An empty table encodes as [].
	if b, err := json.Marshal(newTestTab(7, 0)); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty table, got (%s, %v)", b, err)
	}

	// A zero-value table is a tolerated read: [].
	var zero HashTab[TestData]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value table, got (%s, %v)", b, err)
	}

	// A direct call on a nil table encodes as []; json.Marshal on a nil
	// *HashTab never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilTable *HashTab[TestData]
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
	b, err = json.Marshal(custom)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var us []string
	if err := json.Unmarshal(b, &us); err != nil {
		t.Fatalf("json.Unmarshal of the array: %v", err)
	}
	sort.Strings(us)
	if fmt.Sprint(us) != "[X Y]" {
		t.Errorf(`Expected ["X","Y"] (in some order), got %s`, b)
	}

	// Encoding errors pass through unchanged.
	bad := NewHashTab[chan int](7, 0)
	bad.Insert(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a table of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded elements are inserted; Search works on the rebuilt table.
	ht := newTestTab(7, 0)
	if err := json.Unmarshal([]byte(`[{"S":"a","N":1},{"S":"b","N":2}]`), ht); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if ht.Len() != 2 {
		t.Fatalf("Expected length 2, got %d", ht.Len())
	}
	if got, found := ht.Search(TestData{S: "a"}); !found || got.N != 1 {
		t.Errorf("Expected to find a=1 after unmarshal, got (%v, %v)", got, found)
	}
	if got, found := ht.Search(TestData{S: "b"}); !found || got.N != 2 {
		t.Errorf("Expected to find b=2 after unmarshal, got (%v, %v)", got, found)
	}
	checkInvariants(t, ht)

	// A round trip rebuilds a table with the same membership (the
	// equality/hash functions are kept, so the table stays usable).
	full := newTestTab(7, 0)
	for i := range 50 {
		full.Insert(TestData{S: fmt.Sprintf("k%03d", i), N: i})
	}
	b, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestTab(7, 0)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if again.Len() != 50 {
		t.Fatalf("Expected length 50 after round trip, got %d", again.Len())
	}
	for i := range 50 {
		k := fmt.Sprintf("k%03d", i)
		if got, found := again.Search(TestData{S: k}); !found || got.N != i {
			t.Errorf("Expected to find %s=%d after round trip, got (%v, %v)", k, i, got, found)
		}
	}
	checkInvariants(t, again)

	// Unmarshaling replaces the contents; it does not add to them.
	if err := json.Unmarshal([]byte(`[{"S":"only","N":7}]`), full); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := full.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}
	if _, found := full.Search(TestData{S: "k000"}); found {
		t.Errorf("Expected the old contents to be gone after replacement")
	}

	// Duplicate elements in the array collapse like repeated Insert calls:
	// the last one wins.
	dup := newTestTab(7, 0)
	if err := json.Unmarshal([]byte(`[{"S":"d","N":1},{"S":"d","N":2}]`), dup); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, found := dup.Search(TestData{S: "d"}); !found || got.N != 2 || dup.Len() != 1 {
		t.Errorf("Expected the last duplicate to win, got (%v, %v) len %d", got, found, dup.Len())
	}

	// An empty array and null clear the table.
	clearTab := newTestTab(7, 0)
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
	custom := newUpperTab()
	if err := json.Unmarshal([]byte(`["X","Y"]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(sortedValues(custom)), "[X Y]"; got != want {
		t.Errorf("Expected %s, got %s", want, got)
	}

	// Decode errors are returned and leave the table untouched.
	keep := newTestTab(7, 0)
	keep.Insert(TestData{S: "keep", N: 9})
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `[{"S":"a"},3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got, want := fmt.Sprint(sortedValues(keep)), "[{keep 9}]"; got != want {
			t.Errorf("Table changed after the error on %s: %s", badData, got)
		}
		if keep.Len() != 1 {
			t.Errorf("Length changed after the error on %s: %d", badData, keep.Len())
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
	expectPanicMessage(t, "UnmarshalJSON with elements on a zero-value table", "UnmarshalJSON",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })
	expectPanicMessage(t, "UnmarshalJSON zero-value message names the fix", "NewHashTab",
		func() { _ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`)) })

	var nilTable *HashTab[TestData]
	if err := nilTable.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil table to be tolerated, got %v", err)
	}
	if err := nilTable.UnmarshalJSON([]byte("null")); err != nil {
		t.Errorf("Expected null on a nil table to be tolerated, got %v", err)
	}
	expectPanicMessage(t, "UnmarshalJSON with elements on a nil table", "UnmarshalJSON called on a nil table",
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

	d := Doc{Title: "pluto", Tags: NewHashTab[string](8, 0)}
	d.Tags.Insert("ds")
	d.Tags.Insert("go")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var wire struct {
		Title string   `json:"title"`
		Tags  []string `json:"tags"`
	}
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatalf("json.Unmarshal of the document: %v", err)
	}
	sort.Strings(wire.Tags)
	if wire.Title != "pluto" || fmt.Sprint(wire.Tags) != "[ds go]" {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created table field.
	var out Doc
	out.Tags = NewHashTab[string](8, 0)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := fmt.Sprint(sortedValues(out.Tags)), "[ds go]"; got != want {
		t.Errorf("Expected %s, got %s", want, got)
	}

	// A nil table field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created table and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: NewHashTab[string](8, 0)}
	clearDoc.Tags.Insert("gone")
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the table.")
	}

	// Non-empty data into a nil *HashTab field: the json package allocates a
	// zero-value table, and the insert contract panics through
	// json.Unmarshal (it does not recover panics).
	expectPanicMessage(t, "Unmarshal into an uncreated table field", "NewHashTab",
		func() {
			var bad Doc
			_ = json.Unmarshal([]byte(`{"title":"x","tags":["a"]}`), &bad)
		})
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling against
// a map reference model at fixed seed: after a random mix of inserts and
// deletes the marshaled table must carry exactly the model's membership,
// and unmarshaling it into a fresh table must reproduce that membership.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20260902)) // fixed seed: deterministic run
	ht := newTestTab(7, 0)
	model := make(map[string]int) // key -> satellite value of the latest insert

	key := func(i int) string { return fmt.Sprintf("j%05d", i) }

	for op := range 2000 {
		k := key(rng.Intn(200))
		if rng.Intn(4) == 0 { // delete
			delete(model, k)
			ht.Delete(TestData{S: k})
		} else { // insert
			model[k] = op
			ht.Insert(TestData{S: k, N: op})
		}

		if op%200 != 0 {
			continue
		}

		// Marshal and decode the array: membership must match the model.
		b, err := json.Marshal(ht)
		if err != nil {
			t.Fatalf("op %d: json.Marshal: %v", op, err)
		}
		var arr []TestData
		if err := json.Unmarshal(b, &arr); err != nil {
			t.Fatalf("op %d: json.Unmarshal of the array: %v", op, err)
		}
		if len(arr) != len(model) {
			t.Fatalf("op %d: marshaled %d elements, model has %d", op, len(arr), len(model))
		}
		for _, v := range arr {
			if wantN, ok := model[v.S]; !ok || wantN != v.N {
				t.Fatalf("op %d: marshaled element %v not in the model", op, v)
			}
		}

		// Unmarshal into a fresh table: membership must match again.
		fresh := newTestTab(7, 0)
		if err := json.Unmarshal(b, fresh); err != nil {
			t.Fatalf("op %d: json.Unmarshal into a fresh table: %v", op, err)
		}
		if fresh.Len() != len(model) {
			t.Fatalf("op %d: rebuilt table has length %d, model has %d", op, fresh.Len(), len(model))
		}
		for k, wantN := range model {
			if got, found := fresh.Search(TestData{S: k}); !found || got.N != wantN {
				t.Fatalf("op %d: rebuilt table: Search(%q) = (%v, %v), want N=%d", op, k, got, found, wantN)
			}
		}
	}
	checkInvariants(t, ht)
}
