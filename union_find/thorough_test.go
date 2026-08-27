package union_find

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Invariants
// -------------------------------------------------------------------------------------------------------

// checkInvariants verifies the structural invariants of the forest:
// every parent chain terminates at a root within n steps (no cycles),
// Find returns that root, Connected agrees with Find on every pair,
// and Count equals the number of distinct roots.  Call it after any
// structural change.
func checkInvariants(t *testing.T, uf *UnionFind) {
	t.Helper()
	n := uf.Len()

	roots := make(map[int]bool, n)
	for p := 0; p < n; p++ {
		// Walk the raw parent chain: it must reach a root (parent[r] == r)
		// within n links — a longer walk means a cycle.
		r := p
		for steps := 0; ; steps++ {
			if steps > n {
				t.Fatalf("parent chain from %d did not terminate within %d steps (cycle in the forest)", p, n)
			}
			if uf.parent[r] == r {
				break
			}
			r = uf.parent[r]
		}
		// Find must agree with the raw walk, and Find's path halving must
		// not corrupt the forest.
		got, ok := uf.Find(p)
		if !ok {
			t.Fatalf("Find(%d) returned ok=false for an in-range element", p)
		}
		if got != r {
			t.Fatalf("Find(%d)=%d but the raw parent chain ends at %d", p, got, r)
		}
		roots[r] = true
	}

	if uf.Count() != len(roots) {
		t.Fatalf("Count()=%d but there are %d distinct roots", uf.Count(), len(roots))
	}

	// Connected must agree with Find on every pair.
	for p := 0; p < n; p++ {
		for q := 0; q < n; q++ {
			rp, _ := uf.Find(p)
			rq, _ := uf.Find(q)
			if uf.Connected(p, q) != (rp == rq) {
				t.Fatalf("Connected(%d,%d)=%v but Find roots are %d and %d", p, q, rp == rq, rp, rq)
			}
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against a reference model (fixed seed)
// -------------------------------------------------------------------------------------------------------

// naiveUF is the reference model: a map from element to set label,
// where Union relabels every element of one set.  O(n) per operation
// and obviously correct — the property under test is that the ranked,
// path-halving forest computes exactly the same partition.
type naiveUF struct {
	label map[int]int
	count int
}

func newNaiveUF(n int) *naiveUF {
	m := &naiveUF{label: make(map[int]int, n), count: n}
	for i := 0; i < n; i++ {
		m.label[i] = i
	}
	return m
}

func (m *naiveUF) union(p, q int) bool {
	lp, lq := m.label[p], m.label[q]
	if lp == lq {
		return false
	}
	for k, v := range m.label {
		if v == lp {
			m.label[k] = lq
		}
	}
	m.count--
	return true
}

func (m *naiveUF) connected(p, q int) bool {
	return m.label[p] == m.label[q]
}

func TestUnionFindRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const n = 24 // small element space so merges and reconnects are common
	uf := NewUnionFind(n)
	model := newNaiveUF(n)

	verify := func(step int) {
		if uf.Count() != model.count {
			t.Fatalf("step %d: Count()=%d, model has %d sets", step, uf.Count(), model.count)
		}
		// Full pairwise comparison of the connectivity relation.
		for p := 0; p < n; p++ {
			for q := 0; q < n; q++ {
				if got := uf.Connected(p, q); got != model.connected(p, q) {
					t.Fatalf("step %d: Connected(%d,%d)=%v, model says %v",
						step, p, q, got, model.connected(p, q))
				}
			}
		}
		checkInvariants(t, uf)
	}

	for step := range 800 {
		p := rng.Intn(n)
		q := rng.Intn(n)
		switch rng.Intn(10) {
		case 0, 1, 2, 3, 4: // Union
			got := uf.Union(p, q)
			want := model.union(p, q)
			if got != want {
				t.Fatalf("step %d: Union(%d,%d)=%v, model says %v", step, p, q, got, want)
			}
		case 5, 6, 7, 8: // Connected
			got := uf.Connected(p, q)
			want := model.connected(p, q)
			if got != want {
				t.Fatalf("step %d: Connected(%d,%d)=%v, model says %v", step, p, q, got, want)
			}
		case 9: // Find — the root is arbitrary, but it must be a set member:
			// Connected(p, root) must hold, and repeated Finds must agree.
			r1, ok1 := uf.Find(p)
			r2, ok2 := uf.Find(q)
			if !ok1 || !ok2 {
				t.Fatalf("step %d: Find returned ok=false for in-range elements", step)
			}
			if (r1 == r2) != model.connected(p, q) {
				t.Fatalf("step %d: Find roots (%d,%d) disagree with model connectivity for (%d,%d)",
					step, r1, r2, p, q)
			}
		}
		if step%37 == 0 {
			verify(step)
		}
	}
	verify(800)

	// Union every element into one set; the forest must stay consistent.
	for i := 1; i < n; i++ {
		uf.Union(0, i)
	}
	if uf.Count() != 1 {
		t.Errorf("Expected Count()=1 after a full merge, got %d", uf.Count())
	}
	checkInvariants(t, uf)
}
