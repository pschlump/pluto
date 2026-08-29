package segment_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand"
	"sync"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Invariants
// -------------------------------------------------------------------------------------------------------

// checkInvariants verifies the defining invariant of the array-backed
// tree: every internal node holds the combine of its two children,
// every leaf at size+i (i < n) holds Value(i), and every padding leaf
// holds the identity.  It reads the internals without the lock —
// single-goroutine tests only.
func checkInvariants(t *testing.T, st *SegmentTree[int]) {
	t.Helper()
	if len(st.tree) != 2*st.size {
		t.Fatalf("internal tree has %d slots, expected 2*%d", len(st.tree), st.size)
	}
	if st.size < st.n || st.size >= 2*st.n && st.n > 1 {
		t.Fatalf("size=%d is not the smallest power of two >= n=%d", st.size, st.n)
	}
	for k := 1; k < st.size; k++ {
		want := st.combine(st.tree[2*k], st.tree[2*k+1])
		if st.tree[k] != want {
			t.Fatalf("tree[%d]=%d, expected combine of children = %d", k, st.tree[k], want)
		}
	}
	for i := st.n; i < st.size; i++ {
		if st.tree[st.size+i] != st.identity {
			t.Fatalf("padding leaf %d is %d, expected the identity %d",
				i, st.tree[st.size+i], st.identity)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against reference models (fixed seed)
// -------------------------------------------------------------------------------------------------------

// TestSegmentTreeRandomizedModel cross-checks three trees over the
// SAME data — a sum tree (NewSegmentTree), a min tree, and a max tree
// (both via NewSegmentTreeFunc) — against brute-force range loops over
// a naive reference slice, with random interleaved updates and
// queries.  The same property as in the plain package, exercised
// through the locked API.
func TestSegmentTreeRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const n = 64
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	max := func(a, b int) int {
		if a > b {
			return a
		}
		return b
	}
	sum := func(a, b int) int { return a + b }

	model := make([]int, n)
	for i := range model {
		model[i] = rng.Intn(201) - 100
	}
	sumTree := NewSegmentTree(model)
	minTree := NewSegmentTreeFunc(model, min, math.MaxInt)
	maxTree := NewSegmentTreeFunc(model, max, math.MinInt)

	brute := func(combine func(a, b int) int, identity, lo, hi int) int {
		acc := identity
		for i := lo; i <= hi; i++ {
			acc = combine(acc, model[i])
		}
		return acc
	}

	verify := func(step int) {
		for i := 0; i < n; i++ {
			if v, ok := sumTree.Value(i); !ok || v != model[i] {
				t.Fatalf("step %d: sumTree.Value(%d)=(%d,%v), model has %d", step, i, v, ok, model[i])
			}
			if v, ok := minTree.Value(i); !ok || v != model[i] {
				t.Fatalf("step %d: minTree.Value(%d)=(%d,%v), model has %d", step, i, v, ok, model[i])
			}
			if v, ok := maxTree.Value(i); !ok || v != model[i] {
				t.Fatalf("step %d: maxTree.Value(%d)=(%d,%v), model has %d", step, i, v, ok, model[i])
			}
		}
		checkInvariants(t, sumTree)
		checkInvariants(t, minTree)
		checkInvariants(t, maxTree)
	}

	for step := range 3000 {
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Update
			i := rng.Intn(n)
			v := rng.Intn(201) - 100
			if !sumTree.Update(i, v) || !minTree.Update(i, v) || !maxTree.Update(i, v) {
				t.Fatalf("step %d: Update(%d, %d) returned false for an in-range slot", step, i, v)
			}
			model[i] = v
		case 4, 5, 6, 7: // Query all three trees over one random range
			lo := rng.Intn(n)
			hi := lo + rng.Intn(n-lo)
			if s, ok := sumTree.Query(lo, hi); !ok || s != brute(sum, 0, lo, hi) {
				t.Fatalf("step %d: sum Query(%d,%d)=(%d,%v), model has %d",
					step, lo, hi, s, ok, brute(sum, 0, lo, hi))
			}
			if s, ok := minTree.Query(lo, hi); !ok || s != brute(min, math.MaxInt, lo, hi) {
				t.Fatalf("step %d: min Query(%d,%d)=(%d,%v), model has %d",
					step, lo, hi, s, ok, brute(min, math.MaxInt, lo, hi))
			}
			if s, ok := maxTree.Query(lo, hi); !ok || s != brute(max, math.MinInt, lo, hi) {
				t.Fatalf("step %d: max Query(%d,%d)=(%d,%v), model has %d",
					step, lo, hi, s, ok, brute(max, math.MinInt, lo, hi))
			}
		case 8: // single-slot query
			i := rng.Intn(n)
			if s, ok := sumTree.Query(i, i); !ok || s != model[i] {
				t.Fatalf("step %d: sum Query(%d,%d)=(%d,%v), model has %d", step, i, i, s, ok, model[i])
			}
		case 9: // full-range query
			if s, ok := sumTree.Query(0, n-1); !ok || s != brute(sum, 0, 0, n-1) {
				t.Fatalf("step %d: sum full Query=(%d,%v), model has %d",
					step, s, ok, brute(sum, 0, 0, n-1))
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

// TestConcurrentSegmentTree runs writer and reader goroutines against
// one shared tree.  It is primarily a test for the race detector
// (`make race`): each writer owns a disjoint band of slots (the
// internal node updates of different bands still overlap near the
// root, so a missing lock would corrupt the queries), and the final
// accounting is deterministic.
func TestConcurrentSegmentTree(t *testing.T) {
	const n = 512
	data := make([]int, n)
	st := NewSegmentTree(data)

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
				_, _ = st.Query(0, i)
				_, _ = st.Value(i)
				_ = st.Len()
				_ = st.IsEmpty()
			}
		})
	}

	// Writers: each goroutine Updates its own disjoint band of slots
	// (g*band .. g*band+band-1), so the final value of every slot is
	// deterministic.
	var writersWG sync.WaitGroup
	const writers = 8
	const band = n / writers
	for g := range writers {
		writersWG.Go(func() {
			base := g * band
			for i := 0; i < band; i++ {
				st.Update(base+i, 1)
				st.Update(base+i, 5) // final value: 5 per slot
			}
		})
	}

	writersWG.Wait()
	close(stop)
	readersWG.Wait()

	// Every slot must hold exactly 5, so Query(0, n-1) == 5n.
	for i := 0; i < n; i++ {
		if v, ok := st.Value(i); !ok || v != 5 {
			t.Fatalf("expected Value(%d)=5 after concurrent writes, got (%d, %v)", i, v, ok)
		}
	}
	if s, ok := st.Query(0, n-1); !ok || s != 5*n {
		t.Fatalf("expected Query(0,%d)=(%d, true) after concurrent writes, got (%d, %v)", n-1, 5*n, s, ok)
	}
	checkInvariants(t, st)
}

// TestConcurrentCompound exercises Lock + Nl* read-modify-write
// sequences from multiple goroutines against one shared tree.  Each
// goroutine increments its own slot 100 times as a locked
// NlValue/NlUpdate pair — without the lock the pair would not be
// atomic, and the shared internal array would race under -race.
func TestConcurrentCompound(t *testing.T) {
	const writers = 8
	st := NewSegmentTree(make([]int, writers))

	var wg sync.WaitGroup
	for g := range writers {
		wg.Go(func() {
			for range 100 {
				st.Lock()
				v, ok := st.NlValue(g)
				if ok {
					st.NlUpdate(g, v+1)
				}
				st.Unlock()
			}
		})
	}
	wg.Wait()

	for g := 0; g < writers; g++ {
		if v, ok := st.Value(g); !ok || v != 100 {
			t.Fatalf("expected Value(%d)=100 after 100 locked increments, got (%d, %v)", g, v, ok)
		}
	}
	if s, _ := st.Query(0, writers-1); s != 100*writers {
		t.Errorf("expected Query(0,%d)=%d, got %d", writers-1, 100*writers, s)
	}
	checkInvariants(t, st)
}
