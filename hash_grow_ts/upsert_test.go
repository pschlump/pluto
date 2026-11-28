/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_grow_ts

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"testing"
)

// Tests for the single-probe compound operations Upsert, GetOrInsert and
// Remove.  The model tests drive every hash shape that stresses the probe
// logic — spread (FNV), constant (one long chain), all-zero (the 0 -> 1
// remap on a single chain), and a few-homes hash (distinct hashes sharing
// home buckets) — against a Go map, re-verifying every membership after
// each removal so a broken backward shift cannot hide.

// upsertHashShapes are the hash functions the model tests run under.
var upsertHashShapes = []struct {
	name     string
	universe int // distinct keys; small for the degenerate shapes
	hash     func(TestData) uint64
}{
	{"fnv", 400, func(v TestData) uint64 {
		h := fnv.New64a()
		_, _ = h.Write([]byte(v.S))
		return h.Sum64()
	}},
	{"constant", 60, func(TestData) uint64 { return 12345 }},
	{"zero", 60, func(TestData) uint64 { return 0 }},
	{"sharedHome", 200, func(v TestData) uint64 {
		h := fnv.New64a()
		_, _ = h.Write([]byte(v.S))
		// Distinct hashes, but only 4 distinct values mod any power of two
		// up to 2^20: every key lands on one of 4 home buckets.
		return (h.Sum64()%4)<<20 | (h.Sum64() >> 44 << 22) | 1<<62
	}},
}

func newShapeTab(hash func(TestData) uint64, n int, saturation float64) *HashTab[TestData] {
	return NewHashTabFunc(func(a, b TestData) bool { return a.S == b.S }, hash, n, saturation)
}

// verifyAgainstModel checks Len and every key of the universe against the
// model, then the structural invariants.
func verifyAgainstModel(t *testing.T, ht *HashTab[TestData], model map[string]int, universe int, step int) {
	t.Helper()
	if ht.Len() != len(model) {
		t.Fatalf("step %d: Len() = %d, model has %d", step, ht.Len(), len(model))
	}
	for k := range universe {
		key := fmt.Sprintf("k%04d", k)
		got, found := ht.Search(TestData{S: key})
		want, inModel := model[key]
		if found != inModel {
			t.Fatalf("step %d: Search(%q) found=%v, model=%v", step, key, found, inModel)
		}
		if found && got.N != want {
			t.Fatalf("step %d: Search(%q).N = %d, model %d", step, key, got.N, want)
		}
	}
	checkInvariants(t, ht)
}

func TestUpsertGetOrInsertRemoveModel(t *testing.T) {
	for _, shape := range upsertHashShapes {
		for _, cfg := range []struct {
			n          int
			saturation float64
		}{
			{8, 0},   // power-of-two sizes: the mask home path, normal growth
			{5, 1.0}, // non-power-of-two (% path), growth deferred until full
			{7, 0.9},
		} {
			t.Run(fmt.Sprintf("%s/n=%d/sat=%v", shape.name, cfg.n, cfg.saturation), func(t *testing.T) {
				rng := rand.New(rand.NewSource(42))
				ht := newShapeTab(shape.hash, cfg.n, cfg.saturation)
				model := map[string]int{}
				for step := range 3000 {
					key := fmt.Sprintf("k%04d", rng.Intn(shape.universe))
					n := rng.Intn(1_000_000)
					want, present := model[key]
					switch op := rng.Intn(10); {
					case op < 3:
						old, replaced := ht.Upsert(TestData{S: key, N: n})
						if replaced != present {
							t.Fatalf("step %d: Upsert(%q) replaced=%v, model present=%v", step, key, replaced, present)
						}
						if present && (old.S != key || old.N != want) {
							t.Fatalf("step %d: Upsert(%q) old=%v, want N=%d", step, key, old, want)
						}
						if !present && old != (TestData{}) {
							t.Fatalf("step %d: Upsert(%q) insert returned non-zero old %v", step, key, old)
						}
						model[key] = n
					case op < 6:
						got, found := ht.GetOrInsert(TestData{S: key, N: n})
						if found != present {
							t.Fatalf("step %d: GetOrInsert(%q) found=%v, model present=%v", step, key, found, present)
						}
						if present && got.N != want {
							t.Fatalf("step %d: GetOrInsert(%q) returned N=%d, stored N=%d", step, key, got.N, want)
						}
						if !present {
							if got.S != key || got.N != n {
								t.Fatalf("step %d: GetOrInsert(%q) miss returned %v, want the inserted item", step, key, got)
							}
							model[key] = n
						}
					case op < 9:
						got, found := ht.Remove(TestData{S: key})
						if found != present {
							t.Fatalf("step %d: Remove(%q) found=%v, model present=%v", step, key, found, present)
						}
						if present && (got.S != key || got.N != want) {
							t.Fatalf("step %d: Remove(%q) returned %v, want N=%d", step, key, got, want)
						}
						if !present && got != (TestData{}) {
							t.Fatalf("step %d: Remove(%q) miss returned non-zero %v", step, key, got)
						}
						delete(model, key)
						if present {
							// every membership must survive the backward shift
							verifyAgainstModel(t, ht, model, shape.universe, step)
						}
					default: // interleave the old API on the same table
						if rng.Intn(2) == 0 {
							if added := ht.Insert(TestData{S: key, N: n}); added == present {
								t.Fatalf("step %d: Insert(%q) added=%v, model present=%v", step, key, added, present)
							}
							model[key] = n
						} else {
							if found := ht.Delete(TestData{S: key}); found != present {
								t.Fatalf("step %d: Delete(%q) found=%v, model present=%v", step, key, found, present)
							}
							delete(model, key)
						}
					}
					if step%97 == 0 {
						verifyAgainstModel(t, ht, model, shape.universe, step)
					}
				}
				verifyAgainstModel(t, ht, model, shape.universe, -1)
				// drain with Remove, verifying after every removal
				for len(model) > 0 {
					for key, want := range model {
						got, found := ht.Remove(TestData{S: key})
						if !found || got.N != want {
							t.Fatalf("drain: Remove(%q) = %v, %v; want N=%d", key, got, found, want)
						}
						delete(model, key)
						verifyAgainstModel(t, ht, model, shape.universe, -2)
						break
					}
				}
			})
		}
	}
}

// TestUpsertGrowth fills a table through Upsert and GetOrInsert across
// several doublings, then replaces every element through Upsert.
func TestUpsertGrowth(t *testing.T) {
	ht := newTestTab(8, 0)
	const n = 2000
	for i := range n {
		key := fmt.Sprintf("g%05d", i)
		if i%2 == 0 {
			if old, replaced := ht.Upsert(TestData{S: key, N: i}); replaced || old != (TestData{}) {
				t.Fatalf("Upsert(%q) on a fresh key: old=%v replaced=%v", key, old, replaced)
			}
		} else {
			if got, found := ht.GetOrInsert(TestData{S: key, N: i}); found || got.N != i {
				t.Fatalf("GetOrInsert(%q) on a fresh key: got=%v found=%v", key, got, found)
			}
		}
	}
	if ht.Len() != n {
		t.Fatalf("Len() = %d, want %d", ht.Len(), n)
	}
	if ht.size < 2*n || float64(ht.Len())/float64(ht.size) > 0.5 {
		t.Fatalf("table did not grow under the saturation rule: size %d for %d elements", ht.size, n)
	}
	checkInvariants(t, ht)
	for i := range n {
		key := fmt.Sprintf("g%05d", i)
		old, replaced := ht.Upsert(TestData{S: key, N: -i})
		if !replaced || old.N != i {
			t.Fatalf("Upsert(%q) replace: old=%v replaced=%v, want N=%d", key, old, replaced, i)
		}
		if got, found := ht.GetOrInsert(TestData{S: key, N: 99}); !found || got.N != -i {
			t.Fatalf("GetOrInsert(%q) on a present key: got=%v found=%v, want N=%d (never overwritten)", key, got, found, -i)
		}
	}
	if ht.Len() != n {
		t.Fatalf("Len() = %d after replacing, want %d", ht.Len(), n)
	}
	checkInvariants(t, ht)
}

// TestGetOrInsertNeverReplaces pins the difference from Upsert/Insert.
func TestGetOrInsertNeverReplaces(t *testing.T) {
	ht := newTestTab(8, 0)
	ht.Insert(TestData{S: "a", N: 1})
	got, found := ht.GetOrInsert(TestData{S: "a", N: 2})
	if !found || got != (TestData{S: "a", N: 1}) {
		t.Fatalf("GetOrInsert on a present key = %v, %v; want {a 1}, true", got, found)
	}
	if v, _ := ht.Search(TestData{S: "a"}); v.N != 1 {
		t.Fatalf("GetOrInsert overwrote the stored element: N = %d", v.N)
	}
	old, replaced := ht.Upsert(TestData{S: "a", N: 3})
	if !replaced || old.N != 1 {
		t.Fatalf("Upsert on a present key = %v, %v; want N=1, true", old, replaced)
	}
	if v, _ := ht.Search(TestData{S: "a"}); v.N != 3 {
		t.Fatalf("Upsert did not replace: N = %d", v.N)
	}
}

// TestSingleProbeHashesOnce counts hash-function calls: each new method
// hashes its argument exactly once when no growth happens.
func TestSingleProbeHashesOnce(t *testing.T) {
	calls := 0
	ht := NewHashTabFunc(
		func(a, b TestData) bool { return a.S == b.S },
		func(v TestData) uint64 {
			calls++
			h := fnv.New64a()
			_, _ = h.Write([]byte(v.S))
			return h.Sum64()
		},
		1024, 0,
	)
	for i := range 100 {
		ht.Insert(TestData{S: fmt.Sprintf("h%03d", i)})
	}
	for _, tc := range []struct {
		name string
		fx   func()
	}{
		{"Upsert replace", func() { ht.Upsert(TestData{S: "h001", N: 5}) }},
		{"Upsert insert", func() { ht.Upsert(TestData{S: "new1"}) }},
		{"GetOrInsert hit", func() { ht.GetOrInsert(TestData{S: "h002"}) }},
		{"GetOrInsert miss", func() { ht.GetOrInsert(TestData{S: "new2"}) }},
		{"Remove hit", func() { ht.Remove(TestData{S: "h003"}) }},
		{"Remove miss", func() { ht.Remove(TestData{S: "absent"}) }},
	} {
		calls = 0
		tc.fx()
		if calls != 1 {
			t.Errorf("%s: hash called %d times, want 1", tc.name, calls)
		}
	}
	checkInvariants(t, ht)
}

// TestUpsertFullTableForcesGrowth drives Upsert and GetOrInsert into a
// completely full table (saturation >= 1 defers growth): the insert must
// force a growth instead of being dropped.
func TestUpsertFullTableForcesGrowth(t *testing.T) {
	for _, method := range []string{"Upsert", "GetOrInsert"} {
		ht := newTestTab(5, 1.0)
		for i := range 5 {
			ht.Insert(TestData{S: fmt.Sprintf("f%d", i), N: i})
		}
		if ht.size != 5 || ht.Len() != 5 {
			t.Fatalf("%s: setup expected a full size-5 table, got size %d len %d", method, ht.size, ht.Len())
		}
		item := TestData{S: "extra", N: 42}
		if method == "Upsert" {
			if _, replaced := ht.Upsert(item); replaced {
				t.Fatalf("Upsert into a full table reported a replace")
			}
		} else {
			if _, found := ht.GetOrInsert(item); found {
				t.Fatalf("GetOrInsert into a full table reported found")
			}
		}
		if ht.size <= 5 || ht.Len() != 6 {
			t.Fatalf("%s: full table should have grown and stored the item: size %d len %d", method, ht.size, ht.Len())
		}
		if v, found := ht.Search(item); !found || v.N != 42 {
			t.Fatalf("%s: item not found after forced growth", method)
		}
		checkInvariants(t, ht)
	}
}

// TestUpsertNilAndZeroValue pins the panic contract of the new writers
// and the tolerant Remove.
func TestUpsertNilAndZeroValue(t *testing.T) {
	var nilTab *HashTab[TestData]
	zero := &HashTab[TestData]{}
	expectPanicMessage(t, "Upsert on nil", "Upsert called on a nil table", func() { nilTab.Upsert(TestData{S: "x"}) })
	expectPanicMessage(t, "Upsert on nil names the fix", "NewHashTab or NewHashTabFunc", func() { nilTab.Upsert(TestData{S: "x"}) })
	expectPanicMessage(t, "GetOrInsert on nil", "GetOrInsert called on a nil table", func() { nilTab.GetOrInsert(TestData{S: "x"}) })
	expectPanicMessage(t, "Upsert on zero value", "Upsert called on a table with no equality/hash functions (create the table with NewHashTab or NewHashTabFunc)", func() { zero.Upsert(TestData{S: "x"}) })
	expectPanicMessage(t, "GetOrInsert on zero value", "GetOrInsert called on a table with no equality/hash functions (create the table with NewHashTab or NewHashTabFunc)", func() { zero.GetOrInsert(TestData{S: "x"}) })
	for name, ht := range map[string]*HashTab[TestData]{"nil": nilTab, "zero value": zero} {
		if got, found := ht.Remove(TestData{S: "x"}); found || got != (TestData{}) {
			t.Errorf("%s: Remove should report not-found with a zero value, got %v %v", name, got, found)
		}
	}
	// An emptied table still answers Remove with not-found.
	ht := newTestTab(8, 0)
	ht.Upsert(TestData{S: "x", N: 1})
	if got, found := ht.Remove(TestData{S: "x"}); !found || got.N != 1 {
		t.Fatalf("Remove(x) = %v, %v", got, found)
	}
	if _, found := ht.Remove(TestData{S: "x"}); found {
		t.Fatalf("second Remove(x) should report not-found")
	}
}

// TestUpsertScanGenerationOnGrowth checks that a growth triggered by
// Upsert/GetOrInsert bumps the Scan generation exactly as Insert's does.
func TestUpsertScanGenerationOnGrowth(t *testing.T) {
	ht := newTestTab(8, 0)
	gen := ht.generation
	for i := 0; ht.size == 8; i++ {
		ht.Upsert(TestData{S: fmt.Sprintf("s%d", i)})
	}
	if ht.generation == gen {
		t.Fatalf("growth through Upsert did not bump the generation")
	}
	gen = ht.generation
	size := ht.size
	for i := 0; ht.size == size; i++ {
		ht.GetOrInsert(TestData{S: fmt.Sprintf("t%d", i)})
	}
	if ht.generation == gen {
		t.Fatalf("growth through GetOrInsert did not bump the generation")
	}
	checkInvariants(t, ht)
}

// TestSingleProbeStoredHashPrefilter keys sharing one home bucket with
// distinct hashes: each new method must call eq only on the slot whose
// stored hash matches (once per hit, never on a miss).
func TestSingleProbeStoredHashPrefilter(t *testing.T) {
	var eqCalls int
	ht := NewHashTabFunc(func(a, b int) bool { eqCalls++; return a == b },
		func(v int) uint64 { return uint64(v) }, 16, 0.9)
	for _, key := range []int{3, 19, 35, 51} {
		ht.Insert(key)
	}
	for _, tc := range []struct {
		name string
		want int
		fx   func()
	}{
		{"Upsert hit at chain end", 1, func() { ht.Upsert(51) }},
		{"GetOrInsert hit at chain end", 1, func() { ht.GetOrInsert(51) }},
		{"Upsert miss", 0, func() { ht.Upsert(67) }},
		{"GetOrInsert miss", 0, func() { ht.GetOrInsert(83) }},
		{"Remove hit at chain end", 1, func() { ht.Remove(83) }},
		{"Remove miss", 0, func() { ht.Remove(99) }},
	} {
		eqCalls = 0
		tc.fx()
		if eqCalls != tc.want {
			t.Errorf("%s: %d equality calls, want %d (stored-hash check not applied)", tc.name, eqCalls, tc.want)
		}
	}
	for _, key := range []int{3, 19, 35, 51, 67} {
		if _, found := ht.Search(key); !found {
			t.Fatalf("lost %d", key)
		}
	}
}
