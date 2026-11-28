/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_grow_ts

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/pschlump/pluto/hash_grow"
)

// Twin-specific UpsertFunc tests: NlUpsertFunc under a held Lock,
// step-for-step parity with the plain package, the lock released after a
// panicking fn, and atomic concurrent read-modify-writes (run with -race).

func TestUpsertFuncNlCompound(t *testing.T) {
	ht := newTestTab(16, 0)
	bump := func(e TestData, found bool) TestData {
		if found {
			e.N++
		}
		return e
	}
	ht.Lock()
	for i := range 50 {
		if got := ht.NlUpsertFunc(TestData{S: fmt.Sprintf("n%03d", i), N: i}, bump); got.N != i {
			t.Fatalf("NlUpsertFunc insert returned %v", got)
		}
	}
	if got := ht.NlUpsertFunc(TestData{S: "n007"}, bump); got.N != 8 {
		t.Fatalf("NlUpsertFunc hit returned %v, want N=8", got)
	}
	if v, _ := ht.NlSearch(TestData{S: "n007"}); v.N != 8 {
		t.Fatalf("NlUpsertFunc hit not stored: %v", v)
	}
	if ht.NlLen() != 50 {
		t.Fatalf("NlLen = %d, want 50", ht.NlLen())
	}
	ht.Unlock()
	checkInvariants(t, ht)

	zero := &HashTab[TestData]{}
	expectPanicMessage(t, "NlUpsertFunc on zero value", "UpsertFunc called on a table with no equality/hash functions", func() { zero.NlUpsertFunc(TestData{S: "x"}, bump) })
	expectPanicMessage(t, "NlUpsertFunc with nil fn", "UpsertFunc called with a nil update function", func() { ht.NlUpsertFunc(TestData{S: "x"}, nil) })
	// The zero-value panic must release the write lock (deferred Unlock).
	expectPanic(t, "UpsertFunc on zero value", func() { zero.UpsertFunc(TestData{S: "x"}, bump) })
	zero.Lock()
	zero.Unlock()
}

// TestUpsertFuncPanicReleasesLock: a panicking fn must leave the table
// unchanged and unlocked.
func TestUpsertFuncPanicReleasesLock(t *testing.T) {
	ht := newTestTab(16, 0)
	ht.Insert(TestData{S: "a", N: 1})
	for _, key := range []string{"a", "absent"} {
		expectPanicMessage(t, "UpsertFunc panicking fn", "boom", func() {
			ht.UpsertFunc(TestData{S: key}, func(TestData, bool) TestData { panic("boom") })
		})
		ht.Lock() // would deadlock if the panic left the write lock held
		ht.Unlock()
		if ht.Len() != 1 {
			t.Fatalf("panicking fn changed Len to %d", ht.Len())
		}
		if v, _ := ht.Search(TestData{S: "a"}); v.N != 1 {
			t.Fatalf("panicking fn changed the stored element: %v", v)
		}
		checkInvariants(t, ht)
	}
}

// TestUpsertFuncParityWithPlain runs one fixed-seed stream against both
// packages with the same deterministic hash: every result and the final
// slot order must match.
func TestUpsertFuncParityWithPlain(t *testing.T) {
	eq := func(a, b TestData) bool { return a.S == b.S }
	plain := hash_grow.NewHashTabFunc(eq, fnvTestHash, 8, 0)
	twin := NewHashTabFunc(eq, fnvTestHash, 8, 0)
	rng := rand.New(rand.NewSource(42))
	for step := range 5000 {
		key := fmt.Sprintf("p%03d", rng.Intn(300))
		d := rng.Intn(1000)
		if rng.Intn(4) == 0 {
			g1, f1 := plain.Remove(TestData{S: key})
			g2, f2 := twin.Remove(TestData{S: key})
			if g1 != g2 || f1 != f2 {
				t.Fatalf("step %d Remove: plain %v %v, twin %v %v", step, g1, f1, g2, f2)
			}
			continue
		}
		var f1, f2 bool
		fn := func(found *bool) func(TestData, bool) TestData {
			return func(e TestData, found2 bool) TestData {
				*found = found2
				e.N = e.N*3 + d
				return e
			}
		}
		g1 := plain.UpsertFunc(TestData{S: key, N: d}, fn(&f1))
		g2 := twin.UpsertFunc(TestData{S: key, N: d}, fn(&f2))
		if g1 != g2 || f1 != f2 {
			t.Fatalf("step %d UpsertFunc: plain %v %v, twin %v %v", step, g1, f1, g2, f2)
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

// TestConcurrentUpsertFunc: concurrent increments of shared counters
// through UpsertFunc lose no update (the read-modify-write is atomic
// under the write lock), racing readers and removers of other keys.
func TestConcurrentUpsertFunc(t *testing.T) {
	ht := newTestTab(16, 0)
	const workers = 8
	const perWorker = 2000
	const keys = 50
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(w)))
			for range perWorker {
				ht.UpsertFunc(TestData{S: fmt.Sprintf("c%02d", rng.Intn(keys))}, func(e TestData, found bool) TestData {
					if !found {
						e.N = 0
					}
					e.N++
					return e
				})
				switch rng.Intn(4) {
				case 0:
					ht.Upsert(TestData{S: fmt.Sprintf("x%02d", rng.Intn(20)), N: w})
				case 1:
					ht.Remove(TestData{S: fmt.Sprintf("x%02d", rng.Intn(20))})
				default:
					ht.Search(TestData{S: fmt.Sprintf("c%02d", rng.Intn(keys))})
				}
			}
		}(w)
	}
	wg.Wait()
	total := 0
	for k := range keys {
		v, _ := ht.Search(TestData{S: fmt.Sprintf("c%02d", k)})
		total += v.N
	}
	if total != workers*perWorker {
		t.Fatalf("sum of counters = %d, want %d (lost read-modify-write updates)", total, workers*perWorker)
	}
	checkInvariants(t, ht)
}
