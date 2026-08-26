package cuckoo_ts

// Thread-safe version of cuckoo table.

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"fmt"
	"hash/fnv"
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
// tests can reason about slot placement.  The default NewHashTab hashes
// with a per-process random maphash seed, which is fine for membership
// assertions but not for placement.
func newTestTab(n int, growAt, shrinkAt float64) *HashTab[TestData] {
	return NewHashTabFunc(
		func(a, b TestData) bool { return a.S == b.S },
		func(v TestData) uint64 {
			h := fnv.New64a()
			_, _ = h.Write([]byte(v.S))
			return h.Sum64()
		},
		n, growAt, shrinkAt,
	)
}

// TestBasicInsertSearchDelete covers the membership core: insert, search,
// duplicate replacement, delete, and the counters.
func TestBasicInsertSearchDelete(t *testing.T) {
	ht := newTestTab(16, 0, 0)

	if !ht.Insert(TestData{S: "one", N: 1}) {
		t.Errorf("first insert of \"one\" should report added")
	}
	if !ht.Insert(TestData{S: "two", N: 2}) {
		t.Errorf("first insert of \"two\" should report added")
	}
	if ht.Insert(TestData{S: "one", N: 11}) {
		t.Errorf("duplicate insert of \"one\" should report replaced")
	}
	if ht.Len() != 2 {
		t.Fatalf("Len = %d, want 2 (a duplicate replaces, it does not add)", ht.Len())
	}
	if v, found := ht.Search(TestData{S: "one"}); !found {
		t.Errorf("\"one\" not found")
	} else if v.N != 11 {
		t.Errorf("\"one\" stored N = %d, want the replaced value 11", v.N)
	}
	if _, found := ht.Search(TestData{S: "three"}); found {
		t.Errorf("\"three\" should not be found")
	}

	if !ht.Delete(TestData{S: "one"}) {
		t.Errorf("Delete of \"one\" should return true")
	}
	if ht.Delete(TestData{S: "one"}) {
		t.Errorf("second Delete of \"one\" should return false")
	}
	if _, found := ht.Search(TestData{S: "one"}); found {
		t.Errorf("\"one\" should be gone after the delete")
	}
	if ht.Len() != 1 || ht.IsEmpty() {
		t.Errorf("Len = %d, IsEmpty = %v after one delete", ht.Len(), ht.IsEmpty())
	}
}

// TestPositionsDerivedFromBaseHash pins the position derivation itself: for
// a known base hash each occupied slot must be one of the four shifted/masked
// candidates.  The internals are inspected under Lock; no background resizer
// is active (the thresholds are never crossed).
func TestPositionsDerivedFromBaseHash(t *testing.T) {
	ht := newTestTab(32, 0, 0)
	for i := range 10 {
		ht.Insert(TestData{S: fmt.Sprintf("k%d", i), N: i})
	}
	ht.Lock()
	mask := uint64(ht.size - 1)
	bad := -1
	var badHash uint64
	for i := range ht.slots {
		if !ht.slots[i].used {
			continue
		}
		ok := false
		for j := 0; j < 4; j++ {
			if i == posOf(ht.slots[i].hash, j, mask) {
				ok = true
				break
			}
		}
		if !ok {
			bad, badHash = i, ht.slots[i].hash
			break
		}
	}
	ht.Unlock()
	if bad >= 0 {
		t.Fatalf("slot %d holds h=%d which is not one of its own four candidates for mask %d", bad, badHash, mask)
	}
}

// TestCapacityIsPowerOfTwo verifies the constructor rounds up to a power of
// two with the minimum size honored.
func TestCapacityIsPowerOfTwo(t *testing.T) {
	for _, n := range []int{5, 8, 9, 13, 16, 17, 100, 129, 1000} {
		ht := newTestTab(n, 0, 0)
		c := ht.Capacity()
		if c < minTableSize || c&(c-1) != 0 {
			t.Errorf("n = %d: capacity %d is not a power of two >= %d", n, c, minTableSize)
		}
		if c < n {
			t.Errorf("n = %d: capacity %d is smaller than the requested size", n, c)
		}
	}
}

// TestDefaultConstructor exercises NewHashTab (random maphash seed) for
// membership only — placement is nondeterministic.  The growth past 0.85 is
// asynchronous, so the capacity check waits for the resizer.
func TestDefaultConstructor(t *testing.T) {
	ht := NewHashTab[string](16, 0, 0)
	for i := range 100 {
		ht.Insert(fmt.Sprintf("item-%d", i))
	}
	if ht.Len() != 100 {
		t.Fatalf("Len = %d, want 100", ht.Len())
	}
	for i := range 100 {
		if _, found := ht.Search(fmt.Sprintf("item-%d", i)); !found {
			t.Fatalf("item-%d not found", i)
		}
	}
	waitQuiescent(t, ht)
	if ht.Saturation() > 0.85 {
		t.Errorf("saturation %.4f above the 0.85 grow threshold after the resizer finished", ht.Saturation())
	}
}

// TestHashOfExactlyZero needs no remapping: the occupied flag marks a slot,
// so a base hash of exactly 0 is stored and compared as-is.
func TestHashOfExactlyZero(t *testing.T) {
	ht := NewHashTabFunc(
		func(a, b int) bool { return a == b },
		func(int) uint64 { return 0 },
		16, 0, 0,
	)
	ht.Insert(7)
	ht.Lock()
	zeroStored := ht.slots[0].hash == 0 && ht.slots[0].used
	ht.Unlock()
	if !zeroStored {
		t.Fatalf("the zero hash should be stored as-is in slot 0")
	}
	if _, found := ht.Search(7); !found {
		t.Fatalf("7 not found with an all-zero hash")
	}
	if _, found := ht.Search(8); found {
		t.Errorf("8 should not be found")
	}
	if !ht.Delete(7) {
		t.Errorf("Delete with an all-zero hash should succeed")
	}
	if ht.Len() != 0 || !ht.IsEmpty() {
		t.Errorf("Len = %d after the delete", ht.Len())
	}
}

// TestDumpOutput checks the Dump header and slot lines.
func TestDumpOutput(t *testing.T) {
	ht := newTestTab(8, 0, 0) // rounds up to the 256 minimum
	ht.Insert(TestData{S: "a"})
	ht.Insert(TestData{S: "b"})
	buf := new(bytes.Buffer)
	ht.Dump(buf)
	out := buf.String()
	if !strings.HasPrefix(out, "Elements: 2, table size:256, saturation:") {
		t.Errorf("unexpected Dump header: %q", strings.SplitN(out, "\n", 2)[0])
	}
	if strings.Count(out, "empty") != minTableSize-2 {
		t.Errorf("Dump should list %d empty slots for 2 of %d used, got %d", minTableSize-2, minTableSize, strings.Count(out, "empty"))
	}
}

// TestTruncateReuse verifies Truncate clears the table but keeps it usable.
func TestTruncateReuse(t *testing.T) {
	ht := newTestTab(16, 0, 0)
	for i := range 20 {
		ht.Insert(TestData{S: fmt.Sprintf("x%d", i)})
	}
	ht.Truncate()
	if !ht.IsEmpty() || ht.Len() != 0 {
		t.Fatalf("table not empty after Truncate: Len = %d", ht.Len())
	}
	// The table remains usable afterwards.
	if !ht.Insert(TestData{S: "fresh"}) {
		t.Errorf("insert after Truncate should add")
	}
	if _, found := ht.Search(TestData{S: "fresh"}); !found {
		t.Errorf("insert after Truncate not found")
	}
}

// TestOracleVsMap cross-checks a filled table against a map model.
func TestOracleVsMap(t *testing.T) {
	ht := newTestTab(16, 0, 0)
	model := map[string]int{}
	for i := range 500 {
		k := fmt.Sprintf("key-%d", i)
		ht.Insert(TestData{S: k, N: i})
		model[k] = i
	}
	if ht.Len() != len(model) {
		t.Fatalf("Len = %d, model has %d", ht.Len(), len(model))
	}
	for k, n := range model {
		v, found := ht.Search(TestData{S: k})
		if !found {
			t.Fatalf("%s not found", k)
		}
		if v.N != n {
			t.Fatalf("%s stored N = %d, want %d", k, v.N, n)
		}
	}
}

// TestIterators checks All/Values yield exactly the stored elements and that
// All's single-variable form yields the position.
func TestIterators(t *testing.T) {
	ht := newTestTab(16, 0, 0)
	for i := range 40 {
		ht.Insert(TestData{S: fmt.Sprintf("it%d", i)})
	}
	seen := map[string]int{}
	for pos, item := range ht.All() {
		if pos < 0 || pos >= ht.Capacity() {
			t.Fatalf("All yielded out-of-range position %d", pos)
		}
		seen[item.S]++
	}
	if len(seen) != 40 {
		t.Fatalf("All visited %d distinct elements, want 40", len(seen))
	}
	n := 0
	for item := range ht.Values() {
		if _, ok := seen[item.S]; !ok {
			t.Fatalf("Values yielded %q which All did not", item.S)
		}
		n++
	}
	if n != 40 {
		t.Fatalf("Values yielded %d elements, want 40", n)
	}
}

// TestWalk counts visits and checks the early-stop convention (returning
// false stops the walk and makes Walk return false).
func TestWalk(t *testing.T) {
	ht := newTestTab(16, 0, 0)
	for i := range 10 {
		ht.Insert(TestData{S: fmt.Sprintf("w%d", i)})
	}
	n := 0
	if !ht.Walk(func(pos int, data TestData) bool {
		n++
		return true
	}) || n != 10 {
		t.Errorf("full walk: n = %d, Walk returned false", n)
	}
	n = 0
	if ht.Walk(func(pos int, data TestData) bool {
		n++
		return n < 3 // returning false on the third visit stops the walk
	}) || n != 3 {
		t.Errorf("stopped walk: n = %d, Walk returned true", n)
	}
}
