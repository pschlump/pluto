package segment_tree

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Invariants
// -------------------------------------------------------------------------------------------------------

// checkInvariants verifies the defining invariant of the array-backed
// tree: every internal node holds the combine of its two children,
// every leaf at size+i (i < n) holds Value(i), and every padding leaf
// holds the identity.  Call it after any structural change.
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

// TestSegmentTreeRandomizedModel cross-checks three trees over the SAME
// data — a sum tree (NewSegmentTree), a min tree, and a max tree (both
// via NewSegmentTreeFunc) — against brute-force range loops over a
// naive reference slice, with random interleaved updates and queries.
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
			if s, ok := sumTree.Query(lo, hi); !ok || s != brute(func(a, b int) int { return a + b }, 0, lo, hi) {
				t.Fatalf("step %d: sum Query(%d,%d)=(%d,%v), model has %d",
					step, lo, hi, s, ok, brute(func(a, b int) int { return a + b }, 0, lo, hi))
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
			if s, ok := sumTree.Query(0, n-1); !ok || s != brute(func(a, b int) int { return a + b }, 0, 0, n-1) {
				t.Fatalf("step %d: sum full Query=(%d,%v), model has %d",
					step, s, ok, brute(func(a, b int) int { return a + b }, 0, 0, n-1))
			}
		}
		if step%251 == 0 {
			verify(step)
		}
	}
	verify(3000)
}
