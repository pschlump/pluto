package cuckoo

// Thorough tests for the cuckoo table: nil and zero-value behavior, the
// panic contract with its messages, replacement semantics, the position
// derivation, growth and shrink thresholds, deferred growth, the
// pathological-hash guard, and a randomized cross-check against a map model.
// TestData and newTestTab are defined in hash_tab_test.go and reused here.

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
// occupied slot's stored hash re-hashes to itself and the slot is one of the
// four candidates derived from that hash, the occupied count matches Len,
// Search finds every stored element, and Walk and the iterators agree with
// the slots.  Call it after structural changes.
func checkInvariants(t *testing.T, ht *HashTab[TestData]) {
	t.Helper()
	var zero TestData
	occupied := 0
	for i := range ht.slots {
		s := ht.slots[i]
		if !s.used {
			if s.data != zero || s.hash != 0 {
				t.Fatalf("slot %d is empty but holds %v h=%d", i, s.data, s.hash)
			}
			continue
		}
		occupied++
		if s.hash != ht.hash(s.data) {
			t.Fatalf("slot %d stored hash %d, but re-hashing %v gives %d", i, s.hash, s.data, ht.hash(s.data))
		}
		ok := false
		for j := 0; j < numPositions; j++ {
			if i == posOf(s.hash, j, ht.mask) {
				ok = true
				break
			}
		}
		if !ok {
			t.Fatalf("slot %d is not one of the four candidates of its hash %d (mask %d)", i, s.hash, ht.mask)
		}
		if _, found := ht.Search(s.data); !found {
			t.Fatalf("element in slot %d (%v) not found by Search", i, s.data)
		}
	}
	if occupied != ht.Len() {
		t.Fatalf("occupied slots = %d, Len() = %d", occupied, ht.Len())
	}
	seen := make(map[string]int)
	for pos, item := range ht.All() {
		if pos < 0 || pos >= ht.size {
			t.Fatalf("All reported out-of-range position %d", pos)
		}
		if !ht.slots[pos].used || ht.slots[pos].data != item {
			t.Fatalf("All reported position %d for %v but the slot disagrees", pos, item)
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
		if ht.Capacity() != 0 {
			t.Errorf("%s: capacity should be 0, got %d", name, ht.Capacity())
		}
		if ht.Saturation() != 0 {
			t.Errorf("%s: saturation should be 0, got %v", name, ht.Saturation())
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
	expectPanicMessage(t, "Insert on nil table", "cuckoo: Insert called on a nil table",
		func() { nilTable.Insert(TestData{S: "x"}) })
	expectPanicMessage(t, "Insert on zero-value table", "no equality/hash functions",
		func() { (&HashTab[TestData]{}).Insert(TestData{S: "x"}) })
	expectPanicMessage(t, "NewHashTabFunc nil eq", "cuckoo: NewHashTabFunc called with a nil equality function",
		func() { NewHashTabFunc(nil, func(TestData) uint64 { return 1 }, 16, 0, 0) })
	expectPanicMessage(t, "NewHashTabFunc nil hash", "cuckoo: NewHashTabFunc called with a nil hash function",
		func() { NewHashTabFunc(func(a, b TestData) bool { return a.S == b.S }, nil, 16, 0, 0) })
	expectPanicMessage(t, "n < 5", "initial size must be at least 5",
		func() { NewHashTab[string](4, 0, 0) })
	expectPanic(t, "n < 5 via NewHashTabFunc", func() {
		NewHashTabFunc(func(a, b int) bool { return a == b }, func(int) uint64 { return 1 }, 1, 0, 0)
	})
}

// TestInsertReplacesExisting verifies a duplicate insert replaces the stored
// satellite data in place: count unchanged, the stored hash unchanged.
func TestInsertReplacesExisting(t *testing.T) {
	ht := newTestTab(16, 0, 0)
	ht.Insert(TestData{S: "dup", N: 1})
	oldLen, oldCap := ht.Len(), ht.Capacity()
	var oldHash uint64
	var oldSlot int
	for i := range ht.slots {
		if ht.slots[i].used {
			oldSlot, oldHash = i, ht.slots[i].hash
		}
	}
	if ht.Insert(TestData{S: "dup", N: 2}) {
		t.Errorf("duplicate insert should report replaced")
	}
	if ht.Len() != oldLen || ht.Capacity() != oldCap {
		t.Errorf("replacement changed the table: Len %d->%d, Capacity %d->%d", oldLen, ht.Len(), oldCap, ht.Capacity())
	}
	if !ht.slots[oldSlot].used || ht.slots[oldSlot].hash != oldHash {
		t.Errorf("replacement should keep the element in place with its hash")
	}
	if v, _ := ht.Search(TestData{S: "dup"}); v.N != 2 {
		t.Errorf("stored satellite = %d, want 2", v.N)
	}
	checkInvariants(t, ht)
}

// TestGrowthDoubles fills the table past several thresholds and verifies the
// capacity stays a power of two, the saturation stays under the grow
// threshold after every insert, and no element is lost.
func TestGrowthDoubles(t *testing.T) {
	ht := newTestTab(8, 0.85, 0.10) // starts at the 256 minimum
	for i := range 600 {
		ht.Insert(TestData{S: fmt.Sprintf("g%04d", i), N: i})
		if c := ht.Capacity(); c&(c-1) != 0 {
			t.Fatalf("capacity %d is not a power of two after insert %d", c, i)
		}
		if s := ht.Saturation(); s > 0.85 {
			t.Fatalf("saturation %.4f above 0.85 after insert %d", s, i)
		}
	}
	if ht.Len() != 600 {
		t.Fatalf("Len = %d, want 600", ht.Len())
	}
	for i := range 600 {
		k := fmt.Sprintf("g%04d", i)
		if v, found := ht.Search(TestData{S: k}); !found || v.N != i {
			t.Fatalf("%s not intact after growth: found=%v v=%v", k, found, v)
		}
	}
	checkInvariants(t, ht)
}

// TestThresholdDefaults verifies the constructor's threshold handling: <= 0
// or NaN selects a default, a shrink threshold >= the grow threshold selects
// both defaults, and a shrink threshold above half the grow threshold is
// clamped (the hysteresis that keeps a resize from oscillating).
func TestThresholdDefaults(t *testing.T) {
	cases := []struct {
		grow, shrink         float64
		wantGrow, wantShrink float64
	}{
		{0, 0, defaultGrowAt, defaultShrinkAt},
		{-1, -1, defaultGrowAt, defaultShrinkAt},
		{math.NaN(), 0.1, defaultGrowAt, 0.1},
		{0.9, math.NaN(), 0.9, defaultShrinkAt},
		{0.5, 0.05, 0.5, 0.05},
		{2.0, 0.1, 2.0, 0.1},                        // a grow threshold above 1 defers threshold growth
		{0.05, 0.5, defaultGrowAt, defaultShrinkAt}, // inverted band: both defaults
		{0.5, 0.4, 0.5, 0.25},                       // narrow band: shrink clamped to grow/2
		{0.3, 0.2, 0.3, 0.15},                       // ditto
	}
	for i, c := range cases {
		ht := newTestTab(16, c.grow, c.shrink)
		if ht.growAt != c.wantGrow || ht.shrinkAt != c.wantShrink {
			t.Errorf("case %d: thresholds (%v, %v), want (%v, %v)", i, ht.growAt, ht.shrinkAt, c.wantGrow, c.wantShrink)
		}
	}
}

// TestDeferredGrowthAtHighThreshold sets a grow threshold above 1.0 so the
// threshold can never fire; the table still grows through the collision-loop
// path (a full table forces a resize on the next insert) and keeps every
// element.
func TestDeferredGrowthAtHighThreshold(t *testing.T) {
	ht := newTestTab(8, 2.0, 0.1) // starts at the 256 minimum
	for i := range 300 {
		ht.Insert(TestData{S: fmt.Sprintf("%d", 1000+i), N: i})
	}
	if ht.Len() != 300 {
		t.Fatalf("Len = %d, want 300", ht.Len())
	}
	if ht.Capacity() < 512 || ht.Capacity()&(ht.Capacity()-1) != 0 {
		t.Fatalf("capacity %d cannot hold 300 elements as a power of two past the 256 minimum", ht.Capacity())
	}
	for i := range 300 {
		if _, found := ht.Search(TestData{S: fmt.Sprintf("%d", 1000+i)}); !found {
			t.Fatalf("%d lost with deferred growth", 1000+i)
		}
	}
	checkInvariants(t, ht)
}

// TestShrinkOnDelete fills a 1024-slot table, deletes down to a handful, and
// verifies the table halves back into the 10% band and keeps the survivors.
// The inserts alone may already have grown the table — a collision loop
// forces a resize at any saturation — so the comparison is against the
// capacity right after the inserts.
func TestShrinkOnDelete(t *testing.T) {
	ht := newTestTab(1024, 0.85, 0.10)
	for i := range 500 {
		ht.Insert(TestData{S: fmt.Sprintf("%d", i), N: i})
	}
	capAfterInserts := ht.Capacity()
	if capAfterInserts < 1024 || capAfterInserts&(capAfterInserts-1) != 0 {
		t.Fatalf("capacity = %d after the inserts, want a power of two >= 1024", capAfterInserts)
	}
	for i := range 495 {
		if !ht.Delete(TestData{S: fmt.Sprintf("%d", i)}) {
			t.Fatalf("delete of %d failed", i)
		}
	}
	if ht.Len() != 5 {
		t.Fatalf("Len = %d, want 5", ht.Len())
	}
	if c := ht.Capacity(); c >= capAfterInserts {
		t.Errorf("capacity %d after shrinking to 5 elements (it was %d after the inserts)", c, capAfterInserts)
	}
	if c := ht.Capacity(); c != minTableSize {
		t.Errorf("capacity %d after shrinking to 5 elements, want the %d floor", c, minTableSize)
	}
	if s := ht.Saturation(); s < 0.10 && ht.Capacity() > minTableSize {
		t.Errorf("saturation %.4f still below the shrink threshold at capacity %d", s, ht.Capacity())
	}
	for i := 495; i < 500; i++ {
		if v, found := ht.Search(TestData{S: fmt.Sprintf("%d", i)}); !found || v.N != i {
			t.Fatalf("survivor %d lost after the shrinks", i)
		}
	}
	checkInvariants(t, ht)
}

// TestShrinkFloor verifies the table never halves below the minimum size.
func TestShrinkFloor(t *testing.T) {
	ht := newTestTab(16, 0.85, 0.10)
	for i := range 10 {
		ht.Insert(TestData{S: fmt.Sprintf("%d", i)})
	}
	for i := range 10 {
		ht.Delete(TestData{S: fmt.Sprintf("%d", i)})
	}
	if ht.Capacity() < minTableSize {
		t.Errorf("capacity %d below the minimum %d", ht.Capacity(), minTableSize)
	}
	if !ht.IsEmpty() {
		t.Errorf("table should be empty")
	}
}

// TestPathologicalSameHash pins the pathological panic: more than four
// distinct elements sharing one 64-bit hash whose candidates are distinct
// compete for those four slots at every table size, so the fifth insert can
// never place.
func TestPathologicalSameHash(t *testing.T) {
	ht := NewHashTabFunc(
		func(a, b int) bool { return a == b },
		func(int) uint64 { return 12345 }, // candidates 9,4,14,7 at size 16 — four distinct slots
		16, 0, 0,
	)
	for i := range numPositions {
		if !ht.Insert(i) {
			t.Fatalf("insert %d should place (the family has four candidates)", i)
		}
	}
	expectPanicMessage(t, "fifth same-hash insert", "candidate positions coincide", func() {
		ht.Insert(numPositions)
	})
	// The four placed elements are still intact after the failed insert.
	for i := range numPositions {
		if _, found := ht.Search(i); !found {
			t.Errorf("%d lost after the pathological insert", i)
		}
	}
	if ht.Len() != numPositions {
		t.Errorf("Len = %d, want %d", ht.Len(), numPositions)
	}
}

// TestRandomizedModel is an 800-step property test against a map reference
// model with a fixed seed (42) — keep it deterministic.  Every insert's
// added/replaced report, every delete's and search's found report, and the
// final contents must agree with the model; the structural invariants are
// re-checked periodically.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ht := newTestTab(16, 0, 0)
	model := map[string]int{}

	for step := 0; step < 800; step++ {
		k := fmt.Sprintf("%d", rng.Intn(400))
		switch rng.Intn(10) {
		case 0, 1, 2, 3, 4, 5: // insert
			_, inModel := model[k]
			if added := ht.Insert(TestData{S: k, N: step}); added == inModel {
				t.Fatalf("step %d: Insert(%q) reported added=%v, model has it=%v", step, k, added, inModel)
			}
			model[k] = step
		case 6, 7: // delete
			if found := ht.Delete(TestData{S: k}); found != (model[k] != 0 || false) && found != keyIn(model, k) {
				t.Fatalf("step %d: Delete(%q) reported %v, model presence %v", step, k, found, keyIn(model, k))
			}
			delete(model, k)
		default: // search
			v, found := ht.Search(TestData{S: k})
			if found != keyIn(model, k) {
				t.Fatalf("step %d: Search(%q) found=%v, model presence %v", step, k, found, keyIn(model, k))
			}
			if found && v.N != model[k] {
				t.Fatalf("step %d: Search(%q) satellite %d, model %d", step, k, v.N, model[k])
			}
		}
		if step%100 == 99 {
			checkInvariants(t, ht)
		}
	}

	if ht.Len() != len(model) {
		t.Fatalf("final Len = %d, model has %d", ht.Len(), len(model))
	}
	for k, n := range model {
		if v, found := ht.Search(TestData{S: k}); !found || v.N != n {
			t.Fatalf("final contents disagree on %q: found=%v v=%v model N=%d", k, found, v, n)
		}
	}
	checkInvariants(t, ht)
}

// keyIn is a presence helper that tolerates the zero-value satellite data
// stored by the tests.
func keyIn(model map[string]int, k string) bool {
	_, ok := model[k]
	return ok
}

// TestInfoCounters checks that Info reports the table's size consistently
// with Len/Capacity/Saturation and that the resize counters track the
// synchronous grow and shrink resizes exactly.
func TestInfoCounters(t *testing.T) {
	var nilTab *HashTab[int]
	if got := nilTab.Info(); got != (Info{}) {
		t.Errorf("nil table Info = %+v, want the zero Info", got)
	}

	ht := NewHashTab[int](5, 0, 0) // size 256, thresholds 0.85/0.20
	if got := ht.Info(); got != (Info{Capacity: minTableSize}) {
		t.Errorf("fresh table Info = %+v, want {Capacity:%d}", got, minTableSize)
	}

	// Grow: 1000 inserts cross the grow threshold several times.  Every
	// growth doubles, every shrink halves, so the capacity is exactly the
	// minimum size shifted by the net counter difference.
	for i := range 1000 {
		ht.Insert(i)
	}
	info := ht.Info()
	if info.Len != 1000 || info.Capacity != ht.Capacity() {
		t.Errorf("Info = %+v, but Len = %d and Capacity = %d", info, ht.Len(), ht.Capacity())
	}
	if info.Saturation != ht.Saturation() {
		t.Errorf("Info.Saturation = %v, Saturation() = %v", info.Saturation, ht.Saturation())
	}
	if info.Grows == 0 {
		t.Errorf("Grows = 0 after 1000 inserts into a %d-slot table", minTableSize)
	}
	if info.Forced != 0 {
		t.Errorf("Forced = %d, want 0 — threshold growth should preempt collision loops", info.Forced)
	}
	if want := minTableSize << (info.Grows - info.Shrinks); info.Capacity != want {
		t.Errorf("Capacity = %d, want %d = %d << (%d grows - %d shrinks)",
			info.Capacity, want, minTableSize, info.Grows, info.Shrinks)
	}

	// Shrink: delete down to a single element; the table halves back to
	// the minimum size.
	for i := 0; i < 999; i++ {
		ht.Delete(i)
	}
	info = ht.Info()
	if info.Shrinks == 0 {
		t.Errorf("Shrinks = 0 after deleting 999 of 1000 elements")
	}
	if info.Capacity != minTableSize {
		t.Errorf("Capacity = %d, want %d (the shrink floor)", info.Capacity, minTableSize)
	}
	if want := minTableSize << (info.Grows - info.Shrinks); info.Capacity != want {
		t.Errorf("Capacity = %d, want %d = %d << (%d grows - %d shrinks)",
			info.Capacity, want, minTableSize, info.Grows, info.Shrinks)
	}
}

// TestInfoForcedCountsCollisionLoops checks that the resizes a pathological
// hash forces inside Insert are counted as Forced (and as Grows) — one per
// escalation, maxResizeAttempts of them before the panic.
func TestInfoForcedCountsCollisionLoops(t *testing.T) {
	ht := NewHashTabFunc(
		func(a, b int) bool { return a == b },
		func(int) uint64 { return 12345 }, // four distinct slots, the same for every element
		16, 0, 0,
	)
	for i := range numPositions {
		if !ht.Insert(i) {
			t.Fatalf("insert %d should place (the family has four candidates)", i)
		}
	}
	expectPanicMessage(t, "fifth same-hash insert", "candidate positions coincide", func() {
		ht.Insert(numPositions)
	})
	// The fifth insert escalates through maxResizeAttempts forced resizes
	// before panicking; at size 256 the constant hash yields only three
	// distinct slots, so placing the fourth element forced one more.  The
	// exact split is hash-dependent — assert the totals, not the split.
	info := ht.Info()
	if info.Forced < maxResizeAttempts {
		t.Errorf("Forced = %d, want at least %d (one per escalation)", info.Forced, maxResizeAttempts)
	}
	if info.Grows != info.Forced {
		t.Errorf("Grows = %d, want %d = Forced (every grow was forced; the thresholds never engaged at length %d)",
			info.Grows, info.Forced, numPositions)
	}
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
// compare contents without depending on slot order (which varies with the
// hash seed and the displacement history).
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
	// The elements are all present, as a JSON array (slot order varies).
	ht := newTestTab(16, 0, 0)
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
	if b, err := json.Marshal(newTestTab(16, 0, 0)); err != nil || string(b) != "[]" {
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
	custom := NewHashTab[upperString](16, 0, 0)
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
	bad := NewHashTab[chan int](16, 0, 0)
	bad.Insert(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a table of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Every decoded element is stored and searchable.
	ht := NewHashTab[string](16, 0, 0)
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
	items := newTestTab(16, 0, 0)
	for i, s := range []string{"a", "b", "c"} {
		items.Insert(TestData{S: s, N: i})
	}
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestTab(16, 0, 0)
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
	full := newTestTab(16, 0, 0)
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
	dup := newTestTab(16, 0, 0)
	if err := json.Unmarshal([]byte(`[{"S":"k","N":1},{"S":"k","N":2}]`), dup); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v, found := dup.Search(TestData{S: "k"}); !found || v.N != 2 || dup.Len() != 1 {
		t.Errorf("duplicate decode: (%v, %v) Len=%d, want N=2 found Len=1", v, found, dup.Len())
	}

	// Element-level unmarshalers are honored.
	custom := NewHashTab[upperString](16, 0, 0)
	if err := json.Unmarshal([]byte(`["X","Y"]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	for _, want := range []upperString{"X", "Y"} {
		if _, found := custom.Search(want); !found {
			t.Errorf("%q not found after unmarshal", want)
		}
	}

	// Decode errors are returned and leave the table untouched.
	keep := newTestTab(16, 0, 0)
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
	expectPanicMessage(t, "UnmarshalJSON on nil table", "cuckoo: UnmarshalJSON called on a nil table",
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

	d := Doc{Title: "pluto", Tags: NewHashTab[string](16, 0, 0)}
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
	out.Tags = NewHashTab[string](16, 0, 0)
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
	clearDoc := Doc{Title: "x", Tags: NewHashTab[string](16, 0, 0)}
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
	rng := rand.New(rand.NewSource(20260903))
	const ops = 400

	ht := newTestTab(16, 0, 0)
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
			fresh := newTestTab(16, 0, 0)
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
			checkInvariants(t, fresh)
		}
	}
	checkInvariants(t, ht)
}
