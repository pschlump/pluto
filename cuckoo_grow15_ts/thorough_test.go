package cuckoo_grow15_ts

// Thorough tests for the thread-safe cuckoo table: nil and zero-value
// behavior, the panic contract with its messages, replacement semantics, the
// asynchronous grow and shrink resizes, snapshot iterator semantics,
// Lock+Nl compound operations, a randomized cross-check against a map model,
// and concurrent hammering for the race detector.  TestData and newTestTab
// are defined in hash_tab_test.go and reused here.

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
	"time"
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

// waitQuiescent waits until no background resize is in flight.  The resizing
// flag is written under the write lock, so reading it under Lock is
// definitive: false means the goroutine has exited (or never started) and
// the thresholds are not due, so the table is stable.
func waitQuiescent[T any](t *testing.T, ht *HashTab[T]) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		ht.Lock()
		busy := ht.resizing
		ht.Unlock()
		if !busy {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("background resizer did not become quiescent within 5s")
		}
		time.Sleep(time.Millisecond)
	}
}

// checkInvariants verifies the structural invariants of the table: every
// occupied slot's stored hash re-hashes to itself and the slot is one of the
// four candidates derived from that hash, the occupied count matches Len,
// Search finds every stored element, and Walk and the iterators agree with
// the slots.  It takes the write lock itself, so it is safe to call even
// while a background resizer might still be running; once it holds the lock
// only the Nl methods are used inside.  Call it after structural changes.
func checkInvariants(t *testing.T, ht *HashTab[TestData]) {
	t.Helper()
	var zero TestData
	ht.Lock()
	occupied := 0
	var stored []TestData
	for i := range ht.slots {
		s := ht.slots[i]
		if !s.used {
			if s.data != zero || s.hash != 0 {
				ht.Unlock()
				t.Fatalf("slot %d is empty but holds %v h=%d", i, s.data, s.hash)
			}
			continue
		}
		occupied++
		if s.hash != ht.hash(s.data) {
			ht.Unlock()
			t.Fatalf("slot %d stored hash %d, but re-hashing %v gives %d", i, s.hash, s.data, ht.hash(s.data))
		}
		ok := false
		for j := 0; j < numPositions; j++ {
			if i == posOf(s.hash, j, ht.size) {
				ok = true
				break
			}
		}
		if !ok {
			ht.Unlock()
			t.Fatalf("slot %d is not one of the four candidates of its hash %d (size %d)", i, s.hash, ht.size)
		}
		if _, found := ht.NlSearch(s.data); !found {
			ht.Unlock()
			t.Fatalf("element in slot %d (%v) not found by Search", i, s.data)
		}
		stored = append(stored, s.data)
	}
	length := ht.length
	ht.Unlock()

	if occupied != length {
		t.Fatalf("occupied slots = %d, Len() = %d", occupied, length)
	}
	seen := make(map[string]int)
	for _, d := range stored {
		seen[d.S]++
	}
	if len(seen) != length {
		t.Fatalf("table holds %d distinct elements, Len() = %d", len(seen), length)
	}
	for k, n := range seen {
		if n != 1 {
			t.Fatalf("key %q held %d times, want 1", k, n)
		}
	}
	seenAll := make(map[string]int)
	for pos, item := range ht.All() {
		if pos < 0 || pos >= ht.Capacity() {
			t.Fatalf("All reported out-of-range position %d", pos)
		}
		seenAll[item.S]++
	}
	if len(seenAll) != length {
		t.Fatalf("All visited %d distinct elements, Len() = %d", len(seenAll), length)
	}
	for k, n := range seenAll {
		if seen[k] != n {
			t.Fatalf("key %q seen %d times by All but %d times in the table", k, n, seen[k])
		}
	}
	nWalk := 0
	ht.Walk(func(pos int, data TestData) bool {
		nWalk++
		return true
	})
	if nWalk != length {
		t.Fatalf("Walk visited %d, Len() = %d", nWalk, length)
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

	// A nil table's Lock/Unlock are no-ops (must not panic); on a nil table
	// no lock is really taken, so a read between them is safe.
	var nilLockTable *HashTab[TestData]
	nilLockTable.Lock()
	if !nilLockTable.IsEmpty() {
		t.Errorf("nil table should be empty inside Lock")
	}
	nilLockTable.Unlock()

	// The zero value locks for real; an internal read belongs between.
	zero := &HashTab[TestData]{}
	zero.Lock()
	if zero.length != 0 {
		t.Errorf("zero value should be empty inside Lock")
	}
	zero.Unlock()
}

// TestHashTabNilPanics verifies the panic contract and that each message
// names the method and the fix.
func TestHashTabNilPanics(t *testing.T) {
	var nilTable *HashTab[TestData]
	expectPanicMessage(t, "Insert on nil table", "cuckoo_grow15_ts: Insert called on a nil table",
		func() { nilTable.Insert(TestData{S: "x"}) })
	expectPanicMessage(t, "Insert on zero-value table", "no equality/hash functions",
		func() { (&HashTab[TestData]{}).Insert(TestData{S: "x"}) })
	expectPanicMessage(t, "NewHashTabFunc nil eq", "cuckoo_grow15_ts: NewHashTabFunc called with a nil equality function",
		func() { NewHashTabFunc(nil, func(TestData) uint64 { return 1 }, 16, 0, 0) })
	expectPanicMessage(t, "NewHashTabFunc nil hash", "cuckoo_grow15_ts: NewHashTabFunc called with a nil hash function",
		func() { NewHashTabFunc(func(a, b TestData) bool { return a.S == b.S }, nil, 16, 0, 0) })
	expectPanicMessage(t, "n < 5", "initial size must be at least 5",
		func() { NewHashTab[string](4, 0, 0) })
	expectPanic(t, "n < 5 via NewHashTabFunc", func() {
		NewHashTabFunc(func(a, b int) bool { return a == b }, func(int) uint64 { return 1 }, 1, 0, 0)
	})
}

// TestThresholdDefaults verifies the constructor's threshold handling: <= 0
// or NaN selects a default, a shrink threshold >= the grow threshold selects
// both defaults, and a shrink threshold above two thirds of the grow
// threshold is clamped (the hysteresis that keeps the background resizer
// from oscillating).
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
		{0.5, 0.4, 0.5, 0.5 / 1.5},                  // narrow band: shrink clamped to grow/1.5
		{0.3, 0.2, 0.3, 0.19999999999999998},        // ditto (runtime 0.3/1.5, not the exact constant)
	}
	for i, c := range cases {
		ht := newTestTab(16, c.grow, c.shrink)
		ht.Lock()
		gotGrow, gotShrink := ht.growAt, ht.shrinkAt
		ht.Unlock()
		if gotGrow != c.wantGrow || gotShrink != c.wantShrink {
			t.Errorf("case %d: thresholds (%v, %v), want (%v, %v)", i, gotGrow, gotShrink, c.wantGrow, c.wantShrink)
		}
	}
}

// TestNarrowBandNoOscillation pins the hysteresis clamp: with the requested
// shrink threshold above two thirds of the grow threshold, a threshold
// resize would otherwise oscillate (grow, shrink, grow, ...) on the
// background goroutine forever.  waitQuiescent fatals after 5s if the
// resizer never exits.
func TestNarrowBandNoOscillation(t *testing.T) {
	ht := newTestTab(16, 0.5, 0.4) // starts at the 256 minimum; shrink clamped to 0.5/1.5
	ht.Lock()
	clamped := ht.shrinkAt
	ht.Unlock()
	if clamped != 0.5/1.5 {
		t.Fatalf("shrink threshold %.4f, want the clamped %v", clamped, 0.5/1.5)
	}
	for i := range 140 { // crosses 0.5 at 130/256 = 0.508
		ht.Insert(TestData{S: fmt.Sprintf("n%03d", i)})
	}
	waitQuiescent(t, ht)
	if ht.Len() != 140 {
		t.Fatalf("Len = %d, want 140", ht.Len())
	}
	for i := range 140 {
		if _, found := ht.Search(TestData{S: fmt.Sprintf("n%03d", i)}); !found {
			t.Fatalf("n%03d lost by the narrow-band resize", i)
		}
	}
	checkInvariants(t, ht)
}

// TestInsertReplacesExisting verifies a duplicate insert replaces the stored
// satellite data in place: count unchanged, the stored hash unchanged.
func TestInsertReplacesExisting(t *testing.T) {
	ht := newTestTab(16, 0, 0)
	ht.Insert(TestData{S: "dup", N: 1})
	oldLen, oldCap := ht.Len(), ht.Capacity()
	ht.Lock()
	var oldHash uint64
	oldSlot := -1
	for i := range ht.slots {
		if ht.slots[i].used {
			oldSlot, oldHash = i, ht.slots[i].hash
		}
	}
	ht.Unlock()
	if ht.Insert(TestData{S: "dup", N: 2}) {
		t.Errorf("duplicate insert should report replaced")
	}
	if ht.Len() != oldLen || ht.Capacity() != oldCap {
		t.Errorf("replacement changed the table: Len %d->%d, Capacity %d->%d", oldLen, ht.Len(), oldCap, ht.Capacity())
	}
	ht.Lock()
	inPlace := ht.slots[oldSlot].used && ht.slots[oldSlot].hash == oldHash
	ht.Unlock()
	if !inPlace {
		t.Errorf("replacement should keep the element in place with its hash")
	}
	if v, _ := ht.Search(TestData{S: "dup"}); v.N != 2 {
		t.Errorf("stored satellite = %d, want 2", v.N)
	}
	checkInvariants(t, ht)
}

// TestAsyncGrowPastThreshold fills a 256-slot table past the 0.85 grow
// threshold and verifies the background resizer grows the table and that
// every element survives.
func TestAsyncGrowPastThreshold(t *testing.T) {
	ht := newTestTab(16, 0.85, 0.10) // starts at the 256 minimum
	for i := range 220 {             // crosses 0.85 at 219/256 = 0.855
		ht.Insert(TestData{S: fmt.Sprintf("a%03d", i), N: i})
	}
	waitQuiescent(t, ht)
	if c := ht.Capacity(); c <= 256 {
		t.Errorf("capacity %d after crossing 0.85 on 256 slots, the resizer should have grown it", c)
	}
	if s := ht.Saturation(); s > 0.85 {
		t.Errorf("saturation %.4f above 0.85 after the resizer finished", s)
	}
	for i := range 220 {
		if v, found := ht.Search(TestData{S: fmt.Sprintf("a%03d", i)}); !found || v.N != i {
			t.Fatalf("a%03d lost by the background grow: found=%v v=%v", i, found, v)
		}
	}
	checkInvariants(t, ht)
}

// TestAsyncShrinkBelowThreshold fills a table, deletes down to a handful,
// and verifies the background resizer shrinks the table back into the 10%
// band and keeps the survivors.
func TestAsyncShrinkBelowThreshold(t *testing.T) {
	ht := newTestTab(1024, 0.85, 0.10)
	for i := range 600 {
		ht.Insert(TestData{S: fmt.Sprintf("%d", i), N: i})
	}
	waitQuiescent(t, ht)
	capAfterInserts := ht.Capacity()
	if capAfterInserts < 1024 {
		t.Fatalf("capacity = %d after the inserts, want at least 1024", capAfterInserts)
	}
	for i := range 595 {
		if !ht.Delete(TestData{S: fmt.Sprintf("%d", i)}) {
			t.Fatalf("delete of %d failed", i)
		}
	}
	waitQuiescent(t, ht)
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
	for i := 595; i < 600; i++ {
		if v, found := ht.Search(TestData{S: fmt.Sprintf("%d", i)}); !found || v.N != i {
			t.Fatalf("survivor %d lost after the shrinks", i)
		}
	}
	checkInvariants(t, ht)
}

// TestShrinkFloor verifies the table never shrinks below the minimum size.
func TestShrinkFloor(t *testing.T) {
	ht := newTestTab(16, 0.85, 0.10)
	for i := range 10 {
		ht.Insert(TestData{S: fmt.Sprintf("%d", i)})
	}
	for i := range 10 {
		ht.Delete(TestData{S: fmt.Sprintf("%d", i)})
	}
	waitQuiescent(t, ht)
	if ht.Capacity() < minTableSize {
		t.Errorf("capacity %d below the minimum %d", ht.Capacity(), minTableSize)
	}
	if !ht.IsEmpty() {
		t.Errorf("table should be empty")
	}
}

// TestTruncateDuringPendingResize truncates while a background resize is in
// flight: the resizer must wake, find nothing to do, and exit leaving a
// consistent empty table.
func TestTruncateDuringPendingResize(t *testing.T) {
	ht := newTestTab(16, 0.85, 0.10) // starts at the 256 minimum
	for i := range 220 {             // crosses 0.85: a background grow is armed or running
		ht.Insert(TestData{S: fmt.Sprintf("t%03d", i)})
	}
	ht.Truncate()
	waitQuiescent(t, ht)
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Fatalf("table not empty after Truncate: Len = %d", ht.Len())
	}
	for i := range 220 {
		if _, found := ht.Search(TestData{S: fmt.Sprintf("t%03d", i)}); found {
			t.Fatalf("t%03d resurrected after Truncate", i)
		}
	}
	checkInvariants(t, ht)
}

// TestDeferredGrowthAtHighThreshold sets a grow threshold above 1.0 so the
// threshold can never fire; the table still grows through the synchronous
// collision-loop path and keeps every element.
func TestDeferredGrowthAtHighThreshold(t *testing.T) {
	ht := newTestTab(8, 2.0, 0.1) // starts at the 256 minimum
	for i := range 300 {
		ht.Insert(TestData{S: fmt.Sprintf("%d", 1000+i), N: i})
	}
	if ht.Len() != 300 {
		t.Fatalf("Len = %d, want 300", ht.Len())
	}
	if ht.Capacity() < 384 { // 256 grown by half again, the first ×1.5 step
		t.Fatalf("capacity %d cannot hold 300 elements past the 256 minimum", ht.Capacity())
	}
	for i := range 300 {
		if _, found := ht.Search(TestData{S: fmt.Sprintf("%d", 1000+i)}); !found {
			t.Fatalf("%d lost with deferred growth", 1000+i)
		}
	}
	checkInvariants(t, ht)
}

// TestPathologicalSameHash pins the pathological panic (synchronous path: it
// is the insert itself that cannot place): more than four distinct elements
// sharing one 64-bit hash compete for those four candidate slots at every
// table size, so the fifth insert can never place.
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

// TestIteratorSnapshotSemantics verifies All and Values iterate a value
// snapshot taken when they are called: later mutations neither appear in the
// loop nor invalidate it, and the loop body may safely touch the table.
func TestIteratorSnapshotSemantics(t *testing.T) {
	ht := newTestTab(16, 0, 0)
	for i := range 10 {
		ht.Insert(TestData{S: fmt.Sprintf("s%d", i)})
	}
	n := 0
	for _, item := range ht.All() {
		n++
		ht.Delete(item) // mutating from inside the loop must not affect the snapshot
		ht.Search(item)
	}
	if n != 10 {
		t.Fatalf("snapshot iteration visited %d elements, want 10", n)
	}
	if !ht.IsEmpty() {
		t.Fatalf("deletes from inside the snapshot loop should have emptied the table, Len = %d", ht.Len())
	}
}

// TestLockNlCompound verifies compound Lock + Nl sections behave as one
// atomic step against concurrent locked operations.
func TestLockNlCompound(t *testing.T) {
	ht := newTestTab(16, 0, 0)
	for i := range 20 {
		ht.Insert(TestData{S: fmt.Sprintf("c%02d", i), N: i})
	}

	// Atomic search-then-delete: only one of two racing deleters wins.
	ht.Lock()
	if _, found := ht.NlSearch(TestData{S: "c07"}); !found {
		ht.Unlock()
		t.Fatalf("c07 not found under Lock")
	}
	won := ht.NlDelete(TestData{S: "c07"})
	ht.Unlock()
	if !won {
		t.Errorf("NlDelete after NlSearch should win")
	}
	if ht.Delete(TestData{S: "c07"}) {
		t.Errorf("second delete of c07 should lose — the compound was atomic")
	}
	if _, found := ht.Search(TestData{S: "c07"}); found {
		t.Errorf("c07 should be gone")
	}

	// NlInsert is the Insert body — including its zero-value panic.
	ht.Lock()
	added := ht.NlInsert(TestData{S: "new"})
	n := ht.NlLen()
	ht.Unlock()
	if !added || n != 20 {
		t.Errorf("NlInsert under Lock: added=%v NlLen=%d, want true and 20", added, n)
	}
}

// TestRandomizedModel is an 800-step property test against a map reference
// model with a fixed seed (42) — keep it deterministic.  Every insert's
// added/replaced report, every delete's and search's found report, and the
// final contents must agree with the model; the structural invariants are
// re-checked periodically (after waiting out any background resize).
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
			if found := ht.Delete(TestData{S: k}); found != keyIn(model, k) {
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
			waitQuiescent(t, ht)
			checkInvariants(t, ht)
		}
	}

	waitQuiescent(t, ht)
	if ht.Len() != len(model) {
		t.Fatalf("final Len = %d, model has %d", ht.Len(), len(model))
	}
	for k, n := range model {
		if v, found := ht.Search(TestData{S: k}); !found || v.N != n {
			t.Fatalf("final contents disagree on %q: found=%v v=%v model N=%d", k, found, v, n)
		}
	}
}

// keyIn is a presence helper that tolerates the zero-value satellite data
// stored by the tests.
func keyIn(model map[string]int, k string) bool {
	_, ok := model[k]
	return ok
}

// TestFailedShrinkResizerExits pins the liveness guard on the background
// resizer: a best-effort shrink whose rebuild cannot place every element
// must end the resizer (keeping the current size), not retry the same
// failed shrink in a hot loop forever.  Five elements sharing one 64-bit
// hash compete for the same four candidate slots at every table size, so
// the shrink rebuild always fails.  Called synchronously here — before the
// guard this call never returned.
func TestFailedShrinkResizerExits(t *testing.T) {
	ht := NewHashTabFunc(
		func(a, b TestData) bool { return a.S == b.S },
		func(TestData) uint64 { return 0xdeadbeef }, // constant: every element shares one hash
		512, 0.85, 0.10,
	)
	// Populate five slots by hand with that one hash — Insert would panic
	// on the fifth element (the pathological panic), which is exactly the
	// state a shrink rebuild cannot re-place at any size.
	ht.Lock()
	for i := range 5 {
		ht.slots[i] = slot[TestData]{data: TestData{S: fmt.Sprintf("s%d", i)}, hash: 0xdeadbeef, used: true}
	}
	ht.length = 5 // saturation 5/512 = 0.0098 < 0.10: the shrink is due
	ht.Unlock()

	ht.backgroundResize() // must return, not spin on the failed shrink

	ht.Lock()
	size, resizing := ht.size, ht.resizing
	ht.Unlock()
	if resizing {
		t.Errorf("resizer still marked in flight after the failed shrink")
	}
	if size != 512 {
		t.Errorf("size = %d after the failed shrink, want the unchanged 512", size)
	}
}

// TestConcurrentInsertSearchIterate runs writers inserting disjoint key
// ranges against one shared table while readers search and drain the
// snapshot iterators — with the background resizer growing the table under
// everyone.  It is primarily a test for the race detector (`make race`); the
// final state must be exactly the union of the ranges.
func TestConcurrentInsertSearchIterate(t *testing.T) {
	ht := newTestTab(16, 0.85, 0.10)
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
				ht.Saturation() // reads are lock-safe while the resizer runs
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
	waitQuiescent(t, ht)

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
// from concurrent goroutines while readers search and iterate — exercising
// the background shrink alongside the deletions.  After the wait the table
// must be empty and every key not-found.  Race-detector target
// (`make race`).
func TestConcurrentDelete(t *testing.T) {
	ht := newTestTab(16, 0.85, 0.10)
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
	waitQuiescent(t, ht)

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
	ht := newTestTab(16, 0.85, 0.10)

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
	waitQuiescent(t, ht)

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
	expectPanicMessage(t, "UnmarshalJSON on nil table", "cuckoo_grow15_ts: UnmarshalJSON called on a nil table",
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
	rng := rand.New(rand.NewSource(20260902))
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
		}
	}
	waitQuiescent(t, ht)
	checkInvariants(t, ht)
}

// TestJSONConcurrent hammers MarshalJSON and UnmarshalJSON concurrently with
// a marshaling reader; every output must be a valid JSON array.  Run under
// -race (make race).
func TestJSONConcurrent(t *testing.T) {
	ht := newTestTab(16, 0, 0)

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
	waitQuiescent(t, ht)
	checkInvariants(t, ht)
}
