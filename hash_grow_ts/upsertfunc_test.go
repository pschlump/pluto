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

// Tests for UpsertFunc, the single-probe read-modify-write.  The model
// test reuses upsert_test.go's hash shapes and size/saturation configs;
// every UpsertFunc call checks that fn ran exactly once, with the stored
// element on a hit and with item on a miss, and that the returned value
// is what got stored.

// fnCall records one invocation of an UpsertFunc callback.
type fnCall struct {
	existing TestData
	found    bool
}

func TestUpsertFuncModel(t *testing.T) {
	for _, shape := range upsertHashShapes {
		for _, cfg := range []struct {
			n          int
			saturation float64
		}{
			{8, 0},
			{5, 1.0},
			{7, 0.9},
		} {
			t.Run(fmt.Sprintf("%s/n=%d/sat=%v", shape.name, cfg.n, cfg.saturation), func(t *testing.T) {
				rng := rand.New(rand.NewSource(42))
				ht := newShapeTab(shape.hash, cfg.n, cfg.saturation)
				model := map[string]int{}
				for step := range 3000 {
					key := fmt.Sprintf("k%04d", rng.Intn(shape.universe))
					delta := rng.Intn(1000) + 1
					want, present := model[key]
					switch op := rng.Intn(10); {
					case op < 6:
						var calls []fnCall
						item := TestData{S: key, N: -delta} // N of item is visible only on a miss
						got := ht.UpsertFunc(item, func(existing TestData, found bool) TestData {
							calls = append(calls, fnCall{existing, found})
							return TestData{S: existing.S, N: existing.N + delta}
						})
						if len(calls) != 1 {
							t.Fatalf("step %d: fn called %d times, want 1", step, len(calls))
						}
						c := calls[0]
						if c.found != present {
							t.Fatalf("step %d: UpsertFunc(%q) found=%v, model present=%v", step, key, c.found, present)
						}
						if present && (c.existing.S != key || c.existing.N != want) {
							t.Fatalf("step %d: hit passed existing=%v, want the stored {%s %d}", step, c.existing, key, want)
						}
						if !present && c.existing != item {
							t.Fatalf("step %d: miss passed existing=%v, want item %v", step, c.existing, item)
						}
						newN := want + delta
						if !present {
							newN = -delta + delta
						}
						if got.S != key || got.N != newN {
							t.Fatalf("step %d: UpsertFunc returned %v, want {%s %d}", step, got, key, newN)
						}
						model[key] = newN
					case op < 8:
						got, found := ht.Remove(TestData{S: key})
						if found != present || (present && got.N != want) {
							t.Fatalf("step %d: Remove(%q) = %v, %v; model %d, %v", step, key, got, found, want, present)
						}
						delete(model, key)
					default:
						ht.Insert(TestData{S: key, N: delta})
						model[key] = delta
					}
					if step%97 == 0 {
						verifyAgainstModel(t, ht, model, shape.universe, step)
					}
				}
				verifyAgainstModel(t, ht, model, shape.universe, -1)
			})
		}
	}
}

// TestUpsertFuncGrowth fills a table through UpsertFunc's insert path
// across several doublings, then bumps every element through its hit
// path.
func TestUpsertFuncGrowth(t *testing.T) {
	ht := newTestTab(8, 0)
	const n = 2000
	gen := ht.generation
	for i := range n {
		key := fmt.Sprintf("g%05d", i)
		got := ht.UpsertFunc(TestData{S: key, N: i}, func(existing TestData, found bool) TestData {
			if found {
				t.Fatalf("UpsertFunc(%q): fresh key reported found", key)
			}
			return existing
		})
		if got.N != i {
			t.Fatalf("UpsertFunc(%q) insert returned %v", key, got)
		}
		if float64(ht.Len())/float64(ht.size) > 0.5 {
			t.Fatalf("after %d inserts load factor %d/%d passed the saturation threshold (growth skipped)", i+1, ht.Len(), ht.size)
		}
	}
	if ht.Len() != n || ht.size < 2*n {
		t.Fatalf("Len %d size %d after %d inserts", ht.Len(), ht.size, n)
	}
	if ht.generation == gen {
		t.Fatalf("growth through UpsertFunc did not bump the Scan generation")
	}
	checkInvariants(t, ht)
	for i := range n {
		key := fmt.Sprintf("g%05d", i)
		got := ht.UpsertFunc(TestData{S: key}, func(existing TestData, found bool) TestData {
			if !found {
				t.Fatalf("UpsertFunc(%q): present key reported not found", key)
			}
			existing.N *= 10
			return existing
		})
		if got.N != 10*i {
			t.Fatalf("UpsertFunc(%q) hit returned %v, want N=%d", key, got, 10*i)
		}
	}
	if ht.Len() != n {
		t.Fatalf("Len %d after bumping, want %d", ht.Len(), n)
	}
	for i := range n {
		if v, _ := ht.Search(TestData{S: fmt.Sprintf("g%05d", i)}); v.N != 10*i {
			t.Fatalf("bumped value not stored: %v", v)
		}
	}
	checkInvariants(t, ht)
}

// TestUpsertFuncFullTableForcesGrowth: an insert into a completely full
// table (saturation >= 1 defers growth) must grow, not drop the element.
func TestUpsertFuncFullTableForcesGrowth(t *testing.T) {
	ht := newTestTab(5, 1.0)
	for i := range 5 {
		ht.Insert(TestData{S: fmt.Sprintf("f%d", i), N: i})
	}
	if ht.size != 5 || ht.Len() != 5 {
		t.Fatalf("setup expected a full size-5 table, got size %d len %d", ht.size, ht.Len())
	}
	// A hit on a full table must not grow.
	ht.UpsertFunc(TestData{S: "f3"}, func(e TestData, _ bool) TestData { e.N = 33; return e })
	if ht.size != 5 {
		t.Fatalf("a hit on a full table grew it to %d", ht.size)
	}
	got := ht.UpsertFunc(TestData{S: "extra", N: 42}, func(e TestData, found bool) TestData {
		if found {
			t.Fatalf("absent key reported found")
		}
		return e
	})
	if got.N != 42 || ht.size <= 5 || ht.Len() != 6 {
		t.Fatalf("full table should have grown and stored the item: got %v size %d len %d", got, ht.size, ht.Len())
	}
	for key, want := range map[string]int{"extra": 42, "f3": 33, "f0": 0} {
		if v, found := ht.Search(TestData{S: key}); !found || v.N != want {
			t.Fatalf("Search(%q) = %v, %v; want N=%d", key, v, found, want)
		}
	}
	checkInvariants(t, ht)
}

// TestUpsertFuncPanicLeavesTableUnchanged: a panicking fn on the hit
// path, the plain miss path, a miss that would trigger load-factor growth
// and a miss into a completely full table must leave length, size,
// generation and every element exactly as they were.
func TestUpsertFuncPanicLeavesTableUnchanged(t *testing.T) {
	boom := func(TestData, bool) TestData { panic("boom") }
	for _, tc := range []struct {
		name       string
		n          int
		saturation float64
		fill       int
		key        string
	}{
		{"hit", 16, 0, 5, "p2"},
		{"miss", 16, 0, 5, "absent"},
		{"miss at growth threshold", 16, 0, 8, "absent"}, // the 9th element would pass 0.5 and grow
		{"miss into full table", 5, 1.0, 5, "absent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ht := newTestTab(tc.n, tc.saturation)
			for i := range tc.fill {
				ht.Insert(TestData{S: fmt.Sprintf("p%d", i), N: i})
			}
			before := ht.Slice()
			size, length, gen := ht.size, ht.Len(), ht.generation
			expectPanicMessage(t, "UpsertFunc with a panicking fn", "boom", func() {
				ht.UpsertFunc(TestData{S: tc.key, N: 99}, boom)
			})
			if ht.size != size || ht.Len() != length || ht.generation != gen {
				t.Fatalf("panicking fn changed the table: size %d->%d len %d->%d gen %d->%d", size, ht.size, length, ht.Len(), gen, ht.generation)
			}
			after := ht.Slice()
			if len(after) != len(before) {
				t.Fatalf("slot contents changed: %v -> %v", before, after)
			}
			for i := range before {
				if before[i] != after[i] {
					t.Fatalf("slot %d changed: %v -> %v", i, before[i], after[i])
				}
			}
			checkInvariants(t, ht)
			// The table is still fully usable.
			ht.UpsertFunc(TestData{S: tc.key, N: 7}, func(e TestData, _ bool) TestData { return e })
			if _, found := ht.Search(TestData{S: tc.key}); !found {
				t.Fatalf("table unusable after a recovered fn panic")
			}
			checkInvariants(t, ht)
		})
	}
}

// TestUpsertFuncHashesOnce counts hash and eq calls: one hash per call,
// and eq only on the slot whose stored hash matches (shared-home keys).
func TestUpsertFuncHashesOnce(t *testing.T) {
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
	keep := func(e TestData, _ bool) TestData { return e }
	for _, key := range []string{"h001", "new1"} {
		calls = 0
		ht.UpsertFunc(TestData{S: key}, keep)
		if calls != 1 {
			t.Errorf("UpsertFunc(%q): hash called %d times, want 1", key, calls)
		}
	}
	checkInvariants(t, ht)

	var eqCalls int
	ih := NewHashTabFunc(func(a, b int) bool { eqCalls++; return a == b },
		func(v int) uint64 { return uint64(v) }, 16, 0.9)
	for _, key := range []int{3, 19, 35, 51} {
		ih.Insert(key)
	}
	same := func(e int, _ bool) int { return e }
	eqCalls = 0
	ih.UpsertFunc(51, same)
	if eqCalls != 1 {
		t.Errorf("UpsertFunc hit at chain end: %d eq calls, want 1", eqCalls)
	}
	eqCalls = 0
	ih.UpsertFunc(67, same)
	if eqCalls != 0 {
		t.Errorf("UpsertFunc miss on a shared-home chain: %d eq calls, want 0", eqCalls)
	}
}

// TestUpsertFuncNilAndZeroValue pins the panic contract.
func TestUpsertFuncNilAndZeroValue(t *testing.T) {
	var nilTab *HashTab[TestData]
	zero := &HashTab[TestData]{}
	keep := func(e TestData, _ bool) TestData { return e }
	expectPanicMessage(t, "UpsertFunc on nil", "UpsertFunc called on a nil table", func() { nilTab.UpsertFunc(TestData{S: "x"}, keep) })
	expectPanicMessage(t, "UpsertFunc on nil names the fix", "NewHashTab or NewHashTabFunc", func() { nilTab.UpsertFunc(TestData{S: "x"}, keep) })
	expectPanicMessage(t, "UpsertFunc on zero value", "UpsertFunc called on a table with no equality/hash functions (create the table with NewHashTab or NewHashTabFunc)", func() { zero.UpsertFunc(TestData{S: "x"}, keep) })
	ht := newTestTab(8, 0)
	ht.Insert(TestData{S: "x", N: 1})
	expectPanicMessage(t, "UpsertFunc with nil fn", "UpsertFunc called with a nil update function", func() { ht.UpsertFunc(TestData{S: "x"}, nil) })
	expectPanicMessage(t, "UpsertFunc with nil fn on a miss", "UpsertFunc called with a nil update function", func() { ht.UpsertFunc(TestData{S: "y"}, nil) })
	if ht.Len() != 1 {
		t.Fatalf("nil-fn panic changed Len to %d", ht.Len())
	}
	checkInvariants(t, ht)
}

// TestUpsertFuncAllocs: the hit path and a non-growing miss allocate
// nothing (fn is a non-escaping closure).
func TestUpsertFuncAllocs(t *testing.T) {
	ht := newTestTab(1024, 0)
	ht.Insert(TestData{S: "hot", N: 0})
	bump := 1
	if a := testing.AllocsPerRun(1000, func() {
		ht.UpsertFunc(TestData{S: "hot"}, func(e TestData, _ bool) TestData { e.N += bump; return e })
	}); a != 0 {
		t.Errorf("UpsertFunc hit: %v allocs/op, want 0", a)
	}
	if a := testing.AllocsPerRun(1000, func() {
		ht.UpsertFunc(TestData{S: "cold"}, func(e TestData, _ bool) TestData { e.N += bump; return e })
		ht.Delete(TestData{S: "cold"})
	}); a != 0 {
		t.Errorf("UpsertFunc miss (+Delete): %v allocs/op, want 0", a)
	}
}
