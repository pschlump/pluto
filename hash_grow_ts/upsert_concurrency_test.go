/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_grow_ts

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pschlump/pluto/hash_grow"
)

// Twin-specific tests for Upsert/GetOrInsert/Remove: the Nl variants
// under a held Lock, step-for-step parity with the plain package, and
// concurrent use (run with -race).

func fnvTestHash(v TestData) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(v.S))
	return h.Sum64()
}

// TestUpsertNlCompound uses the Nl variants inside one Lock hold: a
// check-then-act sequence no other goroutine can interleave with.
func TestUpsertNlCompound(t *testing.T) {
	ht := newTestTab(16, 0)
	ht.Lock()
	for i := range 50 {
		if old, replaced := ht.NlUpsert(TestData{S: fmt.Sprintf("n%03d", i), N: i}); replaced || old != (TestData{}) {
			t.Fatalf("NlUpsert fresh key: %v %v", old, replaced)
		}
	}
	if old, replaced := ht.NlUpsert(TestData{S: "n007", N: 700}); !replaced || old.N != 7 {
		t.Fatalf("NlUpsert replace: %v %v", old, replaced)
	}
	if got, found := ht.NlGetOrInsert(TestData{S: "n007", N: 1}); !found || got.N != 700 {
		t.Fatalf("NlGetOrInsert hit: %v %v", got, found)
	}
	if got, found := ht.NlGetOrInsert(TestData{S: "n999", N: 999}); found || got.N != 999 {
		t.Fatalf("NlGetOrInsert miss: %v %v", got, found)
	}
	if got, found := ht.NlRemove(TestData{S: "n010"}); !found || got.N != 10 {
		t.Fatalf("NlRemove hit: %v %v", got, found)
	}
	if _, found := ht.NlRemove(TestData{S: "n010"}); found {
		t.Fatalf("NlRemove of a removed key reported found")
	}
	if ht.NlLen() != 50 {
		t.Fatalf("NlLen = %d, want 50", ht.NlLen())
	}
	ht.Unlock()
	checkInvariants(t, ht)

	zero := &HashTab[TestData]{}
	expectPanicMessage(t, "NlUpsert on zero value", "Upsert called on a table with no equality/hash functions", func() { zero.NlUpsert(TestData{S: "x"}) })
	expectPanicMessage(t, "NlGetOrInsert on zero value", "GetOrInsert called on a table with no equality/hash functions", func() { zero.NlGetOrInsert(TestData{S: "x"}) })
	if _, found := zero.NlRemove(TestData{S: "x"}); found {
		t.Fatalf("NlRemove on a zero value reported found")
	}
	// The zero-value panic must release the write lock (deferred Unlock).
	expectPanic(t, "Upsert on zero value", func() { zero.Upsert(TestData{S: "x"}) })
	zero.Lock()
	zero.Unlock()
}

// TestUpsertParityWithPlain runs one fixed-seed operation stream against
// the plain package and the twin with the same deterministic hash; every
// result and the final slot order must match exactly.
func TestUpsertParityWithPlain(t *testing.T) {
	eq := func(a, b TestData) bool { return a.S == b.S }
	plain := hash_grow.NewHashTabFunc(eq, fnvTestHash, 8, 0)
	twin := NewHashTabFunc(eq, fnvTestHash, 8, 0)
	rng := rand.New(rand.NewSource(42))
	for step := range 5000 {
		item := TestData{S: fmt.Sprintf("p%03d", rng.Intn(300)), N: rng.Intn(1000)}
		switch rng.Intn(3) {
		case 0:
			o1, r1 := plain.Upsert(item)
			o2, r2 := twin.Upsert(item)
			if o1 != o2 || r1 != r2 {
				t.Fatalf("step %d Upsert: plain %v %v, twin %v %v", step, o1, r1, o2, r2)
			}
		case 1:
			g1, f1 := plain.GetOrInsert(item)
			g2, f2 := twin.GetOrInsert(item)
			if g1 != g2 || f1 != f2 {
				t.Fatalf("step %d GetOrInsert: plain %v %v, twin %v %v", step, g1, f1, g2, f2)
			}
		default:
			g1, f1 := plain.Remove(item)
			g2, f2 := twin.Remove(item)
			if g1 != g2 || f1 != f2 {
				t.Fatalf("step %d Remove: plain %v %v, twin %v %v", step, g1, f1, g2, f2)
			}
		}
	}
	s1, s2 := plain.Slice(), twin.Slice()
	if len(s1) != len(s2) {
		t.Fatalf("Slice lengths differ: %d vs %d", len(s1), len(s2))
	}
	for i := range s1 {
		if s1[i] != s2[i] {
			t.Fatalf("slot order differs at %d: %v vs %v", i, s1[i], s2[i])
		}
	}
	checkInvariants(t, twin)
}

// TestConcurrentUpsertGetOrInsertRemove hammers overlapping keys from
// several goroutines.  GetOrInsert's atomicity is checked exactly: for
// each key, exactly one goroutine sees found == false.
func TestConcurrentUpsertGetOrInsertRemove(t *testing.T) {
	ht := newTestTab(16, 0)
	const workers = 8
	const keys = 300

	var firsts [keys]atomic.Int32
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for k := range keys {
				if _, found := ht.GetOrInsert(TestData{S: fmt.Sprintf("g%03d", k), N: w}); !found {
					firsts[k].Add(1)
				}
			}
		}(w)
	}
	wg.Wait()
	for k := range keys {
		if n := firsts[k].Load(); n != 1 {
			t.Fatalf("key g%03d: %d goroutines saw GetOrInsert insert, want exactly 1", k, n)
		}
	}

	// Upsert/Remove churn on a shared key space plus readers; each
	// Remove's returned element must be one some worker stored.
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(w)))
			for range 2000 {
				key := fmt.Sprintf("c%02d", rng.Intn(40))
				switch rng.Intn(4) {
				case 0, 1:
					ht.Upsert(TestData{S: key, N: w})
				case 2:
					if got, found := ht.Remove(TestData{S: key}); found && (got.S != key || got.N < 0 || got.N >= workers) {
						t.Errorf("Remove(%q) returned a foreign element %v", key, got)
					}
				default:
					ht.Search(TestData{S: key})
					for range ht.Values() {
					}
				}
			}
		}(w)
	}
	wg.Wait()
	checkInvariants(t, ht)
	for k := range keys {
		if _, found := ht.Search(TestData{S: fmt.Sprintf("g%03d", k)}); !found {
			t.Fatalf("g%03d lost during churn on a disjoint key space", k)
		}
	}
}
