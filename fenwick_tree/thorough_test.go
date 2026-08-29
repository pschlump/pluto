package fenwick_tree

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"testing"

	"github.com/pschlump/pluto/g_lib"
)

// -------------------------------------------------------------------------------------------------------
// Invariants
// -------------------------------------------------------------------------------------------------------

// checkInvariants verifies the defining invariant of the 1-based
// internal array: for every k in 1..n, tree[k] holds the sum of the
// lowbit(k) values ending at slot k-1 — i.e. tree[k] == Sum(k-1) -
// Sum(k-1-lowbit(k)), where the Sums are reconstructed from the public
// API.  Call it after any structural change.
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
// reference: a plain slice whose prefix and range sums are computed by
// brute-force loops.  O(n) per query and obviously correct — the
// property under test is that the binary-indexed bookkeeping computes
// exactly the same sums under interleaved Adds and Sets.
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
			delta := rng.Intn(201) - 100 // in [-100, 100]
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

// TestFenwickFromMatchesAdd verifies that the O(n) build of
// NewFenwickTreeFrom produces the same internal array as n individual
// Adds, at a fixed seed.
func TestFenwickFromMatchesAdd(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const n = 200
	data := make([]int, n)
	for i := range data {
		data[i] = rng.Intn(1000) - 500
	}

	fromData := NewFenwickTreeFrom(data)
	fromAdds := NewFenwickTree[int](n)
	for i, v := range data {
		fromAdds.Add(i, v)
	}

	for k := range fromData.tree {
		if fromData.tree[k] != fromAdds.tree[k] {
			t.Fatalf("tree[%d]: bulk build has %d, incremental build has %d",
				k, fromData.tree[k], fromAdds.tree[k])
		}
	}
	checkInvariants(t, fromData)
	checkInvariants(t, fromAdds)
}
