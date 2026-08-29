package fenwick_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/pschlump/pluto/g_lib"
)

// -------------------------------------------------------------------------------------------------------
// Invariants
// -------------------------------------------------------------------------------------------------------

// checkInvariants verifies the defining invariant of the 1-based
// internal array: for every k in 1..n, tree[k] holds the sum of the
// lowbit(k) values ending at slot k-1.  It reads the internals without
// the lock — single-goroutine tests only.
func checkInvariants[T g_lib.Numeric](t *testing.T, ft *FenwickTree[T]) {
	t.Helper()
	n := ft.Len()
	if len(ft.tree) != n+1 {
		t.Fatalf("internal tree has %d slots, expected %d", len(ft.tree), n+1)
	}
	var zero T
	if ft.tree[0] != zero {
		t.Fatalf("tree[0] is %v, expected the unused slot to stay zero", ft.tree[0])
	}
	for k := 1; k <= n; k++ {
		lo := k - (k & -k) // tree[k] covers slots lo..k-1 (0-based)
		want := ft.Sum(k-1) - ft.Sum(lo-1)
		if ft.tree[k] != want {
			t.Fatalf("tree[%d]=%v, expected %v (sum of slots %d..%d)", k, ft.tree[k], want, lo, k-1)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against a reference model (fixed seed)
// -------------------------------------------------------------------------------------------------------

// TestFenwickRandomizedModel cross-checks the tree against a naive
// reference slice with brute-force range sums — the same property as in
// the plain package, exercised through the locked API.
func TestFenwickRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const n = 64
	ft := NewFenwickTree[int](n)
	model := make([]int, n)

	naiveRange := func(lo, hi int) int {
		s := 0
		for i := lo; i <= hi; i++ {
			s += model[i]
		}
		return s
	}

	verify := func(step int) {
		for i := 0; i < n; i++ {
			if v, ok := ft.Value(i); !ok || v != model[i] {
				t.Fatalf("step %d: Value(%d)=(%d,%v), model has %d", step, i, v, ok, model[i])
			}
			if s := ft.Sum(i); s != naiveRange(0, i) {
				t.Fatalf("step %d: Sum(%d)=%d, model has %d", step, i, s, naiveRange(0, i))
			}
		}
		checkInvariants(t, ft)
	}

	for step := range 3000 {
		i := rng.Intn(n)
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Add
			delta := rng.Intn(201) - 100
			if !ft.Add(i, delta) {
				t.Fatalf("step %d: Add(%d, %d) returned false for an in-range slot", step, i, delta)
			}
			model[i] += delta
		case 4, 5: // Set
			v := rng.Intn(201) - 100
			if !ft.Set(i, v) {
				t.Fatalf("step %d: Set(%d, %d) returned false for an in-range slot", step, i, v)
			}
			model[i] = v
		case 6, 7: // RangeSum
			lo := rng.Intn(n)
			hi := lo + rng.Intn(n-lo)
			if s, ok := ft.RangeSum(lo, hi); !ok || s != naiveRange(lo, hi) {
				t.Fatalf("step %d: RangeSum(%d,%d)=(%d,%v), model has %d",
					step, lo, hi, s, ok, naiveRange(lo, hi))
			}
		case 8: // Sum
			if s := ft.Sum(i); s != naiveRange(0, i) {
				t.Fatalf("step %d: Sum(%d)=%d, model has %d", step, i, s, naiveRange(0, i))
			}
		case 9: // Value
			if v, ok := ft.Value(i); !ok || v != model[i] {
				t.Fatalf("step %d: Value(%d)=(%d,%v), model has %d", step, i, v, ok, model[i])
			}
		}
		if step%251 == 0 {
			verify(step)
		}
	}
	verify(3000)
}

// -------------------------------------------------------------------------------------------------------
// Concurrency (run with -race)
// -------------------------------------------------------------------------------------------------------

// TestConcurrentFenwick runs writer and reader goroutines against one
// shared tree.  It is primarily a test for the race detector
// (`make race`): each writer owns a disjoint band of slots (the
// internal tree-array updates of different bands still overlap, so a
// missing lock would corrupt the sums), and the final accounting is
// deterministic.
func TestConcurrentFenwick(t *testing.T) {
	const n = 512
	ft := NewFenwickTree[int](n)

	stop := make(chan struct{})
	var readersWG sync.WaitGroup

	// Readers hammer the query operations until the writers finish.
	for range 4 {
		readersWG.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				i := rand.Intn(n)
				_ = ft.Sum(i)
				_, _ = ft.RangeSum(0, i)
				_, _ = ft.Value(i)
				_ = ft.Len()
				_ = ft.IsEmpty()
			}
		})
	}

	// Writers: each goroutine Adds to and Sets its own disjoint band of
	// slots (g*band .. g*band+band-1), so the final value of every slot
	// is deterministic.
	var writersWG sync.WaitGroup
	const writers = 8
	const band = n / writers
	for g := range writers {
		writersWG.Go(func() {
			base := g * band
			for i := 0; i < band; i++ {
				ft.Add(base+i, 1)
				ft.Set(base+i, 2)
				ft.Add(base+i, 3) // final value: 5 per slot
			}
		})
	}

	writersWG.Wait()
	close(stop)
	readersWG.Wait()

	// Every slot must hold exactly 5, so Sum(n-1) == 5n.
	for i := 0; i < n; i++ {
		if v, ok := ft.Value(i); !ok || v != 5 {
			t.Fatalf("expected Value(%d)=5 after concurrent writes, got (%d, %v)", i, v, ok)
		}
	}
	if s := ft.Sum(n - 1); s != 5*n {
		t.Fatalf("expected Sum(%d)=%d after concurrent writes, got %d", n-1, 5*n, s)
	}
	checkInvariants(t, ft)
}

// TestConcurrentCompound exercises Lock + Nl* read-modify-write
// sequences from multiple goroutines against one shared tree.  Each
// goroutine increments its own slot 100 times as a locked
// NlValue/NlSet pair — without the lock the pair would not be atomic,
// and the shared internal array would race under -race.
func TestConcurrentCompound(t *testing.T) {
	const writers = 8
	ft := NewFenwickTree[int](writers)

	var wg sync.WaitGroup
	for g := range writers {
		wg.Go(func() {
			for range 100 {
				ft.Lock()
				v, ok := ft.NlValue(g)
				if ok {
					ft.NlSet(g, v+1)
				}
				ft.Unlock()
			}
		})
	}
	wg.Wait()

	for g := 0; g < writers; g++ {
		if v, ok := ft.Value(g); !ok || v != 100 {
			t.Fatalf("expected Value(%d)=100 after 100 locked increments, got (%d, %v)", g, v, ok)
		}
	}
	if s := ft.Sum(writers - 1); s != 100*writers {
		t.Errorf("expected Sum(%d)=%d, got %d", writers-1, 100*writers, s)
	}
	checkInvariants(t, ft)
}
